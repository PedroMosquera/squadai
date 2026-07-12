# Troubleshooting

Common problems and solutions for SquadAI.

---

<!-- squadai:section:toc -->
## Contents

- [Installation](#installation)
- [Init and Configuration](#init-and-configuration)
- [Apply Failures](#apply-failures)
- [Verify Failures](#verify-failures)
- [Backup and Restore](#backup-and-restore)
- [Adapters Not Detected](#adapters-not-detected)
- [MCP Servers](#mcp-servers)
- [Policy Issues](#policy-issues)
- [Doctor Command](#doctor-command)
- [Plugins](#plugins)
- [Skills](#skills)
- [Remove / Uninstall](#remove--uninstall)
<!-- /squadai:section:toc -->

---

## Installation

**`squadai: command not found` after install**

The install script places the binary in `~/.local/bin` (or another directory on your `PATH`). If the shell does not find it, add the directory to `PATH`:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Add this line to your shell profile (`~/.zshrc`, `~/.bashrc`, etc.) to make it permanent.

**Permission denied when running the install script**

The install script requires write permission to the target directory. Run the curl command as a regular user (not root). If you need a system-wide install, install manually to `/usr/local/bin`.

**`squadai update` fails**

Make sure the binary is writable by your user. If `squadai` was installed to a root-owned path, re-install to a user-writable path.

---

## Init and Configuration

**`squadai init` exits with "project.json already exists"**

A `.squadai/project.json` is already present. To overwrite it:

```sh
squadai init --force
```

Without `--force`, `init` reports `exists` for pre-existing files and skips them.

**Init does not detect my language**

SquadAI detects the project language by looking for well-known files (`go.mod`, `package.json`, `Cargo.toml`, `requirements.txt`, etc.). If your project has an unusual structure, set the language manually after init:

```json
{
  "meta": {
    "language": "Go"
  }
}
```

**Config changes are not taking effect after editing `project.json`**

Run `squadai apply` after every config change. The planner re-reads all config layers on each invocation. Files that are already up to date are skipped automatically (idempotent).

**"Unknown field" error when loading config**

Config files are validated against a JSON schema. Run `squadai schema` to print the current schema for `project.json`, `policy.json`, and `config.json`. Remove or correct any unrecognized fields.

---

## Apply Failures

**Apply fails with "permission denied"**

The process lacks write permission for a target file or its parent directory. Common causes:

- Files owned by root (e.g., if SquadAI was previously run with `sudo`)
- `.git/`-adjacent paths with restricted permissions
- Agent config files locked by another process

Fix permissions and re-run `squadai apply`. SquadAI will roll back any partial changes automatically.

**Apply fails partway through and leaves partial changes**

SquadAI creates a backup before touching any file. If a step fails, all completed steps are rolled back automatically. The backup ID is printed in the error output:

```
Apply failed. All changes rolled back (backup: a1b2c3d4-...).
```

If the automatic rollback also fails (e.g., disk full), run:

```sh
squadai restore <backup-id>
```

**"No adapters detected" during apply**

SquadAI detects adapters by looking for agent binaries or config directories on your system. If no agents are found:

- Confirm at least one supported agent is installed (opencode, claude-code, cursor, vscode-copilot, windsurf, pi, codex).
- If an agent is installed but not detected, explicitly enable it in `.squadai/project.json`:

```json
{
  "adapters": {
    "claude-code": { "enabled": true }
  }
}
```

**Apply is slow**

SquadAI skips files whose content is already correct (byte-level comparison). On a large project with many managed files, the first apply may take a moment. Subsequent runs are faster because most files are skipped.

---

## Verify Failures

**`squadai verify` reports stale content**

A managed file exists but its content does not match what SquadAI would write today (e.g., a template was updated). Re-run apply to bring it up to date:

```sh
squadai apply
```

**`squadai verify` reports missing marker blocks**

The marker block was removed from a file that SquadAI shares with user content (e.g., `CLAUDE.md`, `AGENTS.md`). SquadAI only touches content inside `<!-- squadai:... -->` blocks — content outside is never modified. Re-run apply to re-inject the marker block.

**`squadai verify --strict` fails but `verify` passes**

`--strict` treats warnings as failures. Review the warning messages in the output; they describe which optional checks did not pass. These are typically non-critical issues that `apply` can resolve.

---

## Backup and Restore

**"backup not found" when running restore**

The backup ID does not match any stored backup. List available backups:

```sh
squadai backup list
```

Backups are stored at `~/.squadai/backups/`. If the directory was manually deleted or the backup was pruned, it cannot be recovered.

**Backup directory missing**

The backup directory is created automatically on the first backup. If `~/.squadai/backups/` is missing, create it:

```sh
mkdir -p ~/.squadai/backups
```

**Restore overwrote content I wanted to keep**

Restore always writes the backed-up content, regardless of what is currently in the file. If you had manual edits since the backup, they are overwritten. To avoid this, create a manual backup before making edits:

```sh
squadai backup create
```

**Too many backups accumulating**

Prune old backups, keeping only the most recent N:

```sh
squadai backup prune --keep=10
```

---

## Adapters Not Detected

SquadAI detects each agent by looking for its binary on `PATH` or its config directory in well-known locations. Detection rules:

| Adapter | Detected when |
|---------|---------------|
| `opencode` | `opencode` binary on PATH, or `opencode.json` / `.opencode/` present |
| `claude-code` | `claude` binary on PATH, or `~/.claude/` present |
| `cursor` | `cursor` binary on PATH, or `.cursor/` present |
| `vscode-copilot` | `code` binary on PATH, or `.vscode/` present |
| `windsurf` | `windsurf` binary on PATH, or `.windsurf/` present |

If an agent is installed but not detected, explicitly enable it in `.squadai/project.json` under `"adapters"`.

---

## MCP Servers

**MCP server is not appearing in my agent's config**

1. Confirm the server is listed in `.squadai/project.json` under `"mcp"` and `"enabled": true`.
2. Confirm the `mcp` component is enabled: `"components": { "mcp": { "enabled": true } }`.
3. Run `squadai apply` to propagate the config.
4. Run `squadai verify` to confirm the server was written correctly.

**Context7 is not working**

Context7 requires Node.js. Verify Node.js is installed:

```sh
node --version
npx --version
```

If Node.js is missing, install it and re-run `squadai apply`.

**Adding a custom MCP server**

Add the server to `.squadai/project.json`:

```json
{
  "mcp": {
    "my-server": {
      "type": "local",
      "command": ["node", "path/to/server.js"],
      "enabled": true
    }
  }
}
```

Then run `squadai apply` to write it to all detected agents.

---

## Policy Issues

**"Policy violation" appears in plan output**

A user or project config value conflicts with a locked policy field. The policy value wins. No action is required unless the policy itself needs updating. To inspect which fields are locked, view `.squadai/policy.json`.

**`squadai validate-policy` reports lock consistency errors**

Every field in the `"locked"` array must have a matching value in `"required"`. Add the missing required entry or remove the lock:

```sh
$ squadai validate-policy
Policy validation found 1 issue:
  1. locked field "adapters.cursor.enabled" has no corresponding required value
```

Fix by adding `"adapters": { "cursor": { "enabled": true } }` to the `"required"` block, or by removing `"adapters.cursor.enabled"` from `"locked"`.

**Policy file in the wrong location**

Policy must be at `.squadai/policy.json` (project root, relative to where you run `squadai`). It is not read from the user config directory.

---

## Doctor Command

`squadai doctor` runs a comprehensive set of checks and is the fastest way to diagnose most problems:

```sh
squadai doctor
```

Useful flags:

| Flag | Description |
|------|-------------|
| `--fix` | Attempt to auto-repair detected issues |
| `--json` | Machine-readable output |
| `--verbose` | Show details for passing checks too |
| `--category=<name>` | Run only checks in a specific category |
| `--check=<name>` | Run only a specific named check |

If `doctor --fix` cannot resolve an issue, it prints the manual steps needed.

---

## Plugins

**A plugin is not being installed**

Plugins are filtered during apply based on:

1. **Supported agents** — the plugin is skipped if none of its `supported_agents` are detected on your system.
2. **Methodology exclusion** — the `superpowers` plugin is excluded when methodology is `tdd`.
3. **Component disabled** — ensure `"components": { "plugins": { "enabled": true } }` is set.

Run `squadai plugins list` to see all available plugins and their current state.

**Syncing plugins after catalog changes**

```sh
squadai plugins sync
```

---

## Skills

**Skills are missing after apply**

Confirm the `skills` component is enabled in `project.json` and re-run apply:

```sh
squadai apply
```

Skills are written to each adapter's project skills directory (e.g., `.opencode/skills/`, `.claude/skills/`). Check that the target agent is detected.

**Installing community skills**

The `find-skills` skill (installed during `init`) enables your agent to discover and install skills from the community registry:

```sh
# Ask your agent to run:
npx skills find <topic>
npx skills install <skill-name>
```

Node.js is required.

---

## Remove / Uninstall

**`squadai remove` exits with an error without `--force`**

The `--force` flag is required to prevent accidental removal:

```sh
squadai remove --force
```

Use `--dry-run` first to preview what will be removed:

```sh
squadai remove --force --dry-run
```

**Shared files (e.g., `CLAUDE.md`) still contain SquadAI content after remove**

`remove` strips `<!-- squadai:... -->` marker blocks from shared files. If a marker block was manually malformed or the closing tag is missing, the stripping may not work correctly. Edit the file manually to remove the block, or restore to a pre-apply backup:

```sh
squadai backup list
squadai restore <backup-id>
```

**The `.squadai/` directory was not removed**

`remove` intentionally leaves the `.squadai/` config directory in place. It only removes generated output files. To fully clean up, delete the config directory manually:

```sh
rm -rf .squadai/
```

---

## Still stuck?

Run `squadai doctor --verbose` for a full diagnostic report. For architecture details, see [`docs/architecture.md`](architecture.md). For backup and recovery procedures, see [`docs/recovery.md`](recovery.md).
