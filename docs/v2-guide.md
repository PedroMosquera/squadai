# V2 User Guide

squadai V2 adds sub-agent teams, methodology-driven workflows, MCP server management, and plugin integration. This guide covers the concepts and configuration needed to use these features.

---

## Introduction

V1 standardized system prompts and settings across agents. V2 extends this with:

- **Methodology teams.** Choose TDD, SDD, or Conventional. Each methodology generates a team of specialized roles with an orchestrator that delegates work to sub-agents.
- **Delegation strategies.** Native sub-agent files (OpenCode, Cursor), Task tool prompt injection (Claude Code), or solo all-in-one prompts (VS Code, Windsurf). The same team definition produces different file structures per agent.
- **MCP servers.** Context7 is included by default. Each agent receives MCP config in its native format.
- **Plugins.** Third-party capabilities from a curated catalog, filtered by methodology compatibility and detected agents.
- **Methodology skills.** Embedded skill files installed per methodology phase (TDD red-green-refactor, SDD spec workflow, etc.) alongside shared skills (code-review, testing, pr-description).
- **Community skills.** The `find-skills` meta-skill connects to the Vercel skills ecosystem (skills.sh) for discovering and installing 91K+ community skill definitions.

---

## Methodologies

Set the methodology during init:

```sh
squadai init --methodology=tdd
```

Each methodology defines a team composition. The orchestrator coordinates the workflow; sub-agents execute individual phases.

### TDD (Test-Driven Development) -- 6 Roles

| Role | Description | Skill Reference |
|------|-------------|-----------------|
| orchestrator | Delegates phases to specialized sub-agents | -- |
| brainstormer | Requirements exploration and question-asking | `tdd/brainstorming` |
| planner | Test plan and implementation plan creation | `tdd/writing-plans` |
| implementer | Red-green-refactor implementation cycles | `tdd/test-driven-development` |
| reviewer | Two-stage code review: automated + design | `shared/code-review` |
| debugger | 4-phase debugging: reproduce, isolate, fix, verify | `tdd/systematic-debugging` |

Best for: projects that prioritize test coverage and want the red-green-refactor discipline enforced by the orchestrator.

TDD also installs the `tdd/subagent-driven-development` skill, which teaches the orchestrator how to manage the delegation lifecycle.

### SDD (Specification-Driven Development) -- 8 Roles

| Role | Description | Skill Reference |
|------|-------------|-----------------|
| orchestrator | Manages spec-driven workflow | -- |
| explorer | Codebase analysis and context gathering | `sdd/sdd-explore` |
| proposer | Solution proposals with tradeoff analysis | `sdd/sdd-propose` |
| spec-writer | Formal specification document authoring | `sdd/sdd-spec` |
| designer | Architecture and interface design | `sdd/sdd-design` |
| task-planner | Dependency-ordered task breakdown | `sdd/sdd-tasks` |
| implementer | Spec-faithful implementation | `sdd/sdd-apply` |
| verifier | Spec compliance verification | `sdd/sdd-verify` |

Best for: complex systems that need formal specification before implementation. The 7-phase pipeline (explore, propose, spec, design, tasks, apply, verify) ensures designs are validated before code is written.

### Conventional -- 4 Roles

| Role | Description | Skill Reference |
|------|-------------|-----------------|
| orchestrator | Direct implementation with review gates | -- |
| implementer | General-purpose implementation | -- |
| reviewer | Code review checklist | `shared/code-review` |
| tester | Test writing and coverage | `shared/testing` |

Best for: straightforward projects that want lightweight structure without a formal methodology pipeline. Conventional uses only shared skills (no methodology-specific skills are installed).

---

## Team Composition

When you set a methodology, `init` writes the full team definition into `.squadai/project.json` under the `"team"` key and enables the `agents` and `commands` components:

```json
{
  "methodology": "tdd",
  "team": {
    "orchestrator": {
      "description": "TDD orchestrator -- delegates phases to specialized sub-agents",
      "mode": "subagent"
    },
    "brainstormer": {
      "description": "Question-asking and requirements exploration",
      "mode": "subagent",
      "skill_ref": "tdd/brainstorming"
    }
  }
}
```

