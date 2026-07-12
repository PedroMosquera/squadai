package doctor

import (
	"fmt"
	"time"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/session"
)

// checkTokenBudgetUsage compares the last-24h real token usage (from Claude
// Code, OpenCode and Pi session transcripts) against the configured
// DailyTokenBudget. Warn-only: over budget never fails doctor. Skipped when
// no project config, no daily budget, or enforcement is off.
func (d *Doctor) checkTokenBudgetUsage() CheckResult {
	proj, err := config.LoadProject(d.projectDir)
	if err != nil {
		return skip(catConfig, "token-budget-usage", "no project config — nothing to check")
	}
	if proj.Usage.Enforcement == "off" {
		return skip(catConfig, "token-budget-usage", "usage enforcement is off")
	}
	if proj.Usage.DailyTokenBudget <= 0 {
		return skip(catConfig, "token-budget-usage", "no daily token budget configured")
	}

	agg, err := session.Aggregate(d.homeDir, session.AggregateOptions{
		Since:      24 * time.Hour,
		ProjectDir: d.projectDir,
	})
	if err != nil {
		return skip(catConfig, "token-budget-usage",
			fmt.Sprintf("could not aggregate session usage: %v", err))
	}

	used := agg.Total.TotalTokens
	budget := proj.Usage.DailyTokenBudget
	if used > budget {
		return warn(catConfig, "token-budget-usage",
			fmt.Sprintf("last-24h token usage %d exceeds daily budget %d", used, budget),
			fmt.Sprintf("%d sessions in the window", agg.Total.SessionCount),
			"Run 'squadai apply --profile=cheap' to reduce per-session context cost")
	}
	return pass(catConfig, "token-budget-usage",
		fmt.Sprintf("last-24h token usage %d within daily budget %d", used, budget),
		fmt.Sprintf("%d sessions in the window", agg.Total.SessionCount))
}
