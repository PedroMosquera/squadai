# SquadAI — 5-Minute User Guide

SquadAI is a local-first control plane for AI coding agents. It manages system prompts, sub-agent teams, MCP servers, skills, permissions, memory metadata, context profiles, model routing, usage budgets, and settings for OpenCode, Claude Code, Cursor, VS Code Copilot, Windsurf, and Pi from a single source of truth in `.squadai/`.

---

## Install

```sh
curl -sSL https://raw.githubusercontent.com/PedroMosquera/squadai/main/scripts/install.sh | sh
```

After installation, verify it works:

```sh
squadai version
```

---

## Interactive TUI Wizard

Run `squadai` with no arguments to launch the guided wizard. It walks you through setup in a few keystrokes:

### Welcome / Intro screen

Shows the tool version, the detected operational mode (`team` or `personal`), and a summary of AI agents detected on your machine along with their delegation strategy (native, prompt, or solo).

### Menu screen

The main menu offers: **Init/Setup**, **Plan** (dry-run), **Apply**, **Sync**, **Team Status**, **Verify**, **Restore backup**, and **Quit**.

### Methodology screen

Select a development methodology. Three options are available:

| Methodology | Team size | Best for |
|-------------|-----------|----------|
| TDD | 6 roles | Projects that enforce red-green-refactor discipline |
| SDD | 8 roles | Complex systems that need formal spec before code |
| Conventional | 4 roles | Lightweight structure without a formal pipeline |

Each option shows the role pipeline it will generate.

### Agents / Team Status screen

Displays the current methodology, team roles, active MCP servers, and enabled plugins. Available from the main menu as **Team Status**.

### Daily status

Use `squadai status --daily` for a compact daily-driver view: active repo, preset, context profile, memory backend, usage budgets, MCP servers, drift/refinement state, backup state, and health summary.

### MCP configuration screen

Toggle which MCP servers to enable. Context7 (live documentation lookup) is pre-selected by default. MCP servers are written to each agent's native config format.

### Review / Apply (Init Summary screen)

Shows all selections — methodology, MCP servers, plugins — before confirming. Confirm to run `apply`, which creates a backup and writes all managed files.

---

## Quick Cheat Sheet

| Task | Command |
|------|---------|
| First-time setup (with wizard) | `squadai` |
| First-time setup (CLI) | `squadai init --methodology=tdd && squadai apply` |
| Solo power preset | `squadai init --preset=solo-power && squadai apply` |
| Re-apply after editing config | `squadai apply` |
| Health check | `squadai status` |
| Daily control-plane summary | `squadai status --daily` |
| Full compliance check | `squadai verify` |
| Preview changes before applying | `squadai plan` or `squadai diff` |
| Dry-run (no writes) | `squadai apply --dry-run` |
| Update SquadAI itself | `squadai update` |
| Clean uninstall (remove managed files) | `squadai remove --force` |

---

## Recovery

### Diagnose issues

```sh
squadai doctor
```

Runs a set of checks across config, adapters, and managed files. Use `--fix` to auto-repair common problems, `--json` for machine-readable output.

### Rollback via backup restore

Every `apply` creates a backup before touching any file. If something goes wrong, restore the previous state:

```sh
# List available backups
squadai backup list

# Preview what a restore would do
squadai restore <backup-id> --dry-run

# Execute the restore
squadai restore <backup-id>
```

Backups are stored at `~/.squadai/backups/`. To keep the backup store tidy:

```sh
squadai backup prune --keep=5
```

### Clean uninstall

To remove all files managed by SquadAI from a project (preserving any content you wrote outside SquadAI marker blocks):

```sh
squadai remove --force
```

This deletes managed-only files and strips `<!-- squadai:... -->` marker blocks from shared files. Your own content outside the markers is never touched. The `.squadai/` config directory itself is not removed.

---

## What's next

Full command reference: [`docs/commands.md`](commands.md)  
Backup and rollback details: [`docs/recovery.md`](recovery.md)  
Troubleshooting guide: [`docs/troubleshooting.md`](troubleshooting.md)  
Architecture internals: [`docs/architecture.md`](architecture.md)

Project Memory and Squad Refinement are active features. Memory commands manage the indexed `docs/memory/` exchange format, while new project configs also include native local memory metadata for the daily-driver roadmap.
