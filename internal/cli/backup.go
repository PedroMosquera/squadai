package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/PedroMosquera/squadai/internal/backup"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/planner"
)

// RunBackupDelete removes a backup by ID.
func RunBackupDelete(args []string, stdout io.Writer) error {
	jsonOut := false
	var id string

	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai backup delete <id> [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Delete a backup snapshot by its ID. The backup and all its files are permanently")
			fmt.Fprintln(stdout, "removed. Use 'squadai backup list' to see available backup IDs.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output the result as JSON.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai backup delete 20240115T103000Z-abc123")
			fmt.Fprintln(stdout, "  squadai backup delete <id> --json")
			return nil
		default:
			if id == "" {
				id = arg
			} else {
				return fmt.Errorf("unexpected argument %q", arg)
			}
		}
	}

	if id == "" {
		return exitcode.ErrMissingArg("backup-id", "squadai backup delete <id>")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		merged = &domain.MergedConfig{
			Paths: domain.PathsConfig{BackupDir: "~/.squadai/backups"},
		}
	}

	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)

	manifest, err := store.Get(id)
	if err != nil {
		return exitcode.ErrBackupNotFound(id)
	}

	fileCount := len(manifest.AffectedFiles)

	if err := store.Delete(id); err != nil {
		return fmt.Errorf("delete backup: %w", err)
	}

	if jsonOut {
		result := map[string]interface{}{
			"backup_id": id,
			"status":    "deleted",
			"files":     fileCount,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal delete result: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "Deleted backup %s (%d files).\n", id, fileCount)
	return nil
}

// RunBackupPrune removes all but the N most recent backups.
func RunBackupPrune(args []string, stdout io.Writer) error {
	keep := 10
	jsonOut := false

	for _, arg := range args {
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "-h" || arg == "--help":
			fmt.Fprintln(stdout, "Usage: squadai backup prune [--keep=N] [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Remove all but the N most recent backup snapshots. Keeps the newest N backups")
			fmt.Fprintln(stdout, "and permanently deletes the rest. Use 'squadai backup list' to see available")
			fmt.Fprintln(stdout, "backups before pruning.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --keep=N   Number of recent backups to keep (default 10).")
			fmt.Fprintln(stdout, "  --json     Output the result as JSON.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai backup prune")
			fmt.Fprintln(stdout, "  squadai backup prune --keep=5")
			fmt.Fprintln(stdout, "  squadai backup prune --keep=3 --json")
			return nil
		case strings.HasPrefix(arg, "--keep="):
			val := strings.TrimPrefix(arg, "--keep=")
			n, err := strconv.Atoi(val)
			if err != nil {
				return fmt.Errorf("invalid --keep value %q: %w", val, err)
			}
			keep = n
		default:
			return fmt.Errorf("unknown flag %q for backup prune", arg)
		}
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		merged = &domain.MergedConfig{
			Paths: domain.PathsConfig{BackupDir: "~/.squadai/backups"},
		}
	}

	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)

	// Count current backups before pruning to report accurate "kept" count.
	manifests, err := store.List()
	if err != nil {
		return fmt.Errorf("list backups: %w", err)
	}
	total := len(manifests)

	deleted, err := store.Prune(keep)
	if err != nil {
		return fmt.Errorf("prune backups: %w", err)
	}

	kept := total - deleted

	if jsonOut {
		result := map[string]interface{}{
			"deleted": deleted,
			"kept":    keep,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal prune result: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	if deleted == 0 {
		fmt.Fprintf(stdout, "Nothing to prune (%d backups, keeping %d).\n", kept, keep)
		return nil
	}

	fmt.Fprintf(stdout, "Pruned %d backups (kept %d most recent).\n", deleted, kept)
	return nil
}

// RunBackupCreate creates a manual backup snapshot of all managed files.
func RunBackupCreate(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai backup create [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Create a manual snapshot of all files that SquadAI manages. The backup")
			fmt.Fprintln(stdout, "includes every file that would be written by apply, even those that are already")
			fmt.Fprintln(stdout, "up to date. Backups are stored under ~/.squadai/backups by default.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output the backup manifest as JSON (includes ID, timestamp, and file list).")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai backup create")
			fmt.Fprintln(stdout, "  squadai backup create --json")
			return nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}

	// Plan to discover which files would be affected.
	adapters := DetectAdapters(homeDir)
	p := planner.New()
	actions, err := p.Plan(merged, adapters, homeDir, projectDir)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	// Collect all target paths (including skip — we want a full snapshot).
	paths := collectAllTargetPaths(actions)
	if len(paths) == 0 {
		fmt.Fprintln(stdout, "No managed files found to back up.")
		return nil
	}

	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)
	manifest, err := store.SnapshotFiles(paths, "manual")
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}

	if jsonOut {
		data, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal backup manifest: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "Backup created: %s\n", manifest.ID)
	fmt.Fprintf(stdout, "  Files: %d\n", len(manifest.AffectedFiles))
	fmt.Fprintf(stdout, "  Time:  %s\n", manifest.Timestamp.Format("2006-01-02 15:04:05 UTC"))
	return nil
}

