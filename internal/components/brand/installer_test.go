package brand

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/cursor"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/pi"
	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// ─── Interface compliance ───────────────────────────────────────────────────

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	inst := New()
	if inst.ID() != domain.ComponentBrand {
		t.Errorf("ID() = %q, want %q", inst.ID(), domain.ComponentBrand)
	}
}

// ─── Per-agent template selection ───────────────────────────────────────────
//
// Banners are ASCII art, so they do not contain the literal agent name. We
// verify the mapping by comparing against the embedded asset files and by
// asserting that the co-branded banners differ from the standalone one.

func TestTemplateForAgentID(t *testing.T) {
	standalone := assets.MustRead("brand/banner-squadai.txt")
	tests := []struct {
		name    string
		agentID domain.AgentID
		asset   string
	}{
		{"opencode", domain.AgentOpenCode, "brand/banner-opencode.txt"},
		{"pi", domain.AgentPi, "brand/banner-pi.txt"},
		{"claude", domain.AgentClaudeCode, "brand/banner-claude-code.txt"},
		{"cursor", domain.AgentCursor, "brand/banner-cursor.txt"},
		{"windsurf", domain.AgentWindsurf, "brand/banner-squadai.txt"},
		{"vscode", domain.AgentVSCodeCopilot, "brand/banner-squadai.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := templateForAgentID(tc.agentID)
			want := assets.MustRead(tc.asset)
			if got != want {
				t.Errorf("templateForAgentID(%q) does not match %q", tc.agentID, tc.asset)
			}
			// Co-branded banners must differ from the standalone banner.
			if tc.agentID == domain.AgentOpenCode || tc.agentID == domain.AgentPi ||
				tc.agentID == domain.AgentClaudeCode || tc.agentID == domain.AgentCursor {
				if got == standalone {
					t.Errorf("templateForAgentID(%q) should be co-branded, not the standalone banner", tc.agentID)
				}
			}
		})
	}
}

func TestTemplateForAgentID_Exported(t *testing.T) {
	if TemplateForAgentID(domain.AgentOpenCode) != templateForAgentID(domain.AgentOpenCode) {
		t.Error("exported TemplateForAgentID should match internal templateForAgentID")
	}
}

func TestProtocolTemplate(t *testing.T) {
	if ProtocolTemplate() != assets.MustRead("brand/banner-squadai.txt") {
		t.Error("ProtocolTemplate should return the standalone squadai banner")
	}
}

// TestTemplateForAgentID_ASCIIOnly verifies all banner content is printable
// ASCII (0x20-0x7E plus newline 0x0A). Some terminal renderers mangle non-ASCII
// inside fenced code blocks, so the banners must stay 7-bit clean.
func TestTemplateForAgentID_ASCIIOnly(t *testing.T) {
	agentIDs := []domain.AgentID{
		domain.AgentOpenCode,
		domain.AgentPi,
		domain.AgentClaudeCode,
		domain.AgentCursor,
		domain.AgentWindsurf,
		domain.AgentVSCodeCopilot,
	}
	for _, id := range agentIDs {
		t.Run(string(id), func(t *testing.T) {
			content := templateForAgentID(id)
			for i, r := range content {
				if r == '\n' {
					continue
				}
				if r < 0x20 || r > 0x7E {
					t.Errorf("non-ASCII byte 0x%X at offset %d in banner for %q", r, i, id)
				}
			}
		})
	}
}

// ─── Project-level target path ──────────────────────────────────────────────

func TestBrandTargetPath_UsesProjectLevel(t *testing.T) {
	adapter := pi.New()
	home := t.TempDir()
	project := t.TempDir()

	target := brandTargetPath(adapter, home, project)
	expected := filepath.Join(project, "AGENTS.md")
	if target != expected {
		t.Errorf("target = %q, want %q", target, expected)
	}
}

func TestBrandTargetPath_EmptyProjectDir_UsesGlobal(t *testing.T) {
	adapter := pi.New()
	home := t.TempDir()

	target := brandTargetPath(adapter, home, "")
	expected := adapter.SystemPromptFile(home)
	if target != expected {
		t.Errorf("target = %q, want %q", target, expected)
	}
}

// ─── Plan ───────────────────────────────────────────────────────────────────

