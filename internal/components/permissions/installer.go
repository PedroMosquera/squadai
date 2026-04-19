package permissions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/managed"
)

// metaKey is injected as a sibling key to mark that SquadAI manages this
// permissions block. It is used for drift detection and clean removal.
const metaKey = "_squadai_permissions"

// metaValue is the value written to the meta key — acts as a version marker.
const metaValue = "managed"

// denyPaths lists file patterns that should be denied for reading.
var denyPaths = []string{
	"./.env*",
	"**/secrets/**",
	"**/.aws/credentials",
	"**/.ssh/id_*",
}

// confirmCommands lists Bash patterns that require user confirmation before
// execution.
var confirmCommands = []string{
	"git push --force*",
	"git push -f*",
	"git rebase*",
	"rm -rf /*",
	"rm -rf ~*",
	"sudo rm*",
	"dd if=*",
}

// Installer implements domain.ComponentInstaller for the permissions component.
// It merges a security overlay into each supported adapter's project settings
// file (deep-merge, preserving user keys).
type Installer struct {
	projectDir string
}

// New returns a new permissions Installer.
func New() *Installer {
	return &Installer{}
}

// ID returns the component identifier.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentPermissions
}

// Plan computes what actions are needed for the permissions component on the
// given adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	i.projectDir = projectDir

	if !adapter.SupportsComponent(domain.ComponentPermissions) {
		return nil, nil
	}

	targetPath := settingsPath(adapter, projectDir)
	if targetPath == "" {
		return nil, nil
	}

	agentID := string(adapter.ID())
	actionID := agentID + "-permissions"

	existing, err := fileutil.ReadJSONFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("permissions plan read %s: %w", targetPath, err)
	}

	if existing != nil && hasPermissionsMarker(existing) {
		return []domain.PlannedAction{
			{
				ID:          actionID,
				Agent:       adapter.ID(),
				Component:   domain.ComponentPermissions,
				Action:      domain.ActionSkip,
				TargetPath:  targetPath,
				Description: "permissions overlay already present",
			},
		}, nil
	}

	action := domain.ActionUpdate
	if existing == nil {
		action = domain.ActionCreate
	}

	return []domain.PlannedAction{
		{
			ID:          actionID,
			Agent:       adapter.ID(),
			Component:   domain.ComponentPermissions,
			Action:      action,
			TargetPath:  targetPath,
			Description: "merge security permissions overlay",
		},
	}, nil
}

// Apply executes a single planned action for the permissions component.
func (i *Installer) Apply(action domain.PlannedAction) error {
	if action.Action == domain.ActionSkip {
		return nil
	}

	content, err := i.buildContent(action)
	if err != nil {
		return err
	}

	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create permissions dir: %w", err)
	}

	if _, err := fileutil.WriteAtomic(action.TargetPath, content, 0644); err != nil {
		return fmt.Errorf("write permissions: %w", err)
	}

	// Track managed keys in the centralized sidecar for clean removal.
	if i.projectDir != "" {
		relPath, err := filepath.Rel(i.projectDir, action.TargetPath)
		if err != nil {
			relPath = action.TargetPath
		}
		managedKeys := managedKeyNames(action.Agent)
		if err := managed.WriteManagedKeys(i.projectDir, relPath, managedKeys); err != nil {
			return fmt.Errorf("write permissions managed keys sidecar: %w", err)
		}
	}

	return nil
}

// Verify checks that the permissions overlay is present in the settings file.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsComponent(domain.ComponentPermissions) {
		return nil, nil
	}

	targetPath := settingsPath(adapter, projectDir)
	if targetPath == "" {
		return nil, nil
	}

	existing, err := fileutil.ReadJSONFile(targetPath)
	if err != nil || existing == nil {
		return []domain.VerifyResult{
			{
				Check:     "permissions-overlay-present",
				Passed:    false,
				Severity:  domain.SeverityWarning,
				Component: string(domain.ComponentPermissions),
				Message:   fmt.Sprintf("permissions settings file not found: %s", targetPath),
			},
		}, nil
	}

	if !hasPermissionsMarker(existing) {
		return []domain.VerifyResult{
			{
				Check:     "permissions-overlay-present",
				Passed:    false,
				Severity:  domain.SeverityWarning,
				Component: string(domain.ComponentPermissions),
				Message:   fmt.Sprintf("security permissions overlay missing from %s", targetPath),
			},
		}, nil
	}

	return []domain.VerifyResult{
		{
			Check:    "permissions-overlay-present",
			Passed:   true,
			Severity: domain.SeverityInfo,
		},
	}, nil
}

