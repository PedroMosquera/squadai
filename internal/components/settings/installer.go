package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/managed"
)

// Installer implements domain.ComponentInstaller for the settings component.
// It writes adapter-specific JSON settings files (opencode.json, .claude/settings.json)
// with managed key tracking to preserve user-authored keys.
type Installer struct {
	// adapterSettings maps adapter ID → settings key/value pairs to write.
	adapterSettings map[string]map[string]interface{}

	// projectDir is captured during Plan so Apply can write to the sidecar.
	projectDir string

	// policy carries user decisions from the review screen. Zero value = no
	// overrides, which is the legacy behavior.
	policy domain.ApplyPolicy
}

// SetApplyPolicy implements domain.PolicyAware.
func (i *Installer) SetApplyPolicy(p domain.ApplyPolicy) {
	i.policy = p
}

// New returns a settings installer configured from the merged adapter configs.
// Only adapters with non-empty Settings maps produce output.
func New(adapters map[string]domain.AdapterConfig) *Installer {
	settings := make(map[string]map[string]interface{})
	for id, cfg := range adapters {
		if len(cfg.Settings) > 0 {
			settings[id] = cfg.Settings
		}
	}
	return &Installer{adapterSettings: settings}
}

// ID returns the component identifier.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentSettings
}

// SettingsForAdapter returns the settings map for a given adapter ID.
// Empty map means no settings configured.
func (i *Installer) SettingsForAdapter(adapterID string) map[string]interface{} {
	return i.adapterSettings[adapterID]
}

// Plan determines what settings actions are needed for the given adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	if !adapter.SupportsComponent(domain.ComponentSettings) {
		return nil, nil
	}

	// Capture project dir so Apply can write to the centralized sidecar.
	i.projectDir = projectDir

	agentID := string(adapter.ID())
	settings, ok := i.adapterSettings[agentID]
	if !ok || len(settings) == 0 {
		return nil, nil
	}

	targetPath := adapter.ProjectConfigFile(projectDir)
	if targetPath == "" {
		return nil, nil
	}

	actionID := fmt.Sprintf("%s-settings", agentID)

	existing, err := fileutil.ReadJSONFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read settings file: %w", err)
	}

	if existing == nil {
		return []domain.PlannedAction{
			{
				ID:          actionID,
				Agent:       adapter.ID(),
				Component:   domain.ComponentSettings,
				Action:      domain.ActionCreate,
				TargetPath:  targetPath,
				Description: "create settings file with managed keys",
			},
		}, nil
	}

	// Check if managed keys are all up to date.
	// For OpenCode, also check that $schema is present.
	allExpected := settings
	if agentID == string(domain.AgentOpenCode) {
		allExpected = make(map[string]interface{}, len(settings)+1)
		for k, v := range settings {
			allExpected[k] = v
		}
		allExpected["$schema"] = "https://opencode.ai/config.json"
	}
	if managedKeysMatch(existing, allExpected) {
		return []domain.PlannedAction{
			{
				ID:          actionID,
				Agent:       adapter.ID(),
				Component:   domain.ComponentSettings,
				Action:      domain.ActionSkip,
				TargetPath:  targetPath,
				Description: "settings file already up to date",
			},
		}, nil
	}

	return []domain.PlannedAction{
		{
			ID:          actionID,
			Agent:       adapter.ID(),
			Component:   domain.ComponentSettings,
			Action:      domain.ActionUpdate,
			TargetPath:  targetPath,
			Description: "update managed settings keys",
		},
	}, nil
}

// Apply executes a single planned action. It merges the adapter's managed
// settings into the target file using user-wins semantics — any top-level
// key SquadAI has never claimed is preserved on disk unless the active
// ApplyPolicy grants overwrite consent for that key.
func (i *Installer) Apply(action domain.PlannedAction) error {
	if action.Action == domain.ActionSkip {
		return nil
	}

	agentID := string(action.Agent)
	settings := i.adapterSettings[agentID]
	if len(settings) == 0 {
		return nil
	}

	incoming := make(map[string]any, len(settings)+1)
	incomingKeys := make([]string, 0, len(settings)+1)
	for key, val := range settings {
		incoming[key] = val
		incomingKeys = append(incomingKeys, key)
	}
	if agentID == string(domain.AgentOpenCode) {
		incoming["$schema"] = "https://opencode.ai/config.json"
		incomingKeys = append(incomingKeys, "$schema")
	}

	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
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
		return fmt.Errorf("merge and write settings: %w", err)
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

// unionKeys returns the sorted, deduped union of two string slices. Used to
// maintain the sidecar's managed-keys set across merges — keys SquadAI
// previously claimed must be retained even if this merge didn't touch them.
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
	sort.Strings(out)
	return out
}

