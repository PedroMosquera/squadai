package doctor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// --- helpers ---

// fakeLooker implements PathLooker with a fixed set of "found" binaries.
type fakeLooker struct {
	found map[string]string // binary name → full path
}

func (f fakeLooker) LookPath(file string) (string, error) {
	if path, ok := f.found[file]; ok {
		return path, nil
	}
	return "", errors.New("not found: " + file)
}

// fakeRunner implements CommandRunner with canned outputs.
type fakeRunner struct {
	outputs map[string][]byte // "cmd arg1 arg2" → output bytes
	errors  map[string]error  // "cmd arg1 arg2" → error
}

func (f fakeRunner) Output(name string, args ...string) ([]byte, error) {
	key := name
	for _, a := range args {
		key += " " + a
	}
	if err, ok := f.errors[key]; ok {
		return nil, err
	}
	if out, ok := f.outputs[key]; ok {
		return out, nil
	}
	return nil, errors.New("no output for: " + key)
}

func newTestDoctor(t *testing.T, looker PathLooker, runner CommandRunner) *Doctor {
	t.Helper()
	tmp := t.TempDir()
	return NewWithDeps(tmp, tmp, nil, nil, looker, runner)
}

// --- Environment checks ---

func TestCheckGo(t *testing.T) {
	tests := []struct {
		name       string
		lookFound  map[string]string
		runOutputs map[string][]byte
		runErrors  map[string]error
		wantStatus CheckStatus
	}{
		{
			name:       "Go_found_with_version",
			lookFound:  map[string]string{"go": "/usr/local/go/bin/go"},
			runOutputs: map[string][]byte{"go version": []byte("go version go1.22.4 linux/amd64")},
			wantStatus: CheckPass,
		},
		{
			name:       "Go_not_found",
			lookFound:  map[string]string{},
			wantStatus: CheckFail,
		},
		{
			name:       "Go_found_but_version_fails",
			lookFound:  map[string]string{"go": "/usr/local/bin/go"},
			runErrors:  map[string]error{"go version": errors.New("exec error")},
			wantStatus: CheckWarn,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := newTestDoctor(t, fakeLooker{found: tc.lookFound},
				fakeRunner{outputs: tc.runOutputs, errors: tc.runErrors})
			r := d.checkGo()
			if r.Status != tc.wantStatus {
				t.Errorf("checkGo() status = %v, want %v; message: %s", r.Status, tc.wantStatus, r.Message)
			}
		})
	}
}

func TestCheckNode(t *testing.T) {
	tests := []struct {
		name       string
		lookFound  map[string]string
		runOutputs map[string][]byte
		wantStatus CheckStatus
		wantDetail string
	}{
		{
			name:       "Node20_pass",
			lookFound:  map[string]string{"node": "/usr/bin/node"},
			runOutputs: map[string][]byte{"node --version": []byte("v20.11.0\n")},
			wantStatus: CheckPass,
			wantDetail: "v20.11.0",
		},
		{
			name:       "Node18_warn",
			lookFound:  map[string]string{"node": "/usr/bin/node"},
			runOutputs: map[string][]byte{"node --version": []byte("v18.20.0\n")},
			wantStatus: CheckWarn,
		},
		{
			name:       "Node_not_found",
			lookFound:  map[string]string{},
			wantStatus: CheckWarn,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := newTestDoctor(t, fakeLooker{found: tc.lookFound},
				fakeRunner{outputs: tc.runOutputs})
			r := d.checkNode()
			if r.Status != tc.wantStatus {
				t.Errorf("checkNode() status = %v, want %v; message: %s", r.Status, tc.wantStatus, r.Message)
			}
			if tc.wantDetail != "" && r.Detail != tc.wantDetail {
				t.Errorf("checkNode() detail = %q, want %q", r.Detail, tc.wantDetail)
			}
		})
	}
}