// Strip removes the permissions overlay keys from the settings file at path.
// It is idempotent — if the overlay is not present, it returns nil.
func Strip(settingsFilePath string, agentID domain.AgentID) error {
	existing, err := fileutil.ReadJSONFile(settingsFilePath)
	if err != nil || existing == nil {
		return nil
	}

	switch agentID {
	case domain.AgentClaudeCode:
		delete(existing, "permissions")
	case domain.AgentOpenCode:
		delete(existing, "permission")
	case domain.AgentVSCodeCopilot:
		delete(existing, "chat.tools.autoApprove")
		delete(existing, "files.readonlyInclude")
	default:
		return nil
	}

	delete(existing, metaKey)

	if len(existing) == 0 {
		// Nothing left — remove the file entirely to avoid empty JSON cruft.
		if err := os.Remove(settingsFilePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove empty permissions file: %w", err)
		}
		return nil
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal stripped permissions: %w", err)
	}
	data = append(data, '\n')

	if _, err := fileutil.WriteAtomic(settingsFilePath, data, 0644); err != nil {
		return fmt.Errorf("write stripped permissions: %w", err)
	}

	return nil
}

// RenderContent returns the JSON content the installer would write without
// performing the write. Used by the diff renderer.
func (i *Installer) RenderContent(action domain.PlannedAction) ([]byte, error) {
	return i.buildContent(action)
}

