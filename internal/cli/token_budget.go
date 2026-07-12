package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/planner"
	"github.com/PedroMosquera/squadai/internal/tokenprofile"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/tokenizer"
)

// RunTokenBudget reports the approximate token cost of the current squadai install.
func RunTokenBudget(args []string, stdout io.Writer) error {
	jsonOut := false
	model := ""
	planned := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "--planned":
			planned = true
		case "--model":
			model = ""
		default:
			if len(arg) > 8 && arg[:8] == "--model=" {
				model = arg[8:]
			}
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai token-budget [--json] [--planned] [--model=<name>]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Estimate the per-session token cost of the current squadai install.")
			fmt.Fprintln(stdout, "By default reads installed files on disk and groups by component.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json         Output as JSON")
			fmt.Fprintln(stdout, "  --planned      Estimate planned rendered content before apply")
			fmt.Fprintln(stdout, "  --model=<name> Use model-aware tokenizer (e.g. claude-sonnet-4, gpt-4o)")
			fmt.Fprintln(stdout, "                 Models without an exact tokenizer (and the no-model")
			fmt.Fprintln(stdout, "                 default) use a chars/token approximation, marked '~'.")
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
		if planErr != nil {
			fmt.Fprintf(os.Stderr, "warning: plan failed: %v\n", planErr)
		} else if planned {
			report, err := plannedTokenBudgetReport(p, actions, homeDir, projectDir, model)
			if err != nil {
				return err
			}
			if jsonOut {
				return printTokenBudgetJSON(stdout, report)
			}
			printTokenBudgetHuman(stdout, report)
			return nil
		} else {
			for _, a := range actions {
				if a.TargetPath == "" || a.Action == domain.ActionDelete {
					continue
				}
				cat := string(a.Component)
				if cat == "" {
					cat = "other"
				}
				pathToCategory[a.TargetPath] = cat
			}
		}
	}

	report, err := tokenprofile.ScanPaths(pathToCategory)
	if err != nil {
		return fmt.Errorf("scan paths: %w", err)
	}

	if model != "" {
		counter := tokenizer.ForModel(model)
		for i, e := range report.Entries {
			data, _ := os.ReadFile(e.Path)
			report.Entries[i].Tokens = counter.Count(string(data))
		}
		report.TotalTokens = 0
		for k, s := range report.ByCategory {
			s.Tokens = 0
			report.ByCategory[k] = s
		}
		for _, e := range report.Entries {
			s := report.ByCategory[e.Category]
			s.Tokens += e.Tokens
			report.ByCategory[e.Category] = s
			report.TotalTokens += e.Tokens
		}
		report.Model = model
		report.Approximate = counter.Approximate()
	}

	if jsonOut {
		return printTokenBudgetJSON(stdout, report)
	}
	printTokenBudgetHuman(stdout, report)
	return nil
}

