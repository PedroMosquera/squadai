package claude

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
	if a.ID() != domain.AgentClaudeCode {
		t.Errorf("ID() = %q, want %q", a.ID(), domain.AgentClaudeCode)
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
		func(name string) (string, error) { return "/usr/local/bin/claude", nil },
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
		func(name string) (string, error) { return "/usr/local/bin/claude", nil },
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
		func(name string) (string, error) { return "/bin/claude", nil },
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
		{"GlobalConfigDir", a.GlobalConfigDir(home), "/Users/test/.claude"},
		{"SystemPromptFile", a.SystemPromptFile(home), "/Users/test/.claude/CLAUDE.md"},
		{"SkillsDir", a.SkillsDir(home), "/Users/test/.claude/skills"},
		{"SettingsPath", a.SettingsPath(home), "/Users/test/.claude/settings.json"},
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
		t.Error("Claude Code should support memory component")
	}
}

func TestSupportsComponent_Unknown(t *testing.T) {
	a := New()
	if a.SupportsComponent(domain.ComponentID("mcp")) {
		t.Error("Claude Code should not support unknown components in V1")
	}
}

// ─── Interface compliance ───────────────────────────────────────────────────

func TestAdapter_ImplementsInterface(t *testing.T) {
	var _ domain.Adapter = (*Adapter)(nil)
}
