package claude

import (
	"fmt"
	"strconv"
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
//   - name:     injected from role parameter (lowercase + hyphens)
//   - mode:     stripped (Claude has no equivalent)
//   - tools:    map {key: bool} → comma-separated capitalized list; omitted when all enabled
//   - color:    injected based on role
//   - model:    inject "inherit"
//   - skills:   passed through as-is when present
//   - maxTurns: derived from max_turns field when present
//   - memory:   explicit file path list when present; falls back to memoryScope string
//   - effort:   passed through when present
func TranslateFrontmatter(content, role, memoryScope string) (string, error) {
	fm, body, err := splitFrontmatter(content)
	if err != nil {
		// No frontmatter — return unchanged.
		return content, nil
	}

	parsed := parseFrontmatterFields(fm)

	if memoryScope == "" {
		memoryScope = "project"
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("name: %s\n", role))

	// description — pass through
	if desc, ok := parsed.fields["description"]; ok && desc != "" {
		b.WriteString(fmt.Sprintf("description: %s\n", desc))
	}

	// tools — omit if all 6 known tools are enabled (Claude inherits all by default)
	if toolsList := buildToolsList(parsed.toolsEnabled, parsed.totalTools); toolsList != "" {
		b.WriteString(fmt.Sprintf("tools: %s\n", toolsList))
	}

	// skills — pass through list when present
	if len(parsed.skills) > 0 {
		b.WriteString("skills:\n")
		for _, s := range parsed.skills {
			b.WriteString(fmt.Sprintf("  - %s\n", s))
		}
	}

	// color — injected based on role
	b.WriteString(fmt.Sprintf("color: %s\n", roleColor(role)))

	// model — always inherit
	b.WriteString("model: inherit\n")

	// maxTurns — emit when max_turns field present and > 0
	if raw, ok := parsed.fields["max_turns"]; ok && raw != "" && raw != "0" {
		if n, parseErr := strconv.Atoi(raw); parseErr == nil && n > 0 {
			b.WriteString(fmt.Sprintf("maxTurns: %d\n", n))
		}
	}

	// memory — explicit file paths take priority over the scope string
	if len(parsed.memoryPaths) > 0 {
		b.WriteString("memory:\n")
		for _, m := range parsed.memoryPaths {
			b.WriteString(fmt.Sprintf("  - %s\n", m))
		}
	} else {
		b.WriteString(fmt.Sprintf("memory: %s\n", memoryScope))
	}

	// effort — pass through when present
	if effort, ok := parsed.fields["effort"]; ok && effort != "" {
		b.WriteString(fmt.Sprintf("effort: %s\n", effort))
	}

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
	body = strings.TrimPrefix(body, "\n")
	return fm, body, nil
}

// parsedFrontmatter holds the result of parsing a YAML frontmatter block.
type parsedFrontmatter struct {
	fields       map[string]string // scalar key→value pairs (mode/permission stripped)
	toolsEnabled []string          // names of tools with value "true"
	totalTools   int               // total number of tool entries in the map
	skills       []string          // items from a skills: list block
	memoryPaths  []string          // items from a memory: list block (absent when scalar)
}

// parseFrontmatterFields parses the frontmatter YAML block into a structured result.
// Handles three kinds of block fields:
//   - tools: (bool map)   → toolsEnabled / totalTools
//   - skills: (list)      → skills
//   - memory: (list)      → memoryPaths (only when the value is a list, not a scalar)
//
// All other scalar fields are collected in fields. mode and permission are stripped.
func parseFrontmatterFields(fm string) parsedFrontmatter {
	p := parsedFrontmatter{fields: make(map[string]string)}
	lines := strings.Split(fm, "\n")

	type blockKind int
	const (
		blockNone   blockKind = iota
		blockTools            // key: bool map
		blockSkills           // - item list
		blockMemory           // - item list
	)

	inBlock := blockNone
	toolsMap := make(map[string]bool)

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Indented line → entry inside the current block.
		if inBlock != blockNone && strings.HasPrefix(line, "  ") {
			trimmed := strings.TrimSpace(line)
			switch inBlock {
			case blockTools:
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					val := strings.TrimSpace(parts[1])
					toolsMap[key] = val == "true"
					p.totalTools++
				}
			case blockSkills:
				p.skills = append(p.skills, strings.TrimPrefix(trimmed, "- "))
			case blockMemory:
				p.memoryPaths = append(p.memoryPaths, strings.TrimPrefix(trimmed, "- "))
			}
			continue
		}

		// Non-indented line → end of any current block, start of a new top-level field.
		inBlock = blockNone
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch {
		case key == "tools" && val == "":
			inBlock = blockTools
		case key == "skills" && val == "":
			inBlock = blockSkills
		case key == "memory" && val == "":
			// List form — collect items; scalar form is handled as a normal field.
			inBlock = blockMemory
		case key == "mode" || key == "permission":
			// Strip fields Claude does not support.
		default:
			p.fields[key] = val
		}
	}

	// Collect enabled tools.
	for tool, enabled := range toolsMap {
		if enabled {
			p.toolsEnabled = append(p.toolsEnabled, tool)
		}
	}
	return p
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
