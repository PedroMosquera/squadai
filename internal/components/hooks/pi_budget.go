package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
)

const piBudgetActionID = "pi-budget-extension"

// piBudgetOwnedMarker identifies the extension file as squadai's, so a user
// file that happens to share the name is never deleted.
const piBudgetOwnedMarker = "Written by `squadai apply`"

// PiBudgetExtensionPath is the project extension Pi loads once the project is
// trusted: https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/extensions.md
func PiBudgetExtensionPath(projectDir string) string {
	return filepath.Join(projectDir, ".pi", "extensions", "squadai-budget.ts")
}

// PiBudgetExtension returns the extension source enforcing usage in Pi, or ""
// when nothing should be installed (the same rule as BudgetHooks).
func PiBudgetExtension(usage domain.UsageConfig) string {
	if len(BudgetHooks(usage)) == 0 {
		return ""
	}
	src := assets.MustRead("budget/pi-extension.ts")
	return strings.ReplaceAll(src, "{{ENFORCEMENT}}", usage.Enforcement) + "\n"
}

func (i *Installer) planPiBudget(projectDir string) ([]domain.PlannedAction, error) {
	path := PiBudgetExtensionPath(projectDir)
	existing, err := fileutil.ReadFileOrEmpty(path)
	if err != nil {
		return nil, fmt.Errorf("read pi budget extension: %w", err)
	}
	desired := PiBudgetExtension(i.usage)
	action := domain.PlannedAction{ID: piBudgetActionID, Agent: domain.AgentPi, Component: domain.ComponentHooks, TargetPath: path}
	switch {
	case desired == "" && !strings.Contains(string(existing), piBudgetOwnedMarker):
		return nil, nil
	case desired == "":
		action.Action = domain.ActionDelete
		action.Description = "remove SquadAI budget extension from .pi/extensions"
	case string(existing) == desired:
		action.Action = domain.ActionSkip
		action.Description = "pi budget extension already installed"
	case len(existing) == 0:
		action.Action = domain.ActionCreate
		action.Description = "install SquadAI budget extension into .pi/extensions"
	default:
		action.Action = domain.ActionUpdate
		action.Description = "update SquadAI budget extension in .pi/extensions"
	}
	return []domain.PlannedAction{action}, nil
}

func (i *Installer) applyPiBudget(action domain.PlannedAction) error {
	if action.Action == domain.ActionDelete {
		if err := os.Remove(action.TargetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove pi budget extension: %w", err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(action.TargetPath), 0o755); err != nil {
		return fmt.Errorf("create pi extensions dir: %w", err)
	}
	if _, err := fileutil.WriteAtomic(action.TargetPath, []byte(PiBudgetExtension(i.usage)), 0o644); err != nil {
		return fmt.Errorf("write pi budget extension: %w", err)
	}
	return nil
}

func (i *Installer) verifyPiBudget(projectDir string) []domain.VerifyResult {
	desired := PiBudgetExtension(i.usage)
	if desired == "" {
		return nil
	}
	existing, _ := fileutil.ReadFileOrEmpty(PiBudgetExtensionPath(projectDir))
	r := domain.VerifyResult{Check: "pi-budget-extension", Passed: true, Severity: domain.SeverityInfo, Component: string(domain.ComponentHooks)}
	if string(existing) != desired {
		r.Passed, r.Severity = false, domain.SeverityError
		r.Message = "SquadAI budget extension in .pi/extensions is missing or out of date"
	}
	return []domain.VerifyResult{r}
}
