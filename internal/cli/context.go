package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
)

// RunContext implements `squadai context` — dumps the merged config as an LLM-ready
// context block, useful for pasting into a conversation or piping into a tool.
func RunContext(args []string, stdout io.Writer) error {
	format := "prompt"
	adapter := ""

	for _, arg := range args {
		switch {
		case arg == "-h" || arg == "--help":
			fmt.Fprintln(stdout, "Usage: squadai context [--format=prompt|json|mcp] [--adapter=<id>]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Dump the merged SquadAI configuration as LLM-ready context. Useful for:")
			fmt.Fprintln(stdout, "  - Pasting into a Claude conversation to give it project context")
			fmt.Fprintln(stdout, "  - Piping into scripts that need config metadata")
			fmt.Fprintln(stdout, "  - Feeding into the MCP server (--format=mcp)")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --format=<prompt|json|mcp>   Output format (default: prompt)")
			fmt.Fprintln(stdout, "  --adapter=<id>               Scope output to a specific adapter")
			return nil
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
		case strings.HasPrefix(arg, "--adapter="):
			adapter = strings.TrimPrefix(arg, "--adapter=")
		}
	}

	switch format {
	case "prompt", "json", "mcp":
	default:
		return exitcode.ErrUnknownValue("--format", format, "prompt, json, mcp")
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

	switch format {
	case "json":
		if merged == nil {
			fmt.Fprintln(stdout, `{"error":"no config found"}`)
			return nil
		}
		if adapter != "" {
			cfg, ok := merged.Adapters[adapter]
			if !ok {
				return exitcode.ErrNotFound(fmt.Sprintf("adapter %q", adapter))
			}
			data, _ := json.MarshalIndent(map[string]any{
				"adapter": adapter,
				"config":  cfg,
			}, "", "  ")
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		data, _ := json.MarshalIndent(merged, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil

	case "mcp":
		// MCP format: resource-compatible JSON with type hints for MCP tool servers.
		ctx := buildContextResource(merged, projectDir, adapter)
		data, _ := json.MarshalIndent(ctx, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil

	default: // "prompt"
		return printContextPrompt(stdout, merged, projectDir, adapter)
	}
}

// contextResource is the MCP-compatible context payload.
type contextResource struct {
	Type       string         `json:"type"`
	URI        string         `json:"uri"`
	ProjectDir string         `json:"project_dir"`
	Mode       string         `json:"mode,omitempty"`
	Adapters   []string       `json:"adapters,omitempty"`
	Summary    map[string]any `json:"summary,omitempty"`
}

func buildContextResource(merged *domain.MergedConfig, projectDir, adapterFilter string) contextResource {
	res := contextResource{
		Type:       "squadai/config",
		URI:        "squadai://context",
		ProjectDir: projectDir,
	}
	if merged == nil {
		return res
	}
	res.Mode = string(merged.Mode)
	for id, cfg := range merged.Adapters {
		if cfg.Enabled && (adapterFilter == "" || adapterFilter == id) {
			res.Adapters = append(res.Adapters, id)
		}
	}
	summary := map[string]any{
		"methodology": string(merged.Methodology),
		"model_tier":  string(merged.ModelTier),
		"agent_count": len(merged.Agents),
		"mcp_count":   len(merged.MCP),
		"skill_count": len(merged.Skills),
	}
	res.Summary = summary
	return res
}

func printContextPrompt(w io.Writer, merged *domain.MergedConfig, projectDir, adapterFilter string) error {
	fmt.Fprintln(w, "# SquadAI Project Context")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Project directory: %s\n", projectDir)

	if merged == nil {
		fmt.Fprintln(w, "\nNo SquadAI configuration found. Run 'squadai init' to create one.")
		return nil
	}

	fmt.Fprintf(w, "Mode: %s\n", merged.Mode)
	fmt.Fprintf(w, "Methodology: %s\n", merged.Methodology)
	fmt.Fprintf(w, "Model tier: %s\n", merged.ModelTier)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "## Enabled Adapters")
	for id, cfg := range merged.Adapters {
		if cfg.Enabled && (adapterFilter == "" || adapterFilter == id) {
			fmt.Fprintf(w, "  - %s\n", id)
		}
	}
	fmt.Fprintln(w)

	if len(merged.Agents) > 0 {
		fmt.Fprintln(w, "## Agents")
		for name, def := range merged.Agents {
			fmt.Fprintf(w, "  - %s: %s\n", name, def.Description)
		}
		fmt.Fprintln(w)
	}

	if len(merged.MCP) > 0 {
		fmt.Fprintln(w, "## MCP Servers")
		for name := range merged.MCP {
			fmt.Fprintf(w, "  - %s\n", name)
		}
		fmt.Fprintln(w)
	}

	if len(merged.Skills) > 0 {
		fmt.Fprintln(w, "## Skills")
		for name, def := range merged.Skills {
			fmt.Fprintf(w, "  - %s: %s\n", name, def.Description)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "---")
	fmt.Fprintln(w, "Generated by `squadai context`. Run `squadai verify` to check compliance.")
	return nil
}
