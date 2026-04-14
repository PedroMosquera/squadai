package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
)

// Installer implements domain.ComponentInstaller for skill definitions.
// It writes .opencode/skills/<name>/SKILL.md files with YAML frontmatter.
// When config.Methodology is set, it also installs embedded methodology skills.
type Installer struct {
	skills     map[string]domain.SkillDef // custom skills from config
	config     *domain.MergedConfig       // methodology config (nil = V1 behavior)
	projectDir string
}

// New returns a skills installer configured from the merged skill definitions.
// cfg may be nil for backward compatibility (V1 behavior: only custom skills).
func New(skills map[string]domain.SkillDef, cfg *domain.MergedConfig, projectDir string) *Installer {
	resolved := make(map[string]domain.SkillDef)
	for name, def := range skills {
		// Resolve content from file if needed.
		if def.Content == "" && def.ContentFile != "" && projectDir != "" {
			filePath := filepath.Join(projectDir, ".squadai", def.ContentFile)
			data, err := os.ReadFile(filePath)
			if err == nil {
				def.Content = string(data)
			}
		}
		resolved[name] = def
	}
	return &Installer{
		skills:     resolved,
		config:     cfg,
		projectDir: projectDir,
	}
}

// ID returns the component identifier.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentSkills
}

// Plan determines what skill file actions are needed for the given adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	if !adapter.SupportsComponent(domain.ComponentSkills) {
		return nil, nil
	}

	skillsDir := adapter.ProjectSkillsDir(projectDir)
	if skillsDir == "" {
		return nil, nil
	}

	var actions []domain.PlannedAction

	// Phase 1: Embedded methodology skills (V2 behavior).
	if i.config != nil && i.config.Methodology != "" {
		// Shared skills (always installed regardless of methodology).
		for _, assetDir := range sharedSkillPaths() {
			a, err := i.planEmbeddedSkill(adapter, assetDir, skillsDir)
			if err != nil {
				return nil, err
			}
			if a != nil {
				actions = append(actions, *a)
			}
		}
		// Methodology-specific skills.
		for _, assetDir := range methodologySkillPaths(i.config.Methodology) {
			a, err := i.planEmbeddedSkill(adapter, assetDir, skillsDir)
			if err != nil {
				return nil, err
			}
			if a != nil {
				actions = append(actions, *a)
			}
		}
	}

	// Phase 2: Custom skills from config (V1 behavior).
	names := sortedKeys(i.skills)
	for _, name := range names {
		def := i.skills[name]
		targetPath := filepath.Join(skillsDir, name, "SKILL.md")
		content := renderSkill(name, def)
		actionID := fmt.Sprintf("%s-skill-%s", adapter.ID(), name)

		existing, err := fileutil.ReadFileOrEmpty(targetPath)
		if err != nil {
			return nil, fmt.Errorf("read skill %s: %w", name, err)
		}

		if string(existing) == content {
			actions = append(actions, domain.PlannedAction{
				ID:          actionID,
				Agent:       adapter.ID(),
				Component:   domain.ComponentSkills,
				Action:      domain.ActionSkip,
				TargetPath:  targetPath,
				Description: fmt.Sprintf("skill %s already up to date", name),
			})
			continue
		}

		action := domain.ActionCreate
		if len(existing) > 0 {
			action = domain.ActionUpdate
		}

		actions = append(actions, domain.PlannedAction{
			ID:          actionID,
			Agent:       adapter.ID(),
			Component:   domain.ComponentSkills,
			Action:      action,
			TargetPath:  targetPath,
			Description: fmt.Sprintf("%s skill %s", action, name),
		})
	}

	return actions, nil
}

