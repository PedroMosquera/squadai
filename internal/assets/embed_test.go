package assets

import (
	"encoding/json"
	"strings"
	"testing"
)

// ─── Memory protocol files ──────────────────────────────────────────────────

func TestAllMemoryFilesReadable(t *testing.T) {
	files := []struct {
		path     string
		contains string
	}{
		{"memory/opencode.md", "AGENTS.md"},
		{"memory/claude.md", "CLAUDE.md"},
		{"memory/generic.md", "persistent memory tools"},
	}

	for _, f := range files {
		t.Run(f.path, func(t *testing.T) {
			content := MustRead(f.path)
			if len(content) < 50 {
				t.Errorf("%s: content too short (%d bytes), expected >= 50", f.path, len(content))
			}
			if !strings.Contains(content, f.contains) {
				t.Errorf("%s: expected to contain %q", f.path, f.contains)
			}
		})
	}
}

// ─── Standards files ────────────────────────────────────────────────────────

func TestAllStandardsFilesReadable(t *testing.T) {
	files := []struct {
		path     string
		contains string
	}{
		{"standards/go.md", "Error Handling"},
		{"standards/javascript.md", "TypeScript"},
		{"standards/python.md", "Type Hints"},
		{"standards/generic.md", "Code Quality"},
	}

	for _, f := range files {
		t.Run(f.path, func(t *testing.T) {
			content := MustRead(f.path)
			if len(content) < 50 {
				t.Errorf("%s: content too short (%d bytes), expected >= 50", f.path, len(content))
			}
			if !strings.Contains(content, f.contains) {
				t.Errorf("%s: expected to contain %q", f.path, f.contains)
			}
		})
	}
}

// ─── Copilot template ───────────────────────────────────────────────────────

func TestCopilotTemplateReadable(t *testing.T) {
	content := MustRead("copilot/standard.tmpl")
	if len(content) < 50 {
		t.Errorf("copilot template too short (%d bytes), expected >= 50", len(content))
	}
	if !strings.Contains(content, "{{.Name}}") {
		t.Error("copilot template should contain {{.Name}} template variable")
	}
	if !strings.Contains(content, "Team Standards") {
		t.Error("copilot template should contain Team Standards heading")
	}
}

// ─── Skill files ────────────────────────────────────────────────────────────

func TestAllSkillFilesReadable(t *testing.T) {
	files := []string{
		"skills/shared/code-review/SKILL.md",
		"skills/shared/testing/SKILL.md",
		"skills/shared/pr-description/SKILL.md",
	}

	for _, path := range files {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if len(content) < 50 {
				t.Errorf("%s: content too short (%d bytes), expected >= 50", path, len(content))
			}
			if !strings.Contains(content, "---") {
				t.Errorf("%s: expected YAML frontmatter markers (---)", path)
			}
		})
	}
}

// ─── MCP asset files ─────────────────────────────────────────────────────────

func TestMCPAssetReadable(t *testing.T) {
	content := MustRead("mcp/context7.json")
	if len(content) == 0 {
		t.Error("mcp/context7.json should return non-empty content")
	}
}

func TestMCPAssetValidJSON(t *testing.T) {
	content := MustRead("mcp/context7.json")
	if !json.Valid([]byte(content)) {
		t.Errorf("mcp/context7.json should be valid JSON, got: %s", content)
	}
}

// ─── Orchestrator templates ──────────────────────────────────────────────────

var orchestratorPaths = []string{
	"teams/tdd/orchestrator-native.md",
	"teams/tdd/orchestrator-prompt.md",
	"teams/tdd/orchestrator-solo.md",
	"teams/sdd/orchestrator-native.md",
	"teams/sdd/orchestrator-prompt.md",
	"teams/sdd/orchestrator-solo.md",
	"teams/conventional/orchestrator-native.md",
	"teams/conventional/orchestrator-prompt.md",
	"teams/conventional/orchestrator-solo.md",
}

func TestOrchestratorTemplates_AllExist(t *testing.T) {
	for _, path := range orchestratorPaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if len(content) == 0 {
				t.Errorf("%s: expected non-empty content", path)
			}
		})
	}
}

func TestOrchestratorTemplates_MinimumSize(t *testing.T) {
	for _, path := range orchestratorPaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if len(content) < 100 {
				t.Errorf("%s: content too short (%d bytes), expected >= 100", path, len(content))
			}
		})
	}
}

func TestOrchestratorTemplates_HasIdentitySection(t *testing.T) {
	for _, path := range orchestratorPaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if !strings.Contains(content, "## Identity") {
				t.Errorf("%s: expected to contain '## Identity' section", path)
			}
		})
	}
}

