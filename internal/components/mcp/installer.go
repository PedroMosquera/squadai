package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/managed"
)

const (
	// mcpKey is the top-level JSON key for MCP server configurations.
	mcpKey = "mcp"
)

// rootKeyForAgentMethod returns the cached MCP root key for the given agent,
// falling back to the package-level rootKeyForAgent if not cached.
func (i *Installer) rootKeyForAgent(agent domain.AgentID) string {
	if cfg, ok := i.agentConfigs[agent]; ok {
		return cfg.rootKey
	}
	return rootKeyForAgent(agent)
}

// rootKeyForAgent returns the top-level JSON key for MCP server configs.
// VS Code Copilot expects "servers"; all other adapters use "mcpServers".
func rootKeyForAgent(agent domain.AgentID) string {
	if agent == domain.AgentVSCodeCopilot {
		return "servers"
	}
	return "mcpServers"
}

// urlKeyForAgent returns the JSON field name for remote server URLs.
// Windsurf expects "serverUrl"; all other agents use "url".
func urlKeyForAgent(agent domain.AgentID) string {
	if agent == domain.AgentWindsurf {
		return "serverUrl"
	}
	return "url"
}

// agentMCPConfig caches adapter-provided MCP configuration for use during Apply.
type agentMCPConfig struct {
	rootKey string
	urlKey  string
}

// Installer implements domain.ComponentInstaller for MCP server configuration.
// It supports three strategies:
//   - MergeIntoSettings: merges all servers into a single config file's "mcp" key
//     (used by OpenCode — adapter.MCPConfigPath returns "").
//   - MCPConfigFile: writes all servers into a dedicated MCP config file
//     (used by adapters where MCPConfigPath returns a non-empty path).
type Installer struct {
	// servers is the desired MCP server configuration.
	servers map[string]domain.MCPServerDef

	// projectDir is captured during Plan so Apply can write to the centralized sidecar.
	projectDir string

	// agentConfigs caches adapter-declared MCP keys per agent, populated during Plan.
	agentConfigs map[domain.AgentID]agentMCPConfig
}

// New returns an MCP installer configured from the merged MCP config.
// Only enabled servers are included.
func New(mcpConfig map[string]domain.MCPServerDef) *Installer {
	servers := make(map[string]domain.MCPServerDef)
	for name, def := range mcpConfig {
		if def.Enabled {
			servers[name] = def
		}
	}
	return &Installer{servers: servers, agentConfigs: make(map[domain.AgentID]agentMCPConfig)}
}

// ID returns the component identifier.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentMCP
}

// Servers returns the configured MCP servers.
func (i *Installer) Servers() map[string]domain.MCPServerDef {
	return i.servers
}

// Plan determines what MCP actions are needed for the given adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	if !adapter.SupportsComponent(domain.ComponentMCP) {
		return nil, nil
	}

	if len(i.servers) == 0 {
		return nil, nil
	}

	// Capture project dir so Apply can write to the centralized sidecar.
	i.projectDir = projectDir

	// Cache adapter-declared MCP keys for use during Apply.
	i.agentConfigs[adapter.ID()] = agentMCPConfig{
		rootKey: adapter.MCPRootKey(),
		urlKey:  adapter.MCPURLKey(),
	}

	// Strategy 1: MCPConfigFile — adapter declares a separate MCP config path.
	if mcpPath := adapter.MCPConfigPath(projectDir); mcpPath != "" {
		return i.planMCPConfigFile(adapter, mcpPath)
	}

	// Strategy 3: MergeIntoSettings — adapter merges into its project config file.
	return i.planMergedConfig(adapter, projectDir)
}

// planMergedConfig plans actions for the MergeIntoSettings strategy.
// All servers are merged into the adapter's project config file under the "mcp" key.
func (i *Installer) planMergedConfig(adapter domain.Adapter, projectDir string) ([]domain.PlannedAction, error) {
	targetPath := adapter.ProjectConfigFile(projectDir)
	if targetPath == "" {
		return nil, nil
	}

	actionID := fmt.Sprintf("%s-mcp", adapter.ID())

	existing, err := fileutil.ReadJSONFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	if existing == nil {
		return []domain.PlannedAction{
			{
				ID:          actionID,
				Agent:       adapter.ID(),
				Component:   domain.ComponentMCP,
				Action:      domain.ActionCreate,
				TargetPath:  targetPath,
				Description: "create config file with MCP servers",
			},
		}, nil
	}

	// Check if MCP key matches desired state.
	if mcpKeyMatches(existing, i.servers) {
		return []domain.PlannedAction{
			{
				ID:          actionID,
				Agent:       adapter.ID(),
				Component:   domain.ComponentMCP,
				Action:      domain.ActionSkip,
				TargetPath:  targetPath,
				Description: "MCP server configuration already up to date",
			},
		}, nil
	}

	return []domain.PlannedAction{
		{
			ID:          actionID,
			Agent:       adapter.ID(),
			Component:   domain.ComponentMCP,
			Action:      domain.ActionUpdate,
			TargetPath:  targetPath,
			Description: "update MCP server configuration",
		},
	}, nil
}

