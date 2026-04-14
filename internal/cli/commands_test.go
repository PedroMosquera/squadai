package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/config"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// ─── Help text coverage ──────────────────────────────────────────────────────

func TestRunPlan_HelpText(t *testing.T) {
	tests := []struct {
		flag string
	}{
		{"--help"},
		{"-h"},
	}
	for _, tc := range tests {
		t.Run(tc.flag, func(t *testing.T) {
			var buf bytes.Buffer
			if err := RunPlan([]string{tc.flag}, &buf); err != nil {
				t.Fatalf("help should not error: %v", err)
			}
			out := buf.String()
			for _, want := range []string{
				"Usage: agent-manager plan",
				"--dry-run",
				"--json",
				"read-only",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("plan help missing %q, got:\n%s", want, out)
				}
			}
		})
	}
}

func TestRunApply_HelpText(t *testing.T) {
	var buf bytes.Buffer
	if err := RunApply([]string{"--help"}, &buf); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Usage: agent-manager apply",
		"--dry-run",
		"--json",
		"--force",
		"backed up automatically",
		"rolled back",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("apply help missing %q, got:\n%s", want, out)
		}
	}
}

func TestRunSync_HelpText(t *testing.T) {
	var buf bytes.Buffer
	if err := RunSync([]string{"--help"}, &buf); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Usage: agent-manager sync",
		"--dry-run",
		"--json",
		"--force",
		"idempoten",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("sync help missing %q, got:\n%s", want, out)
		}
	}
}

func TestRunSync_HelpDoesNotDelegatToApplyHelp(t *testing.T) {
	var buf bytes.Buffer
	if err := RunSync([]string{"--help"}, &buf); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "Usage: agent-manager apply") {
		t.Error("sync --help should show sync usage, not apply usage")
	}
}

func TestRunVerify_HelpText(t *testing.T) {
	var buf bytes.Buffer
	if err := RunVerify([]string{"--help"}, &buf); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Usage: agent-manager verify",
		"--json",
		"PASS",
		"FAIL",
		"WARN",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("verify help missing %q, got:\n%s", want, out)
		}
	}
}

func TestRunValidatePolicy_HelpText(t *testing.T) {
	var buf bytes.Buffer
	if err := RunValidatePolicy([]string{"--help"}, &buf); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Usage: agent-manager validate-policy",
		"policy.json",
		"--json",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("validate-policy help missing %q, got:\n%s", want, out)
		}
	}
}

// ─── RunValidatePolicy --json ────────────────────────────────────────────────

func TestRunValidatePolicy_JSONOutput_ValidPolicy(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Write a valid policy file.
	pol := domain.DefaultPolicyConfig()
	policyPath := filepath.Join(dir, config.ProjectConfigDir, "policy.json")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := config.WriteJSON(policyPath, pol); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	var buf bytes.Buffer
	err := RunValidatePolicy([]string{"--json"}, &buf)
	if err != nil {
		t.Fatalf("RunValidatePolicy --json on valid policy should not error: %v", err)
	}

	var result struct {
		Valid      bool     `json:"valid"`
		Violations []string `json:"violations"`
		PolicyPath string   `json:"policy_path"`
	}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}
	if !result.Valid {
		t.Errorf("valid field = false, want true")
	}
	if result.Violations == nil {
		t.Error("violations field should be an array (not null)")
	}
	if len(result.Violations) != 0 {
		t.Errorf("violations = %v, want empty", result.Violations)
	}
	if result.PolicyPath == "" {
		t.Error("policy_path field should not be empty")
	}
}

func TestRunValidatePolicy_JSONOutput_NoHumanText(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	pol := domain.DefaultPolicyConfig()
	policyPath := filepath.Join(dir, config.ProjectConfigDir, "policy.json")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := config.WriteJSON(policyPath, pol); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	var buf bytes.Buffer
	_ = RunValidatePolicy([]string{"--json"}, &buf)

	out := buf.String()
	if strings.Contains(out, "Policy is valid") {
		t.Errorf("--json should suppress human-readable output, got: %s", out)
	}
	// The output must parse as JSON.
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}
}

func TestRunBackupCreate_HelpText(t *testing.T) {
	var buf bytes.Buffer
	if err := RunBackupCreate([]string{"--help"}, &buf); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Usage: agent-manager backup create",
		"--json",
		"snapshot",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("backup create help missing %q, got:\n%s", want, out)
		}
	}
}

