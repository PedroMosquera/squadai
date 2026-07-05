package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// writeUsageProject writes a minimal project.json with the given usage
// config under projectDir.
func writeUsageProject(t *testing.T, projectDir string, usage domain.UsageConfig) {
	t.Helper()
	proj := domain.DefaultProjectConfig()
	proj.Usage = usage
	dir := filepath.Join(projectDir, config.ProjectConfigDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteJSON(filepath.Join(dir, "project.json"), proj); err != nil {
		t.Fatal(err)
	}
}

// writeRecentSession writes an OpenCode session file with the given token
// counts under the temp home, so the 24h aggregation window picks it up.
func writeRecentSession(t *testing.T, home string, in, out int) {
	t.Helper()
	dir := filepath.Join(home, ".local/share/opencode/sessions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf(`{"model":"claude-sonnet-4-6","usage":{"input_tokens":%d,"output_tokens":%d}}`, in, out)
	if err := os.WriteFile(filepath.Join(dir, "s1.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckTokenBudgetUsage_NoProjectConfig(t *testing.T) {
	d := &Doctor{homeDir: t.TempDir(), projectDir: t.TempDir()}
	got := d.checkTokenBudgetUsage()
	if got.Status != CheckSkip {
		t.Errorf("Status = %v, want CheckSkip", got.Status)
	}
}

func TestCheckTokenBudgetUsage_EnforcementOff(t *testing.T) {
	projectDir := t.TempDir()
	writeUsageProject(t, projectDir, domain.UsageConfig{
		DailyTokenBudget: 100,
		Enforcement:      "off",
	})
	d := &Doctor{homeDir: t.TempDir(), projectDir: projectDir}
	got := d.checkTokenBudgetUsage()
	if got.Status != CheckSkip {
		t.Errorf("Status = %v, want CheckSkip when enforcement is off", got.Status)
	}
}

func TestCheckTokenBudgetUsage_NoBudgetConfigured(t *testing.T) {
	projectDir := t.TempDir()
	writeUsageProject(t, projectDir, domain.UsageConfig{Enforcement: "warn"})
	d := &Doctor{homeDir: t.TempDir(), projectDir: projectDir}
	got := d.checkTokenBudgetUsage()
	if got.Status != CheckSkip {
		t.Errorf("Status = %v, want CheckSkip without a daily budget", got.Status)
	}
}

func TestCheckTokenBudgetUsage_UnderBudget(t *testing.T) {
	home := t.TempDir()
	projectDir := t.TempDir()
	writeUsageProject(t, projectDir, domain.UsageConfig{
		DailyTokenBudget: 100000,
		Enforcement:      "warn",
	})
	writeRecentSession(t, home, 100, 50)
	d := &Doctor{homeDir: home, projectDir: projectDir}
	got := d.checkTokenBudgetUsage()
	if got.Status != CheckPass {
		t.Errorf("Status = %v, want CheckPass, msg=%q", got.Status, got.Message)
	}
}

func TestCheckTokenBudgetUsage_OverBudget(t *testing.T) {
	home := t.TempDir()
	projectDir := t.TempDir()
	writeUsageProject(t, projectDir, domain.UsageConfig{
		DailyTokenBudget: 100,
		Enforcement:      "warn",
	})
	writeRecentSession(t, home, 900, 500)
	d := &Doctor{homeDir: home, projectDir: projectDir}
	got := d.checkTokenBudgetUsage()
	if got.Status != CheckWarn {
		t.Fatalf("Status = %v, want CheckWarn, msg=%q", got.Status, got.Message)
	}
	if !strings.Contains(got.FixHint, "--profile=cheap") {
		t.Errorf("FixHint = %q, want it to suggest --profile=cheap", got.FixHint)
	}
}
