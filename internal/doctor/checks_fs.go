package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const catFS = "Filesystem"

// runFilesystem checks write access and backup directory status.
func (d *Doctor) runFilesystem(_ context.Context) []CheckResult {
	return []CheckResult{
		d.checkWriteAccess(d.projectDir, "project directory"),
		d.checkWriteAccess(filepath.Join(d.homeDir, ".squadai"), "~/.squadai/"),
		d.checkBackupDir(),
	}
}

// checkWriteAccess attempts to create a temporary file in dir to verify write access.
func (d *Doctor) checkWriteAccess(dir, label string) CheckResult {
	name := fmt.Sprintf("write access to %s", label)

	// Ensure directory exists first.
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// ~/.squadai/ is auto-fixable; project dir is not (needs init).
			if label == "~/.squadai/" {
				return CheckResult{
					Category:    catFS,
					Name:        name,
					Status:      CheckFail,
					Message:     fmt.Sprintf("%s does not exist yet", label),
					Detail:      dir,
					FixHint:     "Create ~/.squadai directory",
					AutoFixable: true,
				}
			}
			return warn(catFS, name,
				fmt.Sprintf("%s does not exist yet", label),
				dir,
				fmt.Sprintf("Run 'squadai init' to create %s", label))
		}
		return fail(catFS, name,
			fmt.Sprintf("cannot stat %s: %v", label, err), dir, "")
	}
	if !info.IsDir() {
		return fail(catFS, name,
			fmt.Sprintf("%s is not a directory", label), dir, "")
	}

	// Try creating a temp file.
	tmp, err := os.CreateTemp(dir, ".squadai-doctor-*")
	if err != nil {
		return fail(catFS, name,
			fmt.Sprintf("no write access to %s: %v", label, err),
			dir,
			fmt.Sprintf("Fix permissions on %s", dir))
	}
	tmp.Close()           // #nosec G104 — temp file, ignore close error
	os.Remove(tmp.Name()) // #nosec G104 — best effort cleanup

	return pass(catFS, name,
		fmt.Sprintf("write access to %s", label), dir)
}

// checkBackupDir reports the number of backups and total size.
func (d *Doctor) checkBackupDir() CheckResult {
	backupDir := filepath.Join(d.homeDir, ".squadai", "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return skip(catFS, "backup directory",
				fmt.Sprintf("backup directory %s does not exist yet (created on first apply)", backupDir))
		}
		return warn(catFS, "backup directory",
			fmt.Sprintf("backup directory error: %v", err), backupDir, "")
	}

	count := 0
	var totalSize int64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		count++
		// Walk subdirectory to sum sizes.
		subDir := filepath.Join(backupDir, e.Name())
		_ = filepath.Walk(subDir, func(_ string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				totalSize += info.Size()
			}
			return nil
		})
	}

	detail := fmt.Sprintf("%d backups, %s", count, formatBytes(totalSize))
	return pass(catFS, "backup directory",
		fmt.Sprintf("backup directory: %s (%s)", backupDir, detail), detail)
}

// formatBytes returns a human-readable byte size.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	val := fmt.Sprintf("%.1f", float64(b)/float64(div))
	val = strings.TrimRight(strings.TrimRight(val, "0"), ".")
	units := []string{"KB", "MB", "GB"}
	return fmt.Sprintf("%s %s", val, units[exp])
}
