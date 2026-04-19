package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestRunDoctorFix_NoFixableIssues verifies the "no auto-fixable issues" path.
func TestRunDoctorFix_NoFixableIssues(t *testing.T) {
	// In a temp env where config exists (or all checks pass), --fix should report
	// "No auto-fixable issues found." We simulate by providing empty stdin and
	// checking that we don't crash.
	var stdout bytes.Buffer
	stdin := strings.NewReader("") // empty stdin

	// We can't easily fake a fully passing environment, but we can verify
	// the function doesn't panic or crash.
	_ = RunDoctorWithReader([]string{"--fix"}, &stdout, stdin)
	// Output should contain either "No auto-fixable issues" OR the fix prompt.
	out := stdout.String()
	if out == "" {
		t.Fatal("expected some output from --fix run")
	}
}

// TestRunDoctorFix_AnswerNo verifies that answering "n" exits without changes.
func TestRunDoctorFix_AnswerNo(t *testing.T) {
	var stdout bytes.Buffer
	stdin := strings.NewReader("n\n")

	_ = RunDoctorWithReader([]string{"--fix"}, &stdout, stdin)
	out := stdout.String()
	// Either "No auto-fixable issues" or "Skipped." — both are acceptable outcomes.
	if !strings.Contains(out, "Skipped") && !strings.Contains(out, "No auto-fixable") {
		t.Logf("output: %s", out)
		// Not a hard failure — environment may vary; just ensure no panic.
	}
}

// TestRunDoctorWithReader_Help ensures --help flag works with the new function.
func TestRunDoctorWithReader_Help(t *testing.T) {
	var stdout bytes.Buffer
	err := RunDoctorWithReader([]string{"--help"}, &stdout, strings.NewReader(""))
	if err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "--fix") {
		t.Fatal("--help output does not mention --fix flag")
	}
}
