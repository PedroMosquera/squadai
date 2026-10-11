package efficiency

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/pi"
	"github.com/PedroMosquera/squadai/internal/adapters/vscode"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/marker"
)

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	if New().ID() != domain.ComponentEfficiency {
		t.Errorf("ID() = %q, want %q", New().ID(), domain.ComponentEfficiency)
	}
}

// ─── Conditional rendering ──────────────────────────────────────────────────

func TestContentForAgentID_ConditionalRendering(t *testing.T) {
	tests := []struct {
		name          string
		agentID       domain.AgentID
		memoryEnabled bool
		wantContains  []string
		wantAbsent    []string
	}{
		{
			name:          "delegating adapter with memory",
			agentID:       domain.AgentOpenCode,
			memoryEnabled: true,
			wantContains:  []string{"Delegate exploration", "Memory first", "Checkpoint at ~60% context", "Search before read"},
			wantAbsent:    []string{"Timebox exploration"},
		},
		{
			name:          "delegating adapter without memory",
			agentID:       domain.AgentClaudeCode,
			memoryEnabled: false,
			wantContains:  []string{"Delegate exploration"},
			wantAbsent:    []string{"Memory first", "Timebox exploration"},
		},
		{
			name:          "solo adapter gets timebox variant",
			agentID:       domain.AgentVSCodeCopilot,
			memoryEnabled: true,
			wantContains:  []string{"Timebox exploration", "Memory first"},
			wantAbsent:    []string{"Delegate exploration"},
		},
		{
			name:          "windsurf is solo too",
			agentID:       domain.AgentWindsurf,
			memoryEnabled: false,
			wantContains:  []string{"Timebox exploration"},
			wantAbsent:    []string{"Delegate exploration", "Memory first"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := New(Options{MemoryEnabled: tt.memoryEnabled})
			content, err := inst.ContentForAgentID(tt.agentID)
			if err != nil {
				t.Fatalf("ContentForAgentID: %v", err)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("content missing %q:\n%s", want, content)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(content, absent) {
					t.Errorf("content should not contain %q:\n%s", absent, content)
				}
			}
			if strings.Contains(content, "\n\n\n") {
				t.Error("rendered content has triple blank lines (template whitespace bug)")
			}
		})
	}
}

// ─── Plan / Apply idempotence ────────────────────────────────────────────────

func TestPlanApply_Idempotent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(Options{MemoryEnabled: true})

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(actions) != 1 || actions[0].Action != domain.ActionCreate {
		t.Fatalf("expected 1 create action, got %+v", actions)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Second plan: everything current → skip.
	again, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("re-plan: %v", err)
	}
	if len(again) != 1 || again[0].Action != domain.ActionSkip {
		t.Fatalf("expected skip on re-plan, got %+v", again)
	}
}

func TestPlan_OptionChange_TriggersUpdate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	withMemory := New(Options{MemoryEnabled: true})
	actions, err := withMemory.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if err := withMemory.Apply(actions[0]); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Memory turned off → the memory-first rule must be removed → update.
	withoutMemory := New(Options{MemoryEnabled: false})
	again, err := withoutMemory.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("re-plan: %v", err)
	}
	if len(again) != 1 || again[0].Action != domain.ActionUpdate {
		t.Fatalf("expected update after option change, got %+v", again)
	}
}

// ─── One shared section in AGENTS.md ─────────────────────────────────────────

func TestApply_SharedAgentsMD_OneSection(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	inst := New(Options{MemoryEnabled: false})
	path := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(path, []byte(marker.InjectSection("", "efficiency:opencode", "old copy")), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, adapter := range []domain.Adapter{opencode.New(), pi.New()} {
		actions, err := inst.Plan(adapter, home, project)
		if err != nil {
			t.Fatalf("plan %s: %v", adapter.ID(), err)
		}
		if len(actions) != 1 {
			t.Fatalf("expected 1 action for %s, got %d", adapter.ID(), len(actions))
		}
		if err := inst.Apply(actions[0]); err != nil {
			t.Fatalf("apply %s: %v", adapter.ID(), err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	doc := string(data)
	if ids := marker.SectionIDs(doc); len(ids) != 1 || ids[0] != SectionID {
		t.Errorf("AGENTS.md sections = %v, want one shared %q section", ids, SectionID)
	}
	if n := strings.Count(doc, "## Session Efficiency Protocol"); n != 1 {
		t.Errorf("expected one protocol in AGENTS.md, got %d", n)
	}
}

func TestContentForAgentID_SharedCopyDelegatesWhenAnyAgentCan(t *testing.T) {
	solo := New(Options{Agents: []domain.AgentID{domain.AgentVSCodeCopilot, domain.AgentWindsurf}})
	got, err := solo.ContentForAgentID("")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "**Timebox exploration.**") {
		t.Errorf("solo-only project should get the timebox variant:\n%s", got)
	}
	mixed := New(Options{Agents: []domain.AgentID{domain.AgentVSCodeCopilot, domain.AgentOpenCode}})
	got, err = mixed.ContentForAgentID("")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "**Delegate exploration.**") {
		t.Errorf("a project with a delegating adapter should get the delegation variant:\n%s", got)
	}
}

func TestInjectContent_ClearsLegacyUnscopedSection(t *testing.T) {
	legacy := marker.InjectSection("", SectionID, "old unscoped efficiency block")
	updated := InjectContent(legacy, domain.AgentOpenCode, "new content")
	if marker.HasSection(updated, SectionID) && marker.ExtractSection(updated, SectionID) != "" {
		t.Error("legacy unscoped section should be removed on migration")
	}
	if marker.ExtractSection(updated, SectionIDForAgentID(domain.AgentOpenCode)) != "new content" {
		t.Error("adapter-scoped section not written")
	}
}

// ─── Verify ──────────────────────────────────────────────────────────────────

func TestVerify_PassesAfterApply_FailsWhenOutdated(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := vscode.New()
	inst := New(Options{MemoryEnabled: true})

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("apply: %v", err)
	}

	results, err := inst.Verify(adapter, home, project)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed after apply: %s", r.Check, r.Message)
		}
	}

	// A different option set makes the on-disk content outdated.
	changed := New(Options{MemoryEnabled: false})
	results, err = changed.Verify(adapter, home, project)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	sawOutdated := false
	for _, r := range results {
		if r.Check == "efficiency-content-current" && !r.Passed {
			sawOutdated = true
		}
	}
	if !sawOutdated {
		t.Error("expected efficiency-content-current failure after option change")
	}
}
