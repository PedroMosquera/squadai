package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/tokenprofile"
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

	// Make Pi detectable (personal-lane adapters are detection-gated) so the
	// shared AGENTS.md actually carries both OpenCode and Pi sections.
	if err := os.MkdirAll(filepath.Join(home, ".pi", "agent"), 0o755); err != nil {
		t.Fatalf("create pi config dir: %v", err)
	}

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

const approxFootnote = "~ = character-based approximation; no exact tokenizer available for this model"

func TestPrintTokenBudgetHuman_ApproximateMarker(t *testing.T) {
	r := &tokenprofile.Report{
		ByCategory: map[string]tokenprofile.CategorySummary{
			"agents": {Files: 2, Bytes: 800, Tokens: 123},
		},
		TotalBytes:  800,
		TotalTokens: 123,
		Approximate: true,
	}
	var buf bytes.Buffer
	printTokenBudgetHuman(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "~123") {
		t.Errorf("approximate counts should carry a '~' prefix:\n%s", out)
	}
	if !strings.Contains(out, approxFootnote) {
		t.Errorf("approximate output should include the footnote:\n%s", out)
	}
}

func TestPrintTokenBudgetHuman_ExactCounts_NoMarker(t *testing.T) {
	r := &tokenprofile.Report{
		ByCategory: map[string]tokenprofile.CategorySummary{
			"agents": {Files: 2, Bytes: 800, Tokens: 123},
		},
		TotalBytes:  800,
		TotalTokens: 123,
		Model:       "gpt-4o",
		Approximate: false,
	}
	var buf bytes.Buffer
	printTokenBudgetHuman(&buf, r)
	out := buf.String()

	if strings.Contains(out, "~") {
		t.Errorf("exact counts must not carry a '~' prefix:\n%s", out)
	}
	if strings.Contains(out, approxFootnote) {
		t.Errorf("exact output must not include the approximation footnote:\n%s", out)
	}
}

// A Claude model has no exact tokenizer, so the approximate flag must
// propagate end-to-end into both human and JSON output.
func TestRunTokenBudget_ClaudeModel_MarkedApproximate(t *testing.T) {
	dir := t.TempDir()
	writeTDDProjectForBudget(t, dir)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(dir)

	var human bytes.Buffer
	if err := RunTokenBudget([]string{"--model=claude-sonnet-4-6"}, &human); err != nil {
		t.Fatalf("RunTokenBudget: %v", err)
	}
	if !strings.Contains(human.String(), approxFootnote) {
		t.Errorf("human output for a Claude model should include the approximation footnote:\n%s", human.String())
	}

	var raw map[string]interface{}
	var jsonBuf bytes.Buffer
	if err := RunTokenBudget([]string{"--model=claude-sonnet-4-6", "--json"}, &jsonBuf); err != nil {
		t.Fatalf("RunTokenBudget --json: %v", err)
	}
	if err := json.Unmarshal(jsonBuf.Bytes(), &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, jsonBuf.String())
	}
	approx, ok := raw["approximate"].(bool)
	if !ok || !approx {
		t.Errorf("JSON output should carry \"approximate\": true for a Claude model, got %v", raw["approximate"])
	}
}
