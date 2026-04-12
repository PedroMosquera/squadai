package rules

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/fileutil"
	"github.com/PedroMosquera/agent-manager-pro/internal/marker"
)

const (
	// SectionID is the marker section identifier for team standards content.
	SectionID = "team-standards"
)

// Installer implements domain.ComponentInstaller for the rules component.
// It manages team standards sections in project-level instruction files
// (AGENTS.md for OpenCode, CLAUDE.md for Claude Code).
type Installer struct {
	// content is the resolved team standards content to inject.
	content string
}

// New returns a rules component installer with the resolved team standards content.
// The content is resolved once at construction from RulesConfig:
//   - If TeamStandards is non-empty, use it directly (inline content).
//   - If TeamStandardsFile is non-empty, read from .agent-manager/<path> in projectDir.
//   - If both are empty, the installer produces no actions.
func New(cfg domain.RulesConfig, projectDir string) *Installer {
	content := resolveContent(cfg, projectDir)
	return &Installer{content: content}
}

// ID returns the component identifier.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentRules
}

// Content returns the resolved team standards content. Empty means no rules configured.
func (i *Installer) Content() string {
	return i.content
}

// Plan determines what actions are needed for this adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	if !adapter.SupportsComponent(domain.ComponentRules) {
		return nil, nil
	}

	if i.content == "" {
		return nil, nil
	}

	targetPath := adapter.ProjectRulesFile(projectDir)
	if targetPath == "" {
		return nil, nil
	}

	existing, err := fileutil.ReadFileOrEmpty(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read rules file: %w", err)
	}

	actionID := fmt.Sprintf("%s-rules", adapter.ID())

	if marker.HasSection(string(existing), SectionID) {
		current := marker.ExtractSection(string(existing), SectionID)
		if current == i.content {
			return []domain.PlannedAction{
				{
					ID:          actionID,
					Agent:       adapter.ID(),
					Component:   domain.ComponentRules,
					Action:      domain.ActionSkip,
					TargetPath:  targetPath,
					Description: "team standards section already up to date",
				},
			}, nil
		}
		return []domain.PlannedAction{
			{
				ID:          actionID,
				Agent:       adapter.ID(),
				Component:   domain.ComponentRules,
				Action:      domain.ActionUpdate,
				TargetPath:  targetPath,
				Description: "update team standards section",
			},
		}, nil
	}

	action := domain.ActionCreate
	if len(existing) > 0 {
		action = domain.ActionUpdate
	}

	return []domain.PlannedAction{
		{
			ID:          actionID,
			Agent:       adapter.ID(),
			Component:   domain.ComponentRules,
			Action:      action,
			TargetPath:  targetPath,
			Description: "inject team standards section",
		},
	}, nil
}

// Apply executes a single planned action.
func (i *Installer) Apply(action domain.PlannedAction) error {
	if action.Action == domain.ActionSkip {
		return nil
	}

	existing, err := fileutil.ReadFileOrEmpty(action.TargetPath)
	if err != nil {
		return fmt.Errorf("read target: %w", err)
	}

	updated := marker.InjectSection(string(existing), SectionID, i.content)

	_, err = fileutil.WriteAtomic(action.TargetPath, []byte(updated), 0644)
	if err != nil {
		return fmt.Errorf("write target: %w", err)
	}

	return nil
}

// Verify checks post-apply state for the rules component.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsComponent(domain.ComponentRules) {
		return nil, nil
	}

	if i.content == "" {
		return nil, nil
	}

	targetPath := adapter.ProjectRulesFile(projectDir)
	if targetPath == "" {
		return nil, nil
	}

	var results []domain.VerifyResult

	data, err := os.ReadFile(targetPath)
	if err != nil {
		results = append(results, domain.VerifyResult{
			Check:   "rules-file-exists",
			Passed:  false,
			Message: fmt.Sprintf("rules file not found: %s", targetPath),
		})
		return results, nil
	}
	results = append(results, domain.VerifyResult{
		Check:  "rules-file-exists",
		Passed: true,
	})

	doc := string(data)
	if !marker.HasSection(doc, SectionID) {
		results = append(results, domain.VerifyResult{
			Check:   "rules-markers-present",
			Passed:  false,
			Message: "team standards marker section not found",
		})
		return results, nil
	}
	results = append(results, domain.VerifyResult{
		Check:  "rules-markers-present",
		Passed: true,
	})

	current := marker.ExtractSection(doc, SectionID)
	if current != i.content {
		results = append(results, domain.VerifyResult{
			Check:   "rules-content-current",
			Passed:  false,
			Message: "team standards content is outdated",
		})
	} else {
		results = append(results, domain.VerifyResult{
			Check:  "rules-content-current",
			Passed: true,
		})
	}

	return results, nil
}

// resolveContent resolves team standards content from RulesConfig.
func resolveContent(cfg domain.RulesConfig, projectDir string) string {
	// Inline content takes precedence.
	if cfg.TeamStandards != "" {
		return cfg.TeamStandards
	}

	// File reference: relative to .agent-manager/ in projectDir.
	if cfg.TeamStandardsFile != "" && projectDir != "" {
		filePath := filepath.Join(projectDir, ".agent-manager", cfg.TeamStandardsFile)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "" // silently skip if file not found
		}
		return string(data)
	}

	return ""
}
