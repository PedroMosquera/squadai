package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
)

// RunProfile shows the configured context profiles or persists a new default.
//
//	squadai profile          — show the active profile and list all profiles
//	squadai profile <name>   — persist context.default_profile to project.json
func RunProfile(args []string, stdout io.Writer) error {
	jsonOut := false
	name := ""
	for _, arg := range args {
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "-h" || arg == "--help":
			printProfileHelp(stdout)
			return nil
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown flag %q for profile", arg)
		default:
			if name != "" {
				return fmt.Errorf("profile takes at most one name argument")
			}
			name = arg
		}
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

	if name == "" {
		return printProfiles(stdout, merged, jsonOut)
	}
	return setDefaultProfile(stdout, merged, projectDir, name)
}

func printProfileHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Usage: squadai profile [<name>] [--json]")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Show or switch the active context profile. Profiles shape what `apply`")
	fmt.Fprintln(stdout, "installs: which MCP servers stay enabled, which skills are installed, the")
	fmt.Fprintln(stdout, "memory protocol scope, and the session token cap used for budget fitting.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Without arguments, prints the active profile and lists all configured")
	fmt.Fprintln(stdout, "profiles. With a name, validates it and persists context.default_profile")
	fmt.Fprintln(stdout, "to .squadai/project.json — run `squadai apply` afterwards to take effect.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Note: a profile's include/exclude globs and adapter_overrides are recorded")
	fmt.Fprintln(stdout, "in config but not enforced yet.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Flags:")
	fmt.Fprintln(stdout, "  --json    Output profiles as JSON")
}

// profileJSON is the machine-readable output of `squadai profile --json`.
type profileJSON struct {
	Active   string                           `json:"active,omitempty"`
	Profiles map[string]domain.ContextProfile `json:"profiles"`
}

func printProfiles(stdout io.Writer, merged *domain.MergedConfig, jsonOut bool) error {
	active := merged.Context.DefaultProfile
	if _, ok := merged.Context.Profiles[active]; !ok {
		active = ""
	}

	if jsonOut {
		out := profileJSON{Active: active, Profiles: merged.Context.Profiles}
		if out.Profiles == nil {
			out.Profiles = map[string]domain.ContextProfile{}
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal profiles: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	if len(merged.Context.Profiles) == 0 {
		fmt.Fprintln(stdout, "No context profiles configured.")
		fmt.Fprintln(stdout, "Add them under \"context.profiles\" in .squadai/project.json.")
		return nil
	}

	if active != "" {
		fmt.Fprintf(stdout, "Active profile: %s\n\n", active)
	} else {
		fmt.Fprintf(stdout, "Active profile: (none)\n\n")
	}

	fmt.Fprintf(stdout, "  %-10s %-10s %-8s %-24s %s\n", "Name", "Memory", "Cap", "MCP servers", "Skill scopes")
	for _, n := range profileNames(merged) {
		p := merged.Context.Profiles[n]
		marker := " "
		if n == active {
			marker = "*"
		}
		mem := p.MemoryScope
		if mem == "" {
			mem = "project"
		}
		cap := "-"
		if p.MaxApproxTokens > 0 {
			cap = fmt.Sprintf("%d", p.MaxApproxTokens)
		}
		mcp := "(all)"
		if p.MCPServers != nil {
			mcp = strings.Join(p.MCPServers, ",")
			if mcp == "" {
				mcp = "(none)"
			}
		}
		scopes := "(all)"
		if p.SkillScopes != nil {
			scopes = strings.Join(p.SkillScopes, ",")
			if scopes == "" {
				scopes = "(none)"
			}
		}
		fmt.Fprintf(stdout, "%s %-10s %-10s %-8s %-24s %s\n", marker, n, mem, cap, mcp, scopes)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Switch with 'squadai profile <name>', or use 'squadai apply --profile=<name>' for one run.")
	return nil
}

func setDefaultProfile(stdout io.Writer, merged *domain.MergedConfig, projectDir, name string) error {
	if _, ok := merged.Context.Profiles[name]; !ok {
		available := strings.Join(profileNames(merged), ", ")
		if available == "" {
			available = "none configured"
		}
		return fmt.Errorf("unknown context profile %q (available: %s)", name, available)
	}

	project, err := config.LoadProject(projectDir)
	if err != nil {
		return exitcode.ErrPrecondition(
			"no project.json found in current directory",
			"Run 'squadai init' to create one before setting a profile.")
	}
	project.Context.DefaultProfile = name
	if err := config.SaveProject(projectDir, project); err != nil {
		return fmt.Errorf("save project config: %w", err)
	}

	fmt.Fprintf(stdout, "Default context profile set to %q.\n", name)
	fmt.Fprintln(stdout, "Run 'squadai apply' to take effect.")
	return nil
}
