package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/marker"
)

const (
	// SectionID is the marker section identifier for team standards content.
	SectionID = "team-standards"
)

// Installer implements domain.ComponentInstaller for the rules component.
// It manages team standards sections in project-level instruction files
// (AGENTS.md for OpenCode, CLAUDE.md for Claude Code).
// For Windsurf and Cursor, it writes structured rules files with YAML frontmatter.
type Installer struct {
	// content is the resolved team standards content to inject.
	content string

	// agentFrontmatter caches adapter-declared frontmatter per agent, populated during Plan.
	agentFrontmatter map[domain.AgentID]string
}

// New returns a rules component installer with the resolved team standards content.
// The content is resolved once at construction from RulesConfig:
//   - If TeamStandards is non-empty, use it directly (inline content).
//   - If TeamStandardsFile is non-empty, read from .squadai/<path> in projectDir.
//   - If both are empty, the installer produces no actions.
func New(cfg domain.RulesConfig, projectDir string) (*Installer, error) {
	content, err := resolveContent(cfg, projectDir)
	if err != nil {
		return nil, fmt.Errorf("resolve rules content: %w", err)
	}
	return &Installer{content: content, agentFrontmatter: make(map[domain.AgentID]string)}, nil
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

	// Cache adapter-declared frontmatter for use during Apply.
	fm := adapter.RulesFrontmatter()
	i.agentFrontmatter[adapter.ID()] = fm

	// Structured rules (Windsurf/Cursor) use frontmatter comparison, not markers.
	if fm != "" {
		expectedContent := fm + i.content
		if !strings.HasSuffix(expectedContent, "\n") {
			expectedContent += "\n"
		}
		if string(existing) == expectedContent {
			return []domain.PlannedAction{
				{
					ID:          actionID,
					Agent:       adapter.ID(),
					Component:   domain.ComponentRules,
					Action:      domain.ActionSkip,
					TargetPath:  targetPath,
					Description: "structured rules file already up to date",
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
				Description: "write structured rules file",
			},
		}, nil
	}

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

	var updated string

	// Structured rules formats: Windsurf (.md with trigger frontmatter)
	// and Cursor (.mdc with alwaysApply frontmatter) use full-file replacement
	// with YAML frontmatter instead of marker-based injection.
	if fm := i.agentFrontmatter[action.Agent]; fm != "" {
		updated = fm + i.content
		if !strings.HasSuffix(updated, "\n") {
			updated += "\n"
		}
	} else {
		updated = marker.InjectSection(string(existing), SectionID, i.content)
	}

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

	// Structured rules (Windsurf/Cursor) use frontmatter, not markers.
	if fm := adapter.RulesFrontmatter(); fm != "" {
		expectedContent := fm + i.content
		if !strings.HasSuffix(expectedContent, "\n") {
			expectedContent += "\n"
		}
		if doc == expectedContent {
			results = append(results, domain.VerifyResult{
				Check:  "rules-content-current",
				Passed: true,
			})
		} else {
			results = append(results, domain.VerifyResult{
				Check:   "rules-content-current",
				Passed:  false,
				Message: "structured rules content is outdated",
			})
		}
		return results, nil
	}

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
func resolveContent(cfg domain.RulesConfig, projectDir string) (string, error) {
	// Inline content takes precedence.
	if cfg.TeamStandards != "" {
		return cfg.TeamStandards, nil
	}

	// File reference: relative to .squadai/ in projectDir.
	if cfg.TeamStandardsFile != "" && projectDir != "" {
		filePath := filepath.Join(projectDir, ".squadai", cfg.TeamStandardsFile)
		data, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", nil // file not yet created — skip silently
			}
			return "", fmt.Errorf("read team standards file: %w", err)
		}
		return string(data), nil
	}

	return "", nil
}
