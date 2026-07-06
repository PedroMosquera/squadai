// Package budget implements a post-planning step that fits a planned action
// list within a token budget cap. Given the planner's []domain.PlannedAction
// and a cap, Fit decides which components to keep in full, which to summarize,
// and which to omit, then filters the action list accordingly. The chosen
// layout can be persisted to .squadai/.applied-budget.json so that later runs
// can detect budget drift (e.g. after swapping agents or components).
package budget

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/tokenprofile"
)

// Mode is the truncation mode for a component within a budget fit.
type Mode string

const (
	ModeFull    Mode = "full"
	ModeSummary Mode = "summary"
	ModeOmit    Mode = "omit"
)

// ComponentDecision is the budget decision for one component.
type ComponentDecision struct {
	Component domain.ComponentID `json:"component"`
	Mode      Mode               `json:"mode"`
	Tokens    int                `json:"tokens"`
}

// FitResult is the outcome of a budget fit.
type FitResult struct {
	Decisions   []ComponentDecision    `json:"decisions"`
	TotalTokens int                    `json:"total_tokens"`
	Cap         int                    `json:"cap"`
	Model       string                 `json:"model,omitempty"`
	Profile     string                 `json:"profile,omitempty"`
	FitAchieved bool                   `json:"fit_achieved"`
	Actions     []domain.PlannedAction `json:"-"`
}

// Options controls the budget fitting process.
type Options struct {
	MaxTokens       int                        // token cap; 0 means no cap (return all full)
	Model           string                     // model name for tokenizer selection
	Profile         string                     // active context profile name, recorded for drift detection
	ComponentTokens map[domain.ComponentID]int // optional precomputed desired-token counts
	// SummaryTokens holds real token counts for each component's summary
	// render, computed by the caller. Missing entries fall back to tokens/2.
	SummaryTokens map[domain.ComponentID]int
}

// summarizableComponents can degrade to a condensed render (ModeSummary)
// instead of being omitted outright. Other components go straight to omit.
var summarizableComponents = map[domain.ComponentID]bool{
	domain.ComponentMemory: true,
	domain.ComponentRules:  true,
	// Agents degrade to a compact orchestrator digest in shared rules files;
	// native agent files are unchanged (they are lazy-loaded by the agent).
	domain.ComponentAgents: true,
}

// componentPriority lists content components ordered from lowest to highest
// priority — the fitter drops the lowest first when the budget is tight.
var componentPriority = []domain.ComponentID{
	domain.ComponentPlugins,
	domain.ComponentCommands,
	domain.ComponentSkills,
	domain.ComponentMemory,
	domain.ComponentRules,
	domain.ComponentSettings,
	domain.ComponentMCP,
	domain.ComponentAgents,
	domain.ComponentEfficiency,
	domain.ComponentBrand,
	domain.ComponentPermissions,
}

// budgetFileName is the persisted budget sidecar under .squadai/.
const budgetFileName = ".applied-budget.json"

// budgetPath returns the absolute path to the persisted budget file.
func budgetPath(projectDir string) string {
	return filepath.Join(projectDir, ".squadai", budgetFileName)
}

// isContentComponent reports whether c is a content component subject to budget
// truncation. Components not in the priority list (cleanup, hooks, agent_teams,
// workflows) are operational and always kept in ModeFull.
func isContentComponent(c domain.ComponentID) bool {
	for _, p := range componentPriority {
		if p == c {
			return true
		}
	}
	return false
}

// priorityRank returns the drop-order rank of a content component (lower drops
// first). Operational components sort after all content components.
func priorityRank(c domain.ComponentID) int {
	for i, p := range componentPriority {
		if p == c {
			return i
		}
	}
	return len(componentPriority)
}

