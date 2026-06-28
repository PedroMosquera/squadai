package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/PedroMosquera/squadai/internal/pluginsdk"
)

func RunPluginsAddGit(args []string, stdout, stderr io.Writer) error {
	jsonOut := false
	var gitURL string

	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		} else if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "Usage: squadai plugins add-git <git-url> [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Clone a git-based plugin into .squadai/plugins/<id>/.")
			fmt.Fprintln(stdout, "URL format: git:github.com/user/repo or git:https://github.com/user/repo.git")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output result as JSON")
			return nil
		} else if gitURL == "" {
			gitURL = a
		}
	}

	if gitURL == "" {
		return fmt.Errorf("usage: squadai plugins add-git <git-url>")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}

	if !jsonOut {
		fmt.Fprintf(stdout, "cloning %s …\n", gitURL)
	}

	result, err := pluginsdk.Install(projectDir, gitURL)
	if err != nil {
		if jsonOut {
			writeJSONResult(stdout, false, map[string]any{"error": err.Error(), "url": gitURL})
		}
		return fmt.Errorf("install git plugin: %w", err)
	}

	if jsonOut {
		out := map[string]any{
			"plugin_id": result.PluginID,
			"path":      result.Path,
		}
		if result.Manifest != nil {
			out["manifest"] = result.Manifest
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "installed plugin %q → %s\n", result.PluginID, result.Path)
	if result.Manifest != nil {
		if result.Manifest.Description != "" {
			fmt.Fprintf(stdout, "  description: %s\n", result.Manifest.Description)
		}
		if len(result.Manifest.Skills) > 0 {
			fmt.Fprintf(stdout, "  skills     : %d\n", len(result.Manifest.Skills))
		}
		if len(result.Manifest.Agents) > 0 {
			fmt.Fprintf(stdout, "  agents     : %d\n", len(result.Manifest.Agents))
		}
	}
	return nil
}
