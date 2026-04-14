package workflows

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PedroMosquera/agent-manager-pro/internal/assets"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/fileutil"
)

// methodologyWorkflowFile maps a methodology to its workflow asset filename.
var methodologyWorkflowFile = map[domain.Methodology]string{
	domain.MethodologyTDD:          "tdd-pipeline.md",
	domain.MethodologySDD:          "sdd-pipeline.md",
	domain.MethodologyConventional: "conventional-pipeline.md",
}

// Installer implements domain.ComponentInstaller for Windsurf workflow files.
// Only adapters that return true from SupportsWorkflows() get workflow files.
type Installer struct {
	config *domain.MergedConfig
}

// New returns a workflows installer configured from merged config.
func New(cfg *domain.MergedConfig) *Installer {
	return &Installer{config: cfg}
}

// ID returns the component identifier.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentWorkflows
}

// Plan determines what workflow actions are needed for the given adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	if !adapter.SupportsWorkflows() {
		return nil, nil
	}

	if i.config == nil || i.config.Methodology == "" {
		return nil, nil
	}

	filename, ok := methodologyWorkflowFile[i.config.Methodology]
	if !ok {
		return nil, nil
	}

	workflowsDir := adapter.WorkflowsDir(projectDir)
	if workflowsDir == "" {
		return nil, nil
	}

	// Load expected content from embedded assets.
	content, err := assets.Read("workflows/" + filename)
	if err != nil {
		return nil, fmt.Errorf("read workflow asset %s: %w", filename, err)
	}

	targetPath := filepath.Join(workflowsDir, filename)
	actionID := fmt.Sprintf("%s-workflow-%s", adapter.ID(), filename)
	description := fmt.Sprintf("workflow:%s:%s", i.config.Methodology, filename)

	existing, err := fileutil.ReadFileOrEmpty(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read workflow %s: %w", filename, err)
	}

	if string(existing) == content {
		return []domain.PlannedAction{{
			ID:          actionID,
			Agent:       adapter.ID(),
			Component:   domain.ComponentWorkflows,
			Action:      domain.ActionSkip,
			TargetPath:  targetPath,
			Description: description,
		}}, nil
	}

	action := domain.ActionCreate
	if len(existing) > 0 {
		action = domain.ActionUpdate
	}

	return []domain.PlannedAction{{
		ID:          actionID,
		Agent:       adapter.ID(),
		Component:   domain.ComponentWorkflows,
		Action:      action,
		TargetPath:  targetPath,
		Description: description,
	}}, nil
}

// Apply executes a single planned action.
func (i *Installer) Apply(action domain.PlannedAction) error {
	if action.Action == domain.ActionSkip {
		return nil
	}

	if i.config == nil || i.config.Methodology == "" {
		return fmt.Errorf("no methodology configured for workflow apply")
	}

	filename, ok := methodologyWorkflowFile[i.config.Methodology]
	if !ok {
		return fmt.Errorf("unknown methodology: %s", i.config.Methodology)
	}

	content, err := assets.Read("workflows/" + filename)
	if err != nil {
		return fmt.Errorf("read workflow asset %s: %w", filename, err)
	}

	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create workflows dir: %w", err)
	}

	if _, err := fileutil.WriteAtomic(action.TargetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write workflow: %w", err)
	}

	return nil
}

// Verify checks post-apply state for the workflows component.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsWorkflows() {
		return nil, nil
	}

	if i.config == nil || i.config.Methodology == "" {
		return nil, nil
	}

	filename, ok := methodologyWorkflowFile[i.config.Methodology]
	if !ok {
		return nil, nil
	}

	workflowsDir := adapter.WorkflowsDir(projectDir)
	if workflowsDir == "" {
		return nil, nil
	}

	expected, err := assets.Read("workflows/" + filename)
	if err != nil {
		return nil, fmt.Errorf("read workflow asset for verify %s: %w", filename, err)
	}

	targetPath := filepath.Join(workflowsDir, filename)
	checkName := fmt.Sprintf("workflow-%s", filename)

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return []domain.VerifyResult{{
			Check:     checkName + "-exists",
			Passed:    false,
			Severity:  domain.SeverityError,
			Component: "workflows",
			Message:   fmt.Sprintf("workflow file not found: %s", targetPath),
		}}, nil
	}

	if string(data) == expected {
		return []domain.VerifyResult{{
			Check:     checkName + "-current",
			Passed:    true,
			Severity:  domain.SeverityInfo,
			Component: "workflows",
		}}, nil
	}

	return []domain.VerifyResult{{
		Check:     checkName + "-current",
		Passed:    false,
		Severity:  domain.SeverityError,
		Component: "workflows",
		Message:   fmt.Sprintf("workflow %s content does not match expected", filename),
	}}, nil
}
