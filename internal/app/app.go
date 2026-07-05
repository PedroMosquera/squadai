package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/PedroMosquera/squadai/internal/cli"
	"github.com/PedroMosquera/squadai/internal/doctor"
	"github.com/PedroMosquera/squadai/internal/mcpserver"
	"github.com/PedroMosquera/squadai/internal/state"
	"github.com/PedroMosquera/squadai/internal/tui"
	"github.com/PedroMosquera/squadai/internal/update"
)

// Version is set from main via ldflags at build time.
var Version = "dev"

// init wires the review-prompt hooks from the tui package into the cli package.
// This breaks what would otherwise be a cli→tui→cli import cycle: the cli
// package declares the hook variables and calls them; the app package — which
// imports both — injects the concrete implementation at process start.
func init() {
	cli.ReviewPromptHook = tui.RunReviewPrompt
	cli.IsTTYHook = tui.IsTTY
	cli.ReviewScreenWired = true
	doctor.ReviewScreenWiredHook = func() bool { return cli.ReviewScreenWired }
}

// Run is the top-level entry point. It parses args and dispatches to subcommands.
// When no args are given, it launches the interactive TUI.
func Run(args []string, stdout, stderr io.Writer) error {
	// Phase 1: apply any pending update before doing anything else.
	// (Non-fatal — apply writes its own warnings to stderr.)
	if err := update.Apply(stderr); err != nil {
		fmt.Fprintf(stderr, "squadai: warning: %v\n", err)
	}

	if len(args) == 0 {
		// Start background update check, then launch TUI.
		cancelBg := maybeStartBackgroundCheck(stderr)
		defer cancelBg()
		return tui.Run(Version)
	}

	// Propagate version to cli package so RunDoctor can include it in JSON output.
	cli.Version = Version

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "SquadAI %s\n", Version)
		return nil

	case "help", "--help", "-h":
		for _, a := range args[1:] {
			if a == "--json" {
				return printUsageJSON(stdout)
			}
		}
		printUsage(stdout)
		return nil

	case "schema":
		return cli.RunSchema(args[1:], stdout)

	case "context":
		return cli.RunContext(args[1:], stdout)

	case "mcp-server":
		return mcpserver.RunMCPServer(args[1:], stdout, stderr, Version, buildMCPRunners(stdout, stderr))

	case "update":
		return cli.RunUpdate(args[1:], stdout, stderr)

	case "init":
		return cli.RunInit(args[1:], stdout)

	case "validate-policy":
		return cli.RunValidatePolicy(args[1:], stdout)

	case "plan":
		return cli.RunPlan(args[1:], stdout)

	case "diff":
		return cli.RunDiff(args[1:], stdout)

	case "apply":
		cancelBg := maybeStartBackgroundCheck(stderr)
		defer cancelBg()
		return cli.RunApply(args[1:], stdout)

	case "verify":
		return cli.RunVerify(args[1:], stdout)

	case "status":
		return cli.RunStatus(args[1:], stdout)

	case "backup":
		if len(args) < 2 || args[1] == "--help" || args[1] == "-h" || args[1] == "help" {
			printBackupUsage(stdout)
			return nil
		}
		switch args[1] {
		case "create":
			return cli.RunBackupCreate(args[2:], stdout)
		case "list":
			return cli.RunBackupList(args[2:], stdout)
		case "delete":
			return cli.RunBackupDelete(args[2:], stdout)
		case "prune":
			return cli.RunBackupPrune(args[2:], stdout)
		default:
			return fmt.Errorf("unknown backup subcommand %q", args[1])
		}

	case "restore":
		return cli.RunRestore(args[1:], stdout)

	case "remove":
		return cli.RunRemove(args[1:], stdout)

	case "doctor":
		return cli.RunDoctor(args[1:], stdout)

	case "watch":
		return cli.RunWatch(args[1:], stdout, stderr)

	case "audit":
		return cli.RunAudit(args[1:], stdout)

	case "install-hooks":
		return cli.RunInstallHooks(args[1:], stdout)

	case "install-commands":
		return cli.RunInstallCommands(args[1:], stdout)

	case "explain":
		return cli.RunExplain(args[1:], stdout)

	case "plugins":
		if len(args) < 2 || args[1] == "--help" || args[1] == "-h" || args[1] == "help" {
			printPluginsUsage(stdout)
			return nil
		}
		switch args[1] {
		case "sync":
			return cli.RunPluginsSync(args[2:], stdout, stderr)
		case "list":
			return cli.RunPluginsList(args[2:], stdout)
		case "add":
			return cli.RunPluginsAdd(args[2:], stdout, stderr)
		case "add-git":
			return cli.RunPluginsAddGit(args[2:], stdout, stderr)
		case "remove":
			return cli.RunPluginsRemove(args[2:], stdout)
		default:
			return fmt.Errorf("unknown plugins subcommand %q", args[1])
		}

	case "models":
		if len(args) < 2 || args[1] == "--help" || args[1] == "-h" || args[1] == "help" {
			printModelsUsage(stdout)
			return nil
		}
		switch args[1] {
		case "list":
			return cli.RunModelsList(args[2:], stdout)
		case "check":
			return cli.RunModelsCheck(args[2:], stdout)
		case "update":
			return cli.RunModelsUpdate(args[2:], stdout, os.Stdin)
		default:
			return fmt.Errorf("unknown models subcommand %q", args[1])
		}

	case "_hook":
		return cli.RunHookCommand(args[1:])

	case "memory":
		return cli.RunMemoryCommand(args[1:])

	case "token-budget":
		return cli.RunTokenBudget(args[1:], stdout)

	case "token-usage":
		return cli.RunTokenUsage(args[1:], stdout)

	default:
		return fmt.Errorf("unknown command %q — run 'squadai help' for available commands", args[0])
	}
}

