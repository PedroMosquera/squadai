package planner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/codex"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/PedroMosquera/squadai/internal/marker"
	"github.com/PedroMosquera/squadai/internal/pipeline"
)

func cleanupCfg(adapters map[string]bool) *domain.MergedConfig {
	cfg := &domain.MergedConfig{
		Mode:     domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentEfficiency): {Enabled: false},
			string(domain.ComponentMemory):     {Enabled: true},
		},
	}
	for id, on := range adapters {
		cfg.Adapters[id] = domain.AdapterConfig{Enabled: on}
	}
	return cfg
}

func planAndApply(t *testing.T, cfg *domain.MergedConfig, adapters []domain.Adapter, home, project string) []domain.PlannedAction {
	t.Helper()
	p := New()
	actions, err := p.Plan(cfg, adapters, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	report, err := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, cfg.Copilot, nil).Execute(actions)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !report.Success {
		for _, s := range report.Steps {
			if s.Error != "" {
				t.Fatalf("step %s failed: %s", s.Action.ID, s.Error)
			}
		}
	}
	return actions
}

func actionFor(actions []domain.PlannedAction, path string) *domain.PlannedAction {
	for i := range actions {
		if actions[i].TargetPath == path && actions[i].Component == domain.ComponentCleanup {
			return &actions[i]
		}
	}
	return nil
}

// seedSquadAICreatedFile simulates a file a previous apply created that holds
// only SquadAI content, which is the one case cleanup may delete outright.
func seedSquadAICreatedFile(project, path string) error {
	if err := os.WriteFile(path, []byte(marker.InjectSection("", "memory", "managed content")), 0644); err != nil {
		return err
	}
	rel, err := filepath.Rel(project, path)
	if err != nil {
		return err
	}
	return managed.TrackCreatedFile(project, rel)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestStaleCleanup_DisabledCodex_KeepsUserConfigTOMLAndStripsSquadAITables(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cx := codex.New()
	configPath := cx.SettingsPath(home)

	userPart := "model = \"o3\"\napproval_policy = \"on-request\"\n\n[profiles.work]\nmodel = \"gpt-5\"\n"
	squadBlock := "[mcp_servers.context7]\ncommand = \"npx\"\nargs = [\"-y\", \"@upstash/context7-mcp\"]"
	writeFile(t, configPath, marker.InjectHashSection(userPart, "mcp", squadBlock))

	actions := planAndApply(t, cleanupCfg(map[string]bool{"codex": false}), []domain.Adapter{cx}, home, project)

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("user config.toml must survive disabling codex: %v", err)
	}
	if !strings.Contains(string(got), userPart) {
		t.Errorf("user keys lost; got:\n%s", got)
	}
	if strings.Contains(string(got), "mcp_servers.context7") || strings.Contains(string(got), "squadai:mcp") {
		t.Errorf("squadai MCP tables still present; got:\n%s", got)
	}
	a := actionFor(actions, configPath)
	if a == nil || a.Action == domain.ActionDelete || !strings.HasPrefix(a.Description, "strip") {
		t.Errorf("plan must describe a strip for %s, got %+v", configPath, a)
	}
}

func TestStaleCleanup_DisabledCodex_StripsMarkerBlockFromUserMarkdown(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cx := codex.New()
	promptPath := cx.SystemPromptFile(home)

	userText := "# My global Codex notes\n\nAlways run the linter.\n"
	writeFile(t, promptPath, marker.InjectSection(userText, "memory:codex", "squadai memory protocol"))

	actions := planAndApply(t, cleanupCfg(map[string]bool{"codex": false}), []domain.Adapter{cx}, home, project)

	got, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("user AGENTS.md must survive disabling codex: %v", err)
	}
	if !strings.HasPrefix(string(got), strings.TrimRight(userText, "\n")) {
		t.Errorf("user text changed; got:\n%s", got)
	}
	if strings.Contains(string(got), "squadai") {
		t.Errorf("squadai marker block still present; got:\n%s", got)
	}
	if a := actionFor(actions, promptPath); a == nil || a.Action != domain.ActionUpdate {
		t.Errorf("plan must strip (update) %s, got %+v", promptPath, a)
	}
}

func TestStaleCleanup_DisabledClaude_RemovesOnlyOwnedJSONKeys(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cl := claude.New()
	settingsPath := cl.ProjectConfigFile(project)

	writeFile(t, settingsPath, `{"theme": "dark", "permissions": {"allow": ["Bash(go test:*)"]}}`)
	if err := managed.WriteManagedKeys(project, filepath.Join(".claude", "settings.json"), []string{"permissions"}); err != nil {
		t.Fatal(err)
	}

	planAndApply(t, cleanupCfg(map[string]bool{"claude-code": false}), []domain.Adapter{cl}, home, project)

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("user settings.json must survive disabling claude: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got["theme"] != "dark" {
		t.Errorf("user key lost; got %s", data)
	}
	if _, ok := got["permissions"]; ok {
		t.Errorf("squadai-owned key still present; got %s", data)
	}
}