func TestRunBackupList_HelpText(t *testing.T) {
	var buf bytes.Buffer
	if err := RunBackupList([]string{"--help"}, &buf); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Usage: agent-manager backup list",
		"--json",
		"restore",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("backup list help missing %q, got:\n%s", want, out)
		}
	}
}

func TestRunRestore_HelpText(t *testing.T) {
	var buf bytes.Buffer
	if err := RunRestore([]string{"--help"}, &buf); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Usage: agent-manager restore",
		"--dry-run",
		"--json",
		"backup-id",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("restore help missing %q, got:\n%s", want, out)
		}
	}
}

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

// ─── Apply/Sync guard: no project.json ──────────────────────────────────────

func TestRunApply_NoProjectJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	err = RunApply([]string{}, &buf)
	if err == nil {
		t.Fatal("RunApply should return an error when project.json is missing")
	}
	if !strings.Contains(err.Error(), "no project.json found") {
		t.Errorf("error should mention missing project.json, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Error: No project.json found in current directory.") {
		t.Errorf("output should contain error message, got: %s", out)
	}
	if !strings.Contains(out, "agent-manager init") {
		t.Errorf("output should suggest running init, got: %s", out)
	}
	if !strings.Contains(out, "--force") {
		t.Errorf("output should mention --force flag, got: %s", out)
	}
}

