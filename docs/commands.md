# Commands

Complete reference for all SquadAI commands.

## Global Flags

These flags are accepted by all mutating commands:

| Flag | Description |
|------|-------------|
| `--dry-run` | Preview changes without applying |
| `--json` | Machine-readable JSON output |
| `-h`, `--help` | Show help for any command |

---

## `squadai init`

Initialize a project for SquadAI.

```sh
squadai init [--methodology=<tdd|sdd|conventional>] [--mcp=<csv>] [--plugins=<csv>] [--with-policy] [--force]
```

Creates:
- `.squadai/project.json` — project config with defaults
- `.squadai/templates/team-standards.md` — language-specific team standards
- `.squadai/skills/` — starter skill files (code-review, testing, pr-description, find-skills)
- `~/.squadai/config.json` — user config (if it doesn't already exist)

With `--with-policy`:
- `.squadai/policy.json` — team policy template with locked fields

With `--methodology=<tdd|sdd|conventional>`:
- Sets the development methodology in `project.json`
- Generates team composition (TDD: 6 roles, SDD: 8 roles, Conventional: 4 roles)
- Enables the `agents` and `commands` components

With `--mcp=<csv>`:
- Comma-separated list of MCP server IDs to enable (e.g., `context7`)
- Omit to include all recommended servers

With `--plugins=<csv>`:
- Comma-separated list of plugin IDs to enable (e.g., `code-review`)
- Omit to skip plugin installation

With `--force`:
- Overwrites existing template and skill files
- Overwrites `project.json` if it already exists

Existing files are never overwritten without `--force`. The command reports `exists` for files that are already present.

**Example:**

```sh
$ squadai init --methodology=tdd --with-policy
  created .squadai/project.json
  created .squadai/policy.json
  created .squadai/templates/team-standards.md
  created .squadai/skills/code-review.md
  created .squadai/skills/testing.md
  created .squadai/skills/pr-description.md
  created .squadai/skills/find-skills.md
  created /Users/you/.squadai/config.json

Detected:
  Language: Go
  Project:  my-project
  Agents:   opencode, claude-code, cursor
  Methodology: tdd
  Team roles:  6
  MCP servers: context7

Run 'squadai apply' to configure your environment.
```

---

## `squadai plan`

Compute and display the action plan without making changes.

```sh
squadai plan [--dry-run] [--json]
```

The plan shows what `apply` would do. Each action is one of:
- **create** — file will be created
- **update** — file will be modified
- **skip** — file is already in desired state

If a team policy is active, any overridden user/project values are reported under "Policy overrides".

**Example:**

```sh
$ squadai plan
Mode: team
  create   Install memory protocol for opencode         ~/.opencode/memory/protocol.md
  create   Write copilot instructions                   .github/copilot-instructions.md

Use 'squadai apply' to execute.
```

**JSON output:**

```sh
squadai plan --json
```

Returns an array of `PlannedAction` objects.

---

## `squadai diff`

Preview what `apply` would change, rendered as unified diffs.

```sh
squadai diff [--json]
```

Computes the same action plan as `plan`, but instead of listing actions, shows the exact content changes for each file. Useful for reviewing what will be written before committing to `apply`.

- **create** actions show the full file content as additions
- **update** actions show a unified diff between current and desired content
- **skip** actions are omitted (no changes)

**Example:**

```sh
$ squadai diff
--- AGENTS.md (current)
+++ AGENTS.md (desired)
@@ -1,3 +1,12 @@
+<!-- squadai:memory-protocol -->
+# Memory Protocol
+...
+<!-- /squadai:memory-protocol -->

--- .github/copilot-instructions.md (new file)
+++ .github/copilot-instructions.md (desired)
@@ -0,0 +1,8 @@
+<!-- squadai:copilot -->
+# Project: my-project
+...
+<!-- /squadai:copilot -->
```

**JSON output:**

```sh
squadai diff --json
```

Returns an array of objects with `path`, `action`, and `diff` fields.

---

## `squadai apply`

Execute the action plan with backup safety.

```sh
squadai apply [--dry-run] [--json]
```

Steps:
1. Loads and merges config (user + project + policy)
2. Detects installed adapters
3. Computes action plan
4. Creates a backup of all target files
5. Executes each action in order
6. On failure: rolls back all changes from backup

**Example:**

```sh
$ squadai apply
Backup: a1b2c3d4-e5f6-...

  [ok] Install memory protocol for opencode
  [ok] Write copilot instructions

Apply complete. Use 'squadai verify' to check.
```

**Dry run:**

```sh
$ squadai apply --dry-run
Dry run: 2 action(s) would be executed.
  create   Install memory protocol for opencode
  create   Write copilot instructions
```

On failure, the output includes the backup ID and instructions for manual restore.

> **`apply` is idempotent** — re-running it is safe and skips up-to-date files automatically.

---

## `squadai verify`

Run compliance checks and print a health report.

```sh
squadai verify [--json]
```

Checks include:
- Memory protocol files exist and contain correct content
- Copilot instructions file exists and contains managed marker blocks
- Policy-required values are in effect

**Example:**

```sh
$ squadai verify
  [PASS] memory protocol exists for opencode
  [PASS] memory protocol content is current for opencode
  [PASS] copilot-instructions.md exists
  [PASS] copilot managed section is present

All checks passed.
```

Failed checks include a message explaining what's wrong.

---

## `squadai status`

Show project health: detected adapters, enabled components, and managed file states.

```sh
squadai status [--json]
```

Provides an at-a-glance summary of the current project configuration and file state without computing a full plan or running compliance checks. Faster than `verify` for quick orientation.

**Example:**

```sh
$ squadai status
Project: my-project
Language: Go
Methodology: tdd
Mode: team
  opencode       enabled   delegation: native
  claude-code    enabled   delegation: prompt
  cursor         enabled   delegation: native

Components (9):
  memory    enabled    rules     enabled    settings  enabled
  mcp       enabled    agents    enabled    skills    enabled
  commands  enabled    plugins   enabled    workflows enabled

Managed files (12):
  AGENTS.md                          current
  CLAUDE.md                          current
  .github/copilot-instructions.md    current
  .cursorrules                       current
  .windsurfrules                     stale
  .opencode/agents/orchestrator.md   current
  ...

MCP servers: context7
Team roles: 6 (orchestrator, brainstormer, planner, implementer, reviewer, debugger)
```

**JSON output:**

```sh
squadai status --json
```

Returns a structured object with `project`, `adapters`, `components`, `files`, and `team` fields.

---

## `squadai validate-policy`

Validate the team policy file for schema correctness and lock/required consistency.

```sh
squadai validate-policy
```

Checks:
- Required `version` field is present and valid
- `locked` array entries reference known fields
- Every locked field has a corresponding entry in `required`
- `required` values are valid types

**Example:**

```sh
$ squadai validate-policy
Policy is valid. No issues found.
```

---

## `squadai backup create`

Manually snapshot all managed files.

```sh
squadai backup create [--json]
```

Creates a backup of every file that the planner identifies as managed, including files in `skip` state. Each backup gets a unique ID and timestamp.

**Example:**

```sh
$ squadai backup create
Backup created: a1b2c3d4-e5f6-...
  Files: 2
  Time:  2026-04-12 15:30:00 UTC
```

---

## `squadai backup list`

List all available backups.

```sh
squadai backup list [--json]
```

**Example:**

```sh
$ squadai backup list
Backups (2):

  ID                                    COMMAND     FILES  STATUS
  a1b2c3d4-e5f6-...                     apply       2      complete
  e7f8a9b0-c1d2-...                     manual      2      complete
```

---

## `squadai backup delete`

Delete a specific backup snapshot.

```sh
squadai backup delete <backup-id>
```

Permanently removes a single backup by ID. The backup directory and all its stored file snapshots are deleted. This operation is irreversible.

**Example:**

```sh
$ squadai backup delete a1b2c3d4-e5f6-...
Deleted backup: a1b2c3d4-e5f6-...
```

If the backup ID does not exist, the command exits with an error:

```sh
$ squadai backup delete nonexistent
Error: backup not found: nonexistent
```

---

## `squadai backup prune`

Remove old backups, keeping the N most recent.

```sh
squadai backup prune [--keep=N] [--dry-run] [--json]
```

Sorts backups by creation time and deletes all but the most recent N. Default retention is 10 backups.

| Flag | Description |
|------|-------------|
| `--keep=N` | Number of backups to retain (default 10) |
| `--dry-run` | List backups that would be deleted without deleting |

**Example:**

```sh
$ squadai backup prune --keep=3
Pruned 5 backup(s), kept 3.
  deleted  20260401T120000Z-abc12345
  deleted  20260402T130000Z-def67890
  deleted  20260403T140000Z-ghi11111
  deleted  20260404T150000Z-jkl22222
  deleted  20260405T160000Z-mno33333
```

**Dry run:**

```sh
$ squadai backup prune --keep=3 --dry-run
Dry run: would prune 5 backup(s), keep 3.
  would delete  20260401T120000Z-abc12345
  would delete  20260402T130000Z-def67890
  ...
```

---

## `squadai restore`

Restore files from a backup snapshot.

```sh
squadai restore <backup-id> [--dry-run] [--json]
```

Behavior:
- Files that existed before the backup are restored to their original content
- Files that were created during the backed-up operation are removed

**Dry run:**

```sh
$ squadai restore a1b2c3d4 --dry-run
Dry run: would restore 2 file(s) from backup a1b2c3d4
  restore ~/.opencode/memory/protocol.md
  restore .github/copilot-instructions.md
```

---

## `squadai remove`

Remove all managed files and strip marker blocks from shared files.

```sh
squadai remove --force [--dry-run] [--json]
```

The `--force` flag is required to prevent accidental removal. Without it, the command exits with an error.

Behavior:
- Files that are entirely managed (e.g., `.opencode/agents/orchestrator.md`, `.cursor/mcp.json`) are deleted
- Files that contain both managed and user content (e.g., `AGENTS.md` with user notes outside markers) have only the managed marker blocks stripped; user content is preserved
- The `.squadai/` config directory is not removed — only the generated output files

**Example:**

```sh
$ squadai remove --force
  removed  .opencode/agents/orchestrator.md
  removed  .opencode/agents/brainstormer.md
  removed  .cursor/mcp.json
  stripped AGENTS.md (managed markers removed, user content preserved)
  stripped .github/copilot-instructions.md (managed markers removed)
  removed  .windsurf/workflows/tdd-workflow.md

Removed 4 files, stripped 2 files.
```

**Dry run:**

```sh
$ squadai remove --force --dry-run
Dry run: would remove 4 files, strip 2 files.
  would remove  .opencode/agents/orchestrator.md
  would strip   AGENTS.md
  ...
```

**Without --force:**

```sh
$ squadai remove
Error: --force is required to remove managed files.
```

---

## `squadai version`

Print the version.

```sh
$ squadai version
squadai v0.1.0
```

The version is set at build time via Go ldflags.

---

## Interactive TUI

When invoked with no arguments, `squadai` launches a terminal UI with nine screens:

1. **Intro** — tool name, version, current mode, detected adapters with delegation strategies
2. **Menu** — Init/Setup, Plan (dry-run), Apply, Sync, Team Status, Verify, Restore backup, Quit
3. **Running** — progress indicator while a command executes
4. **Result** — command output display
5. **Init Methodology** — select TDD, SDD, or Conventional (shows role pipeline for each)
6. **Team Status** — current methodology, team roles, MCP servers, and enabled plugins
7. **Init MCP** — toggle MCP servers (Context7 pre-selected by default)
8. **Init Plugins** — toggle available plugins (filtered by detected agents and methodology)
9. **Init Summary** — review all selections before confirming

The init wizard flow is: Methodology → MCP → Plugins → Summary → Apply.

The TUI delegates to the same command handlers as the CLI.
