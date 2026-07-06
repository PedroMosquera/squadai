package budget

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// writeFile creates a file of n zero bytes under dir and returns its path.
// ApproxTokens uses 4 bytes/token, so n bytes → n/4 tokens (ceiling).
func writeFile(t *testing.T, dir, name string, n int) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, make([]byte, n), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

// contentAction builds a create-action for a content component targeting path.
func contentAction(component domain.ComponentID, path string) domain.PlannedAction {
	return domain.PlannedAction{
		ID:          string(component) + "-" + filepath.Base(path),
		Agent:       domain.AgentOpenCode,
		Component:   component,
		Action:      domain.ActionCreate,
		TargetPath:  path,
		Description: "test " + string(component),
	}
}

// modesFrom converts a FitResult's decisions into a component→mode lookup.
func modesFrom(res *FitResult) map[domain.ComponentID]Mode {
	m := make(map[domain.ComponentID]Mode, len(res.Decisions))
	for _, d := range res.Decisions {
		m[d.Component] = d.Mode
	}
	return m
}

func TestFit_NoCap(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "plugins.md", 800) // 200 tokens
	actions := []domain.PlannedAction{
		contentAction(domain.ComponentPlugins, p),
	}
	res, err := Fit(actions, Options{MaxTokens: 0})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if !res.FitAchieved {
		t.Error("FitAchieved = false, want true for no cap")
	}
	if len(res.Decisions) != 1 || res.Decisions[0].Mode != ModeFull {
		t.Errorf("expected 1 full decision, got %+v", res.Decisions)
	}
	if len(res.Actions) != 1 {
		t.Errorf("expected 1 action returned, got %d", len(res.Actions))
	}
	if res.TotalTokens != 200 {
		t.Errorf("TotalTokens = %d, want 200", res.TotalTokens)
	}
	if res.Cap != 0 {
		t.Errorf("Cap = %d, want 0", res.Cap)
	}
}

func TestFit_AllFull(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "plugins.md", 800)  // 200
	c := writeFile(t, dir, "commands.md", 800) // 200
	actions := []domain.PlannedAction{
		contentAction(domain.ComponentPlugins, p),
		contentAction(domain.ComponentCommands, c),
	}
	res, err := Fit(actions, Options{MaxTokens: 1000})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if !res.FitAchieved {
		t.Error("FitAchieved = false, want true")
	}
	for _, d := range res.Decisions {
		if d.Mode != ModeFull {
			t.Errorf("component %s: mode = %s, want full", d.Component, d.Mode)
		}
	}
	if len(res.Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(res.Actions))
	}
	if res.TotalTokens != 400 {
		t.Errorf("TotalTokens = %d, want 400", res.TotalTokens)
	}
}

func TestFit_OmitLowestPriority(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "plugins.md", 800)  // 200
	c := writeFile(t, dir, "commands.md", 800) // 200
	s := writeFile(t, dir, "skills.md", 800)   // 200
	actions := []domain.PlannedAction{
		contentAction(domain.ComponentPlugins, p),
		contentAction(domain.ComponentCommands, c),
		contentAction(domain.ComponentSkills, s),
	}
	// Full total = 600. cap = 250. None of plugins/commands/skills is
	// summarizable, so pass 1 is a no-op and pass 2 omits lowest priority
	// first: plugins → 400 > 250; commands → 200 ≤ 250. Skills stay full.
	res, err := Fit(actions, Options{MaxTokens: 250})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if !res.FitAchieved {
		t.Error("FitAchieved = false, want true")
	}
	m := modesFrom(res)
	if m[domain.ComponentPlugins] != ModeOmit {
		t.Errorf("plugins: %s, want omit", m[domain.ComponentPlugins])
	}
	if m[domain.ComponentCommands] != ModeOmit {
		t.Errorf("commands: %s, want omit", m[domain.ComponentCommands])
	}
	if m[domain.ComponentSkills] != ModeFull {
		t.Errorf("skills: %s, should stay full (higher priority than commands)", m[domain.ComponentSkills])
	}
	// Omitted actions must be filtered out; kept components stay.
	seen := make(map[domain.ComponentID]bool)
	for _, a := range res.Actions {
		seen[a.Component] = true
	}
	if seen[domain.ComponentPlugins] || seen[domain.ComponentCommands] {
		t.Errorf("omitted components should not appear in actions: %+v", seen)
	}
	if !seen[domain.ComponentSkills] {
		t.Error("skills action should be kept in full")
	}
}