// Apply executes a single planned action.
func (i *Installer) Apply(action domain.PlannedAction) error {
	if action.Action == domain.ActionSkip {
		return nil
	}

	if len(i.servers) == 0 {
		return nil
	}

	// MCPConfigFile actions have a "mcp:configfile:" prefix.
	if strings.HasPrefix(action.Description, "mcp:configfile:") {
		return i.applyMCPConfigFile(action)
	}

	return i.applyMergedConfig(action)
}

// applyMergedConfig writes all servers into the "mcp" key of a single config file.
func (i *Installer) applyMergedConfig(action domain.PlannedAction) error {
	// Read existing file or start empty.
	existing, err := fileutil.ReadJSONFile(action.TargetPath)
	if err != nil {
		return fmt.Errorf("read target: %w", err)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}

	// Convert servers to a generic map for JSON serialization.
	// MergeIntoSettings is used by OpenCode — uses default "url" key.
	mcpMap := make(map[string]interface{})
	for name, def := range i.servers {
		mcpMap[name] = serverToMap(def, action.Agent)
	}
	existing[mcpKey] = mcpMap

	// Marshal with indentation.
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')

	// Ensure parent directory exists.
	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if _, err := fileutil.WriteAtomic(action.TargetPath, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	// Write managed-key tracking to the centralized sidecar (if we know projectDir).
	if i.projectDir != "" {
		relPath, err := filepath.Rel(i.projectDir, action.TargetPath)
		if err != nil {
			relPath = action.TargetPath
		}
		if err := managed.WriteManagedKeys(i.projectDir, relPath, []string{mcpKey}); err != nil {
			return fmt.Errorf("write managed keys sidecar: %w", err)
		}
	}

	return nil
}

// isMCPConfigFileAdapter checks if the adapter uses the MCPConfigFile strategy.
// Claude Code, VS Code Copilot, Cursor, and Windsurf use a dedicated MCP config file.
func isMCPConfigFileAdapter(adapter domain.Adapter) bool {
	switch adapter.ID() {
	case domain.AgentClaudeCode, domain.AgentVSCodeCopilot, domain.AgentCursor, domain.AgentWindsurf:
		return true
	}
	return false
}

// planMCPConfigFile plans actions for the MCPConfigFile strategy.
// All servers are merged into a dedicated MCP config file under the adapter's root key.
func (i *Installer) planMCPConfigFile(adapter domain.Adapter, targetPath string) ([]domain.PlannedAction, error) {
	if targetPath == "" {
		return nil, nil
	}

	actionID := fmt.Sprintf("%s-mcp", adapter.ID())

	existing, err := fileutil.ReadJSONFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read MCP config file: %w", err)
	}

	if existing == nil {
		return []domain.PlannedAction{
			{
				ID:          actionID,
				Agent:       adapter.ID(),
				Component:   domain.ComponentMCP,
				Action:      domain.ActionCreate,
				TargetPath:  targetPath,
				Description: "mcp:configfile:create MCP config with mcpServers",
			},
		}, nil
	}

	// Check if mcpServers key matches desired state.
	if mcpServersKeyMatches(existing, i.servers, adapter.ID()) {
		return []domain.PlannedAction{
			{
				ID:          actionID,
				Agent:       adapter.ID(),
				Component:   domain.ComponentMCP,
				Action:      domain.ActionSkip,
				TargetPath:  targetPath,
				Description: "mcp:configfile:MCP server configuration already up to date",
			},
		}, nil
	}

	return []domain.PlannedAction{
		{
			ID:          actionID,
			Agent:       adapter.ID(),
			Component:   domain.ComponentMCP,
			Action:      domain.ActionUpdate,
			TargetPath:  targetPath,
			Description: "mcp:configfile:update MCP server configuration",
		},
	}, nil
}

