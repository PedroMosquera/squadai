package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/squadai/internal/assets"
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

// RunInstallCommands writes SquadAI slash commands to .claude/commands/ and
// the squadai-manager agent to .claude/agents/. Idempotent.
func RunInstallCommands(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai install-commands [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Install SquadAI slash commands into .claude/commands/ and the")
			fmt.Fprintln(stdout, "squadai-manager agent into .claude/agents/.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Slash commands installed:")
			fmt.Fprintln(stdout, "  /squadai-plan      — Preview planned changes")
			fmt.Fprintln(stdout, "  /squadai-apply     — Apply configuration")
			fmt.Fprintln(stdout, "  /squadai-verify    — Run compliance checks")
			fmt.Fprintln(stdout, "  /squadai-status    — Show health overview")
			fmt.Fprintln(stdout, "  /squadai-doctor    — Run diagnostics")
			fmt.Fprintln(stdout, "  /squadai-context   — Dump config as LLM context")
			fmt.Fprintln(stdout, "  /squadai-init      — Tune agent roles for this codebase")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output result as JSON.")
			return nil
		}
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	commandsDir := filepath.Join(projectDir, ".claude", "commands")
	agentsDir := filepath.Join(projectDir, ".claude", "agents")

	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return exitcode.ErrPermission(commandsDir, err)
	}
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return exitcode.ErrPermission(agentsDir, err)
	}

	type fileResult struct {
		Path   string `json:"path"`
		Status string `json:"status"`
	}

	var installed []fileResult

	// Install slash commands.
	commandAssets := []string{
		"squadai-plan",
		"squadai-apply",
		"squadai-verify",
		"squadai-status",
		"squadai-doctor",
		"squadai-context",
		"squadai-init",
	}
	for _, name := range commandAssets {
		content, err := assets.Read("commands/" + name + ".md")
		if err != nil {
			return fmt.Errorf("read asset %s: %w", name, err)
		}
		dest := filepath.Join(commandsDir, name+".md")
		status := "installed"
		if _, statErr := os.Stat(dest); statErr == nil {
			status = "updated"
		}
		if err := os.WriteFile(dest, []byte(content+"\n"), 0644); err != nil {
			return exitcode.ErrPermission(dest, err)
		}
		installed = append(installed, fileResult{Path: dest, Status: status})
		if !jsonOut {
			fmt.Fprintf(stdout, "  [%s] %s\n", status, dest)
		}
	}

	// Install squadai-manager agent.
	agentContent, err := assets.Read("agents/squadai-manager.md")
	if err != nil {
		return fmt.Errorf("read asset squadai-manager.md: %w", err)
	}
	agentDest := filepath.Join(agentsDir, "squadai-manager.md")
	agentStatus := "installed"
	if _, statErr := os.Stat(agentDest); statErr == nil {
		agentStatus = "updated"
	}
	if err := os.WriteFile(agentDest, []byte(agentContent+"\n"), 0644); err != nil {
		return exitcode.ErrPermission(agentDest, err)
	}
	installed = append(installed, fileResult{Path: agentDest, Status: agentStatus})
	if !jsonOut {
		fmt.Fprintf(stdout, "  [%s] %s\n", agentStatus, agentDest)
	}

	if jsonOut {
		writeJSONResult(stdout, true, map[string]any{"installed": installed})
		return nil
	}

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Installed %d slash command(s) and 1 agent.\n", len(commandAssets))
	return nil
}

// ─── Plugins marketplace ──────────────────────────────────────────────────────