Each role has:

- **`description`** -- what the role does.
- **`mode`** -- always `"subagent"` in V2.
- **`skill_ref`** -- path to the embedded skill that defines the role's workflow (e.g., `tdd/brainstorming`). The orchestrator has no skill_ref; it uses the orchestrator template instead.

You can customize roles by editing the `"team"` key directly. Add roles, change descriptions, or swap skill references. The planner re-renders all agent files on the next `apply`.

---

## Delegation Strategies

The same team definition produces different file structures depending on the agent's delegation strategy. The strategy is determined by the agent, not by configuration.

### Native (OpenCode, Cursor)

Each team role becomes a separate `.md` file in the agent's project agents directory.

```
.opencode/agents/
  orchestrator.md     # Rendered from teams/tdd/orchestrator-native.md
  brainstormer.md     # Rendered from teams/tdd/brainstormer.md
  planner.md          # Rendered from teams/tdd/planner.md
  implementer.md
  reviewer.md
  debugger.md
```

```
.cursor/agents/
  orchestrator.md     # Same templates, different target directory
  brainstormer.md
  ...
```

Each file contains YAML frontmatter (description, mode) and the rendered template content. The orchestrator template includes delegation rules that reference other agent files by name.

### Prompt (Claude Code)

Claude Code does not support named sub-agent files. Instead, the orchestrator template is injected into the project rules file (`CLAUDE.md`) using marker blocks:

```markdown
<!-- squadai:team -->
## TDD Orchestrator
...delegation rules using Task tool...
<!-- squadai:end:team -->
```

The orchestrator instructions tell Claude Code to use the Task tool for delegation. Each sub-agent's role and skill are described inline in the prompt, so the Task tool receives the full context when invoked.

No separate sub-agent files are created. All team instructions live in `CLAUDE.md`.

### Solo (VS Code Copilot, Windsurf)

Solo agents cannot delegate to sub-agents at all. The orchestrator template is injected into the project rules file (`.instructions.md` for VS Code, `.windsurfrules` for Windsurf) using marker blocks:

```markdown
<!-- squadai:team -->
## TDD Orchestrator (Solo Mode)
...all phases executed sequentially inline...
<!-- squadai:end:team -->
```

The solo template instructs the agent to execute all methodology phases sequentially within a single context. Phase boundaries are marked with section headers rather than sub-agent invocations.

### Strategy Summary

| Agent | Strategy | Orchestrator Location | Sub-Agent Files |
|-------|----------|----------------------|-----------------|
| OpenCode | native | `.opencode/agents/orchestrator.md` | `.opencode/agents/<role>.md` |
| Cursor | native | `.cursor/agents/orchestrator.md` | `.cursor/agents/<role>.md` |
| Claude Code | prompt | `CLAUDE.md` (marker block) | None |
| VS Code Copilot | solo | `.instructions.md` (marker block) | None |
| Windsurf | solo | `.windsurfrules` (marker block) | None |

---

## MCP Server Configuration

MCP (Model Context Protocol) servers provide AI agents with live tool access. squadai configures MCP servers in each agent's native format.

### Default Server

Context7 is included and enabled by default:

```json
{
  "context7": {
    "type": "local",
    "command": ["npx", "-y", "@upstash/context7-mcp@latest"],
    "enabled": true
  }
}
```

Context7 provides live documentation lookup for libraries and frameworks, reducing hallucination in generated code.

### Three MCP Strategies

Each agent receives MCP configuration through a different mechanism:

| Strategy | Agent | File | JSON Key |
|----------|-------|------|----------|
| MergeIntoSettings | OpenCode | `opencode.json` | `"mcp"` |
| SeparateMCPFiles | Claude Code | `~/.claude/mcp/<name>.json` | -- (one file per server) |
| MCPConfigFile | VS Code Copilot | `.vscode/mcp.json` | `"mcpServers"` |
| MCPConfigFile | Cursor | `.cursor/mcp.json` | `"mcpServers"` |
| MCPConfigFile | Windsurf | `.windsurf/mcp_config.json` | `"mcpServers"` |

