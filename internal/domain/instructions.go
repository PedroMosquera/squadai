package domain

import "path/filepath"

// SharedInstructionsFile returns <projectDir>/AGENTS.md, the one project
// instructions file every supported harness reads (Claude Code through an
// @AGENTS.md import in CLAUDE.md). Shared blocks are written here once.
func SharedInstructionsFile(projectDir string) string {
	return filepath.Join(projectDir, "AGENTS.md")
}

// IsSharedInstructionsFile reports whether path is projectDir's shared
// AGENTS.md. Blocks written there must be adapter-neutral, because every
// enabled harness reads the same copy.
func IsSharedInstructionsFile(projectDir, path string) bool {
	return projectDir != "" && path == SharedInstructionsFile(projectDir)
}

// BlockOwner returns the agent a marker block written to path belongs to: ""
// for the shared AGENTS.md, which carries one adapter-neutral copy, otherwise
// agent. Installers map "" to the unscoped section ID and neutral content.
func BlockOwner(agent AgentID, projectDir, path string) AgentID {
	if IsSharedInstructionsFile(projectDir, path) {
		return ""
	}
	return agent
}

// BlockActionPrefix names the owner in action IDs: "agents-md" for the shared
// file, otherwise the agent ID.
func BlockActionPrefix(owner AgentID) string {
	if owner == "" {
		return "agents-md"
	}
	return string(owner)
}
