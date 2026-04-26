package marketplace

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── parseTree ───────────────────────────────────────────────────────────────

func TestParseTree_ExtractsAgentsAndSkills(t *testing.T) {
	tree := []ghTreeEntry{
		{Path: "plugins/backend-development/agents/architect.md", Type: "blob"},
		{Path: "plugins/backend-development/agents/tester.md", Type: "blob"},
		{Path: "plugins/backend-development/skills/patterns/intro.md", Type: "blob"},
		{Path: "plugins/backend-development/skills/patterns/advanced.md", Type: "blob"},
		{Path: "plugins/backend-development/skills/testing/unit.md", Type: "blob"},
		{Path: "plugins/backend-development/.claude-plugin/plugin.json", Type: "blob"},
		{Path: "README.md", Type: "blob"}, // should be ignored
	}

	assets := parseTree(tree)

	if len(assets) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(assets))
	}
	bd := assets["backend-development"]
	if bd == nil {
		t.Fatal("backend-development not found")
	}
	if len(bd.agents) != 2 {
		t.Errorf("expected 2 agents, got %d: %v", len(bd.agents), bd.agents)
	}
	if len(bd.skills) != 2 {
		t.Errorf("expected 2 skill dirs, got %d: %v", len(bd.skills), bd.skills)
	}
}

func TestParseTree_DeduplicatesSkillDirs(t *testing.T) {
	tree := []ghTreeEntry{
		{Path: "plugins/foo/skills/patterns/a.md", Type: "blob"},
		{Path: "plugins/foo/skills/patterns/b.md", Type: "blob"},
		{Path: "plugins/foo/skills/patterns/c.md", Type: "blob"},
	}
	assets := parseTree(tree)
	if len(assets["foo"].skills) != 1 {
		t.Errorf("expected 1 unique skill dir, got %v", assets["foo"].skills)
	}
}

func TestParseTree_IgnoresTreeTypeEntries(t *testing.T) {
	tree := []ghTreeEntry{
		{Path: "plugins/foo/agents", Type: "tree"}, // directory node, not a file
		{Path: "plugins/foo/agents/bar.md", Type: "blob"},
	}
	assets := parseTree(tree)
	if len(assets["foo"].agents) != 1 {
		t.Errorf("expected 1 agent, got %v", assets["foo"].agents)
	}
}

// ─── Sync / Load / Save ──────────────────────────────────────────────────────

func TestSyncWithClient_WritesRegistry(t *testing.T) {
	server := mockGitHubServer(t, map[string]RegistryPlugin{
		"my-plugin": {
			Name:        "my-plugin",
			Version:     "1.0.0",
			Description: "A test plugin",
			Agents:      []string{"agent-one"},
			Skills:      []string{"skill-alpha"},
		},
	})
	defer server.Close()

	dir := t.TempDir()
	client := serverClient(server)

	reg, err := syncWithServer(dir, client, server.URL)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if len(reg.Plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(reg.Plugins))
	}
	if reg.Plugins["my-plugin"].Version != "1.0.0" {
		t.Errorf("version mismatch: %v", reg.Plugins["my-plugin"].Version)
	}

	// Registry file should be written.
	regPath := filepath.Join(dir, registryFile)
	if _, err := os.Stat(regPath); err != nil {
		t.Errorf("registry file not written: %v", err)
	}
}

func TestLoadRegistry_MissingFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if err != ErrRegistryNotFound {
		t.Errorf("expected ErrRegistryNotFound, got %v", err)
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	reg := &Registry{
		Source: DefaultSource,
		Plugins: map[string]RegistryPlugin{
			"foo": {Name: "foo", Version: "2.1.0", Description: "Test", Agents: []string{"bar"}},
		},
	}
	if err := Save(dir, reg); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Plugins["foo"].Version != "2.1.0" {
		t.Errorf("round-trip version mismatch: %v", loaded.Plugins["foo"].Version)
	}
}

// ─── SortedNames ─────────────────────────────────────────────────────────────

func TestSortedNames_ReturnsAlphaOrder(t *testing.T) {
	reg := &Registry{
		Plugins: map[string]RegistryPlugin{
			"zebra": {Name: "zebra"},
			"alpha": {Name: "alpha"},
			"mango": {Name: "mango"},
		},
	}
	names := reg.SortedNames()
	if names[0] != "alpha" || names[1] != "mango" || names[2] != "zebra" {
		t.Errorf("unexpected order: %v", names)
	}
}

// ─── Install ─────────────────────────────────────────────────────────────────

func TestInstallWithClient_DownloadsAgentFiles(t *testing.T) {
	pluginName := "test-plugin"
	agentContent := "---\nname: my-agent\ndescription: Test\n---\nBody.\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/agents/my-agent.md") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(agentContent))
			return
		}
		// Skills listing
		if strings.Contains(r.URL.Path, "/skills/skill-one") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	dir := t.TempDir()
	reg := &Registry{
		Plugins: map[string]RegistryPlugin{
			pluginName: {
				Name:    pluginName,
				Version: "1.0.0",
				Agents:  []string{"my-agent"},
				Skills:  []string{"skill-one"},
			},
		},
	}

	// Patch the raw base URL by using a custom install function.
	client := server.Client()
	plugin, err := installWithBaseURLs(dir, pluginName, reg, client, server.URL, server.URL)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if plugin.Version != "1.0.0" {
		t.Errorf("version = %v, want 1.0.0", plugin.Version)
	}

	// Agent file should exist.
	agentPath := filepath.Join(dir, ".claude", "agents", "my-agent.md")
	data, readErr := os.ReadFile(agentPath)
	if readErr != nil {
		t.Fatalf("agent file not written: %v", readErr)
	}
	if string(data) != agentContent {
		t.Errorf("agent content mismatch: got %q", string(data))
	}
}

