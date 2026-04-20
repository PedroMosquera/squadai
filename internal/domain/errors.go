package domain

import (
	"errors"
	"fmt"
)

var (
	// ErrConfigNotFound is returned when a config file does not exist.
	ErrConfigNotFound = errors.New("config file not found")

	// ErrInvalidConfig is returned when a config file fails schema validation.
	ErrInvalidConfig = errors.New("invalid config")

	// ErrPolicyViolation is returned when a user/project value conflicts with a locked policy field.
	ErrPolicyViolation = errors.New("policy violation")

	// ErrAdapterNotFound is returned when requesting an adapter that isn't registered.
	ErrAdapterNotFound = errors.New("adapter not found")

	// ErrComponentNotSupported is returned when an adapter doesn't support a component.
	ErrComponentNotSupported = errors.New("component not supported by adapter")

	// ErrBackupFailed is returned when backup creation fails before apply.
	ErrBackupFailed = errors.New("backup creation failed")

	// ErrRollbackFailed is returned when rollback itself fails (critical state).
	ErrRollbackFailed = errors.New("rollback failed")

	// ErrMergeConflict is returned when an installer cannot complete a JSON
	// merge because the user has hand-edited one or more top-level keys
	// SquadAI would have overwritten and no ApplyPolicy override is present.
	// Callers use errors.Is(err, ErrMergeConflict) to detect the condition and
	// errors.As(err, &*ConflictError) to recover the offending keys.
	ErrMergeConflict = errors.New("merge conflict")
)

// ConflictError is the concrete error returned when a merge hits one or more
// unresolvable conflicts. It wraps ErrMergeConflict so errors.Is matches and
// carries the per-key details so the CLI/TUI can render a useful report.
type ConflictError struct {
	TargetPath string
	Conflicts  []Conflict
}

func (e *ConflictError) Error() string {
	if len(e.Conflicts) == 0 {
		return fmt.Sprintf("merge conflict at %s", e.TargetPath)
	}
	keys := make([]string, 0, len(e.Conflicts))
	for _, c := range e.Conflicts {
		keys = append(keys, c.Key)
	}
	return fmt.Sprintf("merge conflict at %s: user owns %d key(s) SquadAI would overwrite: %v",
		e.TargetPath, len(e.Conflicts), keys)
}

func (e *ConflictError) Unwrap() error {
	return ErrMergeConflict
}

// PolicyViolationError provides detail about which field violated policy.
type PolicyViolationError struct {
	Field          string
	PolicyValue    string
	AttemptedValue string
}

func (e *PolicyViolationError) Error() string {
	return fmt.Sprintf("policy violation: field %q is locked to %q, cannot set %q",
		e.Field, e.PolicyValue, e.AttemptedValue)
}

func (e *PolicyViolationError) Unwrap() error {
	return ErrPolicyViolation
}

// ValidationError collects multiple validation failures.
type ValidationError struct {
	Source string // e.g., "config.json", "policy.json"
	Issues []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for %s: %d issue(s)", e.Source, len(e.Issues))
}

func (e *ValidationError) Unwrap() error {
	return ErrInvalidConfig
}
