// Package legacyinstructions removes squadai's marker blocks from instruction
// files an adapter used to write but the agent never read. It runs whatever
// components are enabled, because memory, rules, brand, efficiency and agents
// all injected blocks into the old file.
package legacyinstructions

import (
	"fmt"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// legacyLister is implemented by adapters that moved their instructions file.
type legacyLister interface {
	LegacyInstructionsFiles(homeDir, projectDir string) []string
}

// Installer plans, applies and verifies the migration for one project.
type Installer struct {
	projectDir string
}

// New returns an Installer that untracks deleted files from the managed
// sidecar under projectDir.
func New(projectDir string) *Installer {
	return &Installer{projectDir: projectDir}
}

// ID returns ComponentCleanup: the migration is housekeeping, not a
// configurable component, so it has no key in project.json.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentCleanup
}

type legacyFile struct {
	path     string
	stripped string
}

// pending returns each legacy file of adapter that still holds squadai blocks.
func pending(adapter domain.Adapter, homeDir, projectDir string) ([]legacyFile, error) {
	l, ok := adapter.(legacyLister)
	if !ok {
		return nil, nil
	}
	current := map[string]bool{
		adapter.ProjectRulesFile(projectDir): true,
		adapter.SystemPromptFile(homeDir):    true,
	}
	var out []legacyFile
	for _, path := range l.LegacyInstructionsFiles(homeDir, projectDir) {
		if path == "" || current[path] {
			continue
		}
		data, err := fileutil.ReadFileOrEmpty(path)
		if err != nil {
			return nil, fmt.Errorf("read legacy instructions %s: %w", path, err)
		}
		stripped, found := marker.StripAll(string(data))
		if !found {
			continue
		}
		out = append(out, legacyFile{path: path, stripped: stripped})
	}
	return out, nil
}

// Plan emits one update per legacy file that still holds squadai blocks.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	files, err := pending(adapter, homeDir, projectDir)
	if err != nil {
		return nil, err
	}
	var actions []domain.PlannedAction
	for n, f := range files {
		desc := fmt.Sprintf("remove squadai blocks from %s, which %s does not read", f.path, adapter.ID())
		if strings.TrimSpace(f.stripped) == "" {
			desc = fmt.Sprintf("delete %s: it holds only squadai blocks %s does not read", f.path, adapter.ID())
		}
		actions = append(actions, domain.PlannedAction{
			ID:          fmt.Sprintf("%s-legacy-instructions-%d", adapter.ID(), n),
			Agent:       adapter.ID(),
			Component:   domain.ComponentCleanup,
			Action:      domain.ActionUpdate,
			TargetPath:  f.path,
			Description: desc,
		})
	}
	return actions, nil
}

// Apply is a no-op: the executor routes every ComponentCleanup action
// through managed.InspectStale, which strips the blocks and deletes the file
// only when squadai created it and nothing else is left.
func (i *Installer) Apply(action domain.PlannedAction) error {
	return nil
}

// Verify fails once per legacy file that still holds squadai blocks.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	files, err := pending(adapter, homeDir, projectDir)
	if err != nil {
		return nil, err
	}
	var results []domain.VerifyResult
	for _, f := range files {
		results = append(results, domain.VerifyResult{
			Check:    "instructions-legacy-migrated",
			Passed:   false,
			Severity: domain.SeverityError,
			Message: fmt.Sprintf("squadai blocks remain in %s, which %s does not read; run 'squadai apply' to remove them",
				f.path, adapter.ID()),
		})
	}
	return results, nil
}
