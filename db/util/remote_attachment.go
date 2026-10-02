package util

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

const (
	// Per-file size limits, equal to the maxSize of the matching collection fields.
	RemoteGPXMaxBytes    int64 = 5 << 20
	RemotePhotoMaxBytes  int64 = 20 << 20
	RemoteAvatarMaxBytes int64 = 5 << 20

	// RemoteAttachmentTimeout bounds a single attachment download.
	RemoteAttachmentTimeout = 30 * time.Second

	// RemoteMaxPhotosPerObject is the photo cap used when the collection's photos
	// field cannot be read (maxSelect is 99 on trails and summit_logs).
	RemoteMaxPhotosPerObject = 99
)

var (
	remoteAttachmentClientMu sync.RWMutex
	remoteAttachmentClient   HTTPDoer
)

// SetRemoteAttachmentClientForTesting replaces the HTTP client used by
// DownloadRemoteFile and returns a restore function. Tests only.
func SetRemoteAttachmentClientForTesting(c HTTPDoer) (restore func()) {
	remoteAttachmentClientMu.Lock()
	previous := remoteAttachmentClient
	remoteAttachmentClient = c
	remoteAttachmentClientMu.Unlock()
	return func() {
		remoteAttachmentClientMu.Lock()
		remoteAttachmentClient = previous
		remoteAttachmentClientMu.Unlock()
	}
}

func installedRemoteAttachmentClient() HTTPDoer {
	remoteAttachmentClientMu.RLock()
	defer remoteAttachmentClientMu.RUnlock()
	return remoteAttachmentClient
}

// RemotePhotoLimit returns how many photos may be stored on one record of the
// collection. Task 2 of 07-18 reads the collection's photos maxSelect; until
// then it always returns RemoteMaxPhotosPerObject and nothing calls it.
func RemotePhotoLimit(app core.App, collection string) int {
	return RemoteMaxPhotosPerObject
}

// DownloadRemoteFile downloads rawURL and returns it as a file together with a
// cleanup function (never nil) that the caller must call once the record has
// been saved or the activity has failed. Task 2 of 07-18 makes the download
// safe; today it reproduces filesystem.NewFileFromURL: maxBytes is not yet
// enforced and there is no deadline.
func DownloadRemoteFile(ctx context.Context, rawURL string, maxBytes int64) (*filesystem.File, func(), error) {
	noop := func() {}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, noop, err
	}
	var client HTTPDoer = http.DefaultClient
	if c := installedRemoteAttachmentClient(); c != nil {
		client = c
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, noop, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 399 {
		return nil, noop, fmt.Errorf("failed to download url %s (%d)", rawURL, res.StatusCode)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, res.Body); err != nil {
		return nil, noop, err
	}
	f, err := filesystem.NewFileFromBytes(buf.Bytes(), path.Base(rawURL))
	if err != nil {
		return nil, noop, err
	}
	return f, noop, nil
}
