package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/PedroMosquera/squadai/internal/governance"
	"github.com/PedroMosquera/squadai/internal/pluginsdk"
)

func RunPluginsAddGit(args []string, stdout, stderr io.Writer) error {
	return RunPluginsAddGitWithReader(args, stdout, stderr, os.Stdin)
}

// RunPluginsAddGitWithReader is RunPluginsAddGit with an injectable stdin
// reader (for tests).
func RunPluginsAddGitWithReader(args []string, stdout, stderr io.Writer, stdin io.Reader) error {
	jsonOut := false
	yes := false
	var gitURL string

	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		} else if a == "--yes" || a == "-y" {
			yes = true
		} else if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "Usage: squadai plugins add-git <git-url> [--yes] [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Clone a git-based plugin into .squadai/plugins/<id>/.")
			fmt.Fprintln(stdout, "URL format: git:github.com/user/repo or git:https://github.com/user/repo.git")
			fmt.Fprintln(stdout, "Pin a commit for reproducible installs: git:github.com/user/repo@<full-sha>")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "The plugin is cloned into a staging area first; the resolved repo URL and")
			fmt.Fprintln(stdout, "everything the manifest declares are shown before anything is installed.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --yes / -y  Skip the interactive confirmation (required when not a TTY)")
			fmt.Fprintln(stdout, "  --json      Output result as JSON")
			return nil
		} else if gitURL == "" {
			gitURL = a
		}
	}

	if gitURL == "" {
		return fmt.Errorf("usage: squadai plugins add-git <git-url> [--yes] [--json]")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Fail closed before anything touches the network: without --yes we need
	// an interactive terminal to ask for confirmation.
	interactive := IsTTYHook != nil && IsTTYHook()
	if !yes && !interactive {
		return fmt.Errorf("refusing to install %q without confirmation — not an interactive terminal; re-run with --yes to confirm", gitURL)
	}

	_, repoURL, ref, err := pluginsdk.ParseGitURL(gitURL)
	if err != nil {
		return fmt.Errorf("parse git url: %w", err)
	}
	if !pluginsdk.IsCommitSHA(ref) {
		fmt.Fprintf(stderr, "WARNING: installing an unpinned ref — the contents of %s can change between installs.\n", repoURL)
		fmt.Fprintf(stderr, "         Pin a commit for reproducible installs: git:%s@<full-sha>\n", strings.TrimPrefix(repoURL, "https://"))
	}

	if !jsonOut {
		fmt.Fprintf(stdout, "cloning %s …\n", gitURL)
	}

	// Clone into a staging area (git clone executes no repository code) so the
	// manifest can be shown before the plugin lands in .squadai/plugins/.
	staged, err := pluginsdk.Stage(projectDir, gitURL)
	if err != nil {
		if jsonOut {
			writeJSONResult(stdout, false, map[string]any{"error": err.Error(), "url": gitURL})
		}
		return fmt.Errorf("install git plugin: %w", err)
	}

	printStagedSummary(stdout, staged)

	if !yes {
		fmt.Fprintf(stdout, "Install plugin %q? [y/N]: ", staged.PluginID)
		line, _ := bufio.NewReader(stdin).ReadString('\n')
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer != "y" && answer != "yes" {
			_ = staged.Discard()
			return fmt.Errorf("install aborted — plugin %q was not installed", staged.PluginID)
		}
	}

	result, err := staged.Commit()
	if err != nil {
		if jsonOut {
			writeJSONResult(stdout, false, map[string]any{"error": err.Error(), "url": gitURL})
		}
		return fmt.Errorf("install git plugin: %w", err)
	}

	auditPluginEvent(projectDir, governance.KindPluginInstall, result.Path,
		fmt.Sprintf("git plugin %q from %s (ref=%s pinned=%v commit=%s)",
			result.PluginID, result.RepoURL, refOrDefault(result.Ref), result.Pinned, result.CommitSHA))

	if jsonOut {
		out := map[string]any{
			"plugin_id": result.PluginID,
			"path":      result.Path,
			"repo_url":  result.RepoURL,
			"pinned":    result.Pinned,
			"commit":    result.CommitSHA,
		}
		if result.Manifest != nil {
			out["manifest"] = result.Manifest
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "installed plugin %q → %s\n", result.PluginID, result.Path)
	if result.CommitSHA != "" {
		fmt.Fprintf(stdout, "  commit     : %s\n", result.CommitSHA)
	}
	return nil
}

// printStagedSummary prints the resolved repo URL and everything the staged
// plugin's manifest declares, so the user confirms with full information.
func printStagedSummary(stdout io.Writer, staged *pluginsdk.StagedInstall) {
	fmt.Fprintf(stdout, "plugin %q resolved:\n", staged.PluginID)
	fmt.Fprintf(stdout, "  repo       : %s\n", staged.RepoURL)
	fmt.Fprintf(stdout, "  ref        : %s\n", refOrDefault(staged.Ref))
	if staged.CommitSHA != "" {
		fmt.Fprintf(stdout, "  commit     : %s\n", staged.CommitSHA)
	}
	if staged.Manifest == nil {
		fmt.Fprintln(stdout, "  manifest   : none (no plugin.json)")
		return
	}
	m := staged.Manifest
	if m.Description != "" {
		fmt.Fprintf(stdout, "  description: %s\n", m.Description)
	}
	for _, group := range []struct {
		label string
		items []string
	}{
		{"commands", m.Commands},
		{"skills", m.Skills},
		{"agents", m.Agents},
		{"mcp", m.MCP},
	} {
		for _, item := range group.items {
			fmt.Fprintf(stdout, "  %-11s: %s\n", group.label, item)
		}
	}
}

// RunPluginsRemoveGit deletes a git-based plugin from .squadai/plugins/<id>/.
func RunPluginsRemoveGit(args []string, stdout io.Writer) error {
	jsonOut := false
	var pluginID string
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		} else if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "Usage: squadai plugins remove-git <plugin-id> [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Delete a git-based plugin from .squadai/plugins/<id>/.")
			return nil
		} else if pluginID == "" {
			pluginID = a
		}
	}
	if pluginID == "" {
		return fmt.Errorf("usage: squadai plugins remove-git <plugin-id>")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}

	installed, err := pluginsdk.List(projectDir)
	if err != nil {
		return err
	}
	found := false
	for _, id := range installed {
		if id == pluginID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("plugin %q is not installed", pluginID)
	}

	if err := pluginsdk.Remove(projectDir, pluginID); err != nil {
		return fmt.Errorf("remove git plugin: %w", err)
	}

	auditPluginEvent(projectDir, governance.KindPluginRemove, pluginID,
		fmt.Sprintf("git plugin %q removed", pluginID))

	if jsonOut {
		writeJSONResult(stdout, true, map[string]any{"plugin_id": pluginID, "removed": true})
		return nil
	}
	fmt.Fprintf(stdout, "removed plugin %q\n", pluginID)
	return nil
}

// auditPluginEvent appends a plugin install/remove event to the project audit
// log. Audit failures never block the plugin operation itself.
func auditPluginEvent(projectDir string, kind governance.EventKind, path, detail string) {
	_ = governance.OpenAuditLog(projectDir).Append(governance.Event{
		Kind:   kind,
		Path:   path,
		Detail: detail,
	})
}

// refOrDefault renders an empty ref as the human-readable default.
func refOrDefault(ref string) string {
	if ref == "" {
		return "(default branch — unpinned)"
	}
	return ref
}
