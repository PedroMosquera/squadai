package integration_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/components/mcp"
	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/managed"
)

// TestUserWinsSmoke_PreviewerFlagsHandEditedKey is the end-to-end gate for
// the user-wins safety story: if a user has hand-edited a top-level key
// SquadAI would overwrite and the sidecar does NOT claim the key, the
// Previewer must emit a Conflict entry so the review screen can block the
// apply.
func TestUserWinsSmoke_PreviewerFlagsHandEditedKey(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	if err := config.WriteJSON(config.UserConfigPath(home), domain.DefaultUserConfig()); err != nil {
		t.Fatalf("write user config: %v", err)
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), domain.DefaultProjectConfig()); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	// Seed an opencode.json that the user has hand-edited: the "mcp" key
	// exists but SquadAI has never claimed it (no sidecar entry).
	target := filepath.Join(project, "opencode.json")
	userConfig := map[string]any{
		"mcp": map[string]any{
			"user-managed-server": map[string]any{
				"type": "remote",
				"url":  "https://user-only.example.com/mcp",
			},
		},
	}
	writeJSON(t, target, userConfig)

	installer := mcp.New(map[string]domain.MCPServerDef{
		"context7": {Type: "remote", URL: "https://mcp.context7.com/mcp", Enabled: true},
	})

	entries, err := installer.Preview(opencode.New(), home, project)
	if err != nil {
		t.Fatalf("Preview() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", entries[0].Action, domain.ActionUpdate)
	}
	if len(entries[0].Conflicts) != 1 {
		t.Fatalf("expected 1 conflict over unmanaged 'mcp' key, got %d: %+v",
			len(entries[0].Conflicts), entries[0].Conflicts)
	}
	if entries[0].Conflicts[0].Key != "mcp" {
		t.Errorf("Conflict.Key = %q, want %q", entries[0].Conflicts[0].Key, "mcp")
	}
}

// TestUserWinsSmoke_ManagedKeyOverwritesCleanly verifies the complementary
// case: when SquadAI has previously claimed the key in the sidecar, the
// Previewer must NOT emit a conflict even though the on-disk value differs
// from what we'd write. This is how a normal re-apply stays unobtrusive.
func TestUserWinsSmoke_ManagedKeyOverwritesCleanly(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	if err := config.WriteJSON(config.UserConfigPath(home), domain.DefaultUserConfig()); err != nil {
		t.Fatalf("write user config: %v", err)
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), domain.DefaultProjectConfig()); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	target := filepath.Join(project, "opencode.json")
	staleConfig := map[string]any{
		"mcp": map[string]any{
			"context7": map[string]any{
				"type": "remote",
				"url":  "https://stale.example.com/mcp", // old URL SquadAI wrote previously
			},
		},
	}
	writeJSON(t, target, staleConfig)

	// Record that SquadAI owns the "mcp" key for this file.
	relPath, err := filepath.Rel(project, target)
	if err != nil {
		t.Fatalf("rel: %v", err)
	}
	if err := managed.WriteManagedKeys(project, relPath, []string{"mcp"}); err != nil {
		t.Fatalf("seed managed keys: %v", err)
	}

	installer := mcp.New(map[string]domain.MCPServerDef{
		"context7": {Type: "remote", URL: "https://mcp.context7.com/mcp", Enabled: true},
	})

	entries, err := installer.Preview(opencode.New(), home, project)
	if err != nil {
		t.Fatalf("Preview() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", entries[0].Action, domain.ActionUpdate)
	}
	if len(entries[0].Conflicts) != 0 {
		t.Errorf("managed key overwrite must not produce conflicts, got %+v", entries[0].Conflicts)
	}
	if entries[0].Diff == "" {
		t.Error("update of stale URL should still produce a diff for user visibility")
	}
}

