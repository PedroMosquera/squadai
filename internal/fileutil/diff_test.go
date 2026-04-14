package fileutil

import (
	"strings"
	"testing"
)

func TestUnifiedDiff_Identical(t *testing.T) {
	content := "line one\nline two\nline three\n"
	result := UnifiedDiff("foo.md", content, content)
	if result != "" {
		t.Errorf("expected empty string for identical content, got %q", result)
	}
}

func TestUnifiedDiff_EmptyBoth(t *testing.T) {
	result := UnifiedDiff("foo.md", "", "")
	if result != "" {
		t.Errorf("expected empty string for both empty, got %q", result)
	}
}

func TestUnifiedDiff_Create(t *testing.T) {
	// old is empty, new has content — all lines should be '+'
	newContent := "# Hello\n\nThis is a new file.\nWith multiple lines.\n"
	result := UnifiedDiff("README.md", "", newContent)

	if result == "" {
		t.Fatal("expected non-empty diff for create")
	}

	lines := strings.Split(result, "\n")
	// Check header
	if !strings.HasPrefix(lines[0], "--- a/") {
		t.Errorf("expected '--- a/' header, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "+++ b/") {
		t.Errorf("expected '+++ b/' header, got %q", lines[1])
	}

	// All content lines should be '+', none should be '-'
	for _, line := range lines[3:] { // skip headers and @@ line
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "-") {
			t.Errorf("create diff should have no '-' lines, got %q", line)
		}
	}

	// Hunk header should reflect 0 old lines
	if !strings.Contains(result, "-0,0") {
		t.Errorf("create diff should have -0,0 hunk header, got:\n%s", result)
	}
}

func TestUnifiedDiff_Delete(t *testing.T) {
	// new is empty, old has content — all lines should be '-'
	oldContent := "# Hello\n\nThis file is being deleted.\nWith multiple lines.\n"
	result := UnifiedDiff("README.md", oldContent, "")

	if result == "" {
		t.Fatal("expected non-empty diff for delete")
	}

	lines := strings.Split(result, "\n")
	// All content lines should be '-', none should be '+'
	for _, line := range lines[3:] {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "+") {
			t.Errorf("delete diff should have no '+' lines, got %q", line)
		}
	}

	// Hunk header should reflect 0 new lines
	if !strings.Contains(result, "+0,0") {
		t.Errorf("delete diff should have +0,0 hunk header, got:\n%s", result)
	}
}

func TestUnifiedDiff_Update(t *testing.T) {
	old := "# SquadAI\n\nOld description here.\n\nSome other content.\n"
	newContent := "# SquadAI\n\nNew description here.\n\nSome other content.\n"
	result := UnifiedDiff("README.md", old, newContent)

	if result == "" {
		t.Fatal("expected non-empty diff for update")
	}

	// Should have both + and - lines
	if !strings.Contains(result, "-Old description") {
		t.Errorf("expected '-Old description' in diff, got:\n%s", result)
	}
	if !strings.Contains(result, "+New description") {
		t.Errorf("expected '+New description' in diff, got:\n%s", result)
	}
}

func TestUnifiedDiff_PathInHeader(t *testing.T) {
	path := "internal/config/config.go"
	old := "package config\n"
	newContent := "package config\n\n// Version is the config version.\nconst Version = 1\n"
	result := UnifiedDiff(path, old, newContent)

	if !strings.Contains(result, "--- a/"+path) {
		t.Errorf("expected '--- a/%s' in diff header, got:\n%s", path, result)
	}
	if !strings.Contains(result, "+++ b/"+path) {
		t.Errorf("expected '+++ b/%s' in diff header, got:\n%s", path, result)
	}
}

func TestUnifiedDiff_ContextLines(t *testing.T) {
	// Change in the middle with context on both sides
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, "unchanged line")
	}
	old := strings.Join(append(append(lines[:5:5], "old middle"), lines[5:]...), "\n") + "\n"
	newContent := strings.Join(append(append(lines[:5:5], "new middle"), lines[5:]...), "\n") + "\n"

	result := UnifiedDiff("file.md", old, newContent)

	if result == "" {
		t.Fatal("expected non-empty diff")
	}

	// Should have context lines (space prefix)
	if !strings.Contains(result, " unchanged line") {
		t.Errorf("expected context lines with space prefix, got:\n%s", result)
	}
}

func TestUnifiedDiff_MultipleHunks(t *testing.T) {
	// Two changes far apart (more than 6 lines apart = 2 separate hunks)
	var oldLines []string
	var newLines []string
	for i := 0; i < 20; i++ {
		oldLines = append(oldLines, "common line")
		newLines = append(newLines, "common line")
	}
	// First change at index 0
	oldLines[0] = "old line A"
	newLines[0] = "new line A"
	// Second change at index 19
	oldLines[19] = "old line B"
	newLines[19] = "new line B"

	old := strings.Join(oldLines, "\n") + "\n"
	newContent := strings.Join(newLines, "\n") + "\n"

	result := UnifiedDiff("file.md", old, newContent)

	// Should have two @@ hunk headers
	hunkCount := strings.Count(result, "@@")
	// Each hunk has two @@ occurrences: "@@ -start,count +start,count @@"
	// Standard unified diff: "@@ -X,Y +X,Y @@" so each hunk = 1 "@@" marker pair
	// Actually strings.Count counts non-overlapping, so "@@ -... @@" = 2 "@@" per hunk
	if hunkCount < 4 {
		t.Errorf("expected at least 2 hunks (4 @@ markers), got %d @@ in:\n%s", hunkCount, result)
	}
}

func TestUnifiedDiff_SingleLineChange(t *testing.T) {
	old := "hello world\n"
	newContent := "hello Go\n"
	result := UnifiedDiff("greeting.txt", old, newContent)

	if result == "" {
		t.Fatal("expected non-empty diff")
	}
	if !strings.Contains(result, "-hello world") {
		t.Errorf("expected '-hello world' in diff, got:\n%s", result)
	}
	if !strings.Contains(result, "+hello Go") {
		t.Errorf("expected '+hello Go' in diff, got:\n%s", result)
	}
}

func TestUnifiedDiff_AddedLines(t *testing.T) {
	old := "line 1\nline 2\n"
	newContent := "line 1\nline 2\nline 3\nline 4\n"
	result := UnifiedDiff("file.txt", old, newContent)

	if result == "" {
		t.Fatal("expected non-empty diff for added lines")
	}
	if !strings.Contains(result, "+line 3") {
		t.Errorf("expected '+line 3' in diff, got:\n%s", result)
	}
	if !strings.Contains(result, "+line 4") {
		t.Errorf("expected '+line 4' in diff, got:\n%s", result)
	}
	// Should not have '-' lines
	for _, line := range strings.Split(result, "\n") {
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			t.Errorf("unexpected deletion line %q", line)
		}
	}
}

func TestUnifiedDiff_RealisticConfig(t *testing.T) {
	old := `# Team Standards

These are the team coding standards.

## Code Style
- Use meaningful variable names
- Keep functions small
`
	newContent := `# Team Standards

These are the team coding standards for Go.

## Code Style
- Use meaningful variable names
- Keep functions small
- Write tests for all public functions
`
	result := UnifiedDiff(".squadai/templates/team-standards.md", old, newContent)

	if result == "" {
		t.Fatal("expected non-empty diff for realistic config change")
	}
	if !strings.Contains(result, "-These are the team coding standards.") {
		t.Errorf("expected old line in diff:\n%s", result)
	}
	if !strings.Contains(result, "+These are the team coding standards for Go.") {
		t.Errorf("expected new line in diff:\n%s", result)
	}
}
