package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mattn/go-isatty"
)

// jsonOutput is the structured JSON schema for --json output.
type jsonOutput struct {
	Version   string          `json:"version"`
	Timestamp string          `json:"timestamp"`
	Summary   jsonSummary     `json:"summary"`
	Checks    []jsonCheckItem `json:"checks"`
}

type jsonSummary struct {
	Pass int `json:"pass"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
	Skip int `json:"skip"`
}

type jsonCheckItem struct {
	Category string `json:"category"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
	Detail   string `json:"detail,omitempty"`
	FixHint  string `json:"fix_hint,omitempty"`
}

// RenderJSON writes the results as structured JSON to w.
func RenderJSON(w io.Writer, results []CheckResult, version string) error {
	summary := jsonSummary{}
	items := make([]jsonCheckItem, 0, len(results))
	for _, r := range results {
		switch r.Status {
		case CheckPass:
			summary.Pass++
		case CheckWarn:
			summary.Warn++
		case CheckFail:
			summary.Fail++
		case CheckSkip:
			summary.Skip++
		}
		items = append(items, jsonCheckItem{
			Category: r.Category,
			Name:     r.Name,
			Status:   r.Status.String(),
			Message:  r.Message,
			Detail:   r.Detail,
			FixHint:  r.FixHint,
		})
	}
	out := jsonOutput{
		Version:   version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Summary:   summary,
		Checks:    items,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal doctor JSON: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

// ANSI color codes.
const (
	ansiReset  = "\033[0m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
	ansiGray   = "\033[90m"
	ansiBold   = "\033[1m"
	ansiCyan   = "\033[36m"
)

// isTerminal returns true when fd refers to a terminal.
func isTerminal(fd uintptr) bool {
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// colorsEnabled returns true when stdout is a real terminal.
func colorsEnabled(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return isTerminal(f.Fd())
	}
	return false
}

// RenderHuman writes colored, human-readable output to w.
func RenderHuman(w io.Writer, results []CheckResult, version string, verbose bool) {
	colors := colorsEnabled(w)
	cat := func(s string) string {
		if colors {
			return ansiBold + ansiCyan + s + ansiReset
		}
		return s
	}

	// Group by category in canonical order.
	grouped := make(map[string][]CheckResult, len(categoryOrder))
	for _, r := range results {
		grouped[r.Category] = append(grouped[r.Category], r)
	}

	fmt.Fprintf(w, "\n  %s\n\n", cat("SquadAI Doctor "+version))

	for _, catName := range categoryOrder {
		items, ok := grouped[catName]
		if !ok {
			continue
		}
		fmt.Fprintf(w, "  %s\n", cat(catName))
		for _, r := range items {
			icon, colorStart, colorEnd := statusIcon(r.Status, colors)
			line := fmt.Sprintf("  %s%s%s  %s", colorStart, icon, colorEnd, r.Message)
			if verbose && r.Detail != "" {
				line += fmt.Sprintf(" [%s]", r.Detail)
			}
			fmt.Fprintln(w, line)
			if verbose && r.FixHint != "" {
				fmt.Fprintf(w, "      → %s\n", r.FixHint)
			}
		}
		fmt.Fprintln(w)
	}

	// Summary line.
	var pass, warns, fails, skips int
	for _, r := range results {
		switch r.Status {
		case CheckPass:
			pass++
		case CheckWarn:
			warns++
		case CheckFail:
			fails++
		case CheckSkip:
			skips++
		}
	}

	summary := fmt.Sprintf("  Summary: %d checks passed, %d warnings, %d errors", pass, warns, fails)
	if skips > 0 {
		summary += fmt.Sprintf(", %d skipped", skips)
	}
	if colors && fails > 0 {
		summary = ansiRed + summary + ansiReset
	} else if colors && warns > 0 {
		summary = ansiYellow + summary + ansiReset
	} else if colors {
		summary = ansiGreen + summary + ansiReset
	}
	fmt.Fprintln(w, summary)

	if fails > 0 {
		fmt.Fprintln(w, "\n  Run 'squadai doctor --fix' to auto-resolve fixable issues.")
	}
	fmt.Fprintln(w)
}

// statusIcon returns the icon and ANSI codes for a CheckStatus.
func statusIcon(s CheckStatus, colors bool) (icon, start, end string) {
	if !colors {
		switch s {
		case CheckPass:
			return "[pass]", "", ""
		case CheckWarn:
			return "[warn]", "", ""
		case CheckFail:
			return "[fail]", "", ""
		default:
			return "[skip]", "", ""
		}
	}
	switch s {
	case CheckPass:
		return "✓", ansiGreen, ansiReset
	case CheckWarn:
		return "⚠", ansiYellow, ansiReset
	case CheckFail:
		return "✗", ansiRed, ansiReset
	default:
		return "──", ansiGray, ansiReset
	}
}