func TestOrchestratorTemplates_HasDelegationRulesSection(t *testing.T) {
	for _, path := range orchestratorPaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if !strings.Contains(content, "## Delegation Rules") {
				t.Errorf("%s: expected to contain '## Delegation Rules' section", path)
			}
		})
	}
}

func TestOrchestratorTemplates_HasMethodologyWorkflowSection(t *testing.T) {
	for _, path := range orchestratorPaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if !strings.Contains(content, "## Methodology Workflow") {
				t.Errorf("%s: expected to contain '## Methodology Workflow' section", path)
			}
		})
	}
}

func TestOrchestratorTemplates_NativeVariants_MentionAgents(t *testing.T) {
	nativePaths := []string{
		"teams/tdd/orchestrator-native.md",
		"teams/sdd/orchestrator-native.md",
		"teams/conventional/orchestrator-native.md",
	}
	for _, path := range nativePaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if !strings.Contains(content, "agent") {
				t.Errorf("%s: native variant expected to mention 'agent'", path)
			}
		})
	}
}

func TestOrchestratorTemplates_PromptVariants_MentionTask(t *testing.T) {
	promptPaths := []string{
		"teams/tdd/orchestrator-prompt.md",
		"teams/sdd/orchestrator-prompt.md",
		"teams/conventional/orchestrator-prompt.md",
	}
	for _, path := range promptPaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if !strings.Contains(content, "Task") {
				t.Errorf("%s: prompt variant expected to mention 'Task'", path)
			}
		})
	}
}

func TestOrchestratorTemplates_SoloVariants_MentionPhaseMarkers(t *testing.T) {
	soloPaths := []string{
		"teams/tdd/orchestrator-solo.md",
		"teams/sdd/orchestrator-solo.md",
		"teams/conventional/orchestrator-solo.md",
	}
	for _, path := range soloPaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if !strings.Contains(content, "PHASE") && !strings.Contains(content, "sequential") {
				t.Errorf("%s: solo variant expected to mention 'PHASE' or 'sequential'", path)
			}
		})
	}
}

func TestOrchestratorTemplates_TDD_MentionsBrainstormer(t *testing.T) {
	tddPaths := []string{
		"teams/tdd/orchestrator-native.md",
		"teams/tdd/orchestrator-prompt.md",
		"teams/tdd/orchestrator-solo.md",
	}
	for _, path := range tddPaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			lower := strings.ToLower(content)
			if !strings.Contains(lower, "brainstorm") {
				t.Errorf("%s: TDD template expected to mention 'brainstorm' or 'Brainstormer'", path)
			}
		})
	}
}

func TestOrchestratorTemplates_TDD_SuperpowersExclusion(t *testing.T) {
	content := MustRead("teams/tdd/orchestrator-native.md")
	if !strings.Contains(content, "Superpowers") {
		t.Error("teams/tdd/orchestrator-native.md: expected to mention 'Superpowers' exclusion")
	}
}

// ─── Sub-agent definition files ─────────────────────────────────────────────

var subAgentPaths = []string{
	"teams/tdd/brainstormer.md",
	"teams/tdd/planner.md",
	"teams/tdd/implementer.md",
	"teams/tdd/reviewer.md",
	"teams/tdd/debugger.md",
	"teams/sdd/explorer.md",
	"teams/sdd/proposer.md",
	"teams/sdd/spec-writer.md",
	"teams/sdd/designer.md",
	"teams/sdd/task-planner.md",
	"teams/sdd/implementer.md",
	"teams/sdd/verifier.md",
	"teams/conventional/implementer.md",
	"teams/conventional/reviewer.md",
	"teams/conventional/tester.md",
}

func TestAllSubAgentDefsExist(t *testing.T) {
	for _, path := range subAgentPaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if len(content) == 0 {
				t.Errorf("%s: expected non-empty content", path)
			}
		})
	}
}

func TestSubAgentDefs_MinimumSize(t *testing.T) {
	for _, path := range subAgentPaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if len(content) < 50 {
				t.Errorf("%s: content too short (%d bytes), expected >= 50", path, len(content))
			}
		})
	}
}

func TestSubAgentDefs_HaveYAMLFrontmatter(t *testing.T) {
	for _, path := range subAgentPaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if !strings.Contains(content, "---") {
				t.Errorf("%s: expected YAML frontmatter markers (---)", path)
			}
		})
	}
}

func TestSubAgentDefs_HaveIdentitySection(t *testing.T) {
	for _, path := range subAgentPaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if !strings.Contains(content, "## Identity") {
				t.Errorf("%s: expected '## Identity' section", path)
			}
		})
	}
}

func TestSubAgentDefs_HaveExecutorBoundary(t *testing.T) {
	for _, path := range subAgentPaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if !strings.Contains(content, "EXECUTOR") && !strings.Contains(content, "not the orchestrator") {
				t.Errorf("%s: expected to mention 'EXECUTOR' or 'not the orchestrator'", path)
			}
		})
	}
}