// applyMCPConfigFile writes all servers into the "mcpServers" key of a dedicated MCP config file.
// Existing keys that are not managed (e.g. "inputs" for VS Code) are preserved.
func (i *Installer) applyMCPConfigFile(action domain.PlannedAction) error {
	// Read existing file or start empty.
	existing, err := fileutil.ReadJSONFile(action.TargetPath)
	if err != nil {
		return fmt.Errorf("read MCP config: %w", err)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}

	// Convert servers to a generic map for JSON serialization.
	rootKey := i.rootKeyForAgent(action.Agent)
	serversMap := make(map[string]interface{})
	for name, def := range i.servers {
		serversMap[name] = serverToMap(def, action.Agent)
	}
	existing[rootKey] = serversMap
	// Note: all other existing keys (e.g. "inputs" in VS Code mcp.json) are preserved
	// because we only overwrite the servers root key.

	// Marshal with indentation.
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal MCP config: %w", err)
	}
	data = append(data, '\n')

	// Ensure parent directory exists.
	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create MCP config dir: %w", err)
	}

	if _, err := fileutil.WriteAtomic(action.TargetPath, data, 0644); err != nil {
		return fmt.Errorf("write MCP config: %w", err)
	}

	// Write managed-key tracking to the centralized sidecar (if we know projectDir).
	if i.projectDir != "" {
		relPath, err := filepath.Rel(i.projectDir, action.TargetPath)
		if err != nil {
			relPath = action.TargetPath
		}
		if err := managed.WriteManagedKeys(i.projectDir, relPath, []string{rootKey}); err != nil {
			return fmt.Errorf("write managed keys sidecar: %w", err)
		}
	}

	return nil
}

// verifyMCPConfigFile checks the MCPConfigFile strategy result.
func (i *Installer) verifyMCPConfigFile(adapter domain.Adapter, projectDir string) ([]domain.VerifyResult, error) {
	targetPath := adapter.MCPConfigPath(projectDir)
	if targetPath == "" {
		return nil, nil
	}

	var results []domain.VerifyResult

	existing, err := fileutil.ReadJSONFile(targetPath)
	if err != nil || existing == nil {
		results = append(results, domain.VerifyResult{
			Check:     "mcp-configfile-exists",
			Passed:    false,
			Severity:  domain.SeverityError,
			Component: "mcp",
			Message:   fmt.Sprintf("MCP config file not found: %s", targetPath),
		})
		return results, nil
	}
	results = append(results, domain.VerifyResult{
		Check:     "mcp-configfile-exists",
		Passed:    true,
		Severity:  domain.SeverityInfo,
		Component: "mcp",
	})

	if mcpServersKeyMatches(existing, i.servers, adapter.ID()) {
		results = append(results, domain.VerifyResult{
			Check:     "mcp-configfile-servers-current",
			Passed:    true,
			Severity:  domain.SeverityInfo,
			Component: "mcp",
		})
	} else {
		results = append(results, domain.VerifyResult{
			Check:     "mcp-configfile-servers-current",
			Passed:    false,
			Severity:  domain.SeverityError,
			Component: "mcp",
			Message:   "MCP server configuration does not match expected state",
		})
	}

	return results, nil
}

// mcpServersKeyMatches checks whether the adapter-specific root key in the document
// matches the expected server definitions.
func mcpServersKeyMatches(doc map[string]interface{}, expected map[string]domain.MCPServerDef, agent domain.AgentID) bool {
	rootKey := rootKeyForAgent(agent)
	mcpVal, exists := doc[rootKey]
	if !exists {
		return false
	}

	// Compare via JSON serialization for deep equality.
	expectedMap := make(map[string]interface{})
	for name, def := range expected {
		expectedMap[name] = serverToMap(def, agent)
	}

	expectedJSON, _ := json.Marshal(expectedMap)
	actualJSON, _ := json.Marshal(mcpVal)
	return string(expectedJSON) == string(actualJSON)
}

// Verify checks post-apply state for the MCP component.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsComponent(domain.ComponentMCP) {
		return nil, nil
	}

	if len(i.servers) == 0 {
		return nil, nil
	}

	if isMCPConfigFileAdapter(adapter) {
		return i.verifyMCPConfigFile(adapter, projectDir)
	}

	return i.verifyMergedConfig(adapter, projectDir)
}

// verifyMergedConfig checks the MergeIntoSettings strategy result.
func (i *Installer) verifyMergedConfig(adapter domain.Adapter, projectDir string) ([]domain.VerifyResult, error) {
	targetPath := adapter.ProjectConfigFile(projectDir)
	if targetPath == "" {
		return nil, nil
	}

	var results []domain.VerifyResult

	existing, err := fileutil.ReadJSONFile(targetPath)
	if err != nil || existing == nil {
		results = append(results, domain.VerifyResult{
			Check:   "mcp-file-exists",
			Passed:  false,
			Message: fmt.Sprintf("config file not found: %s", targetPath),
		})
		return results, nil
	}
	results = append(results, domain.VerifyResult{
		Check:  "mcp-file-exists",
		Passed: true,
	})

	if mcpKeyMatches(existing, i.servers) {
		results = append(results, domain.VerifyResult{
			Check:  "mcp-servers-current",
			Passed: true,
		})
	} else {
		results = append(results, domain.VerifyResult{
			Check:   "mcp-servers-current",
			Passed:  false,
			Message: "MCP server configuration does not match expected state",
		})
	}

	return results, nil
}