// componentTokens sums the approximate token count of all non-delete,
// non-empty-target actions for a component. Missing files count as 0 tokens
// and are not an error; other read errors propagate.
func componentTokens(component domain.ComponentID, actions []domain.PlannedAction, overrides map[domain.ComponentID]int) (int, error) {
	if overrides != nil {
		if n, ok := overrides[component]; ok {
			return n, nil
		}
	}
	total := 0
	for _, a := range actions {
		if a.Action == domain.ActionDelete || a.TargetPath == "" {
			continue
		}
		data, err := os.ReadFile(a.TargetPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return 0, fmt.Errorf("read %s: %w", a.TargetPath, err)
		}
		total += tokenprofile.ApproxTokens(data)
	}
	return total, nil
}

// Fit decides which components to keep, summarize, or omit so the planned
// actions fit within opts.MaxTokens. It groups actions by Component, estimates
// per-component token counts from the TargetPath files, then — when over cap —
// walks the priority list (lowest first) trying ModeSummary before ModeOmit.
// Operational components (cleanup, hooks, agent_teams, workflows) are always
// kept in ModeFull. Missing target files count as 0 tokens and never error.
func Fit(actions []domain.PlannedAction, opts Options) (*FitResult, error) {
	// Group actions by component, preserving first-seen order.
	groups := make(map[domain.ComponentID][]domain.PlannedAction)
	var order []domain.ComponentID
	for _, a := range actions {
		if _, ok := groups[a.Component]; !ok {
			order = append(order, a.Component)
		}
		groups[a.Component] = append(groups[a.Component], a)
	}

	// Compute per-component full token counts.
	tokensByComp := make(map[domain.ComponentID]int, len(order))
	for _, c := range order {
		t, err := componentTokens(c, groups[c], opts.ComponentTokens)
		if err != nil {
			return nil, err
		}
		tokensByComp[c] = t
	}

	// Initialize every component to ModeFull.
	modeByComp := make(map[domain.ComponentID]Mode, len(order))
	for _, c := range order {
		modeByComp[c] = ModeFull
	}

	// effective returns the token contribution of a component under its mode.
	effective := func(c domain.ComponentID) int {
		switch modeByComp[c] {
		case ModeOmit:
			return 0
		case ModeSummary:
			if opts.SummaryTokens != nil {
				if n, ok := opts.SummaryTokens[c]; ok {
					return n
				}
			}
			return tokensByComp[c] / 2
		default:
			return tokensByComp[c]
		}
	}
	total := func() int {
		t := 0
		for _, c := range order {
			t += effective(c)
		}
		return t
	}

	fitAchieved := true
	switch {
	case opts.MaxTokens == 0:
		// No cap — everything stays full.
	case total() <= opts.MaxTokens:
		// Everything already fits as-is.
	default:
		fitAchieved = false
		// Pass 1: summarize lowest-priority summarizable content components,
		// accumulating until the cap is met. Components without a summary
		// render skip this pass and go straight to omit in pass 2.
		for _, c := range componentPriority {
			if _, ok := groups[c]; !ok {
				continue
			}
			if !summarizableComponents[c] {
				continue
			}
			if modeByComp[c] != ModeFull {
				continue
			}
			modeByComp[c] = ModeSummary
			if total() <= opts.MaxTokens {
				fitAchieved = true
				break
			}
		}
		// Pass 2: omit lowest-priority content components until it fits.
		if !fitAchieved {
			for _, c := range componentPriority {
				if _, ok := groups[c]; !ok {
					continue
				}
				if modeByComp[c] == ModeOmit {
					continue
				}
				modeByComp[c] = ModeOmit
				if total() <= opts.MaxTokens {
					fitAchieved = true
					break
				}
			}
		}
	}

	// Build the filtered action list. Omitted components are dropped; summary
	// components keep their actions tagged Mode="summary" so installers render
	// the condensed variant. A summary action planned as Skip is upgraded to
	// Update — the on-disk content is the full render, not the summary.
	finalActions := make([]domain.PlannedAction, 0, len(actions))
	for _, a := range actions {
		switch modeByComp[a.Component] {
		case ModeOmit:
			continue
		case ModeSummary:
			a.Mode = string(ModeSummary)
			if a.Action == domain.ActionSkip {
				a.Action = domain.ActionUpdate
			}
		}
		finalActions = append(finalActions, a)
	}

	return &FitResult{
		Decisions:   buildDecisions(order, modeByComp, tokensByComp),
		TotalTokens: total(),
		Cap:         opts.MaxTokens,
		Model:       opts.Model,
		Profile:     opts.Profile,
		FitAchieved: fitAchieved,
		Actions:     finalActions,
	}, nil
}

