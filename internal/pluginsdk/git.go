// Package pluginsdk provides git-based plugin installation for SquadAI.
// Plugins are cloned into .squadai/plugins/<id>/ and described by a
// plugin.json manifest.
package pluginsdk

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Manifest describes a plugin's contributions.
type Manifest struct {
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	Description     string   `json:"description"`
	Methodologies   []string `json:"methodologies,omitempty"` // methodology names contributed
	Skills          []string `json:"skills,omitempty"`        // skill file paths relative to plugin root
	Commands        []string `json:"commands,omitempty"`      // command file paths
	Agents          []string `json:"agents,omitempty"`        // agent file paths
	MCP             []string `json:"mcp,omitempty"`           // MCP server names
	MemoryTemplates []string `json:"memory_templates,omitempty"`
}

// InstallResult reports what was installed.
type InstallResult struct {
	PluginID string
	Path     string
	Manifest *Manifest
}

// pluginsDir is the relative location of installed git-based plugins.
const pluginsDir = ".squadai/plugins"

// ParseGitURL extracts a plugin ID from a git: URL.
// "git:github.com/user/my-plugin" -> "my-plugin"
// "git:https://github.com/user/my-plugin.git" -> "my-plugin"
func ParseGitURL(url string) (id string, repoURL string, err error) {
	if url == "" {
		return "", "", fmt.Errorf("empty git url")
	}
	if !strings.HasPrefix(url, "git:") {
		return "", "", fmt.Errorf("url must start with 'git:' prefix, got %q", url)
	}
	repoURL = strings.TrimPrefix(url, "git:")
	repoURL = strings.TrimSuffix(repoURL, ".git")
	repoURL = strings.TrimRight(repoURL, "/")

	segments := strings.Split(repoURL, "/")
	id = segments[len(segments)-1]
	if id == "" {
		return "", "", fmt.Errorf("could not extract plugin id from %q", url)
	}
	return id, repoURL, nil
}

// Install clones a git-based plugin into .squadai/plugins/<id>/
// and reads its plugin.json manifest.
func Install(projectDir, gitURL string) (*InstallResult, error) {
	id, repoURL, err := ParseGitURL(gitURL)
	if err != nil {
		return nil, fmt.Errorf("parse git url: %w", err)
	}

	destDir := filepath.Join(projectDir, pluginsDir, id)
	if _, err := os.Stat(destDir); err == nil {
		return nil, fmt.Errorf("plugin %q already installed", id)
	}
	if err := os.MkdirAll(filepath.Dir(destDir), 0755); err != nil {
		return nil, fmt.Errorf("create plugins dir: %w", err)
	}

	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, destDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git clone %q: %w\n%s", repoURL, err, out)
	}

	manifest, _ := LoadManifest(destDir) // nil manifest is non-fatal
	return &InstallResult{
		PluginID: id,
		Path:     destDir,
		Manifest: manifest,
	}, nil
}

// Remove deletes a plugin's directory.
func Remove(projectDir, pluginID string) error {
	return os.RemoveAll(filepath.Join(projectDir, pluginsDir, pluginID))
}

// List returns all installed plugin IDs.
func List(projectDir string) ([]string, error) {
	dir := filepath.Join(projectDir, pluginsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read plugins dir: %w", err)
	}

	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			ids = append(ids, entry.Name())
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// LoadManifest reads a plugin's plugin.json.
func LoadManifest(pluginDir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(pluginDir, "plugin.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("plugin.json not found in %s", pluginDir)
		}
		return nil, fmt.Errorf("read plugin.json: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse plugin.json: %w", err)
	}
	return &m, nil
}
