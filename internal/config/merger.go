package config

import (
	"fmt"
	"strings"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// Merge combines user, project, and policy configs with precedence:
//  1. Policy locked fields (highest priority, cannot be overridden)
//  2. Project config
//  3. User config (lowest priority)
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
			merged.Adapters[k] = cloneAdapterConfig(v)
		}
		for k, v := range user.Components {
			merged.Components[k] = cloneComponentConfig(v)
		}
		merged.Copilot = domain.CopilotConfig{}
		if user.Paths.BackupDir != "" {
			merged.Paths.BackupDir = user.Paths.BackupDir
		}
	}

	// Layer 2: project overrides user.
	if project != nil {
		// Merge project adapter settings into user adapter settings.
		for k, v := range project.Adapters {
			if existing, exists := merged.Adapters[k]; exists {
				merged.Adapters[k] = mergeAdapterConfig(existing, v)
			} else {
				merged.Adapters[k] = cloneAdapterConfig(v)
			}
		}

		// Merge project component settings into user component settings.
		for k, v := range project.Components {
			if existing, exists := merged.Components[k]; exists {
				merged.Components[k] = mergeComponentConfig(existing, v)
			} else {
				merged.Components[k] = cloneComponentConfig(v)
			}
		}

		if project.Copilot.InstructionsTemplate != "" {
			merged.Copilot.InstructionsTemplate = project.Copilot.InstructionsTemplate
		}
		if project.Copilot.CustomContent != "" {
			merged.Copilot.CustomContent = project.Copilot.CustomContent
		}

		// Merge project-only sections.
		merged.Rules = project.Rules
		if project.Agents != nil {
			merged.Agents = cloneAgentDefs(project.Agents)
		}
		if project.Skills != nil {
			merged.Skills = cloneSkillDefs(project.Skills)
		}
		if project.Commands != nil {
			merged.Commands = cloneCommandDefs(project.Commands)
		}
		if project.MCP != nil {
			merged.MCP = cloneMCPDefs(project.MCP)
		}
		merged.Meta = project.Meta
		merged.Copilot.Meta = project.Meta
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
			// Merge settings: policy required settings override, but preserve
			// non-conflicting settings from lower layers.
			if existing, exists := merged.Adapters[k]; exists {
				result := mergeAdapterConfig(existing, v)
				// Check for locked settings fields.
				for sk := range v.Settings {
					settingsField := fmt.Sprintf("adapters.%s.settings.%s", k, sk)
					if _, isLocked := locked[settingsField]; isLocked {
						if existingVal, hasExisting := existing.Settings[sk]; hasExisting {
							if fmt.Sprintf("%v", existingVal) != fmt.Sprintf("%v", v.Settings[sk]) {
								merged.Violations = append(merged.Violations,
									fmt.Sprintf("field %q locked by policy", settingsField))
							}
						}
					}
				}
				merged.Adapters[k] = result
			} else {
				merged.Adapters[k] = cloneAdapterConfig(v)
			}
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
			if existing, exists := merged.Components[k]; exists {
				merged.Components[k] = mergeComponentConfig(existing, v)
			} else {
				merged.Components[k] = cloneComponentConfig(v)
			}
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

		// Apply required rules, recording violations for locked fields.
		if policy.Required.Rules.TeamStandards != "" {
			field := "rules.team_standards"
			if _, isLocked := locked[field]; isLocked {
				if merged.Rules.TeamStandards != "" &&
					merged.Rules.TeamStandards != policy.Required.Rules.TeamStandards {
					merged.Violations = append(merged.Violations,
						fmt.Sprintf("field %q locked by policy", field))
				}
			}
			merged.Rules.TeamStandards = policy.Required.Rules.TeamStandards
		}

		// Apply required agents, overriding project agents for locked definitions.
		for name, def := range policy.Required.Agents {
			field := fmt.Sprintf("agents.%s", name)
			if _, isLocked := locked[field]; isLocked {
				if _, exists := merged.Agents[name]; exists {
					merged.Violations = append(merged.Violations,
						fmt.Sprintf("field %q locked by policy", field))
				}
			}
			if merged.Agents == nil {
				merged.Agents = make(map[string]domain.AgentDef)
			}
			merged.Agents[name] = def
		}

		// Apply required MCP servers, overriding project MCP for locked definitions.
		for name, def := range policy.Required.MCP {
			field := fmt.Sprintf("mcp.%s", name)
			if _, isLocked := locked[field]; isLocked {
				if _, exists := merged.MCP[name]; exists {
					merged.Violations = append(merged.Violations,
						fmt.Sprintf("field %q locked by policy", field))
				}
			}
			if merged.MCP == nil {
				merged.MCP = make(map[string]domain.MCPServerDef)
			}
			merged.MCP[name] = def
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

// mergeAdapterConfig combines two AdapterConfigs. The override takes precedence for
// Enabled. Settings maps are shallowly merged: override keys replace base keys.
func mergeAdapterConfig(base, override domain.AdapterConfig) domain.AdapterConfig {
	result := domain.AdapterConfig{
		Enabled:  override.Enabled,
		Settings: mergeSettingsMaps(base.Settings, override.Settings),
	}
	return result
}

// mergeComponentConfig combines two ComponentConfigs. Follows the same pattern as adapters.
func mergeComponentConfig(base, override domain.ComponentConfig) domain.ComponentConfig {
	result := domain.ComponentConfig{
		Enabled:  override.Enabled,
		Settings: mergeSettingsMaps(base.Settings, override.Settings),
	}
	return result
}

// mergeSettingsMaps merges two settings maps. Override keys replace base keys.
// Keys present only in base are preserved.
func mergeSettingsMaps(base, override map[string]interface{}) map[string]interface{} {
	if base == nil && override == nil {
		return nil
	}
	result := make(map[string]interface{})
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}

// cloneAdapterConfig returns a deep copy of an AdapterConfig.
func cloneAdapterConfig(ac domain.AdapterConfig) domain.AdapterConfig {
	clone := domain.AdapterConfig{
		Enabled:  ac.Enabled,
		Settings: cloneSettingsMap(ac.Settings),
	}
	return clone
}

// cloneComponentConfig returns a deep copy of a ComponentConfig.
func cloneComponentConfig(cc domain.ComponentConfig) domain.ComponentConfig {
	clone := domain.ComponentConfig{
		Enabled:  cc.Enabled,
		Settings: cloneSettingsMap(cc.Settings),
	}
	return clone
}

// cloneSettingsMap returns a shallow copy of a settings map.
func cloneSettingsMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	clone := make(map[string]interface{}, len(m))
	for k, v := range m {
		clone[k] = v
	}
	return clone
}

// cloneAgentDefs returns a copy of an agent definitions map.
func cloneAgentDefs(defs map[string]domain.AgentDef) map[string]domain.AgentDef {
	clone := make(map[string]domain.AgentDef, len(defs))
	for k, v := range defs {
		clone[k] = v
	}
	return clone
}

// cloneSkillDefs returns a copy of a skill definitions map.
func cloneSkillDefs(defs map[string]domain.SkillDef) map[string]domain.SkillDef {
	clone := make(map[string]domain.SkillDef, len(defs))
	for k, v := range defs {
		clone[k] = v
	}
	return clone
}

// cloneCommandDefs returns a copy of a command definitions map.
func cloneCommandDefs(defs map[string]domain.CommandDef) map[string]domain.CommandDef {
	clone := make(map[string]domain.CommandDef, len(defs))
	for k, v := range defs {
		clone[k] = v
	}
	return clone
}

// cloneMCPDefs returns a copy of an MCP server definitions map.
func cloneMCPDefs(defs map[string]domain.MCPServerDef) map[string]domain.MCPServerDef {
	clone := make(map[string]domain.MCPServerDef, len(defs))
	for k, v := range defs {
		clone[k] = v
	}
	return clone
}