func TestPlan_AdapterUnsupported(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	inst := New()

	actions, err := inst.Plan(&unsupportedAdapter{}, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actions != nil {
		t.Errorf("expected nil actions for unsupported adapter, got %d", len(actions))
	}
}

func TestPlan_CreateNew(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := pi.New()
	inst := New()

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions for Pi (project + global), got %d", len(actions))
	}
	if actions[0].Action != domain.ActionCreate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionCreate)
	}
	expected := filepath.Join(project, "AGENTS.md")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
	// Pi extra presence: the second action targets the global AGENTS.md.
	expectedGlobal := adapter.SystemPromptFile(home)
	if actions[1].TargetPath != expectedGlobal {
		t.Errorf("global TargetPath = %q, want %q", actions[1].TargetPath, expectedGlobal)
	}
	if actions[1].Action != domain.ActionCreate {
		t.Errorf("global Action = %q, want %q", actions[1].Action, domain.ActionCreate)
	}
}

func TestPlan_UpdateExisting(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := pi.New()
	inst := New()

	targetPath := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(targetPath, []byte("# Existing Prompt\n\nSome content.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions for Pi (project + global), got %d", len(actions))
	}
	if actions[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionUpdate)
	}
}

func TestPlan_SkipWhenCurrent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := pi.New()
	inst := New()

	// Write file with the exact brand section Apply would produce.
	targetPath := filepath.Join(project, "AGENTS.md")
	banner := templateForAgentID(adapter.ID())
	content := marker.InjectSection("", SectionID, "```text\n"+banner+"\n```\n")
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions for Pi (project + global), got %d", len(actions))
	}
	if actions[0].Action != domain.ActionSkip {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionSkip)
	}
}

func TestPlan_UpdateWhenStale(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := pi.New()
	inst := New()

	targetPath := filepath.Join(project, "AGENTS.md")
	content := marker.InjectSection("", SectionID, "old banner content")
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions for Pi (project + global), got %d", len(actions))
	}
	if actions[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionUpdate)
	}
}

// ─── Apply ──────────────────────────────────────────────────────────────────

func TestApply_Create(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := pi.New()
	inst := New()

	actions, _ := inst.Plan(adapter, home, project)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	content, _ := os.ReadFile(actions[0].TargetPath)
	if !marker.HasSection(string(content), SectionIDForAgentID(adapter.ID())) {
		t.Error("expected brand marker section in file after apply")
	}
	if !strings.Contains(string(content), "SquadAI") && !strings.Contains(string(content), "squadai") {
		// ASCII art may not contain literal text; fall back to checking the
		// banner bytes are present.
		if !strings.Contains(string(content), templateForAgentID(adapter.ID())) {
			t.Error("expected banner content to be present after apply")
		}
	}
}

