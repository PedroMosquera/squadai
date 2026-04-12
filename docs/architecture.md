# Architecture

This document describes the internal architecture of Agent Manager Pro.

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
User Config (~/.agent-manager/config.json)
Project Config (.agent-manager/project.json)
Policy Config (.agent-manager/policy.json)
              |
        config.Merge()
  (policy locked > project > user)
              |
        planner.Plan()
  (iterates enabled adapters x components + copilot)
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

- **AgentID** — `opencode`, `claude-code`, `codex`
- **AdapterLane** — `team` (required) or `personal` (optional)
- **ComponentID** — `memory` (V1)
- **OperationalMode** — `team`, `personal`, `hybrid`
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

- `opencode/` — team baseline adapter (always included)
- `claude/` — personal-lane adapter (included only when detected)
- `codex/` — personal-lane adapter (included only when detected)

### `internal/components`

Component installers implement `domain.ComponentInstaller`.

- `memory/` — installs memory protocol files for supported adapters
- `copilot/` — manages `.github/copilot-instructions.md` with marker blocks

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
<!-- agent-manager:SECTION_ID -->
managed content here
<!-- /agent-manager:SECTION_ID -->
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
~/.agent-manager/backups/<id>/manifest.json
~/.agent-manager/backups/<id>/files/0
~/.agent-manager/backups/<id>/files/1
```

Manifest fields: `id`, `timestamp`, `command`, `affected_files`, `status`.

Each `FileSnapshot` records: `path`, `existed_before`, `checksum_before`, `backup_file`.

### `internal/verify`

Aggregates verification checks from memory and copilot components, plus policy compliance. Returns a `VerifyReport` with individual check results.

### `internal/cli`

Command implementations. Each command function accepts `args` and `stdout`, loads config, and delegates to the appropriate subsystem.

### `internal/tui`

Minimal bubbletea TUI with two screens:
1. **Intro** — tool name, version, detected mode, adapter summary
2. **Menu** — Plan, Apply, Sync, Verify, Restore backup, Quit

Delegates to the same command handlers used by CLI.

## Key Design Principles

1. **Adapters own all paths** — no hardcoded agent paths outside adapter packages
2. **Marker blocks for managed content** — user content outside markers is never touched
3. **Atomic writes everywhere** — temp file + rename via `fileutil.WriteAtomic`
4. **Idempotency by default** — planner returns `ActionSkip` when state matches; writer skips when bytes match
5. **Fail loudly** — errors and warnings are always surfaced, never silently ignored
