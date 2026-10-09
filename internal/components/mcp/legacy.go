package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/managed"
)

const legacyPrefix = "mcp:legacy:"

// Ownership is per top-level key, so inside an owned legacy root key squadai
// cannot tell its own servers from ones the user added later. Every apply used
// to overwrite the whole key with the desired set, so a name in the desired set
// is treated as squadai's; any other name is left for the user. A server
// removed from the squadai config before this migration therefore stays behind
// in the legacy file.

// legacyRelPath returns the legacy path relative to the project, which is how
// the managed sidecar keys it.
func (i *Installer) legacyRelPath(agent domain.AgentID) string {
	cfg := i.agentConfigs[agent]
	return relToProject(cfg.projectDir, cfg.legacyPath)
}

func relToProject(projectDir, path string) string {
	if rel, err := filepath.Rel(projectDir, path); err == nil {
		return rel
	}
	return path
}

// legacyPending reports whether squadai still owns the root key in the
// adapter's legacy MCP location.
func (i *Installer) legacyPending(agent domain.AgentID) (bool, error) {
	cfg := i.agentConfigs[agent]
	if cfg.legacyPath == "" || cfg.projectDir == "" {
		return false, nil
	}
	keys, err := managed.ReadManagedKeys(cfg.projectDir, i.legacyRelPath(agent))
	if err != nil {
		return false, fmt.Errorf("read managed keys for %s: %w", cfg.legacyPath, err)
	}
	for _, k := range keys {
		if k == cfg.legacyRootKey {
			return true, nil
		}
	}
	return false, nil
}

func (i *Installer) planLegacyMigration(agent domain.AgentID) ([]domain.PlannedAction, error) {
	pending, err := i.legacyPending(agent)
	if err != nil || !pending {
		return nil, err
	}
	cfg := i.agentConfigs[agent]
	return []domain.PlannedAction{{
		ID:          fmt.Sprintf("%s-mcp-legacy", agent),
		Agent:       agent,
		Component:   domain.ComponentMCP,
		Action:      domain.ActionUpdate,
		TargetPath:  cfg.legacyPath,
		Description: legacyPrefix + "move squadai MCP servers to " + relToProject(cfg.projectDir, cfg.configPath),
	}}, nil
}

func (i *Installer) verifyLegacyMigration(agent domain.AgentID) ([]domain.VerifyResult, error) {
	pending, err := i.legacyPending(agent)
	if err != nil || !pending {
		return nil, err
	}
	return []domain.VerifyResult{{
		Check:     "mcp-legacy-migrated",
		Passed:    false,
		Severity:  domain.SeverityError,
		Component: "mcp",
		Message: fmt.Sprintf("squadai still owns MCP servers in %s; run 'squadai apply' to move them to %s",
			i.legacyRelPath(agent), relToProject(i.agentConfigs[agent].projectDir, i.agentConfigs[agent].configPath)),
	}}, nil
}

// legacyRemainder returns the legacy document with squadai's servers removed,
// dropping the root key when nothing is left under it. Returns nil when the
// legacy file does not exist.
func (i *Installer) legacyRemainder(agent domain.AgentID) (map[string]any, error) {
	cfg := i.agentConfigs[agent]
	doc, err := fileutil.ReadJSONFile(cfg.legacyPath)
	if err != nil {
		return nil, fmt.Errorf("read legacy MCP config: %w", err)
	}
	if doc == nil {
		return nil, nil
	}
	servers, ok := doc[cfg.legacyRootKey].(map[string]any)
	if !ok {
		return doc, nil
	}
	kept := make(map[string]any, len(servers))
	for name, v := range servers {
		if _, ours := i.servers[name]; !ours {
			kept[name] = v
		}
	}
	if len(kept) == 0 {
		delete(doc, cfg.legacyRootKey)
	} else {
		doc[cfg.legacyRootKey] = kept
	}
	return doc, nil
}

// applyLegacyMigration removes squadai's servers from the legacy location and
// gives up ownership of its root key. It refuses to run until the new MCP file
// holds the desired servers, so a blocked or failed write never leaves the
// agent with neither copy.
func (i *Installer) applyLegacyMigration(action domain.PlannedAction) error {
	cfg := i.agentConfigs[action.Agent]
	current, err := fileutil.ReadJSONFile(cfg.configPath)
	if err != nil {
		return fmt.Errorf("read MCP config: %w", err)
	}
	if current == nil || !i.mcpServersKeyMatches(current, i.servers, action.Agent) {
		return fmt.Errorf("%s does not hold the squadai MCP servers yet; left %s unchanged",
			cfg.configPath, action.TargetPath)
	}

	doc, err := i.legacyRemainder(action.Agent)
	if err != nil {
		return err
	}
	rel := i.legacyRelPath(action.Agent)

	if doc != nil && len(doc) == 0 {
		if err := os.Remove(action.TargetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove legacy MCP config: %w", err)
		}
		if err := managed.UntrackCreatedFile(cfg.projectDir, rel); err != nil {
			return fmt.Errorf("untrack legacy MCP config: %w", err)
		}
		return managed.RemoveManagedFile(cfg.projectDir, rel)
	}
	if doc != nil {
		data, err := marshalJSONDoc(doc)
		if err != nil {
			return err
		}
		if _, err := fileutil.WriteAtomic(action.TargetPath, data, 0644); err != nil {
			return fmt.Errorf("write legacy MCP config: %w", err)
		}
	}

	keys, err := managed.ReadManagedKeys(cfg.projectDir, rel)
	if err != nil {
		return fmt.Errorf("read managed keys: %w", err)
	}
	remaining := keys[:0]
	for _, k := range keys {
		if k != cfg.legacyRootKey {
			remaining = append(remaining, k)
		}
	}
	if len(remaining) == 0 {
		return managed.RemoveManagedFile(cfg.projectDir, rel)
	}
	return managed.WriteManagedKeys(cfg.projectDir, rel, remaining)
}

// marshalJSONDoc uses the same layout as fileutil.MergeAndWriteJSON so a
// rewrite does not churn formatting.
func marshalJSONDoc(doc map[string]any) ([]byte, error) {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal JSON: %w", err)
	}
	return append(data, '\n'), nil
}
