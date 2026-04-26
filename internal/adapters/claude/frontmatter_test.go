package claude

import (
	"strings"
	"testing"
)

// ─── TranslateFrontmatter ───────────────────────────────────────────────────

func TestTranslateFrontmatter_AllToolsEnabled_OmitsToolsField(t *testing.T) {
	content := `---
description: My agent
mode: subagent
tools:
  read: true
  edit: true
  write: true
  grep: true
  glob: true
  bash: true
---

Body content here.
`
	result, err := TranslateFrontmatter(content, "implementer", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "tools:") {
		t.Error("all tools enabled — tools field should be omitted")
	}
}

func TestTranslateFrontmatter_SubsetOfTools_EmitsCommaList(t *testing.T) {
	content := `---
description: Read-only agent
mode: subagent
tools:
  read: true
  glob: true
  grep: true
  bash: false
  write: false
  edit: false
---

Body.
`
	result, err := TranslateFrontmatter(content, "explorer", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "tools: Read, Grep, Glob") {
		t.Errorf("expected tools: Read, Grep, Glob in output, got:\n%s", result)
	}
}

func TestTranslateFrontmatter_ToolsInDeterministicOrder(t *testing.T) {
	content := `---
description: Agent
mode: subagent
tools:
  bash: true
  edit: true
  read: true
  glob: false
  grep: false
  write: false
---

Body.
`
	result, err := TranslateFrontmatter(content, "implementer", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Order must be Read, Edit (canonical order from toolOrder)
	if !strings.Contains(result, "tools: Read, Edit, Bash") {
		t.Errorf("expected Read, Edit, Bash in canonical order, got:\n%s", result)
	}
}

func TestTranslateFrontmatter_ModeFieldStripped(t *testing.T) {
	content := `---
description: An agent
mode: primary
tools:
  read: true
  edit: true
  write: true
  grep: true
  glob: true
  bash: true
---

Body.
`
	result, err := TranslateFrontmatter(content, "orchestrator", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "mode:") {
		t.Error("mode field should be stripped from Claude frontmatter")
	}
}

func TestTranslateFrontmatter_ColorInjected(t *testing.T) {
	tests := []struct {
		role      string
		wantColor string
	}{
		{"orchestrator", "blue"},
		{"explorer", "cyan"},
		{"proposer", "purple"},
		{"spec-writer", "green"},
		{"designer", "yellow"},
		{"task-planner", "orange"},
		{"implementer", "red"},
		{"brainstormer", "purple"},
		{"planner", "orange"},
		{"tester", "cyan"},
		{"debugger", "red"},
		{"reviewer", "pink"},
		{"verifier", "pink"},
		{"unknown-role", "blue"}, // fallback
	}
	for _, tt := range tests {
		content := "---\ndescription: test\nmode: subagent\n---\n"
		result, err := TranslateFrontmatter(content, tt.role, "project")
		if err != nil {
			t.Fatalf("role %s: unexpected error: %v", tt.role, err)
		}
		want := "color: " + tt.wantColor
		if !strings.Contains(result, want) {
			t.Errorf("role %s: expected %q in frontmatter, got:\n%s", tt.role, want, result)
		}
	}
}

func TestTranslateFrontmatter_MemoryInjected(t *testing.T) {
	content := "---\ndescription: test\nmode: subagent\n---\n"

	for _, scope := range []string{"project", "user", "local"} {
		result, err := TranslateFrontmatter(content, "orchestrator", scope)
		if err != nil {
			t.Fatalf("scope %s: unexpected error: %v", scope, err)
		}
		want := "memory: " + scope
		if !strings.Contains(result, want) {
			t.Errorf("scope %s: expected %q in output, got:\n%s", scope, want, result)
		}
	}
}

func TestTranslateFrontmatter_MemoryDefaultsToProject(t *testing.T) {
	content := "---\ndescription: test\nmode: subagent\n---\n"
	result, err := TranslateFrontmatter(content, "orchestrator", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "memory: project") {
		t.Errorf("expected memory: project as default, got:\n%s", result)
	}
}

func TestTranslateFrontmatter_NameInjectedFromRole(t *testing.T) {
	content := "---\ndescription: test\nmode: subagent\n---\n"
	result, err := TranslateFrontmatter(content, "spec-writer", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "name: spec-writer") {
		t.Errorf("expected name: spec-writer, got:\n%s", result)
	}
}

func TestTranslateFrontmatter_ModelInheritInjected(t *testing.T) {
	content := "---\ndescription: test\nmode: subagent\n---\n"
	result, err := TranslateFrontmatter(content, "orchestrator", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "model: inherit") {
		t.Errorf("expected model: inherit, got:\n%s", result)
	}
}

func TestTranslateFrontmatter_DescriptionPassedThrough(t *testing.T) {
	content := "---\ndescription: Orchestrates the SDD workflow\nmode: primary\n---\n"
	result, err := TranslateFrontmatter(content, "orchestrator", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "description: Orchestrates the SDD workflow") {
		t.Errorf("expected description to be passed through, got:\n%s", result)
	}
}

func TestTranslateFrontmatter_BodyPreserved(t *testing.T) {
	content := "---\ndescription: test\nmode: subagent\n---\n\n# Title\n\nSome content.\n"
	result, err := TranslateFrontmatter(content, "explorer", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "# Title") {
		t.Error("body should be preserved after frontmatter translation")
	}
	if !strings.Contains(result, "Some content.") {
		t.Error("body content should be preserved")
	}
}

func TestTranslateFrontmatter_NoFrontmatter_ReturnsUnchanged(t *testing.T) {
	content := "# Just a markdown file\n\nNo frontmatter here.\n"
	result, err := TranslateFrontmatter(content, "explorer", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != content {
		t.Error("content without frontmatter should be returned unchanged")
	}
}

func TestTranslateFrontmatter_OutputStartsWithFrontmatterMarker(t *testing.T) {
	content := "---\ndescription: test\nmode: subagent\n---\n"
	result, err := TranslateFrontmatter(content, "orchestrator", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(result, "---\n") {
		t.Error("translated content should start with ---")
	}
}

// ─── Skills, MaxTurns, Memory paths, Effort ─────────────────────────────────

func TestTranslateFrontmatter_SkillsPassedThrough(t *testing.T) {
	content := `---
description: Test agent
mode: subagent
skills:
  - skills/testing.md
  - skills/coverage.md
---
`
	result, err := TranslateFrontmatter(content, "tester", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "skills:") {
		t.Error("skills block should be present in output")
	}
	if !strings.Contains(result, "- skills/testing.md") {
		t.Error("skills/testing.md should be in output")
	}
	if !strings.Contains(result, "- skills/coverage.md") {
		t.Error("skills/coverage.md should be in output")
	}
}

func TestTranslateFrontmatter_MaxTurnsEmitted(t *testing.T) {
	content := `---
description: Test agent
mode: subagent
max_turns: 15
---
`
	result, err := TranslateFrontmatter(content, "implementer", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "maxTurns: 15") {
		t.Errorf("expected maxTurns: 15 in output, got:\n%s", result)
	}
}

func TestTranslateFrontmatter_MaxTurnsZeroOmitted(t *testing.T) {
	content := `---
description: Test agent
mode: subagent
max_turns: 0
---
`
	result, err := TranslateFrontmatter(content, "implementer", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "maxTurns") {
		t.Error("maxTurns should be omitted when value is 0")
	}
}

func TestTranslateFrontmatter_MemoryPathListOverridesScope(t *testing.T) {
	content := `---
description: Test agent
mode: subagent
memory:
  - memory/project.md
  - memory/user.md
---
`
	result, err := TranslateFrontmatter(content, "orchestrator", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "memory:") {
		t.Error("memory block should be present in output")
	}
	if !strings.Contains(result, "- memory/project.md") {
		t.Error("memory/project.md should be in output")
	}
	// Scope string should NOT also appear when path list is given.
	if strings.Contains(result, "memory: project") {
		t.Error("scope string should be replaced by path list")
	}
}

func TestTranslateFrontmatter_MemoryScopeStringFallback(t *testing.T) {
	// When the frontmatter has a scalar memory: value (existing behavior),
	// TranslateFrontmatter should still inject the memoryScope parameter.
	content := `---
description: Test agent
mode: subagent
---
`
	result, err := TranslateFrontmatter(content, "orchestrator", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "memory: user") {
		t.Errorf("expected memory: user (scope fallback), got:\n%s", result)
	}
}

func TestTranslateFrontmatter_EffortPassedThrough(t *testing.T) {
	content := `---
description: Deep thinker
mode: subagent
effort: high
---
`
	result, err := TranslateFrontmatter(content, "proposer", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "effort: high") {
		t.Errorf("expected effort: high in output, got:\n%s", result)
	}
}

func TestTranslateFrontmatter_EffortOmittedWhenEmpty(t *testing.T) {
	content := `---
description: Normal agent
mode: subagent
---
`
	result, err := TranslateFrontmatter(content, "implementer", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "effort:") {
		t.Error("effort field should be omitted when not set")
	}
}

func TestTranslateFrontmatter_AllNewFieldsTogether(t *testing.T) {
	content := `---
description: Full-featured agent
mode: subagent
tools:
  read: true
  bash: true
  edit: false
  write: false
  grep: false
  glob: false
skills:
  - skills/testing.md
max_turns: 10
memory:
  - memory/project.md
effort: normal
---

Body here.
`
	result, err := TranslateFrontmatter(content, "tester", "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := []string{
		"tools: Read, Bash",
		"skills:",
		"- skills/testing.md",
		"maxTurns: 10",
		"memory:",
		"- memory/project.md",
		"effort: normal",
		"Body here.",
	}
	for _, want := range checks {
		if !strings.Contains(result, want) {
			t.Errorf("expected %q in output, got:\n%s", want, result)
		}
	}
}

// ─── roleColor ───────────────────────────────────────────────────────────────

func TestRoleColor_KnownRoles(t *testing.T) {
	tests := map[string]string{
		"orchestrator": "blue",
		"explorer":     "cyan",
		"proposer":     "purple",
		"spec-writer":  "green",
		"designer":     "yellow",
		"task-planner": "orange",
		"implementer":  "red",
		"verifier":     "pink",
	}
	for role, want := range tests {
		got := roleColor(role)
		if got != want {
			t.Errorf("roleColor(%q) = %q, want %q", role, got, want)
		}
	}
}

func TestRoleColor_UnknownRole_ReturnsBlue(t *testing.T) {
	got := roleColor("mystery-role")
	if got != "blue" {
		t.Errorf("roleColor(unknown) = %q, want \"blue\"", got)
	}
}
