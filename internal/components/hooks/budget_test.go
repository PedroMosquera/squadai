package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// Hook contract these entries target (events, matcher groups, onFailure):
// https://code.claude.com/docs/en/hooks
// onFailure: "block" shipped in Claude Code 2.1.295 and PreModelSwitch in
// 2.1.251: https://github.com/anthropics/claude-code/blob/main/CHANGELOG.md

func budgetUsage(enforcement string) domain.UsageConfig {
	return domain.UsageConfig{DailyTokenBudget: 1000, SessionTokenBudget: 500, Enforcement: enforcement}
}

// budgetEntry returns the squadai budget-hook entry for event, or nil.
func budgetEntry(cfg map[string][]domain.HookMatcher, event string) *domain.HookEntry {
	for _, m := range cfg[event] {
		for i, h := range m.Hooks {
			if strings.HasPrefix(h.Command, "squadai budget-hook ") {
				return &m.Hooks[i]
			}
		}
	}
	return nil
}

func TestBudgetHooks_BlockGuardsToolsAndPromptsAndFailsClosed(t *testing.T) {
	got := BudgetHooks(budgetUsage("block"))
	for _, ev := range []string{"PreToolUse", "UserPromptSubmit"} {
		e := budgetEntry(got, ev)
		if e == nil {
			t.Fatalf("%s: no budget-hook entry in %+v", ev, got)
		}
		if e.Command != "squadai budget-hook --event "+ev {
			t.Errorf("%s command = %q", ev, e.Command)
		}
		if e.OnFailure != "block" {
			t.Errorf("%s onFailure = %q, want block", ev, e.OnFailure)
		}
	}
	// onFailure has no effect on Stop per the hooks reference, so it is omitted.
	stop := budgetEntry(got, "Stop")
	if stop == nil || stop.OnFailure != "" {
		t.Errorf("Stop entry = %+v, want present without onFailure", stop)
	}
}

func TestBudgetHooks_WarnOnlyReportsAndNeverFailsClosed(t *testing.T) {
	got := BudgetHooks(budgetUsage("warn"))
	if e := budgetEntry(got, "PreToolUse"); e != nil {
		t.Errorf("warn installs PreToolUse %+v; want none, a per-tool-call warning is noise", e)
	}
	for _, ev := range []string{"UserPromptSubmit", "Stop"} {
		e := budgetEntry(got, ev)
		if e == nil {
			t.Fatalf("%s: no budget-hook entry under warn", ev)
		}
		if e.OnFailure != "" {
			t.Errorf("%s onFailure = %q under warn, want unset", ev, e.OnFailure)
		}
	}
}

func TestBudgetHooks_NothingWithoutEnforcementOrBudget(t *testing.T) {
	for name, usage := range map[string]domain.UsageConfig{
		"enforcement unset": budgetUsage(""),
		"enforcement off":   budgetUsage("off"),
		"no token budget":   {Enforcement: "block"},
	} {
		if got := BudgetHooks(usage); len(got) != 0 {
			t.Errorf("%s: got %+v, want no hooks", name, got)
		}
	}
}

func readHooks(t *testing.T, dir string) (map[string]any, map[string][]domain.HookMatcher) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc, claude.DecodeHooksMap(doc["hooks"])
}

func applyDesired(t *testing.T, dir string, desired domain.HooksConfig) {
	t.Helper()
	inst := New(desired)
	actions, err := inst.Plan(claude.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatal(err)
		}
	}
}

func TestApply_BlockBudgetHooksKeepUserHookIntact(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{map[string]any{
				"matcher": "Bash",
				"hooks": []any{map[string]any{
					"type": "command", "command": "./guard.sh", "if": "Bash(rm *)", "statusMessage": "guarding",
				}},
			}},
		},
	})

	applyDesired(t, dir, WithBudgetHooks(nil, budgetUsage("block")))

	doc, got := readHooks(t, dir)
	if e := budgetEntry(got, "PreToolUse"); e == nil || e.OnFailure != "block" {
		t.Fatalf("PreToolUse budget entry = %+v, want onFailure block", e)
	}
	raw, _ := json.Marshal(doc["hooks"])
	for _, want := range []string{`"command":"./guard.sh"`, `"if":"Bash(rm *)"`, `"statusMessage":"guarding"`, `"matcher":"Bash"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("user hook lost %s after apply:\n%s", want, raw)
		}
	}
}

func TestApply_EnforcementChangeRewritesOnlyOwnedEntries(t *testing.T) {
	dir := t.TempDir()
	user := domain.HooksConfig{"Stop": {{Hooks: []domain.HookEntry{{Type: "command", Command: "./notify.sh"}}}}}
	applyDesired(t, dir, WithBudgetHooks(user, budgetUsage("block")))

	applyDesired(t, dir, WithBudgetHooks(user, budgetUsage("warn")))
	_, got := readHooks(t, dir)
	if e := budgetEntry(got, "PreToolUse"); e != nil {
		t.Errorf("block-only PreToolUse entry survived the switch to warn: %+v", e)
	}
	if e := budgetEntry(got, "UserPromptSubmit"); e == nil || e.OnFailure != "" {
		t.Errorf("UserPromptSubmit entry = %+v, want present without onFailure", e)
	}

	applyDesired(t, dir, WithBudgetHooks(user, budgetUsage("")))
	_, got = readHooks(t, dir)
	for _, ev := range []string{"PreToolUse", "UserPromptSubmit", "Stop"} {
		if e := budgetEntry(got, ev); e != nil {
			t.Errorf("%s budget entry survived unsetting enforcement: %+v", ev, e)
		}
	}
	if !claude.HasCommand(got["Stop"][0].Hooks, "./notify.sh") {
		t.Errorf("user Stop hook removed: %+v", got["Stop"])
	}
}
