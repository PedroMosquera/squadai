package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/fileutil"
)

// Installer implements domain.ComponentInstaller for skill definitions.
// It writes .opencode/skills/<name>/SKILL.md files with YAML frontmatter.
type Installer struct {
	skills     map[string]domain.SkillDef
	projectDir string
}

// New returns a skills installer configured from the merged skill definitions.
func New(skills map[string]domain.SkillDef, projectDir string) *Installer {
	resolved := make(map[string]domain.SkillDef)
	for name, def := range skills {
		// Resolve content from file if needed.
		if def.Content == "" && def.ContentFile != "" && projectDir != "" {
			filePath := filepath.Join(projectDir, ".agent-manager", def.ContentFile)
			data, err := os.ReadFile(filePath)
			if err == nil {
				def.Content = string(data)
			}
		}
		resolved[name] = def
	}
	return &Installer{skills: resolved, projectDir: projectDir}
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

	// Extract skill name from path: .../skills/<name>/SKILL.md
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

	if len(i.skills) == 0 {
		return nil, nil
	}

	var results []domain.VerifyResult

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

// renderSkill generates the markdown content for a skill definition
// with YAML frontmatter.
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

func sortedKeys(m map[string]domain.SkillDef) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
