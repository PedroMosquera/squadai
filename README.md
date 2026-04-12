# Agent Manager Pro

Team-consistent AI setup with safe local customization.

A CLI tool that standardizes AI coding agent environments across a team. It manages configuration for OpenCode (team baseline), optional personal agents (Claude Code, Codex), and GitHub Copilot instructions — with policy enforcement, safe apply/rollback, and verification.

## Install

### Homebrew (recommended)

```sh
brew install alexmosquera/tap/agent-manager-pro
```

### Shell script

```sh
curl -sSL https://raw.githubusercontent.com/alexmosquera/agent-manager-pro/main/scripts/install.sh | sh
```

### GitHub Releases

Download the archive for your architecture from [Releases](https://github.com/alexmosquera/agent-manager-pro/releases), extract it, and move the `agent-manager` binary to a directory on your `PATH`.

### From source (requires Go 1.24+)

```sh
go install github.com/alexmosquera/agent-manager-pro/cmd/agent-manager@latest
```

## Quick Start

```sh
# Initialize a project (creates .agent-manager/project.json)
agent-manager init

# Initialize with team policy enforcement
agent-manager init --with-policy

# Preview what changes would be applied
agent-manager plan --dry-run

# Apply configuration with backup safety
agent-manager apply

# Verify compliance
agent-manager verify
```

## Commands

| Command | Description |
|---------|-------------|
| `agent-manager init` | Initialize project config and optional policy template |
| `agent-manager plan` | Compute action plan (`--dry-run` to preview) |
| `agent-manager apply` | Execute plan with backup and rollback safety |
| `agent-manager sync` | Idempotent reconciliation to desired state |
| `agent-manager verify` | Print compliance and health report |
| `agent-manager validate-policy` | Validate policy schema and lock/required consistency |
| `agent-manager backup create` | Snapshot managed files |
| `agent-manager backup list` | List available backups |
| `agent-manager restore <id>` | Restore from a backup |
| `agent-manager version` | Print version |

All mutating commands support `--dry-run` and `--json` flags.

## Configuration

Agent Manager Pro uses a three-layer configuration system with clear precedence:

1. **Team policy** (`.agent-manager/policy.json`) — locked fields cannot be overridden
2. **Project config** (`.agent-manager/project.json`) — per-repo settings
3. **User config** (`~/.agent-manager/config.json`) — personal preferences

### Operational Modes

- **team** — controlled by policy, required settings enforced
- **personal** — controlled by user config, optional adapters and defaults
- **hybrid** — both active, policy locked fields take precedence

### Example User Config

```json
{
  "version": 1,
  "mode": "hybrid",
  "adapters": {
    "opencode": { "enabled": true },
    "claude-code": { "enabled": false },
    "codex": { "enabled": false }
  },
  "components": {
    "memory": { "enabled": true }
  },
  "paths": {
    "backup_dir": "~/.agent-manager/backups"
  }
}
```

### Example Policy Config

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
    "adapters": {
      "opencode": { "enabled": true }
    },
    "components": {
      "memory": { "enabled": true }
    },
    "copilot": {
      "instructions_template": "standard"
    }
  }
}
```

## Architecture

```
Config Loading:  ~/.agent-manager/config.json (user)
                 .agent-manager/project.json  (project)
                 .agent-manager/policy.json   (policy)
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
                 (runs each action, records step results)
                 (on failure: rollback from backup)
                          |
                    verify.Verify()
                 (checks files exist, markers present, content current)
```

### Adapters

- **OpenCode** — team baseline adapter (required), manages memory component and system prompts
- **GitHub Copilot** — manages `.github/copilot-instructions.md` with marker blocks to preserve user-authored sections
- **Claude Code** — personal optional adapter, detected via binary and `~/.claude/` config
- **Codex** — personal optional adapter, detected via binary and `~/.codex/` config

### Safety

Every `apply` operation:
1. Snapshots all target files to a backup manifest
2. Executes steps in deterministic order with step-level logging
3. On failure, rolls back all completed steps from the manifest
4. Emits a failure summary with the exact file list

Backups can be listed with `agent-manager backup list` and restored with `agent-manager restore <id>`.

## Platform

macOS only (darwin/arm64 and darwin/amd64). Linux and Windows support is planned for a future release.

## License

MIT