func TestFit_SummaryBeforeOmit(t *testing.T) {
	dir := t.TempDir()
	m1 := writeFile(t, dir, "memory.md", 800) // 200
	r1 := writeFile(t, dir, "rules.md", 800)  // 200
	actions := []domain.PlannedAction{
		contentAction(domain.ComponentMemory, m1),
		contentAction(domain.ComponentRules, r1),
	}
	// Full total = 400. cap = 300: halving memory → 300 ≤ 300, so memory is
	// summarized and nothing is omitted.
	res, err := Fit(actions, Options{MaxTokens: 300})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if !res.FitAchieved {
		t.Error("FitAchieved = false, want true")
	}
	m := modesFrom(res)
	if m[domain.ComponentMemory] != ModeSummary {
		t.Errorf("memory: %s, want summary", m[domain.ComponentMemory])
	}
	if m[domain.ComponentRules] != ModeFull {
		t.Errorf("rules: %s, want full", m[domain.ComponentRules])
	}
	for _, d := range res.Decisions {
		if d.Mode == ModeOmit {
			t.Errorf("no component should be omitted, %s was", d.Component)
		}
	}
	if res.TotalTokens != 300 {
		t.Errorf("TotalTokens = %d, want 300 (summary 100 + full 200)", res.TotalTokens)
	}
	// Summarized actions are KEPT, tagged Mode="summary" so installers render
	// the condensed variant.
	if len(res.Actions) != 2 {
		t.Fatalf("expected both actions kept, got %d", len(res.Actions))
	}
	for _, a := range res.Actions {
		switch a.Component {
		case domain.ComponentMemory:
			if a.Mode != string(ModeSummary) {
				t.Errorf("memory action Mode = %q, want %q", a.Mode, ModeSummary)
			}
		case domain.ComponentRules:
			if a.Mode != "" {
				t.Errorf("rules action Mode = %q, want empty (full)", a.Mode)
			}
		}
	}
}

func TestFit_SummarySkipActionUpgradedToUpdate(t *testing.T) {
	dir := t.TempDir()
	m1 := writeFile(t, dir, "memory.md", 800) // 200
	action := contentAction(domain.ComponentMemory, m1)
	action.Action = domain.ActionSkip // full content already on disk
	res, err := Fit([]domain.PlannedAction{action}, Options{MaxTokens: 100})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if len(res.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(res.Actions))
	}
	got := res.Actions[0]
	if got.Mode != string(ModeSummary) {
		t.Errorf("Mode = %q, want summary", got.Mode)
	}
	if got.Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want update (disk has full content, not the summary)", got.Action)
	}
}

func TestFit_NonSummarizableGoesStraightToOmit(t *testing.T) {
	dir := t.TempDir()
	a1 := writeFile(t, dir, "cmds.md", 800) // 200
	res, err := Fit([]domain.PlannedAction{
		contentAction(domain.ComponentCommands, a1),
	}, Options{MaxTokens: 100})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if res.Decisions[0].Mode != ModeOmit {
		t.Errorf("commands Mode = %s, want omit (commands has no summary render)", res.Decisions[0].Mode)
	}
	if len(res.Actions) != 0 {
		t.Errorf("omitted action should be dropped, got %d", len(res.Actions))
	}
}

func TestFit_AgentsSummarizable(t *testing.T) {
	dir := t.TempDir()
	a1 := writeFile(t, dir, "agents.md", 800) // 200 tokens full
	res, err := Fit([]domain.PlannedAction{
		contentAction(domain.ComponentAgents, a1),
	}, Options{
		MaxTokens:     150,
		SummaryTokens: map[domain.ComponentID]int{domain.ComponentAgents: 40},
	})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if res.Decisions[0].Mode != ModeSummary {
		t.Fatalf("agents Mode = %s, want summary", res.Decisions[0].Mode)
	}
	if len(res.Actions) != 1 || res.Actions[0].Mode != "summary" {
		t.Errorf("summarized agents action should be kept with Mode=summary, got %+v", res.Actions)
	}
}

