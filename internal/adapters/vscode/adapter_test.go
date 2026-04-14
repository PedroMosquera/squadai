package vscode

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
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
	// portable between macOS CI runners and Linux CI runners.
	var wantConfigDir string
	if runtime.GOOS == "linux" {
		wantConfigDir = "/Users/test/.config/Code/User"
	} else {
		wantConfigDir = "/Users/test/Library/Application Support/Code/User"
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"GlobalConfigDir", a.GlobalConfigDir(home), wantConfigDir},
		{"SystemPromptFile", a.SystemPromptFile(home), wantConfigDir + "/.instructions.md"},
		{"SkillsDir", a.SkillsDir(home), "/Users/test/.copilot/skills"},
		{"SettingsPath", a.SettingsPath(home), wantConfigDir + "/settings.json"},
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
		{"ProjectConfigFile", a.ProjectConfigFile(project), "/Users/test/myproject/.vscode/settings.json"},
		{"ProjectRulesFile", a.ProjectRulesFile(project), "/Users/test/myproject/.instructions.md"},
		{"ProjectSkillsDir", a.ProjectSkillsDir(project), "/Users/test/myproject/.copilot/skills"},
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
