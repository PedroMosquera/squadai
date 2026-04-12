# Commands

Complete reference for all Agent Manager Pro commands.

## Global Flags

These flags are accepted by all mutating commands:

| Flag | Description |
|------|-------------|
| `--dry-run` | Preview changes without applying |
| `--json` | Machine-readable JSON output |
| `-h`, `--help` | Show help for any command |

---

## `agent-manager init`

Initialize a project for Agent Manager Pro.

```sh
agent-manager init [--with-policy]
```

Creates:
- `.agent-manager/project.json` — project config with defaults
- `~/.agent-manager/config.json` — user config (if it doesn't already exist)

With `--with-policy`:
- `.agent-manager/policy.json` — team policy template with locked fields

Existing files are never overwritten. The command reports `exists` for files that are already present.

**Example:**

```sh
$ agent-manager init --with-policy
  created .agent-manager/project.json
  created .agent-manager/policy.json
  created /Users/you/.agent-manager/config.json

Done. Review the generated files and commit them to your repository.
```

---

## `agent-manager plan`

Compute and display the action plan without making changes.

```sh
agent-manager plan [--dry-run] [--json]
```

The plan shows what `apply` would do. Each action is one of:
- **create** — file will be created
- **update** — file will be modified
- **skip** — file is already in desired state

If a team policy is active, any overridden user/project values are reported under "Policy overrides".

**Example:**

```sh
$ agent-manager plan
Mode: hybrid

Planned actions (2):
  create   Install memory protocol for opencode         ~/.opencode/memory/protocol.md
  create   Write copilot instructions                   .github/copilot-instructions.md

Use 'agent-manager apply' to execute.
```

**JSON output:**

```sh
agent-manager plan --json
```

Returns an array of `PlannedAction` objects.

---

## `agent-manager apply`

Execute the action plan with backup safety.

```sh
agent-manager apply [--dry-run] [--json]
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
$ agent-manager apply
Backup: a1b2c3d4-e5f6-...

  [ok] Install memory protocol for opencode
  [ok] Write copilot instructions

Apply complete. Use 'agent-manager verify' to check.
```

**Dry run:**

```sh
$ agent-manager apply --dry-run
Dry run: 2 action(s) would be executed.
  create   Install memory protocol for opencode
  create   Write copilot instructions
```

On failure, the output includes the backup ID and instructions for manual restore.

---

## `agent-manager sync`

Idempotent reconciliation to desired state.

```sh
agent-manager sync [--dry-run] [--json]
```

Semantically identical to `apply`. The planner automatically skips actions where the current state matches the desired state, making repeated runs safe and no-op when everything is current.

---

## `agent-manager verify`

Run compliance checks and print a health report.

```sh
agent-manager verify [--json]
```

Checks include:
- Memory protocol files exist and contain correct content
- Copilot instructions file exists and contains managed marker blocks
- Policy-required values are in effect

**Example:**

```sh
$ agent-manager verify
  [PASS] memory protocol exists for opencode
  [PASS] memory protocol content is current for opencode
  [PASS] copilot-instructions.md exists
  [PASS] copilot managed section is present

All checks passed.
```

Failed checks include a message explaining what's wrong.

---

## `agent-manager validate-policy`

Validate the team policy file for schema correctness and lock/required consistency.

```sh
agent-manager validate-policy
```

Checks:
- Required `version` field is present and valid
- `locked` array entries reference known fields
- Every locked field has a corresponding entry in `required`
- `required` values are valid types

**Example:**

```sh
$ agent-manager validate-policy
Policy is valid. No issues found.
```

---

## `agent-manager backup create`

Manually snapshot all managed files.

```sh
agent-manager backup create [--json]
```

Creates a backup of every file that the planner identifies as managed, including files in `skip` state. Each backup gets a unique ID and timestamp.

**Example:**

```sh
$ agent-manager backup create
Backup created: a1b2c3d4-e5f6-...
  Files: 2
  Time:  2026-04-12 15:30:00 UTC
```

---

## `agent-manager backup list`

List all available backups.

```sh
agent-manager backup list [--json]
```

**Example:**

```sh
$ agent-manager backup list
Backups (2):

  ID                                    COMMAND     FILES  STATUS
  a1b2c3d4-e5f6-...                     apply       2      complete
  e7f8a9b0-c1d2-...                     manual      2      complete
```

---

## `agent-manager restore`

Restore files from a backup snapshot.

```sh
agent-manager restore <backup-id> [--dry-run] [--json]
```

Behavior:
- Files that existed before the backup are restored to their original content
- Files that were created during the backed-up operation are removed

**Dry run:**

```sh
$ agent-manager restore a1b2c3d4 --dry-run
Dry run: would restore 2 file(s) from backup a1b2c3d4
  restore ~/.opencode/memory/protocol.md
  restore .github/copilot-instructions.md
```

---

## `agent-manager version`

Print the version.

```sh
$ agent-manager version
agent-manager v0.1.0
```

The version is set at build time via Go ldflags.

---

## Interactive TUI

When invoked with no arguments, `agent-manager` launches a minimal terminal UI:

1. **Intro screen** — shows tool name, version, current mode, and detected adapters
2. **Menu** — Plan (dry-run), Apply, Sync, Verify, Restore backup, Quit

The TUI delegates to the same command handlers as the CLI.
