package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/fileutil"
)

const (
	// mcpKey is the top-level JSON key for MCP server configurations.
	mcpKey = "mcp"

	// managedMetaKey mirrors the settings component's metadata key.
	managedMetaKey = "_agent_manager"
)

// Installer implements domain.ComponentInstaller for MCP server configuration.
// It manages the "mcp" key in adapter config files (opencode.json) with
// MCP server definitions from the merged config.
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

// Apply executes a single planned action.
func (i *Installer) Apply(action domain.PlannedAction) error {
	if action.Action == domain.ActionSkip {
		return nil
	}

	if len(i.servers) == 0 {
		return nil
	}

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

// Verify checks post-apply state for the MCP component.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsComponent(domain.ComponentMCP) {
		return nil, nil
	}

	if len(i.servers) == 0 {
		return nil, nil
	}

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
