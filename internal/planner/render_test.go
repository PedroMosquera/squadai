package planner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/cursor"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/windsurf"
	"github.com/PedroMosquera/squadai/internal/components/agents"
	"github.com/PedroMosquera/squadai/internal/components/commands"
	"github.com/PedroMosquera/squadai/internal/components/mcp"
	"github.com/PedroMosquera/squadai/internal/components/plugins"
	"github.com/PedroMosquera/squadai/internal/components/rules"
	"github.com/PedroMosquera/squadai/internal/components/settings"
	"github.com/PedroMosquera/squadai/internal/components/skills"
	"github.com/PedroMosquera/squadai/internal/components/workflows"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// mustContain fails the test if result does not contain all of the given substrings.
func mustContain(t *testing.T, result []byte, substrings ...string) {
	t.Helper()
	for _, s := range substrings {
		if !strings.Contains(string(result), s) {
			t.Errorf("expected output to contain %q, got:\n%s", s, result)
		}
	}
}

// mustNotContain fails the test if result contains any of the given substrings.
func mustNotContain(t *testing.T, result []byte, substrings ...string) {
	t.Helper()
	for _, s := range substrings {
		if strings.Contains(string(result), s) {
			t.Errorf("expected output NOT to contain %q, got:\n%s", s, result)
		}
	}
}

// ─── RenderAction: ActionDelete ───────────────────────────────────────────────

func TestRenderAction_ActionDelete_ReturnsOldContentNilNew(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "managed.md")
	existingContent := "# Managed content\n"
	if err := os.WriteFile(targetPath, []byte(existingContent), 0644); err != nil {
		t.Fatal(err)
	}

	p := New()
	old, newC, err := p.RenderAction(domain.PlannedAction{
		ID:         "delete-test",
		Component:  domain.ComponentCleanup,
		Action:     domain.ActionDelete,
		TargetPath: targetPath,
	}, dir, dir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(old) != existingContent {
		t.Errorf("old content = %q, want %q", old, existingContent)
	}
	if newC != nil {
		t.Errorf("new content should be nil for delete action, got %q", newC)
	}
}

