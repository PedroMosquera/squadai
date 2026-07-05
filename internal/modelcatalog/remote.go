package modelcatalog

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RemoteURL is the canonical location of the latest published catalog.
// It is a variable so tests can point it at an httptest server. Only the
// `models check` and `models update` commands ever fetch it — no other
// code path performs network I/O.
var RemoteURL = "https://raw.githubusercontent.com/PedroMosquera/squadai/main/internal/assets/models/models.json"

// maxRemoteSize caps the remote catalog size (defense against a bad URL
// serving something huge).
const maxRemoteSize = 1 << 20 // 1 MiB

// remoteClient is the HTTP client used for catalog fetches (short timeout).
var remoteClient = &http.Client{Timeout: 5 * time.Second}

// FetchRemote downloads, size-caps, parses, and validates the published
// catalog. It returns both the parsed document and the raw bytes so callers
// can write the exact upstream content to an override file.
func FetchRemote(ctx context.Context) (*File, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, RemoteURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build catalog request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "squadai-models")

	resp, err := remoteClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch models catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("fetch models catalog: server returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteSize+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read models catalog: %w", err)
	}
	if len(data) > maxRemoteSize {
		return nil, nil, fmt.Errorf("fetch models catalog: response exceeds %d bytes", maxRemoteSize)
	}

	f, err := Parse(data)
	if err != nil {
		return nil, nil, fmt.Errorf("remote models catalog: %w", err)
	}
	return f, data, nil
}
