// Package modelcatalog is the single source of truth for model metadata:
// pricing, context windows, tokenizer encodings, and per-adapter tier
// defaults. The embedded catalog (internal/assets/models/models.json) can be
// layered with user (~/.squadai/models.json) and project (.squadai/models.json)
// overrides. Lookups are exact-match first, then longest-prefix, which avoids
// the classic HasPrefix trap where e.g. "claude-sonnet-4-6" would match a
// legacy "claude-sonnet-4" row.
package modelcatalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/PedroMosquera/squadai/internal/assets"
)

// EmbeddedPath is the asset path of the embedded catalog.
const EmbeddedPath = "models/models.json"

// Well-known source labels for catalog entries.
const (
	SourceEmbedded = "embedded"
	SourceUser     = "user"
	SourceProject  = "project"
)

// Model holds the metadata for a single model row.
type Model struct {
	Provider      string   `json:"provider"`
	Display       string   `json:"display,omitempty"`
	InputPerMTok  float64  `json:"input_per_mtok"`
	OutputPerMTok float64  `json:"output_per_mtok"`
	ContextWindow int      `json:"context_window,omitempty"`
	MaxOutput     int      `json:"max_output,omitempty"`
	Encoding      string   `json:"encoding,omitempty"`
	Aliases       []string `json:"aliases,omitempty"`
	Legacy        bool     `json:"legacy,omitempty"`
}

// EncodingPrefix maps a model-name prefix to a tokenizer encoding name.
type EncodingPrefix struct {
	Prefix   string `json:"prefix"`
	Encoding string `json:"encoding"`
}

// AdapterEntry holds per-adapter tier defaults and prompt hints.
type AdapterEntry struct {
	// Tiers maps tier names (premium, standard, cheap) to concrete model IDs.
	Tiers map[string]string `json:"tiers,omitempty"`
	// Hints maps tier names to prompt-hint recommendation strings for adapters
	// that use UI-based model selection.
	Hints map[string]string `json:"hints,omitempty"`
}

// File is the on-disk (and embedded) catalog document. Override files use the
// same schema and may be partial: any model, prefix, or adapter entry present
// replaces the corresponding entry from lower layers.
type File struct {
	SchemaVersion    int                     `json:"schema_version"`
	Updated          string                  `json:"updated,omitempty"`
	Models           map[string]Model        `json:"models,omitempty"`
	EncodingPrefixes []EncodingPrefix        `json:"encoding_prefixes,omitempty"`
	Adapters         map[string]AdapterEntry `json:"adapters,omitempty"`
}

// SupportedSchemaVersion is the only schema version this build understands.
const SupportedSchemaVersion = 1

// Parse decodes and validates a catalog document.
func Parse(data []byte) (*File, error) {
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse models catalog: %w", err)
	}
	if err := Validate(&f); err != nil {
		return nil, err
	}
	return &f, nil
}

// Validate checks a catalog document for structural issues.
func Validate(f *File) error {
	if f.SchemaVersion != SupportedSchemaVersion {
		return fmt.Errorf("models catalog: unsupported schema_version %d (expected %d)",
			f.SchemaVersion, SupportedSchemaVersion)
	}
	if f.Updated != "" {
		if _, err := time.Parse("2006-01-02", f.Updated); err != nil {
			return fmt.Errorf("models catalog: invalid updated date %q (expected YYYY-MM-DD)", f.Updated)
		}
	}
	for id, m := range f.Models {
		if id == "" {
			return fmt.Errorf("models catalog: empty model id")
		}
		if m.InputPerMTok < 0 || m.OutputPerMTok < 0 {
			return fmt.Errorf("models catalog: model %q has negative pricing", id)
		}
		if m.ContextWindow < 0 || m.MaxOutput < 0 {
			return fmt.Errorf("models catalog: model %q has negative context/output window", id)
		}
	}
	for _, ep := range f.EncodingPrefixes {
		if ep.Prefix == "" || ep.Encoding == "" {
			return fmt.Errorf("models catalog: encoding_prefixes entries need both prefix and encoding")
		}
	}
	for name, a := range f.Adapters {
		for tier, id := range a.Tiers {
			if id == "" {
				return fmt.Errorf("models catalog: adapter %q tier %q has empty model id", name, tier)
			}
		}
	}
	return nil
}

// Catalog is the effective, merged model catalog with per-entry source
// attribution.
type Catalog struct {
	schemaVersion    int
	updated          time.Time
	updatedRaw       string
	models           map[string]Model
	sources          map[string]string // model id → source label
	aliases          map[string]string // alias → canonical model id
	encodingPrefixes []EncodingPrefix
	adapters         map[string]AdapterEntry
	adapterSources   map[string]string
}

// newCatalog returns an empty catalog ready for layering.
func newCatalog() *Catalog {
	return &Catalog{
		models:         make(map[string]Model),
		sources:        make(map[string]string),
		aliases:        make(map[string]string),
		adapters:       make(map[string]AdapterEntry),
		adapterSources: make(map[string]string),
	}
}