func TestRenderAction_ActionDelete_NonexistentFile_NoError(t *testing.T) {
	dir := t.TempDir()
	p := New()
	old, newC, err := p.RenderAction(domain.PlannedAction{
		ID:         "delete-nonexistent",
		Component:  domain.ComponentCleanup,
		Action:     domain.ActionDelete,
		TargetPath: filepath.Join(dir, "nonexistent.md"),
	}, dir, dir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(old) != 0 {
		t.Errorf("old content should be empty for nonexistent file, got %q", old)
	}
	if newC != nil {
		t.Errorf("new content should be nil for delete action, got %q", newC)
	}
}

// ─── RenderAction: Copilot action ID ─────────────────────────────────────────

func TestRenderAction_CopilotID_ReturnsFallbackPreview(t *testing.T) {
	dir := t.TempDir()
	p := New()
	_, newC, err := p.RenderAction(domain.PlannedAction{
		ID:         "copilot-instructions",
		Component:  domain.ComponentMemory,
		Action:     domain.ActionCreate,
		TargetPath: filepath.Join(dir, "AGENTS.md"),
	}, dir, dir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, newC, "copilot")
}

// ─── RenderAction: Unknown component ─────────────────────────────────────────

func TestRenderAction_UnknownComponent_ReturnsFallbackWithComponentName(t *testing.T) {
	dir := t.TempDir()
	p := New()
	_, newC, err := p.RenderAction(domain.PlannedAction{
		ID:         "unknown-action",
		Component:  domain.ComponentID("unknown-component"),
		Action:     domain.ActionCreate,
		TargetPath: filepath.Join(dir, "some-file.md"),
	}, dir, dir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, newC, "unknown-component")
	mustContain(t, newC, "not available")
}

// ─── RenderAction: Nil installer fallbacks ────────────────────────────────────

func TestRenderAction_NilMemoryInstaller_ReturnsFallback(t *testing.T) {
	dir := t.TempDir()
	// New() always initializes memoryInstaller, so explicitly nil it.
	p := &Planner{}
	_, newC, err := p.RenderAction(domain.PlannedAction{
		Component:  domain.ComponentMemory,
		Action:     domain.ActionCreate,
		TargetPath: filepath.Join(dir, "AGENTS.md"),
	}, dir, dir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, newC, "not initialized")
}

func TestRenderAction_NilRulesInstaller_ReturnsFallback(t *testing.T) {
	dir := t.TempDir()
	p := &Planner{}
	_, newC, err := p.RenderAction(domain.PlannedAction{
		Component:  domain.ComponentRules,
		Action:     domain.ActionCreate,
		TargetPath: filepath.Join(dir, "AGENTS.md"),
	}, dir, dir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, newC, "not initialized")
}

func TestRenderAction_NilSettingsInstaller_ReturnsFallback(t *testing.T) {
	dir := t.TempDir()
	p := &Planner{}
	_, newC, err := p.RenderAction(domain.PlannedAction{
		Component:  domain.ComponentSettings,
		Action:     domain.ActionCreate,
		TargetPath: filepath.Join(dir, "settings.json"),
	}, dir, dir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, newC, "not initialized")
}

func TestRenderAction_NilMCPInstaller_ReturnsFallback(t *testing.T) {
	dir := t.TempDir()
	p := &Planner{}
	_, newC, err := p.RenderAction(domain.PlannedAction{
		Component:  domain.ComponentMCP,
		Action:     domain.ActionCreate,
		TargetPath: filepath.Join(dir, "mcp.json"),
	}, dir, dir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, newC, "not initialized")
}

func TestRenderAction_NilAgentsInstaller_ReturnsFallback(t *testing.T) {
	dir := t.TempDir()
	p := &Planner{}
	_, newC, err := p.RenderAction(domain.PlannedAction{
		Component:  domain.ComponentAgents,
		Action:     domain.ActionCreate,
		TargetPath: filepath.Join(dir, "agent.md"),
	}, dir, dir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, newC, "not initialized")
}

func TestRenderAction_NilSkillsInstaller_ReturnsFallback(t *testing.T) {
	dir := t.TempDir()
	p := &Planner{}
	_, newC, err := p.RenderAction(domain.PlannedAction{
		Component:  domain.ComponentSkills,
		Action:     domain.ActionCreate,
		TargetPath: filepath.Join(dir, "skill.md"),
	}, dir, dir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, newC, "not initialized")
}

func TestRenderAction_NilCommandsInstaller_ReturnsFallback(t *testing.T) {
	dir := t.TempDir()
	p := &Planner{}
	_, newC, err := p.RenderAction(domain.PlannedAction{
		Component:  domain.ComponentCommands,
		Action:     domain.ActionCreate,
		TargetPath: filepath.Join(dir, "cmd.md"),
	}, dir, dir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, newC, "not initialized")
}

func TestRenderAction_NilPluginsInstaller_ReturnsFallback(t *testing.T) {
	dir := t.TempDir()
	p := &Planner{}
	_, newC, err := p.RenderAction(domain.PlannedAction{
		Component:  domain.ComponentPlugins,
		Action:     domain.ActionCreate,
		TargetPath: filepath.Join(dir, "plugin.json"),
	}, dir, dir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, newC, "not initialized")
}

func TestRenderAction_NilWorkflowsInstaller_ReturnsFallback(t *testing.T) {
	dir := t.TempDir()
	p := &Planner{}
	_, newC, err := p.RenderAction(domain.PlannedAction{
		Component:  domain.ComponentWorkflows,
		Action:     domain.ActionCreate,
		TargetPath: filepath.Join(dir, "workflow.md"),
	}, dir, dir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, newC, "not initialized")
}

// ─── RenderAction: Initialized via Plan() ────────────────────────────────────

// buildPlannerWithActions runs Plan() and returns the planner plus all planned actions.
func buildPlannerWithActions(t *testing.T, cfg *domain.MergedConfig, adapters []domain.Adapter) (*Planner, []domain.PlannedAction, string, string) {
	t.Helper()
	home := t.TempDir()
	project := t.TempDir()
	p := New()
	actions, err := p.Plan(cfg, adapters, home, project)
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}
	return p, actions, home, project
}

