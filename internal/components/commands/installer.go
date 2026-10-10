package commands

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

// Installer implements domain.ComponentInstaller for command definitions.
// It writes <ProjectCommandsDir>/<name>.md files: config-defined commands with
// YAML frontmatter, plus SquadAI's own slash commands for Claude Code.
type Installer struct {
	commands map[string]domain.CommandDef
}

// New returns a commands installer configured from the merged command definitions.
func New(commands map[string]domain.CommandDef) *Installer {
	resolved := make(map[string]domain.CommandDef)
	for name, def := range commands {
		resolved[name] = def
	}
	return &Installer{commands: resolved}
}

// ID returns the component identifier.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentCommands
}

// Plan determines what command file actions are needed for the given adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	if !adapter.SupportsComponent(domain.ComponentCommands) {
		return nil, nil
	}

	commandsDir := adapter.ProjectCommandsDir(projectDir)
	if commandsDir == "" {
		return nil, nil
	}

	desired, err := i.desired(adapter)
	if err != nil {
		return nil, err
	}

	var actions []domain.PlannedAction

	for _, name := range sortedNames(desired) {
		content := desired[name]
		targetPath := filepath.Join(commandsDir, name+".md")
		actionID := fmt.Sprintf("%s-command-%s", adapter.ID(), name)

		existing, err := fileutil.ReadFileOrEmpty(targetPath)
		if err != nil {
			return nil, fmt.Errorf("read command %s: %w", name, err)
		}

		if string(existing) == content {
			actions = append(actions, domain.PlannedAction{
				ID:          actionID,
				Agent:       adapter.ID(),
				Component:   domain.ComponentCommands,
				Action:      domain.ActionSkip,
				TargetPath:  targetPath,
				Description: fmt.Sprintf("command %s already up to date", name),
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
			Component:   domain.ComponentCommands,
			Action:      action,
			TargetPath:  targetPath,
			Description: fmt.Sprintf("%s command %s", action, name),
		})
	}

	return actions, nil
}

// PlanRemoval returns delete actions for the command files Plan would write
// for adapter that are still byte-identical to that content. Files the user
// edited or authored in the same directory are left alone.
func (i *Installer) PlanRemoval(adapter domain.Adapter, projectDir string) ([]domain.PlannedAction, error) {
	if !adapter.SupportsComponent(domain.ComponentCommands) {
		return nil, nil
	}
	commandsDir := adapter.ProjectCommandsDir(projectDir)
	if commandsDir == "" {
		return nil, nil
	}

	desired, err := i.desired(adapter)
	if err != nil {
		return nil, err
	}

	var actions []domain.PlannedAction
	for _, name := range sortedNames(desired) {
		targetPath := filepath.Join(commandsDir, name+".md")
		existing, err := fileutil.ReadFileOrEmpty(targetPath)
		if err != nil {
			return nil, fmt.Errorf("read command %s: %w", name, err)
		}
		if string(existing) != desired[name] {
			continue
		}
		actions = append(actions, domain.PlannedAction{
			ID:          fmt.Sprintf("%s-command-%s", adapter.ID(), name),
			Agent:       adapter.ID(),
			Component:   domain.ComponentCommands,
			Action:      domain.ActionDelete,
			TargetPath:  targetPath,
			Description: fmt.Sprintf("delete command %s (commands component disabled)", name),
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
		if err := os.Remove(action.TargetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete command: %w", err)
		}
		return nil
	}

	content, err := i.RenderContent(action)
	if err != nil {
		return err
	}

	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create commands dir: %w", err)
	}

	if _, err := fileutil.WriteAtomic(action.TargetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write command: %w", err)
	}

	return nil
}

// Verify checks post-apply state for the commands component.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsComponent(domain.ComponentCommands) {
		return nil, nil
	}

	commandsDir := adapter.ProjectCommandsDir(projectDir)
	if commandsDir == "" {
		return nil, nil
	}

	desired, err := i.desired(adapter)
	if err != nil {
		return nil, err
	}

	var results []domain.VerifyResult

	for _, name := range sortedNames(desired) {
		targetPath := filepath.Join(commandsDir, name+".md")
		data, err := os.ReadFile(targetPath)
		if err != nil {
			results = append(results, domain.VerifyResult{
				Check:   fmt.Sprintf("command-%s-exists", name),
				Passed:  false,
				Message: fmt.Sprintf("command file not found: %s", targetPath),
			})
			continue
		}

		if string(data) == desired[name] {
			results = append(results, domain.VerifyResult{
				Check:  fmt.Sprintf("command-%s-current", name),
				Passed: true,
			})
		} else {
			results = append(results, domain.VerifyResult{
				Check:   fmt.Sprintf("command-%s-current", name),
				Passed:  false,
				Message: fmt.Sprintf("command %s content does not match expected", name),
			})
		}
	}

	return results, nil
}

// renderCommand generates the markdown content for a command definition
// with YAML frontmatter.
func renderCommand(name string, def domain.CommandDef) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("description: %s\n", def.Description))
	if def.Agent != "" {
		b.WriteString(fmt.Sprintf("agent: %s\n", def.Agent))
	}
	if def.Model != "" {
		b.WriteString(fmt.Sprintf("model: %s\n", def.Model))
	}
	b.WriteString("---\n")
	if def.Template != "" {
		b.WriteString("\n")
		b.WriteString(def.Template)
		if !strings.HasSuffix(def.Template, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// RenderContent returns the content that Apply would write for the given action,
// without performing the write. Used by the diff renderer.
func (i *Installer) RenderContent(action domain.PlannedAction) (string, error) {
	name := strings.TrimSuffix(filepath.Base(action.TargetPath), ".md")
	if def, ok := i.commands[name]; ok {
		return renderCommand(name, def), nil
	}
	if action.Agent == domain.AgentClaudeCode {
		if content, err := builtinCommand(name); err == nil {
			return content, nil
		}
	}
	return "", fmt.Errorf("command %q not found in config", name)
}

// desired returns the content of every command file this installer manages
// for adapter, keyed by command name. A config-defined command shadows a
// built-in of the same name.
func (i *Installer) desired(adapter domain.Adapter) (map[string]string, error) {
	out := make(map[string]string, len(i.commands))
	if adapter.ID() == domain.AgentClaudeCode {
		entries, err := assets.FS.ReadDir("commands")
		if err != nil {
			return nil, fmt.Errorf("list built-in commands: %w", err)
		}
		for _, e := range entries {
			name := strings.TrimSuffix(e.Name(), ".md")
			content, err := builtinCommand(name)
			if err != nil {
				return nil, err
			}
			out[name] = content
		}
	}
	for name, def := range i.commands {
		out[name] = renderCommand(name, def)
	}
	return out, nil
}

// builtinCommand returns a shipped SquadAI slash command. The trailing
// newline keeps the bytes identical to what the removed install-commands
// command wrote, so existing installs are adopted as up to date.
func builtinCommand(name string) (string, error) {
	content, err := assets.Read("commands/" + name + ".md")
	if err != nil {
		return "", fmt.Errorf("read built-in command %s: %w", name, err)
	}
	return content + "\n", nil
}

func sortedNames(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
