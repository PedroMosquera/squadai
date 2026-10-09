package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/session"
)

func TestEvaluateBudgets(t *testing.T) {
	agg := func(total, maxSession int) *session.Aggregation {
		return &session.Aggregation{Total: session.Usage{TotalTokens: total}, MaxSessionTokens: maxSession}
	}
	tests := []struct {
		name      string
		usage     domain.UsageConfig
		daily     *session.Aggregation
		wantNil   bool
		wantKinds []string
		wantOver  []bool
	}{
		{name: "no token budgets", usage: domain.UsageConfig{Enforcement: "block"}, daily: agg(100_000, 10_000), wantNil: true},
		{name: "nil aggregation", usage: domain.UsageConfig{DailyTokenBudget: 1, Enforcement: "block"}, daily: nil, wantNil: true},
		{name: "daily under", usage: domain.UsageConfig{DailyTokenBudget: 100_000}, daily: agg(50_000, 10_000), wantKinds: []string{"daily"}, wantOver: []bool{false}},
		{name: "daily exactly at limit is not over", usage: domain.UsageConfig{DailyTokenBudget: 100_000}, daily: agg(100_000, 10_000), wantKinds: []string{"daily"}, wantOver: []bool{false}},
		{name: "daily over", usage: domain.UsageConfig{DailyTokenBudget: 100_000}, daily: agg(150_000, 10_000), wantKinds: []string{"daily"}, wantOver: []bool{true}},
		{name: "session uses largest session not total", usage: domain.UsageConfig{SessionTokenBudget: 50_000}, daily: agg(500_000, 20_000), wantKinds: []string{"session"}, wantOver: []bool{false}},
		{name: "session over", usage: domain.UsageConfig{SessionTokenBudget: 50_000}, daily: agg(10_000, 60_000), wantKinds: []string{"session"}, wantOver: []bool{true}},
		{name: "both set, daily under session over", usage: domain.UsageConfig{DailyTokenBudget: 100_000, SessionTokenBudget: 50_000}, daily: agg(80_000, 60_000), wantKinds: []string{"daily", "session"}, wantOver: []bool{false, true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluateBudgets(tc.usage, tc.daily)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("want nil report, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("want report, got nil")
			}
			if len(got.Findings) != len(tc.wantKinds) {
				t.Fatalf("got %d findings, want %d: %+v", len(got.Findings), len(tc.wantKinds), got.Findings)
			}
			for i, f := range got.Findings {
				if f.Kind != tc.wantKinds[i] || f.Over != tc.wantOver[i] {
					t.Errorf("findings[%d] = %+v, want kind %q over %v", i, f, tc.wantKinds[i], tc.wantOver[i])
				}
			}
		})
	}
}

// A project.json that sets budgets but omits enforcement replaces the whole
// usage block on merge, leaving enforcement empty. That must enforce as warn,
// not silently skip.
func TestEvaluateBudgets_EmptyEnforcementIsWarn(t *testing.T) {
	daily := &session.Aggregation{Total: session.Usage{TotalTokens: 200_000}}
	got := evaluateBudgets(domain.UsageConfig{DailyTokenBudget: 100_000}, daily)
	if got == nil || got.Enforcement != "warn" {
		t.Fatalf("enforcement = %+v, want warn", got)
	}
	for _, explicit := range []string{"off", "warn", "ask", "block"} {
		got := evaluateBudgets(domain.UsageConfig{DailyTokenBudget: 100_000, Enforcement: explicit}, daily)
		if got.Enforcement != explicit {
			t.Errorf("enforcement %q normalized to %q, want unchanged", explicit, got.Enforcement)
		}
	}
}

