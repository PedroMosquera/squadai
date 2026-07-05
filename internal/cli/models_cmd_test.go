package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/modelcatalog"
)

// setupModelsEnv points HOME and the working directory at temp dirs so the
// commands never see the developer's real overrides.
func setupModelsEnv(t *testing.T) (home, project string) {
	t.Helper()
	home = t.TempDir()
	project = t.TempDir()
	t.Setenv("HOME", home)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	return home, project
}

// serveCatalog runs an httptest server returning body and points
// modelcatalog.RemoteURL at it.
func serveCatalog(t *testing.T, body string, status int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			http.Error(w, "error", status)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	prev := modelcatalog.RemoteURL
	modelcatalog.RemoteURL = srv.URL
	t.Cleanup(func() { modelcatalog.RemoteURL = prev })
}

const remoteCatalogBody = `{
	"schema_version": 1,
	"updated": "2027-06-01",
	"models": {
		"claude-fable-5": {"provider": "anthropic", "input_per_mtok": 8, "output_per_mtok": 40, "context_window": 1000000, "encoding": "o200k_base"},
		"brand-new-model": {"provider": "acme", "input_per_mtok": 1, "output_per_mtok": 2}
	}
}`

// ─── models list ──────────────────────────────────────────────────────────────

func TestRunModelsList_Human(t *testing.T) {
	setupModelsEnv(t)
	var buf bytes.Buffer
	if err := RunModelsList(nil, &buf); err != nil {
		t.Fatalf("RunModelsList: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"claude-fable-5", "claude-sonnet-4-6", "embedded", "legacy", "MODEL"} {
		if !strings.Contains(out, want) {
			t.Errorf("models list output missing %q:\n%s", want, out)
		}
	}
}

func TestRunModelsList_JSONWithSourceColumn(t *testing.T) {
	home, _ := setupModelsEnv(t)
	// A user override adds a model; the source column must attribute it.
	if err := os.MkdirAll(filepath.Join(home, ".squadai"), 0755); err != nil {
		t.Fatal(err)
	}
	override := `{"schema_version": 1, "models": {"my-model": {"provider": "acme", "input_per_mtok": 1, "output_per_mtok": 2}}}`
	if err := os.WriteFile(filepath.Join(home, ".squadai", "models.json"), []byte(override), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := RunModelsList([]string{"--json"}, &buf); err != nil {
		t.Fatalf("RunModelsList --json: %v", err)
	}
	var out modelsListOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	sources := map[string]string{}
	for _, r := range out.Models {
		sources[r.ID] = r.Source
	}
	if sources["my-model"] != "user" {
		t.Errorf("my-model source = %q, want user", sources["my-model"])
	}
	if sources["claude-fable-5"] != "embedded" {
		t.Errorf("claude-fable-5 source = %q, want embedded", sources["claude-fable-5"])
	}
}

func TestRunModelsList_AdapterFilter(t *testing.T) {
	setupModelsEnv(t)
	var buf bytes.Buffer
	if err := RunModelsList([]string{"--adapter=claude-code", "--json"}, &buf); err != nil {
		t.Fatalf("RunModelsList --adapter: %v", err)
	}
	var out modelsListOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Tiers["standard"] != "claude-sonnet-4-6" {
		t.Errorf("claude-code standard tier = %q", out.Tiers["standard"])
	}
	for _, r := range out.Models {
		if r.Tier == "" {
			t.Errorf("adapter-filtered row %s has no tier", r.ID)
		}
	}
}

func TestRunModelsList_UnknownAdapterAndFlag(t *testing.T) {
	setupModelsEnv(t)
	var buf bytes.Buffer
	if err := RunModelsList([]string{"--adapter=nope"}, &buf); err == nil {
		t.Error("unknown adapter should error")
	}
	if err := RunModelsList([]string{"--bogus"}, &buf); err == nil {
		t.Error("unknown flag should error")
	}
}

// ─── models check ─────────────────────────────────────────────────────────────

func TestRunModelsCheck_ReportsDeltas(t *testing.T) {
	setupModelsEnv(t)
	serveCatalog(t, remoteCatalogBody, http.StatusOK)

	var buf bytes.Buffer
	if err := RunModelsCheck(nil, &buf); err != nil {
		t.Fatalf("RunModelsCheck: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "brand-new-model") {
		t.Errorf("check output missing added model:\n%s", out)
	}
	if !strings.Contains(out, "claude-fable-5") {
		t.Errorf("check output missing changed model:\n%s", out)
	}
	if !strings.Contains(out, "models update") {
		t.Errorf("check output missing update suggestion:\n%s", out)
	}
}

func TestRunModelsCheck_JSON(t *testing.T) {
	setupModelsEnv(t)
	serveCatalog(t, remoteCatalogBody, http.StatusOK)

	var buf bytes.Buffer
	if err := RunModelsCheck([]string{"--json"}, &buf); err != nil {
		t.Fatalf("RunModelsCheck --json: %v", err)
	}
	var d catalogDelta
	if err := json.Unmarshal(buf.Bytes(), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !d.Stale {
		t.Error("remote 2027-06-01 vs embedded should be stale")
	}
	if len(d.Added) != 1 || !strings.Contains(d.Added[0], "brand-new-model") {
		t.Errorf("Added = %v", d.Added)
	}
	if len(d.Changed) != 1 || !strings.Contains(d.Changed[0], "claude-fable-5") {
		t.Errorf("Changed = %v", d.Changed)
	}
	if len(d.LocalOnly) == 0 {
		t.Error("LocalOnly should list embedded models absent from remote")
	}
}

func TestRunModelsCheck_ServerError(t *testing.T) {
	setupModelsEnv(t)
	serveCatalog(t, "", http.StatusInternalServerError)
	var buf bytes.Buffer
	if err := RunModelsCheck(nil, &buf); err == nil {
		t.Error("check against a 500 server should error")
	}
}

// ─── models update ────────────────────────────────────────────────────────────

func TestRunModelsUpdate_ConfirmYesWritesUserOverride(t *testing.T) {
	home, _ := setupModelsEnv(t)
	serveCatalog(t, remoteCatalogBody, http.StatusOK)

	var buf bytes.Buffer
	stdin := strings.NewReader("y\n")
	if err := RunModelsUpdate(nil, &buf, stdin); err != nil {
		t.Fatalf("RunModelsUpdate: %v", err)
	}
	target := filepath.Join(home, ".squadai", "models.json")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("override not written: %v", err)
	}
	if string(data) != remoteCatalogBody {
		t.Error("override should be the exact remote document")
	}
	if !strings.Contains(buf.String(), "Proceed? [y/N]") {
		t.Errorf("update should prompt for confirmation:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), target) {
		t.Errorf("update output should mention target path:\n%s", buf.String())
	}
}

func TestRunModelsUpdate_DeclineWritesNothing(t *testing.T) {
	home, _ := setupModelsEnv(t)
	serveCatalog(t, remoteCatalogBody, http.StatusOK)

	var buf bytes.Buffer
	for _, answer := range []string{"n\n", "\n", "whatever\n"} {
		if err := RunModelsUpdate(nil, &buf, strings.NewReader(answer)); err != nil {
			t.Fatalf("RunModelsUpdate(%q): %v", answer, err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".squadai", "models.json")); !os.IsNotExist(err) {
		t.Error("declining the prompt must not write the override")
	}
	if !strings.Contains(buf.String(), "Aborted") {
		t.Errorf("decline should report abort:\n%s", buf.String())
	}
}

func TestRunModelsUpdate_YesFlagSkipsPrompt(t *testing.T) {
	home, _ := setupModelsEnv(t)
	serveCatalog(t, remoteCatalogBody, http.StatusOK)

	var buf bytes.Buffer
	// stdin would block/refuse: prove it is not read.
	if err := RunModelsUpdate([]string{"--yes"}, &buf, strings.NewReader("n\n")); err != nil {
		t.Fatalf("RunModelsUpdate --yes: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".squadai", "models.json")); err != nil {
		t.Errorf("--yes should write without prompting: %v", err)
	}
	if strings.Contains(buf.String(), "Proceed?") {
		t.Error("--yes must not prompt")
	}
}

func TestRunModelsUpdate_ProjectFlagWritesProjectOverride(t *testing.T) {
	home, project := setupModelsEnv(t)
	serveCatalog(t, remoteCatalogBody, http.StatusOK)

	var buf bytes.Buffer
	if err := RunModelsUpdate([]string{"--yes", "--project"}, &buf, strings.NewReader("")); err != nil {
		t.Fatalf("RunModelsUpdate --project: %v", err)
	}
	projTarget, err := filepath.EvalSymlinks(filepath.Join(project, ".squadai", "models.json"))
	if err != nil {
		t.Errorf("--project should write project override: %v", err)
	} else if projTarget == "" {
		t.Error("empty project target")
	}
	if _, err := os.Stat(filepath.Join(home, ".squadai", "models.json")); !os.IsNotExist(err) {
		t.Error("--project must not write the user override")
	}
}

func TestRunModelsUpdate_UpToDateWritesNothing(t *testing.T) {
	home, _ := setupModelsEnv(t)
	// Serve the exact embedded catalog: no deltas, not stale.
	embedded, err := assets.FS.ReadFile("models/models.json")
	if err != nil {
		t.Fatal(err)
	}
	serveCatalog(t, string(embedded), http.StatusOK)

	var buf bytes.Buffer
	if err := RunModelsUpdate([]string{"--yes"}, &buf, strings.NewReader("")); err != nil {
		t.Fatalf("RunModelsUpdate: %v", err)
	}
	if !strings.Contains(buf.String(), "up to date") {
		t.Errorf("expected up-to-date message:\n%s", buf.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".squadai", "models.json")); !os.IsNotExist(err) {
		t.Error("up-to-date update must not write anything")
	}
}

func TestRunModelsUpdate_InvalidRemoteRejected(t *testing.T) {
	home, _ := setupModelsEnv(t)
	serveCatalog(t, `{"schema_version": 42}`, http.StatusOK)

	var buf bytes.Buffer
	if err := RunModelsUpdate([]string{"--yes"}, &buf, strings.NewReader("")); err == nil {
		t.Error("invalid remote catalog must be rejected")
	}
	if _, err := os.Stat(filepath.Join(home, ".squadai", "models.json")); !os.IsNotExist(err) {
		t.Error("invalid remote must not be written")
	}
}