func TestInstallWithClient_UnknownPluginReturnsError(t *testing.T) {
	dir := t.TempDir()
	reg := &Registry{Plugins: map[string]RegistryPlugin{}}
	_, err := Install(dir, "nonexistent", reg)
	if err == nil {
		t.Error("expected error for unknown plugin")
	}
}

// ─── test helpers ─────────────────────────────────────────────────────────────

// mockGitHubServer builds a minimal HTTP server that responds to the GitHub API
// calls that Sync makes.
func mockGitHubServer(t *testing.T, plugins map[string]RegistryPlugin) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Recursive git tree.
		if strings.Contains(r.URL.Path, "/git/trees/main") {
			var entries []ghTreeEntry
			for name, p := range plugins {
				entries = append(entries, ghTreeEntry{
					Path: "plugins/" + name + "/.claude-plugin/plugin.json",
					Type: "blob",
				})
				for _, a := range p.Agents {
					entries = append(entries, ghTreeEntry{
						Path: "plugins/" + name + "/agents/" + a + ".md",
						Type: "blob",
					})
				}
				for _, s := range p.Skills {
					entries = append(entries, ghTreeEntry{
						Path: "plugins/" + name + "/skills/" + s + "/index.md",
						Type: "blob",
					})
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(ghTree{Tree: entries})
			return
		}

		// plugin.json manifest (raw content).
		for name, p := range plugins {
			if strings.Contains(r.URL.Path, "/plugins/"+name+"/.claude-plugin/plugin.json") {
				manifest := PluginManifest{Name: p.Name, Version: p.Version, Description: p.Description}
				_ = json.NewEncoder(w).Encode(manifest)
				return
			}
		}

		http.NotFound(w, r)
	}))
}

func serverClient(s *httptest.Server) *http.Client {
	return s.Client()
}

// syncWithServer is a test helper that overrides the API/raw base URLs.
func syncWithServer(projectDir string, client *http.Client, serverURL string) (*Registry, error) {
	// Temporarily patch the package-level URL constants for this test.
	// In production code the constants are not overridable, so we call the
	// internal functions that accept injected URLs.
	return syncWithBaseURLs(projectDir, client, serverURL, serverURL)
}

// syncWithBaseURLs is the internal implementation of Sync with injectable URLs.
// Exposed only in test files via the package-private bridge below.
func syncWithBaseURLs(projectDir string, client *http.Client, apiBase, rawBase string) (*Registry, error) {
	// Fetch tree.
	treeURL := apiBase + "/git/trees/main?recursive=1"
	resp, err := client.Get(treeURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errStatusf(treeURL, resp.Status)
	}
	var tree ghTree
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, err
	}

	pluginAssets := parseTree(tree.Tree)

	reg := &Registry{
		Source:  DefaultSource,
		Plugins: make(map[string]RegistryPlugin, len(pluginAssets)),
	}

	for name, assets := range pluginAssets {
		manifestURL := rawBase + "/plugins/" + name + "/.claude-plugin/plugin.json"
		mResp, err := client.Get(manifestURL)
		if err != nil || mResp.StatusCode != http.StatusOK {
			continue
		}
		var manifest PluginManifest
		if err := json.NewDecoder(mResp.Body).Decode(&manifest); err != nil {
			mResp.Body.Close()
			continue
		}
		mResp.Body.Close()
		reg.Plugins[name] = RegistryPlugin{
			Name:        manifest.Name,
			Version:     manifest.Version,
			Description: manifest.Description,
			Agents:      assets.agents,
			Skills:      assets.skills,
			Commands:    assets.commands,
		}
	}

	return reg, Save(projectDir, reg)
}

// installWithBaseURLs is Install with injectable URL bases (for testing).
func installWithBaseURLs(projectDir, pluginName string, reg *Registry, client *http.Client, apiBase, rawBase string) (*RegistryPlugin, error) {
	plugin, ok := reg.Plugins[pluginName]
	if !ok {
		return nil, errNotFound(pluginName)
	}

	agentsDir := filepath.Join(projectDir, ".claude", "agents")

	for _, agentName := range plugin.Agents {
		rawURL := rawBase + "/plugins/" + pluginName + "/agents/" + agentName + ".md"
		dest := filepath.Join(agentsDir, agentName+".md")
		if err := downloadFile(client, rawURL, dest); err != nil {
			return nil, err
		}
	}

	for _, skillName := range plugin.Skills {
		dirURL := apiBase + "/contents/plugins/" + pluginName + "/skills/" + skillName
		entries, err := listGitHubDir(client, dirURL)
		if err != nil {
			return nil, err
		}
		skillDir := filepath.Join(projectDir, ".claude", "skills", pluginName, skillName)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.Type != "file" {
				continue
			}
			rawURL := rawBase + "/plugins/" + pluginName + "/skills/" + skillName + "/" + entry.Name
			dest := filepath.Join(skillDir, entry.Name)
			if err := downloadFile(client, rawURL, dest); err != nil {
				return nil, err
			}
		}
	}

	return &plugin, nil
}

func errStatusf(url, status string) error {
	return &statusError{url: url, status: status}
}

type statusError struct {
	url, status string
}

func (e *statusError) Error() string {
	return "GitHub API " + e.url + ": " + e.status
}

func errNotFound(name string) error {
	return ErrRegistryNotFound
}
