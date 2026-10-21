package routes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"pocketbase/util"
	"strings"
	"testing"
)

type remoteLimitBody struct {
	io.Reader
	read   int64
	closed bool
}

func (b *remoteLimitBody) Read(p []byte) (int, error) {
	n, e := b.Reader.Read(p)
	b.read += int64(n)
	return n, e
}
func (b *remoteLimitBody) Close() error { b.closed = true; return nil }

type remoteLimitTransport struct {
	body   *remoteLimitBody
	status int
}

func (t remoteLimitTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: t.status, Header: make(http.Header), ContentLength: -1, Body: t.body, Request: r}, nil
}

func TestRemoteObjectFetchBoundsChunkedJSONBeforeDecode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"remote-id","values":[`)
		w.(http.Flusher).Flush()
		chunk := strings.Repeat("0,", 4096)
		for i := int64(0); i < util.FederationObjectBodyLimit/8192+2; i++ {
			if _, err := io.WriteString(w, chunk); err != nil {
				return
			}
		}
		fmt.Fprint(w, `0]}`)
	}))
	t.Cleanup(server.Close)
	remoteURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	data, err := fetchRemoteDocument(context.Background(), server.Client(), remoteURL, "")
	if !errors.Is(err, util.ErrFederationBodyTooLarge) || data != nil {
		t.Fatalf("oversize remote record decoded: data=%v err=%v", data, err)
	}
}

func TestRemoteSyncDocumentBodies(t *testing.T) {
	t.Setenv("ORIGIN", "https://local.example")
	for _, collection := range []string{"trails", "lists"} {
		for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
			t.Run(fmt.Sprintf("%s/%d", collection, status), func(t *testing.T) {
				f := newRemoteSyncFixture(t)
				col := f.trails
				kind := "trail"
				if collection == "lists" {
					col = f.lists
					kind = "list"
				}
				record := f.save(col, map[string]any{"name": "Stored", "author": f.remoteActor.Id, "iri": "https://remote.example/api/v1/" + kind + "/remote-id"})
				body := &remoteLimitBody{Reader: strings.NewReader(`{"id":"remote-id","name":"Remote"}` + strings.Repeat(" ", int(util.FederationSyncBodyLimit)))}
				previous := newRemoteSyncHTTPClient
				newRemoteSyncHTTPClient = func() *http.Client { return &http.Client{Transport: remoteLimitTransport{body: body, status: status}} }
				defer func() { newRemoteSyncHTTPClient = previous }()
				var err error
				if collection == "trails" {
					_, err = performFullSync(f.app, t.Context(), &url.URL{}, record)
				} else {
					_, err = performFullListSync(f.app, t.Context(), &url.URL{}, record)
				}
				if err == nil || !body.closed {
					t.Fatalf("document response: error=%v closed=%t", err, body.closed)
				}
				if status == http.StatusOK && (!errors.Is(err, util.ErrFederationBodyTooLarge) || body.read != util.FederationSyncBodyLimit+1) {
					t.Fatalf("document read=%d error=%v", body.read, err)
				}
				stored, findErr := f.app.FindRecordById(collection, record.Id)
				if findErr != nil || stored.GetString("name") != "Stored" {
					t.Fatalf("failed document changed record: %v %v", stored, findErr)
				}
			})
		}
	}
}
