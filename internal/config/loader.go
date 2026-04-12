package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

const (
	// UserConfigDir is the directory under $HOME for user-level config.
	UserConfigDir = ".agent-manager"

	// UserConfigFile is the filename for user-level config.
	UserConfigFile = "config.json"

	// ProjectConfigDir is the directory in a project root for project/policy config.
	ProjectConfigDir = ".agent-manager"

	// ProjectConfigFile is the filename for project-level config.
	ProjectConfigFile = "project.json"

	// PolicyConfigFile is the filename for team policy.
	PolicyConfigFile = "policy.json"
)

// UserConfigPath returns the full path to the user config file.
func UserConfigPath(homeDir string) string {
	return filepath.Join(homeDir, UserConfigDir, UserConfigFile)
}

// ProjectConfigPath returns the full path to the project config file.
func ProjectConfigPath(projectDir string) string {
	return filepath.Join(projectDir, ProjectConfigDir, ProjectConfigFile)
}

// PolicyConfigPath returns the full path to the policy config file.
func PolicyConfigPath(projectDir string) string {
	return filepath.Join(projectDir, ProjectConfigDir, PolicyConfigFile)
}

// LoadUser reads and parses the user config from ~/.agent-manager/config.json.
// Returns domain.ErrConfigNotFound if the file does not exist.
func LoadUser(homeDir string) (*domain.UserConfig, error) {
	path := UserConfigPath(homeDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, domain.ErrConfigNotFound
		}
		return nil, err
	}

	var cfg domain.UserConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, &domain.ValidationError{
			Source: path,
			Issues: []string{err.Error()},
		}
	}

	return &cfg, nil
}

// LoadProject reads and parses .agent-manager/project.json.
// Returns domain.ErrConfigNotFound if the file does not exist.
func LoadProject(projectDir string) (*domain.ProjectConfig, error) {
	path := ProjectConfigPath(projectDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, domain.ErrConfigNotFound
		}
		return nil, err
	}

	var cfg domain.ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, &domain.ValidationError{
			Source: path,
			Issues: []string{err.Error()},
		}
	}

	return &cfg, nil
}

// LoadPolicy reads and parses .agent-manager/policy.json.
// Returns domain.ErrConfigNotFound if the file does not exist.
func LoadPolicy(projectDir string) (*domain.PolicyConfig, error) {
	path := PolicyConfigPath(projectDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, domain.ErrConfigNotFound
		}
		return nil, err
	}

	var cfg domain.PolicyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, &domain.ValidationError{
			Source: path,
			Issues: []string{err.Error()},
		}
	}

	return &cfg, nil
}

// WriteJSON writes a value as indented JSON to path, creating parent dirs.
func WriteJSON(path string, v interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0644)
}
