package skills

import (
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/domain"
)

func tddConfig() *domain.MergedConfig {
	return &domain.MergedConfig{
		Methodology: domain.MethodologyTDD,
	}
}

// planRelPaths runs Plan and returns the set of skill-relative paths planned.
func planRelPaths(t *testing.T, inst *Installer) map[string]bool {
	t.Helper()
	adapter := opencode.New()
	actions, err := inst.Plan(adapter, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	out := make(map[string]bool, len(actions))
	for _, a := range actions {
		out[a.ID] = true
	}
	return out
}

func TestInScope_Matching(t *testing.T) {
	tests := []struct {
		name   string
		scopes []string
		rel    string
		want   bool
	}{
		{"nil filter admits everything", nil, "tdd/brainstorming", true},
		{"prefix scope matches children", []string{"shared"}, "shared/code-review", true},
		{"prefix scope does not match siblings", []string{"shared"}, "tdd/brainstorming", false},
		{"exact scope matches", []string{"tdd/systematic-debugging"}, "tdd/systematic-debugging", true},
		{"exact scope does not match siblings", []string{"tdd/systematic-debugging"}, "tdd/brainstorming", false},
		{"prefix must be a path segment", []string{"shar"}, "shared/code-review", false},
		{"empty filter admits nothing", []string{}, "shared/code-review", false},
		{"multiple scopes union", []string{"shared", "tdd/systematic-debugging"}, "tdd/systematic-debugging", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := New(nil, nil, "", Options{Scopes: tt.scopes})
			if got := inst.inScope(tt.rel); got != tt.want {
				t.Errorf("inScope(%q) with scopes %v = %v, want %v", tt.rel, tt.scopes, got, tt.want)
			}
		})
	}
}

func TestPlan_ScopeFiltering(t *testing.T) {
	// No filter: all shared + tdd skills planned.
	all := planRelPaths(t, New(nil, tddConfig(), t.TempDir()))
	if !all["skill-embedded-shared/code-review"] || !all["skill-embedded-tdd/brainstorming"] {
		t.Fatalf("unfiltered plan missing expected skills: %v", all)
	}

	// Prefix filter: only shared skills.
	shared := planRelPaths(t, New(nil, tddConfig(), t.TempDir(), Options{Scopes: []string{"shared"}}))
	if !shared["skill-embedded-shared/code-review"] {
		t.Error("shared scope should keep shared/code-review")
	}
	if shared["skill-embedded-tdd/brainstorming"] {
		t.Error("shared scope should filter out tdd skills")
	}

	// Exact + prefix union.
	mixed := planRelPaths(t, New(nil, tddConfig(), t.TempDir(),
		Options{Scopes: []string{"shared", "tdd/systematic-debugging"}}))
	if !mixed["skill-embedded-tdd/systematic-debugging"] {
		t.Error("exact scope should keep tdd/systematic-debugging")
	}
	if mixed["skill-embedded-tdd/brainstorming"] {
		t.Error("exact scope should not admit sibling tdd skills")
	}
}

func TestPlan_ScopeFiltering_CustomSkills(t *testing.T) {
	custom := map[string]domain.SkillDef{
		"my-skill":    {Description: "custom"},
		"other-skill": {Description: "custom"},
	}
	inst := New(custom, nil, t.TempDir(), Options{Scopes: []string{"my-skill"}})
	got := planRelPaths(t, inst)
	if !got["opencode-skill-my-skill"] {
		t.Errorf("custom skill matching an exact scope should be planned: %v", got)
	}
	if got["opencode-skill-other-skill"] {
		t.Error("custom skill outside the scopes should be filtered")
	}
}
