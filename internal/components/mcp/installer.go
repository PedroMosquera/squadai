package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/fileutil"
)

const (
	// mcpKey is the top-level JSON key for MCP server configurations.
	mcpKey = "mcp"

	// managedMetaKey mirrors the settings component's metadata key.
	managedMetaKey = "_agent_manager"
)

// mcpDirProvider is an optional interface for adapters that store MCP configs
// as separate files (one file per server). If an adapter implements this
// interface, the installer uses the SeparateMCPFiles strategy; otherwise it
// falls back to MergeIntoSettings.
type mcpDirProvider interface {
	MCPDir(homeDir string) string
}

// mcpServersKey is the top-level JSON key used by the MCPConfigFile strategy.
// VS Code Copilot, Cursor, and Windsurf use "mcpServers" instead of "mcp".
const mcpServersKey = "mcpServers"

// Installer implements domain.ComponentInstaller for MCP server configuration.
// It supports three strategies:
//   - MergeIntoSettings: merges all servers into a single config file's "mcp" key
//     (used by OpenCode).
//   - SeparateMCPFiles: writes one JSON file per server in an mcp/ directory
//     (used by Claude Code via the mcpDirProvider interface).
//   - MCPConfigFile: writes all servers into a dedicated MCP config file's "mcpServers" key
//     (used by VS Code Copilot, Cursor, Windsurf).
type Installer struct {
	// servers is the desired MCP server configuration.
	servers map[string]domain.MCPServerDef
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
	return &Installer{servers: servers}
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

	// Strategy 1: SeparateMCPFiles (Claude Code).
	if provider, ok := adapter.(mcpDirProvider); ok {
		return i.planSeparateFiles(adapter, provider.MCPDir(homeDir))
	}

	// Strategy 2: MCPConfigFile (VS Code, Cursor, Windsurf) — uses "mcpServers" key.
	if isMCPConfigFileAdapter(adapter) {
		return i.planMCPConfigFile(adapter, projectDir)
	}

	// Strategy 3: MergeIntoSettings (OpenCode) — uses "mcp" key.
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

	existing, err := readJSONFile(targetPath)
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

// planSeparateFiles plans actions for the SeparateMCPFiles strategy.
// Each server gets its own JSON file at mcpDir/{name}.json.
func (i *Installer) planSeparateFiles(adapter domain.Adapter, mcpDir string) ([]domain.PlannedAction, error) {
	var actions []domain.PlannedAction

	for name, def := range i.servers {
		targetPath := filepath.Join(mcpDir, name+".json")
		actionID := fmt.Sprintf("%s-mcp-%s", adapter.ID(), name)

		// Generate expected content.
		expected := serverToJSON(def)

		existing, err := os.ReadFile(targetPath)
		if err != nil {
			if os.IsNotExist(err) {
				actions = append(actions, domain.PlannedAction{
					ID:          actionID,
					Agent:       adapter.ID(),
					Component:   domain.ComponentMCP,
					Action:      domain.ActionCreate,
					TargetPath:  targetPath,
					Description: fmt.Sprintf("create MCP config for %s", name),
				})
				continue
			}
			return nil, fmt.Errorf("read MCP file %s: %w", name, err)
		}

		if string(existing) == string(expected) {
			actions = append(actions, domain.PlannedAction{
				ID:          actionID,
				Agent:       adapter.ID(),
				Component:   domain.ComponentMCP,
				Action:      domain.ActionSkip,
				TargetPath:  targetPath,
				Description: fmt.Sprintf("MCP config for %s already up to date", name),
			})
		} else {
			actions = append(actions, domain.PlannedAction{
				ID:          actionID,
				Agent:       adapter.ID(),
				Component:   domain.ComponentMCP,
				Action:      domain.ActionUpdate,
				TargetPath:  targetPath,
				Description: fmt.Sprintf("update MCP config for %s", name),
			})
		}
	}

	return actions, nil
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

	// Check if this is a separate MCP file (path ends with /{name}.json in an mcp/ dir).
	if i.isSeparateFileAction(action) {
		return i.applySeparateFile(action)
	}

	return i.applyMergedConfig(action)
}

// isSeparateFileAction checks if the action targets a separate MCP file
// by checking if the parent directory is named "mcp".
func (i *Installer) isSeparateFileAction(action domain.PlannedAction) bool {
	dir := filepath.Base(filepath.Dir(action.TargetPath))
	return dir == "mcp"
}

// applyMergedConfig writes all servers into the "mcp" key of a single config file.
func (i *Installer) applyMergedConfig(action domain.PlannedAction) error {
	// Read existing file or start empty.
	existing, err := readJSONFile(action.TargetPath)
	if err != nil {
		return fmt.Errorf("read target: %w", err)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}

	// Convert servers to a generic map for JSON serialization.
	mcpMap := make(map[string]interface{})
	for name, def := range i.servers {
		mcpMap[name] = serverToMap(def)
	}
	existing[mcpKey] = mcpMap

	// Update _agent_manager metadata to include "mcp" in managed_keys.
	updateManagedKeys(existing, mcpKey)

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

	return nil
}

// applySeparateFile writes a single MCP server definition to its own JSON file.
func (i *Installer) applySeparateFile(action domain.PlannedAction) error {
	// Extract server name from filename: {name}.json
	baseName := filepath.Base(action.TargetPath)
	name := strings.TrimSuffix(baseName, ".json")

	def, ok := i.servers[name]
	if !ok {
		return fmt.Errorf("MCP server %q not found in config", name)
	}

	data := serverToJSON(def)

	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create MCP dir: %w", err)
	}

	if _, err := fileutil.WriteAtomic(action.TargetPath, data, 0644); err != nil {
		return fmt.Errorf("write MCP config: %w", err)
	}

	return nil
}

