package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PedroMosquera/squadai/internal/fileutil"
)

// Index is the in-memory representation of the search index.
type Index struct {
	Entries []IndexEntry `json:"entries"`
	Built   string       `json:"built"` // RFC3339
}

// IndexEntry is one entry in the search index.
type IndexEntry struct {
	Path      string   `json:"path"`       // relative to projectDir
	FirstLine string   `json:"first_line"` // first non-frontmatter, non-empty line
	Words     []string `json:"words"`      // lowercased words for search
}

var nonAlpha = regexp.MustCompile(`[^a-z]+`)

// tokenize splits text into lowercased words, deduplicating.
func tokenize(text string) []string {
	lower := strings.ToLower(text)
	parts := nonAlpha.Split(lower, -1)
	seen := make(map[string]struct{})
	var words []string
	for _, p := range parts {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			words = append(words, p)
		}
	}
	return words
}

// stripFrontmatter removes the leading YAML frontmatter block (between --- fences)
// and returns the remaining body text.
func stripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}
	// Find end of frontmatter
	rest := content[3:]
	// skip the newline after opening ---
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return content
	}
	body := rest[idx+4:] // skip "\n---"
	// skip optional trailing newline after closing ---
	body = strings.TrimPrefix(body, "\n")
	body = strings.TrimPrefix(body, "\r\n")
	return body
}

// firstNonEmptyLine returns the first non-empty, non-whitespace line from text.
func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// indexPath is where the index is stored relative to projectDir.
const indexPath = ".squadai/memory-index.json"

// Reindex rebuilds docs/memory/ search index and writes to .squadai/memory-index.json.
// Returns number of entries indexed.
func Reindex(projectDir string) (int, error) {
	memoryDir := filepath.Join(projectDir, "docs", "memory")

	var entries []IndexEntry

	err := filepath.Walk(memoryDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable
		}

		// Skip _inbox/ directory
		if info.IsDir() {
			if info.Name() == "_inbox" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process .md files
		if !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}

		// Skip README.md files
		if info.Name() == "README.md" {
			return nil
		}

		// Skip empty files
		if info.Size() == 0 {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		content := string(data)
		body := stripFrontmatter(content)
		firstLine := firstNonEmptyLine(body)
		words := tokenize(content)

		rel, relErr := filepath.Rel(projectDir, path)
		if relErr != nil {
			rel = path
		}

		entries = append(entries, IndexEntry{
			Path:      rel,
			FirstLine: firstLine,
			Words:     words,
		})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return 0, err
	}

	idx := Index{
		Entries: entries,
		Built:   time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return 0, err
	}

	destPath := filepath.Join(projectDir, indexPath)
	if _, err := fileutil.WriteAtomic(destPath, data, 0644); err != nil {
		return 0, err
	}

	return len(entries), nil
}

// LoadIndex reads .squadai/memory-index.json. Returns empty Index if absent.
func LoadIndex(projectDir string) (Index, error) {
	path := filepath.Join(projectDir, indexPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Index{}, nil
		}
		return Index{}, err
	}

	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return Index{}, err
	}
	return idx, nil
}
