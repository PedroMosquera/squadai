# Agent Manager Pro

Standardize AI coding agent environments across your team.

[![Build](https://github.com/PedroMosquera/agent-manager-pro/actions/workflows/ci.yml/badge.svg)](https://github.com/PedroMosquera/agent-manager-pro/actions)
[![Go](https://img.shields.io/github/go-mod/go-version/PedroMosquera/agent-manager-pro)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## What It Does

One command configures every AI coding agent on your team to use the same methodology, team structure, MCP servers, and coding standards.

```sh
agent-manager init --methodology tdd
agent-manager apply
```

That's it. Every developer on the team gets identical agent configurations, regardless of which editor they use.

**5 supported agents.** OpenCode, Claude Code, VS Code Copilot, Cursor, Windsurf.

**9 components.** Memory protocols, team rules, editor settings, MCP servers, team agents, methodology skills, commands, plugins, workflows.

**3 methodologies.** TDD (6 roles), SDD (8 roles), Conventional (4 roles) — each with an orchestrator that delegates to specialized sub-agents.

**3 delegation strategies.** Native sub-agent files (OpenCode, Cursor), prompt-based Task tool injection (Claude Code), solo all-in-one prompts (VS Code, Windsurf).

**MCP integration.** Context7 enabled by default. Each agent gets MCP config in its native format.

**Community skills.** Vercel skills ecosystem integration via the `find-skills` shared skill.

---

## Quick Start

```sh
# Install
go install github.com/PedroMosquera/agent-manager-pro/cmd/agent-manager@latest

# Initialize with TDD methodology
agent-manager init --methodology tdd

# Preview changes
agent-manager plan --dry-run

# Apply configuration
agent-manager apply

# Verify compliance
agent-manager verify
```

Run `agent-manager` with no arguments to launch the interactive TUI wizard (methodology, MCP servers, plugins, summary).

---

## Supported Agents

| Agent | Config File | Delegation | MCP Strategy |
|-------|------------|------------|--------------|
| OpenCode | `AGENTS.md` | native (sub-agent files in `.opencode/agents/`) | MergeIntoSettings (`.opencode/config.json` `"mcp"` key) |
| Claude Code | `CLAUDE.md` | prompt (Task tool injection) | SeparateMCPFiles (`~/.claude/mcp/<server>.json`) |
| VS Code Copilot | `.github/copilot-instructions.md` | solo (all-in-one prompt) | MCPConfigFile (`.vscode/mcp.json`) |
| Cursor | `.cursorrules` | native (agent files in `.cursor/agents/`) | MCPConfigFile (`.cursor/mcp.json`) |
| Windsurf | `.windsurfrules` | solo + workflows | MCPConfigFile (`.windsurf/mcp_config.json`) |

All detected agents are auto-enabled during `init`. The planner generates actions only for agents actually installed on each developer's machine.

---

## Supported Languages

| Language | Auto-detection | Standards Included |
|----------|---------------|-------------------|
| Go | `go.mod` | Error wrapping, table-driven tests, `context.Context`, MixedCaps |
| TypeScript | `tsconfig.json` | `strict: true`, discriminated unions, explicit return types |
| JavaScript | `package.json` | Module patterns, ESLint + Prettier, `const` by default |
| Python | `pyproject.toml`, `requirements.txt` | Type hints, dataclasses, ruff, pytest fixtures |
| Rust | `Cargo.toml` | Ownership, `thiserror`/`anyhow`, Clippy lints |
| Java | `pom.xml`, `build.gradle` | Sealed classes, records, Optional, Javadoc |
| Kotlin | `build.gradle.kts` | Shared Java standards |
| Ruby | `Gemfile` | RuboCop, minitest/RSpec, frozen string literals |
| C# | `*.csproj`, `*.sln` | Nullable refs, async/await, EditorConfig |
| PHP | `composer.json` | PSR-12, PHPStan, type declarations |
| Swift | `Package.swift`, `*.xcodeproj` | SwiftLint, value types, protocol-oriented |
| C/C++ | `CMakeLists.txt` | Smart pointers, RAII, clang-tidy |
| Dart | `pubspec.yaml` | Null safety, Flutter widgets, effective Dart |
| Elixir | `mix.exs` | Pattern matching, GenServer, dialyzer |
| Scala | `build.sbt` | Case classes, implicits, ScalaTest |

Monorepo support: when multiple languages are detected, all language standards are combined.

---

## Components

`agent-manager apply` installs up to 9 components per agent:

| Component | ID | What It Installs |
|-----------|-----|-----------------|
| Memory Protocol | `memory` | Session persistence files (`AGENTS.md`, `CLAUDE.md`) with marker blocks |
| Team Rules | `rules` | Team coding standards injected into each agent's system prompt |
| Editor Settings | `settings` | Agent-specific config files (`.opencode/config.json`, `.vscode/settings.json`, etc.) |
| MCP Servers | `mcp` | MCP server definitions in each agent's native format |
| Team Agents | `agents` | Sub-agent role definitions (orchestrator, implementer, reviewer, etc.) |
| Methodology Skills | `skills` | Skill files for each methodology phase (TDD red-green-refactor, SDD spec workflow, etc.) |
| Commands | `commands` | Agent-specific command definitions (OpenCode `.opencode/commands/`) |
| Plugins | `plugins` | Third-party plugin installation (Claude Code plugins, skill files) |
| Workflows | `workflows` | Agent-specific workflow files (Windsurf `.windsurf/workflows/`) |

Not every agent supports every component. Each adapter declares which components it handles; the planner skips unsupported combinations.

---

## Methodologies

Each methodology defines a team of roles with an orchestrator that delegates work to specialized sub-agents.

### TDD (6 roles)

| Role | Description |
|------|-------------|
| orchestrator | Delegates phases to specialized sub-agents |
| brainstormer | Requirements exploration and question-asking |
| planner | Test plan and implementation plan creation |
| implementer | Red-green-refactor implementation cycles |
| reviewer | Two-stage code review: automated + design |
| debugger | 4-phase debugging: reproduce, isolate, fix, verify |

### SDD (8 roles)

| Role | Description |
|------|-------------|
| orchestrator | Manages spec-driven workflow |
| explorer | Codebase analysis and context gathering |
| proposer | Solution proposals with tradeoff analysis |
| spec-writer | Formal specification document authoring |
| designer | Architecture and interface design |
| task-planner | Dependency-ordered task breakdown |
| implementer | Spec-faithful implementation |
| verifier | Spec compliance verification |

### Conventional (4 roles)

| Role | Description |
|------|-------------|
| orchestrator | Direct implementation with review gates |
| implementer | General-purpose implementation |
| reviewer | Code review checklist |
| tester | Test writing and coverage |

---

## Configuration

### Three-Layer Merge

Configuration follows strict precedence:

```
Policy (locked fields)  >  Project config  >  User defaults
```

| Layer | File | Scope |
|-------|------|-------|
| User defaults | `~/.agent-manager/config.json` | Personal preferences, backup paths |
| Project config | `.agent-manager/project.json` | Per-repo settings, methodology, team, MCP |
| Team policy | `.agent-manager/policy.json` | Locked fields that cannot be overridden |

### Example `project.json`

```json
{
  "version": 1,
  "methodology": "tdd",
  "adapters": {
    "opencode": { "enabled": true },
    "claude-code": { "enabled": true },
    "vscode-copilot": { "enabled": true },
    "cursor": { "enabled": true },
    "windsurf": { "enabled": true }
  },
  "components": {
    "memory": { "enabled": true },
    "rules": { "enabled": true },
    "settings": { "enabled": true },
    "mcp": { "enabled": true },
    "agents": { "enabled": true },
    "skills": { "enabled": true },
    "commands": { "enabled": true },
    "plugins": { "enabled": true },
    "workflows": { "enabled": true }
  },
  "copilot": {
    "instructions_template": "standard"
  },
  "mcp": {
    "context7": {
      "type": "local",
      "command": ["npx", "-y", "@upstash/context7-mcp@latest"],
      "enabled": true
    }
  },
  "team": {
    "orchestrator": { "description": "TDD orchestrator", "mode": "subagent" },
    "brainstormer": { "description": "Requirements exploration", "mode": "subagent", "skill_ref": "tdd/brainstorming" },
    "planner": { "description": "Test plan creation", "mode": "subagent", "skill_ref": "tdd/writing-plans" },
    "implementer": { "description": "Red-green-refactor cycles", "mode": "subagent", "skill_ref": "tdd/test-driven-development" },
    "reviewer": { "description": "Two-stage code review", "mode": "subagent", "skill_ref": "shared/code-review" },
    "debugger": { "description": "Systematic debugging", "mode": "subagent", "skill_ref": "tdd/systematic-debugging" }
  },
  "meta": {
    "name": "my-project",
    "language": "go",
    "test_command": "go test -race ./...",
    "build_command": "go build ./..."
  }
}
```

### Operational Modes

| Mode | Behavior |
|------|----------|
| `team` | Policy-controlled. Required settings enforced, locked fields immutable. |
| `personal` | User-controlled. Optional adapters and personal defaults. |
| `hybrid` | Both active. Policy locked fields take precedence over user/project values. |

---

## Commands

| Command | Description |
|---------|-------------|
| `agent-manager init` | Initialize project config and detect agents |
| `agent-manager plan` | Compute and display the action plan |
| `agent-manager diff` | Preview what apply would change (unified diffs) |
| `agent-manager apply` | Execute plan with backup and rollback safety |
| `agent-manager sync` | Idempotent reconciliation (alias for apply) |
| `agent-manager verify` | Run compliance checks and print health report |
| `agent-manager status` | Show project health: adapters, components, managed files |
| `agent-manager validate-policy` | Validate policy schema and lock/required consistency |
| `agent-manager backup create` | Manually snapshot managed files |
| `agent-manager backup list` | List available backups |
| `agent-manager backup delete <id>` | Delete a specific backup snapshot |
| `agent-manager backup prune --keep=N` | Remove old backups, keep N most recent |
| `agent-manager restore <id>` | Restore files from a backup |
| `agent-manager remove --force` | Remove all managed files and strip marker blocks |
| `agent-manager version` | Print version |

### Flags

| Flag | Commands | Description |
|------|----------|-------------|
| `--methodology=<tdd\|sdd\|conventional>` | `init` | Set development methodology |
| `--mcp=<csv>` | `init` | Comma-separated MCP server IDs to enable |
| `--plugins=<csv>` | `init` | Comma-separated plugin IDs to enable |
| `--with-policy` | `init` | Generate team policy template |
| `--force` | `init`, `remove` | Overwrite existing template and skill files; required for remove |
| `--merge` | `init` | Re-run init, merge new config on top of existing (preserves customizations) |
| `--dry-run` | `plan`, `apply`, `sync`, `restore` | Preview changes without writing files |
| `--json` | `plan`, `apply`, `verify`, `backup` | Machine-readable JSON output |
| `--keep=N` | `backup prune` | Number of backups to retain (default 10) |

### Interactive TUI

Run `agent-manager` with no arguments for a guided wizard:

1. Intro screen with detected agents and mode
2. Methodology selection (TDD / SDD / Conventional)
3. MCP server configuration
4. Plugin selection (filtered by methodology and detected agents)
5. Summary and confirmation
6. Menu: Plan, Apply, Sync, Verify, Restore, Quit

---

## Architecture

Six-layer architecture with strict dependency direction:

```
Layer 1: Domain       Types, interfaces, errors (no side effects)
Layer 2: Config       Three-layer merge with policy enforcement
Layer 3: Planner      Compute actions from merged config + detected adapters
Layer 4: Pipeline     Execute actions with backup/rollback safety
Layer 5: Verifier     Post-apply compliance assertions
Layer 6: Interfaces   CLI commands, TUI wizard
```

Key design principles:

- **Adapters own all paths.** No hardcoded agent paths outside adapter packages.
- **Marker blocks for managed content.** User content outside `<!-- agent-manager:SECTION -->` markers is never modified.
- **Atomic writes.** Temp file + rename via `fileutil.WriteAtomic` for crash safety.
- **Idempotent by default.** Planner returns `ActionSkip` when state matches; writer skips when bytes are identical.
- **Fail loudly.** Errors are always surfaced, never silently swallowed.

Full architecture details: [`docs/architecture.md`](docs/architecture.md)

### Safety

Every `apply` operation:

1. Snapshots all target files to a backup manifest
2. Executes steps in deterministic order with step-level logging
3. On failure, rolls back all completed steps from the manifest
4. Emits a failure summary with the backup ID for manual recovery

---

## Installation

### From source (requires Go 1.24+)

```sh
go install github.com/PedroMosquera/agent-manager-pro/cmd/agent-manager@latest
```

### GitHub Releases

Download the binary for your architecture from [Releases](https://github.com/PedroMosquera/agent-manager-pro/releases).

### Shell script

```sh
curl -sSL https://raw.githubusercontent.com/PedroMosquera/agent-manager-pro/main/scripts/install.sh | sh
```

### Homebrew

Coming soon.

---

## Platform Support

| Platform | Status |
|----------|--------|
| macOS (darwin/arm64, darwin/amd64) | Fully supported |
| Linux (linux/amd64, linux/arm64) | Supported |
| Windows | Planned |

---

## Roadmap

V2 architecture is complete: 5 agents, 9 component installers, 3 methodologies, 3 delegation strategies, 3 MCP strategies, 1000+ tests. See the [V2.1 Follow-up Roadmap](.x9k4v/ROADMAP-V2.1.md) for activation, documentation, and distribution work in progress.

---

## License

MIT