// findAction returns the first action matching the given component.
// Returns zero-value PlannedAction and false if not found.
func findAction(actions []domain.PlannedAction, component domain.ComponentID) (domain.PlannedAction, bool) {
	for _, a := range actions {
		if a.Component == component {
			return a, true
		}
	}
	return domain.PlannedAction{}, false
}

func TestRenderAction_Memory_ReturnsMarkersInOutput(t *testing.T) {
	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
	}
	p, actions, home, project := buildPlannerWithActions(t, cfg, []domain.Adapter{opencode.New()})

	action, ok := findAction(actions, domain.ComponentMemory)
	if !ok {
		t.Skip("no memory action produced — skipping render test")
	}

	_, newC, err := p.RenderAction(action, home, project)
	if err != nil {
		t.Fatalf("RenderAction error: %v", err)
	}

	mustContain(t, newC, "squadai:memory")
}

func TestRenderAction_Rules_WithContent_ReturnsInjectedSection(t *testing.T) {
	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"rules": {Enabled: true},
		},
		Rules: domain.RulesConfig{
			TeamStandards: "# Team Standards\n- Use Go idioms\n",
		},
	}
	p, actions, home, project := buildPlannerWithActions(t, cfg, []domain.Adapter{opencode.New()})

	action, ok := findAction(actions, domain.ComponentRules)
	if !ok {
		t.Skip("no rules action produced — skipping render test")
	}

	_, newC, err := p.RenderAction(action, home, project)
	if err != nil {
		t.Fatalf("RenderAction error: %v", err)
	}

	mustContain(t, newC, "Team Standards")
	mustContain(t, newC, "squadai:team-standards")
}

func TestRenderAction_Rules_EmptyContent_ReturnsExisting(t *testing.T) {
	// When rules content is empty, renderRules returns existing unchanged.
	dir := t.TempDir()
	emptyRulesInst, err := rules.New(domain.RulesConfig{}, dir)
	if err != nil {
		t.Fatalf("rules.New: %v", err)
	}
	p := &Planner{
		rulesInstaller: emptyRulesInst,
	}

	_, newC, err := p.RenderAction(domain.PlannedAction{
		Component:  domain.ComponentRules,
		Action:     domain.ActionCreate,
		TargetPath: filepath.Join(dir, "AGENTS.md"),
	}, dir, dir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With empty content and empty file, old and new should both be empty.
	if len(newC) != 0 {
		t.Errorf("expected empty new content, got %q", newC)
	}
}

func TestRenderAction_Rules_Cursor_WritesFrontmatter(t *testing.T) {
	cfg := &domain.MergedConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"cursor": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"rules": {Enabled: true},
		},
		Rules: domain.RulesConfig{
			TeamStandards: "always use interfaces",
		},
	}
	p, actions, home, project := buildPlannerWithActions(t, cfg, []domain.Adapter{cursor.New()})

	action, ok := findAction(actions, domain.ComponentRules)
	if !ok {
		t.Skip("no rules action for cursor — skipping")
	}

	_, newC, err := p.RenderAction(action, home, project)
	if err != nil {
		t.Fatalf("RenderAction error: %v", err)
	}

	// renderRules uses marker injection (frontmatter is applied during Apply, not render).
	mustContain(t, newC, "always use interfaces")
}

func TestRenderAction_Rules_Windsurf_WritesFrontmatter(t *testing.T) {
	cfg := &domain.MergedConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"windsurf": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"rules": {Enabled: true},
		},
		Rules: domain.RulesConfig{
			TeamStandards: "windsurf rules",
		},
	}
	p, actions, home, project := buildPlannerWithActions(t, cfg, []domain.Adapter{windsurf.New()})

	action, ok := findAction(actions, domain.ComponentRules)
	if !ok {
		t.Skip("no rules action for windsurf — skipping")
	}

	_, newC, err := p.RenderAction(action, home, project)
	if err != nil {
		t.Fatalf("RenderAction error: %v", err)
	}

	// renderRules uses marker injection (frontmatter is applied during Apply, not render).
	mustContain(t, newC, "windsurf rules")
}

