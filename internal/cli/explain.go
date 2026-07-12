package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/PedroMosquera/squadai/internal/exitcode"
)

// RunExplain provides documentation on config fields, error codes, and merge trace.
func RunExplain(args []string, stdout io.Writer) error {
	topic := ""
	jsonOut := false

	for _, arg := range args {
		switch {
		case arg == "-h" || arg == "--help":
			fmt.Fprintln(stdout, "Usage: squadai explain [<topic>] [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Explain a SquadAI concept, config field, or error code.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Topics:")
			fmt.Fprintln(stdout, "  config           Explain the project.json config structure")
			fmt.Fprintln(stdout, "  policy           Explain the policy.json structure and locked fields")
			fmt.Fprintln(stdout, "  methodology      Explain available methodologies (tdd, sdd, conventional)")
			fmt.Fprintln(stdout, "  adapters         Explain supported adapters and their capabilities")
			fmt.Fprintln(stdout, "  components       Explain available components (memory, rules, mcp, agents, brand, etc.)")
			fmt.Fprintln(stdout, "  brand            Explain the brand banner component")
			fmt.Fprintln(stdout, "  budget           Explain token budget management")
			fmt.Fprintln(stdout, "  error-codes      Explain error codes (E-1xx through E-8xx)")
			fmt.Fprintln(stdout, "  merge            Show the config merge trace (user → project → policy)")
			fmt.Fprintln(stdout, "  drift            Explain drift detection and the governance workflow")
			fmt.Fprintln(stdout, "  mcp              Explain MCP server configuration")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json   Return explanation as JSON")
			return nil
		case arg == "--json":
			jsonOut = true
		default:
			if topic == "" {
				topic = arg
			}
		}
	}

	if topic == "" {
		// List available topics.
		if jsonOut {
			topics := []string{"config", "policy", "methodology", "adapters", "components", "brand", "budget", "error-codes", "merge", "drift", "mcp"}
			data, _ := json.Marshal(map[string]any{"topics": topics})
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		fmt.Fprintln(stdout, "Usage: squadai explain <topic>")
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Available topics: config, policy, methodology, adapters, components,")
		fmt.Fprintln(stdout, "  brand, budget, error-codes, merge, drift, mcp")
		return nil
	}

	explanation, ok := explainTopic(topic)
	if !ok {
		return exitcode.ErrNotFound(fmt.Sprintf("topic %q", topic))
	}

	if jsonOut {
		data, _ := json.MarshalIndent(map[string]any{
			"topic":       topic,
			"explanation": explanation,
		}, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintln(stdout, explanation)
	return nil
}

// explainTopic returns the documentation for a known topic.
func explainTopic(topic string) (string, bool) {
	switch topic {
	case "config":
		return `# project.json — Project Configuration

Located at .squadai/project.json. Controls agent team setup for this project.

## Top-level fields

  version        (int, required) Schema version. Must be >= 1.
  methodology    (string) Development methodology: tdd, sdd, or conventional.
  model_tier     (string) Default model tier: balanced, performance, starter, manual.
  adapters       (object) Per-adapter enable/disable (claude, cursor, windsurf, vscode, opencode).
  components     (object) Per-component enable/disable (memory, rules, settings, mcp, agents, ...).
  rules          (object) Team standards: team_standards (inline) or team_standards_file (path).
  agents         (object) Custom agent definitions.
  skills         (object) Custom skill definitions.
  commands       (object) Custom command definitions.
  mcp            (object) MCP server definitions.
  team           (object) Team role definitions (used with methodology).
  claude         (object) Claude Code-specific options (agent_teams.enabled).
  hooks          (object) Claude Code hook event handlers.
  plugins        (object) Community marketplace plugin definitions.

## Three-layer merge

  ~/.squadai/config.json   (user) — lowest priority, personal preferences
  .squadai/project.json    (project) — mid layer, team defaults
  .squadai/policy.json     (policy) — highest priority, team mandates

Run 'squadai explain merge' for details on how layers combine.`, true

	case "policy":
		return `# policy.json — Team Policy Configuration

Located at .squadai/policy.json. Enforced on top of project and user configs.

## Fields

  version   (int, required) Schema version >= 1.
  mode      (string, required) "team" or "personal".
  locked    ([]string) Dot-path fields that cannot be overridden.
             Example: ["adapters.claude.enabled", "claude.agent_teams.enabled"]
  required  (object) Values enforced on top of user/project config.
             Contains: adapters, components, agents, mcp, rules, copilot, claude, hooks, plugins.

## Locked fields

Locked fields prevent user/project config from overriding policy values.
If a user tries to disable an adapter that policy locks as enabled,
the policy value wins and a violation warning is recorded.

## Example

  {
    "version": 1,
    "mode": "team",
    "locked": ["adapters.claude.enabled"],
    "required": {
      "adapters": { "claude": { "enabled": true } }
    }
  }

Run 'squadai validate-policy' to check for issues.`, true

	case "methodology":
		return `# Methodologies

SquadAI supports three built-in development methodologies. Each generates
a default team composition of named roles that Claude Code's Agent Teams
runtime can orchestrate.

## tdd — Test-Driven Development

6 roles: orchestrator, architect, backend-dev, frontend-dev, tester, reviewer
Focus: red/green/refactor cycle with test coverage gates.

## sdd — Spec-Driven Development

8 roles: orchestrator, architect, spec-writer, backend-dev, frontend-dev,
         tester, reviewer, documenter
Focus: specification-first, with explicit review and documentation phases.

## conventional — Conventional Workflow

4 roles: orchestrator, architect, developer, reviewer
Focus: balanced general-purpose software development without methodology lock-in.

## Usage

  squadai init --methodology=tdd
  squadai explain methodology   (this page)`, true

	case "adapters":
		return `# Adapters

SquadAI manages configurations for seven AI coding agents ("adapters"):

  claude     Claude Code — .claude/CLAUDE.md, .claude/agents/, .claude/settings.json
             Delegation: multi-agent (orchestrator + subagents)
             Supports: Agent Teams experimental runtime

  cursor     Cursor IDE — .cursor/rules/*.mdc, .cursor/mcp.json
             Delegation: multi-agent (with Cursor background agents)

  windsurf   Windsurf — .windsurf/rules/squadai.md, .windsurf/mcp_config.json
             Delegation: solo agent (inline only)
             NOTE: global_rules.md has a 6,000-character cap

  opencode   OpenCode — AGENTS.md, .opencode/agents/, .opencode/skills/
             Delegation: multi-agent (subagents)

  vscode     VS Code Copilot — .github/copilot-instructions.md
             Delegation: solo agent (inline only)

  pi         Pi Agent — ~/.pi/agent/AGENTS.md, ~/.pi/agent/agents/, ~/.pi/agent/prompts/
             Delegation: multi-agent (native subagents)
             Commands render as prompt templates in prompts/
             Supports per-agent banner branding

  codex      OpenAI Codex CLI — AGENTS.md, ~/.codex/AGENTS.md, ~/.codex/config.toml
             Delegation: solo agent (inline only)
             MCP servers written as [mcp_servers.*] TOML tables inside a
             squadai-managed marker block in ~/.codex/config.toml

Adapters are auto-detected by checking PATH and config directory presence.
Override detection with 'adapters' in project.json.

## Adapter path overrides (.squadai/adapters/<id>.json)

Built-in adapters are curated defaults. You can override specific path fields
without replacing the adapter entirely by creating a JSON file:

  .squadai/adapters/pi.json:
  {
    "config_dir": "~/.pi/agent",
    "agents_subdir": "agents",
    "delegation": "native"
  }

Fields not specified fall back to the built-in defaults. This lets you adapt
SquadAI to custom agent installs or non-standard config locations.`, true

	case "components":
		return `# Components

Components are the building blocks SquadAI manages per adapter:

  memory      Injects a memory protocol section into agent instruction files.
  rules       Team standards content — injected into CLAUDE.md / AGENTS.md or
              written as a structured rules file (Windsurf, Cursor).
  settings    Agent IDE settings — MCP server injection, env vars, permissions.
  mcp         MCP server definitions written to adapter-specific config files.
  agents      Custom agent definitions written to adapter-specific agent dirs.
  skills      Custom skill files for OpenCode and Claude Code.
  commands    Custom slash commands for Claude Code.
  plugins     Community marketplace plugin installations.
  workflows   Windsurf workflow files.
  permissions Claude Code permission policy.
  copilot     VS Code Copilot instructions.
  hooks       Claude Code hook event handlers.
  brand       ASCII-art banner injected into agent files so developers see a
              visual indicator that SquadAI is active at session start.
              Per-agent themed variants (SquadAI standalone, co-branded for
              OpenCode and Pi). Toggle via project.json:
              "components": { "brand": { "enabled": true } }
              Disable per-apply with: squadai apply --no-brand

Enable/disable per adapter via project.json:
  "components": { "mcp": { "enabled": true }, "agents": { "enabled": false } }`, true

	case "error-codes":
		return `# Error Codes

SquadAI uses structured E-xxx error codes with guided remediation hints.

## E-1xx — Configuration / argument errors

  E-100   Generic config error. Run 'squadai validate-policy'.
  E-101   Unknown value for a flag or field.
  E-102   Conflicting flags provided together.
  E-103   Missing required argument.

## E-2xx — Policy errors

  E-201   Policy violation — locked field overridden.
  E-202   Policy validation failed — check 'squadai validate-policy'.
  E-302   Apply completed with failures — check the apply report.
  E-303   Plan generation failed — check project.json and policy.json.

## E-4xx — Drift errors

  E-401   Drift detected — managed file changed outside SquadAI.
          Fix: run 'squadai diff' then 'squadai apply' to restore.

## E-5xx — Not found

  E-501   Resource not found (plugin, agent, adapter).
  E-502   Backup not found — run 'squadai backup list'.

## E-6xx — Precondition errors

  E-601   Precondition failed — e.g., no project.json, no git repo.
          Fix: run 'squadai init' or ensure you are in a git repository.

## E-7xx — Network errors

  E-701   Network error — check internet connection.

## E-8xx — Permission errors

  E-801   Permission denied — check file permissions.`, true

	case "merge":
		homeDir, _ := os.UserHomeDir()
		projectDir, _ := os.Getwd()
		merged, err := loadAndMerge(homeDir, projectDir)
		if err != nil {
			return fmt.Sprintf(`# Config Merge Trace

SquadAI merges three config layers (lowest → highest priority):

  1. ~/.squadai/config.json    (user layer)
  2. .squadai/project.json     (project layer)
  3. .squadai/policy.json      (policy layer — locked fields win always)

Could not load current config: %v

Run 'squadai init' to create a project.json.`, err), true
		}
		return fmt.Sprintf(`# Config Merge Trace

Current merged configuration (project: %s):

  Mode:        %s
  Methodology: %s
  Model tier:  %s
  Adapters:    %d configured
  Agents:      %d defined
  MCP servers: %d configured
  Violations:  %d (policy fields where user value was overridden)

Merge order (lowest → highest priority):
  1. ~/.squadai/config.json    (user layer)
  2. .squadai/project.json     (project layer)
  3. .squadai/policy.json      (policy layer — locked fields always win)

Run 'squadai verify' to see compliance status.
Run 'squadai status --json' for full structured output.`,
			projectDir,
			merged.Mode, merged.Methodology, merged.ModelTier,
			len(merged.Adapters), len(merged.Agents), len(merged.MCP), len(merged.Violations)), true

	case "drift":
		return `# Drift Detection

Drift occurs when a managed file is changed outside of SquadAI (e.g., manually
edited, overwritten by another tool, or deleted).

## How it works

SquadAI records checksums of managed files in .squadai/managed.json after apply.
The drift detector compares current file contents against these checksums.

## Detection

  squadai verify --strict    Check for drift as part of compliance verification.
  squadai watch              Monitor files in real-time (fsnotify, 300ms debounce).
  squadai audit              View the governance audit log of drift events.

## Recovery

  squadai diff               Show unified diff of what changed.
  squadai apply              Restore managed files to their expected state.
  squadai restore <id>       Restore from a specific backup if apply fails.

## Hooks

Install a pre-commit hook to block commits when drift is detected:
  squadai install-hooks

This runs 'squadai verify --strict' before each commit.`, true

	case "mcp":
		return `# MCP Server Configuration

SquadAI injects MCP (Model Context Protocol) server definitions into each
adapter's settings file (.claude/settings.json, .cursor/mcp.json, etc.).

## project.json structure

  "mcp": {
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp"],
      "env": {}
    }
  }

## Adapter-specific formats

  claude     → .claude/settings.json "mcpServers" (command + args array)
  cursor     → .cursor/mcp.json "mcpServers" (command + args array)
  windsurf   → .windsurf/mcp_config.json "mcpServers" (command + args split)
  opencode   → .opencode/config.json "mcp" (command + args array)
  vscode     → not supported (VS Code manages MCP separately)

## SquadAI as MCP server

SquadAI is itself an MCP tool server ('squadai mcp-server') exposing plan,
apply, verify, status, context, init, doctor, and project memory as tools.

The curated catalog includes a "squadai" entry (pre-checked), so 'squadai init'
+ 'squadai apply' register it in every MCP-capable agent automatically —
Claude Code, OpenCode, Cursor, Windsurf, VS Code Copilot, and Pi all get the
same control plane inside the agent console. Opt out with --mcp=none or by
unchecking it in the wizard.

The registration references the bare "squadai" binary, so it must be on PATH
for agents to start it ('squadai doctor' warns when it is not).

To register manually (Claude Code example):
  { "mcpServers": { "squadai": { "command": "squadai", "args": ["mcp-server"] } } }

Then use 'squadai install-commands' to add slash commands to .claude/commands/.`, true

	case "brand":
		return `# Brand Component

The brand component injects an ASCII-art banner into agent instruction files so
developers see a visual indicator that SquadAI is active when they start a session.

## How it works

When 'squadai apply' runs with the brand component enabled (default: true), it
injects a fenced code block into the agent's system prompt or project rules file,
wrapped in marker blocks for idempotent updates:

  <!-- squadai:brand -->
  ` + "```text" + `
  <ASCII banner>
  ` + "```" + `
  <!-- /squadai:brand -->

## Per-agent themed variants

  SquadAI standalone  — used by Claude Code, Cursor, Windsurf, VS Code Copilot
  SquadAI + OpenCode   — co-branded banner for OpenCode
  SquadAI + Pi         — co-branded banner for Pi Agent

## Configuration

  project.json:
    "components": { "brand": { "enabled": true } }

  Disable per-apply without changing config:
    squadai apply --no-brand

  Check token cost:
    squadai token-budget   (brand appears as its own row)

  Policy enforcement:
    .squadai/policy.json can lock "components.brand.enabled" to enforce
    consistent branding across a team.`, true

	case "budget":
		return `# Token Budget

SquadAI's token budget system helps you understand and control the per-session
token cost of your agent configuration.

## Static estimation

  squadai token-budget          # human-readable per-component breakdown
  squadai token-budget --json   # machine-readable
  squadai token-budget --model=claude-sonnet-4-6

This reads the installed agent files and estimates tokens. Without --model it
uses a chars/4 heuristic; with --model it uses the model-aware tokenizer when
available. The brand component appears as its own row.

## Active fitting

  squadai apply --max-tokens=60000 --fit-model=claude-sonnet-4-6

This renders the planned output, estimates desired tokens, then orders
components by priority and omits lowest-priority content to fit within the
target model's context window:

  Priority (drop lowest first):
    plugins → commands → skills → memory → rules → orchestrator

  Modes: full, summary, omit

Summary mode is recorded for future summary rendering, but currently skips
writing that component rather than writing full content over budget.

The chosen layout is persisted to .squadai/.applied-budget.json so that
'doctor' and 'diff' can detect budget drift (e.g. after swapping agents).

## Per-session telemetry

  squadai token-usage --since 7d   # aggregate real session token usage
  squadai token-usage --watch      # not yet implemented

This will parse agent session transcripts and compute real system+completion
tokens with per-model pricing.`, true

	default:
		return "", false
	}
}
