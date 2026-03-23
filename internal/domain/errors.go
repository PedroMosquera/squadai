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
)

// PolicyViolationError provides detail about which field violated policy.
type PolicyViolationError struct {
	Field       string
	PolicyValue string
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
	Source string   // e.g., "config.json", "policy.json"
	Issues []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for %s: %d issue(s)", e.Source, len(e.Issues))
}

func (e *ValidationError) Unwrap() error {
	return ErrInvalidConfig
}
