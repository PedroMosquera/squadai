package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// writeConsoleProject writes a minimal project.json with the given MCP
// server map under projectDir.
func writeConsoleProject(t *testing.T, projectDir string, mcp map[string]domain.MCPServerDef) {
	t.Helper()
	proj := domain.DefaultProjectConfig()
	proj.MCP = mcp
	dir := filepath.Join(projectDir, config.ProjectConfigDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteJSON(filepath.Join(dir, "project.json"), proj); err != nil {
		t.Fatal(err)
	}
}

// Projects created before the SquadAI MCP server became a default configure
// other servers but not squadai's own: doctor must nudge (warn-only) toward
// the non-destructive `init --merge` upgrade path.
func TestCheckConsoleRegistered(t *testing.T) {
	tests := []struct {
		name       string
		mcp        map[string]domain.MCPServerDef
		noProject  bool
		wantStatus CheckStatus
	}{
		{
			name:       "pre-console project warns",
			mcp:        map[string]domain.MCPServerDef{"context7": {Type: "local", Command: []string{"npx"}, Enabled: true}},
			wantStatus: CheckWarn,
		},
		{
			name: "registered console passes",
			mcp: map[string]domain.MCPServerDef{
				"context7": {Type: "local", Command: []string{"npx"}, Enabled: true},
				"squadai":  {Type: "local", Command: []string{"squadai", "mcp-server"}, Enabled: true},
			},
			wantStatus: CheckPass,
		},
		{
			name:       "no MCP servers is skipped",
			mcp:        nil,
			wantStatus: CheckSkip,
		},
		{
			name:       "no project config is skipped",
			noProject:  true,
			wantStatus: CheckSkip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()
			if !tt.noProject {
				writeConsoleProject(t, projectDir, tt.mcp)
			}
			d := New(t.TempDir(), projectDir, nil, nil)
			got := d.checkConsoleRegistered()
			if got.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q (message: %s)", got.Status, tt.wantStatus, got.Message)
			}
			if tt.wantStatus == CheckWarn && got.FixHint == "" {
				t.Error("warn result should carry the init --merge fix hint")
			}
		})
	}
}
