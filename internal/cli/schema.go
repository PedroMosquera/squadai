package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/squadai/internal/exitcode"
)

// RunSchema handles the `squadai schema` subcommand group.
func RunSchema(args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprintln(stdout, "Usage: squadai schema <subcommand>")
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Subcommands:")
		fmt.Fprintln(stdout, "  export   Write JSON Schema for project.json and/or policy.json")
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Run 'squadai schema export --help' for details.")
		return nil
	}
	switch args[0] {
	case "export":
		return RunSchemaExport(args[1:], stdout)
	default:
		return fmt.Errorf("unknown schema subcommand %q", args[0])
	}
}

// RunSchemaExport writes JSON Schema documents for project.json and/or policy.json.
func RunSchemaExport(args []string, stdout io.Writer) error {
	format := "all"
	outDir := ""

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-h" || args[i] == "--help":
			fmt.Fprintln(stdout, "Usage: squadai schema export [--format=project|policy|all] [--out=<dir>]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Export JSON Schema files for SquadAI config files. These schemas enable")
			fmt.Fprintln(stdout, "VS Code inline validation when referenced via json.schemaStore or")
			fmt.Fprintln(stdout, "a .vscode/settings.json 'json.schemas' entry.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --format=<project|policy|all>   Which schema(s) to export (default: all)")
			fmt.Fprintln(stdout, "  --out=<dir>                     Write to files in directory (default: stdout)")
			return nil
		case strings.HasPrefix(args[i], "--format="):
			format = strings.TrimPrefix(args[i], "--format=")
		case strings.HasPrefix(args[i], "--out="):
			outDir = strings.TrimPrefix(args[i], "--out=")
		}
	}

	switch format {
	case "project", "policy", "all":
	default:
		return exitcode.ErrUnknownValue("--format", format, "project, policy, all")
	}

	type schemaFile struct {
		name    string
		content func() ([]byte, error)
	}

	var files []schemaFile
	if format == "project" || format == "all" {
		files = append(files, schemaFile{"project.schema.json", buildProjectSchema})
	}
	if format == "policy" || format == "all" {
		files = append(files, schemaFile{"policy.schema.json", buildPolicySchema})
	}

	for _, f := range files {
		data, err := f.content()
		if err != nil {
			return fmt.Errorf("build %s: %w", f.name, err)
		}
		if outDir != "" {
			dest := filepath.Join(outDir, f.name)
			if err := os.MkdirAll(outDir, 0755); err != nil {
				return exitcode.ErrPermission(outDir, err)
			}
			if err := os.WriteFile(dest, data, 0644); err != nil {
				return exitcode.ErrPermission(dest, err)
			}
			fmt.Fprintf(stdout, "wrote %s\n", dest)
		} else {
			if len(files) > 1 {
				fmt.Fprintf(stdout, "// --- %s ---\n", f.name)
			}
			fmt.Fprintln(stdout, string(data))
		}
	}
	return nil
}

