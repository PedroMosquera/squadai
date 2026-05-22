package memory

import (
	"sort"
)

// SearchResult is one result from a memory search.
type SearchResult struct {
	Path      string  `json:"path"`
	FirstLine string  `json:"first_line"`
	Score     float64 `json:"score"`
}

// Search queries the index for notes matching the query string.
// Returns results sorted by score descending (max 10 results).
func Search(projectDir, query string) ([]SearchResult, error) {
	idx, err := LoadIndex(projectDir)
	if err != nil {
		return nil, err
	}

	if len(idx.Entries) == 0 {
		return nil, nil
	}

	queryWords := tokenize(query)
	if len(queryWords) == 0 {
		return nil, nil
	}

	var results []SearchResult

	for _, entry := range idx.Entries {
		// Build word set for this entry.
		entryWords := make(map[string]struct{}, len(entry.Words))
		for _, w := range entry.Words {
			entryWords[w] = struct{}{}
		}

		matched := 0
		for _, qw := range queryWords {
			if _, ok := entryWords[qw]; ok {
				matched++
			}
		}

		if matched == 0 {
			continue
		}

		score := float64(matched) / float64(len(queryWords))
		results = append(results, SearchResult{
			Path:      entry.Path,
			FirstLine: entry.FirstLine,
			Score:     score,
		})
	}

	// Sort by score descending.
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Path < results[j].Path
	})

	// Cap at 10 results.
	if len(results) > 10 {
		results = results[:10]
	}

	return results, nil
}
