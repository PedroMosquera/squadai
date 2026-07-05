package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// setupInitDir points HOME and the working directory at fresh temp dirs so
// RunInit never touches the real environment.
func setupInitDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(project)
	return project
}

// ─── --no-memory-scaffold ────────────────────────────────────────────────────

func TestRunInit_NoMemoryScaffold_SkipsScaffoldKeepsMemory(t *testing.T) {
	project := setupInitDir(t)

	var buf bytes.Buffer
	if err := RunInit([]string{"--no-memory-scaffold"}, &buf); err != nil {
		t.Fatalf("RunInit: %v", err)
	}

	if _, err := os.Stat(filepath.Join(project, "docs", "memory")); !os.IsNotExist(err) {
		t.Error("docs/memory/ should not be scaffolded with --no-memory-scaffold")
	}

	// The memory component must stay enabled in project.json.
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project config: %v", err)
	}
	comp, ok := proj.Components[string(domain.ComponentMemory)]
	if !ok || !comp.Enabled {
		t.Error("memory component should stay enabled with --no-memory-scaffold")
	}
}

func TestRunInit_Default_ScaffoldsMemory(t *testing.T) {
	project := setupInitDir(t)

	var buf bytes.Buffer
	if err := RunInit(nil, &buf); err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, "docs", "memory", "README.md")); err != nil {
		t.Errorf("docs/memory/README.md should be scaffolded by default: %v", err)
	}
}

// ─── team presets imply --with-policy ────────────────────────────────────────

func TestRunInit_TeamStandardPreset_WritesPolicy(t *testing.T) {
	project := setupInitDir(t)

	var buf bytes.Buffer
	if err := RunInit([]string{"--preset=team-standard"}, &buf); err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	if _, err := os.Stat(config.PolicyConfigPath(project)); err != nil {
		t.Errorf("team-standard preset should write policy.json: %v", err)
	}
}

func TestRunInit_EnterpriseLockedPreset_WritesPolicy(t *testing.T) {
	project := setupInitDir(t)

	var buf bytes.Buffer
	if err := RunInit([]string{"--preset=enterprise-locked"}, &buf); err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	if _, err := os.Stat(config.PolicyConfigPath(project)); err != nil {
		t.Errorf("enterprise-locked preset should write policy.json: %v", err)
	}
}

func TestRunInit_SoloPresets_NoPolicy(t *testing.T) {
	for _, preset := range []string{"solo-minimal", "solo-power", "full-squad"} {
		t.Run(preset, func(t *testing.T) {
			project := setupInitDir(t)

			var buf bytes.Buffer
			if err := RunInit([]string{"--preset=" + preset}, &buf); err != nil {
				t.Fatalf("RunInit: %v", err)
			}
			if _, err := os.Stat(config.PolicyConfigPath(project)); !os.IsNotExist(err) {
				t.Errorf("%s preset should not write policy.json", preset)
			}
		})
	}
}

// ─── preset help text honesty ────────────────────────────────────────────────

func TestRunInit_Help_NoUnimplementedPresetClaims(t *testing.T) {
	var buf bytes.Buffer
	if err := RunInit([]string{"--help"}, &buf); err != nil {
		t.Fatalf("RunInit --help: %v", err)
	}
	out := buf.String()
	for _, stale := range []string{"low context budget", "strict profile"} {
		if strings.Contains(out, stale) {
			t.Errorf("init help still claims %q which is not implemented", stale)
		}
	}
	if !strings.Contains(out, "team-standard: TDD workflow, balanced models, team policy (policy.json)") {
		t.Error("init help should document that team-standard writes policy.json")
	}
}

// ─── --mcp=none ──────────────────────────────────────────────────────────────

func TestRunInit_MCPNone_EnablesNoServers(t *testing.T) {
	project := setupInitDir(t)

	var buf bytes.Buffer
	if err := RunInit([]string{"--mcp=none"}, &buf); err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project config: %v", err)
	}
	if len(proj.MCP) != 0 {
		t.Errorf("--mcp=none should enable no MCP servers, got %d", len(proj.MCP))
	}
}

// ─── both-memory-systems conflict note ───────────────────────────────────────

func TestBothMemorySystemsEnabled(t *testing.T) {
	tests := []struct {
		name string
		cfg  *domain.MergedConfig
		want bool
	}{
		{"nil config", nil, false},
		{
			"both enabled",
			&domain.MergedConfig{
				MCP:        map[string]domain.MCPServerDef{"memory": {Enabled: true}},
				Components: map[string]domain.ComponentConfig{string(domain.ComponentMemory): {Enabled: true}},
			},
			true,
		},
		{
			"only MCP server",
			&domain.MergedConfig{
				MCP: map[string]domain.MCPServerDef{"memory": {Enabled: true}},
			},
			false,
		},
		{
			"only component",
			&domain.MergedConfig{
				Components: map[string]domain.ComponentConfig{string(domain.ComponentMemory): {Enabled: true}},
			},
			false,
		},
		{
			"MCP server disabled",
			&domain.MergedConfig{
				MCP:        map[string]domain.MCPServerDef{"memory": {Enabled: false}},
				Components: map[string]domain.ComponentConfig{string(domain.ComponentMemory): {Enabled: true}},
			},
			false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := bothMemorySystemsEnabled(tc.cfg); got != tc.want {
				t.Errorf("bothMemorySystemsEnabled = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRunStatus_ShowsMemoryConflictNote(t *testing.T) {
	setupInitDir(t)

	var buf bytes.Buffer
	// Enable both the community memory MCP server and (default) Project Memory.
	if err := RunInit([]string{"--mcp=memory"}, &buf); err != nil {
		t.Fatalf("RunInit: %v", err)
	}

	buf.Reset()
	if err := RunStatus(nil, &buf); err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	if !strings.Contains(buf.String(), memoryConflictNote) {
		t.Errorf("status should show the memory conflict note, got:\n%s", buf.String())
	}
}

func TestRunStatus_NoConflictNote_WithoutMemoryMCP(t *testing.T) {
	setupInitDir(t)

	var buf bytes.Buffer
	if err := RunInit([]string{"--mcp=context7"}, &buf); err != nil {
		t.Fatalf("RunInit: %v", err)
	}

	buf.Reset()
	if err := RunStatus(nil, &buf); err != nil {
		t.Fatalf("RunStatus: %v", err)
	}
	if strings.Contains(buf.String(), memoryConflictNote) {
		t.Errorf("status should not show the conflict note without the memory MCP server, got:\n%s", buf.String())
	}
}

// --mcp=memory must remain accepted for compatibility (config key unchanged).
func TestRunInit_MCPMemory_StillAccepted(t *testing.T) {
	project := setupInitDir(t)

	var buf bytes.Buffer
	if err := RunInit([]string{"--mcp=memory"}, &buf); err != nil {
		t.Fatalf("RunInit --mcp=memory should stay accepted: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project config: %v", err)
	}
	if def, ok := proj.MCP["memory"]; !ok || !def.Enabled {
		t.Error("--mcp=memory should enable the memory MCP server under its compat key")
	}
}
