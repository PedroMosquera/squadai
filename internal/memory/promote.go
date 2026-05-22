package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Promote moves a note from _inbox/ to docs/memory/<category>/<filename>,
// prepending YAML frontmatter. Returns the new relative path.
func Promote(projectDir, inboxPath, category string) (string, error) {
	// Resolve the source path (may be relative to projectDir).
	srcPath := inboxPath
	if !filepath.IsAbs(inboxPath) {
		srcPath = filepath.Join(projectDir, inboxPath)
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("read inbox file: %w", err)
	}

	// Create destination directory.
	destDir := filepath.Join(projectDir, "docs", "memory", category)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("create category dir: %w", err)
	}

	// Build destination file path (same filename as source).
	filename := filepath.Base(srcPath)
	destPath := filepath.Join(destDir, filename)

	// Prepend YAML frontmatter.
	date := time.Now().UTC().Format("2006-01-02")
	frontmatter := fmt.Sprintf("---\ndate: %s\ncategory: %s\n---\n\n", date, category)
	content := frontmatter + string(data)

	if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write promoted file: %w", err)
	}

	// Delete source file after successful write.
	if err := os.Remove(srcPath); err != nil {
		return "", fmt.Errorf("remove inbox file: %w", err)
	}

	rel, err := filepath.Rel(projectDir, destPath)
	if err != nil {
		return destPath, nil
	}
	return rel, nil
}
