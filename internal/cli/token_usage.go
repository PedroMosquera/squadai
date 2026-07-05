package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/session"
)

func RunTokenUsage(args []string, stdout io.Writer) error {
	jsonOut := false
	sinceStr := "7d"

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "--watch":
			fmt.Fprintln(stdout, "--watch is not yet implemented")
			return nil
		case arg == "--since":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return fmt.Errorf("--since requires a value (e.g. --since 7d)")
			}
			i++
			sinceStr = args[i]
		case arg == "-h", arg == "--help":
			fmt.Fprintln(stdout, "Usage: squadai token-usage [--since <dur>] [--json] [--watch]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Aggregate real token usage from agent session transcripts.")
			fmt.Fprintln(stdout, "Scans Claude Code, OpenCode and Pi session transcripts for token")
			fmt.Fprintln(stdout, "counts, including prompt-cache reads and writes.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --since <dur>   Time window: 7d, 30d, or all (default: 7d)")
			fmt.Fprintln(stdout, "  --json          Output as JSON")
			fmt.Fprintln(stdout, "  --watch         Tail the latest session (not yet implemented)")
			return nil
		case strings.HasPrefix(arg, "--since="):
			sinceStr = arg[len("--since="):]
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	var since time.Duration
	switch sinceStr {
	case "all":
		since = 0
	case "30d":
		since = 30 * 24 * time.Hour
	default:
		since = 7 * 24 * time.Hour
	}

	projectDir, _ := os.Getwd()

	agg, err := session.Aggregate(homeDir, session.AggregateOptions{
		Since:      since,
		ProjectDir: projectDir,
	})
	if err != nil {
		return fmt.Errorf("aggregate token usage: %w", err)
	}

	agg.Period = sinceStr

	if jsonOut {
		data, err := json.MarshalIndent(agg, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal token usage: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	printTokenUsageHuman(stdout, agg)

	// Budget footer (warn-only): compare last-24h usage against the merged
	// budget config. Config load failures just suppress the footer.
	if merged, err := loadAndMerge(homeDir, projectDir); err == nil {
		daily, err := session.Aggregate(homeDir, session.AggregateOptions{
			Since:      24 * time.Hour,
			ProjectDir: projectDir,
		})
		if err == nil {
			printBudgetFooter(stdout, merged.Usage, daily)
		}
	}
	return nil
}

func printTokenUsageHuman(w io.Writer, agg *session.Aggregation) {
	if agg.Total.SessionCount == 0 {
		fmt.Fprintf(w, "No sessions found in the last %s.\n", agg.Period)
		fmt.Fprintln(w, "Make sure Claude Code, OpenCode or Pi sessions exist in their session directories.")
		return
	}

	fmt.Fprintf(w, "Token Usage (last %s)\n", agg.Period)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-24s  %8s  %8s  %9s  %9s  %8s  %6s  %10s\n",
		"Model", "Input", "Output", "Cache-Rd", "Cache-Wr", "Total", "Sess.", "Est. Cost")
	fmt.Fprintf(w, "%-24s  %8s  %8s  %9s  %9s  %8s  %6s  %10s\n",
		"────────────────────────", "────────", "────────", "─────────", "─────────", "────────", "──────", "──────────")

	models := make([]string, 0, len(agg.ByModel))
	for m := range agg.ByModel {
		models = append(models, m)
	}
	sortModels(models)

	for _, m := range models {
		u := agg.ByModel[m]
		fmt.Fprintf(w, "%-24s  %8d  %8d  %9d  %9d  %8d  %6d  $%9.4f\n",
			m, u.InputTokens, u.OutputTokens, u.CacheReadTokens, u.CacheCreationTokens,
			u.TotalTokens, u.SessionCount, u.EstimatedCost)
	}
	fmt.Fprintf(w, "%-24s  %8s  %8s  %9s  %9s  %8s  %6s  %10s\n",
		"────────────────────────", "────────", "────────", "─────────", "─────────", "────────", "──────", "──────────")
	fmt.Fprintf(w, "%-24s  %8d  %8d  %9d  %9d  %8d  %6d  $%9.4f\n",
		"TOTAL", agg.Total.InputTokens, agg.Total.OutputTokens,
		agg.Total.CacheReadTokens, agg.Total.CacheCreationTokens,
		agg.Total.TotalTokens, agg.Total.SessionCount, agg.Total.EstimatedCost)
}

// printBudgetFooter appends a warn-only budget summary comparing the
// last-24h aggregation against the configured budgets. Suppressed when
// enforcement is off or no token budgets are configured.
func printBudgetFooter(w io.Writer, usage domain.UsageConfig, daily *session.Aggregation) {
	if usage.Enforcement == "off" || daily == nil {
		return
	}
	if usage.DailyTokenBudget <= 0 && usage.SessionTokenBudget <= 0 {
		return
	}
	fmt.Fprintln(w)
	if usage.DailyTokenBudget > 0 {
		used := daily.Total.TotalTokens
		pct := used * 100 / usage.DailyTokenBudget
		over := ""
		if used > usage.DailyTokenBudget {
			over = " — over budget"
		}
		fmt.Fprintf(w, "Daily budget:   %s / %s (%d%%)%s\n",
			formatKTokens(used), formatKTokens(usage.DailyTokenBudget), pct, over)
	}
	if usage.SessionTokenBudget > 0 {
		largest := daily.MaxSessionTokens
		pct := largest * 100 / usage.SessionTokenBudget
		over := ""
		if largest > usage.SessionTokenBudget {
			over = " — over budget"
		}
		fmt.Fprintf(w, "Session budget: largest session %s / %s (%d%%)%s\n",
			formatKTokens(largest), formatKTokens(usage.SessionTokenBudget), pct, over)
	}
}

// formatKTokens renders a token count as a compact thousands figure
// (e.g. 143.2k); counts under 1000 print as-is.
func formatKTokens(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%.1fk", float64(n)/1000)
}

func sortModels(models []string) {
	for i := 1; i < len(models); i++ {
		for j := i; j > 0 && models[j-1] > models[j]; j-- {
			models[j-1], models[j] = models[j], models[j-1]
		}
	}
}