func plannedTokenBudgetReport(p *planner.Planner, actions []domain.PlannedAction, homeDir, projectDir, model string) (*tokenprofile.Report, error) {
	report := &tokenprofile.Report{
		ByCategory:  make(map[string]tokenprofile.CategorySummary),
		Model:       model,
		Approximate: true, // heuristic unless a real tokenizer counts below
	}
	var counter tokenizer.Counter
	if model != "" {
		counter = tokenizer.ForModel(model)
	}

	// Multiple actions can target the same file — e.g. the OpenCode and Pi
	// memory and brand sections all land in a shared AGENTS.md. Each
	// RenderAction returns the FULL rendered document, so summing per action
	// would multi-count the same file. Dedupe by target path, keeping the
	// fullest render seen for that path (for an already-applied project every
	// action is a Skip returning the identical complete document, so the
	// planned total converges to the authoritative installed scan).
	type pathEntry struct {
		category string
		desired  []byte
	}
	byPath := make(map[string]pathEntry)
	for _, action := range actions {
		// Delete and empty-target actions contribute nothing. Skip actions are
		// included: their rendered "desired" content is the current on-disk
		// content, which is still part of the planned token cost. Excluding
		// them would make an already-applied project report a misleading 0.
		if action.TargetPath == "" || action.Action == domain.ActionDelete {
			continue
		}
		_, desired, err := p.RenderAction(action, homeDir, projectDir)
		if err != nil {
			return nil, fmt.Errorf("render planned %s: %w", action.ID, err)
		}
		if desired == nil {
			continue
		}
		category := string(action.Component)
		if category == "" {
			category = "other"
		}
		if existing, ok := byPath[action.TargetPath]; ok && len(existing.desired) >= len(desired) {
			continue
		}
		byPath[action.TargetPath] = pathEntry{category: category, desired: desired}
	}

	// Aggregate one entry per unique target path.
	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		pe := byPath[path]
		tokens := tokenprofile.ApproxTokens(pe.desired)
		if counter != nil {
			tokens = counter.Count(string(pe.desired))
		}
		report.Entries = append(report.Entries, tokenprofile.Entry{
			Path:     path,
			Category: pe.category,
			Bytes:    len(pe.desired),
			Tokens:   tokens,
		})
		sum := report.ByCategory[pe.category]
		sum.Files++
		sum.Bytes += len(pe.desired)
		sum.Tokens += tokens
		report.ByCategory[pe.category] = sum
		report.TotalBytes += len(pe.desired)
		report.TotalTokens += tokens
	}
	if counter != nil {
		report.Approximate = counter.Approximate()
	}
	return report, nil
}

type tokenBudgetJSON struct {
	TotalBytes  int                                     `json:"total_bytes"`
	TotalTokens int                                     `json:"total_tokens"`
	Missing     int                                     `json:"missing_files"`
	Model       string                                  `json:"model,omitempty"`
	Approximate bool                                    `json:"approximate"`
	ByCategory  map[string]tokenprofile.CategorySummary `json:"by_category"`
}

func printTokenBudgetJSON(w io.Writer, r *tokenprofile.Report) error {
	out := tokenBudgetJSON{
		TotalBytes:  r.TotalBytes,
		TotalTokens: r.TotalTokens,
		Missing:     r.Missing,
		Model:       r.Model,
		Approximate: r.Approximate,
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
	method := "chars/token approximation"
	if r.Model != "" {
		method = "model-aware (" + r.Model + ")"
	}
	fmt.Fprintf(w, "Token Budget (%s)\n", method)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-16s  %5s  %9s  %8s\n", "Component", "Files", "Bytes", "Tokens")
	fmt.Fprintf(w, "%-16s  %5s  %9s  %8s\n",
		"────────────────", "─────", "─────────", "────────")

	// Sort categories for deterministic output.
	cats := make([]string, 0, len(r.ByCategory))
	for k := range r.ByCategory {
		cats = append(cats, k)
	}
	sort.Strings(cats)

	for _, cat := range cats {
		s := r.ByCategory[cat]
		fmt.Fprintf(w, "%-16s  %5d  %9d  %8s\n", cat, s.Files, s.Bytes, formatTokens(s.Tokens, r.Approximate))
	}
	fmt.Fprintf(w, "%-16s  %5s  %9s  %8s\n",
		"────────────────", "─────", "─────────", "────────")
	fmt.Fprintf(w, "%-16s  %5d  %9d  %8s\n",
		"TOTAL", totalFiles(r), r.TotalBytes, formatTokens(r.TotalTokens, r.Approximate))

	if r.Approximate {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "~ = character-based approximation; no exact tokenizer available for this model")
	}

	if r.Missing > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Note: %d planned file(s) not found on disk. Run `squadai apply` to install.\n", r.Missing)
	}
	if r.TotalTokens == 0 && r.Missing == 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "No installed files found. Run `squadai apply` to install.")
	}
}

// formatTokens renders a token count, prefixed with "~" when the count is
// a heuristic approximation rather than a real tokenizer's output.
func formatTokens(n int, approx bool) string {
	if approx {
		return "~" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func totalFiles(r *tokenprofile.Report) int {
	n := 0
	for _, s := range r.ByCategory {
		n += s.Files
	}
	return n
}
