package util

import (
	"context"
	"crypto/rand"
	"net"
	"net/http"
	"time"
)

func (rl *RateLimiter) maintain(now time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.retainWindow {
		threshold := now.Add(-rl.window)
		for key, timestamps := range rl.requests {
			remaining := timestamps[:0]
			for _, timestamp := range timestamps {
				if timestamp.After(threshold) {
					remaining = append(remaining, timestamp)
				}
			}
			if len(remaining) == 0 {
				delete(rl.requests, key)
			} else {
				rl.requests[key] = remaining
			}
		}
		return
	}

	newKey := make([]byte, 32)
	rand.Read(newKey)
	rl.key = newKey
	rl.requests = make(map[string][]time.Time)
}

func safeHTTPClientWithTimeouts(total, headers, handshake time.Duration) *http.Client {
	client := baseHTTPClientWithTimeouts(total, headers, handshake)
	transport := client.Transport.(*http.Transport)
	dial := transport.DialContext
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if bucket, _ := ctx.Value(rateLimitBucketKey{}).(string); bucket != "" {
			ctx = context.WithValue(ctx, "actor", bucket)
		}
		return dial(ctx, network, addr)
	}
	return client
}
