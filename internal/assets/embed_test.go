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
