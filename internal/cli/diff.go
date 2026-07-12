package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/model"
	"github.com/PedroMosquera/squadai/internal/planner"
	"github.com/PedroMosquera/squadai/internal/state"
)

// applyModelOverrides applies --model role=tier,... overrides to the merged config in-memory.
// Overrides do NOT write back to agentmgr.yaml; they last only for the current apply run.
// Returns an error if any role name or tier value is invalid.
func applyModelOverrides(merged *domain.MergedConfig, overrides []string) error {
	for _, pair := range overrides {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("--model: invalid pair %q (expected role=tier)", pair)
		}
		roleName := strings.TrimSpace(parts[0])
		tierStr := strings.TrimSpace(parts[1])

		if _, exists := merged.Team[roleName]; !exists {
			return fmt.Errorf("--model: role %q not found in current config", roleName)
		}
		if _, err := model.ParseTier(tierStr); err != nil {
			return fmt.Errorf("--model: %w", err)
		}

		role := merged.Team[roleName]
		role.Model = tierStr
		merged.Team[roleName] = role
	}
	return nil
}

// shouldRunReview decides whether to render the pre-apply review TUI.
// It falls through (returns false) when: the user opted out, machine output
// was requested, the hook was never wired, or stdout is not a terminal.
func shouldRunReview(noReview, jsonOut bool) bool {
	if noReview || jsonOut {
		return false
	}
	if ReviewPromptHook == nil {
		return false
	}
	if IsTTYHook == nil || !IsTTYHook() {
		return false
	}
	return true
}

// collectPreviewEntries asks every installer that implements domain.Previewer
// for its proposed changes across all adapters and returns the flattened
// list. Installers that don't implement Previewer are skipped silently —
// the TUI only surfaces installers that opted in.
func collectPreviewEntries(
	installers map[domain.ComponentID]domain.ComponentInstaller,
	adapters []domain.Adapter,
	homeDir, projectDir string,
) ([]domain.PreviewEntry, error) {
	var out []domain.PreviewEntry
	for _, inst := range installers {
		pv, ok := inst.(domain.Previewer)
		if !ok {
			continue
		}
		for _, adapter := range adapters {
			entries, err := pv.Preview(adapter, homeDir, projectDir)
			if err != nil {
				return nil, fmt.Errorf("%s preview for %s: %w", inst.ID(), adapter.ID(), err)
			}
			out = append(out, entries...)
		}
	}
	return out, nil
}

// diffEntry is the JSON representation of a single diff action.
type diffEntry struct {
	Path      string            `json:"path"`
	Action    string            `json:"action"`
	Agent     string            `json:"agent,omitempty"`
	Component string            `json:"component"`
	Diff      string            `json:"diff"`
	Conflicts []domain.Conflict `json:"conflicts,omitempty"`
}

// RunDiff shows what apply would change as unified diffs. It is read-only.
func RunDiff(args []string, stdout io.Writer) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	return runDiff(args, stdout, homeDir, projectDir)
}

