package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
)

// Fixer is a function that resolves a specific auto-fixable issue.
type Fixer func(ctx context.Context, d *Doctor) error

// FixResult holds the outcome of running a single fixer.
type FixResult struct {
	CheckResult CheckResult
	Err         error
}

// fixerRegistry maps "Category.Name" to a Fixer function.
// The key is exactly CheckResult.Category + "." + CheckResult.Name.
var fixerRegistry = map[string]Fixer{
	"Project Configuration.config.json":      fixCreateUserConfig,
	"Filesystem.write access to ~/.squadai/": fixCreateSquadAIDir,
}

// fixKey returns the registry key for a CheckResult.
func fixKey(r CheckResult) string {
	return r.Category + "." + r.Name
}

// Fix runs the fixers for all auto-fixable failures in results.
// It returns one FixResult per item that was attempted.
func (d *Doctor) Fix(ctx context.Context, results []CheckResult) []FixResult {
	var out []FixResult
	for _, r := range results {
		if !r.AutoFixable || r.Status != CheckFail {
			continue
		}
		key := fixKey(r)
		fn, ok := fixerRegistry[key]
		if !ok {
			out = append(out, FixResult{
				CheckResult: r,
				Err:         fmt.Errorf("no fixer registered for %q", key),
			})
			continue
		}
		err := fn(ctx, d)
		out = append(out, FixResult{CheckResult: r, Err: err})
	}
	return out
}

// fixCreateUserConfig creates ~/.squadai/config.json with DefaultUserConfig.
func fixCreateUserConfig(_ context.Context, d *Doctor) error {
	squadaiDir := filepath.Join(d.homeDir, config.UserConfigDir)
	if err := os.MkdirAll(squadaiDir, 0o755); err != nil {
		return fmt.Errorf("create ~/.squadai directory: %w", err)
	}

	cfg := domain.DefaultUserConfig()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal default user config: %w", err)
	}

	path := config.UserConfigPath(d.homeDir)
	if _, err := fileutil.WriteAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("write default user config: %w", err)
	}
	return nil
}

// fixCreateSquadAIDir creates the ~/.squadai directory.
func fixCreateSquadAIDir(_ context.Context, d *Doctor) error {
	squadaiDir := filepath.Join(d.homeDir, ".squadai")
	if err := os.MkdirAll(squadaiDir, 0o755); err != nil {
		return fmt.Errorf("create ~/.squadai directory: %w", err)
	}
	return nil
}
