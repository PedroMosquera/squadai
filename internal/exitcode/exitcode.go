// Package exitcode defines the standard exit codes and error types for the
// squadai CLI. Every command exits with a code from this table; 0 is the
// only success code. Callers may test errors with errors.As(err, &AppError{}).
package exitcode

import "fmt"

const (
	OK          = 0 // success
	Unexpected  = 1 // unhandled / internal error
	Config      = 2 // invalid config, unknown flag, schema violation
	Policy      = 3 // policy violation, locked field overridden
	Drift       = 4 // drift detected (verify --strict)
	NotFound    = 5 // resource not found (plugin, backup ID, agent)
	Precondition = 6 // precondition failed (registry not synced, no git repo)
	Network     = 7 // network / GitHub API error
	Permission  = 8 // file permission denied or write protected
)

// AppError is returned by CLI functions when the error maps to a known exit
// code. Wrapping an AppError makes cmd/squadai/main.go emit the right code
// without parsing error strings.
type AppError struct {
	Code    int
	ErrCode string // E-xxx identifier, e.g. "E-201"
	Msg     string
	Hint    string
	Cause   error
}

func (e *AppError) Error() string {
	if e.ErrCode != "" {
		return fmt.Sprintf("%s: %s", e.ErrCode, e.Msg)
	}
	return e.Msg
}

func (e *AppError) Unwrap() error { return e.Cause }

// New constructs an AppError. hint may be empty.
func New(code int, errCode, msg, hint string) *AppError {
	return &AppError{Code: code, ErrCode: errCode, Msg: msg, Hint: hint}
}

// Wrap wraps an existing error into an AppError, preserving the cause chain.
func Wrap(code int, errCode, msg, hint string, cause error) *AppError {
	return &AppError{Code: code, ErrCode: errCode, Msg: msg, Hint: hint, Cause: cause}
}

// ─── pre-defined sentinels ────────────────────────────────────────────────────

// ─── E-1xx: configuration / argument errors ──────────────────────────────────

func ErrConfig(msg string) *AppError {
	return New(Config, "E-100", msg, "Run 'squadai validate-policy' to check your config.")
}

func ErrConfigf(format string, args ...any) *AppError {
	return ErrConfig(fmt.Sprintf(format, args...))
}

// ErrUnknownValue reports an unrecognised value for a named flag or field.
func ErrUnknownValue(flag, value, allowed string) *AppError {
	return New(Config, "E-101",
		fmt.Sprintf("unknown value %q for %s", value, flag),
		fmt.Sprintf("Allowed values: %s", allowed))
}

// ErrFlagConflict reports mutually exclusive flags.
func ErrFlagConflict(flags string) *AppError {
	return New(Config, "E-102",
		"conflicting flags: "+flags,
		"Provide only one of the listed flags.")
}

// ErrMissingArg reports a required argument that was not supplied.
func ErrMissingArg(arg, usage string) *AppError {
	return New(Config, "E-103",
		"missing required argument: "+arg,
		"Usage: "+usage)
}

// ErrApplyFailed reports that squadai apply completed with one or more step failures.
func ErrApplyFailed(hint string) *AppError {
	return New(Config, "E-302", "apply completed with failures", hint)
}

// ErrPlanFailed reports that plan generation failed.
func ErrPlanFailed(cause error) *AppError {
	return Wrap(Config, "E-303", "plan generation failed",
		"Check your project.json and policy.json for errors, then run 'squadai validate-policy'.", cause)
}

// ─── E-2xx: policy errors ─────────────────────────────────────────────────────

func ErrPolicyViolation(field, hint string) *AppError {
	return New(Policy, "E-201", "policy violation: "+field, hint)
}

// ErrPolicyValidation reports that validate-policy found issues.
func ErrPolicyValidation(count int) *AppError {
	return New(Config, "E-202",
		fmt.Sprintf("policy validation failed with %d issue(s)", count),
		"Fix the issues listed above, then re-run 'squadai validate-policy'.")
}

// ─── E-4xx: drift errors ──────────────────────────────────────────────────────

func ErrDrift(path string) *AppError {
	return New(Drift, "E-401", "drift detected: "+path,
		"Run 'squadai diff' to see what changed, then 'squadai apply' to restore.")
}

// ─── E-5xx: not found ─────────────────────────────────────────────────────────

func ErrNotFound(resource string) *AppError {
	return New(NotFound, "E-501", resource+" not found", "")
}

// ErrBackupNotFound reports a missing backup by ID.
func ErrBackupNotFound(id string) *AppError {
	return New(NotFound, "E-502",
		fmt.Sprintf("backup %q not found", id),
		"Run 'squadai backup list' to see available backup IDs.")
}

// ─── E-6xx: precondition errors ───────────────────────────────────────────────

func ErrPrecondition(msg, hint string) *AppError {
	return New(Precondition, "E-601", msg, hint)
}

// ─── E-7xx: network errors ────────────────────────────────────────────────────

func ErrNetwork(url string, cause error) *AppError {
	return Wrap(Network, "E-701", "network error fetching "+url,
		"Check your internet connection and try again.", cause)
}

// ─── E-8xx: permission errors ─────────────────────────────────────────────────

func ErrPermission(path string, cause error) *AppError {
	return Wrap(Permission, "E-801", "permission denied: "+path,
		"Check file permissions or run with appropriate privileges.", cause)
}