// runDiff is the testable core of RunDiff with injected homeDir and projectDir.
func runDiff(args []string, stdout io.Writer, homeDir, projectDir string) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai diff [flags]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Preview what 'apply' would change without modifying any files.")
			fmt.Fprintln(stdout, "Shows a unified diff for each file that would be created, modified, or deleted.")
			fmt.Fprintln(stdout, "This is the \"terraform plan\" equivalent — run it before apply to review changes.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json        Output planned actions as JSON (for scripting and CI)")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai diff                  Show what would change")
			fmt.Fprintln(stdout, "  squadai diff --json           Machine-readable diff output")
			fmt.Fprintln(stdout, "  squadai init && squadai diff    Preview after fresh init")
			return nil
		default:
			return fmt.Errorf("unknown flag %q for diff", arg)
		}
	}

	merged, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}

	adapters := DetectAdapters(homeDir)
	p := planner.New()
	actions, err := p.Plan(merged, adapters, homeDir, projectDir)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	// Filter to only non-skip actions.
	var nonSkip []domain.PlannedAction
	for _, a := range actions {
		if a.Action != domain.ActionSkip {
			nonSkip = append(nonSkip, a)
		}
	}

	if len(nonSkip) == 0 {
		if jsonOut {
			fmt.Fprintln(stdout, "[]")
			return nil
		}
		fmt.Fprintln(stdout, "Nothing to change.")
		return nil
	}

	if jsonOut {
		// Build a per-target conflict map from the previewers so --json
		// output mirrors what the review screen would show. Non-previewed
		// actions fall back to the raw diff with no conflicts.
		previewEntries, _ := collectPreviewEntries(p.ComponentInstallers(), adapters, homeDir, projectDir)
		conflictsByTarget := make(map[string][]domain.Conflict, len(previewEntries))
		for _, pe := range previewEntries {
			if len(pe.Conflicts) > 0 {
				conflictsByTarget[pe.TargetPath] = pe.Conflicts
			}
		}

		entries := make([]diffEntry, 0, len(nonSkip))
		for _, a := range nonSkip {
			entry := diffEntry{
				Path:      a.TargetPath,
				Action:    string(a.Action),
				Agent:     string(a.Agent),
				Component: string(a.Component),
				Conflicts: conflictsByTarget[a.TargetPath],
			}
			if a.Action == domain.ActionDelete {
				entry.Diff = "Would remove: " + a.TargetPath
			} else {
				old, newContent, renderErr := p.RenderAction(a, homeDir, projectDir)
				if renderErr == nil {
					entry.Diff = fileutil.UnifiedDiff(a.TargetPath, string(old), string(newContent))
				}
			}
			entries = append(entries, entry)
		}
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal diff entries: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	// Human-readable output.
	for _, a := range nonSkip {
		agentInfo := ""
		if a.Agent != "" || a.Component != "" {
			parts := []string{}
			if a.Component != "" {
				parts = append(parts, string(a.Component))
			}
			if a.Agent != "" {
				parts = append(parts, string(a.Agent))
			}
			if len(parts) > 0 {
				agentInfo = " (" + strings.Join(parts, "/") + ")"
			}
		}

		switch a.Action {
		case domain.ActionDelete:
			fmt.Fprintf(stdout, "=== Would remove: %s\n\n", a.TargetPath)

		case domain.ActionCreate, domain.ActionUpdate:
			label := "Would create"
			if a.Action == domain.ActionUpdate {
				label = "Would update"
			}
			fmt.Fprintf(stdout, "=== %s: %s%s\n", label, a.TargetPath, agentInfo)

			old, newContent, renderErr := p.RenderAction(a, homeDir, projectDir)
			if renderErr != nil {
				fmt.Fprintf(stdout, "(could not render diff: %v)\n\n", renderErr)
				continue
			}

			diff := fileutil.UnifiedDiff(a.TargetPath, string(old), string(newContent))
			if diff != "" {
				fmt.Fprintln(stdout, diff)
			} else {
				fmt.Fprintln(stdout, "(no textual diff available)")
				fmt.Fprintln(stdout)
			}
		}
	}

	return nil
}

// applyStateFilter restricts adapters to the union of state.InstalledAgents and the
// adapters enabled in the current config. When the state file is missing or empty,
// the original adapters slice is returned unchanged.
func applyStateFilter(adapters []domain.Adapter, merged *domain.MergedConfig, homeDir string) []domain.Adapter {
	statePath, err := state.DefaultPath()
	if err != nil {
		return adapters
	}
	st, err := state.Load(statePath)
	if err != nil || len(st.InstalledAgents) == 0 {
		return adapters
	}

	// Build the allowed set: state agents ∪ config-enabled adapters.
	allowed := make(map[string]bool, len(st.InstalledAgents)+len(merged.Adapters))
	for _, id := range st.InstalledAgents {
		allowed[id] = true
	}
	for id, ac := range merged.Adapters {
		if ac.Enabled {
			allowed[id] = true
		}
	}

	var filtered []domain.Adapter
	for _, a := range adapters {
		if allowed[string(a.ID())] {
			filtered = append(filtered, a)
		}
	}
	if filtered == nil {
		return adapters // fallback: don't lock out everything
	}
	return filtered
}