// maybeStartBackgroundCheck spawns a goroutine to check for updates if
// update checks are enabled and 24 hours have elapsed since the last check.
// It returns a cancel function that should be deferred by the caller.
func maybeStartBackgroundCheck(stderr io.Writer) context.CancelFunc {
	if update.IsDevBuild(Version) {
		return func() {}
	}

	statePath, err := state.DefaultPath()
	if err != nil {
		return func() {}
	}

	s, err := state.Load(statePath)
	if err != nil {
		return func() {}
	}

	if !s.UpdateChecksEnabled {
		return func() {}
	}

	if time.Since(s.LastUpdateCheck) < 24*time.Hour {
		return func() {}
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := update.Run(ctx, Version, stderr); err != nil {
			// Expected non-error conditions are silent; all other errors
			// are silently dropped in background mode.
			_ = err
		}

		// Persist the check timestamp regardless of result.
		s2, loadErr := state.Load(statePath)
		if loadErr != nil {
			return
		}
		s2.LastUpdateCheck = time.Now()
		_ = state.Save(statePath, s2) // best effort
	}()

	return cancel
}

func printPluginsUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: squadai plugins <subcommand> [flags]

Subcommands:
  sync             Fetch the plugin registry from github.com/wshobson/agents
  list             List available plugins (--json for machine output)
  add <name>       Download and install a marketplace plugin; updates project.json
  add-git <url>    Clone a git-based plugin (git:github.com/user/repo) into .squadai/plugins/
  remove <name>    Remove an installed plugin's files; updates project.json

`)
}

func printModelsUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: squadai models <subcommand> [flags]

Subcommands:
  list        List the effective model catalog (offline; --json, --adapter=<id>)
  check       Fetch the published catalog and report staleness/differences
  update      Fetch, show the diff, confirm, then write ~/.squadai/models.json
              (--yes to skip the prompt, --project for .squadai/models.json)

The catalog drives model pricing, tokenizer encodings, and per-adapter tier
defaults. Updates never happen silently: 'update' always shows the diff and
asks for confirmation unless --yes is passed.
`)
}

func printBackupUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: squadai backup <subcommand> [flags]

Subcommands:
  create        Create a backup of all managed files
  list          List all backup snapshots
  delete <id>   Delete a specific backup snapshot
  prune         Remove old backups, keep N most recent (default 10)

Flags:
  --json        Output results in JSON format
`)
}

// ─── MCP tool runners ─────────────────────────────────────────────────────────

// buildMCPRunners constructs the map of MCP tool name → SquadAI CLI handler.
// Each runner mirrors the corresponding CLI command, adapting the args format
// from MCP (flag map → cli args) to the existing RunXxx functions.
func buildMCPRunners(stdout, stderr io.Writer) map[string]mcpserver.ToolRunner {
	return map[string]mcpserver.ToolRunner{
		"plan": func(args []string, w io.Writer) error {
			return cli.RunPlan(args, w)
		},
		"apply": func(args []string, w io.Writer) error {
			return cli.RunApply(args, w)
		},
		"verify": func(args []string, w io.Writer) error {
			return cli.RunVerify(args, w)
		},
		"status": func(args []string, w io.Writer) error {
			return cli.RunStatus(args, w)
		},
		"context": func(args []string, w io.Writer) error {
			return cli.RunContext(args, w)
		},
		"init": func(args []string, w io.Writer) error {
			return cli.RunInit(args, w)
		},
		"validate_policy": func(args []string, w io.Writer) error {
			return cli.RunValidatePolicy(args, w)
		},
		"schema_export": func(args []string, w io.Writer) error {
			return cli.RunSchemaExport(args, w)
		},
		"doctor": func(args []string, w io.Writer) error {
			return cli.RunDoctor(args, w)
		},
		"plugins_sync": func(args []string, w io.Writer) error {
			return cli.RunPluginsSync(args, w, stderr)
		},
		"plugins_list": func(args []string, w io.Writer) error {
			return cli.RunPluginsList(args, w)
		},
		"install_hooks": func(args []string, w io.Writer) error {
			return cli.RunInstallHooks(args, w)
		},
		"squad_init_status": func(args []string, w io.Writer) error {
			return cli.RunSquadInitStatus(args, w)
		},
		"memory_search": func(args []string, w io.Writer) error {
			return cli.RunMemorySearchTool(args, w)
		},
		"memory_add": func(args []string, w io.Writer) error {
			return cli.RunMemoryAddTool(args, w)
		},
	}
}

// ─── machine-readable command registry ───────────────────────────────────────

// cmdFlag is a single flag entry in the machine-readable command registry.
type cmdFlag struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type,omitempty"`
	Default     string `json:"default,omitempty"`
}

// cmdEntry describes a single command or subcommand.
type cmdEntry struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Flags       []cmdFlag   `json:"flags,omitempty"`
	Subcommands []cmdEntry  `json:"subcommands,omitempty"`
}

// helpOutput is the top-level envelope for `squadai help --json`.
type helpOutput struct {
	Version  string     `json:"version"`
	Commands []cmdEntry `json:"commands"`
}

