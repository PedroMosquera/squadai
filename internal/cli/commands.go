package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/claude"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/codex"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/PedroMosquera/agent-manager-pro/internal/assets"
	"github.com/PedroMosquera/agent-manager-pro/internal/backup"
	"github.com/PedroMosquera/agent-manager-pro/internal/config"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/pipeline"
	"github.com/PedroMosquera/agent-manager-pro/internal/planner"
	"github.com/PedroMosquera/agent-manager-pro/internal/verify"
)

// RunInit creates .agent-manager/project.json and optionally .agent-manager/policy.json
// in the current working directory. It detects adapters, selects language-specific
// standards, and writes starter skill files.
func RunInit(args []string, stdout io.Writer) error {
	withPolicy := false
	force := false
	for _, arg := range args {
		switch arg {
		case "--with-policy":
			withPolicy = true
		case "--force":
			force = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager init [--with-policy] [--force]")
			fmt.Fprintln(stdout, "  Creates .agent-manager/project.json in the current directory.")
			fmt.Fprintln(stdout, "  --with-policy  Also create a team policy template.")
			fmt.Fprintln(stdout, "  --force        Overwrite existing template and skill files.")
			return nil
		default:
			return fmt.Errorf("unknown flag %q for init", arg)
		}
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "" // non-fatal, adapter detection will be limited
	}

	// Detect project metadata.
	meta := DetectProjectMeta(projectDir)

	// Detect installed adapters.
	var detectedAdapters []domain.Adapter
	if homeDir != "" {
		detectedAdapters = DetectAdapters(homeDir)
	}

	// Create project config.
	projectPath := config.ProjectConfigPath(projectDir)
	_, projectExists := os.Stat(projectPath)
	if projectExists == nil && !force {
		fmt.Fprintf(stdout, "  exists  %s\n", relPath(projectDir, projectPath))
	} else {
		proj := buildSmartProjectConfig(meta, detectedAdapters)
		if err := config.WriteJSON(projectPath, proj); err != nil {
			return fmt.Errorf("write project config: %w", err)
		}
		if projectExists == nil && force {
			fmt.Fprintf(stdout, "  overwritten %s\n", relPath(projectDir, projectPath))
		} else {
			fmt.Fprintf(stdout, "  created %s\n", relPath(projectDir, projectPath))
		}
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
	if homeDir != "" {
		userPath := config.UserConfigPath(homeDir)
		if _, statErr := os.Stat(userPath); statErr != nil {
			userCfg := domain.DefaultUserConfig()
			if writeErr := config.WriteJSON(userPath, userCfg); writeErr == nil {
				fmt.Fprintf(stdout, "  created %s\n", userPath)
			}
		}
	}

	// Write language-specific team standards.
	standardsContent := selectStandards(meta.Language)
	standardsPath := filepath.Join(projectDir, config.ProjectConfigDir, "templates", "team-standards.md")
	writeInitFile(stdout, projectDir, standardsPath, standardsContent, force)

	// Write starter skill files.
	skillFiles := []struct {
		name string
		path string
	}{
		{"skills/code-review/SKILL.md", filepath.Join(projectDir, config.ProjectConfigDir, "skills", "code-review.md")},
		{"skills/testing/SKILL.md", filepath.Join(projectDir, config.ProjectConfigDir, "skills", "testing.md")},
		{"skills/pr-description/SKILL.md", filepath.Join(projectDir, config.ProjectConfigDir, "skills", "pr-description.md")},
	}
	for _, sf := range skillFiles {
		content := assets.MustRead(sf.name)
		writeInitFile(stdout, projectDir, sf.path, content, force)
	}

	// Print summary.
	fmt.Fprintln(stdout)
	if meta.Name != "" || meta.Language != "" {
		fmt.Fprintln(stdout, "Detected:")
		if meta.Language != "" {
			fmt.Fprintf(stdout, "  Language: %s\n", meta.Language)
		}
		if meta.Name != "" {
			fmt.Fprintf(stdout, "  Project:  %s\n", meta.Name)
		}
		adapterNames := adapterSummary(detectedAdapters)
		if adapterNames != "" {
			fmt.Fprintf(stdout, "  Agents:   %s\n", adapterNames)
		}
		fmt.Fprintln(stdout)
	}

	fmt.Fprintln(stdout, "Run 'agent-manager apply' to configure your environment.")
	return nil
}

// buildSmartProjectConfig creates a rich project.json from detected metadata and adapters.
func buildSmartProjectConfig(meta domain.ProjectMeta, adapters []domain.Adapter) *domain.ProjectConfig {
	proj := &domain.ProjectConfig{
		Version: 1,
		Meta:    meta,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentOpenCode): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			"copilot":                      {Enabled: true},
			string(domain.ComponentRules): {
				Enabled: true,
				Settings: map[string]interface{}{
					"team_standards_file": "templates/team-standards.md",
				},
			},
		},
		Copilot: domain.CopilotConfig{
			InstructionsTemplate: "standard",
		},
		Skills: map[string]domain.SkillDef{
			"code-review": {
				Description: "Structured code review",
				ContentFile: "skills/code-review.md",
			},
			"testing": {
				Description: "Test writing protocol",
				ContentFile: "skills/testing.md",
			},
			"pr-description": {
				Description: "PR description generation",
				ContentFile: "skills/pr-description.md",
			},
		},
	}

	// Enable detected personal-lane adapters.
	for _, a := range adapters {
		if a.ID() == domain.AgentClaudeCode {
			proj.Adapters[string(domain.AgentClaudeCode)] = domain.AdapterConfig{Enabled: true}
		}
		if a.ID() == domain.AgentCodex {
			proj.Adapters[string(domain.AgentCodex)] = domain.AdapterConfig{Enabled: true}
		}
	}

	return proj
}