// buildDecisions assembles ComponentDecisions sorted by drop priority (lowest
// first) for content components, with operational components after; ties break
// by component name for deterministic output.
func buildDecisions(order []domain.ComponentID, modes map[domain.ComponentID]Mode, tokens map[domain.ComponentID]int) []ComponentDecision {
	decs := make([]ComponentDecision, 0, len(order))
	for _, c := range order {
		decs = append(decs, ComponentDecision{
			Component: c,
			Mode:      modes[c],
			Tokens:    tokens[c],
		})
	}
	sort.Slice(decs, func(i, j int) bool {
		ri, rj := priorityRank(decs[i].Component), priorityRank(decs[j].Component)
		if ri != rj {
			return ri < rj
		}
		return decs[i].Component < decs[j].Component
	})
	return decs
}

// Persist writes the fit result (minus Actions) to .squadai/.applied-budget.json
// under projectDir so later runs can detect drift.
func Persist(projectDir string, result *FitResult) error {
	if result == nil {
		return fmt.Errorf("budget.Persist: nil result")
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal applied budget: %w", err)
	}
	data = append(data, '\n')
	if _, err := fileutil.WriteAtomic(budgetPath(projectDir), data, 0o644); err != nil {
		return fmt.Errorf("write applied budget: %w", err)
	}
	return nil
}

// Load reads the persisted budget from .squadai/.applied-budget.json. It returns
// (nil, nil) when the file does not exist.
func Load(projectDir string) (*FitResult, error) {
	data, err := os.ReadFile(budgetPath(projectDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read applied budget: %w", err)
	}
	var r FitResult
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse applied budget: %w", err)
	}
	return &r, nil
}

// DetectDrift reports whether the current plan's content-component set or model
// differs from the persisted budget, indicating a re-fit is needed. Returns
// false when no prior budget has been persisted.
func DetectDrift(projectDir string, currentActions []domain.PlannedAction, opts Options) (bool, error) {
	persisted, err := Load(projectDir)
	if err != nil {
		return false, err
	}
	if persisted == nil {
		return false, nil
	}
	if opts.Model != "" && persisted.Model != "" && opts.Model != persisted.Model {
		return true, nil
	}
	if opts.Profile != persisted.Profile {
		return true, nil
	}
	if !sameSet(contentComponentSet(currentActions), contentComponentSetFromDecisions(persisted.Decisions)) {
		return true, nil
	}
	return false, nil
}

// contentComponentSet returns the set of content components present in actions.
func contentComponentSet(actions []domain.PlannedAction) map[domain.ComponentID]struct{} {
	set := make(map[domain.ComponentID]struct{})
	for _, a := range actions {
		if isContentComponent(a.Component) {
			set[a.Component] = struct{}{}
		}
	}
	return set
}

// contentComponentSetFromDecisions returns the set of content components in
// persisted decisions.
func contentComponentSetFromDecisions(decs []ComponentDecision) map[domain.ComponentID]struct{} {
	set := make(map[domain.ComponentID]struct{})
	for _, d := range decs {
		if isContentComponent(d.Component) {
			set[d.Component] = struct{}{}
		}
	}
	return set
}

// sameSet reports whether two component sets contain the same elements.
func sameSet(a, b map[domain.ComponentID]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}
