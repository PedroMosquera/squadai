package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
)

// Options controls optional skills installer behavior.
type Options struct {
	// Scopes filters which skills are installed. Nil means no filter (install
	// everything). A scope matches a skill's asset path either exactly
	// ("tdd/systematic-debugging") or as a group prefix ("shared" matches
	// "shared/code-review"). Asset paths keep their group; installed skills
	// do not (see skillName).
	Scopes []string
}

// Installer implements domain.ComponentInstaller for skill definitions.
// It writes <ProjectSkillsDir>/<name>/SKILL.md files with YAML frontmatter.
// Embedded skills are grouped in assets (shared/, tdd/, sdd/) but installed
// flat, because Claude Code, VS Code and Windsurf document only one level.
// When config.Methodology is set, it also installs embedded methodology skills.
type Installer struct {
	skills     map[string]domain.SkillDef // custom skills from config
	config     *domain.MergedConfig       // methodology config (nil = V1 behavior)
	projectDir string
	opts       Options
}

// New returns a skills installer configured from the merged skill definitions.
// cfg may be nil for backward compatibility (V1 behavior: only custom skills).
func New(skills map[string]domain.SkillDef, cfg *domain.MergedConfig, projectDir string, opts ...Options) *Installer {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
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
		opts:       o,
	}
}

// inScope reports whether a skill asset path (e.g. "shared/code-review") or
// custom skill name passes the scope filter.
// A nil filter admits everything; scopes match exactly or as a directory
// prefix ("shared" → "shared/...", "tdd" → "tdd/...").
func (i *Installer) inScope(rel string) bool {
	if i.opts.Scopes == nil {
		return true
	}
	for _, scope := range i.opts.Scopes {
		if scope == "" {
			continue
		}
		if rel == scope || strings.HasPrefix(rel, scope+"/") {
			return true
		}
	}
	return false
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
		if !i.inScope(name) {
			continue
		}
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

	removals, err := i.planLegacyRemoval(adapter, projectDir, skillsDir)
	if err != nil {
		return nil, err
	}
	return append(actions, removals...), nil
}

// legacyProjectSkillsDirs are the per-harness directories squadai wrote skills
// to before it wrote them once to .agents/skills, which these harnesses read.
var legacyProjectSkillsDirs = map[domain.AgentID]string{
	domain.AgentOpenCode:      ".opencode/skills",
	domain.AgentPi:            ".pi/skills",
	domain.AgentCursor:        ".cursor/skills",
	domain.AgentVSCodeCopilot: ".copilot/skills",
	domain.AgentWindsurf:      ".windsurf/skills",
}

// planLegacyRemoval returns delete actions for old copies that are
// byte-identical to what Plan now writes: the grouped layout
// (<dir>/<group>/<name>/SKILL.md) in skillsDir, and any copy in the adapter's
// legacy directory. A copy the user edited is not squadai's to remove and
// stays.
func (i *Installer) planLegacyRemoval(adapter domain.Adapter, projectDir, skillsDir string) ([]domain.PlannedAction, error) {
	desired, err := i.desired()
	if err != nil {
		return nil, err
	}
	rels := make([]string, 0, len(desired))
	for rel := range desired {
		rels = append(rels, rel)
	}
	sort.Strings(rels)

	legacy, hasLegacy := legacyProjectSkillsDirs[adapter.ID()]
	legacyDir := filepath.Join(projectDir, filepath.FromSlash(legacy))
	hasLegacy = hasLegacy && legacyDir != skillsDir

	var actions []domain.PlannedAction
	for _, rel := range rels {
		var candidates []string
		if strings.Contains(rel, "/") {
			candidates = append(candidates, filepath.Join(skillsDir, filepath.FromSlash(rel), "SKILL.md"))
		}
		if hasLegacy {
			candidates = append(candidates, filepath.Join(legacyDir, filepath.FromSlash(rel), "SKILL.md"))
			if strings.Contains(rel, "/") {
				candidates = append(candidates, filepath.Join(legacyDir, skillName(rel), "SKILL.md"))
			}
		}
		for _, targetPath := range candidates {
			existing, err := fileutil.ReadFileOrEmpty(targetPath)
			if err != nil {
				return nil, fmt.Errorf("read legacy skill %s: %w", targetPath, err)
			}
			if string(existing) != desired[rel] {
				continue
			}
			actions = append(actions, domain.PlannedAction{
				ID:          fmt.Sprintf("%s-skill-legacy-%s", adapter.ID(), filepath.ToSlash(targetPath)),
				Agent:       adapter.ID(),
				Component:   domain.ComponentSkills,
				Action:      domain.ActionDelete,
				TargetPath:  targetPath,
				Description: fmt.Sprintf("delete old copy of skill %s (now in %s)", skillName(rel), skillsDir),
			})
		}
	}
	return actions, nil
}

