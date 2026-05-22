# Architecture

This document describes the internal architecture of SquadAI.

## Layers

The system is organized into six layers, each with a clear responsibility:

```
1. Domain contracts     (types, interfaces, errors — no side effects)
2. Config merge/validate (three-layer precedence resolution)
3. Planner              (compute intended actions from merged config)
4. Pipeline executor    (apply actions with backup/rollback safety)
5. Verifier             (post-apply compliance assertions)
6. Interfaces           (CLI commands, TUI)
```

## Data Flow

```
User Config (~/.squadai/config.json)
Project Config (.squadai/project.json)
Policy Config (.squadai/policy.json)
              |
        config.Merge()
  (policy locked > project > user)
              |
        planner.Plan()
  (iterates enabled adapters x components + copilot + methodology team)
              |
        backup.SnapshotFiles()
  (copies all target files before mutation)
              |
        pipeline.Execute()
  (runs each PlannedAction, records StepResults)
  (on failure: rollback from backup)
              |
        verify.Verify()
  (checks files exist, markers present, content current)
```

## Package Map

### `internal/domain`

Core types and interfaces with no filesystem dependencies. Defines:

- **AgentID** — `opencode`, `claude-code`, `vscode-copilot`, `cursor`, `windsurf`
- **AdapterLane** — `team` (required) or `personal` (optional)
- **Methodology** — `tdd`, `sdd`, `conventional`
- **DelegationStrategy** — `native` (OpenCode, Cursor), `prompt` (Claude Code), `solo` (VS Code, Windsurf)
- **ComponentID** — `memory`, `rules`, `settings`, `mcp`, `agents`, `skills`, `commands`, `plugins`, `workflows`
- **OperationalMode** — `team`, `personal` (`hybrid` is a deprecated alias, resolved at load time)
- **PlannedAction** — a single step the planner produces
- **StepResult** — outcome of executing one action
- **ApplyReport** — full result of a plan execution
- **VerifyResult / VerifyReport** — compliance check results

Key interfaces:

- **Adapter** — agent detection, path resolution, component support
- **ComponentInstaller** — plan/apply/verify for a single component
- **ConfigLoader** — load and merge the three config layers
- **Planner** — compute action plan from merged config
- **Executor** — run plan with backup/rollback
- **Verifier** — post-apply state checks

### `internal/config`

Three-layer configuration loading and merging.

- `loader.go` — reads JSON files from well-known paths
- `merger.go` — applies precedence: policy locked > project > user
- `validator.go` — schema validation for each config type

### `internal/adapters`

Each adapter is isolated in its own package and implements `domain.Adapter`. No path logic exists outside adapter packages.

- `opencode/` — team baseline adapter (always included), native delegation
- `claude/` — personal-lane adapter, prompt-based delegation
- `vscode/` — personal-lane adapter (VS Code Copilot), solo delegation
- `cursor/` — personal-lane adapter, native delegation
- `windsurf/` — personal-lane adapter, solo delegation

### `internal/components`

Component installers implement `domain.ComponentInstaller`.

- `memory/` — installs memory protocol files for supported adapters
- `copilot/` — manages `.github/copilot-instructions.md` with marker blocks
- `rules/` — writes agent-specific rule/instruction files with team standards
- `settings/` — manages agent-specific settings and preferences
- `mcp/` — installs MCP server configurations for agents that support them
- `agents/` — installs sub-agent definitions from methodology team composition
- `skills/` — installs methodology-specific skill files
- `commands/` — installs custom slash commands for agents that support them
- `plugins/` — installs optional plugin configurations
- `workflows/` — installs workflow definitions

### `internal/fileutil`

Atomic file operations:

- `WriteAtomic` — writes via temp file + rename for crash safety, skips if content unchanged (idempotent)
- `ReadFileOrEmpty` — returns empty string if file doesn't exist

### `internal/marker`

HTML comment-based marker block system for managing content in shared files:

- `InjectSection(document, sectionID, content)` — inserts or replaces a managed section
- `ExtractSection(document, sectionID)` — reads content between markers
- `HasSection(document, sectionID)` — checks if markers exist

Marker format:
```
<!-- squadai:SECTION_ID -->
managed content here
<!-- /squadai:SECTION_ID -->
```

Content outside markers is never modified.

### `internal/planner`

Computes a list of `PlannedAction` values from merged config and detected adapters. Returns `ActionSkip` for items already in desired state (idempotency).

### `internal/pipeline`

Executes planned actions in order. Before mutating any files, it creates a backup snapshot via the backup store. On failure, it rolls back all completed steps and marks remaining steps as `rolled_back`.

### `internal/backup`

Manages backup manifests and file snapshots on disk.

Storage layout:
```
~/.squadai/backups/<id>/manifest.json
~/.squadai/backups/<id>/files/0
~/.squadai/backups/<id>/files/1
```

Manifest fields: `id`, `timestamp`, `command`, `affected_files`, `status`.

Each `FileSnapshot` records: `path`, `existed_before`, `checksum_before`, `backup_file`.

### `internal/verify`

Aggregates verification checks from all component installers, plus policy compliance. Returns a `VerifyReport` with individual check results grouped by component.

### `internal/cli`

Command implementations. Each command function accepts `args` and `stdout`, loads config, and delegates to the appropriate subsystem.

### `internal/tui`

Bubbletea TUI with nine screens:
1. **Intro** — tool name, version, detected mode, adapter summary with delegation strategies
2. **Menu** — Init/Setup, Plan, Apply, Sync, Team Status, Verify, Restore backup, Quit
3. **Running** — progress indicator while a command executes
4. **Result** — command output display
5. **Init Methodology** — select TDD, SDD, or Conventional (shows role pipeline)
6. **Team Status** — shows current methodology, team roles, MCP servers, and plugins
7. **Init MCP** — toggle MCP servers (Context7 pre-selected)
8. **Init Plugins** — toggle available plugins (filtered by agent and methodology)
9. **Init Summary** — review all selections before confirming

Delegates to the same command handlers used by CLI.

## Key Design Principles

1. **Adapters own all paths** — no hardcoded agent paths outside adapter packages
2. **Marker blocks for managed content** — user content outside markers is never touched
3. **Atomic writes everywhere** — temp file + rename via `fileutil.WriteAtomic`
4. **Idempotency by default** — planner returns `ActionSkip` when state matches; writer skips when bytes match
5. **Fail loudly** — errors and warnings are always surfaced, never silently ignored

---

## Planned subsystems (upcoming phases)

### Squad Refinement (`/squadai-init`)
An in-agent slash command that reads the repository and refines installed agent/skill/command
templates per-codebase. State tracked in `.squadai/.squad-refined` (JSON). The `squadrefine`
package (planned: `internal/squadrefine/`) handles drift detection and nudge throttling.

### Project Memory
An indexed `docs/memory/` tree with `_inbox/`, `decisions/`, `learnings/`, `incidents/` buckets.
CLI: `squadai memory add|search|promote|reindex|status`. Librarian agent searches memory before
planning. Per-adapter protocol injection (`<!-- squadai:memory-protocol -->` marker blocks).
