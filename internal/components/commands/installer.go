package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/fileutil"
)

// Installer implements domain.ComponentInstaller for command definitions.
// It writes .opencode/commands/<name>.md files with YAML frontmatter.
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

	var actions []domain.PlannedAction

	names := sortedKeys(i.commands)
	for _, name := range names {
		def := i.commands[name]
		targetPath := filepath.Join(commandsDir, name+".md")
		content := renderCommand(name, def)
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

	name := strings.TrimSuffix(filepath.Base(action.TargetPath), ".md")
	def, ok := i.commands[name]
	if !ok {
		return fmt.Errorf("command %q not found in config", name)
	}

	content := renderCommand(name, def)

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

	if len(i.commands) == 0 {
		return nil, nil
	}

	var results []domain.VerifyResult

	for _, name := range sortedKeys(i.commands) {
		def := i.commands[name]
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

		expected := renderCommand(name, def)
		if string(data) == expected {
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

func sortedKeys(m map[string]domain.CommandDef) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
