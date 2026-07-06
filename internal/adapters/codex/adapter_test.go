package codex

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
	if a.ID() != domain.AgentCodex {
		t.Errorf("ID() = %q, want %q", a.ID(), domain.AgentCodex)
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
		func(name string) (string, error) { return "/usr/local/bin/codex", nil },
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
		func(name string) (string, error) { return "/usr/local/bin/codex", nil },
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
		func(name string) (string, error) { return "/bin/codex", nil },
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

	wantConfigDir := filepath.Join(home, ".codex")

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"GlobalConfigDir", a.GlobalConfigDir(home), wantConfigDir},
		{"SystemPromptFile", a.SystemPromptFile(home), filepath.Join(wantConfigDir, "AGENTS.md")},
		{"SettingsPath", a.SettingsPath(home), filepath.Join(wantConfigDir, "config.toml")},
		{"MCPTOMLConfigPath", a.MCPTOMLConfigPath(home), filepath.Join(wantConfigDir, "config.toml")},
		{"SkillsDir", a.SkillsDir(home), ""},
		{"SubAgentsDir", a.SubAgentsDir(home), ""},
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
		{"ProjectConfigFile", a.ProjectConfigFile(project), ""},
		{"ProjectRulesFile", a.ProjectRulesFile(project), filepath.Join(project, "AGENTS.md")},
		{"ProjectAgentsDir", a.ProjectAgentsDir(project), ""},
		{"ProjectSkillsDir", a.ProjectSkillsDir(project), ""},
		{"ProjectCommandsDir", a.ProjectCommandsDir(project), ""},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

// ─── SupportsComponent ──────────────────────────────────────────────────────

func TestSupportsComponent_Supported(t *testing.T) {
	a := New()
	components := []domain.ComponentID{
		domain.ComponentMemory,
		domain.ComponentRules,
		domain.ComponentMCP,
		domain.ComponentBrand,
		domain.ComponentEfficiency,
	}
	for _, c := range components {
		if !a.SupportsComponent(c) {
			t.Errorf("Codex should support component %q", c)
		}
	}
}

func TestSupportsComponent_Unsupported(t *testing.T) {
	a := New()
	components := []domain.ComponentID{
		domain.ComponentAgents,
		domain.ComponentSkills,
		domain.ComponentCommands,
		domain.ComponentPermissions,
		domain.ComponentSettings,
		domain.ComponentPlugins,
		domain.ComponentWorkflows,
	}
	for _, c := range components {
		if a.SupportsComponent(c) {
			t.Errorf("Codex should not support component %q", c)
		}
	}
}

func TestSupportsComponent_Unknown(t *testing.T) {
	a := New()
	if a.SupportsComponent(domain.ComponentID("nonexistent")) {
		t.Error("Codex should not support unknown components")
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
		t.Error("Codex should not support sub-agents")
	}
}

func TestAdapter_SupportsWorkflows(t *testing.T) {
	a := New()
	if a.SupportsWorkflows() {
		t.Error("Codex should not support workflows")
	}
}

func TestAdapter_WorkflowsDir(t *testing.T) {
	a := New()
	if a.WorkflowsDir("/project") != "" {
		t.Error("WorkflowsDir should be empty for Codex")
	}
}

// ─── MCP / Rules metadata ───────────────────────────────────────────────────

func TestAdapter_MCPRootKey(t *testing.T) {
	a := New()
	if got := a.MCPRootKey(); got != "mcp_servers" {
		t.Errorf("MCPRootKey() = %q, want %q", got, "mcp_servers")
	}
}

func TestAdapter_MCPURLKey(t *testing.T) {
	a := New()
	if got := a.MCPURLKey(); got != "url" {
		t.Errorf("MCPURLKey() = %q, want %q", got, "url")
	}
}

func TestAdapter_MCPConfigPath(t *testing.T) {
	a := New()
	if got := a.MCPConfigPath("/tmp/proj"); got != "" {
		t.Errorf("MCPConfigPath() = %q, want empty string (TOML strategy uses MCPTOMLConfigPath)", got)
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
	}{
		{"stdio omitted", domain.MCPServerDef{Type: "local"}},
		{"remote omitted", domain.MCPServerDef{Type: "remote", URL: "https://x"}},
		{"empty omitted", domain.MCPServerDef{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := a.MCPTypeField(tt.def); got != "" {
				t.Errorf("MCPTypeField(%+v) = %q, want empty string", tt.def, got)
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

func TestAdapter_RulesFileSizeCap(t *testing.T) {
	a := New()
	if got := a.RulesFileSizeCap(); got != 0 {
		t.Errorf("RulesFileSizeCap() = %d, want 0", got)
	}
}