// buildContent computes the merged JSON content for the given action.
func (i *Installer) buildContent(action domain.PlannedAction) ([]byte, error) {
	existing, err := fileutil.ReadJSONFile(action.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", action.TargetPath, err)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}

	overlay := overlayForAgent(action.Agent)
	if overlay == nil {
		return nil, nil
	}

	// Deep-merge: our keys are set, user keys outside our namespace are preserved.
	for k, v := range overlay {
		switch k {
		case "permissions":
			// Claude: merge deny/ask arrays — append our entries if user already
			// has a permissions block so user customizations are preserved.
			existing[k] = mergeClaudePermissions(existing[k], v)
		case "permission":
			// OpenCode: merge nested bash/read maps.
			existing[k] = mergeOpenCodePermission(existing[k], v)
		default:
			existing[k] = v
		}
	}

	// Inject marker meta key.
	existing[metaKey] = metaValue

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal permissions: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

// overlayForAgent returns the per-agent permissions overlay map, or nil if the
// agent is not supported.
func overlayForAgent(id domain.AgentID) map[string]interface{} {
	switch id {
	case domain.AgentClaudeCode:
		deny := make([]interface{}, len(denyPaths))
		for i, p := range denyPaths {
			deny[i] = "Read(" + p + ")"
		}
		ask := make([]interface{}, len(confirmCommands))
		for i, c := range confirmCommands {
			ask[i] = "Bash(" + c + ")"
		}
		return map[string]interface{}{
			"permissions": map[string]interface{}{
				"deny": deny,
				"ask":  ask,
			},
		}

	case domain.AgentOpenCode:
		bash := make(map[string]interface{}, len(confirmCommands))
		for _, c := range confirmCommands {
			bash[c] = "ask"
		}
		read := make(map[string]interface{}, len(denyPaths))
		for _, p := range denyPaths {
			read[p] = "deny"
		}
		return map[string]interface{}{
			"permission": map[string]interface{}{
				"bash": bash,
				"read": read,
			},
		}

	case domain.AgentVSCodeCopilot:
		readonlyInclude := map[string]interface{}{}
		for _, p := range denyPaths {
			readonlyInclude[p] = true
		}
		return map[string]interface{}{
			"chat.tools.autoApprove": false,
			"files.readonlyInclude":  readonlyInclude,
		}

	default:
		// Cursor and Windsurf: no supported permissions schema in v1.
		return nil
	}
}

// managedKeyNames returns the top-level JSON keys managed by this installer
// for a given agent.
func managedKeyNames(id domain.AgentID) []string {
	switch id {
	case domain.AgentClaudeCode:
		return []string{"permissions", metaKey}
	case domain.AgentOpenCode:
		return []string{"permission", metaKey}
	case domain.AgentVSCodeCopilot:
		return []string{"chat.tools.autoApprove", "files.readonlyInclude", metaKey}
	default:
		return nil
	}
}

// settingsPath returns the project settings file path for a given adapter.
// It re-uses the adapter's ProjectConfigFile path which is already per-agent
// in the adapter layer.
func settingsPath(adapter domain.Adapter, projectDir string) string {
	return adapter.ProjectConfigFile(projectDir)
}

// hasPermissionsMarker returns true if the document contains the squadai meta
// key, indicating our overlay is already present.
func hasPermissionsMarker(doc map[string]interface{}) bool {
	v, ok := doc[metaKey]
	if !ok {
		return false
	}
	s, _ := v.(string)
	return s == metaValue
}

// mergeClaudePermissions merges the existing Claude permissions block with
// our overlay, appending our entries if not already present.
func mergeClaudePermissions(existing interface{}, overlay interface{}) interface{} {
	overlayMap, ok := overlay.(map[string]interface{})
	if !ok {
		return overlay
	}
	existingMap, ok := existing.(map[string]interface{})
	if !ok {
		return overlay
	}

	result := make(map[string]interface{}, len(existingMap))
	for k, v := range existingMap {
		result[k] = v
	}

	for key, overlayVal := range overlayMap {
		overlaySlice, isSlice := toStringSlice(overlayVal)
		if !isSlice {
			result[key] = overlayVal
			continue
		}
		existingSlice, _ := toStringSlice(result[key])
		merged := appendMissing(existingSlice, overlaySlice)
		out := make([]interface{}, len(merged))
		for i, s := range merged {
			out[i] = s
		}
		result[key] = out
	}
	return result
}

// mergeOpenCodePermission merges the existing OpenCode permission block with
// our overlay, preserving user entries and appending missing ones.
func mergeOpenCodePermission(existing interface{}, overlay interface{}) interface{} {
	overlayMap, ok := overlay.(map[string]interface{})
	if !ok {
		return overlay
	}
	existingMap, ok := existing.(map[string]interface{})
	if !ok {
		return overlay
	}

	result := make(map[string]interface{}, len(existingMap))
	for k, v := range existingMap {
		result[k] = v
	}

	for section, overlaySection := range overlayMap {
		overlaySec, ok := overlaySection.(map[string]interface{})
		if !ok {
			result[section] = overlaySection
			continue
		}
		existingSec, _ := result[section].(map[string]interface{})
		merged := make(map[string]interface{}, len(existingSec)+len(overlaySec))
		for k, v := range existingSec {
			merged[k] = v
		}
		for k, v := range overlaySec {
			if _, exists := merged[k]; !exists {
				merged[k] = v
			}
		}
		result[section] = merged
	}
	return result
}

// toStringSlice converts an interface{} to []string, handling []interface{}.
func toStringSlice(v interface{}) ([]string, bool) {
	switch val := v.(type) {
	case []string:
		return val, true
	case []interface{}:
		out := make([]string, 0, len(val))
		for _, item := range val {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	}
	return nil, false
}

// appendMissing appends entries from src to dst that are not already present.
func appendMissing(dst, src []string) []string {
	set := make(map[string]bool, len(dst))
	for _, s := range dst {
		set[s] = true
	}
	result := make([]string, len(dst), len(dst)+len(src))
	copy(result, dst)
	for _, s := range src {
		if !set[s] {
			result = append(result, s)
			set[s] = true
		}
	}
	return result
}
