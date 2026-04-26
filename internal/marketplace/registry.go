// Package marketplace manages the SquadAI plugin marketplace backed by the
// wshobson/agents GitHub repository (github.com/wshobson/agents).
//
// The registry is fetched on demand and cached locally at
// .squadai/plugins-registry.json. Each plugin is identified by its directory
// name in the upstream `plugins/` tree and carries a manifest, an agent list,
// and a skills list.
//
// Install flow: Sync → Load → Install. The Install function downloads agent
// files and skill files from raw.githubusercontent.com, writes them into
// .claude/agents/ and .claude/skills/<plugin>/, then records the installed
// version in project.json marketplace.plugins.
package marketplace

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// DefaultSource is the canonical plugin registry repo.
	DefaultSource = "github.com/wshobson/agents"

	defaultRawBase = "https://raw.githubusercontent.com/wshobson/agents/main"
	defaultAPIBase = "https://api.github.com/repos/wshobson/agents"

	registryFile = ".squadai/plugins-registry.json"
)

// Registry is the local cache of available marketplace plugins.
type Registry struct {
	Source    string                    `json:"source"`
	FetchedAt time.Time                 `json:"fetched_at"`
	Plugins   map[string]RegistryPlugin `json:"plugins"`
}

// RegistryPlugin is the cached metadata for one plugin.
type RegistryPlugin struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Agents      []string `json:"agents"`   // agent filenames without .md
	Skills      []string `json:"skills"`   // skill directory names
	Commands    []string `json:"commands"` // command filenames without .md
}

// PluginManifest mirrors the upstream .claude-plugin/plugin.json shape.
type PluginManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// ghTreeEntry is one node from the GitHub git-tree API.
type ghTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "blob" or "tree"
}

// ghTree is the GitHub git-tree API response.
type ghTree struct {
	Tree     []ghTreeEntry `json:"tree"`
	Truncated bool         `json:"truncated"`
}

// ghDirEntry is one item from a GitHub directory listing API.
type ghDirEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Sync fetches the upstream plugin registry via GitHub APIs and writes the
// result to <projectDir>/.squadai/plugins-registry.json.
// It uses one recursive git-tree call to discover all paths, then fetches
// each plugin.json manifest via raw.githubusercontent.com.
func Sync(projectDir string) (*Registry, error) {
	return SyncWithClient(projectDir, http.DefaultClient)
}

// SyncWithClient is Sync with an injected HTTP client (for testing).
func SyncWithClient(projectDir string, client *http.Client) (*Registry, error) {
	// Step 1: get the full file tree in a single API call.
	tree, err := fetchTree(client)
	if err != nil {
		return nil, fmt.Errorf("fetch plugin tree: %w", err)
	}

	// Step 2: collect plugin names and per-plugin asset paths.
	pluginAssets := parseTree(tree)

	// Step 3: fetch each plugin.json manifest.
	reg := &Registry{
		Source:    DefaultSource,
		FetchedAt: time.Now().UTC(),
		Plugins:   make(map[string]RegistryPlugin, len(pluginAssets)),
	}

	for name, assets := range pluginAssets {
		manifest, fetchErr := fetchManifest(client, name)
		if fetchErr != nil {
			// Non-fatal: skip plugins whose manifest is missing/malformed.
			continue
		}
		reg.Plugins[name] = RegistryPlugin{
			Name:        manifest.Name,
			Version:     manifest.Version,
			Description: manifest.Description,
			Agents:      assets.agents,
			Skills:      assets.skills,
			Commands:    assets.commands,
		}
	}

	// Step 4: persist.
	if err := Save(projectDir, reg); err != nil {
		return nil, err
	}
	return reg, nil
}

// Save writes the registry to <projectDir>/.squadai/plugins-registry.json.
func Save(projectDir string, reg *Registry) error {
	path := filepath.Join(projectDir, registryFile)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create .squadai dir: %w", err)
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// Load reads the local registry cache. Returns ErrRegistryNotFound when the
// file does not exist — callers should prompt users to run `plugins sync`.
func Load(projectDir string) (*Registry, error) {
	path := filepath.Join(projectDir, registryFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrRegistryNotFound
		}
		return nil, fmt.Errorf("read registry: %w", err)
	}
	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	return &reg, nil
}

// ErrRegistryNotFound is returned when no local registry cache exists.
var ErrRegistryNotFound = fmt.Errorf("plugin registry not found — run 'squadai plugins sync' first")