func TestApply_Skip(t *testing.T) {
	inst := New()
	// Point at a path that must not be created.
	target := filepath.Join(t.TempDir(), "must-not-exist.md")
	action := domain.PlannedAction{
		Action:     domain.ActionSkip,
		TargetPath: target,
	}
	if err := inst.Apply(action); err != nil {
		t.Fatalf("Skip should succeed, got: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("Skip should not create the target file")
	}
}

func TestApply_FencedBlock(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := pi.New()
	inst := New()

	actions, _ := inst.Plan(adapter, home, project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(actions[0].TargetPath)
	s := string(content)
	banner := templateForAgentID(adapter.ID())

	if !strings.Contains(s, "```text") {
		t.Error("written content should include a ```text opening fence")
	}
	if strings.Count(s, "```") < 2 {
		t.Error("written content should include opening and closing fences")
	}
	if !strings.Contains(s, banner) {
		t.Error("written content should include the raw banner inside the fence")
	}
}

func TestApply_PreservesExistingContent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := pi.New()
	inst := New()

	targetPath := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(targetPath, []byte("# My Custom Prompt\n\nDo not delete this.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	actions, _ := inst.Plan(adapter, home, project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(targetPath)
	s := string(content)
	if !strings.Contains(s, "# My Custom Prompt") {
		t.Error("existing content should be preserved")
	}
	if !strings.Contains(s, "Do not delete this.") {
		t.Error("existing content should be preserved")
	}
	if !marker.HasSection(s, SectionIDForAgentID(adapter.ID())) {
		t.Error("brand section should be injected")
	}
}

func TestApply_SharedAgentsFile_KeepsAdapterSectionsSeparate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	openCode := opencode.New()
	piAdapter := pi.New()
	inst := New()

	openActions, err := inst.Plan(openCode, home, project)
	if err != nil {
		t.Fatal(err)
	}
	if err := inst.Apply(openActions[0]); err != nil {
		t.Fatal(err)
	}

	piActions, err := inst.Plan(piAdapter, home, project)
	if err != nil {
		t.Fatal(err)
	}
	if err := inst.Apply(piActions[0]); err != nil {
		t.Fatal(err)
	}

	targetPath := filepath.Join(project, "AGENTS.md")
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	if !marker.HasSection(s, SectionIDForAgentID(openCode.ID())) {
		t.Fatal("OpenCode brand section missing")
	}
	if !marker.HasSection(s, SectionIDForAgentID(piAdapter.ID())) {
		t.Fatal("Pi brand section missing")
	}

	openActions, err = inst.Plan(openCode, home, project)
	if err != nil {
		t.Fatal(err)
	}
	piActions, err = inst.Plan(piAdapter, home, project)
	if err != nil {
		t.Fatal(err)
	}
	if openActions[0].Action != domain.ActionSkip || piActions[0].Action != domain.ActionSkip {
		t.Fatalf("both adapters should be current, got OpenCode=%s Pi=%s", openActions[0].Action, piActions[0].Action)
	}
}

func TestApply_Idempotent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := pi.New()
	inst := New()

	actions, _ := inst.Plan(adapter, home, project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(actions[0].TargetPath)

	// Second plan — should be Skip.
	actions2, _ := inst.Plan(adapter, home, project)
	if actions2[0].Action != domain.ActionSkip {
		t.Fatalf("second plan should be Skip, got %q", actions2[0].Action)
	}

	second, _ := os.ReadFile(actions[0].TargetPath)
	if string(first) != string(second) {
		t.Error("content should not change on second apply")
	}
}

// ─── Verify ─────────────────────────────────────────────────────────────────

func TestVerify_AllPass(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := pi.New()
	inst := New()

	actions, _ := inst.Plan(adapter, home, project)
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatal(err)
		}
	}

	results, err := inst.Verify(adapter, home, project)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
}

func TestVerify_AdapterUnsupported(t *testing.T) {
	inst := New()
	results, err := inst.Verify(&unsupportedAdapter{}, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for unsupported adapter, got %d", len(results))
	}
}

func TestVerify_FileMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := pi.New()
	inst := New()

	results, err := inst.Verify(adapter, home, project)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("expected brand-file-exists check to fail")
	}
	if results[0].Check != "brand-file-exists" {
		t.Errorf("first check = %q, want %q", results[0].Check, "brand-file-exists")
	}
}

func TestVerify_MarkersMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := pi.New()
	inst := New()

	targetPath := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(targetPath, []byte("no markers here"), 0644); err != nil {
		t.Fatal(err)
	}

	results, _ := inst.Verify(adapter, home, project)

	foundMarkerCheck := false
	for _, r := range results {
		if r.Check == "brand-markers-present" {
			foundMarkerCheck = true
			if r.Passed {
				t.Error("markers-present should fail when no markers")
			}
		}
	}
	if !foundMarkerCheck {
		t.Error("expected brand-markers-present check in results")
	}
}

func TestVerify_ContentStale(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := pi.New()
	inst := New()

	targetPath := filepath.Join(project, "AGENTS.md")
	content := marker.InjectSection("", SectionID, "wrong old banner content")
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results, _ := inst.Verify(adapter, home, project)
	foundContentCheck := false
	for _, r := range results {
		if r.Check == "brand-content-current" {
			foundContentCheck = true
			if r.Passed {
				t.Error("content-current should fail when content is outdated")
			}
		}
	}
	if !foundContentCheck {
		t.Error("expected brand-content-current check in results")
	}
}

// ─── Test helpers ───────────────────────────────────────────────────────────

// unsupportedAdapter is a domain.Adapter that supports no components.
// Used to verify the SupportsComponent early-return paths.
type unsupportedAdapter struct{}

