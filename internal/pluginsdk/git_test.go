package pluginsdk

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── ParseGitURL ──────────────────────────────────────────────────────────────

func TestParseGitURL_Simple(t *testing.T) {
	id, repoURL, err := ParseGitURL("git:github.com/user/my-plugin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "my-plugin" {
		t.Errorf("id = %q, want %q", id, "my-plugin")
	}
	if repoURL != "github.com/user/my-plugin" {
		t.Errorf("repoURL = %q, want %q", repoURL, "github.com/user/my-plugin")
	}
}

func TestParseGitURL_WithGitSuffix(t *testing.T) {
	id, repoURL, err := ParseGitURL("git:https://github.com/user/my-plugin.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "my-plugin" {
		t.Errorf("id = %q, want %q", id, "my-plugin")
	}
	if repoURL != "https://github.com/user/my-plugin" {
		t.Errorf("repoURL = %q, want %q", repoURL, "https://github.com/user/my-plugin")
	}
}

func TestParseGitURL_Invalid(t *testing.T) {
	cases := []string{
		"",                          // empty
		"https://github.com/user/x", // no git: prefix
		"github.com/user/x",         // no git: prefix
	}
	for _, url := range cases {
		if _, _, err := ParseGitURL(url); err == nil {
			t.Errorf("ParseGitURL(%q) expected error, got nil", url)
		}
	}
}

// ─── Install (error path only — real clones need network) ─────────────────────

func TestInstall_AlreadyInstalled(t *testing.T) {
	dir := t.TempDir()
	// pre-create the plugin destination so Install detects it as installed.
	dest := filepath.Join(dir, ".squadai", "plugins", "my-plugin")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, err := Install(dir, "git:github.com/user/my-plugin")
	if err == nil {
		t.Fatal("expected error for already-installed plugin, got nil")
	}
}

// ─── Remove ───────────────────────────────────────────────────────────────────

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, ".squadai", "plugins", "my-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := Remove(dir, "my-plugin"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Errorf("plugin dir still exists after Remove")
	}
}

func TestRemove_NotInstalled(t *testing.T) {
	dir := t.TempDir()
	if err := Remove(dir, "nonexistent"); err != nil {
		t.Errorf("Remove for missing plugin returned error: %v", err)
	}
}

// ─── List ─────────────────────────────────────────────────────────────────────

func TestList_Empty(t *testing.T) {
	dir := t.TempDir()
	ids, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty list, got %v", ids)
	}
}

func TestList_WithPlugins(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, ".squadai", "plugins")
	for _, name := range []string{"beta", "alpha", "gamma"} {
		if err := os.MkdirAll(filepath.Join(base, name), 0755); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	// a stray file should be ignored, not listed.
	if err := os.WriteFile(filepath.Join(base, "stray.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	ids, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"alpha", "beta", "gamma"}
	if len(ids) != len(want) {
		t.Fatalf("expected %d plugins, got %d: %v", len(want), len(ids), ids)
	}
	for i, w := range want {
		if ids[i] != w {
			t.Errorf("ids[%d] = %q, want %q", i, ids[i], w)
		}
	}
}

// ─── LoadManifest ─────────────────────────────────────────────────────────────

func TestLoadManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"name":"my-plugin","version":"1.0.0","description":"a plugin","skills":["s1"],"agents":["a1"]}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Name != "my-plugin" {
		t.Errorf("Name = %q, want %q", m.Name, "my-plugin")
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", m.Version, "1.0.0")
	}
	if m.Description != "a plugin" {
		t.Errorf("Description = %q, want %q", m.Description, "a plugin")
	}
	if len(m.Skills) != 1 || m.Skills[0] != "s1" {
		t.Errorf("Skills = %v, want [s1]", m.Skills)
	}
	if len(m.Agents) != 1 || m.Agents[0] != "a1" {
		t.Errorf("Agents = %v, want [a1]", m.Agents)
	}
}

func TestLoadManifest_NoFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadManifest(dir); err == nil {
		t.Fatal("expected error for missing plugin.json, got nil")
	}
}

func TestLoadManifest_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte("{not json"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := LoadManifest(dir); err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
