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
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	var buf bytes.Buffer
	// With no config, the planner returns 0 actions and we print a no-install note.
	err := RunTokenBudget([]string{}, &buf)
	if err != nil {
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
}

func TestRunTokenBudget_JSON_EmptyInstall(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	var buf bytes.Buffer
	err := RunTokenBudget([]string{"--json"}, &buf)
	if err != nil {
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
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	var buf bytes.Buffer
	err := RunTokenBudget([]string{}, &buf)
	if err != nil {
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