// ─── Methodology skill files ─────────────────────────────────────────────────

var tddSkillPaths = []string{
	"skills/tdd/brainstorming/SKILL.md",
	"skills/tdd/writing-plans/SKILL.md",
	"skills/tdd/test-driven-development/SKILL.md",
	"skills/tdd/systematic-debugging/SKILL.md",
	"skills/tdd/subagent-driven-development/SKILL.md",
}

var sddSkillPaths = []string{
	"skills/sdd/sdd-explore/SKILL.md",
	"skills/sdd/sdd-propose/SKILL.md",
	"skills/sdd/sdd-spec/SKILL.md",
	"skills/sdd/sdd-design/SKILL.md",
	"skills/sdd/sdd-tasks/SKILL.md",
	"skills/sdd/sdd-apply/SKILL.md",
	"skills/sdd/sdd-verify/SKILL.md",
}

var sharedSkillPaths = []string{
	"skills/shared/code-review/SKILL.md",
	"skills/shared/testing/SKILL.md",
	"skills/shared/pr-description/SKILL.md",
}

func TestAllMethodologySkillsExist(t *testing.T) {
	allSkills := append(append(tddSkillPaths, sddSkillPaths...), sharedSkillPaths...)
	for _, path := range allSkills {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if len(content) == 0 {
				t.Errorf("%s: expected non-empty content", path)
			}
		})
	}
}

func TestMethodologySkills_MinimumSize(t *testing.T) {
	allSkills := append(append(tddSkillPaths, sddSkillPaths...), sharedSkillPaths...)
	for _, path := range allSkills {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if len(content) < 50 {
				t.Errorf("%s: content too short (%d bytes), expected >= 50", path, len(content))
			}
		})
	}
}

func TestMethodologySkills_TDD_HaveMethodologyTag(t *testing.T) {
	for _, path := range tddSkillPaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if !strings.Contains(content, "methodology: tdd") {
				t.Errorf("%s: expected 'methodology: tdd' in frontmatter", path)
			}
		})
	}
}

func TestMethodologySkills_SDD_HaveMethodologyTag(t *testing.T) {
	for _, path := range sddSkillPaths {
		t.Run(path, func(t *testing.T) {
			content := MustRead(path)
			if !strings.Contains(content, "methodology: sdd") {
				t.Errorf("%s: expected 'methodology: sdd' in frontmatter", path)
			}
		})
	}
}

func TestSkillRef_MatchesEmbeddedFiles(t *testing.T) {
	// Verify that every SkillRef in DefaultTeam() corresponds to an embedded asset.
	// Import-free: we call domain.DefaultTeam via the assets package test context.
	// We enumerate the known SkillRef values directly from domain.DefaultTeam output.
	skillRefs := []string{
		// TDD roles
		"tdd/brainstorming",
		"tdd/writing-plans",
		"tdd/test-driven-development",
		"shared/code-review",
		"tdd/systematic-debugging",
		// SDD roles
		"sdd/sdd-explore",
		"sdd/sdd-propose",
		"sdd/sdd-spec",
		"sdd/sdd-design",
		"sdd/sdd-tasks",
		"sdd/sdd-apply",
		"sdd/sdd-verify",
		// Conventional roles
		"shared/testing",
	}

	for _, ref := range skillRefs {
		path := "skills/" + ref + "/SKILL.md"
		t.Run(ref, func(t *testing.T) {
			content := MustRead(path)
			if len(content) == 0 {
				t.Errorf("SkillRef %q maps to %q but file is empty or missing", ref, path)
			}
		})
	}
}

// ─── Workflow asset files ────────────────────────────────────────────────────

func TestWorkflowAssetsReadable(t *testing.T) {
	files := []struct {
		path     string
		contains string
	}{
		{"workflows/tdd-pipeline.md", "TDD"},
		{"workflows/sdd-pipeline.md", "SDD"},
		{"workflows/conventional-pipeline.md", "Conventional"},
	}

	for _, f := range files {
		t.Run(f.path, func(t *testing.T) {
			content := MustRead(f.path)
			if len(content) == 0 {
				t.Errorf("%s: expected non-empty content", f.path)
			}
			if !strings.Contains(content, f.contains) {
				t.Errorf("%s: expected to contain %q", f.path, f.contains)
			}
		})
	}
}

// ─── Error handling ─────────────────────────────────────────────────────────

