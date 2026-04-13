package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// ─── printVerifyResult ──────────────────────────────────────────────────────

func TestPrintVerifyResult_Pass(t *testing.T) {
	var buf bytes.Buffer
	r := domain.VerifyResult{
		Check:  "config-exists",
		Passed: true,
	}
	printVerifyResult(&buf, r)

	got := buf.String()
	if !strings.Contains(got, "[PASS]") {
		t.Errorf("expected [PASS], got %q", got)
	}
	if !strings.Contains(got, "config-exists") {
		t.Errorf("expected check name, got %q", got)
	}
}

func TestPrintVerifyResult_Fail(t *testing.T) {
	var buf bytes.Buffer
	r := domain.VerifyResult{
		Check:   "memory-installed",
		Passed:  false,
		Message: "file missing",
	}
	printVerifyResult(&buf, r)

	got := buf.String()
	if !strings.Contains(got, "[FAIL]") {
		t.Errorf("expected [FAIL], got %q", got)
	}
	if !strings.Contains(got, "memory-installed") {
		t.Errorf("expected check name, got %q", got)
	}
	if !strings.Contains(got, "file missing") {
		t.Errorf("expected message, got %q", got)
	}
}

func TestPrintVerifyResult_Warning(t *testing.T) {
	var buf bytes.Buffer
	r := domain.VerifyResult{
		Check:    "optional-config",
		Passed:   false,
		Severity: domain.SeverityWarning,
	}
	printVerifyResult(&buf, r)

	got := buf.String()
	if !strings.Contains(got, "[WARN]") {
		t.Errorf("expected [WARN], got %q", got)
	}
}

func TestPrintVerifyResult_WarningOverridesPass(t *testing.T) {
	var buf bytes.Buffer
	r := domain.VerifyResult{
		Check:    "optional-config",
		Passed:   true,
		Severity: domain.SeverityWarning,
	}
	printVerifyResult(&buf, r)

	got := buf.String()
	if !strings.Contains(got, "[WARN]") {
		t.Errorf("warning severity should show [WARN] even when passed, got %q", got)
	}
	if strings.Contains(got, "[PASS]") {
		t.Errorf("warning severity should not show [PASS], got %q", got)
	}
}

func TestPrintVerifyResult_MessageAppended(t *testing.T) {
	var buf bytes.Buffer
	r := domain.VerifyResult{
		Check:   "check-a",
		Passed:  true,
		Message: "all good",
	}
	printVerifyResult(&buf, r)

	got := buf.String()
	if !strings.Contains(got, "— all good") {
		t.Errorf("expected message appended with em dash, got %q", got)
	}
}

func TestPrintVerifyResult_NoMessageNoSeparator(t *testing.T) {
	var buf bytes.Buffer
	r := domain.VerifyResult{
		Check:  "check-b",
		Passed: true,
	}
	printVerifyResult(&buf, r)

	got := buf.String()
	if strings.Contains(got, "—") {
		t.Errorf("no message should mean no em dash separator, got %q", got)
	}
}

// ─── printGroupedResults ────────────────────────────────────────────────────

func TestPrintGroupedResults_GroupsByComponent(t *testing.T) {
	var buf bytes.Buffer
	results := []domain.VerifyResult{
		{Check: "mem-1", Passed: true, Component: "Memory"},
		{Check: "rule-1", Passed: true, Component: "Rules"},
		{Check: "mem-2", Passed: true, Component: "Memory"},
	}
	printGroupedResults(&buf, results)

	got := buf.String()

	// Verify group headers appear.
	if !strings.Contains(got, "Memory:\n") {
		t.Errorf("expected Memory group header, got %q", got)
	}
	if !strings.Contains(got, "Rules:\n") {
		t.Errorf("expected Rules group header, got %q", got)
	}

	// Memory should appear before Rules (first-appearance order).
	memIdx := strings.Index(got, "Memory:")
	ruleIdx := strings.Index(got, "Rules:")
	if memIdx > ruleIdx {
		t.Errorf("Memory should appear before Rules (first-appearance order)")
	}

	// Memory group should contain both checks.
	if strings.Count(got, "mem-") != 2 {
		t.Errorf("expected 2 mem- checks, got %q", got)
	}
}