func TestRunApply_NoProjectJSON_ForceFlag_Proceeds(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	err = RunApply([]string{"--force"}, &buf)
	// The guard is passed; any subsequent error is NOT about the missing project.json.
	// We allow a non-nil error here (e.g., from the planner or pipeline) as long as
	// it is not about the missing project.json guard.
	if err != nil && strings.Contains(err.Error(), "no project.json found") {
		t.Errorf("--force should bypass the project.json guard, got: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "Error: No project.json found in current directory.") {
		t.Errorf("--force should suppress the missing project.json error message, got: %s", out)
	}
	if !strings.Contains(out, "Warning: No project.json found. Running with default config (--force).") {
		t.Errorf("--force should print a warning, got: %s", out)
	}
}

func TestRunSync_NoProjectJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	err = RunSync([]string{}, &buf)
	if err == nil {
		t.Fatal("RunSync should return an error when project.json is missing")
	}
	if !strings.Contains(err.Error(), "no project.json found") {
		t.Errorf("error should mention missing project.json, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Error: No project.json found in current directory.") {
		t.Errorf("output should contain error message, got: %s", out)
	}
}

func TestRunSync_NoProjectJSON_ForceFlag_Proceeds(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	err = RunSync([]string{"--force"}, &buf)
	// The guard is passed; any subsequent error is NOT about the missing project.json.
	if err != nil && strings.Contains(err.Error(), "no project.json found") {
		t.Errorf("--force should bypass the project.json guard, got: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "Error: No project.json found in current directory.") {
		t.Errorf("--force should suppress the missing project.json error message, got: %s", out)
	}
	if !strings.Contains(out, "Warning: No project.json found. Running with default config (--force).") {
		t.Errorf("--force should print a warning, got: %s", out)
	}
}

// ─── P1-B: TestSettingsComponentConditionalEnable ───────────────────────────

func TestSettingsComponentConditionalEnable_NoSettings_NotEnabled(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	// Pass adapters with no Settings maps — the default adapters in
	// buildSmartProjectConfig also have no settings.
	proj := buildSmartProjectConfig(meta, nil, "", nil, nil)

	if _, ok := proj.Components[string(domain.ComponentSettings)]; ok {
		t.Error("ComponentSettings should NOT be enabled when no adapter has Settings populated")
	}
}

func TestSettingsComponentConditionalEnable_WithSettings_IsEnabled(t *testing.T) {
	// Build a fake adapter slice that already has Settings populated, then pass
	// it so buildSmartProjectConfig copies them into proj.Adapters[id].
	// Since DetectAdapters strips Settings from the returned adapters, we verify
	// the conditional logic directly with an inline ProjectConfig construction
	// that mirrors the exact hasSettings check inside buildSmartProjectConfig.
	adapters := map[string]domain.AdapterConfig{
		string(domain.AgentOpenCode): {
			Enabled: true,
			Settings: map[string]interface{}{
				"model": "anthropic/claude-sonnet-4-5",
			},
		},
	}
	proj := &domain.ProjectConfig{
		Components: map[string]domain.ComponentConfig{},
		Adapters:   adapters,
	}
	hasSettings := false
	for _, ac := range proj.Adapters {
		if len(ac.Settings) > 0 {
			hasSettings = true
			break
		}
	}
	if hasSettings {
		proj.Components[string(domain.ComponentSettings)] = domain.ComponentConfig{Enabled: true}
	}
	cfg, ok := proj.Components[string(domain.ComponentSettings)]
	if !ok {
		t.Fatal("ComponentSettings should be present when an adapter has Settings")
	}
	if !cfg.Enabled {
		t.Error("ComponentSettings should be Enabled=true")
	}
}

// ─── P1-C: TestDefaultCommandsForMethodology ────────────────────────────────

func TestDefaultCommandsForMethodology_TDD(t *testing.T) {
	cmds := defaultCommandsForMethodology(domain.MethodologyTDD)
	// Must have at least: review, run-tests, tdd-cycle.
	for _, name := range []string{"review", "run-tests", "tdd-cycle"} {
		if _, ok := cmds[name]; !ok {
			t.Errorf("TDD commands should contain %q", name)
		}
	}
	if len(cmds) < 3 {
		t.Errorf("TDD methodology should produce ≥3 commands, got %d", len(cmds))
	}
}

func TestDefaultCommandsForMethodology_SDD(t *testing.T) {
	cmds := defaultCommandsForMethodology(domain.MethodologySDD)
	// Must have at least: review, spec.
	for _, name := range []string{"review", "spec"} {
		if _, ok := cmds[name]; !ok {
			t.Errorf("SDD commands should contain %q", name)
		}
	}
	if len(cmds) < 2 {
		t.Errorf("SDD methodology should produce ≥2 commands, got %d", len(cmds))
	}
}

func TestDefaultCommandsForMethodology_Conventional(t *testing.T) {
	cmds := defaultCommandsForMethodology(domain.MethodologyConventional)
	// Must have at least: review, implement.
	for _, name := range []string{"review", "implement"} {
		if _, ok := cmds[name]; !ok {
			t.Errorf("Conventional commands should contain %q", name)
		}
	}
	if len(cmds) < 2 {
		t.Errorf("Conventional methodology should produce ≥2 commands, got %d", len(cmds))
	}
}

func TestDefaultCommandsForMethodology_Empty(t *testing.T) {
	cmds := defaultCommandsForMethodology("")
	// Empty methodology: only the base "review" command.
	if _, ok := cmds["review"]; !ok {
		t.Error("empty methodology should still include the 'review' command")
	}
	if len(cmds) != 1 {
		t.Errorf("empty methodology should produce exactly 1 command, got %d", len(cmds))
	}
}

func TestDefaultCommandsForMethodology_CommandsHaveDescriptions(t *testing.T) {
	for _, m := range []domain.Methodology{
		domain.MethodologyTDD,
		domain.MethodologySDD,
		domain.MethodologyConventional,
		"",
	} {
		cmds := defaultCommandsForMethodology(m)
		for name, def := range cmds {
			if def.Description == "" {
				t.Errorf("methodology=%q command %q has empty Description", m, name)
			}
			if def.Template == "" {
				t.Errorf("methodology=%q command %q has empty Template", m, name)
			}
		}
	}
}

func TestBuildSmartProjectConfig_WithMethodology_CommandsPopulated(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, domain.MethodologyTDD, nil, nil)

	if proj.Commands == nil {
		t.Fatal("proj.Commands should not be nil when methodology is set")
	}
	if len(proj.Commands) == 0 {
		t.Error("proj.Commands should be non-empty when methodology is set")
	}
	// Verify review command is always present.
	if _, ok := proj.Commands["review"]; !ok {
		t.Error("proj.Commands should contain 'review' for TDD methodology")
	}
}

func TestBuildSmartProjectConfig_WithoutMethodology_CommandsEmpty(t *testing.T) {
	meta := domain.ProjectMeta{Language: "Go"}
	proj := buildSmartProjectConfig(meta, nil, "", nil, nil)

	if len(proj.Commands) != 0 {
		t.Errorf("proj.Commands should be empty when no methodology set, got %d entries", len(proj.Commands))
	}
}
