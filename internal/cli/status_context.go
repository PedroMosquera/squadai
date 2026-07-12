package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/PedroMosquera/squadai/internal/planner/budget"
	"github.com/PedroMosquera/squadai/internal/tokenprofile"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/session"
)

// Installed-token sources for the status Context section.
const (
	contextSourceBudget = "applied-budget"
	contextSourceScan   = "scan"
)

// contextHealth summarizes the context cost of the current install for the
// status command. Last7dTokens is only populated when usage aggregation was
// requested (the daily view) — session scanning can be slow.
type contextHealth struct {
	Profile           string `json:"profile"`
	TokenCap          int    `json:"token_cap,omitempty"`
	InstalledTokens   int    `json:"installed_tokens"`
	Source            string `json:"source"`
	HasFit            bool   `json:"fit_recorded"`
	FullComponents    int    `json:"full_components,omitempty"`
	SummaryComponents int    `json:"summary_components,omitempty"`
	OmittedComponents int    `json:"omitted_components,omitempty"`
	HasUsage          bool   `json:"-"`
	Last7dTokens      int    `json:"-"`
}

// collectContextHealth gathers the Context section data: active profile and
// cap from the merged config, installed tokens from the persisted budget fit
// (.squadai/.applied-budget.json) when present or a fast ApproxTokens scan of
// managed files otherwise, and — when includeUsage is set — the last-7d real
// usage total from session transcripts.
func collectContextHealth(homeDir, projectDir string, merged *domain.MergedConfig, includeUsage bool) *contextHealth {
	ch := &contextHealth{Profile: "default", Source: contextSourceScan}
	if merged != nil {
		if merged.Context.DefaultProfile != "" {
			ch.Profile = merged.Context.DefaultProfile
		}
		if prof, ok := merged.Context.Profiles[merged.Context.DefaultProfile]; ok {
			ch.TokenCap = prof.MaxApproxTokens
		}
	}

	if fit, err := budget.Load(projectDir); err == nil && fit != nil {
		ch.Source = contextSourceBudget
		ch.HasFit = true
		ch.InstalledTokens = fit.TotalTokens
		if ch.TokenCap == 0 {
			ch.TokenCap = fit.Cap
		}
		for _, d := range fit.Decisions {
			switch d.Mode {
			case budget.ModeSummary:
				ch.SummaryComponents++
			case budget.ModeOmit:
				ch.OmittedComponents++
			default:
				ch.FullComponents++
			}
		}
	} else {
		ch.InstalledTokens = scanManagedTokens(projectDir)
	}

	if includeUsage {
		if agg, err := session.Aggregate(homeDir, session.AggregateOptions{
			Since:      7 * 24 * time.Hour,
			ProjectDir: projectDir,
		}); err == nil {
			ch.HasUsage = true
			ch.Last7dTokens = agg.Total.TotalTokens
		}
	}
	return ch
}

// scanManagedTokens sums the approximate token counts of all files SquadAI
// manages or created under projectDir. Missing files count zero.
func scanManagedTokens(projectDir string) int {
	seen := map[string]bool{}
	total := 0
	addPaths := func(rels []string) {
		for _, rel := range rels {
			if seen[rel] {
				continue
			}
			seen[rel] = true
			data, err := os.ReadFile(filepath.Join(projectDir, rel))
			if err != nil {
				continue
			}
			total += tokenprofile.ApproxTokens(data)
		}
	}
	if rels, err := managed.ListManagedFiles(projectDir); err == nil {
		addPaths(rels)
	}
	if rels, err := managed.ListCreatedFiles(projectDir); err == nil {
		addPaths(rels)
	}
	return total
}

// printContextSection renders the Context health section of status.
func printContextSection(w io.Writer, ch *contextHealth) {
	if ch == nil {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Context:")
	if ch.TokenCap > 0 {
		fmt.Fprintf(w, "  Profile:     %s (cap %s tokens)\n", ch.Profile, formatKTokens(ch.TokenCap))
	} else {
		fmt.Fprintf(w, "  Profile:     %s (no token cap)\n", ch.Profile)
	}
	sourceLabel := "fast scan of managed files"
	if ch.Source == contextSourceBudget {
		sourceLabel = "from applied budget"
	}
	fmt.Fprintf(w, "  Installed:   %s tokens (%s)\n", formatKTokens(ch.InstalledTokens), sourceLabel)
	if ch.HasFit {
		fmt.Fprintf(w, "  Fit:         %d full / %d summary / %d omitted\n",
			ch.FullComponents, ch.SummaryComponents, ch.OmittedComponents)
	}
	if ch.HasUsage {
		fmt.Fprintf(w, "  Last 7d use: %s tokens\n", formatKTokens(ch.Last7dTokens))
	}
}