// TestUserWinsSmoke_OverrideWritesThrough verifies the end-to-end flow when
// the user grants per-key consent from the review screen: Preview flags the
// conflict, the policy marks the key for overwrite, and Apply writes through
// cleanly and updates the sidecar.
func TestUserWinsSmoke_OverrideWritesThrough(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	if err := config.WriteJSON(config.UserConfigPath(home), domain.DefaultUserConfig()); err != nil {
		t.Fatalf("write user config: %v", err)
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), domain.DefaultProjectConfig()); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	target := filepath.Join(project, "opencode.json")
	userConfig := map[string]any{
		"mcp": map[string]any{
			"user-managed-server": map[string]any{
				"type": "remote",
				"url":  "https://user-only.example.com/mcp",
			},
		},
	}
	writeJSON(t, target, userConfig)

	installer := mcp.New(map[string]domain.MCPServerDef{
		"context7": {Type: "remote", URL: "https://mcp.context7.com/mcp", Enabled: true},
	})

	// Preview surfaces the conflict.
	entries, err := installer.Preview(opencode.New(), home, project)
	if err != nil {
		t.Fatalf("Preview() error: %v", err)
	}
	if len(entries[0].Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %+v", entries[0].Conflicts)
	}

	// Simulate the user granting consent via the review screen.
	installer.SetApplyPolicy(domain.ApplyPolicy{
		Overrides: map[string]map[string]bool{
			target: {"mcp": true},
		},
	})

	actions, err := installer.Plan(opencode.New(), home, project)
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}
	for _, a := range actions {
		if err := installer.Apply(a); err != nil {
			t.Fatalf("Apply(%s) error: %v", a.Description, err)
		}
	}

	// File should now contain SquadAI's mcp value, not the user's.
	got := readJSON(t, target)
	mcpBlock, ok := got["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("mcp key missing or wrong type: %v", got["mcp"])
	}
	if _, hasContext7 := mcpBlock["context7"]; !hasContext7 {
		t.Errorf("expected context7 server under mcp after override, got %v", mcpBlock)
	}
	if _, hasUserOnly := mcpBlock["user-managed-server"]; hasUserOnly {
		t.Errorf("user-managed-server should be replaced after overwrite, got %v", mcpBlock)
	}

	// Sidecar should now claim "mcp".
	keys, err := managed.ReadManagedKeys(project, "opencode.json")
	if err != nil {
		t.Fatalf("ReadManagedKeys: %v", err)
	}
	found := false
	for _, k := range keys {
		if k == "mcp" {
			found = true
		}
	}
	if !found {
		t.Errorf("sidecar should claim mcp after overwrite, got %v", keys)
	}
}

// TestUserWinsSmoke_NoOverrideReturnsConflictError verifies the safety path:
// with a user-edited unmanaged key and no override, Apply refuses to write
// and returns a ConflictError wrapping ErrMergeConflict. File and sidecar
// are left unchanged.
func TestUserWinsSmoke_NoOverrideReturnsConflictError(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	if err := config.WriteJSON(config.UserConfigPath(home), domain.DefaultUserConfig()); err != nil {
		t.Fatalf("write user config: %v", err)
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), domain.DefaultProjectConfig()); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	target := filepath.Join(project, "opencode.json")
	userConfig := map[string]any{
		"mcp": map[string]any{
			"user-managed-server": map[string]any{
				"type": "remote",
				"url":  "https://user-only.example.com/mcp",
			},
		},
	}
	writeJSON(t, target, userConfig)
	originalBytes, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("snapshot original: %v", err)
	}

	installer := mcp.New(map[string]domain.MCPServerDef{
		"context7": {Type: "remote", URL: "https://mcp.context7.com/mcp", Enabled: true},
	})
	// No SetApplyPolicy — zero-value means no overrides.

	actions, err := installer.Plan(opencode.New(), home, project)
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	var applyErr error
	for _, a := range actions {
		if applyErr = installer.Apply(a); applyErr != nil {
			break
		}
	}
	if applyErr == nil {
		t.Fatalf("expected conflict error, got nil")
	}
	if !errors.Is(applyErr, domain.ErrMergeConflict) {
		t.Errorf("expected error wrapping ErrMergeConflict, got %v", applyErr)
	}
	var cerr *domain.ConflictError
	if !errors.As(applyErr, &cerr) {
		t.Errorf("expected *domain.ConflictError, got %T: %v", applyErr, applyErr)
	} else if cerr.TargetPath != target {
		t.Errorf("ConflictError.TargetPath = %q, want %q", cerr.TargetPath, target)
	}

	// File must be untouched.
	afterBytes, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("re-read target: %v", err)
	}
	if string(afterBytes) != string(originalBytes) {
		t.Errorf("file should be unchanged on conflict, diff:\n---before---\n%s\n---after---\n%s",
			string(originalBytes), string(afterBytes))
	}

	// Sidecar must not have been created with this key.
	keys, err := managed.ReadManagedKeys(project, "opencode.json")
	if err != nil {
		t.Fatalf("ReadManagedKeys: %v", err)
	}
	for _, k := range keys {
		if k == "mcp" {
			t.Errorf("sidecar must not claim mcp after failed apply, got %v", keys)
		}
	}
}

func writeJSON(t *testing.T, path string, data map[string]any) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return out
}
