package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/alexmosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/alexmosquera/agent-manager-pro/internal/backup"
	"github.com/alexmosquera/agent-manager-pro/internal/config"
	"github.com/alexmosquera/agent-manager-pro/internal/domain"
	"github.com/alexmosquera/agent-manager-pro/internal/pipeline"
	"github.com/alexmosquera/agent-manager-pro/internal/planner"
	"github.com/alexmosquera/agent-manager-pro/internal/verify"
)

// RunInit creates .agent-manager/project.json and optionally .agent-manager/policy.json
// in the current working directory.
func RunInit(args []string, stdout io.Writer) error {
	withPolicy := false
	for _, arg := range args {
		switch arg {
		case "--with-policy":
			withPolicy = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager init [--with-policy]")
			fmt.Fprintln(stdout, "  Creates .agent-manager/project.json in the current directory.")
			fmt.Fprintln(stdout, "  --with-policy  Also create a team policy template.")
			return nil
		default:
			return fmt.Errorf("unknown flag %q for init", arg)
		}
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	configDir := filepath.Join(projectDir, config.ProjectConfigDir)

	// Create project config.
	projectPath := config.ProjectConfigPath(projectDir)
	if _, err := os.Stat(projectPath); err == nil {
		fmt.Fprintf(stdout, "  exists  %s\n", relPath(projectDir, projectPath))
	} else {
		proj := domain.DefaultProjectConfig()
		if err := config.WriteJSON(projectPath, proj); err != nil {
			return fmt.Errorf("write project config: %w", err)
		}
		fmt.Fprintf(stdout, "  created %s\n", relPath(projectDir, projectPath))
	}

	// Create policy config if requested.
	if withPolicy {
		policyPath := config.PolicyConfigPath(projectDir)
		if _, err := os.Stat(policyPath); err == nil {
			fmt.Fprintf(stdout, "  exists  %s\n", relPath(projectDir, policyPath))
		} else {
			pol := domain.DefaultPolicyConfig()
			if err := config.WriteJSON(policyPath, pol); err != nil {
				return fmt.Errorf("write policy config: %w", err)
			}
			fmt.Fprintf(stdout, "  created %s\n", relPath(projectDir, policyPath))
		}
	}

	// Create user config if it doesn't exist.
	homeDir, err := os.UserHomeDir()
	if err == nil {
		userPath := config.UserConfigPath(homeDir)
		if _, statErr := os.Stat(userPath); statErr != nil {
			userCfg := domain.DefaultUserConfig()
			if writeErr := config.WriteJSON(userPath, userCfg); writeErr == nil {
				fmt.Fprintf(stdout, "  created %s\n", userPath)
			}
		}
	}

	_ = configDir
	fmt.Fprintln(stdout, "\nDone. Review the generated files and commit them to your repository.")
	return nil
}

// relPath returns a relative path from base, falling back to abs on error.
func relPath(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}

// RunValidatePolicy validates .agent-manager/policy.json in the current directory.
func RunValidatePolicy(args []string, stdout io.Writer) error {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			fmt.Fprintln(stdout, "Usage: agent-manager validate-policy")
			fmt.Fprintln(stdout, "  Validates .agent-manager/policy.json schema and lock consistency.")
			return nil
		}
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	policy, err := config.LoadPolicy(projectDir)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return fmt.Errorf("no policy file found at %s", config.PolicyConfigPath(projectDir))
		}
		return fmt.Errorf("load policy: %w", err)
	}

	issues := config.ValidatePolicy(policy)
	if len(issues) == 0 {
		fmt.Fprintln(stdout, "Policy is valid. No issues found.")
		return nil
	}

	fmt.Fprintf(stdout, "Policy validation found %d issue(s):\n", len(issues))
	for i, issue := range issues {
		fmt.Fprintf(stdout, "  %d. %s\n", i+1, issue)
	}
	return fmt.Errorf("policy validation failed with %d issue(s)", len(issues))
}