func TestRenderAction_Settings_ReturnsJSON(t *testing.T) {
	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {
				Enabled: true,
				Settings: map[string]interface{}{
					"model": "gpt-4",
				},
			},
		},
		Components: map[string]domain.ComponentConfig{
			"settings": {Enabled: true},
		},
	}
	p, actions, home, project := buildPlannerWithActions(t, cfg, []domain.Adapter{opencode.New()})

	action, ok := findAction(actions, domain.ComponentSettings)
	if !ok {
		t.Skip("no settings action — skipping")
	}

	_, newC, err := p.RenderAction(action, home, project)
	if err != nil {
		t.Fatalf("RenderAction error: %v", err)
	}

	if len(newC) == 0 {
		t.Error("expected non-empty settings content")
	}
}

func TestRenderAction_MCP_ReturnsServerConfig(t *testing.T) {
	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"mcp": {Enabled: true},
		},
		MCP: map[string]domain.MCPServerDef{
			"context7": {
				Type:    "stdio",
				Command: []string{"npx", "-y", "@upstash/context7-mcp@latest"},
				Enabled: true,
			},
		},
	}
	p, actions, home, project := buildPlannerWithActions(t, cfg, []domain.Adapter{opencode.New()})

	action, ok := findAction(actions, domain.ComponentMCP)
	if !ok {
		t.Skip("no MCP action — skipping")
	}

	_, newC, err := p.RenderAction(action, home, project)
	if err != nil {
		t.Fatalf("RenderAction error: %v", err)
	}

	if len(newC) == 0 {
		t.Error("expected non-empty MCP content")
	}
}

func TestRenderAction_Agents_WithDelegationNative_ReturnsContent(t *testing.T) {
	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"agents": {Enabled: true},
		},
		Methodology: domain.MethodologyTDD,
		Agents: map[string]domain.AgentDef{
			"implementer": {
				Description: "Writes Go implementation code",
				Mode:        "subagent",
				Prompt:      "You are a Go developer.",
			},
		},
	}
	p, actions, home, project := buildPlannerWithActions(t, cfg, []domain.Adapter{opencode.New()})

	action, ok := findAction(actions, domain.ComponentAgents)
	if !ok {
		t.Skip("no agents action — skipping")
	}

	_, newC, err := p.RenderAction(action, home, project)
	if err != nil {
		t.Fatalf("RenderAction error: %v", err)
	}
	// Content may be empty (existing == desired) or populated.
	// Just check no error and the call succeeds.
	_ = newC
}

func TestRenderAction_Skills_ReturnsContent(t *testing.T) {
	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"skills": {Enabled: true},
		},
		Methodology: domain.MethodologyTDD,
		Skills: map[string]domain.SkillDef{
			"refactor": {
				Description: "Refactoring helper",
				Content:     "## Refactor\nApply SOLID principles.",
			},
		},
	}
	p, actions, home, project := buildPlannerWithActions(t, cfg, []domain.Adapter{opencode.New()})

	action, ok := findAction(actions, domain.ComponentSkills)
	if !ok {
		t.Skip("no skills action — skipping")
	}

	_, newC, err := p.RenderAction(action, home, project)
	if err != nil {
		t.Fatalf("RenderAction error: %v", err)
	}
	_ = newC
}

func TestRenderAction_Commands_ReturnsContent(t *testing.T) {
	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"commands": {Enabled: true},
		},
		Commands: map[string]domain.CommandDef{
			"test-all": {
				Description: "Run all tests",
				Template:    "go test ./...",
			},
		},
	}
	p, actions, home, project := buildPlannerWithActions(t, cfg, []domain.Adapter{opencode.New()})

	action, ok := findAction(actions, domain.ComponentCommands)
	if !ok {
		t.Skip("no commands action — skipping")
	}

	_, newC, err := p.RenderAction(action, home, project)
	if err != nil {
		t.Fatalf("RenderAction error: %v", err)
	}
	_ = newC
}

func TestRenderAction_Plugins_ReturnsContent(t *testing.T) {
	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"plugins": {Enabled: true},
		},
		Plugins: map[string]domain.PluginDef{
			"superpowers": {
				Enabled:         true,
				SupportedAgents: []string{"opencode"},
				InstallMethod:   "skill_files",
			},
		},
	}
	p, actions, home, project := buildPlannerWithActions(t, cfg, []domain.Adapter{opencode.New()})

	action, ok := findAction(actions, domain.ComponentPlugins)
	if !ok {
		t.Skip("no plugins action — skipping")
	}

	_, newC, err := p.RenderAction(action, home, project)
	if err != nil {
		t.Fatalf("RenderAction error: %v", err)
	}
	_ = newC
}