func TestReadNonexistent(t *testing.T) {
	content, err := Read("does/not/exist.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
	if content != "" {
		t.Errorf("expected empty string for nonexistent file, got %q", content)
	}
}

// ─── Skill catalog ─────────────────────────────────────────────────────────

func TestSkillCatalog_Readable(t *testing.T) {
	content, err := Read("skills/catalog.json")
	if err != nil {
		t.Fatalf("skills/catalog.json: Read returned error: %v", err)
	}
	if len(content) == 0 {
		t.Error("skills/catalog.json: expected non-empty content")
	}
}

func TestSkillCatalog_ValidJSON(t *testing.T) {
	content, err := Read("skills/catalog.json")
	if err != nil {
		t.Fatalf("skills/catalog.json: Read returned error: %v", err)
	}
	if !json.Valid([]byte(content)) {
		t.Errorf("skills/catalog.json: not valid JSON, got:\n%s", content)
	}
}

func TestSkillCatalog_HasCategories(t *testing.T) {
	content := MustRead("skills/catalog.json")
	var cat struct {
		Categories []struct {
			Name   string `json:"name"`
			Skills []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Source      string `json:"source"`
			} `json:"skills"`
		} `json:"categories"`
	}
	if err := json.Unmarshal([]byte(content), &cat); err != nil {
		t.Fatalf("skills/catalog.json: unmarshal error: %v", err)
	}
	if len(cat.Categories) == 0 {
		t.Error("skills/catalog.json: expected at least one category")
	}
}

func TestSkillCatalog_EachCategoryHasName(t *testing.T) {
	content := MustRead("skills/catalog.json")
	var cat struct {
		Categories []struct {
			Name string `json:"name"`
		} `json:"categories"`
	}
	if err := json.Unmarshal([]byte(content), &cat); err != nil {
		t.Fatalf("skills/catalog.json: unmarshal error: %v", err)
	}
	for i, c := range cat.Categories {
		if c.Name == "" {
			t.Errorf("skills/catalog.json: category[%d] has empty name", i)
		}
	}
}

func TestSkillCatalog_EachSkillHasNameAndDescription(t *testing.T) {
	content := MustRead("skills/catalog.json")
	var cat struct {
		Categories []struct {
			Name   string `json:"name"`
			Skills []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"skills"`
		} `json:"categories"`
	}
	if err := json.Unmarshal([]byte(content), &cat); err != nil {
		t.Fatalf("skills/catalog.json: unmarshal error: %v", err)
	}
	for _, c := range cat.Categories {
		for j, s := range c.Skills {
			if s.Name == "" {
				t.Errorf("skills/catalog.json: category %q skill[%d] has empty name", c.Name, j)
			}
			if s.Description == "" {
				t.Errorf("skills/catalog.json: category %q skill %q has empty description", c.Name, s.Name)
			}
		}
	}
}

func TestSkillCatalog_HasInstallCommand(t *testing.T) {
	content := MustRead("skills/catalog.json")
	var cat struct {
		InstallCommand string `json:"install_command"`
	}
	if err := json.Unmarshal([]byte(content), &cat); err != nil {
		t.Fatalf("skills/catalog.json: unmarshal error: %v", err)
	}
	if cat.InstallCommand == "" {
		t.Error("skills/catalog.json: expected non-empty install_command")
	}
}

func TestSkillCatalog_HasSearchCommand(t *testing.T) {
	content := MustRead("skills/catalog.json")
	var cat struct {
		SearchCommand string `json:"search_command"`
	}
	if err := json.Unmarshal([]byte(content), &cat); err != nil {
		t.Fatalf("skills/catalog.json: unmarshal error: %v", err)
	}
	if cat.SearchCommand == "" {
		t.Error("skills/catalog.json: expected non-empty search_command")
	}
}

func TestSkillCatalog_HasBrowseURL(t *testing.T) {
	content := MustRead("skills/catalog.json")
	var cat struct {
		BrowseURL string `json:"browse_url"`
	}
	if err := json.Unmarshal([]byte(content), &cat); err != nil {
		t.Fatalf("skills/catalog.json: unmarshal error: %v", err)
	}
	if cat.BrowseURL == "" {
		t.Error("skills/catalog.json: expected non-empty browse_url")
	}
	if !strings.Contains(cat.BrowseURL, "skills.sh") {
		t.Errorf("skills/catalog.json: browse_url should mention 'skills.sh', got %q", cat.BrowseURL)
	}
}

func TestSkillCatalog_AtLeastTwentySkillsTotal(t *testing.T) {
	content := MustRead("skills/catalog.json")
	var cat struct {
		Categories []struct {
			Skills []struct {
				Name string `json:"name"`
			} `json:"skills"`
		} `json:"categories"`
	}
	if err := json.Unmarshal([]byte(content), &cat); err != nil {
		t.Fatalf("skills/catalog.json: unmarshal error: %v", err)
	}
	total := 0
	for _, c := range cat.Categories {
		total += len(c.Skills)
	}
	if total < 20 {
		t.Errorf("skills/catalog.json: expected at least 20 skills total, got %d", total)
	}
}