func TestEnvironmentCategory(t *testing.T) {
	looker := fakeLooker{found: map[string]string{
		"go":   "/usr/local/go/bin/go",
		"npx":  "/usr/bin/npx",
		"git":  "/usr/bin/git",
		"node": "/usr/bin/node",
	}}
	runner := fakeRunner{outputs: map[string][]byte{
		"go version":     []byte("go version go1.22.4 linux/amd64"),
		"npx --version":  []byte("10.8.2"),
		"git --version":  []byte("git version 2.44.0"),
		"node --version": []byte("v20.11.0"),
	}}
	d := newTestDoctor(t, looker, runner)
	// Stub the review-screen hook for this test so checkReviewScreen passes
	// deterministically; real builds set it via app.init.
	prev := ReviewScreenWiredHook
	ReviewScreenWiredHook = func() bool { return true }
	t.Cleanup(func() { ReviewScreenWiredHook = prev })

	results := d.runEnvironment(context.Background())
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Status != CheckPass {
			t.Errorf("expected all pass, got %v for %s: %s", r.Status, r.Name, r.Message)
		}
	}
}

// --- MCP checks ---

func TestCheckMCPServer(t *testing.T) {
	tests := []struct {
		name       string
		server     domain.CuratedMCPServer
		envVars    map[string]string
		lookFound  map[string]string
		runOutputs map[string][]byte
		wantStatus CheckStatus
	}{
		{
			name: "MCP_ready_no_auth",
			server: domain.CuratedMCPServer{
				Name:    "context7",
				Type:    "local",
				Command: "npx",
			},
			lookFound:  map[string]string{"npx": "/usr/bin/npx"},
			wantStatus: CheckPass,
		},
		{
			name: "MCP_missing_required_env_var_fails",
			server: domain.CuratedMCPServer{
				Name:            "github",
				Type:            "local",
				Command:         "npx",
				RequiresAuth:    true,
				RequiredEnvVars: []string{"GITHUB_PERSONAL_ACCESS_TOKEN"},
			},
			lookFound:  map[string]string{"npx": "/usr/bin/npx"},
			wantStatus: CheckFail,
		},
		{
			name: "MCP_env_var_set_passes",
			server: domain.CuratedMCPServer{
				Name:            "github",
				Type:            "local",
				Command:         "npx",
				RequiresAuth:    true,
				RequiredEnvVars: []string{"GITHUB_PERSONAL_ACCESS_TOKEN"},
			},
			envVars:    map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": "ghp_test"},
			lookFound:  map[string]string{"npx": "/usr/bin/npx"},
			wantStatus: CheckPass,
		},
		{
			name: "MCP_binary_not_found",
			server: domain.CuratedMCPServer{
				Name:    "memory",
				Type:    "local",
				Command: "npx",
			},
			lookFound:  map[string]string{},
			wantStatus: CheckFail,
		},
		{
			name: "MCP_min_node_version_warn",
			server: domain.CuratedMCPServer{
				Name:           "context7",
				Type:           "local",
				Command:        "npx",
				MinNodeVersion: "20",
			},
			lookFound:  map[string]string{"npx": "/usr/bin/npx"},
			runOutputs: map[string][]byte{"node --version": []byte("v18.20.0\n")},
			wantStatus: CheckWarn,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set env vars for this sub-test.
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			d := newTestDoctor(t, fakeLooker{found: tc.lookFound},
				fakeRunner{outputs: tc.runOutputs})
			r := d.checkMCPServer(tc.server)
			if r.Status != tc.wantStatus {
				t.Errorf("checkMCPServer(%q) status = %v, want %v; message: %s",
					tc.name, r.Status, tc.wantStatus, r.Message)
			}
		})
	}
}

// --- Filesystem checks ---

