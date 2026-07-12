package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/governance"
	"github.com/PedroMosquera/squadai/internal/marketplace"
)

// RunPluginsSync fetches the remote plugin registry and caches it locally at
// .squadai/plugins-registry.json.
func RunPluginsSync(args []string, stdout, stderr io.Writer) error {
	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		}
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}

	if !jsonOut {
		fmt.Fprintln(stdout, "syncing plugin registry from github.com/wshobson/agents …")
	}
	reg, err := marketplace.Sync(projectDir)
	if err != nil {
		if jsonOut {
			writeJSONResult(stdout, false, map[string]any{"error": err.Error()})
		}
		return exitcode.Wrap(exitcode.Network, "E-701", "sync registry failed",
			"Check your internet connection and try again.", err)
	}

	if jsonOut {
		writeJSONResult(stdout, true, map[string]any{
			"plugins":       len(reg.Plugins),
			"registry_path": projectDir + "/.squadai/plugins-registry.json",
			"fetched_at":    reg.FetchedAt,
		})
		return nil
	}
	fmt.Fprintf(stdout, "synced %d plugins → %s/.squadai/plugins-registry.json\n",
		len(reg.Plugins), projectDir)
	return nil
}

// RunPluginsList reads the local registry and prints a table of available plugins.
func RunPluginsList(args []string, stdout io.Writer) error {
	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}

	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		}
	}

	reg, err := marketplace.Load(projectDir)
	if err != nil {
		return err
	}

	if jsonOut {
		data, _ := json.MarshalIndent(reg.Plugins, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	// Load project config to mark installed plugins.
	installed := make(map[string]string)
	if proj, loadErr := config.LoadProject(projectDir); loadErr == nil {
		for name, ver := range proj.Marketplace.Plugins {
			installed[name] = ver
		}
	}

	names := reg.SortedNames()
	fmt.Fprintf(stdout, "%-40s %-10s %s\n", "PLUGIN", "VERSION", "DESCRIPTION")
	fmt.Fprintf(stdout, "%-40s %-10s %s\n",
		strings.Repeat("-", 40), strings.Repeat("-", 10), strings.Repeat("-", 40))
	for _, name := range names {
		p := reg.Plugins[name]
		marker := "  "
		if _, ok := installed[name]; ok {
			marker = "✓ "
		}
		desc := p.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		fmt.Fprintf(stdout, "%s%-38s %-10s %s\n", marker, name, p.Version, desc)
	}
	fmt.Fprintf(stdout, "\n(%d plugins, ✓ = installed in this project)\n", len(names))
	return nil
}

// RunPluginsAdd downloads and installs a plugin into the project's agent/skill/command
// directories for all enabled adapters, then records the version in project.json.
func RunPluginsAdd(args []string, stdout, stderr io.Writer) error {
	jsonOut := false
	var pluginName string
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		} else if pluginName == "" {
			pluginName = a
		}
	}
	if pluginName == "" {
		return fmt.Errorf("usage: squadai plugins add <plugin-name>")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}

	reg, err := marketplace.Load(projectDir)
	if err != nil {
		if errors.Is(err, marketplace.ErrRegistryNotFound) {
			if !jsonOut {
				fmt.Fprintln(stdout, "registry not found — syncing first…")
			}
			reg, err = marketplace.Sync(projectDir)
			if err != nil {
				return exitcode.Wrap(exitcode.Network, "E-701",
					"auto-sync registry failed", "Check your internet connection.", err)
			}
		} else {
			return err
		}
	}

	if !jsonOut {
		fmt.Fprintf(stdout, "installing plugin %q …\n", pluginName)
	}
	plugin, err := marketplace.Install(projectDir, pluginName, reg)
	if err != nil {
		if jsonOut {
			writeJSONResult(stdout, false, map[string]any{"error": err.Error(), "plugin": pluginName})
		}
		return exitcode.Wrap(exitcode.NotFound, "E-501", "install plugin failed", "", err)
	}

	proj, loadErr := config.LoadProject(projectDir)
	if loadErr != nil {
		return fmt.Errorf("load project config: %w", loadErr)
	}
	if proj.Marketplace.Plugins == nil {
		proj.Marketplace.Plugins = make(map[string]string)
	}
	proj.Marketplace.Plugins[pluginName] = plugin.Version
	if err := config.SaveProject(projectDir, proj); err != nil {
		return fmt.Errorf("update project.json: %w", err)
	}

	auditPluginEvent(projectDir, governance.KindPluginInstall, pluginName,
		fmt.Sprintf("marketplace plugin %q v%s installed", pluginName, plugin.Version))

	if jsonOut {
		writeJSONResult(stdout, true, map[string]any{
			"plugin":   pluginName,
			"version":  plugin.Version,
			"agents":   plugin.Agents,
			"skills":   plugin.Skills,
			"commands": plugin.Commands,
		})
		return nil
	}
	fmt.Fprintf(stdout, "installed %q v%s\n", pluginName, plugin.Version)
	if len(plugin.Agents) > 0 {
		fmt.Fprintf(stdout, "  agents   : %s\n", strings.Join(plugin.Agents, ", "))
	}
	if len(plugin.Skills) > 0 {
		fmt.Fprintf(stdout, "  skills   : %s\n", strings.Join(plugin.Skills, ", "))
	}
	if len(plugin.Commands) > 0 {
		fmt.Fprintf(stdout, "  commands : %s\n", strings.Join(plugin.Commands, ", "))
	}
	return nil
}

// RunPluginsRemove removes an installed plugin's files and updates project.json.
func RunPluginsRemove(args []string, stdout io.Writer) error {
	jsonOut := false
	var pluginName string
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		} else if pluginName == "" {
			pluginName = a
		}
	}
	if pluginName == "" {
		return fmt.Errorf("usage: squadai plugins remove <plugin-name>")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}

	reg, err := marketplace.Load(projectDir)
	if err != nil {
		return err
	}

	if !jsonOut {
		fmt.Fprintf(stdout, "removing plugin %q …\n", pluginName)
	}
	if err := marketplace.Remove(projectDir, pluginName, reg); err != nil {
		if jsonOut {
			writeJSONResult(stdout, false, map[string]any{"error": err.Error(), "plugin": pluginName})
		}
		return exitcode.Wrap(exitcode.NotFound, "E-501", "remove plugin failed", "", err)
	}

	proj, loadErr := config.LoadProject(projectDir)
	if loadErr != nil {
		return fmt.Errorf("load project config: %w", loadErr)
	}
	delete(proj.Marketplace.Plugins, pluginName)
	if err := config.SaveProject(projectDir, proj); err != nil {
		return fmt.Errorf("update project.json: %w", err)
	}

	auditPluginEvent(projectDir, governance.KindPluginRemove, pluginName,
		fmt.Sprintf("marketplace plugin %q removed", pluginName))

	if jsonOut {
		writeJSONResult(stdout, true, map[string]any{"plugin": pluginName, "removed": true})
		return nil
	}
	fmt.Fprintf(stdout, "removed %q\n", pluginName)
	return nil
}

// writeJSONResult writes a uniform JSON envelope to stdout.
// success=true → {"success":true, ...extra fields merged at top level}
// success=false → {"success":false, ...extra fields merged at top level}
func writeJSONResult(w io.Writer, success bool, extra map[string]any) {
	out := make(map[string]any, len(extra)+1)
	out["success"] = success
	for k, v := range extra {
		out[k] = v
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Fprintln(w, string(data))
}

// ─── Schema export ────────────────────────────────────────────────────────────

// ─── Explain ─────────────────────────────────────────────────────────────────
