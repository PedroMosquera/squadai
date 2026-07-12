package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/PedroMosquera/squadai/internal/backup"
	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/pipeline"
	"github.com/PedroMosquera/squadai/internal/planner"
	"github.com/PedroMosquera/squadai/internal/planner/budget"
	"github.com/PedroMosquera/squadai/internal/state"
)

// RunPlan computes and displays the action plan.
func RunPlan(args []string, stdout io.Writer) error {
	dryRun := false
	jsonOut := false
	verbose := false
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--json":
			jsonOut = true
		case "--verbose", "-v":
			verbose = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai plan [--dry-run] [--json] [--verbose]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Compute the set of actions needed to bring all detected agents into the desired")
			fmt.Fprintln(stdout, "state described by .squadai/project.json. Covers all 9 components (memory,")
			fmt.Fprintln(stdout, "rules, settings, MCP, agents, skills, commands, plugins, workflows) across all 5")
			fmt.Fprintln(stdout, "supported agents. No files are written — this is always a read-only preview.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --dry-run  Accepted for consistency with apply; plan is inherently read-only.")
			fmt.Fprintln(stdout, "  --json     Output the planned actions as a JSON array.")
			fmt.Fprintln(stdout, "  --verbose  Show each action individually with its target path.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai plan")
			fmt.Fprintln(stdout, "  squadai plan --json")
			fmt.Fprintln(stdout, "  squadai plan --verbose")
			return nil
		}
	}

	_ = dryRun // plan is inherently dry-run; flag accepted for consistency

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
	applyDefaultProfile(merged)

	adapters := DetectAdapters(homeDir)
	p := planner.New()
	actions, err := p.Plan(merged, adapters, homeDir, projectDir)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	if jsonOut {
		data, err := json.MarshalIndent(actions, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal plan actions: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	// Report violations first.
	if len(merged.Violations) > 0 {
		fmt.Fprintln(stdout, "Policy overrides:")
		for _, v := range merged.Violations {
			fmt.Fprintf(stdout, "  - %s\n", v)
		}
		fmt.Fprintln(stdout)
	}

	fmt.Fprintf(stdout, "Mode: %s\n\n", merged.Mode)

	if len(actions) == 0 {
		fmt.Fprintln(stdout, "No actions needed. Everything is up to date.")
		return nil
	}

	fmt.Fprintf(stdout, "Planned actions (%d):\n", len(actions))
	writePlannedActions(stdout, actions, verbose)

	fmt.Fprintln(stdout, "\nUse 'squadai apply' to execute.")
	return nil
}

// RunApply executes the plan with backup safety and step-level reporting.
func RunApply(args []string, stdout io.Writer) error {
	return runApplyImpl(args, stdout, nil)
}

// RunApplyWithProgress is like RunApply but forwards pipeline events to sink.
// When sink is nil it behaves identically to RunApply.
func RunApplyWithProgress(args []string, stdout io.Writer, sink pipeline.EventSink) error {
	return runApplyImpl(args, stdout, sink)
}

// runApplyImpl is the shared implementation for RunApply and RunApplyWithProgress.
// externalSink is used when non-nil; the --verbose flag creates its own internal sink.
func runApplyImpl(args []string, stdout io.Writer, externalSink pipeline.EventSink) error {
	dryRun := false
	jsonOut := false
	force := false
	verbose := false
	noReview := false
	overwriteUnmanaged := false
	setClaudeDefaultAgent := false
	respectState := true
	noBrand := false
	maxTokens := 0
	fitModel := ""
	flagProfile := ""
	var explicitAgents []string
	var modelOverrides []string // raw "role=tier" pairs from --model flag
	for _, arg := range args {
		switch {
		case arg == "--dry-run":
			dryRun = true
		case arg == "--json":
			jsonOut = true
		case arg == "--force":
			force = true
		case arg == "--verbose":
			verbose = true
		case arg == "--no-review":
			noReview = true
		case arg == "--no-brand":
			noBrand = true
		case strings.HasPrefix(arg, "--max-tokens="):
			if v, err := strconv.Atoi(arg[len("--max-tokens="):]); err == nil {
				maxTokens = v
			}
		case strings.HasPrefix(arg, "--fit-model="):
			fitModel = arg[len("--fit-model="):]
		case strings.HasPrefix(arg, "--profile="):
			flagProfile = arg[len("--profile="):]
		case arg == "--overwrite-unmanaged":
			overwriteUnmanaged = true
		case arg == "--set-claude-default-agent":
			setClaudeDefaultAgent = true
		case arg == "--respect-state" || arg == "--respect-state=true":
			respectState = true
		case arg == "--no-respect-state" || arg == "--respect-state=false":
			respectState = false
		case strings.HasPrefix(arg, "--agent=") || strings.HasPrefix(arg, "-a="):
			val := arg[strings.Index(arg, "=")+1:]
			if val != "" {
				explicitAgents = append(explicitAgents, strings.Split(val, ",")...)
			}
		case strings.HasPrefix(arg, "--model="):
			val := strings.TrimPrefix(arg, "--model=")
			if val != "" {
				for _, pair := range strings.Split(val, ",") {
					pair = strings.TrimSpace(pair)
					if pair != "" {
						modelOverrides = append(modelOverrides, pair)
					}
				}
			}
		case arg == "-h" || arg == "--help":
			fmt.Fprintln(stdout, "Usage: squadai apply [--dry-run] [--json] [--force] [--respect-state] [--verbose] [--model role=tier,...] [--no-brand] [--profile=<name>] [--max-tokens=N] [--fit-model=<name>]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Apply the planned configuration changes to your project. Creates or updates agent")
			fmt.Fprintln(stdout, "config files, MCP server settings, skill files, and team definitions for all")
			fmt.Fprintln(stdout, "detected agents (Claude Code, Cursor, VS Code Copilot, Windsurf, OpenCode).")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "All managed files are backed up automatically before any changes are written.")
			fmt.Fprintln(stdout, "If any step fails, all completed changes are rolled back using the backup.")
			fmt.Fprintln(stdout, "The backup ID is printed so you can restore manually if needed.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --dry-run           Preview the actions that would be executed without writing any files.")
			fmt.Fprintln(stdout, "  --json              Output the execution report as JSON (includes backup ID and step results).")
			fmt.Fprintln(stdout, "  --force             Apply with default config even when no project.json is found.")
			fmt.Fprintln(stdout, "  --verbose           Stream per-step progress to stderr as each action executes.")
			fmt.Fprintln(stdout, "  --no-review         Skip the pre-apply review screen (non-interactive / CI).")
			fmt.Fprintln(stdout, "  --no-brand          Skip the brand banner component for this apply (useful in CI).")
			fmt.Fprintln(stdout, "  --profile=<name>    Context profile for this run (overrides context.default_profile).")
			fmt.Fprintln(stdout, "                      Profiles filter MCP servers and skills, set the memory scope, and")
			fmt.Fprintln(stdout, "                      cap tokens. include/exclude/adapter_overrides are not enforced yet.")
			fmt.Fprintln(stdout, "  --max-tokens=N      Budget cap: fit components within N tokens (drops lowest priority first).")
			fmt.Fprintln(stdout, "                      Defaults to the active profile's max_approx_tokens.")
			fmt.Fprintln(stdout, "  --fit-model=<name>  Model to use for budget fitting (e.g. claude-sonnet-4-6, gpt-5-mini).")
			fmt.Fprintln(stdout, "                      Defaults to the profile's usage tier, then the standard-tier model.")
			fmt.Fprintln(stdout, "  --overwrite-unmanaged  Grant blanket consent to overwrite any user-owned key")
			fmt.Fprintln(stdout, "                         SquadAI would write. Complements --no-review / CI flows;")
			fmt.Fprintln(stdout, "                         without this flag non-TTY applies halt on merge conflicts.")
			fmt.Fprintln(stdout, "  --agent=<csv>       Explicitly select agents to apply (e.g. opencode,cursor). Bypasses state filter.")
			fmt.Fprintln(stdout, "  --model=role=tier,... Override model tier per role for this run (in-memory only).")
			fmt.Fprintln(stdout, "                      Tiers: premium, standard, cheap. Example: --model=orchestrator=premium,implementer=cheap")
			fmt.Fprintln(stdout, "                      For permanent changes, edit agentmgr.yaml and re-run apply.")
			fmt.Fprintln(stdout, "  --respect-state     (default true) When state exists, restrict apply to previously-installed")
			fmt.Fprintln(stdout, "                      agents union current config. Use --no-respect-state to apply to all detected.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai apply")
			fmt.Fprintln(stdout, "  squadai apply --dry-run")
			fmt.Fprintln(stdout, "  squadai apply --json")
			fmt.Fprintln(stdout, "  squadai apply --force")
			fmt.Fprintln(stdout, "  squadai apply --verbose")
			fmt.Fprintln(stdout, "  squadai apply --no-respect-state")
			fmt.Fprintln(stdout, "  squadai apply --model=orchestrator=premium,implementer=cheap")
			return nil
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

	// Guard: require project.json to exist unless --force is given.
	projectConfigPath := config.ProjectConfigPath(projectDir)
	if _, statErr := os.Stat(projectConfigPath); os.IsNotExist(statErr) {
		if !force {
			return exitcode.ErrPrecondition(
				"no project.json found in current directory",
				"Run 'squadai init' to create one, or use --force to apply with defaults.")
		}
		fmt.Fprintln(stdout, "Warning: No project.json found. Running with default config (--force).")
	}

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}

	// Apply --no-brand: disable the brand component in-memory for this run.
	if noBrand {
		if merged.Components == nil {
			merged.Components = make(map[string]domain.ComponentConfig)
		}
		merged.Components[string(domain.ComponentBrand)] = domain.ComponentConfig{Enabled: false}
	}

	// Apply --model overrides in-memory (does NOT write back to config file).
	if len(modelOverrides) > 0 {
		if err := applyModelOverrides(merged, modelOverrides); err != nil {
			return err
		}
	}

	// Resolve and apply the active context profile (flag > default profile).
	profileName, activeProfile, err := resolveActiveProfile(merged, flagProfile)
	if err != nil {
		return err
	}
	applyProfileToConfig(merged, profileName, activeProfile)

	adapters := DetectAdapters(homeDir)

	// Apply state-based filtering when --respect-state is active (default) and
	// the user did not explicitly pass --agent flags.
	if respectState && len(explicitAgents) == 0 {
		adapters = applyStateFilter(adapters, merged, homeDir)
	} else if len(explicitAgents) > 0 {
		adapters = filterAdapters(adapters, explicitAgents)
	}

	p := planner.New(planner.Options{SetClaudeDefaultAgent: setClaudeDefaultAgent})
	actions, err := p.Plan(merged, adapters, homeDir, projectDir)
	if err != nil {
		return exitcode.ErrPlanFailed(err)
	}

	// Budget fitting: effective cap = --max-tokens flag > profile cap > none.
	// When fitting runs, token counts always come from a real tokenizer via
	// resolveFitModel (--fit-model > profile tier > standard-tier default).
	if cap := effectiveTokenCap(maxTokens, activeProfile); cap > 0 {
		fitModel = resolveFitModel(merged, profileName, fitModel)
		componentTokens, nativeAgentsTokens, tokenErr := desiredComponentTokens(p, actions, homeDir, projectDir, fitModel)
		if tokenErr != nil {
			return fmt.Errorf("budget token estimate: %w", tokenErr)
		}
		fitResult, fitErr := budget.Fit(actions, budget.Options{
			MaxTokens:       cap,
			Model:           fitModel,
			Profile:         profileName,
			ComponentTokens: componentTokens,
			SummaryTokens:   summaryComponentTokens(actions, fitModel, nativeAgentsTokens),
		})
		if fitErr != nil {
			return fmt.Errorf("budget fit: %w", fitErr)
		}
		if !fitResult.FitAchieved {
			fmt.Fprintf(os.Stderr, "warning: could not fit within %d tokens even with all truncation. Proceeding with minimal set.\n", cap)
		}
		actions = fitResult.Actions
		if !dryRun {
			if persistErr := budget.Persist(projectDir, fitResult); persistErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not persist budget: %v\n", persistErr)
			}
		}
	}

	if dryRun {
		if jsonOut {
			data, err := json.MarshalIndent(actions, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal apply actions: %w", err)
			}
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		fmt.Fprintf(stdout, "Dry run: %d action(s) would be executed.\n", len(actions))
		for _, a := range actions {
			fmt.Fprintf(stdout, "  %-8s %s\n", a.Action, a.Description)
		}
		return nil
	}

	// Pre-apply review: show the user every file change before touching disk.
	// Skipped when --no-review, --json, or stdout is not a TTY (CI, pipes).
	var applyPolicy domain.ApplyPolicy
	if shouldRunReview(noReview, jsonOut) {
		entries, err := collectPreviewEntries(p.ComponentInstallers(), adapters, homeDir, projectDir)
		if err != nil {
			return fmt.Errorf("build review preview: %w", err)
		}
		if len(entries) > 0 {
			decision, err := ReviewPromptHook(entries)
			if err != nil {
				return fmt.Errorf("review prompt: %w", err)
			}
			if !decision.Confirmed {
				fmt.Fprintln(stdout, "Apply canceled.")
				return nil
			}
			applyPolicy = decision.Policy
		}
	}
	if overwriteUnmanaged {
		applyPolicy.OverwriteAll = true
	}

	// Create backup store for apply safety.
	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)

	exec := pipeline.New(
		p.ComponentInstallers(),
		p.CopilotManager(),
		projectDir,
		merged.Copilot,
		store,
	)
	exec.WithPolicy(applyPolicy)

	// Determine the effective event sink.
	// --verbose takes precedence and creates its own channel sink.
	// If externalSink was provided (e.g. from TUI), use it when not verbose.
	var effectiveSink pipeline.EventSink
	var verboseCh chan pipeline.Event
	var verboseDone chan struct{}
	if verbose {
		verboseCh = make(chan pipeline.Event, len(actions)+4)
		effectiveSink = pipeline.NewChannelSink(verboseCh, true)
		verboseDone = make(chan struct{})
		go func() {
			defer close(verboseDone)
			for ev := range verboseCh {
				fmt.Fprintln(os.Stderr, ev.String())
			}
		}()
	} else if externalSink != nil {
		effectiveSink = externalSink
	} else {
		effectiveSink = pipeline.NopSink{}
	}

	exec.WithSink(effectiveSink)
	report, execErr := exec.Execute(actions)

	if verbose {
		// Close the channel so the drainer goroutine exits, then wait for it.
		close(verboseCh)
		<-verboseDone
	}

	if execErr != nil {
		if errors.Is(execErr, domain.ErrBackupFailed) {
			return fmt.Errorf("backup failed before apply: %w", execErr)
		}
		if errors.Is(execErr, domain.ErrRollbackFailed) {
			// Critical: rollback itself failed. Report what we can.
			fmt.Fprintln(stdout, "CRITICAL: rollback failed — manual recovery may be needed.")
			if report != nil && report.BackupID != "" {
				fmt.Fprintf(stdout, "  Backup ID: %s\n", report.BackupID)
				fmt.Fprintf(stdout, "  Try: squadai restore %s\n", report.BackupID)
			}
			return execErr
		}
		if errors.Is(execErr, domain.ErrMergeConflict) {
			fmt.Fprintln(stdout, "Apply halted: user-owned keys would be overwritten.")
			fmt.Fprintln(stdout, "Re-run without --no-review to resolve interactively, or")
			fmt.Fprintln(stdout, "pass --overwrite-unmanaged to grant blanket consent.")
			return execErr
		}
		return fmt.Errorf("apply: %w", execErr)
	}

	// Persist installed agent IDs to state (warning-only on failure).
	if report.Success {
		agentIDs := make([]string, 0, len(adapters))
		for _, a := range adapters {
			agentIDs = append(agentIDs, string(a.ID()))
		}
		if statePath, pathErr := state.DefaultPath(); pathErr == nil {
			if st, loadErr := state.Load(statePath); loadErr == nil {
				st.AddAgents(agentIDs)
				st.LastApply = timeNowUTC()
				if saveErr := state.Save(statePath, st); saveErr != nil {
					fmt.Fprintf(stdout, "Warning: could not save state: %v\n", saveErr)
				}
			} else {
				fmt.Fprintf(stdout, "Warning: could not load state: %v\n", loadErr)
			}
		}
	}

	if jsonOut {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal apply report: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		if !report.Success {
			return exitcode.ErrApplyFailed(fmt.Sprintf("rolled back, backup: %s — run 'squadai restore %s' if needed", report.BackupID, report.BackupID))
		}
		return nil
	}

	if report.BackupID != "" {
		fmt.Fprintf(stdout, "Backup: %s\n\n", report.BackupID)
	}

	for _, s := range report.Steps {
		icon := "ok"
		switch s.Status {
		case domain.StepFailed:
			icon = "FAIL"
		case domain.StepRolledBack:
			icon = "SKIP"
		}
		fmt.Fprintf(stdout, "  [%s] %s\n", icon, s.Action.Description)
		if s.Error != "" {
			fmt.Fprintf(stdout, "        error: %s\n", s.Error)
		}
	}

	// Print summary line.
	printApplySummary(stdout, report.Steps)

	if !report.Success {
		fmt.Fprintf(stdout, "\nApply failed. All changes rolled back (backup: %s).\n", report.BackupID)
		fmt.Fprintf(stdout, "Use 'squadai restore %s' to manually restore if needed.\n", report.BackupID)
		return exitcode.ErrApplyFailed(fmt.Sprintf("run 'squadai restore %s' to manually restore if needed", report.BackupID))
	}

	fmt.Fprintln(stdout, "\nApply complete. Use 'squadai verify' to check.")
	return nil
}
