package modelcatalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// withRemoteURL points FetchRemote at a test server for the test's duration.
func withRemoteURL(t *testing.T, url string) {
	t.Helper()
	prev := RemoteURL
	RemoteURL = url
	t.Cleanup(func() { RemoteURL = prev })
}

func TestFetchRemote_Happy(t *testing.T) {
	body := `{
		"schema_version": 1,
		"updated": "2027-03-01",
		"models": {"fresh-model": {"provider": "acme", "input_per_mtok": 1, "output_per_mtok": 2}}
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	withRemoteURL(t, srv.URL)

	f, raw, err := FetchRemote(context.Background())
	if err != nil {
		t.Fatalf("FetchRemote: %v", err)
	}
	if f.Updated != "2027-03-01" {
		t.Errorf("Updated = %q, want 2027-03-01", f.Updated)
	}
	if _, ok := f.Models["fresh-model"]; !ok {
		t.Error("fresh-model missing from fetched catalog")
	}
	if string(raw) != body {
		t.Error("raw bytes should be the exact server response")
	}
}

func TestFetchRemote_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()
	withRemoteURL(t, srv.URL)

	_, _, err := FetchRemote(context.Background())
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("FetchRemote error = %v, want 404 mention", err)
	}
}

func TestFetchRemote_Oversize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"schema_version": 1, "updated": "`))
		filler := strings.Repeat("x", maxRemoteSize+16)
		_, _ = w.Write([]byte(filler))
	}))
	defer srv.Close()
	withRemoteURL(t, srv.URL)

	_, _, err := FetchRemote(context.Background())
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("FetchRemote error = %v, want size-cap error", err)
	}
}

func TestFetchRemote_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not json`))
	}))
	defer srv.Close()
	withRemoteURL(t, srv.URL)

	_, _, err := FetchRemote(context.Background())
	if err == nil || !strings.Contains(err.Error(), "parse models catalog") {
		t.Errorf("FetchRemote error = %v, want parse error", err)
	}
}

func TestFetchRemote_InvalidSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"schema_version": 42}`))
	}))
	defer srv.Close()
	withRemoteURL(t, srv.URL)

	_, _, err := FetchRemote(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unsupported schema_version") {
		t.Errorf("FetchRemote error = %v, want schema error", err)
	}
}
