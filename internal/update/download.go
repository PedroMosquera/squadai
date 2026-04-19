package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// downloadClient is the HTTP client used for asset downloads (longer timeout).
var downloadClient = &http.Client{Timeout: 30 * time.Second}

// PendingUpdate is the manifest written to pending.json after a successful download.
type PendingUpdate struct {
	Version      string    `json:"version"`
	BinaryPath   string    `json:"binary_path"`
	DownloadedAt time.Time `json:"downloaded_at"`
}

// pendingDir returns ~/.squadai/updates.
func pendingDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".squadai", "updates"), nil
}

// PendingManifestPath returns the path to pending.json.
func PendingManifestPath() (string, error) {
	dir, err := pendingDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pending.json"), nil
}

// assetName returns the expected goreleaser archive name for the current platform.
// Goreleaser template: {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}
// .Version strips the leading "v" from the tag.
func assetName(tagVersion string) string {
	v := strings.TrimPrefix(tagVersion, "v")
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	return fmt.Sprintf("squadai_%s_%s_%s.tar.gz", v, goos, goarch)
}

// checksumAssetName returns the checksums file name.
func checksumAssetName() string {
	return "checksums.txt"
}

// Download fetches the release binary for the current platform, verifies its
// checksum (if available), extracts it, and writes a pending.json manifest.
// progress receives download status messages (may be nil).
func Download(ctx context.Context, release *githubRelease, progress func(string)) error {
	wantAsset := assetName(release.TagName)
	wantChecksum := checksumAssetName()

	var assetURL string
	var assetSize int64
	var checksumURL string

	for _, a := range release.Assets {
		switch a.Name {
		case wantAsset:
			assetURL = a.BrowserDownloadURL
			assetSize = a.Size
		case wantChecksum:
			checksumURL = a.BrowserDownloadURL
		}
	}

	if assetURL == "" {
		return fmt.Errorf("no asset found for platform %s/%s (wanted %q)", runtime.GOOS, runtime.GOARCH, wantAsset)
	}

	dir, err := pendingDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create updates dir: %w", err)
	}

	// Download archive to a temp file.
	archiveTmp := filepath.Join(dir, ".download-*.tar.gz.tmp")
	archivePath, err := downloadToTemp(ctx, assetURL, archiveTmp, assetSize, progress)
	if err != nil {
		return fmt.Errorf("download archive: %w", err)
	}
	defer os.Remove(archivePath)

	// Optionally verify checksum.
	if checksumURL != "" {
		if err := verifyChecksum(ctx, archivePath, wantAsset, checksumURL); err != nil {
			return fmt.Errorf("checksum verification: %w", err)
		}
	} else {
		if progress != nil {
			progress("warning: no checksums.txt found — skipping checksum verification")
		}
	}

	// Extract binary from archive.
	binaryPath := filepath.Join(dir, fmt.Sprintf("squadai-%s", release.TagName))
	if err := extractBinary(archivePath, binaryPath); err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}

	// Write pending manifest.
	manifest := PendingUpdate{
		Version:      release.TagName,
		BinaryPath:   binaryPath,
		DownloadedAt: time.Now().UTC(),
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	data = append(data, '\n')

	manifestPath, err := PendingManifestPath()
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".pending-*.tmp")
	if err != nil {
		return fmt.Errorf("create manifest temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write manifest temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close manifest temp: %w", err)
	}
	if err := os.Rename(tmpName, manifestPath); err != nil {
		return fmt.Errorf("rename manifest: %w", err)
	}
	tmpName = ""

	return nil
}

// downloadToTemp streams url to a temp file and returns its path.
func downloadToTemp(ctx context.Context, url, pattern string, _ int64, progress func(string)) (string, error) {
	dir := filepath.Dir(pattern)
	base := filepath.Base(pattern)
	// pattern like ".download-*.tar.gz.tmp" — CreateTemp uses dir + pattern
	tmp, err := os.CreateTemp(dir, base)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	path := tmp.Name()

	cleanup := func() {
		tmp.Close()
		os.Remove(path)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		cleanup()
		return "", fmt.Errorf("build download request: %w", err)
	}

	resp, err := downloadClient.Do(req)
	if err != nil {
		cleanup()
		return "", fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cleanup()
		return "", fmt.Errorf("download returned %d", resp.StatusCode)
	}

	if progress != nil {
		progress(fmt.Sprintf("downloading %s…", url))
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		cleanup()
		return "", fmt.Errorf("stream download: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("close temp file: %w", err)
	}

	return path, nil
}

// verifyChecksum downloads checksums.txt and verifies the archive against it.
func verifyChecksum(ctx context.Context, archivePath, assetName, checksumURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return fmt.Errorf("build checksum request: %w", err)
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return fmt.Errorf("checksum request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checksum download returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}

	// Parse checksums.txt (sha256sum format: "<hash>  <filename>\n").
	wantHash := ""
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			wantHash = parts[0]
			break
		}
	}

	if wantHash == "" {
		return fmt.Errorf("asset %q not found in checksums.txt", assetName)
	}

	// Hash the local file.
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive for hashing: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash archive: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))

	if got != wantHash {
		return fmt.Errorf("checksum mismatch: got %s, want %s", got, wantHash)
	}

	return nil
}

// extractBinary extracts the "squadai" binary from a .tar.gz archive and writes
// it to destPath with mode 0755.
func extractBinary(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		// Match the binary by base name "squadai" (could be in a subdir).
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) != "squadai" {
			continue
		}

		// Write to temp then rename.
		dir := filepath.Dir(destPath)
		tmp, err := os.CreateTemp(dir, ".squadai-binary-*.tmp")
		if err != nil {
			return fmt.Errorf("create binary temp: %w", err)
		}
		tmpName := tmp.Name()

		cleanup := func() {
			tmp.Close()
			os.Remove(tmpName)
		}

		if _, err := io.Copy(tmp, tr); err != nil {
			cleanup()
			return fmt.Errorf("write binary: %w", err)
		}
		if err := tmp.Chmod(0o755); err != nil {
			cleanup()
			return fmt.Errorf("chmod binary: %w", err)
		}
		if err := tmp.Close(); err != nil {
			os.Remove(tmpName)
			return fmt.Errorf("close binary temp: %w", err)
		}
		if err := os.Rename(tmpName, destPath); err != nil {
			os.Remove(tmpName)
			return fmt.Errorf("rename binary: %w", err)
		}
		return nil
	}

	return fmt.Errorf("binary %q not found in archive", "squadai")
}

// LoadPendingUpdate reads pending.json. Returns nil, nil if not present.
func LoadPendingUpdate() (*PendingUpdate, error) {
	path, err := PendingManifestPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read pending manifest: %w", err)
	}
	var p PendingUpdate
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode pending manifest: %w", err)
	}
	return &p, nil
}

// ClearPendingUpdate deletes pending.json.
func ClearPendingUpdate() error {
	path, err := PendingManifestPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove pending manifest: %w", err)
	}
	return nil
}
