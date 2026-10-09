package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/session"
)

// budgetFinding compares one token metric against its configured limit.
// Kind is "daily" (24h total) or "session" (largest single session in 24h).
type budgetFinding struct {
	Kind  string `json:"kind"`
	Used  int    `json:"used"`
	Limit int    `json:"limit"`
	Over  bool   `json:"over"`
}

// budgetReport is also the shape of the additive "budget" field in
// token-usage --json output; renaming its tags breaks JSON consumers.
type budgetReport struct {
	Enforcement string          `json:"enforcement"`
	Findings    []budgetFinding `json:"findings"`
}

// evaluateBudgets compares the last-24h aggregation against the configured
// token budgets. It returns nil when no token budget is set or daily is nil,
// so callers can treat nil as "nothing to report or enforce".
func evaluateBudgets(usage domain.UsageConfig, daily *session.Aggregation) *budgetReport {
	if daily == nil || (usage.DailyTokenBudget <= 0 && usage.SessionTokenBudget <= 0) {
		return nil
	}
	// A project.json that sets budgets without enforcement replaces the merged
	// usage block wholesale, leaving it empty; treat that as the default warn.
	enforcement := usage.Enforcement
	if enforcement == "" {
		enforcement = "warn"
	}
	r := &budgetReport{Enforcement: enforcement, Findings: []budgetFinding{}}
	if usage.DailyTokenBudget > 0 {
		used := daily.Total.TotalTokens
		r.Findings = append(r.Findings, budgetFinding{
			Kind: "daily", Used: used, Limit: usage.DailyTokenBudget, Over: used > usage.DailyTokenBudget,
		})
	}
	if usage.SessionTokenBudget > 0 {
		used := daily.MaxSessionTokens
		r.Findings = append(r.Findings, budgetFinding{
			Kind: "session", Used: used, Limit: usage.SessionTokenBudget, Over: used > usage.SessionTokenBudget,
		})
	}
	return r
}

// applyBudgetEnforcement enforces b for --against-budget. block with any
// finding over returns ErrBudgetExceeded (exit code exitcode.Budget); warn and
// ask write one stderr line per exceeded budget and return nil. ask cannot
// prompt here because token-usage is a non-interactive report, so it acts as
// warn. Nothing is ever written to stdout, keeping --json output parseable.
func applyBudgetEnforcement(stderr io.Writer, b *budgetReport) error {
	if b == nil {
		return nil
	}
	var over []string
	for _, f := range b.Findings {
		if f.Over {
			over = append(over, f.Kind)
		}
	}
	if len(over) == 0 {
		return nil
	}
	switch b.Enforcement {
	case "block":
		return exitcode.ErrBudgetExceeded(strings.Join(over, ", "))
	case "warn", "ask":
		for _, f := range b.Findings {
			if f.Over {
				fmt.Fprintf(stderr, "warning: %s token budget exceeded (%d / %d); enforcement: %s\n",
					f.Kind, f.Used, f.Limit, b.Enforcement)
			}
		}
	}
	return nil
}
