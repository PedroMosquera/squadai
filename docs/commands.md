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
squadai init [--methodology=<tdd|sdd|conventional>] [--mcp=<csv>] [--plugins=<csv>] [--model-tier=<balanced|performance|starter|manual>] [--agents=<csv>] [--preset=<solo-minimal|solo-power|team-standard|enterprise-locked|full-squad|lean|custom>] [--with-policy] [--force]
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

With `--preset=<name>`:
- `solo-minimal` — Conventional workflow, starter models, low-overhead local defaults
- `solo-power` — TDD workflow, balanced models, native memory/context/usage defaults
- `team-standard` — TDD workflow, balanced models, shared governance baseline
- `enterprise-locked` — SDD workflow, performance models, strict-profile baseline
- `full-squad` — compatibility preset for SDD + balanced models
- `lean` — compatibility preset for Conventional + starter models
- `custom` — keep explicit flags or wizard defaults

New configs include these local control-plane blocks:
- `memory` — native backend metadata with `docs/memory` export compatibility
- `context` — built-in profiles: `default`, `debug`, `feature`, `review`, `docs`, `incident`, `cheap`
- `usage` — approximate session/daily token budgets and enforcement mode
- `models` — cheap, balanced, and premium routing labels plus role overrides

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
squadai diff [--json] [--exit-code]
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

**CI drift check:**

```sh
squadai diff --exit-code
```

