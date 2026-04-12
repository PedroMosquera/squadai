package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/fileutil"
)

// Installer implements domain.ComponentInstaller for agent definitions.
// It writes .opencode/agents/<name>.md files with YAML frontmatter.
type Installer struct {
	agents     map[string]domain.AgentDef
	projectDir string
}

// New returns an agents installer configured from the merged agent definitions.
func New(agents map[string]domain.AgentDef, projectDir string) *Installer {
	resolved := make(map[string]domain.AgentDef)
	for name, def := range agents {
		// Resolve prompt content from file if needed.
		if def.Prompt == "" && def.PromptFile != "" && projectDir != "" {
			filePath := filepath.Join(projectDir, ".agent-manager", def.PromptFile)
			data, err := os.ReadFile(filePath)
			if err == nil {
				def.Prompt = string(data)
			}
		}
		resolved[name] = def
	}
	return &Installer{agents: resolved, projectDir: projectDir}
}

// ID returns the component identifier.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentAgents
}

// Plan determines what agent file actions are needed for the given adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	if !adapter.SupportsComponent(domain.ComponentAgents) {
		return nil, nil
	}

	agentsDir := adapter.ProjectAgentsDir(projectDir)
	if agentsDir == "" {
		return nil, nil
	}

	var actions []domain.PlannedAction

	// Plan create/update for each defined agent.
	names := sortedKeys(i.agents)
	for _, name := range names {
		def := i.agents[name]
		targetPath := filepath.Join(agentsDir, name+".md")
		content := renderAgent(name, def)
		actionID := fmt.Sprintf("%s-agent-%s", adapter.ID(), name)

		existing, err := fileutil.ReadFileOrEmpty(targetPath)
		if err != nil {
			return nil, fmt.Errorf("read agent %s: %w", name, err)
		}

		if string(existing) == content {
			actions = append(actions, domain.PlannedAction{
				ID:          actionID,
				Agent:       adapter.ID(),
				Component:   domain.ComponentAgents,
				Action:      domain.ActionSkip,
				TargetPath:  targetPath,
				Description: fmt.Sprintf("agent %s already up to date", name),
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
			Component:   domain.ComponentAgents,
			Action:      action,
			TargetPath:  targetPath,
			Description: fmt.Sprintf("%s agent %s", action, name),
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
			return fmt.Errorf("delete agent: %w", err)
		}
		return nil
	}

	// Extract agent name from target path.
	name := strings.TrimSuffix(filepath.Base(action.TargetPath), ".md")
	def, ok := i.agents[name]
	if !ok {
		return fmt.Errorf("agent %q not found in config", name)
	}

	content := renderAgent(name, def)

	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create agents dir: %w", err)
	}

	if _, err := fileutil.WriteAtomic(action.TargetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write agent: %w", err)
	}

	return nil
}

// Verify checks post-apply state for the agents component.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsComponent(domain.ComponentAgents) {
		return nil, nil
	}

	agentsDir := adapter.ProjectAgentsDir(projectDir)
	if agentsDir == "" {
		return nil, nil
	}

	if len(i.agents) == 0 {
		return nil, nil
	}

	var results []domain.VerifyResult

	for _, name := range sortedKeys(i.agents) {
		def := i.agents[name]
		targetPath := filepath.Join(agentsDir, name+".md")
		data, err := os.ReadFile(targetPath)
		if err != nil {
			results = append(results, domain.VerifyResult{
				Check:   fmt.Sprintf("agent-%s-exists", name),
				Passed:  false,
				Message: fmt.Sprintf("agent file not found: %s", targetPath),
			})
			continue
		}

		expected := renderAgent(name, def)
		if string(data) == expected {
			results = append(results, domain.VerifyResult{
				Check:  fmt.Sprintf("agent-%s-current", name),
				Passed: true,
			})
		} else {
			results = append(results, domain.VerifyResult{
				Check:   fmt.Sprintf("agent-%s-current", name),
				Passed:  false,
				Message: fmt.Sprintf("agent %s content does not match expected", name),
			})
		}
	}

	return results, nil
}

// renderAgent generates the markdown content for an agent definition
// with YAML frontmatter.
func renderAgent(name string, def domain.AgentDef) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("description: %s\n", def.Description))
	if def.Mode != "" {
		b.WriteString(fmt.Sprintf("mode: %s\n", def.Mode))
	}
	if def.Model != "" {
		b.WriteString(fmt.Sprintf("model: %s\n", def.Model))
	}
	if len(def.Permission) > 0 {
		b.WriteString("permission:\n")
		permKeys := make([]string, 0, len(def.Permission))
		for k := range def.Permission {
			permKeys = append(permKeys, k)
		}
		sort.Strings(permKeys)
		for _, k := range permKeys {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, def.Permission[k]))
		}
	}
	b.WriteString("---\n")
	if def.Prompt != "" {
		b.WriteString("\n")
		b.WriteString(def.Prompt)
		if !strings.HasSuffix(def.Prompt, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func sortedKeys(m map[string]domain.AgentDef) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
