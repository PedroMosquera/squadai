# V2 User Guide

SquadAI V2 turns per-agent setup into a local control plane for daily AI engineering work. It adds sub-agent teams, methodology-driven workflows, MCP server management, plugin integration, native memory metadata, context profiles, usage budgets, and model routing labels.

---

## Introduction

V1 standardized system prompts and settings across agents. V2 extends this with:

- **Methodology teams.** Choose TDD, SDD, or Conventional. Each methodology generates a team of specialized roles with an orchestrator that delegates work to sub-agents.
- **Delegation strategies.** Native sub-agent files (OpenCode, Cursor), Task tool prompt injection (Claude Code), or solo all-in-one prompts (VS Code, Windsurf). The same team definition produces different file structures per agent.
- **MCP servers.** Context7 is included by default. Each agent receives MCP config in its native format.
- **Plugins.** Third-party capabilities from a curated catalog, filtered by methodology compatibility and detected agents.
- **Methodology skills.** Embedded skill files installed per methodology phase (TDD red-green-refactor, SDD spec workflow, etc.) alongside shared skills (code-review, testing, pr-description).
- **Community skills.** The `find-skills` meta-skill connects to the Vercel skills ecosystem (skills.sh) for discovering and installing 91K+ community skill definitions.
- **Daily-driver controls.** New project configs include `preset`, `memory`, `context`, `usage`, and `models` blocks so a repo can describe the whole operating model, not just generated files.

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

### Native (OpenCode, Cursor, Claude Code, Pi)

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
.cursor/agents/       # also .claude/agents/ and .pi/agents/
  orchestrator.md     # Same templates, different target directory
  brainstormer.md
  ...
```

Each file contains YAML frontmatter (description, mode) and the rendered template content. The orchestrator template includes delegation rules that reference other agent files by name.

### Solo (VS Code Copilot, Windsurf, Codex)

Solo agents cannot delegate to sub-agents at all. The orchestrator template is injected once into the shared `AGENTS.md`, which VS Code, Windsurf and Codex all read, using a marker block rendered for the first enabled solo agent:

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
| Claude Code | native | `.claude/agents/orchestrator.md` | `.claude/agents/<role>.md` |
| VS Code Copilot | solo | `AGENTS.md` (marker block) | None |
| Windsurf | solo | `AGENTS.md` (marker block) | None |

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
| MCPConfigFile | Claude Code | `.mcp.json` | `"mcpServers"` |
| MCPConfigFile | VS Code Copilot | `.mcp.json` | `"mcpServers"` |
| MCPConfigFile | Cursor | `.cursor/mcp.json` | `"mcpServers"` |
| MCPConfigFile | Windsurf | `.windsurf/mcp_config.json` | `"mcpServers"` |
| MCPConfigFile | Pi | `.pi/mcp.json` | `"mcpServers"` |
| TOMLConfigFile | Codex | `~/.codex/config.toml` | `[mcp_servers.<name>]` |

**MergeIntoSettings (OpenCode):** Servers are merged into the project config file under the `"mcp"` key. Other keys in the file are preserved.

**MCPConfigFile (Claude Code, VS Code, Cursor, Windsurf, Pi):** All servers are written into a dedicated MCP config file under the `"mcpServers"` key. Claude Code and VS Code share the root `.mcp.json` (the portable format VS Code 1.140 recommends, and the only workspace file the VS Code Agent Host reads), so enabling both writes one file with identical entries. Pi reads `.pi/mcp.json` only after you grant project trust.

**Moved locations:** Earlier releases wrote VS Code servers to `.vscode/mcp.json` (`"servers"` key) and Pi servers to `pi.json` (`"mcp"` key, which Pi never read). On the next `squadai apply`, squadai writes the new file first, then removes the servers it owns from the old location. Servers you added yourself stay where they are, and the old file is deleted only when nothing is left in it. Until that apply runs, `squadai verify` fails the `mcp-legacy-migrated` check.

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
      "plugin_id": "code-review@claude-plugins-official"
    }
  }
}
```

### How apply writes plugins

A plugin declaration delegates installation to the agent. SquadAI writes the declaration into the agent's own config and never downloads plugin content or touches the network during apply.

For `claude_plugin`, `apply` writes into the project's `.claude/settings.json` (enable the `plugins` component):

- `plugin_id` (`<plugin>@<marketplace>`) becomes an `enabledPlugins` entry set to `true`.
- `marketplace`, when present, becomes an `extraKnownMarketplaces` entry named after the part of `plugin_id` after `@`. Omit it for marketplaces Claude Code already knows, such as `claude-plugins-official`.

