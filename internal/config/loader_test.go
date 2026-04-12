package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// ─── Loader tests ────────────────────────────────────────────────────────────

func TestLoadUser_FileNotFound_ReturnsErrConfigNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadUser(dir)
	if err != domain.ErrConfigNotFound {
		t.Fatalf("expected ErrConfigNotFound, got %v", err)
	}
}

func TestLoadUser_ValidJSON_ParsesCorrectly(t *testing.T) {
	dir := t.TempDir()
	cfg := domain.DefaultUserConfig()
	writeCfg(t, UserConfigPath(dir), cfg)

	loaded, err := LoadUser(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.Version != 1 {
		t.Errorf("Version = %d, want 1", loaded.Version)
	}
	if loaded.Mode != domain.ModeHybrid {
		t.Errorf("Mode = %q, want %q", loaded.Mode, domain.ModeHybrid)
	}
	if !loaded.Adapters[string(domain.AgentOpenCode)].Enabled {
		t.Error("opencode adapter should be enabled by default")
	}
}

func TestLoadUser_InvalidJSON_ReturnsValidationError(t *testing.T) {
	dir := t.TempDir()
	path := UserConfigPath(dir)
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte(`{invalid`), 0644)

	_, err := LoadUser(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	var ve *domain.ValidationError
	if !isValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestLoadProject_FileNotFound_ReturnsErrConfigNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadProject(dir)
	if err != domain.ErrConfigNotFound {
		t.Fatalf("expected ErrConfigNotFound, got %v", err)
	}
}

func TestLoadProject_ValidJSON_ParsesCorrectly(t *testing.T) {
	dir := t.TempDir()
	cfg := domain.DefaultProjectConfig()
	writeCfg(t, ProjectConfigPath(dir), cfg)

	loaded, err := LoadProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.Version != 1 {
		t.Errorf("Version = %d, want 1", loaded.Version)
	}
	if loaded.Copilot.InstructionsTemplate != "standard" {
		t.Errorf("InstructionsTemplate = %q, want %q", loaded.Copilot.InstructionsTemplate, "standard")
	}
}

func TestLoadPolicy_FileNotFound_ReturnsErrConfigNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadPolicy(dir)
	if err != domain.ErrConfigNotFound {
		t.Fatalf("expected ErrConfigNotFound, got %v", err)
	}
}

func TestLoadPolicy_ValidJSON_ParsesCorrectly(t *testing.T) {
	dir := t.TempDir()
	cfg := domain.DefaultPolicyConfig()
	writeCfg(t, PolicyConfigPath(dir), cfg)

	loaded, err := LoadPolicy(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.Version != 1 {
		t.Errorf("Version = %d, want 1", loaded.Version)
	}
	if loaded.Mode != domain.ModeTeam {
		t.Errorf("Mode = %q, want %q", loaded.Mode, domain.ModeTeam)
	}
	if len(loaded.Locked) != 3 {
		t.Errorf("Locked count = %d, want 3", len(loaded.Locked))
	}
}

// ─── WriteJSON tests ─────────────────────────────────────────────────────────

func TestWriteJSON_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "file.json")

	err := WriteJSON(path, map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist at %s", path)
	}
}

func TestWriteJSON_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	original := domain.DefaultUserConfig()
	if err := WriteJSON(path, original); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var loaded domain.UserConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if loaded.Version != original.Version {
		t.Errorf("Version = %d, want %d", loaded.Version, original.Version)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func writeCfg(t *testing.T, path string, v interface{}) {
	t.Helper()
	if err := WriteJSON(path, v); err != nil {
		t.Fatalf("writeCfg: %v", err)
	}
}

func isValidationError(err error, target **domain.ValidationError) bool {
	var ve *domain.ValidationError
	if ok := isErrorType(err, &ve); ok {
		*target = ve
		return true
	}
	return false
}

// isErrorType uses type assertion since errors.As requires matching.
func isErrorType(err error, target interface{}) bool {
	switch t := target.(type) {
	case **domain.ValidationError:
		var ve *domain.ValidationError
		switch e := err.(type) {
		case *domain.ValidationError:
			*t = e
			return true
		default:
			_ = ve
			return false
		}
	}
	return false
}