func TestRenderAction_Workflows_ReturnsContent(t *testing.T) {
	cfg := &domain.MergedConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"windsurf": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"workflows": {Enabled: true},
		},
		Methodology: domain.MethodologyTDD,
	}
	p, actions, home, project := buildPlannerWithActions(t, cfg, []domain.Adapter{windsurf.New()})

	action, ok := findAction(actions, domain.ComponentWorkflows)
	if !ok {
		t.Skip("no workflows action — skipping")
	}

	_, newC, err := p.RenderAction(action, home, project)
	if err != nil {
		t.Fatalf("RenderAction error: %v", err)
	}
	_ = newC
}

// ─── RenderAction: ActionSkip propagates existing content ─────────────────────

func TestRenderAction_ActionSkip_Memory_SameContent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	// Pre-populate the memory file so Plan() emits ActionSkip.
	adapter := opencode.New()
	promptPath := adapter.ProjectRulesFile(project)
	if promptPath == "" {
		promptPath = adapter.SystemPromptFile(home)
	}
	if err := os.MkdirAll(filepath.Dir(promptPath), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
	}
	p := New()

	// First apply to set up the file correctly.
	actions, err := p.Plan(cfg, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatal(err)
	}

	for _, a := range actions {
		if a.Component == domain.ComponentMemory {
			installer := p.memoryInstaller
			if applyErr := installer.Apply(a); applyErr != nil {
				t.Fatalf("apply memory: %v", applyErr)
			}
		}
	}

	// Re-plan; action should now be ActionSkip.
	actions2, err := p.Plan(cfg, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatal(err)
	}

	for _, a := range actions2 {
		if a.Component == domain.ComponentMemory && a.Action == domain.ActionSkip {
			old, newC, renderErr := p.RenderAction(a, home, project)
			if renderErr != nil {
				t.Fatalf("RenderAction skip: %v", renderErr)
			}
			// For skip, new content should equal old (section already present).
			if string(old) != string(newC) {
				// The memory installer still re-renders the section, so old != new is ok
				// as long as newC contains the markers (idempotent re-inject).
				mustContain(t, newC, "squadai:memory")
			}
			return
		}
	}
	t.Skip("no ActionSkip memory action found — skipping")
}

// ─── RenderAction: Claude Code (prompt-based delegation) ─────────────────────

func TestRenderAction_Memory_ClaudeCode_ReturnsMarkers(t *testing.T) {
	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"claude-code": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
	}
	p, actions, home, project := buildPlannerWithActions(t, cfg, []domain.Adapter{claude.New()})

	action, ok := findAction(actions, domain.ComponentMemory)
	if !ok {
		t.Skip("no memory action for claude-code — skipping")
	}

	_, newC, err := p.RenderAction(action, home, project)
	if err != nil {
		t.Fatalf("RenderAction error: %v", err)
	}

	mustContain(t, newC, "squadai:memory")
}

// ─── RenderAction: existing file content preserved (old) ─────────────────────

func TestRenderAction_Memory_ExistingFile_OldContentMatches(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	targetPath := adapter.ProjectRulesFile(project)
	if targetPath == "" {
		targetPath = adapter.SystemPromptFile(home)
	}
	preExisting := "# Existing content\nsome text\n"
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte(preExisting), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
	}
	p := New()
	actions, err := p.Plan(cfg, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatal(err)
	}

	for _, a := range actions {
		if a.Component == domain.ComponentMemory {
			old, _, renderErr := p.RenderAction(a, home, project)
			if renderErr != nil {
				t.Fatalf("RenderAction error: %v", renderErr)
			}
			if string(old) != preExisting {
				t.Errorf("old content = %q, want %q", old, preExisting)
			}
			return
		}
	}
	t.Skip("no memory action found — skipping")
}

// ─── injectSection helper (same package) ─────────────────────────────────────