```json
{
  "plugins": {
    "formatter": {
      "enabled": true,
      "supported_agents": ["claude-code"],
      "install_method": "claude_plugin",
      "plugin_id": "formatter@team-tools",
      "marketplace": { "source": "github", "repo": "example-org/claude-plugins" }
    }
  }
}
```

`marketplace.source` is one of `github` (with `repo`), `git` or `url` (with `url`), `file` or `directory` (with `path`), plus an optional `ref`. These are the source types Claude Code documents for `extraKnownMarketplaces` ([settings reference](https://code.claude.com/docs/en/settings-reference), [plugin marketplaces](https://code.claude.com/docs/en/plugin-marketplaces)).

SquadAI owns individual entries, not the two maps. Plugins and marketplaces you add by hand stay as they are, and removing a declaration removes only the entry SquadAI wrote. If you already set a declared plugin to a different value (for example `false`), apply reports a conflict instead of overwriting it.

Claude Code does the rest. It registers a declared marketplace once you accept the folder trust prompt. A plugin stored inside that marketplace loads from there. A plugin whose marketplace entry points at an external source still has to be installed by each user (`claude plugin install <plugin>@<marketplace> --scope project`). To opt out of a project plugin locally, set it to `false` in `.claude/settings.local.json`.

Other agents have their own plugin systems, and a Claude plugin cannot be loaded by them, so `claude_plugin` declarations write nothing for them. Pi and OpenCode load plugins as packages instead, declared with `install_method: "package"`:

| Agent | Native mechanism | What SquadAI writes |
|-------|------------------|---------------------|
| Claude Code | `enabledPlugins` and `extraKnownMarketplaces` in `.claude/settings.json` | `claude_plugin` declarations, as above |
| Codex | `[plugins."<plugin>@<marketplace>"]` and `[marketplaces.<name>]` in `.codex/config.toml` ([config reference](https://developers.openai.com/codex/config-reference), [plugins](https://developers.openai.com/codex/plugins)) | Nothing. SquadAI configures Codex globally and does not write project `.codex/config.toml`. |
| Pi | `packages` in `.pi/settings.json` ([packages](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md)) | `package` declarations, as written: `npm:<name>@<version>` or `git:<host>/<repo>@<ref>` |
| OpenCode | `plugin` in `opencode.json` ([plugins](https://opencode.ai/docs/plugins/)) | npm `package` declarations, without the `npm:` prefix: `<name>@<version>` |

#### Package plugins

```json
{
  "plugins": {
    "agent-tools": {
      "enabled": true,
      "supported_agents": ["pi", "opencode"],
      "install_method": "package",
      "package": "npm:@example-org/agent-tools@1.2.3"
    },
    "pi-review": {
      "enabled": true,
      "supported_agents": ["pi"],
      "install_method": "package",
      "package": "git:github.com/example-org/pi-review@v1.0.0"
    }
  }
}
```

`package` uses Pi's source syntax. A package runs code inside the agent, so the validator requires it pinned: an npm package needs an exact version (`1.2.3`, not `^1.2.0`, `latest` or no version) and a git package needs a ref. Use a tag or a commit, because a branch ref still moves. `supported_agents` may list only `pi` and `opencode`. OpenCode installs plugins from npm only, so a git package must list `pi` alone. Two declarations of the same package (same npm name, or same repository) are rejected, because each agent loads a package once. Local paths are not declarable; reference them in the agent's file yourself.

SquadAI owns individual array entries, recorded in `.squadai/managed.json` as the exact strings it wrote. Entries you add stay where they are and in their order. A declared package is appended after them the first time, replaced in place when its version changes, and removed when the declaration goes. If you already list the same package at another version, apply reports a conflict instead of overwriting it. `opencode.json` is shared with SquadAI's MCP servers and settings, and each keeps its own keys. Pi and OpenCode fetch and install the packages themselves when they start. Pi loads project packages only after you grant project trust.

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
| Skills | `.agents/skills/<name>/SKILL.md` (shared, see [Installation Layout](#installation-layout)) |
| Commands | `.opencode/commands/<name>.md` |

Supported components: memory, rules, settings, mcp, agents, skills, commands, plugins.

### Claude Code

| Component | File Path |
|-----------|-----------|
| System prompt | `AGENTS.md` (project root), imported by a marker block in `CLAUDE.md` |
| Agents (native) | `.claude/agents/<name>.md` (team roles, your `agents`, plus SquadAI's `squadai-manager`) |
| Settings | `.claude/settings.json` |
| MCP | `.mcp.json` `"mcpServers"` key (shared with VS Code) |
| Skills | `.claude/skills/<name>/SKILL.md` (Claude Code does not read `.agents/skills`) |
| Commands | `.claude/commands/<name>.md` (your `commands` plus SquadAI's `/squadai-*` and `/memory-*`) |

Supported components: memory, rules, settings, skills, mcp, plugins, agents, commands. Does not support workflows.

The agents component writes `squadai-manager.md` whenever it is enabled for Claude Code. Define an agent named `squadai-manager` in `agents` to replace it. Disabling the component leaves `.claude/agents/` as it is.

Disabling the commands component (`"commands": {"enabled": false}`) makes the next apply delete the command files it wrote, as long as they are unchanged. Edited files and your own commands in `.claude/commands/` stay.

### Cursor

| Component | File Path |
|-----------|-----------|
| System prompt | `AGENTS.md` (project root) |
| Settings | `.cursor/mcp.json` |
| MCP | `.cursor/mcp.json` `"mcpServers"` key |
| Agents (native) | `.cursor/agents/<role>.md` |
| Skills | `.agents/skills/<name>/SKILL.md` (shared, see [Installation Layout](#installation-layout)) |

Supported components: memory, rules, settings, mcp, agents, skills, plugins.

### VS Code Copilot

| Component | File Path |
|-----------|-----------|
| System prompt | `AGENTS.md` (project root); `.github/copilot-instructions.md` only for the copilot instructions template |
| Team (solo) | `AGENTS.md` (marker block) |
| Settings | `.vscode/settings.json` |
| MCP | `.mcp.json` `"mcpServers"` key (shared with Claude Code) |
| Skills | `.agents/skills/<name>/SKILL.md` (shared, see [Installation Layout](#installation-layout)) |

Supported components: memory, rules, settings, mcp, skills.

### Windsurf

| Component | File Path |
|-----------|-----------|
| System prompt | `AGENTS.md` (project root) |
| Team (solo) | `AGENTS.md` (marker block) |
| Settings | `.windsurf/mcp_config.json` |
| MCP | `.windsurf/mcp_config.json` `"mcpServers"` key |
| Skills | `.agents/skills/<name>/SKILL.md` (shared, see [Installation Layout](#installation-layout)) |
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

Skills are written once to `.agents/skills/`, the [Agent Skills](https://agentskills.io) standard location:

```
.agents/skills/
  code-review/SKILL.md
  testing/SKILL.md
  pr-description/SKILL.md
  brainstorming/SKILL.md       # TDD only
  writing-plans/SKILL.md       # TDD only
  ...
```

Each skill sits one level below the skills directory, in a folder named after its frontmatter `name`. The asset paths above (`sdd/sdd-spec`) group skills in the SquadAI source and in `skill_scopes`, but the group is not part of the installed path. Claude Code, VS Code Copilot and Windsurf document only `<skills-dir>/<skill-name>/SKILL.md`, and VS Code and Cursor skip a skill whose `name` differs from its folder. A custom skill in `skills` with the same name as an embedded one replaces it.

Every supported agent except Claude Code reads that directory natively, per its docs as of 2026-10-11:

| Agent | Reads `.agents/skills` | Source |
|-------|------------------------|--------|
| Codex | yes (its only project location) | https://learn.chatgpt.com/docs/build-skills |
| Pi | yes | https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/skills.md |
| OpenCode | yes | https://opencode.ai/docs/skills/ |
| Cursor | yes | https://cursor.com/docs/context/skills |
| VS Code Copilot | yes | https://code.visualstudio.com/docs/copilot/customization/agent-skills |
| Windsurf | yes | https://docs.devin.ai/desktop/cascade/skills |
| Claude Code | no, reads `.claude/skills` only | https://code.claude.com/docs/en/skills |

When Claude Code is enabled, apply also writes a copy of each skill to `.claude/skills/`. It is a plain copy, not a symlink, so it behaves the same on every OS and in every git checkout. The orchestrator each agent receives points at the directory that agent reads (`{{.SkillsDir}}`).

Projects applied with an earlier release have per-agent copies in `.opencode/skills/`, `.pi/skills/`, `.cursor/skills/`, `.copilot/skills/` or `.windsurf/skills/`. The next apply deletes each copy that is still byte-identical to what SquadAI writes, so agents do not load the same skill twice. A copy you edited, and any file SquadAI did not write, stays where it is.

Releases before the flat layout wrote `<skills-dir>/<group>/<skill>/SKILL.md` (for example `.agents/skills/shared/testing/`). Apply removes those copies the same way, byte-identical ones only, and then removes folders left empty.

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
  "preset": "solo-power",
  "methodology": "tdd",
  "model_tier": "balanced",
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
  "memory": {
    "backend": "native",
    "auto_capture": true,
    "project_key_strategy": "git-remote",
    "export_path": "docs/memory"
  },
  "context": {
    "default_profile": "default",
    "profiles": {
      "default": {
        "memory_scope": "project",
        "mcp_servers": ["context7"],
        "skill_scopes": ["shared"],
        "max_approx_tokens": 12000,
        "include": ["**/*"],
        "exclude": [".git/**", "node_modules/**", "dist/**"]
      },
      "cheap": {
        "memory_scope": "summary",
        "mcp_servers": [],
        "skill_scopes": ["shared"],
        "max_approx_tokens": 6000
      }
    }
  },
  "usage": {
    "daily_token_budget": 200000,
    "session_token_budget": 50000,
    "enforcement": "warn",
    "currency": "USD",
    "price_catalog_source": "embedded"
  },
  "models": {
    "profiles": {
      "cheap": { "tier": "cheap" },
      "balanced": { "tier": "balanced" },
      "premium": { "tier": "premium" },
      "default": { "tier": "balanced" }
    },
    "overrides": {
      "tdd.planner": "premium",
      "tdd.reviewer": "premium",
      "tdd.debugger": "premium"
    }
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
      "plugin_id": "code-review@claude-plugins-official"
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

> **Note:** `hybrid` mode is deprecated. If your config uses `hybrid`, SquadAI will resolve it to `team` (if `policy.json` exists) or `personal` (if not), and print a deprecation warning.

---

## CLI Reference (V2)

### `squadai init`

Initialize `.squadai/project.json`. Detects installed agents, project language, and writes starter files.

```sh
squadai init [--methodology=<tdd|sdd|conventional>] [--mcp=<csv>] [--plugins=<csv>] [--model-tier=<balanced|performance|starter|manual>] [--agents=<csv>] [--preset=<solo-minimal|solo-power|team-standard|enterprise-locked|full-squad|lean|custom>] [--with-policy] [--force]
```

| Flag | Description |
|------|-------------|
| `--methodology=<tdd\|sdd\|conventional>` | Set the development methodology. Generates team composition and enables agents/commands components. |
| `--mcp=<csv>` | Comma-separated MCP server IDs to enable. Omit to include all recommended servers. |
| `--plugins=<csv>` | Comma-separated plugin IDs to enable. Omit to skip plugin installation. |
| `--model-tier=<balanced\|performance\|starter\|manual>` | Set compatibility model defaults for generated agent config. |
| `--agents=<csv>` | Configure only the listed detected agents; OpenCode is always included. |
| `--preset=<name>` | Use `solo-minimal`, `solo-power`, `team-standard`, `enterprise-locked`, or compatibility presets `full-squad`, `lean`, `custom`. |
| `--with-policy` | Also create `.squadai/policy.json` with a starter template. |
| `--force` | Overwrite existing template and skill files. |

Examples:

```sh
squadai init --methodology=tdd
squadai init --preset=solo-power
squadai init --methodology=sdd --mcp=context7 --plugins=code-review
squadai init --with-policy --force
```

### `squadai plan`

Compute the action plan without writing files.

```sh
squadai plan [--dry-run] [--json]
```

Covers all 9 components (memory, rules, settings, mcp, agents, skills, commands, plugins, workflows) across all 7 supported agents.

### `squadai apply`

Execute the plan with backup and rollback safety.

```sh
squadai apply [--dry-run] [--json]
```

All managed files are backed up before changes. If any step fails, all completed changes are rolled back. The backup ID is printed for manual recovery.

> **`apply` is idempotent** — re-running it produces the same result. There is no separate `sync` command.

### `squadai verify`

Run compliance checks against the current project configuration.

```sh
squadai verify [--json]
```

Checks that all enabled components are correctly installed for each detected agent: expected files exist, marker blocks are present, settings are valid.

### `squadai status`

Show the current project configuration, health, backup, and refinement summary.

```sh
squadai status [--json] [--fix] [--daily]
```

Use `--daily` for the compact local control-plane dashboard: active repo, agents, preset, context profile, memory backend, usage budgets, MCP servers, drift/refinement state, and health.

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
| `{{.SkillsDir}}` | Project skills directory the agent reads | `.agents/skills` (`.claude/skills` for Claude Code) |
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
