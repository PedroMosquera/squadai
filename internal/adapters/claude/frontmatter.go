package claude

import (
	"fmt"
	"strings"
)

// toolOrder defines the canonical display order for tool names in Claude frontmatter.
var toolOrder = []string{"Read", "Edit", "Write", "Grep", "Glob", "Bash"}

// roleColors maps role names to Claude UI colors.
var roleColors = map[string]string{
	"orchestrator": "blue",
	"explorer":     "cyan",
	"proposer":     "purple",
	"spec-writer":  "green",
	"designer":     "yellow",
	"task-planner": "orange",
	"implementer":  "red",
	"brainstormer": "purple",
	"planner":      "orange",
	"tester":       "cyan",
	"debugger":     "red",
	"reviewer":     "pink",
	"verifier":     "pink",
}

// TranslateFrontmatter takes rendered Markdown content with OpenCode-style
// frontmatter and returns content with Claude-native frontmatter.
//
// Claude-specific changes:
//   - name:   injected from role parameter (lowercase + hyphens)
//   - mode:   stripped (Claude has no equivalent)
//   - tools:  map {key: bool} → comma-separated capitalized list; omitted when all enabled
//   - color:  injected based on role
//   - model:  inject "inherit"
//   - memory: inject based on memoryScope parameter
func TranslateFrontmatter(content, role, memoryScope string) (string, error) {
	fm, body, err := splitFrontmatter(content)
	if err != nil {
		// No frontmatter — return unchanged.
		return content, nil
	}

	fields, toolsEnabled, totalTools := parseFrontmatterFields(fm)

	if memoryScope == "" {
		memoryScope = "project"
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("name: %s\n", role))

	// description — pass through; handle multi-line value
	if desc, ok := fields["description"]; ok && desc != "" {
		b.WriteString(fmt.Sprintf("description: %s\n", desc))
	}

	// tools — omit if all 6 known tools are enabled (Claude inherits all by default)
	if toolsList := buildToolsList(toolsEnabled, totalTools); toolsList != "" {
		b.WriteString(fmt.Sprintf("tools: %s\n", toolsList))
	}

	// color — injected based on role
	b.WriteString(fmt.Sprintf("color: %s\n", roleColor(role)))

	// model — always inherit
	b.WriteString("model: inherit\n")

	// memory — injected based on scope
	b.WriteString(fmt.Sprintf("memory: %s\n", memoryScope))

	b.WriteString("---")
	if body != "" {
		b.WriteString("\n")
		b.WriteString(body)
	}

	return b.String(), nil
}

// splitFrontmatter splits content into the frontmatter block and the body.
// Returns an error if no frontmatter is found.
func splitFrontmatter(content string) (fm, body string, err error) {
	// Trim leading blank lines.
	trimmed := strings.TrimLeft(content, "\n")
	if !strings.HasPrefix(trimmed, "---\n") {
		return "", "", fmt.Errorf("no frontmatter found")
	}

	// Skip the opening "---\n".
	rest := trimmed[4:]

	// Find the closing "---".
	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		return "", "", fmt.Errorf("frontmatter not closed")
	}

	fm = rest[:idx]
	body = rest[idx+4:] // skip "\n---"
	// Consume one optional newline after the closing marker.
	if strings.HasPrefix(body, "\n") {
		body = body[1:]
	}
	return fm, body, nil
}

// parseFrontmatterFields parses the frontmatter YAML into a flat key→value map.
// It also returns the list of enabled tool names and the total number of tool entries.
func parseFrontmatterFields(fm string) (fields map[string]string, toolsEnabled []string, totalTools int) {
	fields = make(map[string]string)
	lines := strings.Split(fm, "\n")

	inTools := false
	toolsMap := make(map[string]bool)

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Indented line → tool entry inside the tools: block.
		if inTools && strings.HasPrefix(line, "  ") {
			trimmed := strings.TrimSpace(line)
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				toolsMap[key] = val == "true"
				totalTools++
			}
			continue
		}

		// Not indented → top-level key.
		inTools = false
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if key == "tools" && val == "" {
			inTools = true
			continue
		}
		// Strip fields that Claude does not support.
		if key == "mode" || key == "permission" {
			continue
		}
		fields[key] = val
	}

	// Collect enabled tools.
	for tool, enabled := range toolsMap {
		if enabled {
			toolsEnabled = append(toolsEnabled, tool)
		}
	}
	return fields, toolsEnabled, totalTools
}

// buildToolsList returns the comma-separated, capitalized tool list for Claude.
// Returns empty string when all tools are enabled (Claude inherits all by default).
func buildToolsList(toolsEnabled []string, totalTools int) string {
	if len(toolsEnabled) == 0 {
		return ""
	}
	// All tools enabled → omit the field (cleaner output).
	if len(toolsEnabled) >= 6 || (totalTools > 0 && len(toolsEnabled) >= totalTools) {
		return ""
	}

	// Build a set of enabled tools (lowercase lookup).
	enabledSet := make(map[string]bool, len(toolsEnabled))
	for _, t := range toolsEnabled {
		enabledSet[strings.ToLower(t)] = true
	}

	// Emit in canonical order.
	var result []string
	for _, t := range toolOrder {
		if enabledSet[strings.ToLower(t)] {
			result = append(result, t)
		}
	}
	return strings.Join(result, ", ")
}

// roleColor returns the display color for the given role name.
func roleColor(role string) string {
	if color, ok := roleColors[role]; ok {
		return color
	}
	return "blue"
}
