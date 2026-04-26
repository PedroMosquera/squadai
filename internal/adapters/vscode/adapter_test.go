package vscode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

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
		// On Windows ConfigDir uses %APPDATA%; fall back to the homeDir path
		// when APPDATA is unset (as it is in this test which passes a fake home).
		if appData := os.Getenv("APPDATA"); appData != "" {
			wantConfigDir = filepath.Join(appData, "Code", "User")
		} else {
			wantConfigDir = filepath.Join("/Users/test", "AppData", "Roaming", "Code", "User")
		}
	default:
		wantConfigDir = "/Users/test/Library/Application Support/Code/User"
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"GlobalConfigDir", a.GlobalConfigDir(home), wantConfigDir},
		{"SystemPromptFile", a.SystemPromptFile(home), filepath.Join(wantConfigDir, ".instructions.md")},
		{"SkillsDir", a.SkillsDir(home), filepath.Join(home, ".copilot", "skills")},
		{"SettingsPath", a.SettingsPath(home), filepath.Join(wantConfigDir, "settings.json")},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
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
		{"ProjectRulesFile", a.ProjectRulesFile(project), filepath.Join(project, ".instructions.md")},
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
	want := "servers"
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
		{"with project dir", "/tmp/proj", filepath.Join("/tmp/proj", ".vscode", "mcp.json")},
		{"empty project dir", "", filepath.Join(".vscode", "mcp.json")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := a.MCPConfigPath(tt.projectDir); got != tt.want {
				t.Errorf("MCPConfigPath(%q) = %q, want %q", tt.projectDir, got, tt.want)
			}
		})
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