Exits 4 when `apply` would change any file and 0 when nothing would change. The diff is still printed, so the CI log shows what drifted. Combines with `--json`. Only agents detected on the machine running the check are compared, and a CI runner usually has none installed besides the always-on OpenCode baseline. For a CI gate use `squadai verify --ci`, which compares every agent the project config enables.

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
squadai verify [--json] [--strict] [--ci]
```

Checks include:
- Memory protocol files exist and contain correct content
- Copilot instructions file exists and contains managed marker blocks
- Policy-required values are in effect
- When Codex is enabled, `AGENTS.md` fits in Codex's `project_doc_max_bytes` budget (`rules-file-size-cap`)

Codex reads at most `project_doc_max_bytes` of `AGENTS.md` (32768 bytes by default) and truncates the rest, so anything past the cap never reaches the model. The budget is shared by the whole chain of `AGENTS.md` files from the repo root down to the working directory, and the root file is read first. Verify compares the root `AGENTS.md` against the cap, using `project_doc_max_bytes` from `.codex/config.toml` in the project, then `~/.codex/config.toml`, then the default. An oversized file is reported as a warning with its size and the cap; it does not fail verify.

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

**CI gate (`--ci`):**

```sh
squadai verify --ci [--json]
```

Checks that the committed project files match what `apply` would write for every adapter enabled in the project config, whether or not that agent is installed. It is the same comparison as `squadai diff --exit-code`, without depending on the machine:

- Agents come from `.squadai/project.json` (and `policy.json`), not from detection. Disabled adapters are still checked for stale SquadAI content that `apply` would remove.
- It plans against an empty scratch home, so `~/.squadai/config.json`, user-level files such as `~/.claude/CLAUDE.md`, agent binaries and environment variables (MCP tokens included) do not affect the result. The same checkout gives the same answer on a laptop and on a runner.
- Files the project gitignores are skipped and counted in a `project-content-gitignored` line, since a checkout can never contain them. Outside a git work tree, or without `git` on PATH, nothing is skipped.
- MCP servers you added yourself to `.mcp.json` and the other MCP files are not drift; only the servers SquadAI owns are compared.

Each drifted file is one failed result with a `path`: `project-file-missing` (apply would create it), `project-file-drift` (content differs, the diff is printed after the summary) or `project-file-stale` (apply would remove it). Policy overrides are reported as warnings. With `GITHUB_ACTIONS=true` each failure is also printed as a `::error file=<path>::` annotation, with paths relative to `GITHUB_WORKSPACE`; annotations are not printed with `--json`.

Exits 4 (`E-401`) on any drifted or missing file and 0 otherwise. When the policy has an `allowed` block, each declaration outside it is a failing `policy-allow-list` result and the command exits 3 (`E-201`), ahead of drift, because `apply` cannot fix it. `--strict` adds nothing under `--ci`. The repo's [`action.yml`](../action.yml) runs this command followed by `squadai scan`.

---

## `squadai scan`

Read-only security scan of the agent config in the current project. Nothing is written or executed.

```sh
squadai scan [--json] [--user] [--fail-on <info|low|medium|high|none>]
```

It reads the project MCP config of every supported harness, whether or not the harness is installed on this machine (`.mcp.json`, `.cursor/mcp.json`, `.windsurf/mcp_config.json`, `.pi/mcp.json`, the `mcp` block of `opencode.json`, and `[mcp_servers.*]` tables in `.codex/config.toml`), plus:

- hooks, `env`, `permissions` and `enabledPlugins` in `.claude/settings.json` and `.claude/settings.local.json`,
- hooks in `.codex/hooks.json` (same schema as Claude Code hooks),
- skills: `*/SKILL.md` under `.agents/skills/` and `.claude/skills/`,
- Pi extensions: `.pi/extensions/*.ts` and `.pi/extensions/*/index.ts`.

Skills and Pi extensions are free text, so only `SCAN-001` and `SCAN-002` run over them, line by line. The hook rules (`SCAN-003`, `SCAN-004`) stay on hook commands, where `tool_input` and package runners mean something.

User-level config under your home directory is scanned only with `--user`. It adds what the adapters know about: `~/.claude/settings.json` (hooks, `env`, plugins), `~/.codex/config.toml` MCP servers and `~/.codex/hooks.json`, the MCP key of each harness's user settings file (Cursor and Windsurf global MCP files, OpenCode `opencode.json`, Pi `settings.json`), each harness's user skills directory, and Pi's global extensions. Those findings carry absolute paths. VS Code user settings are skipped: the file is JSONC and squadai writes no MCP servers there. Claude Code's `~/.claude.json` is not read.

| Rule | Severity | Flags |
|------|----------|-------|
| `SCAN-001` | high | Literal secrets in MCP `env`, headers or args, hook commands, the settings `env` block, skills or Pi extensions: `sk-`, `ghp_`/`gho_`/`ghs_`, `github_pat_`, `xoxb-`/`xoxp-`, AWS `AKIA`, and high-entropy values under `*_TOKEN`/`*_KEY`/`*SECRET*`/`*PASSWORD*` names. `${VAR}`, `$VAR` and `{env:VAR}` references are fine. Messages never print the full value. |
| `SCAN-002` | high | Code downloaded and executed at runtime: `curl ... \| sh`, `wget ... \| bash`, `bash -c "$(curl ...)"`, `source <(curl ...)`. |
| `SCAN-003` | medium | `npx -y`, `bunx` or `uvx` running a package without an exact version, including `@latest`. |
| `SCAN-004` | high / medium | Hook commands (Claude Code or Codex) that pass `tool_input` to `eval`, `sh -c` or a pipe into a shell (high), or expand it in an unquoted `$(...)` (medium). |
| `SCAN-005` | low | `permissions.defaultMode: "bypassPermissions"` in project `.claude/settings.json`, which Claude Code has ignored since 2026-09-02. Not reported for `settings.local.json` or user settings. |
| `SCAN-006` | medium | MCP servers reached over plain `http://` on a host other than localhost or a loopback address. |
| `SCAN-007` | high | MCP servers, hook commands or `enabledPlugins` marketplaces outside the `allowed` lists in `.squadai/policy.json` (see [Allow-lists](policy.md#allow-lists)). Reports nothing when the policy has no `allowed` block. |

Exit codes: `0` when no finding is at or above `--fail-on` (default `high`), `3` (`E-203`) when at least one is, `2` for an invalid flag. `--fail-on none` always exits `0` after a successful scan. A config file that cannot be parsed is listed as skipped and does not stop the scan. A `.squadai/policy.json` that cannot be parsed fails the scan, so the allow-lists are never skipped silently.

`--json` prints `{"scanned": [...], "skipped": [...], "findings": [...]}`. Each finding has `id`, `severity`, `file` (relative to the project, absolute for files `--user` adds), `line` (omitted when unknown), `subject` and `message`, sorted by severity, then file and line.

The MCP servers in SquadAI's curated catalog launch through `npx -y` without a pinned version, so a default `apply` reports `SCAN-003` at medium. It does not fail the default gate.

---

## `squadai status`

Show project health: detected adapters, enabled components, and managed file states.

```sh
squadai status [--json] [--fix] [--daily]
```

Provides an at-a-glance summary of the current project configuration, managed files, compliance health, latest backup, and refinement state.

With `--daily`, prints a compact daily-driver dashboard for the current repo:
- active preset, methodology, mode, and context profile
- detected agents and configured MCP servers
- memory backend/export path and auto-capture flag
- approximate usage budgets and enforcement mode
- health, refinement drift, and latest backup

Use `--json` for the same data in machine-readable form, including `preset`, `context_profile`, `memory`, `usage`, and `model_tier`.

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
- `allowed` lists contain no empty patterns
- `.squadai/project.json` declares no MCP server, model, hook command or plugin source outside the `allowed` lists (see [Allow-lists](policy.md#allow-lists))

Exits `2` (`E-202`) when the policy file itself has issues and `3` (`E-201`) when project config is outside the allow-lists.

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

## Budget enforcement in agent sessions

`squadai token-usage --against-budget` checks budgets after the fact. For Claude Code, Codex and Pi, `squadai apply` also moves the check into the session, using the `usage` block of `.squadai/project.json`. Claude Code gets hooks in the project's `.claude/settings.json`, Codex gets hooks in `.codex/hooks.json`, and Pi gets the extension `.pi/extensions/squadai-budget.ts`:

```json
"usage": {
  "daily_token_budget": 200000,
  "session_token_budget": 50000,
  "enforcement": "block"
}
```

Hooks are installed only when `enforcement` is `warn`, `ask` or `block` and at least one token budget is set, and only for the agents enabled in the project. With `enforcement` unset or `off`, nothing is installed, and an earlier install is removed on the next apply. Your own hooks in the same file are kept as they are.

The table below is for Claude Code. Codex and Pi differences follow it.

| `enforcement` | Hook events | Over budget |
|---------------|-------------|-------------|
| `block` | `PreToolUse`, `UserPromptSubmit`, `Stop` | Tool calls and prompts are refused (exit 2) with the exceeded budget as the reason. Entries carry `onFailure: "block"`, so a hook that cannot start or times out also refuses (Claude Code 2.1.295 or later). |
| `ask` | `PreToolUse`, `UserPromptSubmit`, `Stop` | Claude Code asks before each tool call. Prompts get a warning. |
| `warn` | `UserPromptSubmit`, `Stop` | A warning is shown once per prompt and at the end of each turn. Nothing is refused. |

`Stop` never blocks: in Claude Code a blocking `Stop` hook keeps Claude working, the opposite of what a budget wants. It only warns.

The daily budget counts the last 24 hours of usage for this project, the same total `token-usage --against-budget` reports. The session budget counts the current session's transcript.

Each entry runs `squadai budget-hook --event <name>`, an internal command that reads the hook input on stdin. It needs `squadai` on the `PATH` Claude Code runs hooks with. It reads only the project config and local transcripts, makes no network calls, and has a 10 second timeout.

### Codex

`apply` merges entries into the project's `.codex/hooks.json`, which uses the Claude Code hooks schema. Each runs `squadai budget-hook --harness codex --event <name>`. Codex project hooks have the same contract as Claude Code for these events: exit 2 refuses a tool call or prompt with stderr as the reason, `systemMessage` shows a warning, and a blocking `Stop` would keep Codex working, so `Stop` only warns here too.

- `block` installs `PreToolUse`, `UserPromptSubmit` and `Stop`. Codex has no `onFailure`, so a hook that cannot start or times out lets the action through.
- `ask` behaves like `warn` (`UserPromptSubmit`, `Stop`). Codex rejects `permissionDecision: "ask"` from a hook.
- Codex runs project hooks only after you trust the project and then review and trust each new or changed hook in Codex. squadai does not write that trust for you.
- Usage comes from `ccusage codex session --json --offline` ([ccusage](https://github.com/ccusage/ccusage)), because the Codex session log format is not documented. ccusage reports no project directory, so the daily budget counts every Codex session with activity in the last 24 hours, in full, across all projects. The session budget counts the current session. Without `ccusage` on the `PATH` (or if it fails or exceeds 8 seconds), the hook allows everything and warns once per session that the budget is not enforced.

### Pi

`apply` writes `.pi/extensions/squadai-budget.ts`, a small extension with no dependencies that runs `squadai budget-hook --harness pi` and maps the result onto Pi's extension events:

| Claude Code event | Pi event | Over budget under `block` |
|-------------------|----------|---------------------------|
| `PreToolUse` | `tool_call` | The tool call is refused with the reason. |
| `UserPromptSubmit` | `input` | The prompt is dropped before any model call and the reason is shown. |
| `Stop` | `agent_before_settle` | A warning only. The extension never asks Pi to continue. |

- Under `block`, a hook that cannot run refuses, like `onFailure: "block"`.
- Under `ask`, Pi asks for confirmation before each tool call, and refuses when there is no UI to ask with (print and JSON modes).
- Under `warn`, the extension skips tool calls and warns on prompts and at the end of each run. In print and JSON modes, warnings and reasons go to stderr.
- Pi loads project extensions only for a trusted project (`/trust`, or `--approve` for one run).
- Usage comes from Pi's own session files. The daily budget counts this project's sessions modified in the last 24 hours, from the directory holding the current session file, so a custom `--session-dir` works. The session budget counts the current session file.
- The file is rewritten when `enforcement` changes and deleted when it is unset. Do not edit it.

Not covered yet: refusing a `/model` switch to a pricier model. Claude Code has a `PreModelSwitch` hook for it, but `usage` has no tier ceiling to compare against. `profile_tiers` maps context profiles to tiers for budget fitting, not to a maximum. Codex and Pi budget files are not removed when you disable those agents in the project.

---

## Memory capture at the end of Claude Code sessions

A hook cannot summarize a session, so this one asks Claude to. Opt in through the `memory` block of `.squadai/project.json`:

```json
"memory": {
  "backend": "native",
  "export_path": "docs/memory",
  "capture_on_stop": true
}
```

With `capture_on_stop` true and the `memory` component enabled, `squadai apply` adds a `Stop` hook running `squadai memory-hook --event Stop` to the project's `.claude/settings.json`. It sits beside any budget `Stop` hook and your own hooks. Set the flag to false (the default) and the next apply removes it.

When Claude finishes a turn, the hook asks it, once per session, to record the session's decisions, non-obvious fixes and dead ends with the `memory_add` MCP tool (or `squadai memory add`), and to stop without writing anything if nothing durable came out of it. Notes land in `docs/memory/_inbox/` like any other. The request arrives as `Stop` hook feedback (`hookSpecificOutput.additionalContext`), which gives Claude one more turn and is not shown as a hook error.

The hook stays silent, and Claude stops normally, when:

- the session has fewer than 10 tool calls in its transcript, so quick questions are never interrupted,
- Claude is already continuing because of a `Stop` hook (`stop_hook_active`),
- the session was already asked. A marker per session id is kept under `~/.squadai/memory-hook/` for 30 days.

Like `budget-hook`, it needs `squadai` on the `PATH` Claude Code runs hooks with, reads only the project config and the session transcript, and has a 10 second timeout. Codex and Pi are not covered yet.

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