**MergeIntoSettings (OpenCode):** Servers are merged into the project config file under the `"mcp"` key. Other keys in the file are preserved.

**SeparateMCPFiles (Claude Code):** Each server gets its own JSON file at `~/.claude/mcp/{name}.json`. This matches Claude Code's native MCP directory structure.

**MCPConfigFile (VS Code, Cursor, Windsurf):** All servers are written into a dedicated MCP config file under the `"mcpServers"` key.

### Selecting MCP Servers

By default, all recommended servers are included. Use `--mcp` to select specific ones:

```sh
# Include only context7
squadai init --methodology=tdd --mcp=context7

# Include all defaults (same as omitting the flag)
squadai init --methodology=tdd
```

### Adding Custom MCP Servers

Edit `.squadai/project.json` to add servers:

```json
{
  "mcp": {
    "context7": {
      "type": "local",
      "command": ["npx", "-y", "@upstash/context7-mcp@latest"],
      "enabled": true
    },
    "my-server": {
      "type": "local",
      "command": ["node", "path/to/server.js"],
      "enabled": true,
      "environment": {
        "API_KEY": "your-key"
      }
    }
  }
}
```

Remote servers use `"url"` instead of `"command"`:

```json
{
  "my-remote": {
    "type": "remote",
    "url": "https://mcp.example.com",
    "enabled": true,
    "headers": {
      "Authorization": "Bearer token"
    }
  }
}
```

Run `squadai apply` to propagate changes to all agents.

---

## Plugin Catalog

Plugins extend agent capabilities beyond what squadai manages directly. The current catalog:

| Plugin | Description | Supported Agents | Install Method | Constraints |
|--------|-------------|-----------------|----------------|-------------|
| `superpowers` | Advanced AI coding with autonomous workflows | claude-code, opencode, cursor | `claude_plugin` | Excluded when methodology is `tdd` |
| `code-simplifier` | Simplifies and refactors complex code | claude-code | `claude_plugin` | -- |
| `code-review` | Automated code review with actionable feedback | claude-code | `claude_plugin` | -- |
| `frontend-design` | AI-assisted frontend design and component generation | claude-code | `claude_plugin` | -- |

### Enabling Plugins

Use `--plugins` during init:

```sh
squadai init --methodology=sdd --plugins=code-review,code-simplifier
```

Or edit `project.json`:

```json
{
  "plugins": {
    "code-review": {
      "description": "Automated code review with actionable feedback",
      "enabled": true,
      "supported_agents": ["claude-code"],
      "install_method": "claude_plugin",
      "plugin_id": "code-review@anthropic"
    }
  }
}
```

### Plugin Filtering

Plugins are automatically filtered during init based on:

1. **Detected agents.** A plugin is excluded if none of its `supported_agents` are installed on the system.
2. **Methodology exclusion.** The `superpowers` plugin is excluded when the methodology is `tdd` (its `excludes_methodology` field is set to `"tdd"`). This prevents autonomous workflows from bypassing the TDD discipline.

---

## Per-Agent File Breakdown

`squadai apply` writes files to each detected agent. Here is what each agent receives:

### OpenCode

| Component | File Path |
|-----------|-----------|
| System prompt | `AGENTS.md` (project root) |
| Settings | `opencode.json` (project root) |
| MCP | `opencode.json` `"mcp"` key |
| Agents (native) | `.opencode/agents/<role>.md` |
| Skills | `.opencode/skills/<name>/SKILL.md` |
| Commands | `.opencode/commands/<name>.md` |

Supported components: memory, rules, settings, mcp, agents, skills, commands, plugins.

### Claude Code

| Component | File Path |
|-----------|-----------|
| System prompt | `CLAUDE.md` (project root) |
| Team (prompt) | `CLAUDE.md` (marker block) |
| Settings | `.claude/settings.json` |
| MCP | `~/.claude/mcp/<name>.json` |
| Skills | `.claude/skills/<name>/SKILL.md` |

Supported components: memory, rules, settings, skills, mcp, plugins. Does not support agents (uses prompt delegation instead), commands, or workflows.

### Cursor

