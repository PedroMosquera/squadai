package plugins

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/domain"
)

func declaredMarketplacePlugin() map[string]domain.PluginDef {
	return map[string]domain.PluginDef{
		"formatter": {
			Enabled:         true,
			SupportedAgents: []string{"claude-code", "opencode"},
			InstallMethod:   "claude_plugin",
			PluginID:        "formatter@team-tools",
			Marketplace:     &domain.PluginMarketplace{Source: "github", Repo: "example-org/claude-plugins"},
		},
	}
}

func planAndApply(t *testing.T, inst *Installer, adapter domain.Adapter, home, project string) []domain.PlannedAction {
	t.Helper()
	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("apply %s: %v", a.ID, err)
		}
	}
	return actions
}

func TestApply_ClaudePluginDeclarationsGoToProjectSettingsAndKeepUserPlugins(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	settingsPath := filepath.Join(project, ".claude", "settings.json")
	writeTestJSON(t, settingsPath, map[string]interface{}{
		"model":          "sonnet",
		"enabledPlugins": map[string]interface{}{"mine@my-market": true},
	})

	planAndApply(t, New(declaredMarketplacePlugin(), &domain.MergedConfig{}), claude.New(), home, project)

	doc := readTestJSONHelper(t, settingsPath)
	wantPlugins := map[string]interface{}{"mine@my-market": true, "formatter@team-tools": true}
	if !reflect.DeepEqual(doc["enabledPlugins"], wantPlugins) {
		t.Errorf("enabledPlugins = %v, want %v", doc["enabledPlugins"], wantPlugins)
	}
	wantMarkets := map[string]interface{}{
		"team-tools": map[string]interface{}{"source": map[string]interface{}{"source": "github", "repo": "example-org/claude-plugins"}},
	}
	if !reflect.DeepEqual(doc["extraKnownMarketplaces"], wantMarkets) {
		t.Errorf("extraKnownMarketplaces = %v, want %v", doc["extraKnownMarketplaces"], wantMarkets)
	}
	if doc["model"] != "sonnet" {
		t.Errorf("user key model was not preserved: %v", doc)
	}
}

func TestApply_RemovingAPluginDeclarationRemovesOnlySquadaisEntries(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	settingsPath := filepath.Join(project, ".claude", "settings.json")
	writeTestJSON(t, settingsPath, map[string]interface{}{
		"enabledPlugins": map[string]interface{}{"mine@my-market": true},
	})
	planAndApply(t, New(declaredMarketplacePlugin(), &domain.MergedConfig{}), claude.New(), home, project)
	if plugins, _ := readTestJSONHelper(t, settingsPath)["enabledPlugins"].(map[string]interface{}); plugins["formatter@team-tools"] != true {
		t.Fatalf("the declared plugin should be enabled before it is removed, got %v", plugins)
	}

	planAndApply(t, New(map[string]domain.PluginDef{}, &domain.MergedConfig{}), claude.New(), home, project)

	doc := readTestJSONHelper(t, settingsPath)
	wantPlugins := map[string]interface{}{"mine@my-market": true}
	if !reflect.DeepEqual(doc["enabledPlugins"], wantPlugins) {
		t.Errorf("enabledPlugins = %v, want only the user's entry %v", doc["enabledPlugins"], wantPlugins)
	}
	if _, ok := doc["extraKnownMarketplaces"]; ok {
		t.Errorf("extraKnownMarketplaces should be gone once squadai's only entry is removed: %v", doc)
	}
}

func TestPlan_ClaudePluginDeclarationsAreCurrentAfterApply(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	inst := New(declaredMarketplacePlugin(), &domain.MergedConfig{})
	applied := planAndApply(t, inst, claude.New(), home, project)
	if len(applied) != 1 || applied[0].TargetPath != filepath.Join(project, ".claude", "settings.json") {
		t.Fatalf("want one action on the project settings, got %+v", applied)
	}

	actions, err := inst.Plan(claude.New(), home, project)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range actions {
		if a.Action != domain.ActionSkip {
			t.Errorf("a second plan should skip, got %s %s", a.Action, a.TargetPath)
		}
	}
	results, err := inst.Verify(claude.New(), home, project)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("verify should report the declared plugin")
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("verify %s failed: %s", r.Check, r.Message)
		}
	}
}

func TestApply_ClaudePluginTheUserDisabledIsAConflictNotAnOverwrite(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	settingsPath := filepath.Join(project, ".claude", "settings.json")
	writeTestJSON(t, settingsPath, map[string]interface{}{
		"enabledPlugins": map[string]interface{}{"formatter@team-tools": false},
	})
	inst := New(declaredMarketplacePlugin(), &domain.MergedConfig{})
	actions, err := inst.Plan(claude.New(), home, project)
	if err != nil || len(actions) != 1 {
		t.Fatalf("plan: %v %+v", err, actions)
	}

	var conflict *domain.ConflictError
	if err := inst.Apply(actions[0]); !errors.As(err, &conflict) || conflict.Conflicts[0].Key != "enabledPlugins.formatter@team-tools" {
		t.Fatalf("want a conflict on the user's entry, got %v", err)
	}
	if got := readTestJSONHelper(t, settingsPath)["enabledPlugins"]; !reflect.DeepEqual(got, map[string]interface{}{"formatter@team-tools": false}) {
		t.Errorf("the user's value must stay, got %v", got)
	}
}

func TestPlan_ClaudePluginDeclarationDoesNothingForOpenCode(t *testing.T) {
	actions, err := New(declaredMarketplacePlugin(), &domain.MergedConfig{}).Plan(opencode.New(), t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 0 {
		t.Errorf("OpenCode has no Claude plugin mechanism, want no actions, got %+v", actions)
	}
}
