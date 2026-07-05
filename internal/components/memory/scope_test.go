package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// ─── Unified protocol per adapter ───────────────────────────────────────────

func TestUnifiedProtocol_PerAdapter(t *testing.T) {
	tests := []struct {
		agentID      domain.AgentID
		wantContains []string
	}{
		{domain.AgentOpenCode, []string{"docs/memory/", "/memory-search", "@librarian", "Never skip memory-search"}},
		{domain.AgentClaudeCode, []string{"docs/memory/", "/memory-search", "@librarian", "Never skip memory-search"}},
		{domain.AgentID("unknown"), []string{"docs/memory/", "squadai memory search", "@librarian", "Never skip"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.agentID), func(t *testing.T) {
			content := TemplateForAgentID(tt.agentID)
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("%s template missing %q", tt.agentID, want)
				}
			}
			// The unified protocol must not resurrect the old rules-file-as-
			// memory-store instructions.
			for _, stale := range []string{"Session Start", "Save Triggers", "Session End"} {
				if strings.Contains(content, stale) {
					t.Errorf("%s template still contains legacy section %q", tt.agentID, stale)
				}
			}
		})
	}
}

// ─── Memory scopes ───────────────────────────────────────────────────────────

func TestContentForAgentID_Scopes(t *testing.T) {
	tests := []struct {
		name    string
		scope   string
		mode    string
		isStub  bool
		hasFull bool
	}{
		{name: "default scope is full protocol", scope: "", mode: "", hasFull: true},
		{name: "project scope is full protocol", scope: ScopeProject, mode: "", hasFull: true},
		{name: "summary scope is stub", scope: ScopeSummary, mode: "", isStub: true},
		{name: "full scope adds librarian workflow", scope: ScopeFull, mode: "", hasFull: true},
		{name: "summary mode overrides project scope", scope: ScopeProject, mode: "summary", isStub: true},
		{name: "summary mode overrides full scope", scope: ScopeFull, mode: "summary", isStub: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := New(Options{Scope: tt.scope})
			content := inst.ContentForAgentID(domain.AgentOpenCode, tt.mode)
			if tt.isStub && content != ProtocolStub {
				t.Errorf("expected stub, got:\n%s", content)
			}
			if tt.hasFull && !strings.Contains(content, "docs/memory/") {
				t.Errorf("expected full protocol, got:\n%s", content)
			}
			if tt.scope == ScopeFull && tt.mode == "" && !strings.Contains(content, "Full-scope workflow") {
				t.Error("full scope should append the librarian/promote workflow paragraph")
			}
			if tt.scope == ScopeProject && !strings.Contains(content, "docs/memory/") && !tt.isStub {
				t.Error("project scope should carry the unified protocol")
			}
		})
	}
}

func TestApply_SummaryMode_WritesStub(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	action := actions[0]
	action.Mode = "summary"
	action.Action = domain.ActionUpdate
	if err := inst.Apply(action); err != nil {
		t.Fatalf("apply: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(project, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := marker.ExtractSection(string(data), SectionIDForAgentID(domain.AgentOpenCode))
	if got != ProtocolStub {
		t.Errorf("summary mode should write the stub, got:\n%s", got)
	}
}

func TestPlan_SummaryScope_SkipsWhenStubCurrent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := New(Options{Scope: ScopeSummary})

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("apply: %v", err)
	}

	again, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("re-plan: %v", err)
	}
	if len(again) != 1 || again[0].Action != domain.ActionSkip {
		t.Fatalf("expected skip when stub current, got %+v", again)
	}

	// Verify passes against the scope-selected content too.
	results, err := inst.Verify(adapter, home, project)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
}