func buildCommandRegistry() helpOutput {
	return helpOutput{
		Version: Version,
		Commands: []cmdEntry{
			{
				Name:        "init",
				Description: "Initialize project config (.squadai/project.json) and optional policy template.",
				Flags: []cmdFlag{
					{Name: "--methodology", Type: "string", Description: "Set development methodology (tdd, sdd, conventional)"},
					{Name: "--mcp", Type: "csv", Description: "Comma-separated MCP server IDs to enable"},
					{Name: "--plugins", Type: "csv", Description: "Comma-separated plugin IDs to install"},
					{Name: "--model-tier", Type: "string", Description: "Model tier (balanced, performance, starter, manual)", Default: "balanced"},
					{Name: "--agents", Type: "csv", Description: "Comma-separated adapter IDs to enable"},
					{Name: "--preset", Type: "string", Description: "Setup preset (full-squad, lean, custom)"},
					{Name: "--with-policy", Type: "bool", Description: "Also generate a policy.json template"},
					{Name: "--force", Type: "bool", Description: "Overwrite existing config without merging"},
					{Name: "--merge", Type: "bool", Description: "Merge with existing config instead of replacing"},
					{Name: "--global", Type: "bool", Description: "Write to home directory instead of current project"},
					{Name: "--json", Type: "bool", Description: "Output result as JSON"},
				},
			},
			{
				Name:        "validate-policy",
				Description: "Validate policy.json schema, lock/required consistency.",
				Flags: []cmdFlag{
					{Name: "--json", Type: "bool", Description: "Output validation result as JSON"},
				},
			},
			{
				Name:        "plan",
				Description: "Compute the action plan without writing any files.",
				Flags: []cmdFlag{
					{Name: "--json", Type: "bool", Description: "Output plan as JSON"},
				},
			},
			{
				Name:        "diff",
				Description: "Show what apply would change as unified diffs.",
			},
			{
				Name:        "apply",
				Description: "Execute plan with automatic backup and rollback safety.",
				Flags: []cmdFlag{
					{Name: "--dry-run", Type: "bool", Description: "Preview changes without writing files"},
					{Name: "--force", Type: "bool", Description: "Apply even without project.json"},
					{Name: "--json", Type: "bool", Description: "Output apply report as JSON"},
					{Name: "--verbose", Type: "bool", Description: "Stream step events as they execute"},
					{Name: "--no-brand", Type: "bool", Description: "Skip brand banner component for this apply"},
					{Name: "--max-tokens", Type: "int", Description: "Budget cap: fit components within N tokens"},
					{Name: "--fit-model", Type: "string", Description: "Model name for budget fitting (e.g. claude-sonnet-4-6)"},
				},
			},
			{
				Name:        "verify",
				Description: "Print compliance and health report.",
				Flags: []cmdFlag{
					{Name: "--strict", Type: "bool", Description: "Also fail on drift since last apply"},
					{Name: "--json", Type: "bool", Description: "Output verify report as JSON"},
				},
			},
			{
				Name:        "status",
				Description: "Show project configuration summary.",
				Flags: []cmdFlag{
					{Name: "--json", Type: "bool", Description: "Output status as JSON"},
				},
			},
			{
				Name:        "doctor",
				Description: "Run pre-flight diagnostics (environment, agents, config, MCP, filesystem, drift).",
				Flags: []cmdFlag{
					{Name: "--fix", Type: "bool", Description: "Attempt to auto-fix detected issues"},
					{Name: "--json", Type: "bool", Description: "Output diagnostics as JSON"},
				},
			},
			{
				Name:        "watch",
				Description: "Monitor managed files for drift and stream events to stdout.",
				Flags: []cmdFlag{
					{Name: "--daemon", Type: "bool", Description: "Run in background (detached mode)"},
				},
			},
			{
				Name:        "audit",
				Description: "Render the governance audit log (.squadai/audit.log).",
				Flags: []cmdFlag{
					{Name: "--json", Type: "bool", Description: "Output audit events as JSON"},
					{Name: "--since", Type: "duration", Description: "Filter events newer than duration (e.g. 24h)"},
					{Name: "--filter", Type: "string", Description: "Filter by event kind (e.g. drift)"},
				},
			},
			{
				Name:        "install-hooks",
				Description: "Install Git hooks (pre-commit, post-merge, post-checkout) for squadai.",
				Flags: []cmdFlag{
					{Name: "--json", Type: "bool", Description: "Output result as JSON"},
				},
			},
			{
				Name:        "plugins",
				Description: "Manage plugins from the community marketplace.",
				Subcommands: []cmdEntry{
					{Name: "sync", Description: "Fetch the plugin registry from github.com/wshobson/agents.", Flags: []cmdFlag{{Name: "--json", Type: "bool", Description: "Output result as JSON"}}},
					{Name: "list", Description: "List available plugins (✓ = installed in this project).", Flags: []cmdFlag{{Name: "--json", Type: "bool", Description: "Output list as JSON"}}},
					{Name: "add", Description: "Download and install a marketplace plugin.", Flags: []cmdFlag{{Name: "--json", Type: "bool", Description: "Output result as JSON"}}},
					{Name: "add-git", Description: "Clone a git-based plugin (git:github.com/user/repo) into .squadai/plugins/.", Flags: []cmdFlag{{Name: "--json", Type: "bool", Description: "Output result as JSON"}}},
					{Name: "remove", Description: "Remove an installed plugin's files.", Flags: []cmdFlag{{Name: "--json", Type: "bool", Description: "Output result as JSON"}}},
				},
			},
			{
				Name:        "backup",
				Description: "Manage backup snapshots of managed files.",
				Subcommands: []cmdEntry{
					{Name: "create", Description: "Snapshot all managed files.", Flags: []cmdFlag{{Name: "--json", Type: "bool", Description: "Output backup manifest as JSON"}}},
					{Name: "list", Description: "List all backup snapshots.", Flags: []cmdFlag{{Name: "--json", Type: "bool", Description: "Output list as JSON"}}},
					{Name: "delete", Description: "Delete a specific backup snapshot by ID.", Flags: []cmdFlag{{Name: "--json", Type: "bool", Description: "Output result as JSON"}}},
					{Name: "prune", Description: "Remove old backups, keeping N most recent.", Flags: []cmdFlag{
						{Name: "--keep", Type: "int", Description: "Number of backups to keep", Default: "10"},
						{Name: "--json", Type: "bool", Description: "Output result as JSON"},
					}},
				},
			},
			{
				Name:        "models",
				Description: "Inspect and refresh the unified model catalog (pricing, encodings, tier defaults).",
				Subcommands: []cmdEntry{
					{Name: "list", Description: "List the effective model catalog (offline).", Flags: []cmdFlag{
						{Name: "--json", Type: "bool", Description: "Output list as JSON"},
						{Name: "--adapter", Type: "string", Description: "Show tier mapping and models for one adapter"},
					}},
					{Name: "check", Description: "Fetch the published catalog and report staleness/differences.", Flags: []cmdFlag{
						{Name: "--json", Type: "bool", Description: "Output report as JSON"},
					}},
					{Name: "update", Description: "Fetch the published catalog, show the diff, and write the override after confirmation.", Flags: []cmdFlag{
						{Name: "--yes", Type: "bool", Description: "Skip the confirmation prompt"},
						{Name: "--project", Type: "bool", Description: "Write .squadai/models.json instead of ~/.squadai/models.json"},
					}},
				},
			},
			{
				Name:        "restore",
				Description: "Restore managed files from a backup snapshot.",
				Flags: []cmdFlag{
					{Name: "--json", Type: "bool", Description: "Output result as JSON"},
				},
			},
			{
				Name:        "remove",
				Description: "Remove all managed files from the project.",
				Flags: []cmdFlag{
					{Name: "--force", Type: "bool", Description: "Confirm removal without interactive prompt"},
				},
			},
			{
				Name:        "schema",
				Description: "Export JSON Schema for SquadAI config files.",
				Subcommands: []cmdEntry{
					{Name: "export", Description: "Write JSON Schema files for project.json and policy.json to stdout or a directory.", Flags: []cmdFlag{
						{Name: "--out", Type: "string", Description: "Output directory (default: stdout)"},
						{Name: "--format", Type: "string", Description: "Output format: project, policy, or all (default: all)"},
					}},
				},
			},
			{
				Name:        "context",
				Description: "Dump SquadAI configuration as LLM-ready context.",
				Flags: []cmdFlag{
					{Name: "--format", Type: "string", Description: "Output format: prompt, json, or mcp (default: prompt)"},
					{Name: "--adapter", Type: "string", Description: "Scope output to a specific adapter"},
				},
			},
			{
				Name:        "mcp-server",
				Description: "Start SquadAI as an MCP stdio server. Exposes plan, apply, verify, status, context, init, doctor, plugins, and more as MCP tools callable by Claude Code.",
			},
			{
				Name:        "token-budget",
				Description: "Estimate per-session token cost of the current squadai install.",
				Flags: []cmdFlag{
					{Name: "--json", Type: "bool", Description: "Output as JSON"},
					{Name: "--model", Type: "string", Description: "Model name for tokenizer (e.g. claude-sonnet-4-6, gpt-5-mini)"},
				},
			},
			{
				Name:        "token-usage",
				Description: "Aggregate real token usage from agent session transcripts.",
				Flags: []cmdFlag{
					{Name: "--since", Type: "string", Description: "Time window: 7d, 30d, or all (default: 7d)"},
					{Name: "--json", Type: "bool", Description: "Output as JSON"},
				},
			},
			{
				Name:        "update",
				Description: "Check for a newer version of SquadAI and optionally download it.",
				Flags: []cmdFlag{
					{Name: "--check", Type: "bool", Description: "Only check, do not download"},
				},
			},
			{
				Name:        "version",
				Description: "Print the SquadAI version string.",
			},
			{
				Name:        "help",
				Description: "Show the help text for all commands.",
				Flags: []cmdFlag{
					{Name: "--json", Type: "bool", Description: "Output machine-readable command registry as JSON"},
				},
			},
		},
	}
}

