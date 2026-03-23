package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/alexmosquera/agent-manager-pro/internal/config"
	"github.com/alexmosquera/agent-manager-pro/internal/domain"
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
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: agent-manager plan [--dry-run]")
			return nil
		}
	}

	_ = dryRun

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

	// Report violations first.
	if len(merged.Violations) > 0 {
		fmt.Fprintln(stdout, "Policy overrides:")
		for _, v := range merged.Violations {
			fmt.Fprintf(stdout, "  - %s\n", v)
		}
		fmt.Fprintln(stdout)
	}

	fmt.Fprintf(stdout, "Mode: %s\n\n", merged.Mode)

	fmt.Fprintln(stdout, "Enabled adapters:")
	for name, cfg := range merged.Adapters {
		status := "disabled"
		if cfg.Enabled {
			status = "enabled"
		}
		fmt.Fprintf(stdout, "  %-15s %s\n", name, status)
	}
	fmt.Fprintln(stdout)

	fmt.Fprintln(stdout, "Enabled components:")
	for name, cfg := range merged.Components {
		status := "disabled"
		if cfg.Enabled {
			status = "enabled"
		}
		fmt.Fprintf(stdout, "  %-15s %s\n", name, status)
	}
	fmt.Fprintln(stdout)

	if merged.Copilot.InstructionsTemplate != "" {
		fmt.Fprintf(stdout, "Copilot instructions: %s\n", merged.Copilot.InstructionsTemplate)
	}

	fmt.Fprintln(stdout, "\nPlan complete. Use 'agent-manager apply' to execute.")
	return nil
}

// RunApply executes the plan with backup/rollback safety.
func RunApply(args []string, stdout io.Writer) error {
	fmt.Fprintln(stdout, "Apply is not yet implemented. Coming in Milestone B.")
	return nil
}

// RunSync performs idempotent reconciliation.
func RunSync(args []string, stdout io.Writer) error {
	fmt.Fprintln(stdout, "Sync is not yet implemented. Coming in Milestone B.")
	return nil
}

// RunVerify runs compliance checks.
func RunVerify(args []string, stdout io.Writer) error {
	fmt.Fprintln(stdout, "Verify is not yet implemented. Coming in Milestone B.")
	return nil
}

// RunBackupCreate creates a backup snapshot.
func RunBackupCreate(args []string, stdout io.Writer) error {
	fmt.Fprintln(stdout, "Backup create is not yet implemented. Coming in Milestone C.")
	return nil
}

// RunBackupList lists available backups.
func RunBackupList(args []string, stdout io.Writer) error {
	fmt.Fprintln(stdout, "Backup list is not yet implemented. Coming in Milestone C.")
	return nil
}

// RunRestore restores from a backup.
func RunRestore(args []string, stdout io.Writer) error {
	fmt.Fprintln(stdout, "Restore is not yet implemented. Coming in Milestone C.")
	return nil
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
