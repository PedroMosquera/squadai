package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/squadai/internal/exitcode"
)

// RunInstallHooks installs a pre-commit Git hook that runs `squadai verify --strict`.
// Idempotent: if the hook already contains the check, it is not duplicated.
func RunInstallHooks(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai install-hooks [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Install Git hooks for squadai:")
			fmt.Fprintln(stdout, "  pre-commit    → squadai verify --strict")
			fmt.Fprintln(stdout, "  post-merge    → squadai apply --no-review --json (when .squadai/ changed)")
			fmt.Fprintln(stdout, "  post-checkout → squadai apply --no-review --json (when .squadai/ changed)")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Idempotent: calling twice does not duplicate hooks.")
			fmt.Fprintln(stdout, "User-added lines outside the squadai block are preserved.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output the result as JSON.")
			return nil
		}
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	hooksDir := filepath.Join(projectDir, ".git", "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return exitcode.ErrPrecondition(
			"no .git/hooks directory found",
			"Run this command from the root of a Git repository.")
	}

	installed := []string{}

	// pre-commit hook
	if err := installHook(hooksDir, "pre-commit", "squadai verify --strict"); err != nil {
		return fmt.Errorf("write pre-commit hook: %w", err)
	}
	installed = append(installed, "pre-commit")

	// post-merge hook — runs squadai apply when .squadai/ changed
	postMergeBody := `if git diff --name-only ORIG_HEAD HEAD 2>/dev/null | grep -q '^\.squadai/'; then
  squadai apply --no-review --json >/dev/null
fi`
	if err := installHookWithBody(hooksDir, "post-merge", postMergeBody); err != nil {
		return fmt.Errorf("write post-merge hook: %w", err)
	}
	installed = append(installed, "post-merge")

	// post-checkout hook — runs squadai apply when .squadai/ changed
	postCheckoutBody := `if git diff --name-only HEAD@{1} HEAD 2>/dev/null | grep -q '^\.squadai/'; then
  squadai apply --no-review --json >/dev/null
fi`
	if err := installHookWithBody(hooksDir, "post-checkout", postCheckoutBody); err != nil {
		return fmt.Errorf("write post-checkout hook: %w", err)
	}
	installed = append(installed, "post-checkout")

	if jsonOut {
		writeJSONResult(stdout, true, map[string]any{
			"hooks":     installed,
			"hooks_dir": hooksDir,
		})
		return nil
	}

	for _, h := range installed {
		fmt.Fprintf(stdout, "installed %s hook → %s\n", h, filepath.Join(hooksDir, h))
	}
	return nil
}

// installHook writes a hook that runs a single squadai command, appending to
// an existing hook if one exists (without duplicating the squadai line).
func installHook(hooksDir, name, squadaiCmd string) error {
	hookPath := filepath.Join(hooksDir, name)
	existing, readErr := os.ReadFile(hookPath)
	if readErr == nil && strings.Contains(string(existing), squadaiCmd) {
		return nil
	}

	var content string
	if readErr == nil && len(existing) > 0 {
		content = strings.TrimRight(string(existing), "\n") + "\n\n" + squadaiCmd + "\n"
	} else {
		content = "#!/bin/sh\nset -e\n\n" + squadaiCmd + "\n"
	}

	return os.WriteFile(hookPath, []byte(content), 0755)
}

// hooksInstalled returns true when all three squadai-managed hooks (pre-commit,
// post-merge, post-checkout) are already installed in the project's .git/hooks
// directory. Any error during detection causes it to return false so callers
// can silently skip optional behaviour.
func hooksInstalled(projectDir string) bool {
	hooksDir := filepath.Join(projectDir, ".git", "hooks")

	// pre-commit is installed via installHook, so presence is detected by
	// the command string it injects.
	preCommit, err := os.ReadFile(filepath.Join(hooksDir, "pre-commit"))
	if err != nil || !strings.Contains(string(preCommit), "squadai verify --strict") {
		return false
	}

	// post-merge and post-checkout are installed via installHookWithBody, so their
	// presence is detected by the "# squadai: <name>" marker line.
	for _, name := range []string{"post-merge", "post-checkout"} {
		data, err := os.ReadFile(filepath.Join(hooksDir, name))
		if err != nil || !strings.Contains(string(data), "# squadai: "+name) {
			return false
		}
	}
	return true
}

// installHookWithBody writes a hook with a multi-line body, appending to
// an existing hook if one exists (without duplicating the squadai marker).
func installHookWithBody(hooksDir, name, body string) error {
	hookPath := filepath.Join(hooksDir, name)
	marker := "# squadai: " + name

	existing, readErr := os.ReadFile(hookPath)
	if readErr == nil && strings.Contains(string(existing), marker) {
		return nil
	}

	var content string
	if readErr == nil && len(existing) > 0 {
		content = strings.TrimRight(string(existing), "\n") + "\n\n" + marker + "\n" + body + "\n"
	} else {
		content = "#!/bin/sh\n\n" + marker + "\n" + body + "\n"
	}

	return os.WriteFile(hookPath, []byte(content), 0755)
}

// ─── Plugins marketplace ──────────────────────────────────────────────────────
