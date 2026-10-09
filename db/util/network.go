package util

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

var ErrRateLimited = fmt.Errorf("rate limit exceeded for origin")

type RateLimiter struct {
	mu           sync.RWMutex
	requests     map[string][]time.Time
	maxReqs      int
	window       time.Duration
	key          []byte
	retainWindow bool
}

func NewRateLimiter(maxReqs int, window time.Duration) *RateLimiter {
	return newRateLimiter(maxReqs, window, false)
}

func newRateLimiter(maxReqs int, window time.Duration, retainWindow bool) *RateLimiter {
	rl := &RateLimiter{
		requests:     make(map[string][]time.Time),
		maxReqs:      maxReqs,
		window:       window,
		key:          make([]byte, 32),
		retainWindow: retainWindow,
	}
	rand.Read(rl.key)

	// Background worker: Cleans up memory and rotates keys
	go rl.maintenanceWorker()
	return rl
}

func (rl *RateLimiter) maintenanceWorker() {
	ticker := time.NewTicker(rl.window * 2)
	for now := range ticker.C {
		rl.maintain(now)
	}
}

func (rl *RateLimiter) CheckRateLimit(identifier string, host string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	h := hmac.New(sha256.New, rl.key)
	h.Write([]byte(identifier + ":" + host))
	key := hex.EncodeToString(h.Sum(nil))

	now := time.Now()
	threshold := now.Add(-rl.window)

	timestamps := rl.requests[key]
	w := 0
	for _, t := range timestamps {
		if t.After(threshold) {
			timestamps[w] = t
			w++
		}
	}
	timestamps = timestamps[:w]

	if len(timestamps) >= rl.maxReqs {
		rl.requests[key] = timestamps
		return ErrRateLimited
	}

	rl.requests[key] = append(timestamps, now)
	return nil
}

var ActivityPubRateLimiter = NewRateLimiter(30, time.Minute)

// KnownActorKeyReloads admits one refresh per actor and minute. Maintenance
// removes expired claims while preserving the full window of recent claims.
var KnownActorKeyReloads = newRateLimiter(1, time.Minute, true)

type rateLimitBucketKey struct{}

// WithRateLimitBucket changes the dial budget without changing the actor
// whose key signs the request.
func WithRateLimitBucket(ctx context.Context, bucket string) context.Context {
	return context.WithValue(ctx, rateLimitBucketKey{}, bucket)
}

// CheckDialRateLimit applies the dial budget selected by ctx.
func CheckDialRateLimit(ctx context.Context, host string) error {
	identifier, _ := ctx.Value(rateLimitBucketKey{}).(string)
	if identifier == "" {
		identifier, _ = ctx.Value("actor").(string)
	}
	if identifier == "" {
		identifier = "system"
	}
	return ActivityPubRateLimiter.CheckRateLimit(identifier, host)
}

type safeTransport struct {
	transport http.RoundTripper
}

func (t *safeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()
	if host == "" {
		return nil, fmt.Errorf("invalid host in request")
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve host: %w", err)
	}

	for _, ip := range ips {
		if isPrivateOrReservedIP(ip) {
			return nil, fmt.Errorf("request to private/reserved IP address blocked: %s", ip)
		}
	}

	return t.transport.RoundTrip(req)
}

func isPrivateOrReservedIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		// Fail closed: an address we cannot parse is one we cannot vet.
		return true
	}

	// Both the literal address and any IPv4 it embeds via an IPv6 transition
	// mechanism (NAT64, 6to4, Teredo, IPv4-compatible) must clear the policy.
	for _, candidate := range policyCandidates(addr) {
		if isReservedAddr(candidate) || candidate.IsPrivate() {
			return true
		}
	}

	return false
}

// publicDialContext resolves the host, refuses private or reserved addresses
// and dials the vetted IP, so a DNS answer cannot change between check and
// connect.
func publicDialContext(dialer *net.Dialer, rateLimit bool) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, _ := net.SplitHostPort(addr)

		if rateLimit {
			identifier, _ := ctx.Value("actor").(string)
			if identifier == "" {
				identifier = "system"
			}

			if err := ActivityPubRateLimiter.CheckRateLimit(identifier, host); err != nil {
				return nil, err
			}
		}

		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil || len(ips) == 0 {
			return nil, fmt.Errorf("failed to resolve: %w", err)
		}

		for _, ip := range ips {
			if isPrivateOrReservedIP(ip) {
				return nil, fmt.Errorf("SSRF blocked: %s", ip)
			}
		}

		// Standard practice: Dial the first resolved IP to prevent TOCTOU/Rebinding
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
}

func SafeHTTPClient() *http.Client {
	return safeHTTPClientWithTimeouts(30*time.Second, 10*time.Second, 10*time.Second)
}

func baseHTTPClientWithTimeouts(total, headers, handshake time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 30 * time.Second}

	return &http.Client{
		Timeout: total,
		Transport: &http.Transport{
			ResponseHeaderTimeout: headers,
			TLSHandshakeTimeout:   handshake,
			DialContext:           publicDialContext(dialer, true),
		},
	}
}

// SafeDeliveryClient posts activities to remote inboxes. Inbox URLs come from
// remote actor documents, so only public addresses are dialed and redirects
// are not followed. It is not rate limited: fanout to one host is legitimate.
func SafeDeliveryClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout}

	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: publicDialContext(dialer, false),
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func GetSafeActorContext(r *http.Request, userActor *core.Record) (context.Context, error) {
	var identifier string

	if userActor != nil {
		identifier = "actor:" + userActor.Id
	} else if r != nil {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		identifier = "anon:" + ip
	} else {
		return nil, errors.New("request or actor must be defined")
	}

	parentCtx := context.Background()
	if r != nil {
		parentCtx = r.Context()
	}
	return context.WithValue(parentCtx, "actor", identifier), nil
}
