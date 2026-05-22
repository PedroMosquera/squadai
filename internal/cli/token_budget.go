package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/PedroMosquera/squadai/internal/planner"
	"github.com/PedroMosquera/squadai/internal/tokenprofile"
)

// RunTokenBudget reports the approximate token cost of the current squadai install.
func RunTokenBudget(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai token-budget [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Estimate the per-session token cost of the current squadai install.")
			fmt.Fprintln(stdout, "Reads installed files on disk and groups by component.")
			fmt.Fprintln(stdout, "Token count uses a 4 chars/token approximation.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json   Output as JSON")
			return nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		// No config is not an error for token-budget; just report 0.
		merged = nil
	}

	adapters := DetectAdapters(homeDir)

	pathToCategory := make(map[string]string)
	if merged != nil {
		p := planner.New()
		actions, planErr := p.Plan(merged, adapters, homeDir, projectDir)
		if planErr == nil {
			for _, a := range actions {
				if a.TargetPath == "" {
					continue
				}
				pathToCategory[a.TargetPath] = string(a.Component)
			}
		}
	}

	report, err := tokenprofile.ScanPaths(pathToCategory)
	if err != nil {
		return fmt.Errorf("scan paths: %w", err)
	}

	if jsonOut {
		return printTokenBudgetJSON(stdout, report)
	}
	printTokenBudgetHuman(stdout, report)
	return nil
}

type tokenBudgetJSON struct {
	TotalBytes  int                                     `json:"total_bytes"`
	TotalTokens int                                     `json:"total_tokens"`
	Missing     int                                     `json:"missing_files"`
	ByCategory  map[string]tokenprofile.CategorySummary `json:"by_category"`
}

func printTokenBudgetJSON(w io.Writer, r *tokenprofile.Report) error {
	out := tokenBudgetJSON{
		TotalBytes:  r.TotalBytes,
		TotalTokens: r.TotalTokens,
		Missing:     r.Missing,
		ByCategory:  r.ByCategory,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token budget: %w", err)
	}
	fmt.Fprintln(w, string(data))
	return nil
}

func printTokenBudgetHuman(w io.Writer, r *tokenprofile.Report) {
	fmt.Fprintln(w, "Token Budget (approx. 4 chars/token)")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-16s  %5s  %9s  %8s\n", "Component", "Files", "Bytes", "~Tokens")
	fmt.Fprintf(w, "%-16s  %5s  %9s  %8s\n",
		"─────────────────", "─────", "─────────", "────────")

	// Sort categories for deterministic output.
	cats := make([]string, 0, len(r.ByCategory))
	for k := range r.ByCategory {
		cats = append(cats, k)
	}
	sort.Strings(cats)

	for _, cat := range cats {
		s := r.ByCategory[cat]
		fmt.Fprintf(w, "%-16s  %5d  %9d  %8d\n", cat, s.Files, s.Bytes, s.Tokens)
	}
	fmt.Fprintf(w, "%-16s  %5s  %9s  %8s\n",
		"─────────────────", "─────", "─────────", "────────")
	fmt.Fprintf(w, "%-16s  %5d  %9d  %8d\n",
		"TOTAL", totalFiles(r), r.TotalBytes, r.TotalTokens)

	if r.Missing > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Note: %d planned file(s) not found on disk. Run `squadai apply` to install.\n", r.Missing)
	}
	if r.TotalTokens == 0 && r.Missing == 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "No installed files found. Run `squadai apply` to install.")
	}
}

func totalFiles(r *tokenprofile.Report) int {
	n := 0
	for _, s := range r.ByCategory {
		n += s.Files
	}
	return n
}