func TestCheckWriteAccess(t *testing.T) {
	t.Run("writable_dir", func(t *testing.T) {
		dir := t.TempDir()
		d := newTestDoctor(t, fakeLooker{}, fakeRunner{})
		d.projectDir = dir
		r := d.checkWriteAccess(dir, "test dir")
		if r.Status != CheckPass {
			t.Errorf("expected pass, got %v: %s", r.Status, r.Message)
		}
	})

	t.Run("nonexistent_dir", func(t *testing.T) {
		d := newTestDoctor(t, fakeLooker{}, fakeRunner{})
		r := d.checkWriteAccess("/nonexistent/path/abcdef", "ghost dir")
		if r.Status != CheckWarn && r.Status != CheckFail {
			t.Errorf("expected warn/fail, got %v: %s", r.Status, r.Message)
		}
	})
}

// --- Config Drift checks ---

func TestCheckDrift_NoSidecar(t *testing.T) {
	dir := t.TempDir()
	d := NewWithDeps(dir, dir, nil, nil, fakeLooker{}, fakeRunner{})
	results := d.runConfigDrift(context.Background())
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != CheckSkip {
		t.Errorf("expected skip for no sidecar, got %v", results[0].Status)
	}
}

func TestCheckDrift_MarkerIntact(t *testing.T) {
	dir := t.TempDir()

	// Create a managed file with intact markers.
	content := marker.OpenTag("rules") + "\nsome managed content\n" + marker.CloseTag("rules") + "\n"
	filePath := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	// Register in sidecar.
	if err := managed.WriteManagedKeys(dir, "AGENTS.md", []string{"rules"}); err != nil {
		t.Fatal(err)
	}

	d := NewWithDeps(dir, dir, nil, nil, fakeLooker{}, fakeRunner{})
	r := d.checkDriftFile("AGENTS.md")
	if r.Status != CheckPass {
		t.Errorf("expected pass for intact markers, got %v: %s", r.Status, r.Message)
	}
}

