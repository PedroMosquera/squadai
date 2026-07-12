package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/backup"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/PedroMosquera/squadai/internal/marker"
	"github.com/PedroMosquera/squadai/internal/planner"
	"github.com/PedroMosquera/squadai/internal/state"
)

// RemoveOptions configures a Remove operation.
type RemoveOptions struct {
	DryRun     bool
	JSON       bool
	ProjectDir string // when empty, uses os.Getwd()
}

// RemoveReport is the result of a Remove operation.
type RemoveReport struct {
	RemovedFiles []string `json:"removed_files"` // files deleted entirely
	CleanedFiles []string `json:"cleaned_files"` // files with marker blocks stripped
	Errors       []string `json:"errors"`
	DryRun       bool     `json:"dry_run"`
}

// Remove removes all SquadAI-managed configuration from the project:
//   - Files in created_files (sidecar) are deleted entirely.
//   - Files in managed_files (sidecar) have their marker blocks stripped; if
//     the file becomes empty (or only whitespace) after stripping, it is deleted.
//   - On success (non-dry-run) the sidecar itself is removed via DeleteSidecar.
func Remove(opts RemoveOptions) (RemoveReport, error) {
	projectDir := opts.ProjectDir
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			return RemoveReport{}, fmt.Errorf("resolve working directory: %w", err)
		}
	}

	createdFiles, err := managed.ListCreatedFiles(projectDir)
	if err != nil {
		return RemoveReport{}, fmt.Errorf("list created files: %w", err)
	}

	managedFiles, err := managed.ListManagedFiles(projectDir)
	if err != nil {
		return RemoveReport{}, fmt.Errorf("list managed files: %w", err)
	}

	report := RemoveReport{
		RemovedFiles: []string{},
		CleanedFiles: []string{},
		Errors:       []string{},
		DryRun:       opts.DryRun,
	}

	// --- Process created_files: delete entirely ---
	for _, relPath := range createdFiles {
		absPath := filepath.Join(projectDir, relPath)
		if opts.DryRun {
			if _, statErr := os.Stat(absPath); statErr == nil {
				report.RemovedFiles = append(report.RemovedFiles, absPath)
			}
			continue
		}
		if removeErr := os.Remove(absPath); removeErr != nil && !os.IsNotExist(removeErr) {
			report.Errors = append(report.Errors, fmt.Sprintf("remove %s: %v", absPath, removeErr))
		} else {
			report.RemovedFiles = append(report.RemovedFiles, absPath)
		}
	}

	// --- Process managed_files: strip marker blocks ---
	for _, relPath := range managedFiles {
		absPath := filepath.Join(projectDir, relPath)
		data, readErr := os.ReadFile(absPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			report.Errors = append(report.Errors, fmt.Sprintf("read %s: %v", absPath, readErr))
			continue
		}

		stripped, hasMarkers := marker.StripAll(string(data))
		if !hasMarkers {
			// Nothing to strip — file has no marker blocks managed by us.
			continue
		}

		if opts.DryRun {
			if strings.TrimSpace(stripped) == "" {
				report.RemovedFiles = append(report.RemovedFiles, absPath)
			} else {
				report.CleanedFiles = append(report.CleanedFiles, absPath)
			}
			continue
		}

		if strings.TrimSpace(stripped) == "" {
			// File becomes empty — delete it.
			if removeErr := os.Remove(absPath); removeErr != nil && !os.IsNotExist(removeErr) {
				report.Errors = append(report.Errors, fmt.Sprintf("remove %s: %v", absPath, removeErr))
			} else {
				report.RemovedFiles = append(report.RemovedFiles, absPath)
			}
		} else {
			// Preserve user content outside marker blocks.
			if _, writeErr := fileutil.WriteAtomic(absPath, []byte(stripped), 0644); writeErr != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("write %s: %v", absPath, writeErr))
			} else {
				report.CleanedFiles = append(report.CleanedFiles, absPath)
			}
		}
	}

	// Clean up sidecar unless dry-run.
	if !opts.DryRun {
		if delErr := managed.DeleteSidecar(projectDir); delErr != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("delete sidecar: %v", delErr))
		}
	}

	return report, nil
}

