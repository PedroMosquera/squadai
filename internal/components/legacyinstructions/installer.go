// Package legacyinstructions keeps the shared AGENTS.md wired up and moves
// squadai's marker blocks out of instruction files adapters used to write.
// It runs whatever components are enabled, because memory, rules, brand,
// efficiency and agents all injected blocks into the old files.
//
// Two jobs: give adapters that need it (Claude Code) an import of AGENTS.md,
// and strip squadai's blocks from each adapter's legacy files, keeping user
// content, the import block and the copilot template's block. A file squadai
// created that ends up empty is deleted.
package legacyinstructions

import (
	"fmt"
	"strings"

	"github.com/PedroMosquera/squadai/internal/components/agents"
	"github.com/PedroMosquera/squadai/internal/components/copilot"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/PedroMosquera/squadai/internal/marker"
)

const (
	// ImportSectionID is the marker block in CLAUDE.md that imports AGENTS.md.
	ImportSectionID = "agents-md"
	importContent   = "@AGENTS.md"
	importIDSuffix  = "-agents-md-import"
	legacyIDInfix   = "-legacy-instructions-"
)

// keepSections survive the legacy strip: they are written to those files on
// purpose, by this installer and by the copilot instructions template.
var keepSections = []string{ImportSectionID, copilot.SectionID}

// Options carries what the migration needs to recognize squadai content that
// is not inside a marker block.
type Options struct {
	// TeamStandards is the configured team standards text, which the rules
	// installer wrote unmarked into Cursor's and Windsurf's legacy rule files.
	TeamStandards string
}

// Installer plans, applies and verifies the import and the migration for one
// project.
type Installer struct {
	projectDir  string
	opts        Options
	frontmatter map[domain.AgentID]string
}

// New returns an Installer that untracks deleted files from the managed
// sidecar under projectDir.
func New(projectDir string, opts ...Options) *Installer {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	return &Installer{projectDir: projectDir, opts: o, frontmatter: map[domain.AgentID]string{}}
}

// ID returns ComponentInstructions: housekeeping, not a configurable
// component, so it has no key in project.json.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentInstructions
}

// ImportFile returns the file in which adapter needs an @AGENTS.md import,
// or "" when the adapter reads AGENTS.md directly.
func ImportFile(adapter domain.Adapter, projectDir string) string {
	if projectDir == "" {
		return ""
	}
	if a, ok := adapter.(interface{ ProjectImportFile(string) string }); ok {
		return a.ProjectImportFile(projectDir)
	}
	return ""
}

// LegacyFiles returns the instruction files adapter wrote before its current
// target.
func LegacyFiles(adapter domain.Adapter, homeDir, projectDir string) []string {
	if a, ok := adapter.(interface {
		LegacyInstructionsFiles(string, string) []string
	}); ok {
		return a.LegacyInstructionsFiles(homeDir, projectDir)
	}
	return nil
}

// IsMigration reports whether action moves squadai blocks out of a legacy
// instructions file.
func IsMigration(action domain.PlannedAction) bool {
	return action.Component == domain.ComponentInstructions && strings.Contains(action.ID, legacyIDInfix)
}

func legacyFrontmatter(adapter domain.Adapter) string {
	if a, ok := adapter.(interface{ LegacyRulesFrontmatter() string }); ok {
		return a.LegacyRulesFrontmatter()
	}
	return ""
}

// inspect is managed.InspectStaleKeeping plus one legacy shape: a structured
// rule file that is exactly squadai's frontmatter and team standards. The
// rules installer rewrote that file whole on every apply, so it is squadai's
// to delete even when the sidecar predates created-file tracking.
func (i *Installer) inspect(path, frontmatter string) (managed.StaleContent, bool, error) {
	data, err := fileutil.ReadFileOrEmpty(path)
	if err != nil {
		return managed.StaleContent{}, false, err
	}
	keep := keepSections
	// A populated /squadai-init block holds project-specific content that
	// AGENTS.md does not have, so it stays where the harness still reads it.
	if r := marker.ExtractSection(string(data), agents.RefinementSectionID); r != "" && r != agents.RefinementPlaceholder {
		keep = append([]string{agents.RefinementSectionID}, keep...)
	}
	stale, exists, err := managed.InspectStaleKeeping(i.projectDir, path, keep...)
	if err != nil || !exists || frontmatter == "" || stale.Kept {
		return stale, exists, err
	}
	body, ok := strings.CutPrefix(string(stale.Stripped), frontmatter)
	if !ok {
		return stale, exists, nil
	}
	body = strings.TrimSpace(body)
	if body == "" || (i.opts.TeamStandards != "" && body == strings.TrimSpace(i.opts.TeamStandards)) {
		stale = managed.StaleContent{Found: true, OnlySquadAI: true, Created: true}
	}
	return stale, exists, nil
}

type legacyFile struct {
	path  string
	stale managed.StaleContent
}

