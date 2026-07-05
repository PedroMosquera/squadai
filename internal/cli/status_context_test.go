package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/planner/budget"
)

func TestCollectContextHealth_WithAppliedBudget(t *testing.T) {
	projectDir := t.TempDir()
	fit := &budget.FitResult{
		Decisions: []budget.ComponentDecision{
			{Component: domain.ComponentAgents, Mode: budget.ModeFull, Tokens: 3000},
			{Component: domain.ComponentMemory, Mode: budget.ModeSummary, Tokens: 800},
			{Component: domain.ComponentSkills, Mode: budget.ModeOmit, Tokens: 2000},
		},
		TotalTokens: 3400,
		Cap:         6000,
		Profile:     "cheap",
		FitAchieved: true,
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ".squadai"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := budget.Persist(projectDir, fit); err != nil {
		t.Fatal(err)
	}

	merged := &domain.MergedConfig{}
	merged.Context.DefaultProfile = "cheap"
	merged.Context.Profiles = map[string]domain.ContextProfile{
		"cheap": {MaxApproxTokens: 6000},
	}

	ch := collectContextHealth(t.TempDir(), projectDir, merged, false)
	if ch.Profile != "cheap" || ch.TokenCap != 6000 {
		t.Errorf("profile/cap = %s/%d, want cheap/6000", ch.Profile, ch.TokenCap)
	}
	if ch.Source != contextSourceBudget || ch.InstalledTokens != 3400 {
		t.Errorf("source/tokens = %s/%d, want applied-budget/3400", ch.Source, ch.InstalledTokens)
	}
	if ch.FullComponents != 1 || ch.SummaryComponents != 1 || ch.OmittedComponents != 1 {
		t.Errorf("fit counts = %d/%d/%d, want 1/1/1",
			ch.FullComponents, ch.SummaryComponents, ch.OmittedComponents)
	}

	var buf bytes.Buffer
	printContextSection(&buf, ch)
	out := buf.String()
	for _, want := range []string{"Context:", "cheap (cap 6.0k tokens)", "3.4k tokens (from applied budget)", "1 full / 1 summary / 1 omitted"} {
		if !strings.Contains(out, want) {
			t.Errorf("section missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Last 7d use") {
		t.Errorf("usage line should be absent without includeUsage:\n%s", out)
	}
}

func TestCollectContextHealth_ScanFallback(t *testing.T) {
	projectDir := t.TempDir()
	// No .applied-budget.json; a managed sidecar points at one file.
	if err := os.MkdirAll(filepath.Join(projectDir, ".squadai"), 0755); err != nil {
		t.Fatal(err)
	}
	sidecar := `{"managed_files":{"CLAUDE.md":{"managed_keys":[]}}}`
	if err := os.WriteFile(filepath.Join(projectDir, ".squadai", "managed.json"), []byte(sidecar), 0644); err != nil {
		t.Fatal(err)
	}
	// 400 bytes -> 100 approx tokens.
	if err := os.WriteFile(filepath.Join(projectDir, "CLAUDE.md"), bytes.Repeat([]byte("a"), 400), 0644); err != nil {
		t.Fatal(err)
	}

	ch := collectContextHealth(t.TempDir(), projectDir, &domain.MergedConfig{}, false)
	if ch.Source != contextSourceScan {
		t.Errorf("Source = %s, want scan", ch.Source)
	}
	if ch.InstalledTokens != 100 {
		t.Errorf("InstalledTokens = %d, want 100", ch.InstalledTokens)
	}
	if ch.HasFit {
		t.Error("HasFit = true, want false without applied budget")
	}
	if ch.Profile != "default" {
		t.Errorf("Profile = %q, want default", ch.Profile)
	}

	var buf bytes.Buffer
	printContextSection(&buf, ch)
	out := buf.String()
	for _, want := range []string{"Context:", "default (no token cap)", "fast scan of managed files"} {
		if !strings.Contains(out, want) {
			t.Errorf("section missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Fit:") {
		t.Errorf("fit line should be absent without applied budget:\n%s", out)
	}
}

func TestCollectContextHealth_IncludeUsage(t *testing.T) {
	home := t.TempDir()
	projectDir := t.TempDir()
	dir := filepath.Join(home, ".local/share/opencode/sessions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `{"model":"claude-sonnet-4-6","usage":{"input_tokens":700,"output_tokens":300}}`
	if err := os.WriteFile(filepath.Join(dir, "s1.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ch := collectContextHealth(home, projectDir, &domain.MergedConfig{}, true)
	if !ch.HasUsage || ch.Last7dTokens != 1000 {
		t.Errorf("usage = %v/%d, want true/1000", ch.HasUsage, ch.Last7dTokens)
	}

	var buf bytes.Buffer
	printContextSection(&buf, ch)
	if !strings.Contains(buf.String(), "Last 7d use: 1.0k tokens") {
		t.Errorf("section missing usage line:\n%s", buf.String())
	}
}