| Component | File Path |
|-----------|-----------|
| System prompt | `.cursorrules` (project root) |
| Settings | `.cursor/mcp.json` |
| MCP | `.cursor/mcp.json` `"mcpServers"` key |
| Agents (native) | `.cursor/agents/<role>.md` |
| Skills | `.cursor/skills/<name>/SKILL.md` |

Supported components: memory, rules, settings, mcp, agents, skills, plugins.

### VS Code Copilot

| Component | File Path |
|-----------|-----------|
| System prompt | `.instructions.md` (project root) |
| Team (solo) | `.instructions.md` (marker block) |
| Settings | `.vscode/settings.json` |
| MCP | `.vscode/mcp.json` `"mcpServers"` key |
| Skills | `.copilot/skills/<name>/SKILL.md` |

Supported components: memory, rules, settings, mcp, skills.

### Windsurf

| Component | File Path |
|-----------|-----------|
| System prompt | `.windsurfrules` (project root) |
| Team (solo) | `.windsurfrules` (marker block) |
| Settings | `.windsurf/mcp_config.json` |
| MCP | `.windsurf/mcp_config.json` `"mcpServers"` key |
| Skills | `.windsurf/skills/<name>/SKILL.md` |
| Workflows | `.windsurf/workflows/` |

Supported components: memory, rules, settings, mcp, skills, plugins, workflows. Windsurf is the only agent that supports workflow files.

---

## Skills

Skills are markdown files with YAML frontmatter that teach agents specific workflows.

### Shared Skills (All Methodologies)

Installed regardless of methodology choice:

| Skill | Asset Path | Purpose |
|-------|-----------|---------|
| code-review | `shared/code-review` | Structured two-stage code review |
| testing | `shared/testing` | Test writing protocol |
| pr-description | `shared/pr-description` | PR description generation |

### TDD Skills

| Skill | Asset Path | Purpose |
|-------|-----------|---------|
| brainstorming | `tdd/brainstorming` | Requirements exploration |
| writing-plans | `tdd/writing-plans` | Test and implementation planning |
| test-driven-development | `tdd/test-driven-development` | Red-green-refactor cycle |
| subagent-driven-development | `tdd/subagent-driven-development` | Orchestrator delegation lifecycle |
| systematic-debugging | `tdd/systematic-debugging` | 4-phase debugging protocol |

### SDD Skills

| Skill | Asset Path | Purpose |
|-------|-----------|---------|
| sdd-explore | `sdd/sdd-explore` | Codebase analysis |
| sdd-propose | `sdd/sdd-propose` | Solution proposals |
| sdd-spec | `sdd/sdd-spec` | Specification authoring |
| sdd-design | `sdd/sdd-design` | Architecture design |
| sdd-tasks | `sdd/sdd-tasks` | Task breakdown |
| sdd-apply | `sdd/sdd-apply` | Spec-faithful implementation |
| sdd-verify | `sdd/sdd-verify` | Compliance verification |

### Conventional Skills

Conventional uses only shared skills (code-review, testing, pr-description). No methodology-specific skills are installed.

### Installation Layout

Skills are installed to each agent's project skills directory:

```
.opencode/skills/
  shared/code-review/SKILL.md
  shared/testing/SKILL.md
  shared/pr-description/SKILL.md
  tdd/brainstorming/SKILL.md       # TDD only
  tdd/writing-plans/SKILL.md       # TDD only
  ...
```

The same skills are mirrored to `.cursor/skills/`, `.claude/skills/`, `.copilot/skills/`, and `.windsurf/skills/` for their respective agents.

---

## Community Skills

The `find-skills` shared skill enables agents to discover and install community skill definitions from the Vercel skills ecosystem at skills.sh.

### Usage

The agent runs these commands at your request:

```sh
# Search for skills
npx skills find react-testing

# Install a skill
npx skills install react-testing
```

Key facts:

- The registry contains 91K+ skills across 40+ AI agents.
- Skills are static markdown files with no runtime dependency.
- `npx skills` requires Node.js but does not add project dependencies.
- squadai does not depend on this ecosystem. The AI agent runs these commands to extend its own capabilities.

The `find-skills` skill is installed as a shared skill during `init` and written to `.squadai/skills/find-skills.md`.

