package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/governance"
)

// chdirTemp switches the working directory to a fresh temp dir for the test
// and restores the original directory on cleanup.
// setTTYHook overrides IsTTYHook for the test and restores it on cleanup.
func setTTYHook(t *testing.T, hook func() bool) {
	t.Helper()
	orig := IsTTYHook
	t.Cleanup(func() { IsTTYHook = orig })
	IsTTYHook = hook
}

// initPluginRepo creates a local git repository with a plugin.json manifest
// declaring one command, and returns the repo path and its HEAD commit SHA.
func initPluginRepo(t *testing.T) (repo, headSHA string) {
	t.Helper()
	repo = filepath.Join(t.TempDir(), "guard-plugin")
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
	manifest := `{"name":"guard-plugin","version":"1.0.0","commands":["cmd/hello.md"]}`
	if err := os.WriteFile(filepath.Join(repo, "plugin.json"), []byte(manifest), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	run("init", "-b", "main")
	run("add", ".")
	run("commit", "-m", "v1")
	headSHA = run("rev-parse", "HEAD")
	return repo, headSHA
}

// readPluginEvents returns all plugin:* audit events recorded in projectDir.
func readPluginEvents(t *testing.T, projectDir string) []governance.Event {
	t.Helper()
	events, err := governance.OpenAuditLog(projectDir).Read(0, "plugin")
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	return events
}

// ─── fail closed without a TTY ────────────────────────────────────────────────

func TestPluginsAddGit_NonTTYWithoutYesRefuses(t *testing.T) {
	dir := chdirTemp(t)
	setTTYHook(t, nil) // nil hook = non-interactive

	var stdout, stderr bytes.Buffer
	err := RunPluginsAddGit([]string{"git:github.com/user/some-plugin"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected refusal without --yes on a non-TTY, got nil")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error should tell the user to pass --yes, got: %v", err)
	}
	// Nothing may land on disk: no clone, no staging, no audit entry.
	if _, statErr := os.Stat(filepath.Join(dir, ".squadai", "plugins")); !os.IsNotExist(statErr) {
		t.Error("plugins dir was created despite the refusal")
	}
	if events := readPluginEvents(t, dir); len(events) != 0 {
		t.Errorf("expected no audit events, got %d", len(events))
	}
}

func TestPluginsAddGit_NonTTYFalseHookRefuses(t *testing.T) {
	chdirTemp(t)
	setTTYHook(t, func() bool { return false })

	var stdout, stderr bytes.Buffer
	err := RunPluginsAddGit([]string{"git:github.com/user/some-plugin"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected --yes refusal on non-TTY, got: %v", err)
	}
}

// ─── install with --yes: audit + unpinned warning ─────────────────────────────

func TestPluginsAddGit_YesInstallsWarnsUnpinnedAndAudits(t *testing.T) {
	repo, _ := initPluginRepo(t)
	dir := chdirTemp(t)
	setTTYHook(t, nil)

	var stdout, stderr bytes.Buffer
	if err := RunPluginsAddGit([]string{"git:" + repo, "--yes"}, &stdout, &stderr); err != nil {
		t.Fatalf("RunPluginsAddGit: %v\nstderr: %s", err, stderr.String())
	}

	// The unpinned-ref warning must fire.
	if !strings.Contains(stderr.String(), "unpinned") {
		t.Errorf("expected loud unpinned warning on stderr, got: %s", stderr.String())
	}
	// The summary must show the resolved repo URL and the manifest's commands.
	if !strings.Contains(stdout.String(), repo) {
		t.Errorf("stdout should show the resolved repo URL, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "cmd/hello.md") {
		t.Errorf("stdout should list the manifest's declared commands, got: %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".squadai", "plugins", "guard-plugin", "plugin.json")); err != nil {
		t.Errorf("plugin not installed: %v", err)
	}

	events := readPluginEvents(t, dir)
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].Kind != governance.KindPluginInstall {
		t.Errorf("event kind = %q, want %q", events[0].Kind, governance.KindPluginInstall)
	}
	if !strings.Contains(events[0].Detail, "guard-plugin") {
		t.Errorf("event detail should name the plugin, got: %s", events[0].Detail)
	}
}

func TestPluginsAddGit_PinnedSHASuppressesWarning(t *testing.T) {
	repo, sha := initPluginRepo(t)
	dir := chdirTemp(t)
	setTTYHook(t, nil)

	var stdout, stderr bytes.Buffer
	if err := RunPluginsAddGit([]string{"git:" + repo + "@" + sha, "--yes"}, &stdout, &stderr); err != nil {
		t.Fatalf("RunPluginsAddGit: %v\nstderr: %s", err, stderr.String())
	}
	if strings.Contains(stderr.String(), "unpinned") {
		t.Errorf("pinned install should not warn, got: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), sha) {
		t.Errorf("stdout should show the pinned commit, got: %s", stdout.String())
	}

	events := readPluginEvents(t, dir)
	if len(events) != 1 || events[0].Kind != governance.KindPluginInstall {
		t.Fatalf("expected 1 plugin:install audit event, got %v", events)
	}
	if !strings.Contains(events[0].Detail, "pinned=true") || !strings.Contains(events[0].Detail, sha) {
		t.Errorf("event detail should record the pin, got: %s", events[0].Detail)
	}
}

// ─── interactive confirmation ─────────────────────────────────────────────────

func TestPluginsAddGit_InteractiveDeclineInstallsNothing(t *testing.T) {
	repo, _ := initPluginRepo(t)
	dir := chdirTemp(t)
	setTTYHook(t, func() bool { return true })

	var stdout, stderr bytes.Buffer
	err := RunPluginsAddGitWithReader([]string{"git:" + repo}, &stdout, &stderr, strings.NewReader("n\n"))
	if err == nil {
		t.Fatal("expected abort error when the user declines, got nil")
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".squadai", "plugins", "guard-plugin")); !os.IsNotExist(statErr) {
		t.Error("plugin was installed despite the user declining")
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".squadai", "plugins", ".staging-guard-plugin")); !os.IsNotExist(statErr) {
		t.Error("staging dir left behind after decline")
	}
	if events := readPluginEvents(t, dir); len(events) != 0 {
		t.Errorf("expected no audit events after decline, got %d", len(events))
	}
}

func TestPluginsAddGit_InteractiveConfirmInstalls(t *testing.T) {
	repo, _ := initPluginRepo(t)
	dir := chdirTemp(t)
	setTTYHook(t, func() bool { return true })

	var stdout, stderr bytes.Buffer
	err := RunPluginsAddGitWithReader([]string{"git:" + repo}, &stdout, &stderr, strings.NewReader("y\n"))
	if err != nil {
		t.Fatalf("RunPluginsAddGitWithReader: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".squadai", "plugins", "guard-plugin", "plugin.json")); statErr != nil {
		t.Errorf("plugin not installed after confirmation: %v", statErr)
	}
}

// ─── remove-git ───────────────────────────────────────────────────────────────

func TestPluginsRemoveGit_RemovesAndAudits(t *testing.T) {
	repo, _ := initPluginRepo(t)
	dir := chdirTemp(t)
	setTTYHook(t, nil)

	var stdout, stderr bytes.Buffer
	if err := RunPluginsAddGit([]string{"git:" + repo, "--yes"}, &stdout, &stderr); err != nil {
		t.Fatalf("install: %v", err)
	}

	stdout.Reset()
	if err := RunPluginsRemoveGit([]string{"guard-plugin"}, &stdout); err != nil {
		t.Fatalf("RunPluginsRemoveGit: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".squadai", "plugins", "guard-plugin")); !os.IsNotExist(statErr) {
		t.Error("plugin dir still exists after remove-git")
	}

	events := readPluginEvents(t, dir)
	if len(events) != 2 {
		t.Fatalf("expected install + remove audit events, got %d", len(events))
	}
	if events[1].Kind != governance.KindPluginRemove {
		t.Errorf("second event kind = %q, want %q", events[1].Kind, governance.KindPluginRemove)
	}
}

func TestPluginsRemoveGit_NotInstalled(t *testing.T) {
	chdirTemp(t)
	var stdout bytes.Buffer
	if err := RunPluginsRemoveGit([]string{"missing-plugin"}, &stdout); err == nil {
		t.Fatal("expected error for missing plugin, got nil")
	}
}
