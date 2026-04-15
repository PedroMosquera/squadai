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

	existing, err := readJSONFile(targetPath)
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
	if managedKeysMatch(existing, settings) {
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

// Apply executes a single planned action.
func (i *Installer) Apply(action domain.PlannedAction) error {
	if action.Action == domain.ActionSkip {
		return nil
	}

	agentID := string(action.Agent)
	settings := i.adapterSettings[agentID]
	if len(settings) == 0 {
		return nil
	}

	// Read existing file or start with empty map.
	existing, err := readJSONFile(action.TargetPath)
	if err != nil {
		return fmt.Errorf("read target: %w", err)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}

	// Write managed keys into the document.
	managedKeys := make([]string, 0, len(settings))
	for key, val := range settings {
		existing[key] = val
		managedKeys = append(managedKeys, key)
	}
	sort.Strings(managedKeys)

	// Marshal with indentation for readability.
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	data = append(data, '\n')

	// Ensure parent directory exists.
	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	if _, err := fileutil.WriteAtomic(action.TargetPath, data, 0644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	// Write managed-key tracking to the centralized sidecar (if we know projectDir).
	if i.projectDir != "" {
		relPath, err := filepath.Rel(i.projectDir, action.TargetPath)
		if err != nil {
			relPath = action.TargetPath
		}
		if err := managed.WriteManagedKeys(i.projectDir, relPath, managedKeys); err != nil {
			return fmt.Errorf("write managed keys sidecar: %w", err)
		}
	}

	return nil
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

	existing, err := readJSONFile(targetPath)
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
	if managedKeysMatch(existing, settings) {
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

	existing, err := readJSONFile(action.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("read target: %w", err)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}

	managedKeys := make([]string, 0, len(settings))
	for key, val := range settings {
		existing[key] = val
		managedKeys = append(managedKeys, key)
	}
	sort.Strings(managedKeys)

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal settings: %w", err)
	}
	data = append(data, '\n')
	return data, nil
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
