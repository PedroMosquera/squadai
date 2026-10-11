package hooks

import (
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// budgetHookPrefix marks the hook commands SquadAI owns outright. Unlike user
// hooks, which are only ever added to, owned entries are rewritten and removed
// to match usage.enforcement.
const budgetHookPrefix = "squadai budget-hook "

// budgetHookTimeout keeps a stuck hook from stalling the session for Claude
// Code's 600s default; under block, onFailure turns the timeout into a block.
const budgetHookTimeout = 10

func ownedCommand(cmd string) bool {
	return strings.HasPrefix(cmd, budgetHookPrefix) || strings.HasPrefix(cmd, memoryHookPrefix)
}

// BudgetHooks returns the Claude Code hook entries that enforce usage inside
// the session, or nil when enforcement is unset or off or no token budget is
// set. block and ask guard every tool call; warn reports once per prompt and
// at the end of each turn, because a warning on every tool call is noise.
func BudgetHooks(usage domain.UsageConfig) domain.HooksConfig {
	if usage.DailyTokenBudget <= 0 && usage.SessionTokenBudget <= 0 {
		return nil
	}
	var events []string
	switch usage.Enforcement {
	case "block", "ask":
		events = []string{"PreToolUse", "UserPromptSubmit", "Stop"}
	case "warn":
		events = []string{"UserPromptSubmit", "Stop"}
	default:
		return nil
	}
	out := domain.HooksConfig{}
	for _, ev := range events {
		e := domain.HookEntry{Type: "command", Command: budgetHookPrefix + "--event " + ev, Timeout: budgetHookTimeout}
		// onFailure has no effect on Stop, and Stop never blocks on budget.
		if usage.Enforcement == "block" && ev != "Stop" {
			e.OnFailure = "block"
		}
		out[ev] = []domain.HookMatcher{{Hooks: []domain.HookEntry{e}}}
	}
	return out
}

// WithBudgetHooks returns base plus BudgetHooks(usage), leaving base unmodified.
func WithBudgetHooks(base domain.HooksConfig, usage domain.UsageConfig) domain.HooksConfig {
	budget := BudgetHooks(usage)
	if len(budget) == 0 {
		return base
	}
	out := make(domain.HooksConfig, len(base)+len(budget))
	for ev, ms := range base {
		out[ev] = append([]domain.HookMatcher(nil), ms...)
	}
	for ev, ms := range budget {
		out[ev] = append(out[ev], ms...)
	}
	return out
}