func TestFit_SummaryTokensOverride(t *testing.T) {
	dir := t.TempDir()
	m1 := writeFile(t, dir, "memory.md", 800) // 200 full
	// tokens/2 would be 100 > cap 60, but the real summary render is 40 ≤ 60.
	res, err := Fit([]domain.PlannedAction{
		contentAction(domain.ComponentMemory, m1),
	}, Options{
		MaxTokens:     60,
		SummaryTokens: map[domain.ComponentID]int{domain.ComponentMemory: 40},
	})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if !res.FitAchieved {
		t.Error("FitAchieved = false, want true with real summary counts")
	}
	if res.Decisions[0].Mode != ModeSummary {
		t.Errorf("Mode = %s, want summary", res.Decisions[0].Mode)
	}
	if res.TotalTokens != 40 {
		t.Errorf("TotalTokens = %d, want 40 (real summary count)", res.TotalTokens)
	}
}

func TestComponentPriority_EfficiencyDropsNearlyLast(t *testing.T) {
	effRank := priorityRank(domain.ComponentEfficiency)
	if effRank <= priorityRank(domain.ComponentAgents) {
		t.Error("efficiency must rank above agents (drop later)")
	}
	if effRank >= priorityRank(domain.ComponentBrand) {
		t.Error("efficiency must rank below brand (drop before brand)")
	}
	if effRank >= priorityRank(domain.ComponentPermissions) {
		t.Error("efficiency must rank below permissions")
	}
}

func TestFit_UsesComponentTokenOverrides(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "planned-new-file.md")
	actions := []domain.PlannedAction{
		contentAction(domain.ComponentMemory, missing),
	}
	res, err := Fit(actions, Options{
		MaxTokens: 100,
		ComponentTokens: map[domain.ComponentID]int{
			domain.ComponentMemory: 200,
		},
	})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if res.Decisions[0].Tokens != 200 {
		t.Errorf("Tokens = %d, want override value 200", res.Decisions[0].Tokens)
	}
	if res.Decisions[0].Mode != ModeSummary {
		t.Errorf("Mode = %s, want summary", res.Decisions[0].Mode)
	}
	if len(res.Actions) != 1 || res.Actions[0].Mode != string(ModeSummary) {
		t.Errorf("summary action should be kept with Mode set, got %+v", res.Actions)
	}
}

func TestFit_OperationalComponentsKept(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "plugins.md", 800) // 200 tokens
	actions := []domain.PlannedAction{
		contentAction(domain.ComponentPlugins, p),
		{
			ID:          "cleanup-stale",
			Agent:       domain.AgentOpenCode,
			Component:   domain.ComponentCleanup,
			Action:      domain.ActionDelete,
			TargetPath:  filepath.Join(dir, "stale.md"),
			Description: "stale file cleanup",
		},
		{
			ID:          "hooks-1",
			Agent:       domain.AgentClaudeCode,
			Component:   domain.ComponentHooks,
			Action:      domain.ActionCreate,
			TargetPath:  "",
			Description: "hooks install",
		},
		{
			ID:          "teams-1",
			Agent:       domain.AgentClaudeCode,
			Component:   domain.ComponentAgentTeams,
			Action:      domain.ActionCreate,
			TargetPath:  "",
			Description: "agent teams opt-in",
		},
	}
	// cap = 1 forces plugins to be omitted, but operational components are
	// never content-truncated and must stay full with their actions kept.
	res, err := Fit(actions, Options{MaxTokens: 1})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if !res.FitAchieved {
		t.Error("FitAchieved = false, want true")
	}
	m := modesFrom(res)
	if m[domain.ComponentPlugins] != ModeOmit {
		t.Errorf("plugins: %s, want omit", m[domain.ComponentPlugins])
	}
	for _, op := range []domain.ComponentID{domain.ComponentCleanup, domain.ComponentHooks, domain.ComponentAgentTeams} {
		if m[op] != ModeFull {
			t.Errorf("%s: %s, want full", op, m[op])
		}
	}
	seen := make(map[domain.ComponentID]bool)
	for _, a := range res.Actions {
		seen[a.Component] = true
	}
	for _, op := range []domain.ComponentID{domain.ComponentCleanup, domain.ComponentHooks, domain.ComponentAgentTeams} {
		if !seen[op] {
			t.Errorf("operational action %s missing from result actions", op)
		}
	}
	if seen[domain.ComponentPlugins] {
		t.Error("plugins action should be filtered out")
	}
}