// isMCPConfigFileAdapter checks if the adapter uses the MCPConfigFile strategy.
// VS Code Copilot, Cursor, and Windsurf use a dedicated MCP config file with "mcpServers" key.
func isMCPConfigFileAdapter(adapter domain.Adapter) bool {
	switch adapter.ID() {
	case domain.AgentVSCodeCopilot, domain.AgentCursor, domain.AgentWindsurf:
		return true
	}
	return false
}

// mcpConfigFilePath returns the path to the MCP config file for MCPConfigFile adapters.
// VS Code uses a separate mcp.json; Cursor and Windsurf use their ProjectConfigFile.
func mcpConfigFilePath(adapter domain.Adapter, projectDir string) string {
	switch adapter.ID() {
	case domain.AgentVSCodeCopilot:
		return filepath.Join(projectDir, ".vscode", "mcp.json")
	default:
		return adapter.ProjectConfigFile(projectDir)
	}
}

// planMCPConfigFile plans actions for the MCPConfigFile strategy.
// All servers are merged into a dedicated MCP config file under the "mcpServers" key.
func (i *Installer) planMCPConfigFile(adapter domain.Adapter, projectDir string) ([]domain.PlannedAction, error) {
	targetPath := mcpConfigFilePath(adapter, projectDir)
	if targetPath == "" {
		return nil, nil
	}

	actionID := fmt.Sprintf("%s-mcp", adapter.ID())

	existing, err := readJSONFile(targetPath)
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
				Description: fmt.Sprintf("mcp:configfile:create MCP config with mcpServers"),
			},
		}, nil
	}

	// Check if mcpServers key matches desired state.
	if mcpServersKeyMatches(existing, i.servers) {
		return []domain.PlannedAction{
			{
				ID:          actionID,
				Agent:       adapter.ID(),
				Component:   domain.ComponentMCP,
				Action:      domain.ActionSkip,
				TargetPath:  targetPath,
				Description: fmt.Sprintf("mcp:configfile:MCP server configuration already up to date"),
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
			Description: fmt.Sprintf("mcp:configfile:update MCP server configuration"),
		},
	}, nil
}

