package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
)

// validatePolicyResult is the JSON representation of a validate-policy run.
type validatePolicyResult struct {
	Valid      bool     `json:"valid"`
	Violations []string `json:"violations"`
	PolicyPath string   `json:"policy_path"`
}

// RunValidatePolicy validates .squadai/policy.json in the current directory.
func RunValidatePolicy(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai validate-policy [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Validate .squadai/policy.json in the current directory. Checks that the")
			fmt.Fprintln(stdout, "schema is well-formed, that all locked component IDs are valid, and that required")
			fmt.Fprintln(stdout, "component constraints are internally consistent. Exits non-zero when issues are found.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output the validation result as JSON.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai validate-policy")
			fmt.Fprintln(stdout, "  squadai validate-policy --json")
			return nil
		}
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	policyPath := config.PolicyConfigPath(projectDir)

	policy, err := config.LoadPolicy(projectDir)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return exitcode.ErrPrecondition(
				fmt.Sprintf("no policy file found at %s", policyPath),
				"Run 'squadai init --with-policy' to create one.")
		}
		return fmt.Errorf("load policy: %w", err)
	}

	issues := config.ValidatePolicy(policy)

	if jsonOut {
		violations := issues
		if violations == nil {
			violations = []string{}
		}
		result := validatePolicyResult{
			Valid:      len(issues) == 0,
			Violations: violations,
			PolicyPath: policyPath,
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal validate-policy result: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		if len(issues) > 0 {
			return exitcode.ErrPolicyValidation(len(issues))
		}
		return nil
	}

	if len(issues) == 0 {
		fmt.Fprintln(stdout, "Policy is valid. No issues found.")
		return nil
	}

	fmt.Fprintf(stdout, "Policy validation found %d issue(s):\n", len(issues))
	for i, issue := range issues {
		fmt.Fprintf(stdout, "  %d. %s\n", i+1, issue)
	}
	return exitcode.ErrPolicyValidation(len(issues))
}
