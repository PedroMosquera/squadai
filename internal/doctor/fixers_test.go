package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/config"
)

// TestFixCreateUserConfig_CreatesFile verifies that fixCreateUserConfig writes
// ~/.squadai/config.json when it does not exist.
func TestFixCreateUserConfig_CreatesFile(t *testing.T) {
	tmp := t.TempDir()
	d := &Doctor{homeDir: tmp, projectDir: tmp}

	if err := fixCreateUserConfig(context.Background(), d); err != nil {
		t.Fatalf("fixCreateUserConfig: %v", err)
	}

	path := config.UserConfigPath(tmp)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("config file is empty")
	}
}

// TestFixCreateUserConfig_Idempotent verifies re-running the fixer does not error.
func TestFixCreateUserConfig_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	d := &Doctor{homeDir: tmp, projectDir: tmp}

	if err := fixCreateUserConfig(context.Background(), d); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := fixCreateUserConfig(context.Background(), d); err != nil {
		t.Fatalf("second run: %v", err)
	}
}

// TestFixCreateSquadAIDir_CreatesDir verifies that fixCreateSquadAIDir creates ~/.squadai.
func TestFixCreateSquadAIDir_CreatesDir(t *testing.T) {
	tmp := t.TempDir()
	d := &Doctor{homeDir: tmp, projectDir: tmp}

	if err := fixCreateSquadAIDir(context.Background(), d); err != nil {
		t.Fatalf("fixCreateSquadAIDir: %v", err)
	}

	dir := filepath.Join(tmp, ".squadai")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected a directory")
	}
}

// TestDoctor_Fix_NoFixableResults returns empty when no results are fixable.
func TestDoctor_Fix_NoFixableResults(t *testing.T) {
	tmp := t.TempDir()
	d := &Doctor{homeDir: tmp, projectDir: tmp}

	results := []CheckResult{
		{Category: "Environment", Name: "go", Status: CheckPass, AutoFixable: false},
		{Category: "Environment", Name: "node", Status: CheckFail, AutoFixable: false},
	}

	fixResults := d.Fix(context.Background(), results)
	if len(fixResults) != 0 {
		t.Fatalf("expected 0 fix results, got %d", len(fixResults))
	}
}

// TestDoctor_Fix_RunsRegisteredFixer verifies a registered fixer is called.
func TestDoctor_Fix_RunsRegisteredFixer(t *testing.T) {
	tmp := t.TempDir()
	d := &Doctor{homeDir: tmp, projectDir: tmp}

	results := []CheckResult{
		{
			Category:    "Project Configuration",
			Name:        "config.json",
			Status:      CheckFail,
			AutoFixable: true,
		},
	}

	fixResults := d.Fix(context.Background(), results)
	if len(fixResults) != 1 {
		t.Fatalf("expected 1 fix result, got %d", len(fixResults))
	}
	if fixResults[0].Err != nil {
		t.Fatalf("fixer returned error: %v", fixResults[0].Err)
	}

	// Verify the file was actually created.
	path := config.UserConfigPath(tmp)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created after fix: %v", err)
	}
}

// TestDoctor_Fix_UnregisteredKey returns error for unknown fixer keys.
func TestDoctor_Fix_UnregisteredKey(t *testing.T) {
	tmp := t.TempDir()
	d := &Doctor{homeDir: tmp, projectDir: tmp}

	results := []CheckResult{
		{
			Category:    "SomeCategory",
			Name:        "nonexistent",
			Status:      CheckFail,
			AutoFixable: true,
		},
	}

	fixResults := d.Fix(context.Background(), results)
	if len(fixResults) != 1 {
		t.Fatalf("expected 1 result, got %d", len(fixResults))
	}
	if fixResults[0].Err == nil {
		t.Fatal("expected error for unregistered fixer, got nil")
	}
}

// TestDoctor_Fix_WarningsNotFixed verifies that CheckWarn items are skipped.
func TestDoctor_Fix_WarningsNotFixed(t *testing.T) {
	tmp := t.TempDir()
	d := &Doctor{homeDir: tmp, projectDir: tmp}

	results := []CheckResult{
		{
			Category:    "Project Configuration",
			Name:        "config.json",
			Status:      CheckWarn, // warnings are not auto-fixed
			AutoFixable: true,
		},
	}

	fixResults := d.Fix(context.Background(), results)
	if len(fixResults) != 0 {
		t.Fatalf("expected 0 results for warnings, got %d", len(fixResults))
	}
}