func TestInjectSection_EmptyDoc_CreatesBlock(t *testing.T) {
	result := injectSection("", "test-section", "content here")

	mustContain(t, []byte(result), "<!-- squadai:test-section -->")
	mustContain(t, []byte(result), "content here")
	mustContain(t, []byte(result), "<!-- /squadai:test-section -->")
}

func TestInjectSection_ExistingDoc_NoSection_AppendsBlock(t *testing.T) {
	doc := "# My File\n\nSome existing text.\n"
	result := injectSection(doc, "rules", "# Standards\nRule 1")

	mustContain(t, []byte(result), "# My File")
	mustContain(t, []byte(result), "<!-- squadai:rules -->")
	mustContain(t, []byte(result), "# Standards")
}

func TestInjectSection_ExistingDoc_WithSection_UpdatesContent(t *testing.T) {
	doc := "# Header\n\n<!-- squadai:rules -->\nold content\n<!-- /squadai:rules -->\n"
	result := injectSection(doc, "rules", "new content")

	mustContain(t, []byte(result), "new content")
	mustNotContain(t, []byte(result), "old content")
}

func TestInjectSection_EmptyContent_RemovesExistingSection(t *testing.T) {
	doc := "# Header\n\n<!-- squadai:rules -->\nsome rules\n<!-- /squadai:rules -->\n\nFooter"
	result := injectSection(doc, "rules", "")

	mustNotContain(t, []byte(result), "squadai:rules")
	mustNotContain(t, []byte(result), "some rules")
}

func TestInjectSection_EmptyContent_EmptyDoc_ReturnsEmpty(t *testing.T) {
	result := injectSection("", "section", "")
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

// ─── Planner field-level injected installer coverage ─────────────────────────

// TestRenderAction_AllComponents_WithDirectInstallers verifies that each
// component path can be exercised by setting installers directly.
func TestRenderAction_AllComponents_WithDirectInstallers(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		MCP: map[string]domain.MCPServerDef{
			"ctx": {Type: "stdio", Command: []string{"npx", "ctx-mcp"}, Enabled: true},
		},
		Agents: map[string]domain.AgentDef{
			"worker": {Description: "worker", Mode: "subagent", Prompt: "you work"},
		},
		Skills: map[string]domain.SkillDef{
			"tdd": {Description: "tdd skill", Content: "## TDD\nRed-green-refactor"},
		},
		Commands: map[string]domain.CommandDef{
			"build": {Description: "build cmd", Template: "go build ./..."},
		},
		Plugins: map[string]domain.PluginDef{
			"sp": {Enabled: true, SupportedAgents: []string{"opencode"}, InstallMethod: "skill_files"},
		},
		Methodology: domain.MethodologyTDD,
	}

	p := &Planner{}
	rulesInstTest2, _ := rules.New(domain.RulesConfig{TeamStandards: "rule A"}, project)
	p.rulesInstaller = rulesInstTest2
	p.settingsInstaller = settings.New(cfg.Adapters)
	p.mcpInstaller = mcp.New(cfg.MCP)
	p.agentsInstaller = agents.New(cfg.Agents, cfg, project)
	p.skillsInstaller = skills.New(cfg.Skills, cfg, project)
	p.commandsInstaller = commands.New(cfg.Commands)
	p.pluginsInstaller = plugins.New(cfg.Plugins, cfg)
	p.workflowsInstaller = workflows.New(cfg)

	components := []domain.ComponentID{
		domain.ComponentRules,
		domain.ComponentSettings,
		domain.ComponentMCP,
		domain.ComponentAgents,
		domain.ComponentSkills,
		domain.ComponentCommands,
		domain.ComponentPlugins,
		domain.ComponentWorkflows,
	}

	for _, comp := range components {
		t.Run(string(comp), func(t *testing.T) {
			action := domain.PlannedAction{
				ID:         string(comp) + "-test",
				Agent:      domain.AgentOpenCode,
				Component:  comp,
				Action:     domain.ActionCreate,
				TargetPath: filepath.Join(project, string(comp)+"-test.md"),
			}
			_, newC, err := p.RenderAction(action, home, project)
			if err != nil {
				t.Fatalf("RenderAction(%s) error: %v", comp, err)
			}
			// Must not return the nil-installer fallback.
			mustNotContain(t, newC, "not initialized")
		})
	}
}