// RunPlan computes and displays the action plan.
func RunPlan(args []string, stdout io.Writer) error {
	dryRun := false
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager plan [--dry-run] [--json]")
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

	adapters := detectAdapters(homeDir)
	p := planner.New()
	actions, err := p.Plan(merged, adapters, homeDir, projectDir)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	if jsonOut {
		data, _ := json.MarshalIndent(actions, "", "  ")
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
	for _, a := range actions {
		fmt.Fprintf(stdout, "  %-8s %-40s %s\n", a.Action, a.Description, a.TargetPath)
	}

	fmt.Fprintln(stdout, "\nUse 'agent-manager apply' to execute.")
	return nil
}

// RunApply executes the plan with backup safety and step-level reporting.
func RunApply(args []string, stdout io.Writer) error {
	dryRun := false
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager apply [--dry-run] [--json]")
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

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}

	adapters := detectAdapters(homeDir)
	p := planner.New()
	actions, err := p.Plan(merged, adapters, homeDir, projectDir)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	if dryRun {
		if jsonOut {
			data, _ := json.MarshalIndent(actions, "", "  ")
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		fmt.Fprintf(stdout, "Dry run: %d action(s) would be executed.\n", len(actions))
		for _, a := range actions {
			fmt.Fprintf(stdout, "  %-8s %s\n", a.Action, a.Description)
		}
		return nil
	}

	// Create backup store for apply safety.
	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)

	exec := pipeline.New(
		p.ComponentInstallers(),
		p.CopilotManager(),
		projectDir,
		merged.Copilot.InstructionsTemplate,
		store,
	)

	report, execErr := exec.Execute(actions)
	if execErr != nil {
		if errors.Is(execErr, domain.ErrBackupFailed) {
			return fmt.Errorf("backup failed before apply: %w", execErr)
		}
		if errors.Is(execErr, domain.ErrRollbackFailed) {
			// Critical: rollback itself failed. Report what we can.
			fmt.Fprintln(stdout, "CRITICAL: rollback failed — manual recovery may be needed.")
			if report != nil && report.BackupID != "" {
				fmt.Fprintf(stdout, "  Backup ID: %s\n", report.BackupID)
				fmt.Fprintf(stdout, "  Try: agent-manager restore %s\n", report.BackupID)
			}
			return execErr
		}
		return fmt.Errorf("apply: %w", execErr)
	}

	if jsonOut {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintln(stdout, string(data))
		if !report.Success {
			return fmt.Errorf("apply completed with failures (rolled back, backup: %s)", report.BackupID)
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

	if !report.Success {
		fmt.Fprintf(stdout, "\nApply failed. All changes rolled back (backup: %s).\n", report.BackupID)
		fmt.Fprintf(stdout, "Use 'agent-manager restore %s' to manually restore if needed.\n", report.BackupID)
		return fmt.Errorf("apply completed with failures")
	}

	fmt.Fprintln(stdout, "\nApply complete. Use 'agent-manager verify' to check.")
	return nil
}

// RunSync performs idempotent reconciliation (same as apply — plan then execute).
func RunSync(args []string, stdout io.Writer) error {
	// Sync is semantically identical to apply — it plans and executes.
	// The idempotency comes from the planner returning Skip for up-to-date items.
	return RunApply(args, stdout)
}

// RunVerify runs compliance checks and prints the report.
func RunVerify(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager verify [--json]")
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

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}

	adapters := detectAdapters(homeDir)
	v := verify.New()
	report, err := v.Verify(merged, adapters, homeDir, projectDir)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	if jsonOut {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintln(stdout, string(data))
		if !report.AllPass {
			return fmt.Errorf("verification failed")
		}
		return nil
	}

	if len(report.Results) == 0 {
		fmt.Fprintln(stdout, "No checks to run (no components or adapters enabled).")
		return nil
	}

	for _, r := range report.Results {
		icon := "PASS"
		if !r.Passed {
			icon = "FAIL"
		}
		line := fmt.Sprintf("  [%s] %s", icon, r.Check)
		if r.Message != "" {
			line += " — " + r.Message
		}
		fmt.Fprintln(stdout, line)
	}

	if !report.AllPass {
		return fmt.Errorf("verification failed")
	}

	fmt.Fprintln(stdout, "\nAll checks passed.")
	return nil
}

// RunBackupCreate creates a manual backup snapshot of all managed files.
func RunBackupCreate(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager backup create [--json]")
			fmt.Fprintln(stdout, "  Creates a snapshot of all managed files.")
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

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}

	// Plan to discover which files would be affected.
	adapters := detectAdapters(homeDir)
	p := planner.New()
	actions, err := p.Plan(merged, adapters, homeDir, projectDir)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	// Collect all target paths (including skip — we want a full snapshot).
	paths := collectAllTargetPaths(actions)
	if len(paths) == 0 {
		fmt.Fprintln(stdout, "No managed files found to back up.")
		return nil
	}

	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)
	manifest, err := store.SnapshotFiles(paths, "manual")
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}

	if jsonOut {
		data, _ := json.MarshalIndent(manifest, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "Backup created: %s\n", manifest.ID)
	fmt.Fprintf(stdout, "  Files: %d\n", len(manifest.AffectedFiles))
	fmt.Fprintf(stdout, "  Time:  %s\n", manifest.Timestamp.Format("2006-01-02 15:04:05 UTC"))
	return nil
}