func TestStaleCleanup_UnrecognizedContent_FileKeptUntouched(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cx := codex.New()
	promptPath := cx.SystemPromptFile(home)
	writeFile(t, promptPath, "plain user notes, no markers\n")

	actions := planAndApply(t, cleanupCfg(map[string]bool{"codex": false}), []domain.Adapter{cx}, home, project)

	if a := actionFor(actions, promptPath); a != nil {
		t.Errorf("no cleanup action expected for a file without squadai content, got %+v", a)
	}
	if got, err := os.ReadFile(promptPath); err != nil || string(got) != "plain user notes, no markers\n" {
		t.Errorf("file must be untouched; got %q, err %v", got, err)
	}
}

func TestStaleCleanup_FileSquadAICreated_IsDeletedAndForgotten(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cx := codex.New()
	rulesPath := cx.ProjectRulesFile(project)

	planAndApply(t, cleanupCfg(map[string]bool{"codex": true}), []domain.Adapter{cx}, home, project)
	if _, err := os.Stat(rulesPath); err != nil {
		t.Fatalf("test premise: apply should create %s: %v", rulesPath, err)
	}

	actions := planAndApply(t, cleanupCfg(map[string]bool{"codex": false}), []domain.Adapter{cx}, home, project)

	if a := actionFor(actions, rulesPath); a == nil || a.Action != domain.ActionDelete {
		t.Errorf("plan must delete %s, got %+v", rulesPath, a)
	}
	if _, err := os.Stat(rulesPath); !os.IsNotExist(err) {
		t.Errorf("expected %s deleted, stat err = %v", rulesPath, err)
	}
	created, err := managed.ListCreatedFiles(project)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range created {
		if f == "AGENTS.md" {
			t.Errorf("sidecar still lists deleted AGENTS.md as created: %v", created)
		}
	}
}

func TestStaleCleanup_FileSquadAICreated_UserAddedText_IsStrippedNotDeleted(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cx := codex.New()
	rulesPath := cx.ProjectRulesFile(project)

	planAndApply(t, cleanupCfg(map[string]bool{"codex": true}), []domain.Adapter{cx}, home, project)
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("test premise: apply should create %s: %v", rulesPath, err)
	}
	writeFile(t, rulesPath, string(data)+"\n## Team notes\nkeep me\n")

	planAndApply(t, cleanupCfg(map[string]bool{"codex": false}), []domain.Adapter{cx}, home, project)

	got, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("file with user text must survive: %v", err)
	}
	if !strings.Contains(string(got), "## Team notes\nkeep me") || strings.Contains(string(got), "squadai") {
		t.Errorf("expected only user text left; got:\n%s", got)
	}
}

// Sidecars written before created-file tracking, or a user who pasted a block
// into their own file, look like this: never delete without a creation record.
func TestStaleCleanup_OnlySquadAIContentWithoutCreationRecord_FileKept(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cx := codex.New()
	promptPath := cx.SystemPromptFile(home)
	writeFile(t, promptPath, marker.InjectSection("", "memory:codex", "squadai memory protocol"))

	actions := planAndApply(t, cleanupCfg(map[string]bool{"codex": false}), []domain.Adapter{cx}, home, project)

	if a := actionFor(actions, promptPath); a == nil || a.Action != domain.ActionUpdate {
		t.Errorf("plan must strip, not delete, %s; got %+v", promptPath, a)
	}
	if _, err := os.Stat(promptPath); err != nil {
		t.Errorf("file without a creation record must be kept: %v", err)
	}
}

func TestStaleCleanup_RenderStrip_ShowsUserContentKept(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cx := codex.New()
	promptPath := cx.SystemPromptFile(home)
	userText := "# notes\n\nmine\n"
	writeFile(t, promptPath, marker.InjectSection(userText, "memory:codex", "squadai memory protocol"))

	p := New()
	actions, err := p.Plan(cleanupCfg(map[string]bool{"codex": false}), []domain.Adapter{cx}, home, project)
	if err != nil {
		t.Fatal(err)
	}
	a := actionFor(actions, promptPath)
	if a == nil {
		t.Fatalf("expected a cleanup action for %s", promptPath)
	}
	_, newContent, err := p.RenderAction(*a, home, project)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(newContent)) != strings.TrimSpace(userText) {
		t.Errorf("rendered strip = %q, want the user text only", newContent)
	}
}

// The review screen can sit between plan and apply; text the user adds in that
// window must survive a planned delete.
func TestStaleCleanup_PlannedDelete_UserEditsBeforeApply_FileKept(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	cx := codex.New()
	rulesPath := cx.ProjectRulesFile(project)
	planAndApply(t, cleanupCfg(map[string]bool{"codex": true}), []domain.Adapter{cx}, home, project)

	cfg := cleanupCfg(map[string]bool{"codex": false})
	p := New()
	actions, err := p.Plan(cfg, []domain.Adapter{cx}, home, project)
	if err != nil {
		t.Fatal(err)
	}
	if a := actionFor(actions, rulesPath); a == nil || a.Action != domain.ActionDelete {
		t.Fatalf("test premise: plan should delete %s, got %+v", rulesPath, a)
	}
	data, _ := os.ReadFile(rulesPath)
	writeFile(t, rulesPath, string(data)+"\nlate user note\n")

	if _, err := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, cfg.Copilot, nil).Execute(actions); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(rulesPath)
	if err != nil || !strings.Contains(string(got), "late user note") || strings.Contains(string(got), "squadai") {
		t.Errorf("expected file kept with only the late note; got %q, err %v", got, err)
	}
}