func TestCheckDrift_MarkersMissing(t *testing.T) {
	dir := t.TempDir()

	// Create file WITHOUT markers (manually edited).
	filePath := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(filePath, []byte("manually edited content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := managed.WriteManagedKeys(dir, "AGENTS.md", []string{"rules"}); err != nil {
		t.Fatal(err)
	}

	d := NewWithDeps(dir, dir, nil, nil, fakeLooker{}, fakeRunner{})
	r := d.checkDriftFile("AGENTS.md")
	if r.Status != CheckFail {
		t.Errorf("expected fail for missing markers, got %v: %s", r.Status, r.Message)
	}
}

func TestCheckDrift_FileDeleted(t *testing.T) {
	dir := t.TempDir()

	// Register a file but don't create it.
	if err := managed.WriteManagedKeys(dir, "AGENTS.md", []string{"rules"}); err != nil {
		t.Fatal(err)
	}

	d := NewWithDeps(dir, dir, nil, nil, fakeLooker{}, fakeRunner{})
	r := d.checkDriftFile("AGENTS.md")
	if r.Status != CheckFail {
		t.Errorf("expected fail for deleted file, got %v: %s", r.Status, r.Message)
	}
}

// --- Filter / Run integration ---

func TestRun_CategoryFilter(t *testing.T) {
	dir := t.TempDir()
	looker := fakeLooker{found: map[string]string{"go": "/usr/bin/go"}}
	runner := fakeRunner{outputs: map[string][]byte{
		"go version": []byte("go version go1.22.4 linux/amd64"),
	}}
	d := NewWithDeps(dir, dir, nil, nil, looker, runner)

	results, err := d.Run(context.Background(), Options{Category: "environment"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range results {
		if r.Category != "Environment" {
			t.Errorf("expected only Environment results, got category %q", r.Category)
		}
	}
}

func TestRun_InvalidCategory(t *testing.T) {
	dir := t.TempDir()
	d := NewWithDeps(dir, dir, nil, nil, fakeLooker{}, fakeRunner{})
	_, err := d.Run(context.Background(), Options{Category: "invalid"})
	if err == nil {
		t.Error("expected error for invalid category, got nil")
	}
}

func TestRun_CheckFilter(t *testing.T) {
	dir := t.TempDir()
	looker := fakeLooker{found: map[string]string{"go": "/usr/bin/go"}}
	runner := fakeRunner{outputs: map[string][]byte{
		"go version": []byte("go version go1.22.4 linux/amd64"),
	}}
	d := NewWithDeps(dir, dir, nil, nil, looker, runner)

	results, err := d.Run(context.Background(), Options{Check: "environment.Go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "Go" {
		t.Errorf("expected Go check, got %q", results[0].Name)
	}
}

func TestRun_InvalidCheckFormat(t *testing.T) {
	dir := t.TempDir()
	d := NewWithDeps(dir, dir, nil, nil, fakeLooker{}, fakeRunner{})
	_, err := d.Run(context.Background(), Options{Check: "noformat"})
	if err == nil {
		t.Error("expected error for malformed --check, got nil")
	}
}

// --- Render tests ---

func TestRenderJSON(t *testing.T) {
	results := []CheckResult{
		{Category: "Environment", Name: "Go", Status: CheckPass, Message: "go found", Detail: "1.22.4"},
		{Category: "MCP Servers", Name: "github", Status: CheckFail, Message: "token missing", FixHint: "set env var"},
	}
	var buf bytes.Buffer
	if err := RenderJSON(&buf, results, "1.0.0"); err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"version"`) {
		t.Error("JSON output missing 'version' field")
	}
	if !strings.Contains(out, `"timestamp"`) {
		t.Error("JSON output missing 'timestamp' field")
	}
	if !strings.Contains(out, `"summary"`) {
		t.Error("JSON output missing 'summary' field")
	}
	if !strings.Contains(out, `"checks"`) {
		t.Error("JSON output missing 'checks' field")
	}
	if !strings.Contains(out, `"pass"`) {
		t.Error("JSON output missing pass status")
	}
	if !strings.Contains(out, `"fail"`) {
		t.Error("JSON output missing fail status")
	}
}

func TestRenderHuman(t *testing.T) {
	results := []CheckResult{
		{Category: "Environment", Name: "Go", Status: CheckPass, Message: "go found", Detail: "1.22.4"},
		{Category: "Environment", Name: "npx", Status: CheckWarn, Message: "npx missing"},
	}
	var buf bytes.Buffer
	RenderHuman(&buf, results, "1.0.0", false)
	out := buf.String()
	if !strings.Contains(out, "SquadAI Doctor") {
		t.Error("human output missing header")
	}
	if !strings.Contains(out, "Environment") {
		t.Error("human output missing category header")
	}
	if !strings.Contains(out, "Summary") {
		t.Error("human output missing summary line")
	}
}

// --- Determinism test ---

func TestRun_ResultsAreSorted(t *testing.T) {
	dir := t.TempDir()
	looker := fakeLooker{found: map[string]string{
		"go":   "/usr/bin/go",
		"npx":  "/usr/bin/npx",
		"node": "/usr/bin/node",
		"git":  "/usr/bin/git",
	}}
	runner := fakeRunner{outputs: map[string][]byte{
		"go version":     []byte("go version go1.22.4"),
		"npx --version":  []byte("10.8.2"),
		"git --version":  []byte("git version 2.44.0"),
		"node --version": []byte("v20.0.0"),
	}}
	d := NewWithDeps(dir, dir, nil, nil, looker, runner)

	r1, err := d.Run(context.Background(), Options{Category: "environment"})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := d.Run(context.Background(), Options{Category: "environment"})
	if err != nil {
		t.Fatal(err)
	}

	if len(r1) != len(r2) {
		t.Fatalf("inconsistent lengths: %d vs %d", len(r1), len(r2))
	}
	for i := range r1 {
		if r1[i].Name != r2[i].Name {
			t.Errorf("result[%d] differs: %q vs %q", i, r1[i].Name, r2[i].Name)
		}
	}
}
