package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// agentMCPConfig caches adapter-declared MCP schema for use during Apply,
// Verify, and Preview — all of which receive only an AgentID, not the live
// Adapter. The cache is populated during Plan.
type agentMCPConfig struct {
	rootKey      string
	urlKey       string
	envKey       string
	commandStyle string
	typeFieldFn  func(domain.MCPServerDef) string
	isConfigFile bool
	// tomlPath is the marker-managed TOML config target for adapters whose
	// MCP config is TOML (Codex's ~/.codex/config.toml). Empty for JSON
	// adapters. When set it takes precedence over the JSON strategies.
	tomlPath string
}

// tomlMCPConfigurer is the optional adapter capability for TOML-based MCP
// configs. Adapters that implement it (Codex) get their MCP servers rendered
// as [<rootKey>.<name>] TOML tables inside a hash-marker managed block,
// preserving any user content outside the markers.
type tomlMCPConfigurer interface {
	MCPTOMLConfigPath(homeDir string) string
}

// rootKeyForAgent returns the cached MCP root key for the given agent.
// Returns empty string if Plan / ensureAgentConfig was not called first.
func (i *Installer) rootKeyForAgent(agent domain.AgentID) string {
	return i.agentConfigs[agent].rootKey
}

// ensureAgentConfig populates the schema cache for the adapter on first use.
// Plan, Verify, and Preview all call this so the cache is independent of the
// callsite ordering.
func (i *Installer) ensureAgentConfig(adapter domain.Adapter, homeDir, projectDir string) {
	if _, ok := i.agentConfigs[adapter.ID()]; ok {
		return
	}
	mcpPath := adapter.MCPConfigPath(projectDir)
	var tomlPath string
	if tc, ok := adapter.(tomlMCPConfigurer); ok {
		tomlPath = tc.MCPTOMLConfigPath(homeDir)
	}
	i.agentConfigs[adapter.ID()] = agentMCPConfig{
		rootKey:      adapter.MCPRootKey(),
		urlKey:       adapter.MCPURLKey(),
		envKey:       adapter.MCPEnvKey(),
		commandStyle: adapter.MCPCommandStyle(),
		typeFieldFn:  adapter.MCPTypeField,
		isConfigFile: mcpPath != "",
		tomlPath:     tomlPath,
	}
}

// Installer implements domain.ComponentInstaller for MCP server configuration.
// It supports three strategies:
//   - MergeIntoSettings: merges all servers into a single config file's "mcp" key
//     (used by OpenCode — adapter.MCPConfigPath returns "").
//   - MCPConfigFile: writes all servers into a dedicated MCP config file
//     (used by adapters where MCPConfigPath returns a non-empty path).
//   - TOMLConfigFile: renders servers as TOML tables inside a hash-marker
//     managed block (used by Codex — adapter implements MCPTOMLConfigPath).
type Installer struct {
	// servers is the desired MCP server configuration.
	servers map[string]domain.MCPServerDef

	// projectDir is captured during Plan so Apply can write to the centralized sidecar.
	projectDir string

	// agentConfigs caches adapter-declared MCP keys per agent, populated during Plan.
	agentConfigs map[domain.AgentID]agentMCPConfig

	// policy carries user decisions from the review screen (per-key overwrite
	// consent, OverwriteAll). Zero value = no overrides, match legacy behavior.
	policy domain.ApplyPolicy

	// opts carries construction-time options (e.g. profile-driven pruning).
	opts Options
}

// Options controls optional MCP installer behavior.
type Options struct {
	// PruneWhenEmpty plans and applies actions even when the desired server
	// set is empty, so previously managed servers are removed from adapter
	// configs. Set when an active context profile declares an MCP filter —
	// without it an empty desired set is treated as "MCP not configured" and
	// stale managed servers would survive a profile switch.
	PruneWhenEmpty bool
}

// SetApplyPolicy implements domain.PolicyAware. The pipeline executor calls
// this before Apply so the installer can honor per-key overwrite decisions
// when merging into existing JSON files.
func (i *Installer) SetApplyPolicy(p domain.ApplyPolicy) {
	i.policy = p
}

