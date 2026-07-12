package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/PedroMosquera/squadai/internal/tokenprofile/pricing"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/session"
)

func RunTokenUsage(args []string, stdout io.Writer) error {
	jsonOut := false
	sinceStr := "7d"

	for _, arg := range args {
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "--watch":
			fmt.Fprintln(stdout, "--watch is not yet implemented")
			return nil
		case arg == "-h", arg == "--help":
			fmt.Fprintln(stdout, "Usage: squadai token-usage [--since <dur>] [--json] [--watch]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Aggregate real token usage from agent session transcripts.")
			fmt.Fprintln(stdout, "Scans OpenCode and Pi session directories for token counts.")
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

	if msg, ok := pricing.StaleWarning(time.Now()); ok {
		fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
	}

	if jsonOut {
		data, err := json.MarshalIndent(agg, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal token usage: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	printTokenUsageHuman(stdout, agg)
	return nil
}

func printTokenUsageHuman(w io.Writer, agg *session.Aggregation) {
	if agg.Total.SessionCount == 0 {
		fmt.Fprintf(w, "No sessions found in the last %s.\n", agg.Period)
		fmt.Fprintln(w, "Make sure OpenCode or Pi sessions exist in their session directories.")
		return
	}

	fmt.Fprintf(w, "Token Usage (last %s)\n", agg.Period)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-24s  %8s  %8s  %8s  %6s  %10s\n",
		"Model", "Input", "Output", "Total", "Sess.", "Est. Cost")
	fmt.Fprintf(w, "%-24s  %8s  %8s  %8s  %6s  %10s\n",
		"────────────────────────", "────────", "────────", "────────", "──────", "──────────")

	models := make([]string, 0, len(agg.ByModel))
	for m := range agg.ByModel {
		models = append(models, m)
	}
	sortModels(models)

	unknownCosts := 0
	for _, m := range models {
		u := agg.ByModel[m]
		if !u.CostKnown {
			unknownCosts++
		}
		fmt.Fprintf(w, "%-24s  %8d  %8d  %8d  %6d  %10s\n",
			m, u.InputTokens, u.OutputTokens, u.TotalTokens, u.SessionCount, costCell(u))
	}
	fmt.Fprintf(w, "%-24s  %8s  %8s  %8s  %6s  %10s\n",
		"────────────────────────", "────────", "────────", "────────", "──────", "──────────")
	fmt.Fprintf(w, "%-24s  %8d  %8d  %8d  %6d  %10s\n",
		"TOTAL", agg.Total.InputTokens, agg.Total.OutputTokens,
		agg.Total.TotalTokens, agg.Total.SessionCount, totalCostCell(agg.Total))

	if unknownCosts > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "unknown = no pricing data for this model; the total omits %d model(s) with unknown cost\n", unknownCosts)
	}
}

// costCell renders a per-model estimated cost, or "unknown" when no
// pricing data exists for the model — never a misleading $0.00.
func costCell(u session.Usage) string {
	if !u.CostKnown {
		return "unknown"
	}
	return fmt.Sprintf("$%9.4f", u.EstimatedCost)
}

// totalCostCell renders the grand-total cost. When some models have no
// pricing data, the sum of the known ones is shown as a lower bound; when
// none are known, it is simply unknown.
func totalCostCell(total session.Usage) string {
	switch {
	case total.CostKnown:
		return fmt.Sprintf("$%9.4f", total.EstimatedCost)
	case total.EstimatedCost > 0:
		return fmt.Sprintf("≥$%8.4f", total.EstimatedCost)
	default:
		return "unknown"
	}
}

func sortModels(models []string) {
	for i := 1; i < len(models); i++ {
		for j := i; j > 0 && models[j-1] > models[j]; j-- {
			models[j-1], models[j] = models[j], models[j-1]
		}
	}
}
