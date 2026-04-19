package update

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrNoRelease is returned when no suitable stable release is found.
var ErrNoRelease = errors.New("no stable release available")

// ErrUpToDate is returned when the current version is already the latest.
var ErrUpToDate = errors.New("already up to date")

// ErrDevBuild is returned when the current build is a development build.
var ErrDevBuild = errors.New("dev build: update checks skipped")

// semver holds a parsed semantic version.
type semver struct {
	Major, Minor, Patch int
}

// parseSemver parses a version string like "v1.2.3" or "1.2.3".
// Returns an error if the string is not a valid semver.
func parseSemver(s string) (semver, error) {
	s = strings.TrimPrefix(s, "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return semver{}, fmt.Errorf("invalid semver %q: expected major.minor.patch", s)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return semver{}, fmt.Errorf("invalid major in %q: %w", s, err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return semver{}, fmt.Errorf("invalid minor in %q: %w", s, err)
	}
	// Strip any pre-release or build metadata suffix from patch.
	patchStr := parts[2]
	if idx := strings.IndexAny(patchStr, "-+"); idx >= 0 {
		patchStr = patchStr[:idx]
	}
	patch, err := strconv.Atoi(patchStr)
	if err != nil {
		return semver{}, fmt.Errorf("invalid patch in %q: %w", s, err)
	}
	return semver{Major: major, Minor: minor, Patch: patch}, nil
}

// isNewer returns true if candidate is strictly newer than current.
func isNewer(current, candidate semver) bool {
	if candidate.Major != current.Major {
		return candidate.Major > current.Major
	}
	if candidate.Minor != current.Minor {
		return candidate.Minor > current.Minor
	}
	return candidate.Patch > current.Patch
}

// isDevBuild returns true if the version string indicates a development build.
func isDevBuild(version string) bool {
	return version == "" || version == "dev"
}

// IsDevBuild is the exported form for use by external packages.
func IsDevBuild(version string) bool {
	return isDevBuild(version)
}