func TestFit_MissingFiles(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.md")
	actions := []domain.PlannedAction{
		contentAction(domain.ComponentPlugins, missing),
	}
	res, err := Fit(actions, Options{MaxTokens: 100})
	if err != nil {
		t.Fatalf("Fit with missing file should not error: %v", err)
	}
	if res.TotalTokens != 0 {
		t.Errorf("TotalTokens = %d, want 0 for missing file", res.TotalTokens)
	}
	if !res.FitAchieved {
		t.Error("FitAchieved = false, want true (0 tokens ≤ cap)")
	}
	if len(res.Actions) != 1 {
		t.Errorf("expected the action kept in full, got %d actions", len(res.Actions))
	}
	if res.Decisions[0].Mode != ModeFull {
		t.Errorf("mode = %s, want full", res.Decisions[0].Mode)
	}
}

func TestFitResult_ActionsFiltered(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "plugins.md", 800)  // 200
	c := writeFile(t, dir, "commands.md", 800) // 200
	actions := []domain.PlannedAction{
		contentAction(domain.ComponentPlugins, p),
		contentAction(domain.ComponentCommands, c),
	}
	// cap = 50: summarizing both → 200 > 50; omitting plugins → 100 > 50;
	// omitting commands → 0 ≤ 50. Both omitted → no actions remain.
	res, err := Fit(actions, Options{MaxTokens: 50})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if !res.FitAchieved {
		t.Error("FitAchieved = false, want true")
	}
	if len(res.Actions) != 0 {
		t.Errorf("expected 0 actions after omitting all content, got %d: %+v", len(res.Actions), res.Actions)
	}
	for _, d := range res.Decisions {
		if d.Mode != ModeOmit {
			t.Errorf("component %s: %s, want omit", d.Component, d.Mode)
		}
	}
	if res.TotalTokens != 0 {
		t.Errorf("TotalTokens = %d, want 0", res.TotalTokens)
	}
}

func TestPersist_Load_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "plugins.md", 800)  // 200
	c := writeFile(t, dir, "commands.md", 800) // 200
	actions := []domain.PlannedAction{
		contentAction(domain.ComponentPlugins, p),
		contentAction(domain.ComponentCommands, c),
	}
	res, err := Fit(actions, Options{MaxTokens: 300, Model: "claude-sonnet-4"})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if err := Persist(dir, res); err != nil {
		t.Fatalf("Persist: %v", err)
	}

	// The sidecar lives under .squadai/.
	if _, err := os.Stat(filepath.Join(dir, ".squadai", ".applied-budget.json")); err != nil {
		t.Errorf("budget file not created: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded nil result")
	}
	if loaded.Model != res.Model {
		t.Errorf("Model = %q, want %q", loaded.Model, res.Model)
	}
	if loaded.Cap != res.Cap {
		t.Errorf("Cap = %d, want %d", loaded.Cap, res.Cap)
	}
	if loaded.TotalTokens != res.TotalTokens {
		t.Errorf("TotalTokens = %d, want %d", loaded.TotalTokens, res.TotalTokens)
	}
	if loaded.FitAchieved != res.FitAchieved {
		t.Errorf("FitAchieved = %v, want %v", loaded.FitAchieved, res.FitAchieved)
	}
	if len(loaded.Decisions) != len(res.Decisions) {
		t.Fatalf("Decisions len = %d, want %d", len(loaded.Decisions), len(res.Decisions))
	}
	for i := range res.Decisions {
		if loaded.Decisions[i] != res.Decisions[i] {
			t.Errorf("Decision[%d] = %+v, want %+v", i, loaded.Decisions[i], res.Decisions[i])
		}
	}
	// Actions are not persisted.
	if len(loaded.Actions) != 0 {
		t.Errorf("loaded Actions len = %d, want 0 (not persisted)", len(loaded.Actions))
	}
}

