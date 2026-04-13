package opencode

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
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

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"GlobalConfigDir", a.GlobalConfigDir(home), "/Users/test/.config/opencode"},
		{"SystemPromptFile", a.SystemPromptFile(home), "/Users/test/.config/opencode/AGENTS.md"},
		{"SkillsDir", a.SkillsDir(home), "/Users/test/.config/opencode/skills"},
		{"SettingsPath", a.SettingsPath(home), "/Users/test/.config/opencode/opencode.json"},
		{"AgentsDir", a.AgentsDir(home), "/Users/test/.config/opencode/agents"},
		{"CommandsDir", a.CommandsDir(home), "/Users/test/.config/opencode/commands"},
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
		{"ProjectConfigFile", a.ProjectConfigFile(project), "/Users/test/myproject/opencode.json"},
		{"ProjectRulesFile", a.ProjectRulesFile(project), "/Users/test/myproject/AGENTS.md"},
		{"ProjectAgentsDir", a.ProjectAgentsDir(project), "/Users/test/myproject/.opencode/agents"},
		{"ProjectSkillsDir", a.ProjectSkillsDir(project), "/Users/test/myproject/.opencode/skills"},
		{"ProjectCommandsDir", a.ProjectCommandsDir(project), "/Users/test/myproject/.opencode/commands"},
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
	dir := a.SubAgentsDir("/Users/test")
	if dir != "/Users/test/.config/opencode/agents" {
		t.Errorf("SubAgentsDir = %q, want /Users/test/.config/opencode/agents", dir)
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
