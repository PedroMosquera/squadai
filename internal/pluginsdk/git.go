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
	"regexp"
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
	PluginID  string
	Path      string
	Manifest  *Manifest
	RepoURL   string
	Ref       string // ref requested in the URL ("" = default branch)
	Pinned    bool   // true when Ref is a full commit SHA
	CommitSHA string // resolved HEAD commit of the installed checkout
}

// pluginsDir is the relative location of installed git-based plugins.
const pluginsDir = ".squadai/plugins"

// commitSHARe matches a full 40-character hex commit SHA.
var commitSHARe = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// IsCommitSHA reports whether ref is a full 40-character commit SHA,
// i.e. a pin that makes the install reproducible.
func IsCommitSHA(ref string) bool {
	return commitSHARe.MatchString(ref)
}

// ParseGitURL extracts a plugin ID, repository URL, and optional ref from a
// git: URL. An "@<ref>" suffix pins the install to a commit SHA (preferred)
// or names a branch/tag.
//
//	"git:github.com/user/my-plugin"            -> "my-plugin", "github.com/user/my-plugin", ""
//	"git:https://github.com/user/my-plugin.git" -> "my-plugin", "https://github.com/user/my-plugin", ""
//	"git:github.com/user/my-plugin@<sha>"       -> "my-plugin", "github.com/user/my-plugin", "<sha>"
func ParseGitURL(url string) (id string, repoURL string, ref string, err error) {
	if url == "" {
		return "", "", "", fmt.Errorf("empty git url")
	}
	if !strings.HasPrefix(url, "git:") {
		return "", "", "", fmt.Errorf("url must start with 'git:' prefix, got %q", url)
	}
	repoURL = strings.TrimPrefix(url, "git:")

	// A trailing "@<ref>" pins the install. Only the last "@" counts, and only
	// when the suffix cannot be part of the URL itself (ssh URLs contain
	// "user@host:path", so a ref must not contain "/" or ":").
	if at := strings.LastIndex(repoURL, "@"); at > 0 {
		suffix := repoURL[at+1:]
		if suffix != "" && !strings.ContainsAny(suffix, "/:") {
			ref = suffix
			repoURL = repoURL[:at]
		}
	}

	repoURL = strings.TrimSuffix(repoURL, ".git")
	repoURL = strings.TrimRight(repoURL, "/")

	segments := strings.Split(repoURL, "/")
	id = segments[len(segments)-1]
	if id == "" {
		return "", "", "", fmt.Errorf("could not extract plugin id from %q", url)
	}
	return id, repoURL, ref, nil
}

// StagedInstall is a plugin that has been cloned into a quarantine staging
// directory but not yet installed. git clone executes no repository-supplied
// code, so staging lets callers inspect the manifest and ask the user for
// confirmation before anything lands in the plugins directory.
type StagedInstall struct {
	PluginID   string
	RepoURL    string
	Ref        string // ref requested in the URL ("" = default branch)
	Pinned     bool   // true when Ref is a full commit SHA
	CommitSHA  string // resolved HEAD commit of the staged checkout
	Manifest   *Manifest
	stagingDir string
	finalDir   string
}

// Stage clones a git-based plugin into a staging directory next to the final
// destination and reads its plugin.json manifest. Call Commit to finish the
// install or Discard to delete the staged clone.
//
// A ref that is a full commit SHA is checked out exactly (full clone +
// detached checkout) so the install is reproducible. Other refs keep the
// shallow-clone optimization.
func Stage(projectDir, gitURL string) (*StagedInstall, error) {
	id, repoURL, ref, err := ParseGitURL(gitURL)
	if err != nil {
		return nil, fmt.Errorf("parse git url: %w", err)
	}

	finalDir := filepath.Join(projectDir, pluginsDir, id)
	if _, err := os.Stat(finalDir); err == nil {
		return nil, fmt.Errorf("plugin %q already installed", id)
	}
	if err := os.MkdirAll(filepath.Dir(finalDir), 0755); err != nil {
		return nil, fmt.Errorf("create plugins dir: %w", err)
	}

	// Stage inside the plugins dir so Commit is a same-filesystem rename.
	stagingDir := filepath.Join(projectDir, pluginsDir, ".staging-"+id)
	_ = os.RemoveAll(stagingDir) // clear leftovers from an aborted install

	pinned := IsCommitSHA(ref)
	cloneArgs := []string{"clone"}
	switch {
	case pinned:
		// A commit SHA cannot be passed to --branch; clone the full history
		// and check the pinned commit out afterwards.
	case ref != "":
		cloneArgs = append(cloneArgs, "--depth", "1", "--branch", ref)
	default:
		cloneArgs = append(cloneArgs, "--depth", "1")
	}
	cloneArgs = append(cloneArgs, repoURL, stagingDir)

	cmd := exec.Command("git", cloneArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(stagingDir)
		return nil, fmt.Errorf("git clone %q: %w\n%s", repoURL, err, out)
	}

	if pinned {
		cmd := exec.Command("git", "-C", stagingDir, "checkout", "--detach", ref)
		if out, err := cmd.CombinedOutput(); err != nil {
			_ = os.RemoveAll(stagingDir)
			return nil, fmt.Errorf("git checkout %q: %w\n%s", ref, err, out)
		}
	}

	commitSHA := ""
	if out, err := exec.Command("git", "-C", stagingDir, "rev-parse", "HEAD").Output(); err == nil {
		commitSHA = strings.TrimSpace(string(out))
	}

	manifest, _ := LoadManifest(stagingDir) // nil manifest is non-fatal
	return &StagedInstall{
		PluginID:   id,
		RepoURL:    repoURL,
		Ref:        ref,
		Pinned:     pinned,
		CommitSHA:  commitSHA,
		Manifest:   manifest,
		stagingDir: stagingDir,
		finalDir:   finalDir,
	}, nil
}

// Commit moves the staged clone into .squadai/plugins/<id>/.
func (s *StagedInstall) Commit() (*InstallResult, error) {
	if err := os.Rename(s.stagingDir, s.finalDir); err != nil {
		_ = os.RemoveAll(s.stagingDir)
		return nil, fmt.Errorf("install plugin %q: %w", s.PluginID, err)
	}
	return &InstallResult{
		PluginID:  s.PluginID,
		Path:      s.finalDir,
		Manifest:  s.Manifest,
		RepoURL:   s.RepoURL,
		Ref:       s.Ref,
		Pinned:    s.Pinned,
		CommitSHA: s.CommitSHA,
	}, nil
}

// Discard deletes the staged clone without installing it.
func (s *StagedInstall) Discard() error {
	return os.RemoveAll(s.stagingDir)
}

// Install clones a git-based plugin into .squadai/plugins/<id>/ and reads its
// plugin.json manifest. It performs no confirmation — callers that install
// network-sourced plugins interactively should use Stage/Commit instead.
func Install(projectDir, gitURL string) (*InstallResult, error) {
	staged, err := Stage(projectDir, gitURL)
	if err != nil {
		return nil, err
	}
	return staged.Commit()
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
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
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