// merge layers f over the catalog, attributing new/replaced entries to source.
func (c *Catalog) merge(f *File, source string) {
	c.schemaVersion = f.SchemaVersion
	if f.Updated != "" {
		if t, err := time.Parse("2006-01-02", f.Updated); err == nil {
			c.updated = t
			c.updatedRaw = f.Updated
		}
	}
	for id, m := range f.Models {
		if prev, ok := c.models[id]; ok {
			// Drop stale alias index entries from the replaced row.
			for _, a := range prev.Aliases {
				delete(c.aliases, a)
			}
		}
		c.models[id] = m
		c.sources[id] = source
		for _, a := range m.Aliases {
			c.aliases[a] = id
		}
	}
	if len(f.EncodingPrefixes) > 0 {
		// Override by prefix key; new prefixes are appended.
		index := make(map[string]int, len(c.encodingPrefixes))
		for i, ep := range c.encodingPrefixes {
			index[ep.Prefix] = i
		}
		for _, ep := range f.EncodingPrefixes {
			if i, ok := index[ep.Prefix]; ok {
				c.encodingPrefixes[i] = ep
			} else {
				index[ep.Prefix] = len(c.encodingPrefixes)
				c.encodingPrefixes = append(c.encodingPrefixes, ep)
			}
		}
	}
	for name, a := range f.Adapters {
		// Deep-merge per tier/hint key so partial overrides (e.g. remapping
		// only the standard tier) keep the other tiers from lower layers.
		merged := c.adapters[name]
		if merged.Tiers == nil {
			merged.Tiers = make(map[string]string)
		}
		if merged.Hints == nil {
			merged.Hints = make(map[string]string)
		}
		for tier, id := range a.Tiers {
			merged.Tiers[tier] = id
		}
		for tier, hint := range a.Hints {
			merged.Hints[tier] = hint
		}
		c.adapters[name] = merged
		c.adapterSources[name] = source
	}
}

// FromFile builds a catalog from a single parsed document.
func FromFile(f *File, source string) *Catalog {
	c := newCatalog()
	c.merge(f, source)
	return c
}

// loadEmbedded parses the compiled-in catalog. It panics on error because a
// broken embedded catalog is a build defect, caught by tests.
func loadEmbedded() *File {
	data, err := assets.FS.ReadFile(EmbeddedPath)
	if err != nil {
		panic("modelcatalog: embedded catalog missing: " + err.Error())
	}
	f, err := Parse(data)
	if err != nil {
		panic("modelcatalog: embedded catalog invalid: " + err.Error())
	}
	return f
}

// UserOverridePath returns the path of the user-level override file.
func UserOverridePath(homeDir string) string {
	return filepath.Join(homeDir, ".squadai", "models.json")
}

// ProjectOverridePath returns the path of the project-level override file.
func ProjectOverridePath(projectDir string) string {
	return filepath.Join(projectDir, ".squadai", "models.json")
}

// Load returns the effective catalog: the embedded defaults layered with the
// user override (~/.squadai/models.json) and then the project override
// (.squadai/models.json). Missing override files are fine; invalid override
// files return an error. Empty homeDir/projectDir skip that layer.
func Load(homeDir, projectDir string) (*Catalog, error) {
	c := newCatalog()
	c.merge(loadEmbedded(), SourceEmbedded)

	if homeDir != "" {
		if err := c.mergePath(UserOverridePath(homeDir), SourceUser); err != nil {
			return nil, err
		}
	}
	if projectDir != "" {
		if err := c.mergePath(ProjectOverridePath(projectDir), SourceProject); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// mergePath layers the override file at path, if it exists.
func (c *Catalog) mergePath(path, source string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	f, err := Parse(data)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	c.merge(f, source)
	return nil
}

// Updated returns the effective catalog data date (zero when unknown).
func (c *Catalog) Updated() time.Time { return c.updated }

// UpdatedString returns the raw effective updated date string.
func (c *Catalog) UpdatedString() string { return c.updatedRaw }

// SchemaVersion returns the effective schema version.
func (c *Catalog) SchemaVersion() int { return c.schemaVersion }

// Source returns the layer that provided the given model row
// (embedded, user, or project). Empty when the model is unknown.
func (c *Catalog) Source(modelID string) string { return c.sources[modelID] }

// ModelIDs returns all model IDs in the effective catalog, sorted.
func (c *Catalog) ModelIDs() []string {
	ids := make([]string, 0, len(c.models))
	for id := range c.models {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Model returns the model row for the exact catalog ID.
func (c *Catalog) Model(id string) (Model, bool) {
	m, ok := c.models[id]
	return m, ok
}

// AdapterIDs returns all adapter names in the effective catalog, sorted.
func (c *Catalog) AdapterIDs() []string {
	ids := make([]string, 0, len(c.adapters))
	for id := range c.adapters {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Adapter returns the adapter entry for the given adapter name.
func (c *Catalog) Adapter(name string) (AdapterEntry, bool) {
	a, ok := c.adapters[name]
	return a, ok
}
