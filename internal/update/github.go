package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	githubAPIBase = "https://api.github.com"
	repoOwner     = "PedroMosquera"
	repoName      = "squadai"
)

// checkClient is the HTTP client used for release checks (short timeout).
var checkClient = &http.Client{Timeout: 5 * time.Second}

// githubRelease is the subset of the GitHub Releases API response we need.
type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Name       string        `json:"name"`
	Body       string        `json:"body"`
	Prerelease bool          `json:"prerelease"`
	Draft      bool          `json:"draft"`
	Assets     []githubAsset `json:"assets"`
}

// githubAsset represents a single release asset.
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// fetchLatestRelease retrieves the latest non-draft, non-prerelease from GitHub.
// It returns ErrNoRelease if the latest release is a draft or prerelease.
func fetchLatestRelease(ctx context.Context, currentVersion string) (*githubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", githubAPIBase, repoOwner, repoName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", fmt.Sprintf("squadai/%s", currentVersion))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := checkClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode github response: %w", err)
	}

	if release.Draft || release.Prerelease {
		return nil, ErrNoRelease
	}

	return &release, nil
}
