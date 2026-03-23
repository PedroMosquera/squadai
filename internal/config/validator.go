package config

import (
	"fmt"
	"strings"

	"github.com/alexmosquera/agent-manager-pro/internal/domain"
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

	for name := range cfg.Components {
		if !isKnownComponent(name) {
			issues = append(issues, fmt.Sprintf("unknown component %q", name))
		}
	}

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

	default:
		return false
	}
}

var knownAdapters = map[string]struct{}{
	string(domain.AgentOpenCode):   {},
	string(domain.AgentClaudeCode): {},
	string(domain.AgentCodex):      {},
}

var knownComponents = map[string]struct{}{
	string(domain.ComponentMemory): {},
}

func isKnownAdapter(name string) bool {
	_, ok := knownAdapters[name]
	return ok
}

func isKnownComponent(name string) bool {
	_, ok := knownComponents[name]
	return ok
}