func TestPersist_NoFile(t *testing.T) {
	dir := t.TempDir()
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load on missing file should not error: %v", err)
	}
	if loaded != nil {
		t.Errorf("expected nil result for missing file, got %+v", loaded)
	}
}

func TestDetectDrift_NoPersistedBudget(t *testing.T) {
	dir := t.TempDir()
	drift, err := DetectDrift(dir, []domain.PlannedAction{
		contentAction(domain.ComponentPlugins, "/whatever"),
	}, Options{Model: "claude-sonnet-4"})
	if err != nil {
		t.Fatalf("DetectDrift: %v", err)
	}
	if drift {
		t.Error("drift = true, want false when no prior budget exists")
	}
}

func TestDetectDrift_ModelChanged(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "plugins.md", 800)
	res, err := Fit([]domain.PlannedAction{contentAction(domain.ComponentPlugins, p)}, Options{MaxTokens: 1000, Model: "claude-sonnet-4"})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if err := Persist(dir, res); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	drift, err := DetectDrift(dir, []domain.PlannedAction{
		contentAction(domain.ComponentPlugins, p),
	}, Options{Model: "gpt-4o"})
	if err != nil {
		t.Fatalf("DetectDrift: %v", err)
	}
	if !drift {
		t.Error("drift = false, want true when model changed")
	}
}

func TestDetectDrift_ProfileChanged(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "plugins.md", 800)
	res, err := Fit([]domain.PlannedAction{contentAction(domain.ComponentPlugins, p)},
		Options{MaxTokens: 1000, Model: "claude-sonnet-4", Profile: "default"})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if err := Persist(dir, res); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	drift, err := DetectDrift(dir, []domain.PlannedAction{
		contentAction(domain.ComponentPlugins, p),
	}, Options{Model: "claude-sonnet-4", Profile: "cheap"})
	if err != nil {
		t.Fatalf("DetectDrift: %v", err)
	}
	if !drift {
		t.Error("drift = false, want true when the active profile changed")
	}

	same, err := DetectDrift(dir, []domain.PlannedAction{
		contentAction(domain.ComponentPlugins, p),
	}, Options{Model: "claude-sonnet-4", Profile: "default"})
	if err != nil {
		t.Fatalf("DetectDrift: %v", err)
	}
	if same {
		t.Error("drift = true, want false when the profile is unchanged")
	}
}

func TestDetectDrift_ComponentSetChanged(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "plugins.md", 800)
	c := writeFile(t, dir, "commands.md", 800)
	res, err := Fit([]domain.PlannedAction{
		contentAction(domain.ComponentPlugins, p),
		contentAction(domain.ComponentCommands, c),
	}, Options{MaxTokens: 1000, Model: "claude-sonnet-4"})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if err := Persist(dir, res); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	s := writeFile(t, dir, "skills.md", 800)
	// Same model, but commands replaced by skills → component set differs.
	drift, err := DetectDrift(dir, []domain.PlannedAction{
		contentAction(domain.ComponentPlugins, p),
		contentAction(domain.ComponentSkills, s),
	}, Options{Model: "claude-sonnet-4"})
	if err != nil {
		t.Fatalf("DetectDrift: %v", err)
	}
	if !drift {
		t.Error("drift = false, want true when component set changed (commands → skills)")
	}
}

func TestDetectDrift_NoDriftWhenSame(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "plugins.md", 800)
	c := writeFile(t, dir, "commands.md", 800)
	res, err := Fit([]domain.PlannedAction{
		contentAction(domain.ComponentPlugins, p),
		contentAction(domain.ComponentCommands, c),
	}, Options{MaxTokens: 1000, Model: "claude-sonnet-4"})
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if err := Persist(dir, res); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	// Same model + same content component set → no drift, even though the
	// underlying files changed size (drift is structural, not content-based).
	p2 := writeFile(t, dir, "plugins2.md", 400)
	c2 := writeFile(t, dir, "commands2.md", 400)
	drift, err := DetectDrift(dir, []domain.PlannedAction{
		contentAction(domain.ComponentPlugins, p2),
		contentAction(domain.ComponentCommands, c2),
	}, Options{Model: "claude-sonnet-4"})
	if err != nil {
		t.Fatalf("DetectDrift: %v", err)
	}
	if drift {
		t.Error("drift = true, want false when model and component set match")
	}
}
