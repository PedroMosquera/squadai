package governance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/fsnotify/fsnotify"
)

const debounce = 300 * time.Millisecond

// WatchOpts configures Watch behavior.
type WatchOpts struct {
	// AuditLog receives all drift events. May be nil.
	AuditLog *AuditLog
	// OnEvent is called synchronously for every drift event. May be nil.
	OnEvent func(Event)
}

// Watch monitors all files recorded in managed.json for projectDir.
// It blocks until ctx is cancelled. Drift events are forwarded to opts.OnEvent
// and, if opts.AuditLog is non-nil, persisted to the audit log.
//
// Watch watches the parent directories of managed files rather than the files
// themselves so that delete+recreate cycles are handled correctly on all platforms.
func Watch(ctx context.Context, projectDir string, opts WatchOpts) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("watch: %w", err)
	}
	defer w.Close()

	if err := watchManagedDirs(w, projectDir); err != nil {
		return err
	}

	if opts.AuditLog != nil {
		_ = opts.AuditLog.Append(Event{Kind: KindWatchStart, Path: projectDir})
	}
	defer func() {
		if opts.AuditLog != nil {
			_ = opts.AuditLog.Append(Event{Kind: KindWatchStop, Path: projectDir})
		}
	}()

	// Use a stopped timer so the first event fires a fresh debounce window.
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}
	pending := false

	for {
		select {
		case <-ctx.Done():
			return nil

		case _, ok := <-w.Events:
			if !ok {
				return nil
			}
			if !pending {
				timer.Reset(debounce)
				pending = true
			}

		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			return fmt.Errorf("watch: %w", err)

		case <-timer.C:
			pending = false
			runDriftCheck(projectDir, opts)
			// Re-register paths: new managed files may have appeared after an apply.
			_ = watchManagedDirs(w, projectDir)
		}
	}
}

// watchManagedDirs adds the parent directory of every managed / created file to w.
// Existing watches are silently de-duplicated by fsnotify.
func watchManagedDirs(w *fsnotify.Watcher, projectDir string) error {
	files, _ := managed.ListManagedFiles(projectDir)
	created, _ := managed.ListCreatedFiles(projectDir)

	seen := make(map[string]bool)
	for _, relPath := range append(files, created...) {
		dir := filepath.Dir(filepath.Join(projectDir, relPath))
		if seen[dir] {
			continue
		}
		seen[dir] = true
		if _, err := os.Stat(dir); err == nil {
			_ = w.Add(dir) // best-effort; may fail for non-existent dirs
		}
	}
	return nil
}

func runDriftCheck(projectDir string, opts WatchOpts) {
	results, err := CheckDrift(projectDir)
	if err != nil {
		return
	}
	for _, r := range results {
		if !r.Drifted() {
			continue
		}
		e := Event{
			Timestamp: time.Now().UTC(),
			Kind:      r.Kind,
			Path:      r.Path,
			Detail:    r.Detail,
		}
		if opts.OnEvent != nil {
			opts.OnEvent(e)
		}
		if opts.AuditLog != nil {
			_ = opts.AuditLog.Append(e)
		}
	}
}
