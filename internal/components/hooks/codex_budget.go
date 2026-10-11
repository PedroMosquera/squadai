package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	claudeadapter "github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
)

const codexBudgetActionID = "codex-budget-hooks"

// CodexHooksPath is the project hooks file Codex loads once the project is
// trusted. It takes the Claude Code hooks schema (codex-rs/config/src/hook_config.rs
// in github.com/openai/codex), so the Claude reconcile helpers apply as is.
func CodexHooksPath(projectDir string) string {
	return filepath.Join(projectDir, ".codex", "hooks.json")
}

// CodexBudgetHooks returns the Codex entries enforcing usage, or nil under the
// same rule as BudgetHooks. Codex rejects permissionDecision "ask", so ask
// installs the warn set, and it has no onFailure, so a hook that fails allows.
func CodexBudgetHooks(usage domain.UsageConfig) domain.HooksConfig {
	claude := BudgetHooks(usage)
	if len(claude) == 0 {
		return nil
	}
	out := domain.HooksConfig{}
	for ev := range claude {
		if ev == "PreToolUse" && usage.Enforcement != "block" {
			continue
		}
		e := domain.HookEntry{Type: "command", Command: budgetHookPrefix + "--harness codex --event " + ev, Timeout: budgetHookTimeout}
		out[ev] = []domain.HookMatcher{{Hooks: []domain.HookEntry{e}}}
	}
	return out
}

// renderCodexHooks returns hooks.json with the budget entries reconciled and
// user entries kept. A nil result with changed set means nothing is left and
// the file should go.
func (i *Installer) renderCodexHooks(path string) (out []byte, changed bool, err error) {
	data, err := fileutil.ReadFileOrEmpty(path)
	if err != nil {
		return nil, false, fmt.Errorf("read codex hooks: %w", err)
	}
	doc := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, false, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	merged, changed := claudeadapter.ReconcileHooks(claudeadapter.DecodeHooksMap(doc["hooks"]), CodexBudgetHooks(i.usage), ownedCommand)
	if !changed {
		return data, false, nil
	}
	if len(merged) == 0 {
		delete(doc, "hooks")
	} else {
		doc["hooks"] = claudeadapter.EncodeHooksMap(merged)
	}
	if len(doc) == 0 {
		return nil, true, nil
	}
	out, err = json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, false, fmt.Errorf("marshal codex hooks: %w", err)
	}
	return append(out, '\n'), true, nil
}

func (i *Installer) planCodexBudget(projectDir string) ([]domain.PlannedAction, error) {
	path := CodexHooksPath(projectDir)
	out, changed, err := i.renderCodexHooks(path)
	if err != nil {
		return nil, err
	}
	desired := CodexBudgetHooks(i.usage)
	if !changed && len(desired) == 0 {
		return nil, nil
	}
	action := domain.PlannedAction{ID: codexBudgetActionID, Agent: domain.AgentCodex, Component: domain.ComponentHooks, TargetPath: path}
	_, statErr := os.Stat(path)
	switch {
	case !changed:
		action.Action = domain.ActionSkip
		action.Description = "codex budget hooks already installed"
	case out == nil:
		action.Action = domain.ActionDelete
		action.Description = "remove SquadAI budget hooks and the now empty .codex/hooks.json"
	case len(desired) == 0:
		action.Action = domain.ActionUpdate
		action.Description = "remove SquadAI budget hooks from .codex/hooks.json"
	case os.IsNotExist(statErr):
		action.Action = domain.ActionCreate
		action.Description = fmt.Sprintf("merge %d budget hook event(s) into .codex/hooks.json", len(desired))
	default:
		action.Action = domain.ActionUpdate
		action.Description = fmt.Sprintf("merge %d budget hook event(s) into .codex/hooks.json", len(desired))
	}
	return []domain.PlannedAction{action}, nil
}

func (i *Installer) applyCodexBudget(action domain.PlannedAction) error {
	out, changed, err := i.renderCodexHooks(action.TargetPath)
	if err != nil || !changed {
		return err
	}
	if out == nil {
		if err := os.Remove(action.TargetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove codex hooks: %w", err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(action.TargetPath), 0o755); err != nil {
		return fmt.Errorf("create .codex dir: %w", err)
	}
	if _, err := fileutil.WriteAtomic(action.TargetPath, out, 0o644); err != nil {
		return fmt.Errorf("write codex hooks: %w", err)
	}
	return nil
}

func (i *Installer) verifyCodexBudget(projectDir string) []domain.VerifyResult {
	if len(CodexBudgetHooks(i.usage)) == 0 {
		return nil
	}
	r := domain.VerifyResult{Check: "codex-budget-hooks", Passed: true, Severity: domain.SeverityInfo, Component: string(domain.ComponentHooks)}
	if _, changed, err := i.renderCodexHooks(CodexHooksPath(projectDir)); err != nil || changed {
		r.Passed, r.Severity = false, domain.SeverityError
		r.Message = "SquadAI budget hooks in .codex/hooks.json are missing or out of date"
	}
	return []domain.VerifyResult{r}
}