func (a *unsupportedAdapter) ID() domain.AgentID { return "unsupported" }
func (a *unsupportedAdapter) Lane() domain.AdapterLane {
	return domain.LanePersonal
}
func (a *unsupportedAdapter) Detect(_ context.Context, homeDir string) (bool, bool, error) {
	return true, true, nil
}
func (a *unsupportedAdapter) GlobalConfigDir(homeDir string) string       { return "" }
func (a *unsupportedAdapter) SystemPromptFile(homeDir string) string      { return "" }
func (a *unsupportedAdapter) SkillsDir(homeDir string) string             { return "" }
func (a *unsupportedAdapter) SettingsPath(homeDir string) string          { return "" }
func (a *unsupportedAdapter) SupportsComponent(c domain.ComponentID) bool { return false }
func (a *unsupportedAdapter) ProjectConfigFile(projectDir string) string  { return "" }
func (a *unsupportedAdapter) ProjectRulesFile(projectDir string) string   { return "" }
func (a *unsupportedAdapter) ProjectAgentsDir(projectDir string) string   { return "" }
func (a *unsupportedAdapter) ProjectSkillsDir(projectDir string) string   { return "" }
func (a *unsupportedAdapter) ProjectCommandsDir(projectDir string) string { return "" }
func (a *unsupportedAdapter) DelegationStrategy() domain.DelegationStrategy {
	return domain.DelegationSoloAgent
}
func (a *unsupportedAdapter) SupportsSubAgents() bool                   { return false }
func (a *unsupportedAdapter) SubAgentsDir(homeDir string) string        { return "" }
func (a *unsupportedAdapter) SupportsWorkflows() bool                   { return false }
func (a *unsupportedAdapter) WorkflowsDir(projectDir string) string     { return "" }
func (a *unsupportedAdapter) MCPRootKey() string                        { return "mcpServers" }
func (a *unsupportedAdapter) MCPURLKey() string                         { return "url" }
func (a *unsupportedAdapter) MCPConfigPath(projectDir string) string    { return "" }
func (a *unsupportedAdapter) MCPCommandStyle() string                   { return "split" }
func (a *unsupportedAdapter) MCPEnvKey() string                         { return "env" }
func (a *unsupportedAdapter) MCPTypeField(_ domain.MCPServerDef) string { return "" }
func (a *unsupportedAdapter) RulesFrontmatter() string                  { return "" }
func (a *unsupportedAdapter) RulesFileSizeCap() int                     { return 0 }

var _ domain.Adapter = (*unsupportedAdapter)(nil)

// ─── Co-brand round trips ─────────────────────────────────────────────────────

// TestPlanApplyVerify_CoBrandedAgents runs the full plan→apply→verify round
// trip for every co-branded adapter and asserts the co-brand tag lands in the
// target file.
func TestPlanApplyVerify_CoBrandedAgents(t *testing.T) {
	tests := []struct {
		name    string
		adapter domain.Adapter
		tag     string
	}{
		{"claude", claude.New(), "x Claude Code"},
		{"cursor", cursor.New(), "x Cursor"},
		{"opencode", opencode.New(), ""},
		{"pi", pi.New(), "x Pi"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			project := t.TempDir()
			inst := New()

			actions, err := inst.Plan(tc.adapter, home, project)
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if len(actions) == 0 {
				t.Fatal("expected planned actions")
			}
			for _, a := range actions {
				if err := inst.Apply(a); err != nil {
					t.Fatalf("Apply(%s): %v", a.ID, err)
				}
			}

			results, err := inst.Verify(tc.adapter, home, project)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			for _, r := range results {
				if !r.Passed {
					t.Errorf("check %q failed: %s", r.Check, r.Message)
				}
			}

			if tc.tag != "" {
				content, readErr := os.ReadFile(actions[0].TargetPath)
				if readErr != nil {
					t.Fatalf("read target: %v", readErr)
				}
				if !strings.Contains(string(content), tc.tag) {
					t.Errorf("target file missing co-brand tag %q", tc.tag)
				}
			}
		})
	}
}

// TestPiBrand_LandsInProjectAndGlobalAgentsFiles is the explicit Pi extra
// presence check: the banner must land in the project AGENTS.md AND the
// global ~/.pi/agent/AGENTS.md.
func TestPiBrand_LandsInProjectAndGlobalAgentsFiles(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := pi.New()
	inst := New()

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("Apply(%s): %v", a.ID, err)
		}
	}

	banner := assets.MustRead("brand/banner-pi.txt")
	for _, path := range []string{
		filepath.Join(project, "AGENTS.md"),
		filepath.Join(home, ".pi", "agent", "AGENTS.md"),
	} {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("banner target missing: %v", readErr)
		}
		if !strings.Contains(string(content), banner) {
			t.Errorf("%s missing the Pi co-brand banner", path)
		}
		if !marker.HasSection(string(content), SectionIDForAgentID(domain.AgentPi)) {
			t.Errorf("%s missing the scoped brand marker section", path)
		}
	}
}
