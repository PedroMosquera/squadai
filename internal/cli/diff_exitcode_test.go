package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
)

func writeDiffTestProject(t *testing.T, components map[string]domain.ComponentConfig) (homeDir, projectDir string) {
	t.Helper()
	dir := t.TempDir()
	homeDir = filepath.Join(dir, "home")
	projectDir = filepath.Join(dir, "project")
	proj := &domain.ProjectConfig{
		Version:    1,
		Adapters:   map[string]domain.AdapterConfig{"opencode": {Enabled: true}},
		Components: components,
	}
	projectPath := filepath.Join(projectDir, config.ProjectConfigDir, "project.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := config.WriteJSON(projectPath, proj); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	return homeDir, projectDir
}

func TestRunDiff_ExitCode_FailsWithDriftWhenApplyWouldChangeFiles(t *testing.T) {
	for _, args := range [][]string{{"--exit-code"}, {"--exit-code", "--json"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			homeDir, projectDir := writeDiffTestProject(t, map[string]domain.ComponentConfig{
				"memory": {Enabled: true},
			})

			var buf bytes.Buffer
			err := runDiff(args, &buf, homeDir, projectDir)

			var appErr *exitcode.AppError
			if !errors.As(err, &appErr) || appErr.Code != exitcode.Drift {
				t.Fatalf("want AppError with exit code %d (drift), got %v", exitcode.Drift, err)
			}
			if !strings.Contains(buf.String(), "AGENTS.md") {
				t.Errorf("diff output should still be printed before failing, got:\n%s", buf.String())
			}
		})
	}
}

func TestRunDiff_ExitCode_PassesWhenNothingToChange(t *testing.T) {
	homeDir, projectDir := writeDiffTestProject(t, map[string]domain.ComponentConfig{
		"efficiency": {Enabled: false},
	})

	var buf bytes.Buffer
	if err := runDiff([]string{"--exit-code"}, &buf, homeDir, projectDir); err != nil {
		t.Fatalf("want nil error when nothing would change, got %v", err)
	}
}

func TestRunDiff_WithoutExitCode_PendingChangesDoNotFail(t *testing.T) {
	homeDir, projectDir := writeDiffTestProject(t, map[string]domain.ComponentConfig{
		"memory": {Enabled: true},
	})

	var buf bytes.Buffer
	if err := runDiff(nil, &buf, homeDir, projectDir); err != nil {
		t.Fatalf("plain diff is a preview and must exit 0, got %v", err)
	}
}
