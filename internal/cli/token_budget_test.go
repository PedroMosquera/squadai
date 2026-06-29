package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// writeTDDProjectForBudget writes a minimal .squadai/project.json to dir.
func writeTDDProjectForBudget(t *testing.T, dir string) {
	t.Helper()
	proj := domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Methodology: domain.MethodologyTDD,
		Team:        domain.DefaultTeam(domain.MethodologyTDD),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(dir), &proj); err != nil {
		t.Fatalf("write project.json: %v", err)
	}
}

func TestRunTokenBudget_NoConfig_NoError(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	var buf bytes.Buffer
	// With no config, the planner returns 0 actions and we print a no-install note.
	if err := RunTokenBudget([]string{}, &buf); err != nil {
		t.Errorf("unexpected error with no config: %v", err)
	}
}

func TestRunTokenBudget_Help(t *testing.T) {
	var buf bytes.Buffer
	err := RunTokenBudget([]string{"--help"}, &buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "token-budget") {
		t.Error("help output should contain 'token-budget'")
	}
	if !strings.Contains(buf.String(), "--planned") {
		t.Error("help output should contain '--planned'")
	}
}

func TestRunTokenBudget_JSON_EmptyInstall(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	var buf bytes.Buffer
	if err := RunTokenBudget([]string{"--json"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}
	if _, ok := out["total_tokens"]; !ok {
		t.Error("JSON output should contain 'total_tokens'")
	}
}

func TestRunTokenBudget_HumanOutput_ContainsExpectedFields(t *testing.T) {
	dir := t.TempDir()
	writeTDDProjectForBudget(t, dir)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	var buf bytes.Buffer
	if err := RunTokenBudget([]string{}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	// Should contain a header and TOTAL line.
	if !strings.Contains(out, "Token Budget") {
		t.Error("output should contain 'Token Budget' header")
	}
	if !strings.Contains(out, "TOTAL") {
		t.Error("output should contain 'TOTAL' row")
	}
}

func TestRunTokenBudget_PlannedBeforeApply_EstimatesRenderedContent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Chdir(dir)

	var initOut bytes.Buffer
	if err := RunInit([]string{"--preset=solo-power", "--agents=opencode", "--json"}, &initOut); err != nil {
		t.Fatalf("RunInit: %v\n%s", err, initOut.String())
	}

	var buf bytes.Buffer
	if err := RunTokenBudget([]string{"--planned", "--json"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}
	if tokens, _ := raw["total_tokens"].(float64); tokens <= 0 {
		t.Fatalf("planned budget should estimate non-zero tokens, got: %s", buf.String())
	}
	if missing, _ := raw["missing_files"].(float64); missing != 0 {
		t.Fatalf("planned budget should not report missing installed files, got: %s", buf.String())
	}
}

// TestRunTokenBudget_PlannedDoesNotMultiCountSharedFiles guards against the bug
// where OpenCode and Pi both target a shared AGENTS.md: each rendered action
// returns the full document, so a naive per-action sum counts the same file
// multiple times. After apply, the planned total must equal the installed scan
// total (which is path-deduped) — not a multiple of it.
func TestRunTokenBudget_PlannedDoesNotMultiCountSharedFiles(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(project)

	var initOut bytes.Buffer
	if err := RunInit([]string{"--preset=solo-power", "--agents=opencode,pi", "--json"}, &initOut); err != nil {
		t.Fatalf("RunInit: %v\n%s", err, initOut.String())
	}
	var applyOut bytes.Buffer
	if err := RunApply([]string{"--no-review", "--json"}, &applyOut); err != nil {
		t.Fatalf("RunApply: %v\n%s", err, applyOut.String())
	}

	totalTokens := func(args []string) float64 {
		t.Helper()
		var buf bytes.Buffer
		if err := RunTokenBudget(args, &buf); err != nil {
			t.Fatalf("RunTokenBudget %v: %v", args, err)
		}
		var raw map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
			t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, buf.String())
		}
		tok, _ := raw["total_tokens"].(float64)
		return tok
	}

	installed := totalTokens([]string{"--json"})
	planned := totalTokens([]string{"--planned", "--json"})

	if installed <= 0 {
		t.Fatalf("installed scan should report non-zero tokens, got %v", installed)
	}
	if planned != installed {
		t.Fatalf("planned total (%v) should equal installed scan total (%v) after apply; "+
			"a mismatch means shared files are being multi-counted", planned, installed)
	}
}
