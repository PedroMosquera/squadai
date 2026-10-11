# SquadAI

The agent config you can verify. Declare it once, apply it to every harness, and fail CI when it drifts, leaks, or overspends.

[![Build](https://github.com/PedroMosquera/squadai/actions/workflows/ci.yml/badge.svg)](https://github.com/PedroMosquera/squadai/actions)
[![Go](https://img.shields.io/github/go-mod/go-version/PedroMosquera/squadai)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

A repo that uses coding agents carries config for each one: OpenCode, Claude Code, Cursor, Windsurf, VS Code Copilot, Pi and Codex. Each has its own instruction file, MCP file, hooks and skills. The copies drift apart, a token lands in a committed MCP file, a hook runs an unpinned `npx` package, and nobody sees the token bill until it arrives.

SquadAI keeps one declaration in `.squadai/project.json` and writes each harness's native files from it. Then it checks the committed result: drift from the declaration, a security scan of hooks, MCP servers, settings and skills, policy allow-lists, and token budgets enforced inside agent sessions.

---

## Verify in CI

Commit the files `squadai apply` writes, then fail pull requests that drift from `.squadai/project.json` or carry risky agent config:

```yaml
name: squadai
on: pull_request
jobs:
  squadai:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: PedroMosquera/squadai@v0.9.0
        with:
          version: v0.9.0          # squadai release to install
          working-directory: .     # directory that holds .squadai/project.json
          scan: true               # false skips the security scan
          scan-fail-on: high       # info, low, medium, high or none
```

The action runs `go install github.com/PedroMosquera/squadai/cmd/squadai@<version>` (GitHub-hosted runners ship Go), then `squadai verify --ci` and `squadai scan --fail-on <scan-fail-on>`. Drift shows up as annotations on the changed files. Keep `version` at v0.9.0 or later: older releases have no `verify --ci`.

The same checks locally:

```sh
squadai verify --ci   # exit 4 on drift or missing files, 3 on a policy allow-list violation
squadai scan          # exit 3 on a finding at or above --fail-on (default high)
```

---

## Install

```sh
# macOS / Linux via Homebrew (recommended)
brew install PedroMosquera/tap/squadai

# or via curl
curl -sSL https://raw.githubusercontent.com/PedroMosquera/squadai/main/scripts/install.sh | sh

# or from source
go install github.com/PedroMosquera/squadai/cmd/squadai@latest
```

Linux `.deb` and `.rpm` packages are published on every [release](https://github.com/PedroMosquera/squadai/releases/latest) for Debian/Ubuntu and Fedora/RHEL users. Windows binaries (zip) are also published there.

---

## Quick start

The fastest path is the interactive TUI wizard:

```sh
cd your-project
squadai
```

Running `squadai` with no arguments opens a terminal UI that walks you through choosing a methodology, enabling MCP servers, and selecting plugins. At the end it asks to apply the config.

To script it:

```sh
squadai init --methodology tdd   # writes .squadai/project.json
squadai apply                    # writes agent files for every enabled agent
squadai verify                   # check the result
```

Commit `.squadai/project.json` and the files apply wrote, then add the [CI check](#verify-in-ci).

Use a preset when you want a ready-made setup:

```sh
squadai init --preset=solo-minimal      # conventional workflow, starter models
squadai init --preset=solo-power        # TDD workflow with local memory/context defaults
squadai init --preset=team-standard     # TDD workflow plus a team policy.json
squadai init --preset=enterprise-locked # SDD workflow, performance models, team policy.json
squadai init --preset=full-squad        # SDD methodology, balanced models, all components
squadai init --preset=lean              # conventional methodology, starter models, core only
squadai init --preset=custom            # explicit flags or wizard defaults
```

To see what `apply` would change first:

```sh
squadai plan
squadai diff
```

---

## What SquadAI checks

### Drift

`squadai verify --ci` compares the committed project files with what `apply` would write, for every agent enabled in project config, whether or not that agent is installed on the machine. It skips machine checks (agent binaries, user config, home files) and gitignored files. `squadai diff --exit-code` is a lighter gate: it exits 4 when apply would change any file.

Plain `squadai verify` runs the local compliance and health checks, and `--strict` adds a drift check. It also warns when `AGENTS.md` is larger than the 32 KiB Codex reads by default (`project_doc_max_bytes`).

### Security scan

`squadai scan` reads MCP server definitions for every harness, Claude Code and Codex hooks, Claude Code settings, skills and Pi extensions. It writes and executes nothing.

| Rule | Severity | Finds |
|------|----------|-------|
| SCAN-001 | high | A secret literal in an MCP server, hook, settings env, skill or Pi extension |
| SCAN-002 | high | A remote script fetched and executed at runtime |
| SCAN-003 | medium | A package runner without a pinned version |
| SCAN-004 | high | A hook command that executes or splits tool input |
| SCAN-005 | low | A project setting Claude Code ignores |
| SCAN-006 | medium | An MCP server over plain http to a remote host |
| SCAN-007 | high | An MCP server, hook or plugin outside the policy allow-lists |

`--user` also scans user-level config under your home directory. `--json` prints the report as JSON. See [`docs/commands.md`](docs/commands.md#squadai-scan).

### Team policy and allow-lists

To enforce a standard setup and stop individuals from overriding it, create `.squadai/policy.json`:

```json
{
  "version": 1,
  "mode": "team",
  "locked": ["methodology", "mcp"],
  "required": {
    "methodology": "tdd",
    "mcp": { "context7": { "enabled": true } }
  },
  "allowed": {
    "mcp_servers": {
      "names": ["squadai", "context7"],
      "commands": ["squadai mcp-server", "npx -y @upstash/context7-mcp@*"]
    },
    "models": ["claude-sonnet-5-*", "claude-haiku-4-5"],
    "hook_commands": ["squadai *"]
  }
}
```

Fields listed under `locked` cannot be overridden by a developer's `~/.squadai/config.json`. The `allowed` block limits which MCP servers, models, hook commands and plugin sources project config and committed harness files may declare. The lists also apply to what SquadAI configures itself, which is why the example allows `squadai`.

`squadai validate-policy` checks the policy file and the project config against it. `squadai verify --ci` fails on a violation, and `squadai scan` reports hand-added servers and hooks as SCAN-007. See [`docs/policy.md`](docs/policy.md).

### Token budgets

Set budgets in the `usage` block of `.squadai/project.json`:

```json
"usage": {
  "daily_token_budget": 200000,
  "session_token_budget": 50000,
  "enforcement": "block"
}
```

With `enforcement` set to `warn`, `ask` or `block`, `squadai apply` installs budget hooks for Claude Code (`.claude/settings.json`), Codex (`.codex/hooks.json`) and Pi (`.pi/extensions/squadai-budget.ts`). Under `block`, tool calls and prompts over budget are refused. The hooks call `squadai budget-hook`, so the `squadai` on the PATH must be v0.9.0 or later. See [Budget enforcement in agent sessions](docs/commands.md#budget-enforcement-in-agent-sessions).

Outside a session:

```sh
squadai token-usage                    # real usage from Claude Code, OpenCode and Pi transcripts
squadai token-usage --against-budget   # exits 9 when over budget under block
squadai token-budget                   # per-session token cost of the installed agent files
```

The orchestrator gets the full memory protocol and sub-agents get a short stub, which keeps the per-session cost of a full install down.

---

## How it works

`squadai init` detects which agents are installed and writes `.squadai/project.json`: the methodology, the team roles, the active context profile, how memory works, and which components each agent gets.

`squadai apply` reads that config and writes each agent's native files:

- **Instructions** go once into `AGENTS.md`, which every supported agent reads. Shared blocks (memory protocol, team standards, session efficiency, the solo orchestrator) appear there once. `CLAUDE.md` gets a block that imports it with `@AGENTS.md`.
- **Skills** are written once to `.agents/skills/<name>/SKILL.md`. Claude Code keeps a copy in `.claude/skills`.
- **Role files** for agents with native sub-agents are short stubs: frontmatter, the role's boundaries and the path to its skill.
- **Claude Code** also gets SquadAI's slash commands and the `squadai-manager` agent.
- **MCP servers** come from a curated catalog, pinned to exact package versions.

SquadAI only writes inside its own marker blocks, so your edits in the same files stay put. Every apply takes a backup first and rolls back on failure.

The config also carries:

- **A methodology**: TDD (6 roles), SDD (8 roles), or Conventional (4 roles), each with an orchestrator that delegates to specialist sub-agents.
- **Coding standards** auto-detected by language: Go, TypeScript/JavaScript, Python, Rust, Java and eight more, with a generic fallback.
- **Context profiles**: built-in profiles for default, debug, feature, review, docs, incident, and cheap sessions. Switch with `squadai profile <name>` or `squadai apply --profile=<name>`.
- **Model routing**: a model tier (balanced, performance, starter or manual) picked at `squadai init --model-tier`, with per-role overrides for one run via `squadai apply --model`.
- **An optional banner**, off by default. Opt in with `"brand": {"enabled": true}` under `components`, or skip it for one run with `squadai apply --no-brand`.

### Adapters and delegation

Different agents handle sub-agents differently. SquadAI adapts:

| Agent | Instructions file | Sub-agent strategy |
|-------|-------------|-------------------|
| OpenCode | `AGENTS.md` | Native agent files in `.opencode/agents/` |
| Claude Code | `AGENTS.md`, imported from `CLAUDE.md` | Native agent files in `.claude/agents/` |
| Cursor | `AGENTS.md` | Native agent files in `.cursor/agents/` |
| Windsurf | `AGENTS.md` | Solo all-in-one prompt + workflow files |
| VS Code Copilot | `AGENTS.md` | Solo all-in-one prompt |
| Pi | `AGENTS.md` | Native agent files in `.pi/agents/` + prompts in `.pi/prompts/` |
| Codex | `AGENTS.md` | Solo all-in-one prompt; MCP servers as TOML tables in `~/.codex/config.toml` |

The solo orchestrator block is written once, rendered for the first enabled solo agent (VS Code Copilot, then Windsurf, then Codex).

### Migrating to AGENTS.md

Earlier versions wrote a full copy of every shared block into each agent's own file: `CLAUDE.md`, `.github/copilot-instructions.md`, `.cursor/rules/squadai.mdc` and `.windsurf/rules/squadai.md`. The next `squadai apply` after upgrading moves to a single copy in `AGENTS.md` and prints a one-time note when it does:

- `AGENTS.md` keeps one copy of each block. The per-agent copies (`memory:opencode`, `memory:pi`, and so on) are merged away.
- `CLAUDE.md` loses squadai's old blocks and gains `<!-- squadai:agents-md -->` with `@AGENTS.md`. Claude Code reads `AGENTS.md` on its own only when no `CLAUDE.md` exists, so the import keeps it loaded either way.
- `.github/copilot-instructions.md` loses squadai's old blocks. The copilot instructions template block stays if you use one.
- `.cursor/rules/squadai.mdc` and `.windsurf/rules/squadai.md` are deleted when they hold nothing but squadai's frontmatter, team standards and blocks.
- Your own text in any of these files is never touched. A file squadai created that ends up empty is deleted. A populated `/squadai-init` refinement block is kept where it was.

Commit the result. `squadai verify --ci` passes once the migrated files are committed. To undo, `squadai restore <backup-id>` with the backup ID that apply printed.

---

## SquadAI inside your agent

`squadai apply` registers SquadAI's own MCP server (`squadai mcp-server`) in every enabled agent: Claude Code, OpenCode, Cursor, Windsurf, VS Code Copilot, Pi, and Codex. Your agents can then check `status`, `plan`/`apply`/`verify` config changes, run `doctor`, and search or add project memory without leaving the agent console.

Project MCP servers land in each agent's native file: `.mcp.json` (Claude Code and VS Code share it), `.cursor/mcp.json`, `.windsurf/mcp_config.json`, `.pi/mcp.json` (Pi reads it once you trust the project), `opencode.json`, and `~/.codex/config.toml`. Ownership is tracked per server, so servers you add by hand are never overwritten or removed.

The `squadai` entry is pre-checked in the wizard. Deselect it there, or pass `--mcp=none` (or an explicit `--mcp=` list without `squadai`) to `squadai init` to opt out. The registration invokes the `squadai` binary from PATH; `squadai doctor` warns if it cannot be found.

---

## Per-project refinement (`/squadai-init`)

Agent files installed by `squadai apply` contain placeholder blocks. `/squadai-init` is a command you run inside your AI agent to fill those placeholders with project-specific context: the repo layout, which roles matter most, custom instructions per role.

Agents remind you when the refinement is stale (for example after a major refactor). To refresh it, run `/squadai-init` in your agent again.

---

## Project memory

Agents lose context between sessions. New configs default to local project memory, with `docs/memory/` as the shared format agents read and write: indexed notes organized by topic.

```sh
# From inside your agent
/memory-search authentication    # find prior decisions
/memory-add "switched to JWT, see docs/adr/012.md"   # capture a decision
/memory-promote                  # graduate inbox drafts to permanent storage
```

```sh
# From the CLI (for scripting or CI)
squadai memory search authentication
squadai memory add "switched to JWT"
squadai memory status
```

In Claude Code and Codex, set `"capture_on_stop": true` in the `memory` block of `.squadai/project.json` and the agent is asked, once at the end of each non-trivial session, to record what it decided. See [Memory capture at the end of Claude Code and Codex sessions](docs/commands.md#memory-capture-at-the-end-of-claude-code-and-codex-sessions).

See [`docs/project-memory.md`](docs/project-memory.md) for the full protocol.

---

## Commands

Checks:

```sh
squadai verify              # compliance and health report (--ci for the CI drift gate, --strict adds drift)
squadai scan                # security scan of agent config (--fail-on, --user, --json)
squadai validate-policy     # check policy.json and the project config against its allow-lists
squadai diff                # unified diff of what apply would change (--exit-code for CI)
squadai token-usage         # real token usage from session transcripts (--against-budget)
squadai token-budget        # per-session token cost of the current install
```

Setup and daily use:

```sh
squadai                     # interactive TUI wizard (first-time setup or re-configure)
squadai init                # initialize or re-initialize project.json
squadai apply               # write agent files (idempotent, safe to re-run)
squadai plan                # show the action plan without applying
squadai doctor              # pre-flight diagnostics; --fix walks through fixable issues
squadai status              # agents, components, MCP servers, last backup (--daily for a summary)
squadai profile             # show or switch the active context profile
squadai memory <subcommand> # project memory (search, add, promote, status)
```

Maintenance:

```sh
squadai update              # self-update (--check, --enable-checks, or download the latest)
squadai backup create       # snapshot managed files manually
squadai restore <id>        # restore from a backup
squadai remove --force      # remove all managed files
```

For any command: `squadai <command> --help`.

---

## Configuration

`squadai init` creates `.squadai/project.json`. Most fields have defaults; the ones you are likely to touch:

```json
{
  "version": 1,
  "methodology": "tdd",
  "adapters": {
    "claude-code": { "enabled": true },
    "opencode": { "enabled": true }
  },
  "usage": {
    "session_token_budget": 50000,
    "enforcement": "warn"
  },
  "meta": {
    "name": "my-project",
    "language": "go",
    "test_command": "go test -race ./..."
  }
}
```

Config is merged from three sources in this order (last wins, unless locked by policy):

```
~/.squadai/config.json        personal defaults
.squadai/project.json         project config  (this file)
.squadai/policy.json          team policy: locked fields override everything
```

---

## Install via your AI agent

To have your agent set things up, paste this into it:

```text
Install squadai using the best available method on this machine (brew if available, otherwise the curl script), verify with `squadai version`, run `squadai doctor`, and let me know if any checks failed. Don't run `squadai init` yet. I'll do that myself.
```

---

## More docs

- [`docs/MANUAL.md`](docs/MANUAL.md): 5-minute user manual
- [`docs/commands.md`](docs/commands.md): every command and flag
- [`docs/policy.md`](docs/policy.md): policy, allow-lists and team enforcement
- [`docs/project-memory.md`](docs/project-memory.md): memory protocol deep dive
- [`docs/architecture.md`](docs/architecture.md): internals and design decisions
- [`docs/troubleshooting.md`](docs/troubleshooting.md): common issues

---

## License

MIT