// removeResult is the JSON representation of a successful remove run.
type removeResult struct {
	BackupID string   `json:"backup_id"`
	Deleted  []string `json:"deleted"`
	Stripped []string `json:"stripped"`
	DryRun   bool     `json:"dry_run"`
}

// RunRemove removes all SquadAI managed files from the current project.
// Files with marker blocks that also contain user content are stripped of
// the managed sections while preserving user content.
func RunRemove(args []string, stdout io.Writer) error {
	dryRun := false
	jsonOut := false
	force := false
	respectState := true
	var explicitAgents []string

	for _, arg := range args {
		switch {
		case arg == "--dry-run":
			dryRun = true
		case arg == "--json":
			jsonOut = true
		case arg == "--force":
			force = true
		case arg == "--respect-state" || arg == "--respect-state=true":
			respectState = true
		case arg == "--no-respect-state" || arg == "--respect-state=false":
			respectState = false
		case strings.HasPrefix(arg, "--agent=") || strings.HasPrefix(arg, "-a="):
			val := arg[strings.Index(arg, "=")+1:]
			if val != "" {
				explicitAgents = append(explicitAgents, strings.Split(val, ",")...)
			}
		case arg == "-h" || arg == "--help":
			fmt.Fprintln(stdout, "Usage: squadai remove [--force] [--dry-run] [--json] [--respect-state]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Remove all files managed by SquadAI from the current project. Files that")
			fmt.Fprintln(stdout, "contain marker blocks alongside user content are stripped of the managed sections")
			fmt.Fprintln(stdout, "only — user content is preserved. Fully managed files (no user content) are deleted.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "A backup is created automatically before any files are removed.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --force             Required to confirm removal (without it, the command errors).")
			fmt.Fprintln(stdout, "  --dry-run           Preview which files would be removed or stripped without changing anything.")
			fmt.Fprintln(stdout, "  --json              Output the result as JSON.")
			fmt.Fprintln(stdout, "  --agent=<csv>       Explicitly select agents to remove (bypasses state filter).")
			fmt.Fprintln(stdout, "  --respect-state     (default true) Restrict remove to previously-installed agents.")
			fmt.Fprintln(stdout, "                      Use --no-respect-state to operate on all detected agents.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai remove --dry-run")
			fmt.Fprintln(stdout, "  squadai remove --force")
			fmt.Fprintln(stdout, "  squadai remove --force --json")
			return nil
		default:
			return fmt.Errorf("unknown flag %q for remove", arg)
		}
	}

	// Without --force or --dry-run, refuse to proceed.
	if !force && !dryRun {
		return fmt.Errorf("refusing to remove without confirmation — use --force to confirm or --dry-run to preview")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	mergedCfg, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}

	adapters := DetectAdapters(homeDir)

	// Apply state-based filtering when --respect-state is active (default) and
	// the user did not explicitly pass --agent flags.
	if respectState && len(explicitAgents) == 0 {
		adapters = applyStateFilter(adapters, mergedCfg, homeDir)
	} else if len(explicitAgents) > 0 {
		adapters = filterAdapters(adapters, explicitAgents)
	}

	p := planner.New()
	actions, err := p.Plan(mergedCfg, adapters, homeDir, projectDir)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	// Collect ALL target paths (including skip actions — remove wants to clean
	// up everything managed, even files currently in sync).
	paths := collectAllTargetPaths(actions)

	if dryRun {
		result := removeResult{
			BackupID: "",
			Deleted:  []string{},
			Stripped: []string{},
			DryRun:   true,
		}

		// Classify each path as would-delete or would-strip.
		var wouldDelete []string
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return fmt.Errorf("read %s: %w", path, err)
			}
			stripped, hasMarkers := marker.StripAll(string(data))
			if hasMarkers && strings.TrimSpace(stripped) != "" {
				result.Stripped = append(result.Stripped, path)
			} else {
				result.Deleted = append(result.Deleted, path)
				wouldDelete = append(wouldDelete, path)
			}
		}

		// Also check .squadai/ directory.
		squadaiDir := filepath.Join(projectDir, ".squadai")
		if info, err := os.Stat(squadaiDir); err == nil && info.IsDir() {
			result.Deleted = append(result.Deleted, squadaiDir)
		}

		// Report directories that would become empty after removing the files above.
		wouldRemoveDirs := dryRunEmptyManagedDirs(projectDir, wouldDelete)
		result.Deleted = append(result.Deleted, wouldRemoveDirs...)

		if jsonOut {
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal remove result: %w", err)
			}
			fmt.Fprintln(stdout, string(data))
			return nil
		}

		if len(result.Deleted) == 0 && len(result.Stripped) == 0 {
			fmt.Fprintln(stdout, "Dry run: no managed files found.")
			return nil
		}
		fmt.Fprintf(stdout, "Dry run: would remove %d file(s), strip %d file(s).\n", len(result.Deleted), len(result.Stripped))
		for _, p := range result.Deleted {
			fmt.Fprintf(stdout, "  delete:  %s\n", p)
		}
		for _, p := range result.Stripped {
			fmt.Fprintf(stdout, "  strip:   %s (user content preserved)\n", p)
		}
		return nil
	}

	// Create a backup before removing anything.
	backupDir := backup.ResolveBackupDir(mergedCfg.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)

	var backupID string
	if len(paths) > 0 {
		manifest, err := store.SnapshotFiles(paths, "remove")
		if err != nil {
			return fmt.Errorf("create backup: %w", err)
		}
		backupID = manifest.ID
	}

	var deleted []string
	var stripped []string

	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				// File already gone — skip silently.
				continue
			}
			return fmt.Errorf("read %s: %w", path, readErr)
		}

		strippedContent, hasMarkers := marker.StripAll(string(data))

		if hasMarkers && strings.TrimSpace(strippedContent) != "" {
			// File has markers AND user content — write back the stripped version.
			if _, writeErr := fileutil.WriteAtomic(path, []byte(strippedContent), 0644); writeErr != nil {
				return fmt.Errorf("write stripped %s: %w", path, writeErr)
			}
			stripped = append(stripped, path)
		} else {
			// Fully managed file (either: markers with no user content, or no markers
			// at all meaning the whole file is ours) — delete it.
			if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("remove %s: %w", path, removeErr)
			}
			deleted = append(deleted, path)
		}
	}

	// Normalise nil slices to empty slices for consistent JSON output.
	if deleted == nil {
		deleted = []string{}
	}
	if stripped == nil {
		stripped = []string{}
	}

	if jsonOut {
		result := removeResult{
			BackupID: backupID,
			Deleted:  deleted,
			Stripped: stripped,
			DryRun:   false,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal remove result: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	if backupID != "" {
		fmt.Fprintf(stdout, "Backup created: %s\n", backupID)
	}

	// Clean up the .squadai/ directory (project.json, managed.json, templates, etc.).
	squadaiDir := filepath.Join(projectDir, ".squadai")
	if err := os.RemoveAll(squadaiDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove .squadai directory: %w", err)
	}
	if _, err := os.Stat(squadaiDir); os.IsNotExist(err) {
		deleted = append(deleted, squadaiDir)
	}

	// Remove any empty parent directories left behind after file deletion.
	removedDirs := removeEmptyManagedDirs(projectDir, deleted)
	deleted = append(deleted, removedDirs...)

	fmt.Fprintf(stdout, "Removed %d files, stripped markers from %d files.\n", len(deleted), len(stripped))
	for _, p := range deleted {
		fmt.Fprintf(stdout, "  deleted: %s\n", p)
	}
	for _, p := range stripped {
		fmt.Fprintf(stdout, "  stripped: %s (user content preserved)\n", p)
	}

	// Update state: remove agents that were just removed (warning-only on failure).
	agentIDs := make([]string, 0, len(adapters))
	for _, a := range adapters {
		agentIDs = append(agentIDs, string(a.ID()))
	}
	if statePath, pathErr := state.DefaultPath(); pathErr == nil {
		if st, loadErr := state.Load(statePath); loadErr == nil {
			st.RemoveAgents(agentIDs)
			if saveErr := state.Save(statePath, st); saveErr != nil {
				fmt.Fprintf(stdout, "Warning: could not save state: %v\n", saveErr)
			}
		} else {
			fmt.Fprintf(stdout, "Warning: could not load state: %v\n", loadErr)
		}
	}

	return nil
}

// collectAllTargetPaths extracts unique target paths from all actions (including skips).
func collectAllTargetPaths(actions []domain.PlannedAction) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, a := range actions {
		if a.TargetPath != "" && !seen[a.TargetPath] {
			seen[a.TargetPath] = true
			paths = append(paths, a.TargetPath)
		}
	}
	return paths
}

