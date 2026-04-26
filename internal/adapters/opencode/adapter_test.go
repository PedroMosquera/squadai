package opencode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// ─── ID / Lane ──────────────────────────────────────────────────────────────

func TestAdapter_ID(t *testing.T) {
	a := New()
	if a.ID() != domain.AgentOpenCode {
		t.Errorf("ID() = %q, want %q", a.ID(), domain.AgentOpenCode)
	}
}

func TestAdapter_Lane(t *testing.T) {
	a := New()
	if a.Lane() != domain.LaneTeam {
		t.Errorf("Lane() = %q, want %q", a.Lane(), domain.LaneTeam)
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
		func(name string) (string, error) { return "/usr/local/bin/opencode", nil },
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
		func(name string) (string, error) { return "/usr/local/bin/opencode", nil },
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
		func(name string) (string, error) { return "/bin/opencode", nil },
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

	// All OpenCode paths are home-relative with no OS branching;
	// use filepath.Join so separators are correct on every CI platform.
	wantConfigDir := filepath.Join(home, ".config", "opencode")

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"GlobalConfigDir", a.GlobalConfigDir(home), wantConfigDir},
		{"SystemPromptFile", a.SystemPromptFile(home), filepath.Join(wantConfigDir, "AGENTS.md")},
		{"SkillsDir", a.SkillsDir(home), filepath.Join(wantConfigDir, "skills")},
		{"SettingsPath", a.SettingsPath(home), filepath.Join(wantConfigDir, "opencode.json")},
		{"AgentsDir", a.AgentsDir(home), filepath.Join(wantConfigDir, "agents")},
		{"CommandsDir", a.CommandsDir(home), filepath.Join(wantConfigDir, "commands")},
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
		{"ProjectConfigFile", a.ProjectConfigFile(project), filepath.Join(project, "opencode.json")},
		{"ProjectRulesFile", a.ProjectRulesFile(project), filepath.Join(project, "AGENTS.md")},
		{"ProjectAgentsDir", a.ProjectAgentsDir(project), filepath.Join(project, ".opencode", "agents")},
		{"ProjectSkillsDir", a.ProjectSkillsDir(project), filepath.Join(project, ".opencode", "skills")},
		{"ProjectCommandsDir", a.ProjectCommandsDir(project), filepath.Join(project, ".opencode", "commands")},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

// ─── SupportsComponent ──────────────────────────────────────────────────────

func TestSupportsComponent_Memory(t *testing.T) {
	a := New()
	if !a.SupportsComponent(domain.ComponentMemory) {
		t.Error("OpenCode should support memory component")
	}
}

func TestSupportsComponent_NewComponents(t *testing.T) {
	a := New()
	components := []domain.ComponentID{
		domain.ComponentRules,
		domain.ComponentSettings,
		domain.ComponentMCP,
		domain.ComponentAgents,
		domain.ComponentSkills,
		domain.ComponentCommands,
	}
	for _, c := range components {
		if !a.SupportsComponent(c) {
			t.Errorf("OpenCode should support component %q", c)
		}
	}
}

func TestSupportsComponent_Unknown(t *testing.T) {
	a := New()
	if a.SupportsComponent(domain.ComponentID("nonexistent")) {
		t.Error("OpenCode should not support unknown components")
	}
}

// ─── Interface compliance ───────────────────────────────────────────────────

func TestAdapter_ImplementsInterface(t *testing.T) {
	var _ domain.Adapter = (*Adapter)(nil)
}

// ─── V2 interface methods ───────────────────────────────────────────────────

func TestAdapter_DelegationStrategy(t *testing.T) {
	a := New()
	if a.DelegationStrategy() != domain.DelegationNativeAgents {
		t.Errorf("DelegationStrategy() = %q, want %q", a.DelegationStrategy(), domain.DelegationNativeAgents)
	}
}

func TestAdapter_SupportsSubAgents(t *testing.T) {
	a := New()
	if !a.SupportsSubAgents() {
		t.Error("OpenCode should support sub-agents")
	}
}

func TestAdapter_SubAgentsDir(t *testing.T) {
	a := New()
	home := "/Users/test"
	want := filepath.Join(home, ".config", "opencode", "agents")
	if got := a.SubAgentsDir(home); got != want {
		t.Errorf("SubAgentsDir = %q, want %q", got, want)
	}
}

func TestAdapter_SupportsWorkflows(t *testing.T) {
	a := New()
	if a.SupportsWorkflows() {
		t.Error("OpenCode should not support workflows")
	}
}

func TestAdapter_WorkflowsDir(t *testing.T) {
	a := New()
	if a.WorkflowsDir("/project") != "" {
		t.Error("WorkflowsDir should be empty for OpenCode")
	}
}

// ─── MCP / Rules metadata ───────────────────────────────────────────────────

func TestAdapter_MCPRootKey(t *testing.T) {
	a := New()
	want := "mcp"
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
		{"with project dir", "/tmp/proj", ""},
		{"empty project dir", "", ""},
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
	if got := a.MCPCommandStyle(); got != "array" {
		t.Errorf("MCPCommandStyle() = %q, want %q", got, "array")
	}
}

func TestAdapter_MCPEnvKey(t *testing.T) {
	a := New()
	if got := a.MCPEnvKey(); got != "environment" {
		t.Errorf("MCPEnvKey() = %q, want %q", got, "environment")
	}
}

func TestAdapter_MCPTypeField(t *testing.T) {
	a := New()
	tests := []struct {
		name string
		def  domain.MCPServerDef
		want string
	}{
		{"stdio echoes def.Type", domain.MCPServerDef{Type: "local"}, "local"},
		{"remote echoes def.Type", domain.MCPServerDef{Type: "remote", URL: "https://x"}, "remote"},
		{"empty type passes through", domain.MCPServerDef{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := a.MCPTypeField(tt.def); got != tt.want {
				t.Errorf("MCPTypeField(%+v) = %q, want %q", tt.def, got, tt.want)
			}
		})
	}
}
