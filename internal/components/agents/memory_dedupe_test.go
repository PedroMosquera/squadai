package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/vscode"
	"github.com/PedroMosquera/squadai/internal/components/memory"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// teamConfigWithMemory returns a TDD team config with the memory component
// enabled — the configuration that used to produce TWO conflicting memory
// protocols in the same rules file.
func teamConfigWithMemory(enabled bool) *domain.MergedConfig {
	return &domain.MergedConfig{
		Methodology: domain.MethodologyTDD,
		Team:        domain.DefaultTeam(domain.MethodologyTDD),
		MCP:         map[string]domain.MCPServerDef{},
		Meta:        domain.ProjectMeta{},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: enabled},
		},
	}
}

// TestTeamMode_MemoryEnabled_ExactlyOneMemoryProtocolInRulesFile is the
// dedupe regression: with team mode active and the memory component enabled,
// the rules file must end up with exactly ONE memory-protocol section — the
// memory component's — not a second one from the agents installer.
func TestTeamMode_MemoryEnabled_ExactlyOneMemoryProtocolInRulesFile(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := vscode.New() // solo delegation → orchestrator lives in the rules file
	cfg := teamConfigWithMemory(true)

	// Memory component installs its protocol first.
	memInst := memory.New()
	memActions, err := memInst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("memory plan: %v", err)
	}
	for _, a := range memActions {
		if err := memInst.Apply(a); err != nil {
			t.Fatalf("memory apply: %v", err)
		}
	}

	// Agents installer injects the team orchestrator into the same file.
	agInst := New(nil, cfg, project)
	agActions, err := agInst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("agents plan: %v", err)
	}
	for _, a := range agActions {
		if err := agInst.Apply(a); err != nil {
			t.Fatalf("agents apply: %v", err)
		}
	}

	data, err := os.ReadFile(adapter.ProjectRulesFile(project))
	if err != nil {
		t.Fatalf("read rules file: %v", err)
	}
	doc := string(data)

	if got := strings.Count(doc, "## Project Memory Protocol"); got != 1 {
		t.Errorf("rules file must contain exactly ONE memory protocol, got %d:\n%s", got, doc)
	}
	// The agents installer's legacy section must not exist (or be empty).
	if content := marker.ExtractSection(doc, memorySectionID); content != "" {
		t.Errorf("agents memory-protocol section should be cleared when memory component owns the protocol, got:\n%s", content)
	}
	// The memory component's section is the surviving one.
	if !marker.HasSection(doc, memory.SectionIDForAgentID(adapter.ID())) {
		t.Error("memory component section missing from rules file")
	}
}

// TestTeamMode_MemoryEnabled_MigratesLegacyDuplicate verifies re-apply
// idempotently removes a pre-existing duplicate section written by older
// versions.
func TestTeamMode_MemoryEnabled_MigratesLegacyDuplicate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := vscode.New()
	cfg := teamConfigWithMemory(true)

	// Simulate an old install: rules file already has BOTH protocols.
	rulesPath := adapter.ProjectRulesFile(project)
	doc := marker.InjectSection("", memory.SectionIDForAgentID(adapter.ID()),
		memory.TemplateForAgentID(adapter.ID()))
	doc = marker.InjectSection(doc, memorySectionID, "## Project Memory Protocol\n\nlegacy duplicate")
	if err := os.WriteFile(rulesPath, []byte(doc), 0644); err != nil {
		t.Fatal(err)
	}

	inst := New(nil, cfg, project)
	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(actions) != 1 || actions[0].Action == domain.ActionSkip {
		t.Fatalf("migration must plan a real update, got %+v", actions)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("apply: %v", err)
	}

	data, _ := os.ReadFile(rulesPath)
	if got := strings.Count(string(data), "## Project Memory Protocol"); got != 1 {
		t.Errorf("after migration expected exactly one protocol, got %d", got)
	}

	// Second plan is a skip — migration is idempotent.
	again, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("re-plan: %v", err)
	}
	if len(again) != 1 || again[0].Action != domain.ActionSkip {
		t.Fatalf("expected skip after migration, got %+v", again)
	}
}

// TestTeamMode_MemoryDisabled_KeepsFullInjection preserves the legacy
// behavior for projects without the memory component: the agents installer
// still provides the full protocol in the rules file.
func TestTeamMode_MemoryDisabled_KeepsFullInjection(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := vscode.New()
	cfg := teamConfigWithMemory(false)

	inst := New(nil, cfg, project)
	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("apply: %v", err)
	}

	data, _ := os.ReadFile(adapter.ProjectRulesFile(project))
	if !strings.Contains(string(data), "## Project Memory Protocol") {
		t.Error("memory disabled: agents installer should keep the full rules-file protocol")
	}
	if !marker.HasSection(string(data), memorySectionID) {
		t.Error("memory-protocol marker section missing")
	}
}

// ─── Profile memory scope on native agent files ─────────────────────────────

func TestNativeAgents_ProfileScope(t *testing.T) {
	tests := []struct {
		name              string
		scope             string
		orchestratorFull  bool
		orchestratorStub  bool
		orchestratorEmpty bool
	}{
		{name: "no profile keeps full orchestrator protocol", scope: "", orchestratorFull: true},
		{name: "summary scope downgrades orchestrator to stub", scope: "summary", orchestratorStub: true},
		{name: "none scope removes protocol entirely", scope: "none", orchestratorEmpty: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			project := t.TempDir()
			adapter := opencode.New()
			cfg := teamConfigWithMemory(true)
			if tt.scope != "" {
				cfg.ActiveContextProfile = &domain.ContextProfile{MemoryScope: tt.scope}
				cfg.ActiveProfileName = "test"
			}
			inst := New(nil, cfg, project)

			actions, err := inst.Plan(adapter, home, project)
			if err != nil {
				t.Fatalf("plan: %v", err)
			}
			for _, a := range actions {
				if err := inst.Apply(a); err != nil {
					t.Fatalf("apply: %v", err)
				}
			}

			data, err := os.ReadFile(filepath.Join(project, ".opencode", "agents", "orchestrator.md"))
			if err != nil {
				t.Fatalf("read orchestrator: %v", err)
			}
			content := string(data)
			section := marker.ExtractSection(content, memorySectionID)
			switch {
			case tt.orchestratorFull:
				if !strings.Contains(section, "@librarian") {
					t.Errorf("expected full protocol, got:\n%s", section)
				}
			case tt.orchestratorStub:
				if section != memoryProtocolSubagentStub {
					t.Errorf("expected stub, got:\n%s", section)
				}
			case tt.orchestratorEmpty:
				if section != "" {
					t.Errorf("expected no memory protocol, got:\n%s", section)
				}
			}
		})
	}
}