func TestRunTokenUsage_EmptyEnforcementReportsWarn(t *testing.T) {
	setupBudgetEnv(t, map[string]any{"daily_token_budget": 1000})
	var stdout bytes.Buffer
	if err := RunTokenUsage([]string{"--json", "--against-budget"}, &stdout); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Budget *budgetReport `json:"budget"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if out.Budget == nil || out.Budget.Enforcement != "warn" {
		t.Errorf("budget = %+v, want enforcement warn", out.Budget)
	}
}

func TestApplyBudgetEnforcement(t *testing.T) {
	over := []budgetFinding{{Kind: "daily", Used: 200_000, Limit: 100_000, Over: true}}
	under := []budgetFinding{{Kind: "daily", Used: 50_000, Limit: 100_000, Over: false}}
	tests := []struct {
		name        string
		report      *budgetReport
		wantCode    int
		wantStderr  string
		emptyStderr bool
	}{
		{name: "nil report is a no-op", report: nil, emptyStderr: true},
		{name: "off + over", report: &budgetReport{Enforcement: "off", Findings: over}, emptyStderr: true},
		{name: "warn + under", report: &budgetReport{Enforcement: "warn", Findings: under}, emptyStderr: true},
		{name: "warn + over warns", report: &budgetReport{Enforcement: "warn", Findings: over}, wantStderr: "daily token budget exceeded (200000 / 100000); enforcement: warn"},
		{name: "ask + over warns because the command is non-interactive", report: &budgetReport{Enforcement: "ask", Findings: over}, wantStderr: "enforcement: ask"},
		{name: "block + under", report: &budgetReport{Enforcement: "block", Findings: under}, emptyStderr: true},
		{name: "block + over fails with budget code", report: &budgetReport{Enforcement: "block", Findings: over}, wantCode: exitcode.Budget},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			err := applyBudgetEnforcement(&stderr, tc.report)
			if tc.wantCode == 0 && err != nil {
				t.Fatalf("want nil error, got %v", err)
			}
			if tc.wantCode != 0 {
				var ae *exitcode.AppError
				if !errors.As(err, &ae) || ae.Code != tc.wantCode {
					t.Fatalf("want AppError code %d, got %v", tc.wantCode, err)
				}
			}
			if tc.emptyStderr && stderr.Len() != 0 {
				t.Errorf("want empty stderr, got %q", stderr.String())
			}
			if tc.wantStderr != "" && !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stderr %q missing %q", stderr.String(), tc.wantStderr)
			}
		})
	}
}

// setupBudgetEnv points HOME at a temp dir holding one fresh 1500-token
// OpenCode session and chdirs into a project whose project.json carries usage.
// usage == nil writes no project config at all.
func setupBudgetEnv(t *testing.T, usage map[string]any) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	sessDir := filepath.Join(home, ".local/share/opencode/sessions")
	if err := os.MkdirAll(sessDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessDir, "s1.json"),
		[]byte(`{"model":"gpt-4o","usage":{"input_tokens":1000,"output_tokens":500}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	if usage != nil {
		data, err := json.Marshal(map[string]any{"usage": usage})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(project, ".squadai"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(project, ".squadai", "project.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(project)
}

func TestRunTokenUsage_AgainstBudget(t *testing.T) {
	overBlock := map[string]any{"daily_token_budget": 1000, "enforcement": "block"}
	tests := []struct {
		name     string
		usage    map[string]any
		args     []string
		wantCode int
	}{
		{name: "block + over exits with budget code", usage: overBlock, args: []string{"--against-budget"}, wantCode: exitcode.Budget},
		{name: "block + over exits with budget code in JSON mode", usage: overBlock, args: []string{"--json", "--against-budget"}, wantCode: exitcode.Budget},
		{name: "block + over without the flag stays advisory", usage: overBlock, args: []string{"--json"}},
		{name: "block + under passes", usage: map[string]any{"daily_token_budget": 100_000, "enforcement": "block"}, args: []string{"--against-budget"}},
		{name: "warn + over passes", usage: map[string]any{"daily_token_budget": 1000, "enforcement": "warn"}, args: []string{"--against-budget"}},
		{name: "no config passes", usage: nil, args: []string{"--against-budget"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setupBudgetEnv(t, tc.usage)
			var stdout bytes.Buffer
			err := RunTokenUsage(tc.args, &stdout)
			if tc.wantCode == 0 {
				if err != nil {
					t.Fatalf("want nil error, got %v", err)
				}
				return
			}
			var ae *exitcode.AppError
			if !errors.As(err, &ae) || ae.Code != tc.wantCode {
				t.Fatalf("want AppError code %d, got %v", tc.wantCode, err)
			}
		})
	}
}

func TestRunTokenUsage_JSONBudgetField(t *testing.T) {
	t.Run("present with findings when budgets are configured", func(t *testing.T) {
		setupBudgetEnv(t, map[string]any{"daily_token_budget": 1000, "enforcement": "block"})
		var stdout bytes.Buffer
		_ = RunTokenUsage([]string{"--json", "--against-budget"}, &stdout)
		var out struct {
			Total  session.Usage `json:"total"`
			Budget *budgetReport `json:"budget"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
			t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
		}
		if out.Total.TotalTokens != 1500 {
			t.Errorf("total_tokens = %d, want 1500 (existing fields must survive)", out.Total.TotalTokens)
		}
		if out.Budget == nil {
			t.Fatalf("budget field missing:\n%s", stdout.String())
		}
		want := budgetFinding{Kind: "daily", Used: 1500, Limit: 1000, Over: true}
		if out.Budget.Enforcement != "block" || len(out.Budget.Findings) != 1 || out.Budget.Findings[0] != want {
			t.Errorf("budget = %+v, want enforcement block with %+v", out.Budget, want)
		}
	})
	t.Run("omitted when no budgets are configured", func(t *testing.T) {
		setupBudgetEnv(t, nil)
		var stdout bytes.Buffer
		if err := RunTokenUsage([]string{"--json"}, &stdout); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(stdout.String(), `"budget"`) {
			t.Errorf("budget field should be omitted:\n%s", stdout.String())
		}
	})
}