// New returns an MCP installer configured from the merged MCP config.
// Only enabled servers are included.
func New(mcpConfig map[string]domain.MCPServerDef, opts ...Options) *Installer {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	servers := make(map[string]domain.MCPServerDef)
	for name, def := range mcpConfig {
		if def.Enabled {
			servers[name] = def
		}
	}
	return &Installer{servers: servers, agentConfigs: make(map[domain.AgentID]agentMCPConfig), opts: o}
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

	if len(i.servers) == 0 && !i.opts.PruneWhenEmpty {
		return nil, nil
	}

	// Capture project dir so Apply can write to the centralized sidecar.
	i.projectDir = projectDir

	// Populate adapter-declared MCP schema cache so Apply / Verify / Preview
	// (which only receive an AgentID) can resolve schema without the live adapter.
	i.ensureAgentConfig(adapter, homeDir, projectDir)

	// Strategy 0: TOMLConfigFile — adapter declares a marker-managed TOML config.
	if cfg := i.agentConfigs[adapter.ID()]; cfg.tomlPath != "" {
		return i.planTOMLConfigFile(adapter, cfg.tomlPath)
	}

	// Strategy 1: MCPConfigFile — adapter declares a separate MCP config path.
	if i.agentConfigs[adapter.ID()].isConfigFile {
		return i.planMCPConfigFile(adapter, adapter.MCPConfigPath(projectDir))
	}

	// Strategy 2: MergeIntoSettings — adapter merges into its project config file.
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

	// Nothing to install and nothing on disk to prune.
	if len(i.servers) == 0 && (existing == nil || existing[i.rootKeyForAgent(adapter.ID())] == nil) {
		return nil, nil
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
	if i.mcpKeyMatches(existing, i.servers, adapter.ID()) {
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

	if len(i.servers) == 0 && !i.opts.PruneWhenEmpty {
		return nil
	}

	// TOMLConfigFile actions have a "mcp:toml:" prefix.
	if strings.HasPrefix(action.Description, "mcp:toml:") {
		return i.applyTOMLConfigFile(action)
	}

	// MCPConfigFile actions have a "mcp:configfile:" prefix.
	if strings.HasPrefix(action.Description, "mcp:configfile:") {
		return i.applyMCPConfigFile(action)
	}

	return i.applyMergedConfig(action)
}

// applyMergedConfig writes all servers under the adapter's root key in a
// shared config file, respecting user-wins semantics for any unrelated
// top-level key on disk.
func (i *Installer) applyMergedConfig(action domain.PlannedAction) error {
	rootKey := i.rootKeyForAgent(action.Agent)
	mcpMap := make(map[string]interface{}, len(i.servers))
	for name, def := range i.servers {
		mcpMap[name] = i.serverToMap(def, action.Agent)
	}
	incoming := map[string]any{rootKey: mcpMap}

	return i.mergeAndWrite(action, incoming, []string{rootKey})
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

	// Nothing to install and nothing on disk to prune.
	if len(i.servers) == 0 && (existing == nil || existing[i.rootKeyForAgent(adapter.ID())] == nil) {
		return nil, nil
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
	if i.mcpServersKeyMatches(existing, i.servers, adapter.ID()) {
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

// applyMCPConfigFile writes all servers into the servers-root key of a dedicated
// MCP config file, respecting user-wins semantics so unrelated user-authored
// keys (e.g. "inputs" for VS Code) are preserved.
func (i *Installer) applyMCPConfigFile(action domain.PlannedAction) error {
	rootKey := i.rootKeyForAgent(action.Agent)
	serversMap := make(map[string]interface{}, len(i.servers))
	for name, def := range i.servers {
		serversMap[name] = i.serverToMap(def, action.Agent)
	}
	incoming := map[string]any{rootKey: serversMap}

	return i.mergeAndWrite(action, incoming, []string{rootKey})
}

// tomlSectionID is the hash-marker section ID for the managed TOML MCP block:
// "# squadai:mcp:start" / "# squadai:mcp:end".
const tomlSectionID = "mcp"

// planTOMLConfigFile plans actions for the TOMLConfigFile strategy. All
// servers are rendered as TOML tables inside a hash-marker managed block;
// user content outside the markers is never touched.
func (i *Installer) planTOMLConfigFile(adapter domain.Adapter, targetPath string) ([]domain.PlannedAction, error) {
	if targetPath == "" {
		return nil, nil
	}

	actionID := fmt.Sprintf("%s-mcp", adapter.ID())

	existing, err := fileutil.ReadFileOrEmpty(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read MCP TOML config: %w", err)
	}

	desired := i.renderTOMLBlock(adapter.ID())

	// Nothing to install and no managed block on disk to prune.
	if len(i.servers) == 0 && !marker.HasHashSection(string(existing), tomlSectionID) {
		return nil, nil
	}

	base := domain.PlannedAction{
		ID:         actionID,
		Agent:      adapter.ID(),
		Component:  domain.ComponentMCP,
		TargetPath: targetPath,
	}

	if marker.HasHashSection(string(existing), tomlSectionID) {
		if marker.ExtractHashSection(string(existing), tomlSectionID) == desired {
			base.Action = domain.ActionSkip
			base.Description = "mcp:toml:MCP server configuration already up to date"
			return []domain.PlannedAction{base}, nil
		}
		base.Action = domain.ActionUpdate
		base.Description = "mcp:toml:update MCP server block"
		return []domain.PlannedAction{base}, nil
	}

	base.Action = domain.ActionCreate
	if len(existing) > 0 {
		base.Action = domain.ActionUpdate
	}
	base.Description = "mcp:toml:inject MCP server block"
	return []domain.PlannedAction{base}, nil
}

// applyTOMLConfigFile injects the rendered TOML server block between hash
// markers in the target file, preserving user content outside the markers.
// An empty desired server set (profile pruning) removes the managed block.
func (i *Installer) applyTOMLConfigFile(action domain.PlannedAction) error {
	rendered, err := i.renderTOMLConfigContent(action)
	if err != nil {
		return err
	}

	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if _, err := fileutil.WriteAtomic(action.TargetPath, rendered, 0644); err != nil {
		return fmt.Errorf("write MCP TOML config: %w", err)
	}
	return nil
}

// renderTOMLConfigContent computes what applyTOMLConfigFile would write.
func (i *Installer) renderTOMLConfigContent(action domain.PlannedAction) ([]byte, error) {
	existing, err := fileutil.ReadFileOrEmpty(action.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("read MCP TOML config: %w", err)
	}
	block := i.renderTOMLBlock(action.Agent)
	updated := marker.InjectHashSection(string(existing), tomlSectionID, block)
	return []byte(updated), nil
}

// renderTOMLBlock renders the desired server set as the managed TOML block
// body for the given agent. Returns "" when no servers are configured (which
// removes the managed block on inject).
func (i *Installer) renderTOMLBlock(agent domain.AgentID) string {
	if len(i.servers) == 0 {
		return ""
	}
	return renderTOMLServers(i.rootKeyForAgent(agent), i.servers)
}

// verifyTOMLConfigFile checks the TOMLConfigFile strategy result.
func (i *Installer) verifyTOMLConfigFile(adapter domain.Adapter, targetPath string) ([]domain.VerifyResult, error) {
	if targetPath == "" {
		return nil, nil
	}

	var results []domain.VerifyResult

	existing, err := fileutil.ReadFileOrEmpty(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read MCP TOML config: %w", err)
	}

	// A pruned-to-empty profile with no managed block on disk is a clean state.
	if len(i.servers) == 0 && !marker.HasHashSection(string(existing), tomlSectionID) {
		return nil, nil
	}

	if _, statErr := os.Stat(targetPath); statErr != nil {
		results = append(results, domain.VerifyResult{
			Check:     "mcp-toml-exists",
			Passed:    false,
			Severity:  domain.SeverityError,
			Component: "mcp",
			Message:   fmt.Sprintf("MCP TOML config not found: %s", targetPath),
		})
		return results, nil
	}
	results = append(results, domain.VerifyResult{
		Check:     "mcp-toml-exists",
		Passed:    true,
		Severity:  domain.SeverityInfo,
		Component: "mcp",
	})

	desired := i.renderTOMLBlock(adapter.ID())
	if marker.ExtractHashSection(string(existing), tomlSectionID) == desired &&
		(desired == "" || marker.HasHashSection(string(existing), tomlSectionID)) {
		results = append(results, domain.VerifyResult{
			Check:     "mcp-toml-servers-current",
			Passed:    true,
			Severity:  domain.SeverityInfo,
			Component: "mcp",
		})
	} else {
		results = append(results, domain.VerifyResult{
			Check:     "mcp-toml-servers-current",
			Passed:    false,
			Severity:  domain.SeverityError,
			Component: "mcp",
			Message:   "MCP server block does not match expected state",
		})
	}

	return results, nil
}

// mergeAndWrite is the shared helper both MCP apply branches route through.
// It reads the existing sidecar, pulls any per-key overrides from the active
// ApplyPolicy, invokes MergeAndWriteJSON, surfaces conflicts as
// *domain.ConflictError, and finally updates the sidecar to reflect the new
// managed-keys set (existing ∪ newlyManaged).
func (i *Installer) mergeAndWrite(action domain.PlannedAction, incoming map[string]any, incomingKeys []string) error {
	// Ensure the parent directory exists even for fresh installs —
	// WriteAtomic does this too, but failing early gives a clearer error.
	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	relPath := action.TargetPath
	if i.projectDir != "" {
		if rel, relErr := filepath.Rel(i.projectDir, action.TargetPath); relErr == nil {
			relPath = rel
		}
	}

	var existingManaged []string
	if i.projectDir != "" {
		keys, err := managed.ReadManagedKeys(i.projectDir, relPath)
		if err != nil {
			return fmt.Errorf("read managed keys sidecar: %w", err)
		}
		existingManaged = keys
	}

	overrides := i.policy.EffectiveOverrides(action.TargetPath, incomingKeys)

	res, err := fileutil.MergeAndWriteJSON(action.TargetPath, incoming, existingManaged, overrides, 0644)
	if err != nil {
		return fmt.Errorf("merge and write %s: %w", action.TargetPath, err)
	}
	if len(res.Conflicts) > 0 {
		return &domain.ConflictError{
			TargetPath: action.TargetPath,
			Conflicts:  conflictsToDomain(res.Conflicts),
		}
	}

	if i.projectDir != "" {
		finalManaged := unionKeys(existingManaged, res.NewlyManaged)
		if err := managed.WriteManagedKeys(i.projectDir, relPath, finalManaged); err != nil {
			return fmt.Errorf("write managed keys sidecar: %w", err)
		}
	}

	return nil
}

// unionKeys returns the sorted, deduped union of two string slices. It is
// used to maintain the sidecar's managed-keys set across merges — keys
// SquadAI previously claimed must be retained even if this merge didn't
// touch them.
func unionKeys(a, b []string) []string {
	set := make(map[string]bool, len(a)+len(b))
	for _, k := range a {
		set[k] = true
	}
	for _, k := range b {
		set[k] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	// MergeJSON already returns sorted; union may re-order, so sort here.
	sort.Strings(out)
	return out
}

// conflictsToDomain adapts fileutil-level merge conflicts into the
// domain-level Conflict shape the review pipeline expects.
func conflictsToDomain(in []fileutil.MergeConflict) []domain.Conflict {
	out := make([]domain.Conflict, 0, len(in))
	for _, c := range in {
		out = append(out, domain.Conflict{
			Key:           c.Key,
			UserValue:     stringifyForConflict(c.UserValue),
			IncomingValue: stringifyForConflict(c.IncomingValue),
		})
	}
	return out
}

// verifyMCPConfigFile checks the MCPConfigFile strategy result.
func (i *Installer) verifyMCPConfigFile(adapter domain.Adapter, projectDir string) ([]domain.VerifyResult, error) {
	targetPath := adapter.MCPConfigPath(projectDir)
	if targetPath == "" {
		return nil, nil
	}

	var results []domain.VerifyResult

	existing, err := fileutil.ReadJSONFile(targetPath)
	// A pruned-to-empty profile with nothing on disk is a clean state.
	if len(i.servers) == 0 && (err != nil || existing == nil || existing[i.rootKeyForAgent(adapter.ID())] == nil) {
		return nil, nil
	}
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

	if i.mcpServersKeyMatches(existing, i.servers, adapter.ID()) {
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

// mcpServersKeyMatches checks whether the adapter-specific root key in the
// document matches the expected server definitions. Reads the cached schema
// for the agent — Plan must have been called first.
func (i *Installer) mcpServersKeyMatches(doc map[string]interface{}, expected map[string]domain.MCPServerDef, agent domain.AgentID) bool {
	rootKey := i.rootKeyForAgent(agent)
	mcpVal, exists := doc[rootKey]
	if !exists {
		return false
	}

	// Compare via JSON serialization for deep equality.
	expectedMap := make(map[string]interface{})
	for name, def := range expected {
		expectedMap[name] = i.serverToMap(def, agent)
	}

	expectedJSON, _ := json.Marshal(expectedMap)
	actualJSON, _ := json.Marshal(mcpVal)
	return string(expectedJSON) == string(actualJSON)
}

// Verify checks post-apply state for the MCP component. The strategy is
// looked up from the cached schema, populated lazily so Verify works whether
// or not Plan was called first.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsComponent(domain.ComponentMCP) {
		return nil, nil
	}

	if len(i.servers) == 0 && !i.opts.PruneWhenEmpty {
		return nil, nil
	}

	i.ensureAgentConfig(adapter, homeDir, projectDir)

	if cfg := i.agentConfigs[adapter.ID()]; cfg.tomlPath != "" {
		return i.verifyTOMLConfigFile(adapter, cfg.tomlPath)
	}

	if i.agentConfigs[adapter.ID()].isConfigFile {
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
	// A pruned-to-empty profile with nothing on disk is a clean state.
	if len(i.servers) == 0 && (err != nil || existing == nil || existing[i.rootKeyForAgent(adapter.ID())] == nil) {
		return nil, nil
	}
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

	if i.mcpKeyMatches(existing, i.servers, adapter.ID()) {
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
	if strings.HasPrefix(action.Description, "mcp:toml:") {
		return i.renderTOMLConfigContent(action)
	}
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
		mcpMap[name] = i.serverToMap(def, action.Agent)
	}
	existing[i.rootKeyForAgent(action.Agent)] = mcpMap
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
		serversMap[name] = i.serverToMap(def, action.Agent)
	}
	existing[i.rootKeyForAgent(action.Agent)] = serversMap
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal MCP config: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

// serverToMap converts an MCPServerDef to a generic map for JSON output,
// using the cached adapter schema for the given agent. Plan / Verify must
// have populated the cache via ensureAgentConfig before this is called.
//
// Per-agent schema variations encoded via the cached fields:
//
//	OpenCode:    command=array, env="environment", type=def.Type (always)
//	Claude:      command=split, env="env",         type="http" iff URL set
//	VS Code:     command=split, env="env",         type="http" iff URL set
//	Cursor:      command=split, env="env",         type omitted
//	Windsurf:    command=split, env="env",         type omitted (URL key="serverUrl")
func (i *Installer) serverToMap(def domain.MCPServerDef, agent domain.AgentID) map[string]interface{} {
	cfg := i.agentConfigs[agent]
	m := make(map[string]interface{})

	// "type" field — adapter-specific logic encapsulated in MCPTypeField.
	if cfg.typeFieldFn != nil {
		if t := cfg.typeFieldFn(def); t != "" {
			m["type"] = t
		}
	}

	// Command encoding — single array vs split command/args.
	if len(def.Command) > 0 {
		if cfg.commandStyle == "array" {
			m["command"] = def.Command
		} else {
			m["command"] = def.Command[0]
			if len(def.Command) > 1 {
				m["args"] = def.Command[1:]
			}
		}
	}

	if def.URL != "" && cfg.urlKey != "" {
		m[cfg.urlKey] = def.URL
	}
	if len(def.Environment) > 0 && cfg.envKey != "" {
		m[cfg.envKey] = def.Environment
	}
	if len(def.Headers) > 0 {
		m["headers"] = def.Headers
	}
	return m
}

// mcpKeyMatches checks whether the cached root key in the document matches
// the expected server definitions. Used by the MergeIntoSettings strategy.
func (i *Installer) mcpKeyMatches(doc map[string]interface{}, expected map[string]domain.MCPServerDef, agent domain.AgentID) bool {
	rootKey := i.rootKeyForAgent(agent)
	mcpVal, exists := doc[rootKey]
	if !exists {
		return false
	}

	// Compare via JSON serialization for deep equality.
	expectedMap := make(map[string]interface{})
	for name, def := range expected {
		expectedMap[name] = i.serverToMap(def, agent)
	}

	expectedJSON, _ := json.Marshal(expectedMap)
	actualJSON, _ := json.Marshal(mcpVal)
	return string(expectedJSON) == string(actualJSON)
}
