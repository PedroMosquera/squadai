package update

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

// Apply checks for a pending update and, if present and applicable, atomically
// swaps the binary and re-execs the new binary.
//
// It writes status messages to stderr. On a successful swap it re-execs the
// process and does not return. On any non-fatal condition it returns nil so
// the caller can continue normally.
func Apply(stderr io.Writer) error {
	pending, err := LoadPendingUpdate()
	if err != nil {
		// Non-fatal: a corrupt manifest should not prevent startup.
		fmt.Fprintf(stderr, "squadai: warning: could not read pending update: %v\n", err)
		return nil
	}
	if pending == nil {
		return nil
	}

	// Verify the pending binary exists and is executable.
	info, err := os.Stat(pending.BinaryPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Stale manifest; clean up.
			_ = ClearPendingUpdate()
			return nil
		}
		fmt.Fprintf(stderr, "squadai: warning: pending update binary stat failed: %v\n", err)
		return nil
	}
	if info.Mode()&0o111 == 0 {
		fmt.Fprintf(stderr, "squadai: warning: pending update binary is not executable\n")
		_ = ClearPendingUpdate()
		return nil
	}

	// Determine the install path of the running binary.
	installPath, err := resolveInstallPath()
	if err != nil {
		fmt.Fprintf(stderr, "squadai: warning: could not resolve install path: %v\n", err)
		return nil
	}

	// Windows is out of scope.
	if runtime.GOOS == "windows" {
		fmt.Fprintf(stderr, "squadai: update %s downloaded. Windows auto-apply is not supported. Replace the binary manually.\n", pending.Version)
		return nil
	}

	// Check write permission on the install path.
	if !isWritableByUser(installPath) {
		fmt.Fprintf(stderr,
			"squadai: update %s is ready but the install path is not user-writable.\n"+
				"  Run `brew upgrade squadai` or reinstall manually.\n",
			pending.Version)
		return nil
	}

	// Atomic swap.
	if err := os.Rename(pending.BinaryPath, installPath); err != nil {
		fmt.Fprintf(stderr, "squadai: warning: binary swap failed: %v\n", err)
		return nil
	}

	// Clean up manifest — best effort.
	_ = ClearPendingUpdate()

	fmt.Fprintf(stderr, "✓ Updated to %s\n", pending.Version)

	// Re-exec with the new binary.
	if err := syscall.Exec(installPath, os.Args, os.Environ()); err != nil {
		// syscall.Exec failed but the swap succeeded — next launch will use the
		// new binary. Log a warning and continue with the old in-memory binary.
		fmt.Fprintf(stderr, "squadai: warning: re-exec failed: %v — continuing with old binary\n", err)
	}
	// If Exec succeeded it never returns. If it failed we fall through.
	return nil
}

// resolveInstallPath returns the real absolute path of the running binary,
// resolving symlinks so we operate on the actual file.
func resolveInstallPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("os.Executable: %w", err)
	}
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("eval symlinks: %w", err)
	}
	return real, nil
}

// isWritableByUser checks whether the current process can write to path.
func isWritableByUser(path string) bool {
	return os.WriteFile(path+".squadai-write-test", []byte{}, 0o644) == nil &&
		func() bool { _ = os.Remove(path + ".squadai-write-test"); return true }()
}