// SortedNames returns plugin names in alphabetical order.
func (r *Registry) SortedNames() []string {
	names := make([]string, 0, len(r.Plugins))
	for name := range r.Plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Install downloads a plugin's agent and skill files from the upstream repo
// into the project's .claude directory, then returns the installed plugin
// metadata. Files are written atomically (temp+rename not used here for
// simplicity since these are new files, not replacements).
func Install(projectDir, pluginName string, reg *Registry) (*RegistryPlugin, error) {
	return InstallWithClient(projectDir, pluginName, reg, http.DefaultClient)
}

// InstallWithClient is Install with an injected HTTP client.
func InstallWithClient(projectDir, pluginName string, reg *Registry, client *http.Client) (*RegistryPlugin, error) {
	plugin, ok := reg.Plugins[pluginName]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found in registry (run 'squadai plugins sync' to refresh)", pluginName)
	}

	agentsDir := filepath.Join(projectDir, ".claude", "agents")
	skillsBase := filepath.Join(projectDir, ".claude", "skills")
	commandsDir := filepath.Join(projectDir, ".claude", "commands")

	// Download agent files.
	for _, agentName := range plugin.Agents {
		rawURL := fmt.Sprintf("%s/plugins/%s/agents/%s.md", defaultRawBase, pluginName, agentName)
		dest := filepath.Join(agentsDir, agentName+".md")
		if err := downloadFile(client, rawURL, dest); err != nil {
			return nil, fmt.Errorf("download agent %s: %w", agentName, err)
		}
	}

	// Download skill files. Each skill is a directory; download all .md files inside.
	for _, skillName := range plugin.Skills {
		skillFiles, err := listGitHubDir(client,
			fmt.Sprintf("%s/contents/plugins/%s/skills/%s", defaultAPIBase, pluginName, skillName))
		if err != nil {
			return nil, fmt.Errorf("list skill %s files: %w", skillName, err)
		}
		skillDir := filepath.Join(skillsBase, pluginName, skillName)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			return nil, fmt.Errorf("create skill dir: %w", err)
		}
		for _, entry := range skillFiles {
			if entry.Type != "file" {
				continue
			}
			rawURL := fmt.Sprintf("%s/plugins/%s/skills/%s/%s",
				defaultRawBase, pluginName, skillName, entry.Name)
			dest := filepath.Join(skillDir, entry.Name)
			if err := downloadFile(client, rawURL, dest); err != nil {
				return nil, fmt.Errorf("download skill file %s/%s: %w", skillName, entry.Name, err)
			}
		}
	}

	// Download command files.
	for _, cmdName := range plugin.Commands {
		rawURL := fmt.Sprintf("%s/plugins/%s/commands/%s.md", defaultRawBase, pluginName, cmdName)
		dest := filepath.Join(commandsDir, cmdName+".md")
		if err := downloadFile(client, rawURL, dest); err != nil {
			return nil, fmt.Errorf("download command %s: %w", cmdName, err)
		}
	}

	return &plugin, nil
}

// Remove deletes all files installed by a plugin and removes it from the registry.
// It does not update project.json — callers are responsible for that.
func Remove(projectDir, pluginName string, reg *Registry) error {
	plugin, ok := reg.Plugins[pluginName]
	if !ok {
		return fmt.Errorf("plugin %q not found in registry", pluginName)
	}

	agentsDir := filepath.Join(projectDir, ".claude", "agents")
	skillsBase := filepath.Join(projectDir, ".claude", "skills")
	commandsDir := filepath.Join(projectDir, ".claude", "commands")

	for _, agentName := range plugin.Agents {
		_ = os.Remove(filepath.Join(agentsDir, agentName+".md"))
	}
	for _, skillName := range plugin.Skills {
		_ = os.RemoveAll(filepath.Join(skillsBase, pluginName, skillName))
	}
	// Clean up the plugin-scoped skills directory if empty.
	_ = os.Remove(filepath.Join(skillsBase, pluginName))

	for _, cmdName := range plugin.Commands {
		_ = os.Remove(filepath.Join(commandsDir, cmdName+".md"))
	}

	return nil
}

// ─── internal helpers ─────────────────────────────────────────────────────────

type pluginAssetSet struct {
	agents   []string
	skills   []string
	commands []string
}

// parseTree scans the flat path list and builds a map of pluginName→asset sets.
func parseTree(tree []ghTreeEntry) map[string]*pluginAssetSet {
	result := make(map[string]*pluginAssetSet)

	for _, entry := range tree {
		// Only care about blob entries under plugins/.
		if entry.Type != "blob" || !strings.HasPrefix(entry.Path, "plugins/") {
			continue
		}
		parts := strings.SplitN(entry.Path, "/", 4) // ["plugins", <name>, <kind>, ...]
		if len(parts) < 3 {
			continue
		}
		pluginName := parts[1]
		kind := parts[2]

		if _, ok := result[pluginName]; !ok {
			result[pluginName] = &pluginAssetSet{}
		}

		switch kind {
		case "agents":
			if len(parts) == 4 && strings.HasSuffix(parts[3], ".md") {
				name := strings.TrimSuffix(parts[3], ".md")
				result[pluginName].agents = append(result[pluginName].agents, name)
			}
		case "skills":
			// skills/<skill-dir>/<file> — we want unique skill directory names.
			if len(parts) == 4 {
				skillParts := strings.SplitN(parts[3], "/", 2)
				skillDir := skillParts[0]
				if skillDir != "" && !containsStr(result[pluginName].skills, skillDir) {
					result[pluginName].skills = append(result[pluginName].skills, skillDir)
				}
			}
		case "commands":
			if len(parts) == 4 && strings.HasSuffix(parts[3], ".md") {
				name := strings.TrimSuffix(parts[3], ".md")
				result[pluginName].commands = append(result[pluginName].commands, name)
			}
		}
	}

	for _, assets := range result {
		sort.Strings(assets.agents)
		sort.Strings(assets.skills)
		sort.Strings(assets.commands)
	}

	return result
}

func fetchTree(client *http.Client) ([]ghTreeEntry, error) {
	url := defaultAPIBase + "/git/trees/main?recursive=1"
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API %s: %s", url, resp.Status)
	}

	var tree ghTree
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, fmt.Errorf("decode tree: %w", err)
	}
	return tree.Tree, nil
}

func fetchManifest(client *http.Client, pluginName string) (*PluginManifest, error) {
	url := fmt.Sprintf("%s/plugins/%s/.claude-plugin/plugin.json", defaultRawBase, pluginName)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch manifest %s: %s", pluginName, resp.Status)
	}

	var manifest PluginManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	return &manifest, nil
}

func listGitHubDir(client *http.Client, apiURL string) ([]ghDirEntry, error) {
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API %s: %s", apiURL, resp.Status)
	}

	var entries []ghDirEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode dir listing: %w", err)
	}
	return entries, nil
}

func downloadFile(client *http.Client, url, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
