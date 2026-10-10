// Package legacyinstructions removes squadai's marker blocks from instruction
// files an adapter used to write but the agent never read. It runs whatever
// components are enabled, because memory, rules, brand, efficiency and agents
// all injected blocks into the old file.
package legacyinstructions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/managed"
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

// Apply strips squadai's blocks from action.TargetPath and deletes the file
// only when nothing else is left. It re-reads the file rather than trusting
// the plan, so content the user added after planning survives.
func (i *Installer) Apply(action domain.PlannedAction) error {
	data, err := fileutil.ReadFileOrEmpty(action.TargetPath)
	if err != nil {
		return fmt.Errorf("read legacy instructions: %w", err)
	}
	stripped, found := marker.StripAll(string(data))
	if !found {
		return nil
	}
	if strings.TrimSpace(stripped) != "" {
		if _, err := fileutil.WriteAtomic(action.TargetPath, []byte(stripped), 0644); err != nil {
			return fmt.Errorf("write legacy instructions: %w", err)
		}
		return nil
	}
	if err := os.Remove(action.TargetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy instructions: %w", err)
	}
	if i.projectDir == "" {
		return nil
	}
	// The executor records created files relative to the project, home paths
	// included, so untrack with the same computation.
	rel, err := filepath.Rel(i.projectDir, action.TargetPath)
	if err != nil {
		return nil
	}
	return managed.UntrackCreatedFile(i.projectDir, rel)
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

// Render returns what Apply would leave in a file holding existing; empty
// means Apply deletes the file.
func Render(existing []byte) []byte {
	stripped, _ := marker.StripAll(string(existing))
	if strings.TrimSpace(stripped) == "" {
		return nil
	}
	return []byte(stripped)
}
