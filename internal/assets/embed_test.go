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
		"skills/code-review/SKILL.md",
		"skills/testing/SKILL.md",
		"skills/pr-description/SKILL.md",
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