---

## Configuration

### Three-Layer Merge

Configuration follows strict precedence:

```
Policy (locked fields)  >  Project config  >  User defaults
```

| Layer | File | Scope |
|-------|------|-------|
| User defaults | `~/.squadai/config.json` | Personal preferences, backup paths |
| Project config | `.squadai/project.json` | Per-repo: methodology, team, MCP, plugins |
| Team policy | `.squadai/policy.json` | Locked fields that cannot be overridden |

When a project or user value conflicts with a policy-locked field, the policy value wins and the conflict is recorded as a violation (visible in `plan` output).

### Full `project.json` Example

```json
{
  "version": 1,
  "methodology": "tdd",
  "meta": {
    "name": "my-project",
    "language": "Go",
    "test_command": "go test -race ./...",
    "build_command": "go build ./...",
    "lint_command": "golangci-lint run"
  },
  "adapters": {
    "opencode": { "enabled": true },
    "claude-code": { "enabled": true },
    "vscode-copilot": { "enabled": true },
    "cursor": { "enabled": true },
    "windsurf": { "enabled": true }
  },
  "components": {
    "memory": { "enabled": true },
    "rules": { "enabled": true, "settings": { "team_standards_file": "templates/team-standards.md" } },
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
  "skills": {
    "code-review": { "description": "Structured code review", "content_file": "skills/code-review.md" },
    "testing": { "description": "Test writing protocol", "content_file": "skills/testing.md" },
    "pr-description": { "description": "PR description generation", "content_file": "skills/pr-description.md" },
    "find-skills": { "description": "Find and load available skills", "content_file": "skills/find-skills.md" }
  },
  "plugins": {
    "code-review": {
      "description": "Automated code review with actionable feedback",
      "enabled": true,
      "supported_agents": ["claude-code"],
      "install_method": "claude_plugin",
      "plugin_id": "code-review@anthropic"
    }
  }
}
```

### Project Metadata

The `"meta"` block is auto-detected by `init` (language, project name) and used for template rendering:

| Field | Used In |
|-------|---------|
| `language` | Team standards selection, template `{{.Language}}` |
| `test_command` | Template `{{.TestCommand}}` |
| `build_command` | Template `{{.BuildCommand}}` |
| `lint_command` | Template `{{.LintCommand}}` |
| `name` | Display purposes |

### Customization

- **Add or remove team roles.** Edit `"team"` in `project.json`. Any role name is valid.
- **Change skill references.** Point `skill_ref` to a different embedded or custom skill path.
- **Disable components.** Set `"enabled": false` for any component to skip it during apply.
- **Agent-specific settings.** Each adapter entry in `"adapters"` accepts a `"settings"` map.

### Policy Locking

Teams use `.squadai/policy.json` to enforce configuration:

```json
{
  "version": 1,
  "mode": "team",
  "locked": [
    "adapters.opencode.enabled",
    "components.memory.enabled",
    "copilot.instructions_template"
  ],
  "required": {
    "adapters": { "opencode": { "enabled": true } },
    "components": { "memory": { "enabled": true } },
    "copilot": { "instructions_template": "standard" }
  }
}
```

Locked fields cannot be overridden by project or user config. See [`docs/policy.md`](policy.md) for full policy documentation.

### Operational Modes

| Mode | Behavior |
|------|----------|
| `team` | Policy-controlled. Required settings enforced, locked fields immutable. |
| `personal` | User-controlled. Optional adapters and personal defaults. |
| `hybrid` | Both active. Policy locked fields take precedence over user/project values. |

---

## CLI Reference (V2)

### `squadai init`

Initialize `.squadai/project.json`. Detects installed agents, project language, and writes starter files.

```sh
squadai init [--methodology=<tdd|sdd|conventional>] [--mcp=<csv>] [--plugins=<csv>] [--with-policy] [--force]
```

