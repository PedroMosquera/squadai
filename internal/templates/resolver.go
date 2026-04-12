package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveTemplate resolves a template reference to its content.
//
// Template references are resolved in this order:
//   - "standard" or "" → returns ("", nil) — caller should use built-in default
//   - "custom" → returns (customContent, nil) — inline content from config
//   - "file:<path>" → reads the file at .agent-manager/<path> relative to projectDir
//   - anything else → returned as-is (treated as inline content)
func ResolveTemplate(templateRef, customContent, projectDir string) (string, error) {
	if templateRef == "" || templateRef == "standard" {
		return "", nil
	}

	if templateRef == "custom" {
		if customContent == "" {
			return "", fmt.Errorf("template is \"custom\" but custom_content is empty")
		}
		return customContent, nil
	}

	if strings.HasPrefix(templateRef, "file:") {
		relPath := strings.TrimPrefix(templateRef, "file:")
		if relPath == "" {
			return "", fmt.Errorf("template file path is empty in \"file:\" reference")
		}
		absPath := filepath.Join(projectDir, ".agent-manager", relPath)
		content, err := os.ReadFile(absPath)
		if err != nil {
			return "", fmt.Errorf("read template file %s: %w", absPath, err)
		}
		return string(content), nil
	}

	// Treat as inline content.
	return templateRef, nil
}

// IsBuiltin returns true if the template reference means the caller should
// use its built-in default template.
func IsBuiltin(templateRef string) bool {
	return templateRef == "" || templateRef == "standard"
}