// skillName is the installed directory name for a skill asset path or custom
// skill name. It equals the SKILL.md frontmatter name, which VS Code and
// Cursor require to match the directory.
func skillName(rel string) string {
	return path.Base(rel)
}

// shadowedByCustom reports whether a custom skill from config claims the
// directory an embedded skill would be installed to. The user's definition
// wins.
func (i *Installer) shadowedByCustom(rel string) bool {
	_, ok := i.skills[skillName(rel)]
	return ok && strings.Contains(rel, "/")
}

// desired returns the content Plan writes for each in-scope skill, keyed by
// its slash-separated path relative to the skills directory.
func (i *Installer) desired() (map[string]string, error) {
	out := make(map[string]string)
	if i.config != nil && i.config.Methodology != "" {
		paths := append(sharedSkillPaths(), methodologySkillPaths(i.config.Methodology)...)
		for _, assetDir := range paths {
			rel := strings.TrimPrefix(assetDir, "skills/")
			if !i.inScope(rel) || i.shadowedByCustom(rel) {
				continue
			}
			content, err := assets.Read(assetDir + "/SKILL.md")
			if err != nil {
				return nil, fmt.Errorf("read embedded skill %s: %w", assetDir, err)
			}
			out[rel] = content
		}
	}
	for name, def := range i.skills {
		if i.inScope(name) {
			out[name] = renderSkill(name, def)
		}
	}
	return out, nil
}

// planEmbeddedSkill produces a PlannedAction for a single embedded skill asset.
// assetDir is the directory under assets (e.g., "skills/tdd/brainstorming").
// skillsDir is the project-level skills directory for the adapter.
func (i *Installer) planEmbeddedSkill(adapter domain.Adapter, assetDir, skillsDir string) (*domain.PlannedAction, error) {
	relative := strings.TrimPrefix(assetDir, "skills/")
	if !i.inScope(relative) || i.shadowedByCustom(relative) {
		return nil, nil
	}

	assetPath := assetDir + "/SKILL.md"
	content, err := assets.Read(assetPath)
	if err != nil {
		return nil, fmt.Errorf("read embedded skill %s: %w", assetPath, err)
	}
	targetPath := filepath.Join(skillsDir, skillName(relative), "SKILL.md")
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
		if err := os.Remove(action.TargetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete skill: %w", err)
		}
		// Not RemoveAll: the folder may also hold the user's scripts or references.
		_ = os.Remove(filepath.Dir(action.TargetPath))
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

	i.sweepEmptyGroupDirs(filepath.Dir(dir))
	return nil
}

// skillGroups are the asset groups that earlier versions also used as
// directories under the skills dir.
var skillGroups = []string{"shared", "sdd", "tdd"}

// sweepEmptyGroupDirs removes group folders, and emptied legacy skills
// folders, that hold no files. Earlier versions deleted skill files but left
// their folders, and the executor's file deletes cannot reach those.
func (i *Installer) sweepEmptyGroupDirs(skillsDir string) {
	roots := []string{skillsDir}
	if i.projectDir != "" {
		for _, legacy := range legacyProjectSkillsDirs {
			roots = append(roots, filepath.Join(i.projectDir, filepath.FromSlash(legacy)))
		}
	}
	for _, root := range roots {
		for _, group := range skillGroups {
			removeEmptyDirs(filepath.Join(root, group))
		}
		if root != skillsDir {
			_ = os.Remove(root)
		}
	}
}

// removeEmptyDirs removes dir and its subdirectories deepest first. os.Remove
// refuses a directory that still holds anything, so files are never touched.
func removeEmptyDirs(dir string) {
	var dirs []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			dirs = append(dirs, p)
		}
		return nil
	})
	for k := len(dirs) - 1; k >= 0; k-- {
		_ = os.Remove(dirs[k])
	}
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
		if !i.inScope(name) {
			continue
		}
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
	relative := strings.TrimPrefix(assetDir, "skills/")
	if !i.inScope(relative) || i.shadowedByCustom(relative) {
		return nil // filtered out by the active context profile
	}

	assetPath := assetDir + "/SKILL.md"
	expected, err := assets.Read(assetPath)
	if err != nil {
		return nil // asset doesn't exist; skip
	}
	targetPath := filepath.Join(skillsDir, skillName(relative), "SKILL.md")
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
		"skills/shared/context-discipline",
		"skills/shared/find-skills",
		"skills/shared/pr-description",
		"skills/shared/squadai-init-playbook",
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