// pending returns each legacy file of adapter that still holds squadai content.
func (i *Installer) pending(adapter domain.Adapter, homeDir, projectDir string) ([]legacyFile, error) {
	current := map[string]bool{
		adapter.ProjectRulesFile(projectDir): true,
		adapter.SystemPromptFile(homeDir):    true,
	}
	fm := legacyFrontmatter(adapter)
	var out []legacyFile
	for _, path := range LegacyFiles(adapter, homeDir, projectDir) {
		if path == "" || current[path] {
			continue
		}
		stale, exists, err := i.inspect(path, fm)
		if err != nil {
			return nil, fmt.Errorf("read legacy instructions %s: %w", path, err)
		}
		if exists && stale.Found {
			out = append(out, legacyFile{path: path, stale: stale})
		}
	}
	return out, nil
}

func importCurrent(doc string) bool {
	return marker.ExtractSection(doc, ImportSectionID) == importContent
}

// Plan emits the import action, then one update per legacy file that still
// holds squadai content. The import runs first so the strip that follows
// sees it and keeps the file.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	i.frontmatter[adapter.ID()] = legacyFrontmatter(adapter)
	var actions []domain.PlannedAction

	if path := ImportFile(adapter, projectDir); path != "" {
		existing, err := fileutil.ReadFileOrEmpty(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		action := domain.PlannedAction{
			ID:          string(adapter.ID()) + importIDSuffix,
			Agent:       adapter.ID(),
			Component:   domain.ComponentInstructions,
			TargetPath:  path,
			Action:      domain.ActionUpdate,
			Description: fmt.Sprintf("import the shared AGENTS.md from %s", path),
		}
		switch {
		case importCurrent(string(existing)):
			action.Action = domain.ActionSkip
			action.Description = "AGENTS.md import already up to date"
		case len(existing) == 0:
			action.Action = domain.ActionCreate
		}
		actions = append(actions, action)
	}

	files, err := i.pending(adapter, homeDir, projectDir)
	if err != nil {
		return nil, err
	}
	for n, f := range files {
		desc := fmt.Sprintf("remove squadai blocks from %s; shared instructions now live in AGENTS.md", f.path)
		if f.stale.Deletable() {
			desc = fmt.Sprintf("delete %s: it holds only squadai blocks, which now live in AGENTS.md", f.path)
		}
		actions = append(actions, domain.PlannedAction{
			ID:          fmt.Sprintf("%s%s%d", adapter.ID(), legacyIDInfix, n),
			Agent:       adapter.ID(),
			Component:   domain.ComponentInstructions,
			Action:      domain.ActionUpdate,
			TargetPath:  f.path,
			Description: desc,
		})
	}
	return actions, nil
}

// RenderContent returns what Apply would leave at action.TargetPath; nil
// means the file would be deleted.
func (i *Installer) RenderContent(action domain.PlannedAction) ([]byte, error) {
	if strings.HasSuffix(action.ID, importIDSuffix) {
		existing, err := fileutil.ReadFileOrEmpty(action.TargetPath)
		if err != nil {
			return nil, err
		}
		return []byte(marker.InjectSection(string(existing), ImportSectionID, importContent)), nil
	}
	stale, exists, err := i.inspect(action.TargetPath, i.frontmatter[action.Agent])
	if err != nil || !exists {
		return nil, err
	}
	if stale.Deletable() {
		return nil, nil
	}
	return stale.Stripped, nil
}

// Apply writes the import block, or re-inspects a legacy file and strips or
// deletes it with managed.ApplyStale semantics.
func (i *Installer) Apply(action domain.PlannedAction) error {
	if action.Action == domain.ActionSkip {
		return nil
	}
	if strings.HasSuffix(action.ID, importIDSuffix) {
		updated, err := i.RenderContent(action)
		if err != nil {
			return fmt.Errorf("read %s: %w", action.TargetPath, err)
		}
		if _, err := fileutil.WriteAtomic(action.TargetPath, updated, 0644); err != nil {
			return fmt.Errorf("write %s: %w", action.TargetPath, err)
		}
		return nil
	}
	stale, exists, err := i.inspect(action.TargetPath, i.frontmatter[action.Agent])
	if err != nil || !exists {
		return err
	}
	return managed.ApplyStale(i.projectDir, action.TargetPath, stale)
}

// Verify fails once per legacy file that still holds squadai content, and
// when an adapter's AGENTS.md import is missing.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	var results []domain.VerifyResult
	if path := ImportFile(adapter, projectDir); path != "" {
		existing, err := fileutil.ReadFileOrEmpty(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		r := domain.VerifyResult{Check: "instructions-agents-md-import", Passed: importCurrent(string(existing))}
		if !r.Passed {
			r.Severity = domain.SeverityError
			r.Message = fmt.Sprintf("%s does not import AGENTS.md; run 'squadai apply'", path)
		}
		results = append(results, r)
	}
	files, err := i.pending(adapter, homeDir, projectDir)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		results = append(results, domain.VerifyResult{
			Check:    "instructions-legacy-migrated",
			Passed:   false,
			Severity: domain.SeverityError,
			Message: fmt.Sprintf("squadai blocks remain in %s; shared instructions now live in AGENTS.md, run 'squadai apply' to remove them",
				f.path),
		})
	}
	return results, nil
}