// selectStandards returns the content of the language-specific standards asset.
func selectStandards(language string) string {
	switch language {
	case "Go":
		return assets.MustRead("standards/go.md")
	case "TypeScript", "TypeScript/JavaScript":
		return assets.MustRead("standards/javascript.md")
	case "Python":
		return assets.MustRead("standards/python.md")
	default:
		return assets.MustRead("standards/generic.md")
	}
}

// writeInitFile writes content to path, respecting the force flag.
// Reports status to stdout.
func writeInitFile(stdout io.Writer, projectDir, path, content string, force bool) {
	rel := relPath(projectDir, path)
	_, existsErr := os.Stat(path)
	existed := existsErr == nil

	if existed && !force {
		fmt.Fprintf(stdout, "  exists  %s\n", rel)
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Fprintf(stdout, "  error   %s: %v\n", rel, err)
		return
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(stdout, "  error   %s: %v\n", rel, err)
		return
	}

	if existed && force {
		fmt.Fprintf(stdout, "  overwritten %s\n", rel)
	} else {
		fmt.Fprintf(stdout, "  created %s\n", rel)
	}
}

// adapterSummary returns a comma-separated list of adapter names.
func adapterSummary(adapters []domain.Adapter) string {
	if len(adapters) == 0 {
		return ""
	}
	names := ""
	for i, a := range adapters {
		if i > 0 {
			names += ", "
		}
		names += string(a.ID())
	}
	return names
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

	adapters := DetectAdapters(homeDir)
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

	adapters := DetectAdapters(homeDir)
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
		merged.Copilot,
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

	// Print summary line.
	printApplySummary(stdout, report.Steps)

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

	adapters := DetectAdapters(homeDir)
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

	// Group results by component if there are enough.
	if len(report.Results) > 5 {
		printGroupedResults(stdout, report.Results)
	} else {
		for _, r := range report.Results {
			printVerifyResult(stdout, r)
		}
	}

	// Print summary line.
	printVerifySummary(stdout, report.Results)

	if !report.AllPass {
		return fmt.Errorf("verification failed")
	}

	return nil
}

// printVerifyResult prints a single verification result line.
func printVerifyResult(stdout io.Writer, r domain.VerifyResult) {
	icon := "PASS"
	if !r.Passed {
		icon = "FAIL"
	}
	if r.Severity == domain.SeverityWarning {
		icon = "WARN"
	}
	line := fmt.Sprintf("  [%s] %s", icon, r.Check)
	if r.Message != "" {
		line += " — " + r.Message
	}
	fmt.Fprintln(stdout, line)
}

// printGroupedResults groups verification results by Component field and prints them.
func printGroupedResults(stdout io.Writer, results []domain.VerifyResult) {
	// Collect groups in order of first appearance.
	type group struct {
		name    string
		results []domain.VerifyResult
	}
	var groups []group
	seen := make(map[string]int)

	for _, r := range results {
		comp := r.Component
		if comp == "" {
			comp = "General"
		}
		if idx, ok := seen[comp]; ok {
			groups[idx].results = append(groups[idx].results, r)
		} else {
			seen[comp] = len(groups)
			groups = append(groups, group{name: comp, results: []domain.VerifyResult{r}})
		}
	}

	for i, g := range groups {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprintf(stdout, "%s:\n", g.name)
		for _, r := range g.results {
			printVerifyResult(stdout, r)
		}
	}
}

// printApplySummary counts written/skipped/failed steps and prints a one-line summary.
func printApplySummary(stdout io.Writer, steps []domain.StepResult) {
	var written, skipped, failed int
	for _, s := range steps {
		switch {
		case s.Status == domain.StepSuccess:
			if s.Action.Action == domain.ActionSkip {
				skipped++
			} else {
				written++
			}
		case s.Status == domain.StepFailed:
			failed++
		case s.Status == domain.StepRolledBack:
			failed++
		default:
			written++
		}
	}
	fmt.Fprintf(stdout, "\nApplied %d action(s): %d written, %d skipped, %d failed\n", len(steps), written, skipped, failed)
}

// printVerifySummary counts passed/failed/warning results and prints a one-line summary.
func printVerifySummary(stdout io.Writer, results []domain.VerifyResult) {
	var passed, failedCount, warnings int
	for _, r := range results {
		if r.Severity == domain.SeverityWarning {
			warnings++
		} else if r.Passed {
			passed++
		} else {
			failedCount++
		}
	}
	fmt.Fprintf(stdout, "\n%d checks: %d passed, %d failed, %d warnings\n", len(results), passed, failedCount, warnings)
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
	adapters := DetectAdapters(homeDir)
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

// DetectAdapters returns all registered adapters that are installed or have config.
// OpenCode (team lane) is always included. Claude Code and Codex (personal lane)
// are included only when detected on the system.
func DetectAdapters(homeDir string) []domain.Adapter {
	ctx := context.Background()
	var adapters []domain.Adapter

	// OpenCode is always included — team baseline.
	oc := opencode.New()
	adapters = append(adapters, oc)

	// Personal-lane adapters: include only if binary or config is found.
	cc := claude.New()
	if installed, configFound, err := cc.Detect(ctx, homeDir); err == nil && (installed || configFound) {
		adapters = append(adapters, cc)
	}

	cx := codex.New()
	if installed, configFound, err := cx.Detect(ctx, homeDir); err == nil && (installed || configFound) {
		adapters = append(adapters, cx)
	}

	return adapters
}

// LoadAndMerge is the shared config loading logic for commands that need merged config.
func LoadAndMerge(homeDir, projectDir string) (*domain.MergedConfig, error) {
	return loadAndMerge(homeDir, projectDir)
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
