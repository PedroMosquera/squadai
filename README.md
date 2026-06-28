# SquadAI

Give every developer on your team the same AI coding agent setup — same methodology, same roles, same standards — regardless of which editor they use.

[![Build](https://github.com/PedroMosquera/squadai/actions/workflows/ci.yml/badge.svg)](https://github.com/PedroMosquera/squadai/actions)
[![Go](https://img.shields.io/github/go-mod/go-version/PedroMosquera/squadai)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

If you've ever opened a repo on a new machine and had your AI agent forget everything about how the team works — the methodology, the review process, the coding standards — that's the problem squadai solves. You declare the setup once (a config file in the repo), and `squadai apply` writes the right files for every agent each developer has installed: OpenCode, Claude Code, Cursor, Windsurf, VS Code Copilot. Run it after cloning, run it after pulling, and you're always in sync.

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

## Getting started

The fastest path is the interactive TUI wizard:

```sh
cd your-project
squadai
```

Running `squadai` with no arguments opens a terminal UI that walks you through choosing a methodology, enabling MCP servers, and selecting plugins. At the end it asks to apply the config. That's it — your agents are configured.

If you'd rather script it:

```sh
squadai init --methodology tdd   # writes .squadai/project.json
squadai apply                    # installs agent files for all detected editors
squadai doctor                   # sanity-check everything looks right
```

To see what `apply` would change before committing:

```sh
squadai plan --dry-run
squadai diff
```

---

## How it works

When you run `squadai init`, it detects which AI agents are installed on the current machine and writes `.squadai/project.json` — a config that says which methodology to use, which roles exist, and which components each agent should get.

`squadai apply` reads that config, figures out which files each agent needs in its native format, and writes them. For Claude Code it updates `CLAUDE.md`. For OpenCode it writes `AGENTS.md` and individual agent files under `.opencode/agents/`. For Cursor it writes `.cursor/rules/` and `.cursor/agents/`. It never touches content outside its own marker blocks, so any customizations you've made in those files stay put.

The result is a team where everyone's agents have:

- **A methodology** — TDD (6 roles), SDD (8 roles), or Conventional (4 roles), each with an orchestrator that delegates to specialist sub-agents
- **Consistent coding standards** — auto-detected by language (Go, TypeScript, Python, Rust, and [12 more](docs/architecture.md))
- **Shared MCP servers** — Context7 is enabled by default; others are opt-in
- **Memory protocols** — a structured `docs/memory/` system agents can read and write across sessions
- **Visual branding** — an ASCII-art banner injected into agent files so devs see SquadAI is active at session start (disable with `squadai apply --no-brand`)

### Adapters and delegation

Different agents handle sub-agents differently. squadai adapts:

| Agent | Config file | Sub-agent strategy |
|-------|-------------|-------------------|
| OpenCode | `AGENTS.md` | Native agent files in `.opencode/agents/` |
| Claude Code | `CLAUDE.md` | Task tool injection (prompt-based delegation) |
| Cursor | `.cursor/rules/squadai.mdc` | Native agent files in `.cursor/agents/` |
| Windsurf | `.windsurf/rules/squadai.md` | Solo all-in-one prompt + workflow files |
| VS Code Copilot | `.github/copilot-instructions.md` | Solo all-in-one prompt |
| Pi | `~/.pi/agent/AGENTS.md` | Native agent files in `~/.pi/agent/agents/` + prompts in `~/.pi/agent/prompts/` |

---

## Per-project refinement (`/squadai-init`)

Agent files installed by `squadai apply` contain placeholder blocks. `/squadai-init` is a command you run inside your AI agent to fill those placeholders with project-specific context: the repo layout, which roles matter most, custom instructions per role.

Agents will remind you when the refinement is stale (e.g. after a major refactor). To refresh it manually, just run `/squadai-init` in your agent again.

---

## Project memory

Agents lose context between sessions. squadai's memory system is a `docs/memory/` folder the `@librarian` agent can read and write, structured as indexed notes organized by topic.

```sh
# From inside your agent
/memory-search authentication    # find prior decisions
/memory-add "switched to JWT, see docs/adr/012.md"   # capture a decision
/memory-promote                  # graduate inbox drafts to permanent storage
```

```sh
# From the CLI (for scripting or CI)
squadai memory search --query "authentication"
squadai memory add --note "switched to JWT"
squadai memory status
```

See [`docs/project-memory.md`](docs/project-memory.md) for the full protocol.

---

## Token cost

Every squadai install adds agent instruction files to each session's system prompt. On a full TDD install those files add up. You can check the cost:

```sh
squadai token-budget          # human-readable per-component breakdown
squadai token-budget --json   # machine-readable
```

Example output:
```
Component       Files   Tokens
────────────────────────────────────────────────────────────────
agents            6      4,210
skills            6      3,890
rules             1        480
memory            1        260
mcp               1         90
────────────────────────────────────────────────────────────────
TOTAL            15      8,930
```

The orchestrator gets the full memory protocol; sub-agents get a two-line stub — that alone cuts roughly 1,000 tokens per session from the default TDD install.

---

## Team policy

If you want to enforce a standard setup across a team and prevent individuals from overriding it, create `.squadai/policy.json`:

```json
{
  "version": 1,
  "mode": "team",
  "locked": ["methodology", "mcp"],
  "required": {
    "methodology": "tdd",
    "mcp": { "context7": { "enabled": true } }
  }
}
```

Fields listed under `locked` can't be overridden by a developer's local `~/.squadai/config.json`. `squadai doctor` reports policy violations and `squadai validate-policy` checks the file itself.

See [`docs/policy.md`](docs/policy.md) for the full reference.

---

## Commands

The ones you'll use regularly:

```sh
squadai                     # launch the interactive TUI wizard (first-time setup or re-configure)
squadai init                # initialize or re-initialize project.json
squadai apply               # install agent files (idempotent — safe to re-run)
squadai diff                # preview what apply would change
squadai doctor              # run ~22 health checks; --fix auto-resolves common issues
squadai status              # quick view of adapters, components, managed files
squadai token-budget        # per-session token cost of the current install
squadai explain <topic>     # explain a config field, error code, or concept
squadai memory <subcommand> # manage project memory (search, add, promote, status)
```

Less common but good to know:

```sh
squadai verify              # post-apply compliance assertions
squadai plan                # show the action plan without applying
squadai update              # self-update (--check, --enable-checks, or apply latest)
squadai remove --force      # remove all managed files
squadai backup create       # snapshot managed files manually
squadai restore <id>        # restore from a backup
```

For any command: `squadai <command> --help`.

---

## Configuration

`squadai init` creates `.squadai/project.json`. Most fields have sensible defaults; the ones you're likely to touch:

```json
{
  "version": 1,
  "methodology": "tdd",
  "adapters": {
    "claude-code": { "enabled": true },
    "opencode": { "enabled": true }
  },
  "mcp": {
    "context7": {
      "type": "local",
      "command": ["npx", "-y", "@upstash/context7-mcp@latest"],
      "enabled": true
    }
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
.squadai/policy.json          team policy — locked fields override everything
```

---

## Install via your AI agent

If you'd rather have your agent set things up, paste this into it:

> Install squadai using the best available method on this machine (`brew` if available, otherwise the curl script), verify with `squadai version`, run `squadai doctor`, and let me know if any checks failed. Don't run `squadai init` yet — I'll do that myself.

---

## More docs

- [`docs/MANUAL.md`](docs/MANUAL.md) — 5-minute user manual
- [`docs/project-memory.md`](docs/project-memory.md) — memory protocol deep dive  
- [`docs/policy.md`](docs/policy.md) — policy and team enforcement
- [`docs/architecture.md`](docs/architecture.md) — internals and design decisions
- [`docs/troubleshooting.md`](docs/troubleshooting.md) — common issues

---

## License

MIT