// RenderContent returns the content that Apply would write for the given action,
// without performing the write. Used by the diff renderer.
func (i *Installer) RenderContent(action domain.PlannedAction) ([]byte, error) {
	if strings.HasPrefix(action.Description, "mcp:configfile:") {
		return i.renderMCPConfigFileContent(action)
	}
	return i.renderMergedConfigContent(action)
}

// renderMergedConfigContent computes what applyMergedConfig would write.
func (i *Installer) renderMergedConfigContent(action domain.PlannedAction) ([]byte, error) {
	existing, err := fileutil.ReadJSONFile(action.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("read target: %w", err)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}
	mcpMap := make(map[string]interface{})
	for name, def := range i.servers {
		mcpMap[name] = serverToMap(def, action.Agent)
	}
	existing[mcpKey] = mcpMap
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

// renderMCPConfigFileContent computes what applyMCPConfigFile would write.
// Existing keys not managed by squadai (e.g. "inputs") are preserved.
func (i *Installer) renderMCPConfigFileContent(action domain.PlannedAction) ([]byte, error) {
	existing, err := fileutil.ReadJSONFile(action.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("read MCP config: %w", err)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}
	serversMap := make(map[string]interface{})
	for name, def := range i.servers {
		serversMap[name] = serverToMap(def, action.Agent)
	}
	existing[rootKeyForAgent(action.Agent)] = serversMap
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal MCP config: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

// serverToMap converts an MCPServerDef to a generic map for JSON output.
// Each agent has a different MCP server schema:
//
// OpenCode (MergeIntoSettings):
//
//	{"type": "local", "command": ["npx", "-y", "..."]}
//	{"type": "remote", "url": "https://..."}
//
// Claude Code, VS Code Copilot (require "type" for remote):
//
//	Stdio: {"command": "npx", "args": ["-y", "..."]}
//	Remote: {"type": "http", "url": "https://..."}
//
// Cursor (no "type" field for any):
//
//	Stdio: {"command": "npx", "args": ["-y", "..."]}
//	Remote: {"url": "https://..."}
//
// Windsurf (no "type" field, uses "serverUrl"):
//
//	Stdio: {"command": "npx", "args": ["-y", "..."]}
//	Remote: {"serverUrl": "https://..."}
func serverToMap(def domain.MCPServerDef, agent domain.AgentID) map[string]interface{} {
	m := make(map[string]interface{})

	if agent == domain.AgentOpenCode {
		// OpenCode uses the array-style command with explicit type field.
		m["type"] = def.Type
		if len(def.Command) > 0 {
			m["command"] = def.Command
		}
	} else {
		// All other agents use split command/args format for stdio servers.
		if len(def.Command) > 0 {
			m["command"] = def.Command[0]
			if len(def.Command) > 1 {
				m["args"] = def.Command[1:]
			}
		}
		// Claude Code and VS Code require "type": "http" for remote servers.
		// Cursor and Windsurf infer the type from the presence of url/serverUrl.
		if def.URL != "" && (agent == domain.AgentClaudeCode || agent == domain.AgentVSCodeCopilot) {
			m["type"] = "http"
		}
	}

	if def.URL != "" {
		m[urlKeyForAgent(agent)] = def.URL
	}
	if len(def.Environment) > 0 {
		if agent == domain.AgentOpenCode {
			m["environment"] = def.Environment
		} else {
			// Claude Code, Cursor, Windsurf, VS Code use "env" for environment variables.
			m["env"] = def.Environment
		}
	}
	if len(def.Headers) > 0 {
		m["headers"] = def.Headers
	}
	return m
}

// mcpKeyMatches checks whether the "mcp" key in the document matches
// the expected server definitions. Used by the MergeIntoSettings strategy (OpenCode).
func mcpKeyMatches(doc map[string]interface{}, expected map[string]domain.MCPServerDef) bool {
	mcpVal, exists := doc[mcpKey]
	if !exists {
		return false
	}

	// Compare via JSON serialization for deep equality.
	// MergeIntoSettings is only used by OpenCode which uses default "url" key.
	expectedMap := make(map[string]interface{})
	for name, def := range expected {
		expectedMap[name] = serverToMap(def, domain.AgentOpenCode)
	}

	expectedJSON, _ := json.Marshal(expectedMap)
	actualJSON, _ := json.Marshal(mcpVal)
	return string(expectedJSON) == string(actualJSON)
}