// buildProjectSchema returns the JSON Schema for .squadai/project.json.
func buildProjectSchema() ([]byte, error) {
	schema := map[string]any{
		"$schema":     "http://json-schema.org/draft-07/schema#",
		"$id":         "https://squadai.dev/schemas/project.json",
		"title":       "SquadAI project.json",
		"description": "Project-level configuration for SquadAI agent team management.",
		"type":        "object",
		"required":    []string{"version"},
		"properties": map[string]any{
			"version": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "Schema version. Must be >= 1.",
			},
			"methodology": map[string]any{
				"type":        "string",
				"enum":        []string{"tdd", "sdd", "conventional"},
				"description": "Development methodology. Determines default agent team composition.",
			},
			"model_tier": map[string]any{
				"type":        "string",
				"enum":        []string{"balanced", "performance", "starter", "manual"},
				"description": "Default model tier for agents. 'balanced' is recommended for most teams.",
			},
			"adapters": map[string]any{
				"type":        "object",
				"description": "Per-adapter enable/disable and settings.",
				"additionalProperties": map[string]any{
					"type":     "object",
					"required": []string{"enabled"},
					"properties": map[string]any{
						"enabled":  map[string]any{"type": "boolean"},
						"settings": map[string]any{"type": "object", "additionalProperties": true},
					},
				},
			},
			"components": map[string]any{
				"type":        "object",
				"description": "Per-component enable/disable (memory, rules, settings, mcp, agents, skills, commands, plugins, workflows).",
				"additionalProperties": map[string]any{
					"type":     "object",
					"required": []string{"enabled"},
					"properties": map[string]any{
						"enabled":  map[string]any{"type": "boolean"},
						"settings": map[string]any{"type": "object", "additionalProperties": true},
					},
				},
			},
			"rules": map[string]any{
				"type":        "object",
				"description": "Team standards content injected into agent instruction files.",
				"properties": map[string]any{
					"team_standards":      map[string]any{"type": "string", "description": "Inline markdown team standards content."},
					"team_standards_file": map[string]any{"type": "string", "description": "Path relative to .squadai/ containing team standards."},
					"instructions":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
			},
			"agents": map[string]any{
				"type":                 "object",
				"description":          "Custom agent definitions.",
				"additionalProperties": schemaAgentDef(),
			},
			"skills": map[string]any{
				"type":        "object",
				"description": "Custom skill definitions.",
				"additionalProperties": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"description":  map[string]any{"type": "string"},
						"content":      map[string]any{"type": "string"},
						"content_file": map[string]any{"type": "string"},
					},
				},
			},
			"commands": map[string]any{
				"type":        "object",
				"description": "Custom command definitions.",
				"additionalProperties": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"description":  map[string]any{"type": "string"},
						"content":      map[string]any{"type": "string"},
						"content_file": map[string]any{"type": "string"},
					},
				},
			},
			"mcp": map[string]any{
				"type":                 "object",
				"description":          "MCP server definitions to inject into agent settings.",
				"additionalProperties": schemaMCPDef(),
			},
			"team": map[string]any{
				"type":        "object",
				"description": "Team role definitions for methodology-based agent teams.",
				"additionalProperties": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"description":  map[string]any{"type": "string"},
						"mode":         map[string]any{"type": "string", "enum": []string{"subagent", "inline"}},
						"skill_ref":    map[string]any{"type": "string"},
						"delegates_to": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"model":        map[string]any{"type": "string", "enum": []string{"premium", "standard", "cheap"}},
					},
				},
			},
			"claude": map[string]any{
				"type":        "object",
				"description": "Claude Code-specific feature toggles.",
				"properties": map[string]any{
					"agent_teams": map[string]any{
						"type":        "object",
						"description": "Opt into Claude Code's experimental Agent Teams runtime.",
						"properties": map[string]any{
							"enabled": map[string]any{"type": "boolean"},
						},
					},
				},
			},
			"hooks": schemaHooksConfig(),
			"plugins": map[string]any{
				"type":                 "object",
				"description":          "Plugin definitions for community marketplace plugins.",
				"additionalProperties": schemaPluginDef(),
			},
			"marketplace": map[string]any{
				"type":        "object",
				"description": "Marketplace tracking — which plugins are installed.",
				"properties": map[string]any{
					"source":  map[string]any{"type": "string"},
					"plugins": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
				},
			},
			"meta": map[string]any{
				"type":        "object",
				"description": "Project metadata used in template rendering.",
			},
			"copilot": map[string]any{
				"type":        "object",
				"description": "VS Code Copilot instructions configuration.",
				"properties": map[string]any{
					"instructions_template": map[string]any{"type": "string"},
					"custom_content":        map[string]any{"type": "string"},
				},
			},
		},
		"additionalProperties": false,
	}
	return json.MarshalIndent(schema, "", "  ")
}