// dryRunEmptyManagedDirs computes which ancestor directories of the given
// paths WOULD become empty if those paths were deleted. It does not modify
// the filesystem. Used by the dry-run branch of RunRemove.
func dryRunEmptyManagedDirs(projectDir string, wouldDeletePaths []string) []string {
	// Build a set of paths that would be deleted so we can simulate their removal.
	willDelete := make(map[string]bool, len(wouldDeletePaths))
	for _, p := range wouldDeletePaths {
		willDelete[p] = true
	}

	// Collect all ancestor directories up to projectDir.
	candidates := make(map[string]bool)
	for _, p := range wouldDeletePaths {
		dir := filepath.Dir(p)
		for dir != projectDir && dir != "/" && dir != "." && len(dir) > len(projectDir) {
			candidates[dir] = true
			dir = filepath.Dir(dir)
		}
	}

	// Sort deepest-first.
	sorted := make([]string, 0, len(candidates))
	for d := range candidates {
		sorted = append(sorted, d)
	}
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })

	// Simulate deletion: for each candidate dir, check if all its current entries
	// are in the willDelete set (files) or in the would-be-removed dirs set (dirs).
	wouldRemove := make(map[string]bool)
	var result []string
	for _, dir := range sorted {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		allGone := true
		for _, e := range entries {
			entryPath := filepath.Join(dir, e.Name())
			if !willDelete[entryPath] && !wouldRemove[entryPath] {
				allGone = false
				break
			}
		}
		if allGone {
			wouldRemove[dir] = true
			result = append(result, dir)
		}
	}
	return result
}

// removeEmptyManagedDirs removes empty directories that were left behind after
// deleting managed files. It walks deepest-first so that nested empty dirs are
// handled correctly (.claude/agents → .claude). Only directories that are
// ancestors of deleted paths (up to projectDir) are considered.
func removeEmptyManagedDirs(projectDir string, deletedPaths []string) []string {
	// Collect all ancestor directories of deleted files, stopping at projectDir.
	candidates := make(map[string]bool)
	for _, p := range deletedPaths {
		dir := filepath.Dir(p)
		for dir != projectDir && dir != "/" && dir != "." && len(dir) > len(projectDir) {
			candidates[dir] = true
			dir = filepath.Dir(dir)
		}
	}

	// Sort deepest-first (longest path first) to handle nested dirs correctly.
	sorted := make([]string, 0, len(candidates))
	for d := range candidates {
		sorted = append(sorted, d)
	}
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })

	var removed []string
	for _, dir := range sorted {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // dir may have already been removed by a parent removal
		}
		if len(entries) == 0 {
			if err := os.Remove(dir); err == nil {
				removed = append(removed, dir)
			}
		}
	}
	return removed
}
