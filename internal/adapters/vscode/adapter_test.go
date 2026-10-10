package vscode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/paths"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// ─── ID / Lane ──────────────────────────────────────────────────────────────

func TestAdapter_ID(t *testing.T) {
	a := New()
	if a.ID() != domain.AgentVSCodeCopilot {
		t.Errorf("ID() = %q, want %q", a.ID(), domain.AgentVSCodeCopilot)
	}
}

func TestAdapter_Lane(t *testing.T) {
	a := New()
	if a.Lane() != domain.LanePersonal {
		t.Errorf("Lane() = %q, want %q", a.Lane(), domain.LanePersonal)
	}
}

// ─── Detect ─────────────────────────────────────────────────────────────────

func TestDetect_BinaryAndConfigExist(t *testing.T) {
	dir := t.TempDir()
	configDir := ConfigDir(dir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	a := NewWithDeps(
		func(name string) (string, error) { return "/usr/local/bin/code", nil },
		os.Stat,
	)

	installed, configFound, err := a.Detect(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !installed {
		t.Error("expected installed=true")
	}
	if !configFound {
		t.Error("expected configFound=true")
	}
}

func TestDetect_BinaryExists_ConfigMissing(t *testing.T) {
	dir := t.TempDir()

	a := NewWithDeps(
		func(name string) (string, error) { return "/usr/local/bin/code", nil },
		os.Stat,
	)

	installed, configFound, err := a.Detect(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !installed {
		t.Error("expected installed=true")
	}
	if configFound {
		t.Error("expected configFound=false")
	}
}

func TestDetect_BinaryMissing_ConfigExists(t *testing.T) {
	dir := t.TempDir()
	configDir := ConfigDir(dir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	a := NewWithDeps(
		func(name string) (string, error) { return "", fmt.Errorf("not found") },
		os.Stat,
	)

	installed, configFound, err := a.Detect(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if installed {
		t.Error("expected installed=false")
	}
	if !configFound {
		t.Error("expected configFound=true")
	}
}

func TestDetect_BothMissing(t *testing.T) {
	dir := t.TempDir()

	a := NewWithDeps(
		func(name string) (string, error) { return "", fmt.Errorf("not found") },
		os.Stat,
	)

	installed, configFound, err := a.Detect(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if installed {
		t.Error("expected installed=false")
	}
	if configFound {
		t.Error("expected configFound=false")
	}
}

func TestDetect_StatError_ReturnsError(t *testing.T) {
	a := NewWithDeps(
		func(name string) (string, error) { return "/bin/code", nil },
		func(name string) (os.FileInfo, error) { return nil, fmt.Errorf("permission denied") },
	)

	_, _, err := a.Detect(context.Background(), "/fake")
	if err == nil {
		t.Fatal("expected error from stat failure")
	}
}

// ─── Paths ──────────────────────────────────────────────────────────────────

func TestPaths(t *testing.T) {
	a := New()
	home := "/Users/test"

	// ConfigDir differs by OS; compute the expected base so the test is
	// portable between macOS, Linux, and Windows CI runners.
	var wantConfigDir string
	switch runtime.GOOS {
	case "linux":
		wantConfigDir = "/Users/test/.config/Code/User"
	case "windows":
		wantConfigDir = paths.UserConfigDir(home, runtime.GOOS, "Code")
	default:
		wantConfigDir = "/Users/test/Library/Application Support/Code/User"
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"GlobalConfigDir", a.GlobalConfigDir(home), wantConfigDir},
		// Locations from https://code.visualstudio.com/docs/copilot/customization/custom-instructions
		// (checked 2026-10-10): "For personal, always-on instructions in Copilot Agent Host
		// sessions, use ~/.copilot/copilot-instructions.md."
		{"SystemPromptFile", a.SystemPromptFile(home), filepath.Join(home, ".copilot", "copilot-instructions.md")},
		{"SkillsDir", a.SkillsDir(home), filepath.Join(home, ".copilot", "skills")},
		{"SettingsPath", a.SettingsPath(home), filepath.Join(wantConfigDir, "settings.json")},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

// TestConfigDirAppName proves ConfigDir delegates to the shared helper with
// app name "Code". Per-OS resolution details are covered by the paths
// package tests.
func TestConfigDirAppName(t *testing.T) {
	home := "/Users/test"
	if got, want := ConfigDir(home), paths.UserConfigDir(home, runtime.GOOS, "Code"); got != want {
		t.Errorf("ConfigDir = %q, want %q", got, want)
	}
}

func TestProjectPaths(t *testing.T) {
	a := New()
	project := "/Users/test/myproject"

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"ProjectConfigFile", a.ProjectConfigFile(project), filepath.Join(project, ".vscode", "settings.json")},
		// VS Code reads .github/copilot-instructions.md for project-wide guidance and
		// documents no root .instructions.md:
		// https://code.visualstudio.com/docs/copilot/customization/custom-instructions
		{"ProjectRulesFile", a.ProjectRulesFile(project), filepath.Join(project, ".github", "copilot-instructions.md")},
		{"ProjectSkillsDir", a.ProjectSkillsDir(project), filepath.Join(project, ".copilot", "skills")},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestProjectPaths_EmptyForUnsupported(t *testing.T) {
	a := New()
	project := "/Users/test/myproject"

	if got := a.ProjectAgentsDir(project); got != "" {
		t.Errorf("ProjectAgentsDir = %q, want empty", got)
	}
	if got := a.ProjectCommandsDir(project); got != "" {
		t.Errorf("ProjectCommandsDir = %q, want empty", got)
	}
}

// ─── SupportsComponent ──────────────────────────────────────────────────────

func TestSupportsComponent_Supported(t *testing.T) {
	a := New()
	components := []domain.ComponentID{
		domain.ComponentMemory,
		domain.ComponentRules,
		domain.ComponentSettings,
		domain.ComponentMCP,
		domain.ComponentSkills,
		domain.ComponentPlugins,
	}
	for _, c := range components {
		if !a.SupportsComponent(c) {
			t.Errorf("VS Code Copilot should support component %q", c)
		}
	}
}

func TestSupportsComponent_Unsupported(t *testing.T) {
	a := New()
	components := []domain.ComponentID{
		domain.ComponentAgents,
		domain.ComponentCommands,
	}
	for _, c := range components {
		if a.SupportsComponent(c) {
			t.Errorf("VS Code Copilot should not support component %q", c)
		}
	}
}

func TestSupportsComponent_Unknown(t *testing.T) {
	a := New()
	if a.SupportsComponent(domain.ComponentID("nonexistent")) {
		t.Error("VS Code Copilot should not support unknown components")
	}
}

// ─── Interface compliance ───────────────────────────────────────────────────

func TestAdapter_ImplementsInterface(t *testing.T) {
	var _ domain.Adapter = (*Adapter)(nil)
}

// ─── V2 interface methods ───────────────────────────────────────────────────

func TestAdapter_DelegationStrategy(t *testing.T) {
	a := New()
	if a.DelegationStrategy() != domain.DelegationSoloAgent {
		t.Errorf("DelegationStrategy() = %q, want %q", a.DelegationStrategy(), domain.DelegationSoloAgent)
	}
}

func TestAdapter_SupportsSubAgents(t *testing.T) {
	a := New()
	if a.SupportsSubAgents() {
		t.Error("VS Code Copilot should not support sub-agents")
	}
}

func TestAdapter_SubAgentsDir(t *testing.T) {
	a := New()
	if got := a.SubAgentsDir("/Users/test"); got != "" {
		t.Errorf("SubAgentsDir = %q, want empty", got)
	}
}

func TestAdapter_SupportsWorkflows(t *testing.T) {
	a := New()
	if a.SupportsWorkflows() {
		t.Error("VS Code Copilot should not support workflows")
	}
}

func TestAdapter_WorkflowsDir(t *testing.T) {
	a := New()
	if got := a.WorkflowsDir("/project"); got != "" {
		t.Errorf("WorkflowsDir = %q, want empty", got)
	}
}

// ─── MCP / Rules metadata ───────────────────────────────────────────────────

func TestAdapter_MCPRootKey(t *testing.T) {
	a := New()
	want := "mcpServers"
	if got := a.MCPRootKey(); got != want {
		t.Errorf("MCPRootKey() = %q, want %q", got, want)
	}
}

func TestAdapter_MCPURLKey(t *testing.T) {
	a := New()
	want := "url"
	if got := a.MCPURLKey(); got != want {
		t.Errorf("MCPURLKey() = %q, want %q", got, want)
	}
}

func TestAdapter_MCPConfigPath(t *testing.T) {
	a := New()
	tests := []struct {
		name       string
		projectDir string
		want       string
	}{
		{"with project dir", "/tmp/proj", filepath.Join("/tmp/proj", ".mcp.json")},
		{"empty project dir", "", ".mcp.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := a.MCPConfigPath(tt.projectDir); got != tt.want {
				t.Errorf("MCPConfigPath(%q) = %q, want %q", tt.projectDir, got, tt.want)
			}
		})
	}
}

func TestAdapter_LegacyInstructionsFiles(t *testing.T) {
	home, project := "/Users/test", "/Users/test/myproject"
	got := New().LegacyInstructionsFiles(home, project)
	want := []string{
		filepath.Join(project, ".instructions.md"),
		filepath.Join(ConfigDir(home), ".instructions.md"),
	}
	if len(got) != len(want) {
		t.Fatalf("LegacyInstructionsFiles = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("LegacyInstructionsFiles[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAdapter_LegacyMCPConfig(t *testing.T) {
	path, rootKey := New().LegacyMCPConfig("/tmp/proj")
	if want := filepath.Join("/tmp/proj", ".vscode", "mcp.json"); path != want {
		t.Errorf("LegacyMCPConfig path = %q, want %q", path, want)
	}
	if rootKey != "servers" {
		t.Errorf("LegacyMCPConfig rootKey = %q, want %q", rootKey, "servers")
	}
}

func TestAdapter_RulesFrontmatter(t *testing.T) {
	a := New()
	if got := a.RulesFrontmatter(); got != "" {
		t.Errorf("RulesFrontmatter() = %q, want empty string", got)
	}
}

func TestAdapter_MCPCommandStyle(t *testing.T) {
	a := New()
	if got := a.MCPCommandStyle(); got != "split" {
		t.Errorf("MCPCommandStyle() = %q, want %q", got, "split")
	}
}

func TestAdapter_MCPEnvKey(t *testing.T) {
	a := New()
	if got := a.MCPEnvKey(); got != "env" {
		t.Errorf("MCPEnvKey() = %q, want %q", got, "env")
	}
}

func TestAdapter_MCPTypeField(t *testing.T) {
	a := New()
	tests := []struct {
		name string
		def  domain.MCPServerDef
		want string
	}{
		{"stdio omits type", domain.MCPServerDef{Command: []string{"npx"}}, ""},
		{"remote uses http", domain.MCPServerDef{URL: "https://x"}, "http"},
		{"empty omits type", domain.MCPServerDef{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := a.MCPTypeField(tt.def); got != tt.want {
				t.Errorf("MCPTypeField(%+v) = %q, want %q", tt.def, got, tt.want)
			}
		})
	}
}
