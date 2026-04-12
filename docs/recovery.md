# Recovery

This document covers backup, rollback, and restore procedures in Agent Manager Pro.

## How Safety Works

Every `apply` operation follows this sequence:

1. **Plan** — compute which files will be created or modified
2. **Snapshot** — copy all target files to a backup before any mutation
3. **Execute** — apply changes one step at a time
4. **On failure** — automatically roll back all completed steps from the backup

This ensures that a failed apply never leaves the system in a partial state.

## Automatic Rollback

When any step in `apply` fails, the pipeline automatically:

1. Stops executing remaining steps
2. Restores all files from the pre-apply backup
3. Marks remaining steps as `rolled_back` in the report
4. Reports the failure with the backup ID

```sh
$ agent-manager apply
Backup: a1b2c3d4-e5f6-...

  [ok]   Install memory protocol for opencode
  [FAIL] Write copilot instructions
         error: permission denied: .github/copilot-instructions.md
  [SKIP] Update settings for opencode

Apply failed. All changes rolled back (backup: a1b2c3d4-e5f6-...).
Use 'agent-manager restore a1b2c3d4-e5f6-...' to manually restore if needed.
```

The `[ok]` step was undone by the automatic rollback. The filesystem is back to its pre-apply state.

## Manual Backup

Create a snapshot at any time:

```sh
agent-manager backup create
```

This snapshots all files managed by the planner, including those already in desired state. Useful before manual edits or as a safety checkpoint.

```sh
$ agent-manager backup create
Backup created: e7f8a9b0-c1d2-...
  Files: 2
  Time:  2026-04-12 15:30:00 UTC
```

## Listing Backups

```sh
agent-manager backup list
```

Shows all backups sorted by time (newest first):

```sh
$ agent-manager backup list
Backups (3):

  ID                                    COMMAND     FILES  STATUS
  e7f8a9b0-c1d2-...                     manual      2      complete
  a1b2c3d4-e5f6-...                     apply       2      rolled_back
  f3g4h5i6-j7k8-...                     apply       2      complete
```

### Backup Statuses

| Status | Meaning |
|--------|---------|
| `complete` | Backup created successfully, files are intact |
| `rolled_back` | Backup was used for automatic rollback during a failed apply |
| `restored` | Backup was used for a manual restore |

## Restoring from Backup

Restore files to their state at the time of a backup:

```sh
agent-manager restore <backup-id>
```

**Preview first with dry run:**

```sh
$ agent-manager restore a1b2c3d4 --dry-run
Dry run: would restore 2 file(s) from backup a1b2c3d4
  restore ~/.opencode/memory/protocol.md
  restore .github/copilot-instructions.md
```

**Execute the restore:**

```sh
$ agent-manager restore a1b2c3d4
Restored 2 file(s) from backup a1b2c3d4.
  restored ~/.opencode/memory/protocol.md
  restored .github/copilot-instructions.md
```

### Restore Behavior

- Files that **existed before** the backup are restored to their original content
- Files that **did not exist** before (created during the operation) are removed
- The backup manifest is updated to status `restored`

## Backup Storage

Backups are stored at:

```
~/.agent-manager/backups/<id>/manifest.json
~/.agent-manager/backups/<id>/files/0
~/.agent-manager/backups/<id>/files/1
...
```

The backup directory is configurable via the `paths.backup_dir` field in user config.

Each manifest contains:
- Backup ID (UUID)
- Timestamp (UTC)
- Command that triggered the backup
- List of affected files with checksums
- Current status

File contents are stored as numbered copies in the `files/` subdirectory. SHA-256 checksums are recorded for integrity verification.

## Critical Failure: Rollback Failed

In rare cases, the rollback itself may fail (e.g., disk full, permission changes during apply). This is reported as a critical error:

```
CRITICAL: rollback failed — manual recovery may be needed.
  Backup ID: a1b2c3d4-e5f6-...
  Try: agent-manager restore a1b2c3d4-e5f6-...
```

In this situation:

1. Note the backup ID from the error message
2. Fix the underlying issue (permissions, disk space)
3. Run `agent-manager restore <backup-id>` to manually restore
4. Run `agent-manager verify` to confirm the system is in a good state

## Troubleshooting

**"No managed files found to back up"**

The planner found no files to manage. This usually means no adapters are enabled or no components are configured. Check your config with `agent-manager plan`.

**"backup not found"**

The backup ID doesn't match any stored backup. List available backups with `agent-manager backup list`.

**Backup directory missing or unreadable**

Check that `~/.agent-manager/backups/` exists and is writable. The directory is created automatically on the first backup, but may need manual creation if the parent directory doesn't exist.

**Files changed between backup and restore**

The restore always writes the backed-up content, regardless of what's currently in the file. If you've made manual edits since the backup, those edits will be overwritten.
