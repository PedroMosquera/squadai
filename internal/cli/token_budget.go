package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/components/brand"
	"github.com/PedroMosquera/squadai/internal/components/efficiency"
	"github.com/PedroMosquera/squadai/internal/components/memory"
	"github.com/PedroMosquera/squadai/internal/components/rules"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/marker"
	"github.com/PedroMosquera/squadai/internal/planner"
	"github.com/PedroMosquera/squadai/internal/tokenprofile"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/tokenizer"
)

// RunTokenBudget reports the approximate token cost of the current squadai install.
func RunTokenBudget(args []string, stdout io.Writer) error {
	jsonOut := false
	model := ""
	planned := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "--planned":
			planned = true
		case arg == "--model":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return fmt.Errorf("--model requires a value (e.g. --model claude-sonnet-4-6)")
			}
			i++
			model = args[i]
		case strings.HasPrefix(arg, "--model="):
			model = arg[len("--model="):]
		case arg == "-h" || arg == "--help":
			fmt.Fprintln(stdout, "Usage: squadai token-budget [--json] [--planned] [--model <name>]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Estimate the per-session token cost of the current squadai install.")
			fmt.Fprintln(stdout, "By default reads installed files on disk and groups by component.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json          Output as JSON")
			fmt.Fprintln(stdout, "  --planned       Estimate planned rendered content before apply")
			fmt.Fprintln(stdout, "  --model <name>  Use model-aware tokenizer (e.g. claude-sonnet-4-6, gpt-5-mini)")
			fmt.Fprintln(stdout, "                  Accepts --model <name> or --model=<name>.")
			fmt.Fprintln(stdout, "                  Falls back to 4 chars/token heuristic when omitted.")
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
	applyDefaultProfile(merged)

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
	}

	if jsonOut {
		return printTokenBudgetJSON(stdout, report)
	}
	printTokenBudgetHuman(stdout, report)
	return nil
}

// sectionComponents are marker-injection components whose planned cost is the
// content of their own marker section, not the whole shared document. Keying
// them by section lets each appear as its own row even when several share one
// rules file (e.g. memory + efficiency + brand in AGENTS.md).
var sectionComponents = map[domain.ComponentID]func(domain.AgentID) []string{
	domain.ComponentMemory: func(a domain.AgentID) []string {
		return []string{memory.SectionIDForAgentID(a), memory.SectionID}
	},
	domain.ComponentEfficiency: func(a domain.AgentID) []string {
		return []string{efficiency.SectionIDForAgentID(a), efficiency.SectionID}
	},
	domain.ComponentBrand: func(a domain.AgentID) []string {
		return []string{brand.SectionIDForAgentID(a), brand.SectionID}
	},
	domain.ComponentRules: func(domain.AgentID) []string {
		return []string{rules.SectionID}
	},
}

// plannedSectionContent extracts the component's own marker section from a
// full rendered document. Returns ok=false when the component is not
// section-based or its section is absent from the render.
func plannedSectionContent(action domain.PlannedAction, desired []byte) ([]byte, bool) {
	sids, ok := sectionComponents[action.Component]
	if !ok {
		return nil, false
	}
	doc := string(desired)
	for _, sid := range sids(action.Agent) {
		if marker.HasSection(doc, sid) {
			return []byte(marker.ExtractSection(doc, sid)), true
		}
	}
	return nil, false
}

func plannedTokenBudgetReport(p *planner.Planner, actions []domain.PlannedAction, homeDir, projectDir, model string) (*tokenprofile.Report, error) {
	report := &tokenprofile.Report{
		ByCategory: make(map[string]tokenprofile.CategorySummary),
		Model:      model,
	}
	var counter tokenizer.Counter
	if model != "" {
		counter = tokenizer.ForModel(model)
	}

	// Multiple actions can target the same file — e.g. the OpenCode and Pi
	// memory and brand sections all land in a shared AGENTS.md. Each
	// RenderAction returns the FULL rendered document, so summing per action
	// would multi-count the same file. Section-based components (memory,
	// efficiency, brand, rules) are attributed by their own marker section so
	// each keeps its own row; the remaining full-document renders dedupe by
	// target path, keeping the fullest render seen for that path.
	type pathEntry struct {
		path     string
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
		key := action.TargetPath
		if section, ok := plannedSectionContent(action, desired); ok {
			desired = section
			key = action.TargetPath + "\x00" + category + "\x00" + string(action.Agent)
		}
		if existing, ok := byPath[key]; ok && len(existing.desired) >= len(desired) {
			continue
		}
		byPath[key] = pathEntry{path: action.TargetPath, category: category, desired: desired}
	}

	// Aggregate one entry per unique key (path, or path+section-component).
	keys := make([]string, 0, len(byPath))
	for key := range byPath {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		pe := byPath[key]
		tokens := tokenprofile.ApproxTokens(pe.desired)
		if counter != nil {
			tokens = counter.Count(string(pe.desired))
		}
		report.Entries = append(report.Entries, tokenprofile.Entry{
			Path:     pe.path,
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
	return report, nil
}

type tokenBudgetJSON struct {
	TotalBytes  int                                     `json:"total_bytes"`
	TotalTokens int                                     `json:"total_tokens"`
	Missing     int                                     `json:"missing_files"`
	Model       string                                  `json:"model,omitempty"`
	ByCategory  map[string]tokenprofile.CategorySummary `json:"by_category"`
}

func printTokenBudgetJSON(w io.Writer, r *tokenprofile.Report) error {
	out := tokenBudgetJSON{
		TotalBytes:  r.TotalBytes,
		TotalTokens: r.TotalTokens,
		Missing:     r.Missing,
		Model:       r.Model,
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
	method := "approx. 4 chars/token"
	if r.Model != "" {
		method = "model-aware (" + r.Model + ")"
	}
	fmt.Fprintf(w, "Token Budget (%s)\n", method)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-16s  %5s  %9s  %8s\n", "Component", "Files", "Bytes", "~Tokens")
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
		fmt.Fprintf(w, "%-16s  %5d  %9d  %8d\n", cat, s.Files, s.Bytes, s.Tokens)
	}
	fmt.Fprintf(w, "%-16s  %5s  %9s  %8s\n",
		"────────────────", "─────", "─────────", "────────")
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