// planEmbeddedSkill produces a PlannedAction for a single embedded skill asset.
// assetDir is the directory under assets (e.g., "skills/tdd/brainstorming").
// skillsDir is the project-level skills directory for the adapter.
func (i *Installer) planEmbeddedSkill(adapter domain.Adapter, assetDir, skillsDir string) (*domain.PlannedAction, error) {
	assetPath := assetDir + "/SKILL.md"
	content, err := assets.Read(assetPath)
	if err != nil {
		return nil, fmt.Errorf("read embedded skill %s: %w", assetPath, err)
	}

	// Target: <skillsDir>/<relative>/SKILL.md
	// e.g., assetDir = "skills/tdd/brainstorming" → relative = "tdd/brainstorming"
	relative := strings.TrimPrefix(assetDir, "skills/")
	targetPath := filepath.Join(skillsDir, relative, "SKILL.md")
	actionID := fmt.Sprintf("skill-embedded-%s", relative)
	description := fmt.Sprintf("skill:embedded:%s", relative)

	existing, err := fileutil.ReadFileOrEmpty(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read skill target %s: %w", targetPath, err)
	}

	if string(existing) == content {
		return &domain.PlannedAction{
			ID:          actionID,
			Agent:       adapter.ID(),
			Component:   domain.ComponentSkills,
			Action:      domain.ActionSkip,
			TargetPath:  targetPath,
			Description: description,
		}, nil
	}

	action := domain.ActionCreate
	if len(existing) > 0 {
		action = domain.ActionUpdate
	}

	return &domain.PlannedAction{
		ID:          actionID,
		Agent:       adapter.ID(),
		Component:   domain.ComponentSkills,
		Action:      action,
		TargetPath:  targetPath,
		Description: description,
	}, nil
}

// Apply executes a single planned action.
func (i *Installer) Apply(action domain.PlannedAction) error {
	if action.Action == domain.ActionSkip {
		return nil
	}

	if action.Action == domain.ActionDelete {
		// Delete the skill directory.
		dir := filepath.Dir(action.TargetPath)
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("delete skill: %w", err)
		}
		return nil
	}

	// Embedded skill: description starts with "skill:embedded:"
	if strings.HasPrefix(action.Description, "skill:embedded:") {
		return i.applyEmbeddedSkill(action)
	}

	// Custom skill: extract skill name from path: .../skills/<name>/SKILL.md
	return i.applyCustomSkill(action)
}

// applyEmbeddedSkill writes an embedded skill asset to the target path.
func (i *Installer) applyEmbeddedSkill(action domain.PlannedAction) error {
	// Reconstruct the asset path from the description.
	// description = "skill:embedded:tdd/brainstorming"
	relative := strings.TrimPrefix(action.Description, "skill:embedded:")
	assetPath := "skills/" + relative + "/SKILL.md"

	content, err := assets.Read(assetPath)
	if err != nil {
		return fmt.Errorf("read embedded skill %s: %w", assetPath, err)
	}

	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create skill dir: %w", err)
	}

	if _, err := fileutil.WriteAtomic(action.TargetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write embedded skill: %w", err)
	}

	return nil
}

// applyCustomSkill writes a custom skill definition from config.
func (i *Installer) applyCustomSkill(action domain.PlannedAction) error {
	dir := filepath.Dir(action.TargetPath)
	name := filepath.Base(dir)
	def, ok := i.skills[name]
	if !ok {
		return fmt.Errorf("skill %q not found in config", name)
	}

	content := renderSkill(name, def)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create skill dir: %w", err)
	}

	if _, err := fileutil.WriteAtomic(action.TargetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write skill: %w", err)
	}

	return nil
}

// Verify checks post-apply state for the skills component.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsComponent(domain.ComponentSkills) {
		return nil, nil
	}

	skillsDir := adapter.ProjectSkillsDir(projectDir)
	if skillsDir == "" {
		return nil, nil
	}

	var results []domain.VerifyResult

	// Verify embedded methodology skills.
	if i.config != nil && i.config.Methodology != "" {
		for _, assetDir := range sharedSkillPaths() {
			r := i.verifyEmbeddedSkill(assetDir, skillsDir)
			if r != nil {
				results = append(results, *r)
			}
		}
		for _, assetDir := range methodologySkillPaths(i.config.Methodology) {
			r := i.verifyEmbeddedSkill(assetDir, skillsDir)
			if r != nil {
				results = append(results, *r)
			}
		}
	}

	// Verify custom skills.
	if len(i.skills) == 0 {
		return results, nil
	}

	for _, name := range sortedKeys(i.skills) {
		def := i.skills[name]
		targetPath := filepath.Join(skillsDir, name, "SKILL.md")
		data, err := os.ReadFile(targetPath)
		if err != nil {
			results = append(results, domain.VerifyResult{
				Check:   fmt.Sprintf("skill-%s-exists", name),
				Passed:  false,
				Message: fmt.Sprintf("skill file not found: %s", targetPath),
			})
			continue
		}

		expected := renderSkill(name, def)
		if string(data) == expected {
			results = append(results, domain.VerifyResult{
				Check:  fmt.Sprintf("skill-%s-current", name),
				Passed: true,
			})
		} else {
			results = append(results, domain.VerifyResult{
				Check:   fmt.Sprintf("skill-%s-current", name),
				Passed:  false,
				Message: fmt.Sprintf("skill %s content does not match expected", name),
			})
		}
	}

	return results, nil
}