// applyMCPConfigFile writes all servers into the "mcpServers" key of a dedicated MCP config file.
func (i *Installer) applyMCPConfigFile(action domain.PlannedAction) error {
	// Read existing file or start empty.
	existing, err := readJSONFile(action.TargetPath)
	if err != nil {
		return fmt.Errorf("read MCP config: %w", err)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}

	// Convert servers to a generic map for JSON serialization.
	serversMap := make(map[string]interface{})
	for name, def := range i.servers {
		serversMap[name] = serverToMap(def)
	}
	existing[mcpServersKey] = serversMap

	// Update _agent_manager metadata to include "mcpServers" in managed_keys.
	updateManagedKeys(existing, mcpServersKey)

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

	return nil
}

// verifyMCPConfigFile checks the MCPConfigFile strategy result.
func (i *Installer) verifyMCPConfigFile(adapter domain.Adapter, projectDir string) ([]domain.VerifyResult, error) {
	targetPath := mcpConfigFilePath(adapter, projectDir)
	if targetPath == "" {
		return nil, nil
	}

	var results []domain.VerifyResult

	existing, err := readJSONFile(targetPath)
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

	if mcpServersKeyMatches(existing, i.servers) {
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

// mcpServersKeyMatches checks whether the "mcpServers" key in the document matches
// the expected server definitions.
func mcpServersKeyMatches(doc map[string]interface{}, expected map[string]domain.MCPServerDef) bool {
	mcpVal, exists := doc[mcpServersKey]
	if !exists {
		return false
	}

	// Compare via JSON serialization for deep equality.
	expectedMap := make(map[string]interface{})
	for name, def := range expected {
		expectedMap[name] = serverToMap(def)
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

	if provider, ok := adapter.(mcpDirProvider); ok {
		return i.verifySeparateFiles(adapter, provider.MCPDir(homeDir))
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

	existing, err := readJSONFile(targetPath)
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

// verifySeparateFiles checks the SeparateMCPFiles strategy result.
func (i *Installer) verifySeparateFiles(_ domain.Adapter, mcpDir string) ([]domain.VerifyResult, error) {
	var results []domain.VerifyResult

	for name, def := range i.servers {
		targetPath := filepath.Join(mcpDir, name+".json")
		expected := serverToJSON(def)

		data, err := os.ReadFile(targetPath)
		if err != nil {
			results = append(results, domain.VerifyResult{
				Check:     fmt.Sprintf("mcp-%s-file-exists", name),
				Passed:    false,
				Severity:  domain.SeverityError,
				Component: "mcp",
				Message:   fmt.Sprintf("MCP config not found: %s", targetPath),
			})
			continue
		}

		results = append(results, domain.VerifyResult{
			Check:     fmt.Sprintf("mcp-%s-file-exists", name),
			Passed:    true,
			Severity:  domain.SeverityInfo,
			Component: "mcp",
		})

		if string(data) == string(expected) {
			results = append(results, domain.VerifyResult{
				Check:     fmt.Sprintf("mcp-%s-current", name),
				Passed:    true,
				Severity:  domain.SeverityInfo,
				Component: "mcp",
			})
		} else {
			results = append(results, domain.VerifyResult{
				Check:     fmt.Sprintf("mcp-%s-current", name),
				Passed:    false,
				Severity:  domain.SeverityError,
				Component: "mcp",
				Message:   fmt.Sprintf("MCP config for %s does not match expected", name),
			})
		}
	}

	return results, nil
}

// RenderContent returns the content that Apply would write for the given action,
// without performing the write. Used by the diff renderer.
func (i *Installer) RenderContent(action domain.PlannedAction) ([]byte, error) {
	if strings.HasPrefix(action.Description, "mcp:configfile:") {
		return i.renderMCPConfigFileContent(action)
	}
	if i.isSeparateFileAction(action) {
		return i.renderSeparateFileContent(action)
	}
	return i.renderMergedConfigContent(action)
}

// renderMergedConfigContent computes what applyMergedConfig would write.
func (i *Installer) renderMergedConfigContent(action domain.PlannedAction) ([]byte, error) {
	existing, err := readJSONFile(action.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("read target: %w", err)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}
	mcpMap := make(map[string]interface{})
	for name, def := range i.servers {
		mcpMap[name] = serverToMap(def)
	}
	existing[mcpKey] = mcpMap
	updateManagedKeys(existing, mcpKey)
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

// renderSeparateFileContent computes what applySeparateFile would write.
func (i *Installer) renderSeparateFileContent(action domain.PlannedAction) ([]byte, error) {
	baseName := filepath.Base(action.TargetPath)
	name := strings.TrimSuffix(baseName, ".json")
	def, ok := i.servers[name]
	if !ok {
		return nil, fmt.Errorf("MCP server %q not found in config", name)
	}
	return serverToJSON(def), nil
}

// renderMCPConfigFileContent computes what applyMCPConfigFile would write.
func (i *Installer) renderMCPConfigFileContent(action domain.PlannedAction) ([]byte, error) {
	existing, err := readJSONFile(action.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("read MCP config: %w", err)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}
	serversMap := make(map[string]interface{})
	for name, def := range i.servers {
		serversMap[name] = serverToMap(def)
	}
	existing[mcpServersKey] = serversMap
	updateManagedKeys(existing, mcpServersKey)
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal MCP config: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

// serverToMap converts an MCPServerDef to a generic map for JSON output.
// Only non-zero fields are included.
func serverToMap(def domain.MCPServerDef) map[string]interface{} {
	m := map[string]interface{}{
		"type": def.Type,
	}
	if len(def.Command) > 0 {
		m["command"] = def.Command
	}
	if def.URL != "" {
		m["url"] = def.URL
	}
	if len(def.Environment) > 0 {
		m["environment"] = def.Environment
	}
	if len(def.Headers) > 0 {
		m["headers"] = def.Headers
	}
	return m
}

// serverToJSON converts a server definition to indented JSON bytes.
func serverToJSON(def domain.MCPServerDef) []byte {
	m := serverToMap(def)
	data, _ := json.MarshalIndent(m, "", "  ")
	data = append(data, '\n')
	return data
}

// mcpKeyMatches checks whether the "mcp" key in the document matches
// the expected server definitions.
func mcpKeyMatches(doc map[string]interface{}, expected map[string]domain.MCPServerDef) bool {
	mcpVal, exists := doc[mcpKey]
	if !exists {
		return false
	}

	// Compare via JSON serialization for deep equality.
	expectedMap := make(map[string]interface{})
	for name, def := range expected {
		expectedMap[name] = serverToMap(def)
	}

	expectedJSON, _ := json.Marshal(expectedMap)
	actualJSON, _ := json.Marshal(mcpVal)
	return string(expectedJSON) == string(actualJSON)
}

// updateManagedKeys adds the given key to _agent_manager.managed_keys if not already present.
func updateManagedKeys(doc map[string]interface{}, key string) {
	var managedKeys []string

	if meta, ok := doc[managedMetaKey].(map[string]interface{}); ok {
		if keys, ok := meta["managed_keys"].([]interface{}); ok {
			for _, k := range keys {
				if s, ok := k.(string); ok {
					managedKeys = append(managedKeys, s)
				}
			}
		}
	}

	// Add key if not present.
	found := false
	for _, k := range managedKeys {
		if k == key {
			found = true
			break
		}
	}
	if !found {
		managedKeys = append(managedKeys, key)
	}
	sort.Strings(managedKeys)

	doc[managedMetaKey] = map[string]interface{}{
		"managed_keys": managedKeys,
	}
}

// readJSONFile reads a JSON file into a generic map.
// Returns nil, nil if the file does not exist.
func readJSONFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return nil, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	return result, nil
}
