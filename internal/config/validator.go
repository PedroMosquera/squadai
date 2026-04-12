package config

import (
	"fmt"
	"strings"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// ValidateUser checks a UserConfig for structural issues.
func ValidateUser(cfg *domain.UserConfig) []string {
	var issues []string

	if cfg.Version < 1 {
		issues = append(issues, "version must be >= 1")
	}

	switch cfg.Mode {
	case domain.ModeTeam, domain.ModePersonal, domain.ModeHybrid:
		// valid
	case "":
		issues = append(issues, "mode is required")
	default:
		issues = append(issues, fmt.Sprintf("unknown mode %q (expected: team, personal, hybrid)", cfg.Mode))
	}

	for name := range cfg.Adapters {
		if !isKnownAdapter(name) {
			issues = append(issues, fmt.Sprintf("unknown adapter %q", name))
		}
	}

	for name := range cfg.Components {
		if !isKnownComponent(name) {
			issues = append(issues, fmt.Sprintf("unknown component %q", name))
		}
	}

	return issues
}

// ValidateProject checks a ProjectConfig for structural issues.
func ValidateProject(cfg *domain.ProjectConfig) []string {
	var issues []string

	if cfg.Version < 1 {
		issues = append(issues, "version must be >= 1")
	}

	for name := range cfg.Adapters {
		if !isKnownAdapter(name) {
			issues = append(issues, fmt.Sprintf("unknown adapter %q", name))
		}
	}

	for name := range cfg.Components {
		if !isKnownComponent(name) {
			issues = append(issues, fmt.Sprintf("unknown component %q", name))
		}
	}

	issues = append(issues, validateAgentDefs(cfg.Agents)...)
	issues = append(issues, validateSkillDefs(cfg.Skills)...)
	issues = append(issues, validateCommandDefs(cfg.Commands)...)
	issues = append(issues, validateMCPDefs(cfg.MCP)...)

	return issues
}

// ValidatePolicy checks a PolicyConfig for structural and consistency issues.
// This powers the `validate-policy` command.
func ValidatePolicy(cfg *domain.PolicyConfig) []string {
	var issues []string

	if cfg.Version < 1 {
		issues = append(issues, "version must be >= 1")
	}

	switch cfg.Mode {
	case domain.ModeTeam, domain.ModePersonal, domain.ModeHybrid:
		// valid
	case "":
		issues = append(issues, "mode is required")
	default:
		issues = append(issues, fmt.Sprintf("unknown mode %q", cfg.Mode))
	}

	// Check that every locked field has a corresponding value in required block.
	for _, field := range cfg.Locked {
		if !hasRequiredValue(cfg, field) {
			issues = append(issues, fmt.Sprintf("locked field %q has no corresponding value in required block", field))
		}
	}

	// Check that required adapters are known.
	for name := range cfg.Required.Adapters {
		if !isKnownAdapter(name) {
			issues = append(issues, fmt.Sprintf("unknown adapter %q in required block", name))
		}
	}

	// Check that required components are known.
	for name := range cfg.Required.Components {
		if !isKnownComponent(name) {
			issues = append(issues, fmt.Sprintf("unknown component %q in required block", name))
		}
	}

	// Validate required agents, MCP definitions.
	issues = append(issues, validateAgentDefs(cfg.Required.Agents)...)
	issues = append(issues, validateMCPDefs(cfg.Required.MCP)...)

	return issues
}

// hasRequiredValue checks whether a locked field path has a corresponding
// value in the policy's required block.
func hasRequiredValue(cfg *domain.PolicyConfig, field string) bool {
	parts := strings.Split(field, ".")
	if len(parts) < 2 {
		return false
	}

	switch parts[0] {
	case "adapters":
		if len(parts) < 3 {
			return false
		}
		_, exists := cfg.Required.Adapters[parts[1]]
		return exists

	case "components":
		if len(parts) < 3 {
			return false
		}
		_, exists := cfg.Required.Components[parts[1]]
		return exists

	case "copilot":
		return cfg.Required.Copilot.InstructionsTemplate != ""

	case "rules":
		return cfg.Required.Rules.TeamStandards != "" || cfg.Required.Rules.TeamStandardsFile != ""

	case "agents":
		if len(parts) < 2 {
			return false
		}
		_, exists := cfg.Required.Agents[parts[1]]
		return exists

	case "mcp":
		if len(parts) < 2 {
			return false
		}
		_, exists := cfg.Required.MCP[parts[1]]
		return exists

	default:
		return false
	}
}

// validateAgentDefs checks that agent definitions have required fields.
func validateAgentDefs(agents map[string]domain.AgentDef) []string {
	var issues []string
	for name, def := range agents {
		if def.Description == "" {
			issues = append(issues, fmt.Sprintf("agent %q must have a description", name))
		}
		if def.Mode == "" {
			issues = append(issues, fmt.Sprintf("agent %q must have a mode", name))
		} else {
			switch def.Mode {
			case "subagent", "byoa":
				// valid modes
			default:
				issues = append(issues, fmt.Sprintf("agent %q has unknown mode %q (expected: subagent, byoa)", name, def.Mode))
			}
		}
		if def.Prompt == "" && def.PromptFile == "" {
			issues = append(issues, fmt.Sprintf("agent %q must have prompt or prompt_file", name))
		}
	}
	return issues
}

// validateSkillDefs checks that skill definitions have required fields.
func validateSkillDefs(skills map[string]domain.SkillDef) []string {
	var issues []string
	for name, def := range skills {
		if def.Description == "" {
			issues = append(issues, fmt.Sprintf("skill %q must have a description", name))
		}
		if def.Content == "" && def.ContentFile == "" {
			issues = append(issues, fmt.Sprintf("skill %q must have content or content_file", name))
		}
	}
	return issues
}

// validateCommandDefs checks that command definitions have required fields.
func validateCommandDefs(commands map[string]domain.CommandDef) []string {
	var issues []string
	for name, def := range commands {
		if def.Description == "" {
			issues = append(issues, fmt.Sprintf("command %q must have a description", name))
		}
	}
	return issues
}

// validateMCPDefs checks that MCP server definitions have required fields.
func validateMCPDefs(mcpServers map[string]domain.MCPServerDef) []string {
	var issues []string
	for name, def := range mcpServers {
		if def.Type == "" {
			issues = append(issues, fmt.Sprintf("MCP server %q must have a type", name))
		} else {
			switch def.Type {
			case "local", "remote":
				// valid types
			default:
				issues = append(issues, fmt.Sprintf("MCP server %q has unknown type %q (expected: local, remote)", name, def.Type))
			}
		}
		if def.Type == "local" && len(def.Command) == 0 {
			issues = append(issues, fmt.Sprintf("MCP server %q (type=local) must have a command", name))
		}
		if def.Type == "remote" && def.URL == "" {
			issues = append(issues, fmt.Sprintf("MCP server %q (type=remote) must have a url", name))
		}
	}
	return issues
}

var knownAdapters = map[string]struct{}{
	string(domain.AgentOpenCode):   {},
	string(domain.AgentClaudeCode): {},
	string(domain.AgentCodex):      {},
}

var knownComponents = map[string]struct{}{
	string(domain.ComponentMemory):   {},
	string(domain.ComponentRules):    {},
	string(domain.ComponentSettings): {},
	string(domain.ComponentMCP):      {},
	string(domain.ComponentAgents):   {},
	string(domain.ComponentSkills):   {},
	string(domain.ComponentCommands): {},
}

func isKnownAdapter(name string) bool {
	_, ok := knownAdapters[name]
	return ok
}

func isKnownComponent(name string) bool {
	_, ok := knownComponents[name]
	return ok
}