| Flag | Description |
|------|-------------|
| `--methodology=<tdd\|sdd\|conventional>` | Set the development methodology. Generates team composition and enables agents/commands components. |
| `--mcp=<csv>` | Comma-separated MCP server IDs to enable. Omit to include all recommended servers. |
| `--plugins=<csv>` | Comma-separated plugin IDs to enable. Omit to skip plugin installation. |
| `--with-policy` | Also create `.squadai/policy.json` with a starter template. |
| `--force` | Overwrite existing template and skill files. |

Examples:

```sh
squadai init --methodology=tdd
squadai init --methodology=sdd --mcp=context7 --plugins=code-review
squadai init --with-policy --force
```

### `squadai plan`

Compute the action plan without writing files.

```sh
squadai plan [--dry-run] [--json]
```

Covers all 9 components (memory, rules, settings, mcp, agents, skills, commands, plugins, workflows) across all 5 supported agents.

### `squadai apply`

Execute the plan with backup and rollback safety.

```sh
squadai apply [--dry-run] [--json]
```

All managed files are backed up before changes. If any step fails, all completed changes are rolled back. The backup ID is printed for manual recovery.

### `squadai sync`

Idempotent reconciliation. Identical to `apply` but emphasizes that running it multiple times produces the same result. Safe for CI.

```sh
squadai sync [--dry-run] [--json]
```

### `squadai verify`

Run compliance checks against the current project configuration.

```sh
squadai verify [--json]
```

Checks that all enabled components are correctly installed for each detected agent: expected files exist, marker blocks are present, settings are valid.

### `squadai validate-policy`

Validate `.squadai/policy.json` schema and lock/required consistency.

```sh
squadai validate-policy
```

### `squadai backup create`

Manually snapshot all managed files.

```sh
squadai backup create [--json]
```

### `squadai backup list`

List available backups.

```sh
squadai backup list [--json]
```

### `squadai restore <id>`

Restore files from a backup snapshot.

```sh
squadai restore <backup-id> [--dry-run] [--json]
```

### Interactive TUI

Run `squadai` with no arguments for a guided wizard:

1. Intro screen with detected agents and mode
2. Methodology selection (TDD / SDD / Conventional)
3. MCP server configuration
4. Plugin selection (filtered by methodology and detected agents)
5. Summary and confirmation
6. Menu: Plan, Apply, Sync, Verify, Restore, Quit

---

## Template Rendering

Team orchestrator and sub-agent templates are Go `text/template` files rendered with project context. The template data includes:

| Variable | Source | Example |
|----------|--------|---------|
| `{{.Methodology}}` | `project.json` methodology | `tdd` |
| `{{.DelegationStrategy}}` | Adapter type | `native`, `prompt`, `solo` |
| `{{.Language}}` | `meta.language` | `Go` |
| `{{.TestCommand}}` | `meta.test_command` | `go test -race ./...` |
| `{{.BuildCommand}}` | `meta.build_command` | `go build ./...` |
| `{{.LintCommand}}` | `meta.lint_command` | `golangci-lint run` |
| `{{.SkillsDir}}` | Adapter-specific path | `.opencode/skills` |
| `{{.AgentsDir}}` | Adapter-specific path | `.opencode/agents` |
| `{{.HasContext7}}` | MCP config | `true` |

Templates are stored as embedded assets at `internal/assets/teams/{methodology}/`. Each methodology has three orchestrator variants (native, prompt, solo) and one template per sub-agent role.

---

## Quick Start Walkthrough

### 1. Initialize

```sh
cd my-project
squadai init --methodology=tdd
```

Output:

```
  created .squadai/project.json
  created .squadai/templates/team-standards.md
  created .squadai/skills/code-review.md
  created .squadai/skills/testing.md
  created .squadai/skills/pr-description.md
  created .squadai/skills/find-skills.md

Detected:
  Language: Go
  Project:  my-project
  Agents:   opencode, claude-code, cursor
  Methodology: tdd
  Team roles:  6
  MCP servers: context7

Run 'squadai apply' to configure your environment.
```

### 2. Preview

```sh
squadai plan
```

### 3. Apply

```sh
squadai apply
```

### 4. Verify

```sh
squadai verify
```

### 5. Iterate

Edit `.squadai/project.json` to add MCP servers, change team roles, or enable plugins. Run `squadai apply` again. The planner skips files that are already up to date.