// buildPolicySchema returns the JSON Schema for .squadai/policy.json.
func buildPolicySchema() ([]byte, error) {
	schema := map[string]any{
		"$schema":     "http://json-schema.org/draft-07/schema#",
		"$id":         "https://squadai.dev/schemas/policy.json",
		"title":       "SquadAI policy.json",
		"description": "Team policy configuration for SquadAI. Locked fields override user and project configs.",
		"type":        "object",
		"required":    []string{"version", "mode"},
		"properties": map[string]any{
			"version": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "Schema version. Must be >= 1.",
			},
			"mode": map[string]any{
				"type":        "string",
				"enum":        []string{"team", "personal"},
				"description": "Operational mode. 'team' enforces policy on all devs.",
			},
			"locked": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Dot-path fields that cannot be overridden by user or project config (e.g., 'adapters.claude.enabled').",
			},
			"required": map[string]any{
				"type":        "object",
				"description": "Values that are enforced regardless of user/project settings.",
				"properties": map[string]any{
					"adapters": map[string]any{
						"type": "object",
						"additionalProperties": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"enabled":  map[string]any{"type": "boolean"},
								"settings": map[string]any{"type": "object", "additionalProperties": true},
							},
						},
					},
					"components": map[string]any{
						"type": "object",
						"additionalProperties": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"enabled":  map[string]any{"type": "boolean"},
								"settings": map[string]any{"type": "object", "additionalProperties": true},
							},
						},
					},
					"agents":  map[string]any{"type": "object", "additionalProperties": schemaAgentDef()},
					"mcp":     map[string]any{"type": "object", "additionalProperties": schemaMCPDef()},
					"plugins": map[string]any{"type": "object", "additionalProperties": schemaPluginDef()},
					"rules": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"team_standards":      map[string]any{"type": "string"},
							"team_standards_file": map[string]any{"type": "string"},
						},
					},
					"copilot": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"instructions_template": map[string]any{"type": "string"},
							"custom_content":        map[string]any{"type": "string"},
						},
					},
					"claude": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"agent_teams": map[string]any{
								"type":       "object",
								"properties": map[string]any{"enabled": map[string]any{"type": "boolean"}},
							},
						},
					},
					"hooks": schemaHooksConfig(),
				},
			},
		},
		"additionalProperties": false,
	}
	return json.MarshalIndent(schema, "", "  ")
}

func schemaAgentDef() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Agent definition.",
		"properties": map[string]any{
			"description": map[string]any{"type": "string"},
			"mode":        map[string]any{"type": "string"},
			"model":       map[string]any{"type": "string"},
			"prompt":      map[string]any{"type": "string"},
			"prompt_file": map[string]any{"type": "string"},
			"tools":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"skills":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"max_turns":   map[string]any{"type": "integer"},
			"memory":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"effort":      map[string]any{"type": "string", "enum": []string{"low", "normal", "high"}},
			"permission":  map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
		},
	}
}

func schemaMCPDef() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "MCP server definition.",
		"properties": map[string]any{
			"command": map[string]any{"type": "string"},
			"args":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"env":     map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			"url":     map[string]any{"type": "string"},
		},
	}
}

func schemaPluginDef() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Plugin definition.",
		"properties": map[string]any{
			"description":          map[string]any{"type": "string"},
			"enabled":              map[string]any{"type": "boolean"},
			"supported_agents":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"install_method":       map[string]any{"type": "string", "enum": []string{"claude_plugin", "skill_files"}},
			"plugin_id":            map[string]any{"type": "string"},
			"excludes_methodology": map[string]any{"type": "string"},
		},
	}
}

func schemaHooksConfig() map[string]any {
	hookEntry := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type":    map[string]any{"type": "string", "const": "command"},
			"command": map[string]any{"type": "string"},
			"async":   map[string]any{"type": "boolean"},
			"timeout": map[string]any{"type": "integer"},
		},
		"required": []string{"type", "command"},
	}
	hookMatcher := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"matcher": map[string]any{"type": "string"},
			"hooks":   map[string]any{"type": "array", "items": hookEntry},
		},
		"required": []string{"hooks"},
	}
	return map[string]any{
		"type":                 "object",
		"description":          "Claude Code hook event handlers. Keys: PreToolUse, PostToolUse, Stop, UserPromptSubmit, SubagentStart, SubagentStop.",
		"additionalProperties": map[string]any{"type": "array", "items": hookMatcher},
	}
}

// ─── Context dump ─────────────────────────────────────────────────────────────
