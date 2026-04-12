package config

import (
	"fmt"
	"strings"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// Merge combines user, project, and policy configs with precedence:
//   1. Policy locked fields (highest priority, cannot be overridden)
//   2. Project config
//   3. User config (lowest priority)
//
// Any of the three inputs may be nil (absent). A nil policy means no locks.
// A nil user config means defaults are used. A nil project config means no
// project-level overrides.
//
// When a user or project value conflicts with a policy-locked field, the
// policy value wins and the conflict is recorded in MergedConfig.Violations.
func Merge(user *domain.UserConfig, project *domain.ProjectConfig, policy *domain.PolicyConfig) *domain.MergedConfig {
	merged := &domain.MergedConfig{
		Mode:       domain.ModePersonal,
		Adapters:   make(map[string]domain.AdapterConfig),
		Components: make(map[string]domain.ComponentConfig),
		Paths: domain.PathsConfig{
			BackupDir: "~/.agent-manager/backups",
		},
	}

	// Layer 1: user defaults.
	if user != nil {
		merged.Mode = user.Mode
		for k, v := range user.Adapters {
			merged.Adapters[k] = v
		}
		for k, v := range user.Components {
			merged.Components[k] = v
		}
		merged.Copilot = domain.CopilotConfig{}
		if user.Paths.BackupDir != "" {
			merged.Paths.BackupDir = user.Paths.BackupDir
		}
	}

	// Layer 2: project overrides user.
	if project != nil {
		for k, v := range project.Components {
			merged.Components[k] = v
		}
		if project.Copilot.InstructionsTemplate != "" {
			merged.Copilot.InstructionsTemplate = project.Copilot.InstructionsTemplate
		}
	}

	// Layer 3: policy locked fields override everything.
	if policy != nil {
		if policy.Mode != "" {
			merged.Mode = policy.Mode
		}

		locked := buildLockedSet(policy.Locked)

		// Apply required adapter values, recording violations.
		for k, v := range policy.Required.Adapters {
			field := fmt.Sprintf("adapters.%s.enabled", k)
			if _, isLocked := locked[field]; isLocked {
				if existing, exists := merged.Adapters[k]; exists && existing.Enabled != v.Enabled {
					merged.Violations = append(merged.Violations,
						fmt.Sprintf("field %q locked to %v, overriding %v", field, v.Enabled, existing.Enabled))
				}
			}
			merged.Adapters[k] = v
		}

		// Apply required component values, recording violations.
		for k, v := range policy.Required.Components {
			field := fmt.Sprintf("components.%s.enabled", k)
			if _, isLocked := locked[field]; isLocked {
				if existing, exists := merged.Components[k]; exists && existing.Enabled != v.Enabled {
					merged.Violations = append(merged.Violations,
						fmt.Sprintf("field %q locked to %v, overriding %v", field, v.Enabled, existing.Enabled))
				}
			}
			merged.Components[k] = v
		}

		// Apply required copilot values, recording violations.
		if policy.Required.Copilot.InstructionsTemplate != "" {
			field := "copilot.instructions_template"
			if _, isLocked := locked[field]; isLocked {
				if merged.Copilot.InstructionsTemplate != "" &&
					merged.Copilot.InstructionsTemplate != policy.Required.Copilot.InstructionsTemplate {
					merged.Violations = append(merged.Violations,
						fmt.Sprintf("field %q locked to %q, overriding %q",
							field,
							policy.Required.Copilot.InstructionsTemplate,
							merged.Copilot.InstructionsTemplate))
				}
			}
			merged.Copilot.InstructionsTemplate = policy.Required.Copilot.InstructionsTemplate
		}
	}

	return merged
}

// buildLockedSet converts the locked field list into a set for O(1) lookups.
func buildLockedSet(locked []string) map[string]struct{} {
	set := make(map[string]struct{}, len(locked))
	for _, field := range locked {
		set[strings.TrimSpace(field)] = struct{}{}
	}
	return set
}
