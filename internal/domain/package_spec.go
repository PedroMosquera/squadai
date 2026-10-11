package domain

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// PackageSpec is a plugin package in Pi's source syntax, which SquadAI uses
// for every harness: "npm:<name>@<version>" or "git:<host>/<path>@<ref>"
// (https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md).
type PackageSpec struct {
	Kind string // "npm" or "git"
	// Name is the npm package name, or the git host and repository path
	// without a trailing ".git".
	Name string
	// Version is the npm version or the git ref, empty when the spec has none.
	Version string
}

// exactVersion is a full semver version: ranges, dist-tags and partial
// versions such as 1.x or ^1.2.3 can resolve to a different release later.
var exactVersion = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

// ParsePackageSpec parses a declared package spec. It accepts only the npm
// and git forms; whether the spec is pinned is a separate question (Pinned).
func ParsePackageSpec(s string) (PackageSpec, error) {
	kind, rest, ok := strings.Cut(s, ":")
	if !ok || (kind != "npm" && kind != "git") {
		return PackageSpec{}, fmt.Errorf("package %q must start with npm: or git:", s)
	}
	p := PackageSpec{Kind: kind}
	switch kind {
	case "npm":
		p.Name, p.Version = splitVersion(rest, 1)
		if p.Name == "" || strings.HasPrefix(p.Name, "@") && !strings.Contains(p.Name, "/") {
			return PackageSpec{}, fmt.Errorf("package %q has no npm package name", s)
		}
	case "git":
		slash := strings.Index(rest, "/")
		if slash <= 0 || slash == len(rest)-1 {
			return PackageSpec{}, fmt.Errorf("package %q must name a git host and repository, as git:github.com/org/repo@ref", s)
		}
		p.Name, p.Version = splitVersion(rest, slash)
		p.Name = strings.TrimSuffix(p.Name, ".git")
	}
	return p, nil
}

// splitVersion splits "<name>@<version>" at the last "@" at or after from,
// so a scope's leading "@" or an "@" in an ssh host is not taken as the
// version separator.
func splitVersion(s string, from int) (name, version string) {
	if from > len(s) {
		return s, ""
	}
	at := strings.LastIndex(s[from:], "@")
	if at < 0 {
		return s, ""
	}
	return s[:from+at], s[from+at+1:]
}

// Pinned reports whether the spec names one release: an exact npm version or
// a git ref. Packages run code in the agent, so an unpinned spec would load a
// new or compromised release without review. A git ref can still be a branch;
// only tags and commits stay fixed.
func (p PackageSpec) Pinned() bool {
	if p.Kind == "npm" {
		return exactVersion.MatchString(p.Version)
	}
	return p.Version != ""
}

// Identity is what the harnesses deduplicate on: the npm package name or the
// git repository without its ref. Two specs with one identity are two
// versions of the same package.
func (p PackageSpec) Identity() string { return p.Kind + ":" + p.Name }

// Source is the form allowed.plugin_sources matches: "npm:<name>" for npm and
// "<host>/<path>" for git, the same form a github marketplace location takes.
func (p PackageSpec) Source() string {
	if p.Kind == "npm" {
		return "npm:" + p.Name
	}
	return p.Name
}

func (p PackageSpec) String() string {
	if p.Version == "" {
		return p.Kind + ":" + p.Name
	}
	return p.Kind + ":" + p.Name + "@" + p.Version
}

// PackageSettings returns the project file and top-level array key where the
// agent reads plugin packages, or "" for agents without one.
func PackageSettings(a Adapter, projectDir string) (path, key string) {
	switch a.ID() {
	case AgentPi:
		return filepath.Join(projectDir, ".pi", "settings.json"), PackageSettingsKey(AgentPi)
	case AgentOpenCode:
		return a.ProjectConfigFile(projectDir), PackageSettingsKey(AgentOpenCode)
	}
	return "", ""
}

// PackageSettingsKey is the array key holding plugin packages in the agent's
// settings file, at project and user scope alike.
func PackageSettingsKey(agent AgentID) string {
	switch agent {
	case AgentPi:
		return "packages"
	case AgentOpenCode:
		return "plugin"
	}
	return ""
}

// HarnessPackageEntry is the string the agent expects for spec. Pi takes the
// spec as written. OpenCode installs npm packages by name and version
// (https://opencode.ai/docs/plugins/) and has no git form, so ok is false
// for a git spec there.
func HarnessPackageEntry(agent AgentID, spec PackageSpec) (entry string, ok bool) {
	switch agent {
	case AgentPi:
		return spec.String(), true
	case AgentOpenCode:
		if spec.Kind != "npm" {
			return "", false
		}
		return strings.TrimPrefix(spec.String(), "npm:"), true
	}
	return "", false
}

// ParseHarnessPackage reads one string entry of the agent's package array as
// a PackageSpec. ok is false for local paths, which load files already in
// the checkout, and for forms SquadAI does not recognize.
func ParseHarnessPackage(agent AgentID, entry string) (PackageSpec, bool) {
	if LocalPackageEntry(entry) {
		return PackageSpec{}, false
	}
	switch agent {
	case AgentPi:
		// Pi treats an https URL as a git source.
		for _, scheme := range []string{"https://", "http://"} {
			if rest, ok := strings.CutPrefix(entry, scheme); ok {
				entry = "git:" + rest
			}
		}
	case AgentOpenCode:
		if strings.Contains(entry, ":") {
			return PackageSpec{}, false
		}
		entry = "npm:" + entry
	default:
		return PackageSpec{}, false
	}
	p, err := ParsePackageSpec(entry)
	return p, err == nil
}

// LocalPackageEntry reports whether a package entry is a filesystem path,
// the form both Pi and OpenCode resolve relative to the settings file.
func LocalPackageEntry(entry string) bool {
	return strings.HasPrefix(entry, ".") || strings.HasPrefix(entry, "/") || strings.HasPrefix(entry, "~") ||
		strings.HasPrefix(entry, "file:") || len(entry) > 2 && entry[1] == ':' && (entry[2] == '\\' || entry[2] == '/')
}
