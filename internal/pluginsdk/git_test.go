package pluginsdk

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ─── ParseGitURL ──────────────────────────────────────────────────────────────

func TestParseGitURL_Simple(t *testing.T) {
	id, repoURL, ref, err := ParseGitURL("git:github.com/user/my-plugin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "my-plugin" {
		t.Errorf("id = %q, want %q", id, "my-plugin")
	}
	if repoURL != "github.com/user/my-plugin" {
		t.Errorf("repoURL = %q, want %q", repoURL, "github.com/user/my-plugin")
	}
	if ref != "" {
		t.Errorf("ref = %q, want empty", ref)
	}
}

func TestParseGitURL_WithGitSuffix(t *testing.T) {
	id, repoURL, ref, err := ParseGitURL("git:https://github.com/user/my-plugin.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "my-plugin" {
		t.Errorf("id = %q, want %q", id, "my-plugin")
	}
	if repoURL != "https://github.com/user/my-plugin" {
		t.Errorf("repoURL = %q, want %q", repoURL, "https://github.com/user/my-plugin")
	}
	if ref != "" {
		t.Errorf("ref = %q, want empty", ref)
	}
}

func TestParseGitURL_WithRef(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	id, repoURL, ref, err := ParseGitURL("git:github.com/user/my-plugin@" + sha)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "my-plugin" {
		t.Errorf("id = %q, want %q", id, "my-plugin")
	}
	if repoURL != "github.com/user/my-plugin" {
		t.Errorf("repoURL = %q, want %q", repoURL, "github.com/user/my-plugin")
	}
	if ref != sha {
		t.Errorf("ref = %q, want %q", ref, sha)
	}
	if !IsCommitSHA(ref) {
		t.Errorf("IsCommitSHA(%q) = false, want true", ref)
	}
}

func TestParseGitURL_SSHUserNotTreatedAsRef(t *testing.T) {
	// The "@" in an ssh user@host URL must not be mistaken for a ref.
	id, repoURL, ref, err := ParseGitURL("git:git@github.com:user/my-plugin.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "my-plugin" {
		t.Errorf("id = %q, want %q", id, "my-plugin")
	}
	if repoURL != "git@github.com:user/my-plugin" {
		t.Errorf("repoURL = %q, want %q", repoURL, "git@github.com:user/my-plugin")
	}
	if ref != "" {
		t.Errorf("ref = %q, want empty", ref)
	}
}

func TestParseGitURL_Invalid(t *testing.T) {
	cases := []string{
		"",                          // empty
		"https://github.com/user/x", // no git: prefix
		"github.com/user/x",         // no git: prefix
	}
	for _, url := range cases {
		if _, _, _, err := ParseGitURL(url); err == nil {
			t.Errorf("ParseGitURL(%q) expected error, got nil", url)
		}
	}
}

func TestIsCommitSHA(t *testing.T) {
	if !IsCommitSHA("0123456789abcdef0123456789abcdef01234567") {
		t.Error("full 40-char sha should be a pin")
	}
	for _, ref := range []string{"", "main", "v1.2.3", "abc123", "0123456789abcdef0123456789abcdef0123456"} {
		if IsCommitSHA(ref) {
			t.Errorf("IsCommitSHA(%q) = true, want false", ref)
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

// ─── Stage (local repos, no network) ──────────────────────────────────────────

// initTestRepo creates a local git repository named my-plugin with two
// commits and returns the repo path plus the first commit's SHA. The first
// commit carries plugin.json version 1.0.0, the second version 2.0.0.
func initTestRepo(t *testing.T) (repo, firstSHA string) {
	t.Helper()
	repo = filepath.Join(t.TempDir(), "my-plugin")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	writeManifest := func(version string) {
		t.Helper()
		manifest := `{"name":"my-plugin","version":"` + version + `","commands":["cmd/hello.md"]}`
		if err := os.WriteFile(filepath.Join(repo, "plugin.json"), []byte(manifest), 0644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	run("init", "-b", "main")
	writeManifest("1.0.0")
	run("add", ".")
	run("commit", "-m", "v1")
	firstSHA = run("rev-parse", "HEAD")
	writeManifest("2.0.0")
	run("add", ".")
	run("commit", "-m", "v2")
	return repo, firstSHA
}

func TestStage_PinnedInstallIsReproducible(t *testing.T) {
	repo, firstSHA := initTestRepo(t)
	projectDir := t.TempDir()

	staged, err := Stage(projectDir, "git:"+repo+"@"+firstSHA)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if !staged.Pinned {
		t.Error("Pinned = false, want true for a full-SHA ref")
	}
	if staged.CommitSHA != firstSHA {
		t.Errorf("CommitSHA = %q, want pinned %q", staged.CommitSHA, firstSHA)
	}
	if staged.Manifest == nil || staged.Manifest.Version != "1.0.0" {
		t.Fatalf("Manifest = %+v, want version 1.0.0 from the pinned commit", staged.Manifest)
	}

	result, err := staged.Commit()
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if result.CommitSHA != firstSHA {
		t.Errorf("result.CommitSHA = %q, want %q", result.CommitSHA, firstSHA)
	}
	m, err := LoadManifest(result.Path)
	if err != nil {
		t.Fatalf("LoadManifest after install: %v", err)
	}
	if m.Version != "1.0.0" {
		t.Errorf("installed version = %q, want pinned 1.0.0", m.Version)
	}
}

func TestStage_UnpinnedTracksHead(t *testing.T) {
	repo, firstSHA := initTestRepo(t)
	projectDir := t.TempDir()

	staged, err := Stage(projectDir, "git:"+repo)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if staged.Pinned {
		t.Error("Pinned = true, want false for a default-branch install")
	}
	if staged.CommitSHA == firstSHA {
		t.Error("unpinned install should track HEAD, not the first commit")
	}
	if staged.Manifest == nil || staged.Manifest.Version != "2.0.0" {
		t.Fatalf("Manifest = %+v, want version 2.0.0 from HEAD", staged.Manifest)
	}
	if _, err := staged.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

func TestStage_DiscardLeavesNothingInstalled(t *testing.T) {
	repo, _ := initTestRepo(t)
	projectDir := t.TempDir()

	staged, err := Stage(projectDir, "git:"+repo)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if err := staged.Discard(); err != nil {
		t.Fatalf("Discard: %v", err)
	}
	ids, err := List(projectDir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected no installed plugins after Discard, got %v", ids)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".squadai", "plugins", ".staging-my-plugin")); !os.IsNotExist(err) {
		t.Error("staging dir still exists after Discard")
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