func TestPrintGroupedResults_EmptyComponentDefaultsToGeneral(t *testing.T) {
	var buf bytes.Buffer
	results := []domain.VerifyResult{
		{Check: "generic-check", Passed: true, Component: ""},
	}
	printGroupedResults(&buf, results)

	got := buf.String()
	if !strings.Contains(got, "General:\n") {
		t.Errorf("empty component should be grouped as General, got %q", got)
	}
}

func TestPrintGroupedResults_GroupsSeparatedByBlankLine(t *testing.T) {
	var buf bytes.Buffer
	results := []domain.VerifyResult{
		{Check: "a", Passed: true, Component: "Alpha"},
		{Check: "b", Passed: true, Component: "Beta"},
	}
	printGroupedResults(&buf, results)

	got := buf.String()
	// Between first group's last line and second group header there should be a blank line.
	if !strings.Contains(got, "\n\nBeta:\n") {
		t.Errorf("groups should be separated by blank line, got %q", got)
	}
}

func TestPrintGroupedResults_SingleGroupNoLeadingBlankLine(t *testing.T) {
	var buf bytes.Buffer
	results := []domain.VerifyResult{
		{Check: "a", Passed: true, Component: "Only"},
	}
	printGroupedResults(&buf, results)

	got := buf.String()
	if strings.HasPrefix(got, "\n") {
		t.Errorf("single group should not start with blank line, got %q", got)
	}
}

// ─── printApplySummary ──────────────────────────────────────────────────────

func TestPrintApplySummary_AllWritten(t *testing.T) {
	var buf bytes.Buffer
	steps := []domain.StepResult{
		{Status: domain.StepSuccess, Action: domain.PlannedAction{Action: domain.ActionCreate}},
		{Status: domain.StepSuccess, Action: domain.PlannedAction{Action: domain.ActionUpdate}},
	}
	printApplySummary(&buf, steps)
	got := buf.String()

	for _, want := range []string{"2 action(s)", "2 written", "0 skipped", "0 failed"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q missing %q", got, want)
		}
	}
}

func TestPrintApplySummary_Mixed(t *testing.T) {
	var buf bytes.Buffer
	steps := []domain.StepResult{
		{Status: domain.StepSuccess, Action: domain.PlannedAction{Action: domain.ActionCreate}},
		{Status: domain.StepSuccess, Action: domain.PlannedAction{Action: domain.ActionSkip}},
		{Status: domain.StepFailed, Action: domain.PlannedAction{Action: domain.ActionUpdate}},
		{Status: domain.StepRolledBack, Action: domain.PlannedAction{Action: domain.ActionCreate}},
	}
	printApplySummary(&buf, steps)
	got := buf.String()

	for _, want := range []string{"4 action(s)", "1 written", "1 skipped", "2 failed"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q missing %q", got, want)
		}
	}
}

func TestPrintApplySummary_Empty(t *testing.T) {
	var buf bytes.Buffer
	printApplySummary(&buf, nil)
	got := buf.String()

	for _, want := range []string{"0 action(s)", "0 written", "0 skipped", "0 failed"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q missing %q", got, want)
		}
	}
}

// ─── printVerifySummary ─────────────────────────────────────────────────────

func TestPrintVerifySummary_AllPass(t *testing.T) {
	var buf bytes.Buffer
	results := []domain.VerifyResult{
		{Check: "a", Passed: true},
		{Check: "b", Passed: true},
	}
	printVerifySummary(&buf, results)
	got := buf.String()

	for _, want := range []string{"2 checks", "2 passed", "0 failed", "0 warnings"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q missing %q", got, want)
		}
	}
}

func TestPrintVerifySummary_Mixed(t *testing.T) {
	var buf bytes.Buffer
	results := []domain.VerifyResult{
		{Check: "a", Passed: true},
		{Check: "b", Passed: false},
		{Check: "c", Passed: false, Severity: domain.SeverityWarning},
		{Check: "d", Passed: true, Severity: domain.SeverityWarning},
	}
	printVerifySummary(&buf, results)
	got := buf.String()

	for _, want := range []string{"4 checks", "1 passed", "1 failed", "2 warnings"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q missing %q", got, want)
		}
	}
}

func TestPrintVerifySummary_Empty(t *testing.T) {
	var buf bytes.Buffer
	printVerifySummary(&buf, nil)
	got := buf.String()

	for _, want := range []string{"0 checks", "0 passed", "0 failed", "0 warnings"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q missing %q", got, want)
		}
	}
}