// conflictsToDomain adapts fileutil merge conflicts into domain.Conflict.
func conflictsToDomain(in []fileutil.MergeConflict) []domain.Conflict {
	out := make([]domain.Conflict, 0, len(in))
	for _, c := range in {
		out = append(out, domain.Conflict{
			Key:           c.Key,
			UserValue:     stringifyValue(c.UserValue),
			IncomingValue: stringifyValue(c.IncomingValue),
		})
	}
	return out
}

// stringifyValue renders an arbitrary JSON value for TUI-safe display.
func stringifyValue(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	const maxLen = 80
	s := string(data)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

// Verify checks post-apply state for the settings component.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsComponent(domain.ComponentSettings) {
		return nil, nil
	}

	agentID := string(adapter.ID())
	settings, ok := i.adapterSettings[agentID]
	if !ok || len(settings) == 0 {
		return nil, nil
	}

	targetPath := adapter.ProjectConfigFile(projectDir)
	if targetPath == "" {
		return nil, nil
	}

	var results []domain.VerifyResult

	existing, err := fileutil.ReadJSONFile(targetPath)
	if err != nil || existing == nil {
		results = append(results, domain.VerifyResult{
			Check:   "settings-file-exists",
			Passed:  false,
			Message: fmt.Sprintf("settings file not found: %s", targetPath),
		})
		return results, nil
	}
	results = append(results, domain.VerifyResult{
		Check:  "settings-file-exists",
		Passed: true,
	})

	// Check managed keys match expected values.
	// For OpenCode, also verify $schema is present.
	allExpected := settings
	if agentID == string(domain.AgentOpenCode) {
		allExpected = make(map[string]interface{}, len(settings)+1)
		for k, v := range settings {
			allExpected[k] = v
		}
		allExpected["$schema"] = "https://opencode.ai/config.json"
	}
	if managedKeysMatch(existing, allExpected) {
		results = append(results, domain.VerifyResult{
			Check:  "settings-keys-current",
			Passed: true,
		})
	} else {
		results = append(results, domain.VerifyResult{
			Check:   "settings-keys-current",
			Passed:  false,
			Message: "managed settings keys do not match expected values",
		})
	}

	return results, nil
}

// RenderContent returns the content that Apply would write for the given action,
// without performing the write. Used by the diff renderer.
func (i *Installer) RenderContent(action domain.PlannedAction) ([]byte, error) {
	agentID := string(action.Agent)
	settings := i.adapterSettings[agentID]
	if len(settings) == 0 {
		return nil, nil
	}

	existing, err := fileutil.ReadJSONFile(action.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("read target: %w", err)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}

	managedKeys := make([]string, 0, len(settings)+1)
	for key, val := range settings {
		existing[key] = val
		managedKeys = append(managedKeys, key)
	}

	// Inject $schema for OpenCode configs.
	if agentID == string(domain.AgentOpenCode) {
		existing["$schema"] = "https://opencode.ai/config.json"
		managedKeys = append(managedKeys, "$schema")
	}

	sort.Strings(managedKeys)

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal settings: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

// managedKeysMatch checks whether all managed settings keys in the document
// match the expected values.
func managedKeysMatch(doc map[string]interface{}, expected map[string]interface{}) bool {
	for key, expectedVal := range expected {
		actualVal, exists := doc[key]
		if !exists {
			return false
		}
		// Compare via JSON serialization for deep equality.
		expectedJSON, _ := json.Marshal(expectedVal)
		actualJSON, _ := json.Marshal(actualVal)
		if string(expectedJSON) != string(actualJSON) {
			return false
		}
	}
	return true
}
