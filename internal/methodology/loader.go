// Package methodology loads user-defined methodology specs from
// .squadai/methodologies/<name>.json and converts them into the
// domain.TeamRole map format that the rest of the system expects.
package methodology

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// Spec describes a methodology as a JSON-loadable file.
type Spec struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Roles       map[string]RoleSpec `json:"roles"`
}

// RoleSpec is one role in a methodology spec.
type RoleSpec struct {
	Description string   `json:"description"`
	Mode        string   `json:"mode"` // "subagent" or "inline"
	SkillRef    string   `json:"skill_ref,omitempty"`
	Model       string   `json:"model,omitempty"` // tier: premium/standard/cheap
	DelegatesTo []string `json:"delegates_to,omitempty"`
}

// methodologiesDir is the relative location of user methodology specs.
const methodologiesDir = ".squadai/methodologies"

// Load reads a methodology spec from .squadai/methodologies/<name>.json.
// Returns nil, nil if the file doesn't exist.
func Load(projectDir string, name string) (*Spec, error) {
	path := filepath.Join(projectDir, methodologiesDir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read methodology %q: %w", name, err)
	}
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse methodology %q: %w", name, err)
	}
	return &spec, nil
}

// LoadAll reads all methodology specs from .squadai/methodologies/.
// Returns empty slice if directory doesn't exist.
func LoadAll(projectDir string) ([]*Spec, error) {
	dir := filepath.Join(projectDir, methodologiesDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Spec{}, nil
		}
		return nil, fmt.Errorf("read methodologies dir: %w", err)
	}

	specs := make([]*Spec, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("read methodology %q: %w", entry.Name(), readErr)
		}
		var spec Spec
		if err := json.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("parse methodology %q: %w", entry.Name(), err)
		}
		specs = append(specs, &spec)
	}

	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	return specs, nil
}

// ToTeamRoles converts a Spec into the domain.TeamRole map format
// that the rest of the system expects.
func ToTeamRoles(spec *Spec) map[string]domain.TeamRole {
	if spec == nil {
		return nil
	}
	roles := make(map[string]domain.TeamRole, len(spec.Roles))
	for name, r := range spec.Roles {
		roles[name] = domain.TeamRole{
			Description: r.Description,
			Mode:        r.Mode,
			SkillRef:    r.SkillRef,
			DelegatesTo: r.DelegatesTo,
			Model:       r.Model,
		}
	}
	return roles
}

// IsBuiltIn reports whether a methodology name is one of the built-in
// presets (tdd, sdd, conventional).
func IsBuiltIn(name string) bool {
	switch domain.Methodology(name) {
	case domain.MethodologyTDD, domain.MethodologySDD, domain.MethodologyConventional:
		return true
	}
	return false
}

// ListAll returns all available methodologies: built-ins plus any
// user-defined ones found in .squadai/methodologies/.
func ListAll(projectDir string) []string {
	names := map[string]struct{}{
		string(domain.MethodologyTDD):          {},
		string(domain.MethodologySDD):          {},
		string(domain.MethodologyConventional): {},
	}

	dir := filepath.Join(projectDir, methodologiesDir)
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			names[strings.TrimSuffix(entry.Name(), ".json")] = struct{}{}
		}
	}

	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
