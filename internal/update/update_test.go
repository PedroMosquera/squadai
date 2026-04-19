package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// ─── version.go ──────────────────────────────────────────────────────────────

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input   string
		want    semver
		wantErr bool
	}{
		{"v1.2.3", semver{1, 2, 3}, false},
		{"1.2.3", semver{1, 2, 3}, false},
		{"v0.0.0", semver{0, 0, 0}, false},
		{"v2.10.5", semver{2, 10, 5}, false},
		{"1.2.3-rc.1", semver{1, 2, 3}, false},
		{"bad", semver{}, true},
		{"1.2", semver{}, true},
		{"a.b.c", semver{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseSemver(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseSemver(%q) expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSemver(%q): %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("parseSemver(%q) = %+v, want %+v", tc.input, got, tc.want)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current, candidate semver
		want               bool
	}{
		{semver{1, 0, 0}, semver{1, 0, 1}, true},
		{semver{1, 0, 0}, semver{1, 1, 0}, true},
		{semver{1, 0, 0}, semver{2, 0, 0}, true},
		{semver{1, 2, 3}, semver{1, 2, 3}, false},
		{semver{1, 2, 4}, semver{1, 2, 3}, false},
		{semver{2, 0, 0}, semver{1, 9, 9}, false},
	}
	for _, tc := range tests {
		name := fmt.Sprintf("%v_vs_%v", tc.current, tc.candidate)
		t.Run(name, func(t *testing.T) {
			got := isNewer(tc.current, tc.candidate)
			if got != tc.want {
				t.Errorf("isNewer(%v, %v) = %v, want %v", tc.current, tc.candidate, got, tc.want)
			}
		})
	}
}

func TestIsDevBuild(t *testing.T) {
	if !isDevBuild("dev") {
		t.Error("'dev' should be a dev build")
	}
	if !isDevBuild("") {
		t.Error("empty string should be a dev build")
	}
	if isDevBuild("v1.2.3") {
		t.Error("'v1.2.3' should not be a dev build")
	}
}

// ─── github.go ───────────────────────────────────────────────────────────────

// testTransport redirects all requests to the given httptest.Server.
type testTransport struct {
	server *httptest.Server
}

func (tt *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = tt.server.Listener.Addr().String()
	return http.DefaultTransport.RoundTrip(req2)
}

func TestFetchLatestRelease_Success(t *testing.T) {
	release := githubRelease{
		TagName:    "v1.3.0",
		Prerelease: false,
		Draft:      false,
	}
	fakeGitHubServer(t, http.StatusOK, release)

	got, err := fetchLatestRelease(context.Background(), "v1.2.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TagName != "v1.3.0" {
		t.Errorf("TagName = %q, want v1.3.0", got.TagName)
	}
}

func TestFetchLatestRelease_SkipsPrerelease(t *testing.T) {
	fakeGitHubServer(t, http.StatusOK, githubRelease{TagName: "v2.0.0-rc1", Prerelease: true})

	_, err := fetchLatestRelease(context.Background(), "v1.0.0")
	if err != ErrNoRelease {
		t.Errorf("expected ErrNoRelease, got %v", err)
	}
}

func TestFetchLatestRelease_SkipsDraft(t *testing.T) {
	fakeGitHubServer(t, http.StatusOK, githubRelease{TagName: "v2.0.0", Draft: true})

	_, err := fetchLatestRelease(context.Background(), "v1.0.0")
	if err != ErrNoRelease {
		t.Errorf("expected ErrNoRelease, got %v", err)
	}
}

func TestFetchLatestRelease_HTTP403(t *testing.T) {
	fakeGitHubServer(t, http.StatusForbidden, map[string]string{"message": "rate limit"})

	_, err := fetchLatestRelease(context.Background(), "v1.0.0")
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
}

func TestFetchLatestRelease_UserAgent(t *testing.T) {
	var gotUA string
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(githubRelease{TagName: "v1.1.0"}) //nolint:errcheck
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	orig := checkClient
	t.Cleanup(func() { checkClient = orig })
	checkClient = &http.Client{Transport: &testTransport{server: srv}}

	_, _ = fetchLatestRelease(context.Background(), "v1.0.0")
	if gotUA != "squadai/v1.0.0" {
		t.Errorf("User-Agent = %q, want squadai/v1.0.0", gotUA)
	}
}

// ─── download.go ─────────────────────────────────────────────────────────────

func TestAssetName_Format(t *testing.T) {
	name := assetName("v1.2.3")
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	want := fmt.Sprintf("squadai_1.2.3_%s_%s.tar.gz", goos, goarch)
	if name != want {
		t.Errorf("assetName(v1.2.3) = %q, want %q", name, want)
	}
}

func TestAssetName_StripsVPrefix(t *testing.T) {
	if assetName("v2.0.0") != assetName("2.0.0") {
		t.Error("assetName should strip v prefix")
	}
}

func TestDownload_EndToEnd(t *testing.T) {
	archiveData := buildFakeTarGz(t, "squadai", []byte("#!/bin/sh\necho fake"))
	archiveName := assetName("v1.3.0")
	checksumContent := fakeChecksumFile(t, archiveData, archiveName)

	mux := http.NewServeMux()
	mux.HandleFunc("/archive", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(archiveData) //nolint:errcheck
	})
	mux.HandleFunc("/checksums", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(checksumContent) //nolint:errcheck
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	origDL := downloadClient
	t.Cleanup(func() { downloadClient = origDL })
	downloadClient = &http.Client{Transport: &testTransport{server: srv}}

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	release := &githubRelease{
		TagName: "v1.3.0",
		Assets: []githubAsset{
			{
				Name:               archiveName,
				BrowserDownloadURL: "http://ignored/archive",
				Size:               int64(len(archiveData)),
			},
			{
				Name:               "checksums.txt",
				BrowserDownloadURL: "http://ignored/checksums",
			},
		},
	}

	if err := Download(context.Background(), release, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}

	manifest, err := LoadPendingUpdate()
	if err != nil || manifest == nil {
		t.Fatalf("LoadPendingUpdate: err=%v, manifest=%v", err, manifest)
	}
	if manifest.Version != "v1.3.0" {
		t.Errorf("manifest.Version = %q, want v1.3.0", manifest.Version)
	}
	info, err := os.Stat(manifest.BinaryPath)
	if err != nil {
		t.Fatalf("binary stat: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("binary is not executable")
	}
}

func TestDownload_NoAssetForPlatform(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	release := &githubRelease{
		TagName: "v1.3.0",
		Assets: []githubAsset{
			{Name: "squadai_1.3.0_someother_arch.tar.gz", BrowserDownloadURL: "http://x"},
		},
	}
	err := Download(context.Background(), release, nil)
	if err == nil {
		t.Fatal("expected error when no asset matches platform")
	}
}

// ─── update.go ───────────────────────────────────────────────────────────────

func TestCheck_DevBuild(t *testing.T) {
	_, err := Check(context.Background(), "dev")
	if err != ErrDevBuild {
		t.Errorf("expected ErrDevBuild, got %v", err)
	}
}

func TestCheck_AlreadyCurrent(t *testing.T) {
	fakeGitHubServer(t, http.StatusOK, githubRelease{TagName: "v1.2.3"})

	result, err := Check(context.Background(), "v1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UpdateAvailable {
		t.Error("should not have update when versions match")
	}
}

func TestCheck_NewerAvailable(t *testing.T) {
	fakeGitHubServer(t, http.StatusOK, githubRelease{TagName: "v1.3.0"})

	result, err := Check(context.Background(), "v1.2.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.UpdateAvailable {
		t.Error("should have update available")
	}
	if result.LatestVersion != "v1.3.0" {
		t.Errorf("LatestVersion = %q, want v1.3.0", result.LatestVersion)
	}
}

// ─── apply.go (decision logic) ───────────────────────────────────────────────

func TestLoadPendingUpdate_Missing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	p, err := LoadPendingUpdate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != nil {
		t.Error("expected nil for missing pending.json")
	}
}

func TestLoadPendingUpdate_Roundtrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".squadai", "updates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	pending := PendingUpdate{
		Version:      "v1.3.0",
		BinaryPath:   "/tmp/squadai-v1.3.0",
		DownloadedAt: time.Now().UTC().Truncate(time.Second),
	}
	data, _ := json.MarshalIndent(pending, "", "  ")
	path := filepath.Join(dir, "pending.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadPendingUpdate()
	if err != nil {
		t.Fatalf("LoadPendingUpdate: %v", err)
	}
	if got.Version != "v1.3.0" {
		t.Errorf("Version = %q, want v1.3.0", got.Version)
	}
}

func TestClearPendingUpdate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir := filepath.Join(tmp, ".squadai", "updates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "pending.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ClearPendingUpdate(); err != nil {
		t.Fatalf("ClearPendingUpdate: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("pending.json should be deleted")
	}

	// Calling again should be a no-op.
	if err := ClearPendingUpdate(); err != nil {
		t.Errorf("ClearPendingUpdate on missing file: %v", err)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func fakeGitHubServer(t *testing.T, status int, body interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(body) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)

	origCheck := checkClient
	origDL := downloadClient
	t.Cleanup(func() {
		checkClient = origCheck
		downloadClient = origDL
	})
	transport := &testTransport{server: srv}
	checkClient = &http.Client{Transport: transport}
	downloadClient = &http.Client{Transport: transport}

	return srv
}

func buildFakeTarGz(t *testing.T, binaryName string, content []byte) []byte {
	t.Helper()
	tmpFile, err := os.CreateTemp(t.TempDir(), "*.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer tmpFile.Close()

	gw := gzip.NewWriter(tmpFile)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name:     binaryName,
		Mode:     0o755,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fakeChecksumFile(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	h := sha256.Sum256(data)
	return []byte(fmt.Sprintf("%s  %s\n", hex.EncodeToString(h[:]), name))
}
