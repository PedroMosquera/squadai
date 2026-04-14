package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTemplate_Standard(t *testing.T) {
	content, err := ResolveTemplate("standard", "", "/tmp/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("standard should return empty content, got %q", content)
	}
}

func TestResolveTemplate_Empty(t *testing.T) {
	content, err := ResolveTemplate("", "", "/tmp/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("empty ref should return empty content, got %q", content)
	}
}

func TestResolveTemplate_Custom(t *testing.T) {
	custom := "## Our Team Standards\n\nUse TypeScript strict mode."
	content, err := ResolveTemplate("custom", custom, "/tmp/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != custom {
		t.Errorf("custom should return custom_content, got %q", content)
	}
}

func TestResolveTemplate_CustomWithEmptyContent_Error(t *testing.T) {
	_, err := ResolveTemplate("custom", "", "/tmp/project")
	if err == nil {
		t.Fatal("custom with empty content should return error")
	}
}

func TestResolveTemplate_FileReference(t *testing.T) {
	projectDir := t.TempDir()

	// Create .squadai/templates/copilot.md
	tmplDir := filepath.Join(projectDir, ".squadai", "templates")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	expected := "## Custom Copilot Instructions\n\nUse strict linting.\n"
	if err := os.WriteFile(filepath.Join(tmplDir, "copilot.md"), []byte(expected), 0644); err != nil {
		t.Fatal(err)
	}

	content, err := ResolveTemplate("file:templates/copilot.md", "", projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != expected {
		t.Errorf("file: content = %q, want %q", content, expected)
	}
}

func TestResolveTemplate_FileNotFound_Error(t *testing.T) {
	projectDir := t.TempDir()

	_, err := ResolveTemplate("file:templates/missing.md", "", projectDir)
	if err == nil {
		t.Fatal("missing file should return error")
	}
}

func TestResolveTemplate_FileEmptyPath_Error(t *testing.T) {
	_, err := ResolveTemplate("file:", "", "/tmp/project")
	if err == nil {
		t.Fatal("empty file path should return error")
	}
}

func TestResolveTemplate_InlineContent(t *testing.T) {
	// Any value that's not "standard", "custom", or "file:" is treated as inline content.
	inline := "## Direct inline instructions\n\nJust write code."
	content, err := ResolveTemplate(inline, "", "/tmp/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != inline {
		t.Errorf("inline content = %q, want %q", content, inline)
	}
}

func TestIsBuiltin_Standard(t *testing.T) {
	if !IsBuiltin("standard") {
		t.Error("standard should be builtin")
	}
}

func TestIsBuiltin_Empty(t *testing.T) {
	if !IsBuiltin("") {
		t.Error("empty should be builtin")
	}
}

func TestIsBuiltin_Custom(t *testing.T) {
	if IsBuiltin("custom") {
		t.Error("custom should not be builtin")
	}
}

func TestIsBuiltin_File(t *testing.T) {
	if IsBuiltin("file:templates/foo.md") {
		t.Error("file: should not be builtin")
	}
}

func TestResolveTemplate_FileNestedPath(t *testing.T) {
	projectDir := t.TempDir()

	// Create .squadai/deep/nested/template.md
	tmplDir := filepath.Join(projectDir, ".squadai", "deep", "nested")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	expected := "nested template content"
	if err := os.WriteFile(filepath.Join(tmplDir, "template.md"), []byte(expected), 0644); err != nil {
		t.Fatal(err)
	}

	content, err := ResolveTemplate("file:deep/nested/template.md", "", projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != expected {
		t.Errorf("content = %q, want %q", content, expected)
	}
}