func printUsageJSON(w io.Writer) error {
	reg := buildCommandRegistry()
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal help registry: %w", err)
	}
	fmt.Fprintln(w, string(data))
	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `SquadAI %s — Team-consistent AI setup with safe local customization.

Usage:
  squadai <command> [flags]

Commands:
  init               Initialize project config and optional policy template
  validate-policy    Validate policy schema and lock/required consistency
  plan               Compute action plan (use --dry-run to preview)
  diff               Show what apply would change as unified diffs
  apply              Execute plan with backup and rollback safety
  verify             Print compliance and health report (--strict adds drift check)
  status             Show project configuration summary
  doctor             Run pre-flight diagnostics (environment, agents, config, MCP, filesystem, drift)
  token-budget       Estimate per-session token cost of the current install (--model for BPE, --json for JSON)
  token-usage        Aggregate real token usage from agent session transcripts (--since=7d, --json)
  watch              Monitor managed files for drift, stream events to stdout
  audit              Render the governance audit log (.squadai/audit.log)
  install-hooks      Install Git hooks (pre-commit, post-merge, post-checkout) for squadai
  install-commands   Install SquadAI slash commands + squadai-manager agent to .claude/
  explain <topic>    Explain a SquadAI concept (config, policy, adapters, error-codes, ...)
  plugins sync          Fetch plugin registry from github.com/wshobson/agents
  plugins list          List available plugins (✓ = installed in this project)
  plugins add <name>    Download and install a marketplace plugin
  plugins add-git <url> Clone a git-based plugin (git:github.com/user/repo) into .squadai/plugins/
  plugins remove <name> Remove an installed plugin's files
  backup create      Snapshot managed files
  backup list        List available backups
  backup delete <id> Delete a backup snapshot
  backup prune       Remove old backups (keep N most recent)
  models list        List the effective model catalog (offline; --json, --adapter=<id>)
  models check       Fetch the published catalog and report staleness/differences
  models update      Refresh ~/.squadai/models.json after showing a diff and confirming
  restore <id>       Restore from a backup
  remove             Remove all managed files (use --force to confirm)
  schema export      Export JSON Schema for project.json / policy.json (VS Code validation)
  context            Dump config as LLM-ready context (--format prompt|json|mcp)
  mcp-server         Start SquadAI as an MCP stdio server (for Claude Code integration)
  update             Check for updates and download (see 'squadai update --help')
  version            Print version

Flags:
  --dry-run          Preview changes without applying (plan, apply)
  --json             Machine-readable JSON output (all commands)
  --json (help)      Output machine-readable command registry
  -h, --help         Show this help

`, Version)
}
