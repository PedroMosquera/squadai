package update

import (
	"context"
	"fmt"
	"io"
)

// CheckResult holds the result of a version check.
type CheckResult struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	Release         *githubRelease
}

// Check queries GitHub Releases for the latest stable release and compares it
// to currentVersion. Returns ErrDevBuild if currentVersion is a dev build,
// ErrUpToDate if already current, or ErrNoRelease if no stable release exists.
func Check(ctx context.Context, currentVersion string) (*CheckResult, error) {
	if isDevBuild(currentVersion) {
		return nil, ErrDevBuild
	}

	current, err := parseSemver(currentVersion)
	if err != nil {
		return nil, fmt.Errorf("parse current version %q: %w", currentVersion, err)
	}

	release, err := fetchLatestRelease(ctx, currentVersion)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}

	latest, err := parseSemver(release.TagName)
	if err != nil {
		return nil, fmt.Errorf("parse latest version %q: %w", release.TagName, err)
	}

	result := &CheckResult{
		CurrentVersion:  currentVersion,
		LatestVersion:   release.TagName,
		UpdateAvailable: isNewer(current, latest),
		Release:         release,
	}
	return result, nil
}

// Run performs a complete update cycle: check → download → notify.
// progress messages are written to w (typically stderr).
// Returns ErrDevBuild or ErrUpToDate without error when no action is needed.
func Run(ctx context.Context, currentVersion string, w io.Writer) error {
	result, err := Check(ctx, currentVersion)
	if err != nil {
		return err
	}

	if !result.UpdateAvailable {
		return ErrUpToDate
	}

	fmt.Fprintf(w, "squadai %s is available — downloading in background…\n", result.LatestVersion)

	if err := Download(ctx, result.Release, func(msg string) {
		fmt.Fprintf(w, "%s\n", msg)
	}); err != nil {
		return fmt.Errorf("download update: %w", err)
	}

	fmt.Fprintf(w, "✓ squadai %s ready. It will apply on next launch.\n", result.LatestVersion)
	return nil
}
