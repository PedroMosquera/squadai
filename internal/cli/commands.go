package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/codex"
	"github.com/PedroMosquera/squadai/internal/adapters/cursor"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/pi"
	"github.com/PedroMosquera/squadai/internal/adapters/vscode"
	"github.com/PedroMosquera/squadai/internal/adapters/windsurf"
	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// Version is the CLI version string, set by app before calling any Run* function.
var Version = "dev"

// allAdapters returns one instance of every registered adapter, OpenCode first.
func allAdapters() []domain.Adapter {
	return []domain.Adapter{
		opencode.New(),
		claude.New(),
		vscode.New(),
		cursor.New(),
		windsurf.New(),
		pi.New(),
		codex.New(),
	}
}

// DetectAdapters returns all registered adapters that are installed or have config.
// OpenCode (team lane) is always included. Personal-lane adapters (Claude Code,
// VS Code Copilot, Cursor, Windsurf, Pi, Codex) are included only when detected
// on the system.
func DetectAdapters(homeDir string) []domain.Adapter {
	ctx := context.Background()
	var adapters []domain.Adapter
	for _, a := range allAdapters() {
		if a.ID() == domain.AgentOpenCode {
			adapters = append(adapters, a)
			continue
		}
		if installed, configFound, err := a.Detect(ctx, homeDir); err == nil && (installed || configFound) {
			adapters = append(adapters, a)
		}
	}
	return adapters
}

// filterAdapters returns only adapters whose ID is in the selections set.
// If selections is nil/empty, returns detected unchanged (backward-compatible).
// If selections is non-empty but no detected adapter matches, returns an empty slice.
func filterAdapters(detected []domain.Adapter, selections []string) []domain.Adapter {
	if len(selections) == 0 {
		return detected
	}
	allowed := make(map[string]bool, len(selections))
	for _, s := range selections {
		allowed[strings.TrimSpace(s)] = true
	}

	var result []domain.Adapter
	for _, a := range detected {
		if allowed[string(a.ID())] {
			result = append(result, a)
		}
	}
	return result
}

// LoadAndMerge is the shared config loading logic for commands that need merged config.
func LoadAndMerge(homeDir, projectDir string) (*domain.MergedConfig, error) {
	return loadAndMerge(homeDir, projectDir)
}

// loadAndMerge is the shared config loading logic for commands that need merged config.
func loadAndMerge(homeDir, projectDir string) (*domain.MergedConfig, error) {
	user, err := config.LoadUser(homeDir)
	if err != nil && !errors.Is(err, domain.ErrConfigNotFound) {
		return nil, fmt.Errorf("load user config: %w", err)
	}

	project, err := config.LoadProject(projectDir)
	if err != nil && !errors.Is(err, domain.ErrConfigNotFound) {
		return nil, fmt.Errorf("load project config: %w", err)
	}

	policy, err := config.LoadPolicy(projectDir)
	if err != nil && !errors.Is(err, domain.ErrConfigNotFound) {
		return nil, fmt.Errorf("load policy: %w", err)
	}

	return config.Merge(user, project, policy), nil
}

// timeNowUTC returns the current UTC time. Extracted for testability.
var timeNowUTC = func() time.Time {
	return time.Now().UTC()
}