// RunBackupList lists available backups.
func RunBackupList(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager backup list [--json]")
			return nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, "")
	if err != nil {
		// If no project config, use default backup dir.
		merged = &domain.MergedConfig{
			Paths: domain.PathsConfig{BackupDir: "~/.agent-manager/backups"},
		}
	}

	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)
	manifests, err := store.List()
	if err != nil {
		return fmt.Errorf("list backups: %w", err)
	}

	if len(manifests) == 0 {
		fmt.Fprintln(stdout, "No backups found.")
		return nil
	}

	if jsonOut {
		data, _ := json.MarshalIndent(manifests, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "Backups (%d):\n\n", len(manifests))
	fmt.Fprintf(stdout, "  %-36s  %-10s  %-5s  %s\n", "ID", "COMMAND", "FILES", "STATUS")
	for _, m := range manifests {
		fmt.Fprintf(stdout, "  %-36s  %-10s  %-5d  %s\n",
			m.ID, m.Command, len(m.AffectedFiles), m.Status)
	}
	return nil
}

// RunRestore restores files from a backup.
func RunRestore(args []string, stdout io.Writer) error {
	jsonOut := false
	dryRun := false
	var backupID string

	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "--dry-run":
			dryRun = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager restore <backup-id> [--dry-run] [--json]")
			fmt.Fprintln(stdout, "  Restores managed files from a backup snapshot.")
			return nil
		default:
			if backupID == "" {
				backupID = arg
			} else {
				return fmt.Errorf("unexpected argument %q", arg)
			}
		}
	}

	if backupID == "" {
		return fmt.Errorf("backup ID is required — usage: agent-manager restore <backup-id>")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, "")
	if err != nil {
		merged = &domain.MergedConfig{
			Paths: domain.PathsConfig{BackupDir: "~/.agent-manager/backups"},
		}
	}

	backupDir := backup.ResolveBackupDir(merged.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)

	manifest, err := store.Get(backupID)
	if err != nil {
		return fmt.Errorf("load backup: %w", err)
	}

	if dryRun {
		if jsonOut {
			data, _ := json.MarshalIndent(manifest, "", "  ")
			fmt.Fprintln(stdout, string(data))
			return nil
		}
		fmt.Fprintf(stdout, "Dry run: would restore %d file(s) from backup %s\n", len(manifest.AffectedFiles), backupID)
		for _, f := range manifest.AffectedFiles {
			if f.ExistedBefore {
				fmt.Fprintf(stdout, "  restore %s\n", f.Path)
			} else {
				fmt.Fprintf(stdout, "  remove  %s\n", f.Path)
			}
		}
		return nil
	}

	if err := store.Restore(backupID); err != nil {
		return fmt.Errorf("restore: %w", err)
	}

	if jsonOut {
		result := map[string]interface{}{
			"backup_id": backupID,
			"restored":  len(manifest.AffectedFiles),
			"status":    "restored",
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "Restored %d file(s) from backup %s.\n", len(manifest.AffectedFiles), backupID)
	for _, f := range manifest.AffectedFiles {
		if f.ExistedBefore {
			fmt.Fprintf(stdout, "  restored %s\n", f.Path)
		} else {
			fmt.Fprintf(stdout, "  removed  %s\n", f.Path)
		}
	}
	return nil
}

// collectAllTargetPaths extracts unique target paths from all actions (including skips).
func collectAllTargetPaths(actions []domain.PlannedAction) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, a := range actions {
		if a.TargetPath != "" && !seen[a.TargetPath] {
			seen[a.TargetPath] = true
			paths = append(paths, a.TargetPath)
		}
	}
	return paths
}

// detectAdapters returns all registered adapters that are installed or have config.
// For now this is just OpenCode. Claude/Codex will be added later.
func detectAdapters(homeDir string) []domain.Adapter {
	var adapters []domain.Adapter

	oc := opencode.New()
	installed, configFound, err := oc.Detect(context.Background(), homeDir)
	if err == nil && (installed || configFound) {
		adapters = append(adapters, oc)
	}
	// Even if not detected, include OpenCode so the planner can plan file creation.
	if len(adapters) == 0 {
		adapters = append(adapters, oc)
	}

	return adapters
}

// loadAndMerge is the shared config loading logic for commands that need merged config.
func loadAndMerge(homeDir, projectDir string) (*domain.MergedConfig, error) {
	user, err := config.LoadUser(homeDir)
	if err != nil && !errors.Is(err, domain.ErrConfigNotFound) {
		return nil, fmt.Errorf("load user config: %w", err)
	}

	project, err := config.LoadProject(projectDir)
	if err != nil && !errors.Is(err, domain.ErrConfigNotFound) {
		return nil, fmt.Errorf("load project config: %w", err)
	}

	policy, err := config.LoadPolicy(projectDir)
	if err != nil && !errors.Is(err, domain.ErrConfigNotFound) {
		return nil, fmt.Errorf("load policy: %w", err)
	}

	return config.Merge(user, project, policy), nil
}