// verifyEmbeddedSkill checks that an embedded skill file exists and is current.
func (i *Installer) verifyEmbeddedSkill(assetDir, skillsDir string) *domain.VerifyResult {
	assetPath := assetDir + "/SKILL.md"
	expected, err := assets.Read(assetPath)
	if err != nil {
		return nil // asset doesn't exist; skip
	}

	relative := strings.TrimPrefix(assetDir, "skills/")
	targetPath := filepath.Join(skillsDir, relative, "SKILL.md")
	checkName := fmt.Sprintf("skill-embedded-%s", relative)

	data, err := os.ReadFile(targetPath)
	if err != nil {
		r := domain.VerifyResult{
			Check:   checkName + "-exists",
			Passed:  false,
			Message: fmt.Sprintf("embedded skill file not found: %s", targetPath),
		}
		return &r
	}

	if string(data) == expected {
		r := domain.VerifyResult{
			Check:  checkName + "-current",
			Passed: true,
		}
		return &r
	}

	r := domain.VerifyResult{
		Check:   checkName + "-current",
		Passed:  false,
		Message: fmt.Sprintf("embedded skill %s content does not match", relative),
	}
	return &r
}

// methodologySkillPaths returns the embedded skill asset directories for a methodology.
// Each path is relative to the assets root (e.g., "skills/tdd/brainstorming").
func methodologySkillPaths(m domain.Methodology) []string {
	switch m {
	case domain.MethodologyTDD:
		return []string{
			"skills/tdd/brainstorming",
			"skills/tdd/writing-plans",
			"skills/tdd/test-driven-development",
			"skills/tdd/subagent-driven-development",
			"skills/tdd/systematic-debugging",
		}
	case domain.MethodologySDD:
		return []string{
			"skills/sdd/sdd-explore",
			"skills/sdd/sdd-propose",
			"skills/sdd/sdd-spec",
			"skills/sdd/sdd-design",
			"skills/sdd/sdd-tasks",
			"skills/sdd/sdd-apply",
			"skills/sdd/sdd-verify",
		}
	default:
		// Conventional uses shared skills only.
		return nil
	}
}

// sharedSkillPaths returns the shared embedded skill asset directories.
func sharedSkillPaths() []string {
	return []string{
		"skills/shared/code-review",
		"skills/shared/find-skills",
		"skills/shared/pr-description",
		"skills/shared/testing",
	}
}

// renderSkill generates the markdown content for a skill definition with YAML frontmatter.
func renderSkill(name string, def domain.SkillDef) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("description: %s\n", def.Description))
	b.WriteString("---\n")
	if def.Content != "" {
		b.WriteString("\n")
		b.WriteString(def.Content)
		if !strings.HasSuffix(def.Content, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// RenderContent returns the content that Apply would write for the given action,
// without performing the write. Used by the diff renderer.
func (i *Installer) RenderContent(action domain.PlannedAction) (string, error) {
	if strings.HasPrefix(action.Description, "skill:embedded:") {
		relative := strings.TrimPrefix(action.Description, "skill:embedded:")
		assetPath := "skills/" + relative + "/SKILL.md"
		content, err := assets.Read(assetPath)
		if err != nil {
			return "", fmt.Errorf("read embedded skill %s: %w", assetPath, err)
		}
		return content, nil
	}

	// Custom skill: extract name from path.
	dir := filepath.Dir(action.TargetPath)
	name := filepath.Base(dir)
	def, ok := i.skills[name]
	if !ok {
		return "", fmt.Errorf("skill %q not found in config", name)
	}
	return renderSkill(name, def), nil
}

func sortedKeys(m map[string]domain.SkillDef) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