// RunBackupList lists available backups.
func RunBackupList(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai backup list [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "List all available backup snapshots. Shows the backup ID, the command that created")
			fmt.Fprintln(stdout, "the backup (apply or manual), the number of files captured, and the status.")
			fmt.Fprintln(stdout, "Use the ID with 'squadai restore <id>' to roll back to a specific snapshot.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output the backup list as a JSON array.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai backup list")
			fmt.Fprintln(stdout, "  squadai backup list --json")
			return nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, "")
	if err != nil {
		// If no project config, use default backup dir.
		merged = &domain.MergedConfig{
			Paths: domain.PathsConfig{BackupDir: "~/.squadai/backups"},
		}
	}

	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)
	manifests, err := store.List()
	if err != nil {
		return fmt.Errorf("list backups: %w", err)
	}

	if len(manifests) == 0 {
		fmt.Fprintln(stdout, "No backups found.")
		return nil
	}

	if jsonOut {
		data, err := json.MarshalIndent(manifests, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal backup list: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "Backups (%d):\n\n", len(manifests))
	fmt.Fprintf(stdout, "  %-36s  %-10s  %-5s  %s\n", "ID", "COMMAND", "FILES", "STATUS")
	for _, m := range manifests {
		fmt.Fprintf(stdout, "  %-36s  %-10s  %-5d  %s\n",
			m.ID, m.Command, len(m.AffectedFiles), m.Status)
	}
	return nil
}

// RunRestore restores files from a backup.
func RunRestore(args []string, stdout io.Writer) error {
	jsonOut := false
	dryRun := false
	var backupID string

	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "--dry-run":
			dryRun = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai restore <backup-id> [--dry-run] [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Restore managed files from a backup snapshot. Files that existed before the backup")
			fmt.Fprintln(stdout, "are written back to their original content; files that did not exist before are")
			fmt.Fprintln(stdout, "removed. The backup ID is printed after every apply and can be listed with")
			fmt.Fprintln(stdout, "'squadai backup list'.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --dry-run  Show which files would be restored or removed without changing anything.")
			fmt.Fprintln(stdout, "  --json     Output the restore result as JSON.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai restore 2024-01-15T10-30-00Z-abc123")
			fmt.Fprintln(stdout, "  squadai restore <id> --dry-run")
			return nil
		default:
			if backupID == "" {
				backupID = arg
			} else {
				return fmt.Errorf("unexpected argument %q", arg)
			}
		}
	}

	if backupID == "" {
		return exitcode.ErrMissingArg("backup-id", "squadai restore <backup-id>")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, "")
	if err != nil {
		merged = &domain.MergedConfig{
			Paths: domain.PathsConfig{BackupDir: "~/.squadai/backups"},
		}
	}

	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)

	manifest, err := store.Get(backupID)
	if err != nil {
		return exitcode.ErrBackupNotFound(backupID)
	}

	if dryRun {
		if jsonOut {
			data, err := json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal restore manifest: %w", err)
			}
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		fmt.Fprintf(stdout, "Dry run: would restore %d file(s) from backup %s\n", len(manifest.AffectedFiles), backupID)
		for _, f := range manifest.AffectedFiles {
			if f.ExistedBefore {
				fmt.Fprintf(stdout, "  restore %s\n", f.Path)
			} else {
				fmt.Fprintf(stdout, "  remove  %s\n", f.Path)
			}
		}
		return nil
	}

	if err := store.Restore(backupID); err != nil {
		return fmt.Errorf("restore: %w", err)
	}

	if jsonOut {
		result := map[string]interface{}{
			"backup_id": backupID,
			"restored":  len(manifest.AffectedFiles),
			"status":    "restored",
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal restore result: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "Restored %d file(s) from backup %s.\n", len(manifest.AffectedFiles), backupID)
	for _, f := range manifest.AffectedFiles {
		if f.ExistedBefore {
			fmt.Fprintf(stdout, "  restored %s\n", f.Path)
		} else {
			fmt.Fprintf(stdout, "  removed  %s\n", f.Path)
		}
	}
	return nil
}
