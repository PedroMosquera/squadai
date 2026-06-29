package memory

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PedroMosquera/squadai/internal/fileutil"
)

// tfidfPath is where the TF-IDF index is stored relative to projectDir.
const tfidfPath = ".squadai/memory-tfidf.json"

// tfidfIndex holds precomputed TF-IDF vectors for all notes.
type tfidfIndex struct {
	entries   []tfidfEntry
	df        map[string]int // document frequency per term
	totalDocs int
	idf       map[string]float64
}

type tfidfEntry struct {
	path      string
	firstLine string
	modTime   time.Time
	tf        map[string]float64 // term frequency for this doc
	vector    map[string]float64 // TF-IDF vector
	norm      float64            // L2 norm
}

// buildTFIDFIndex walks docs/memory/ and computes TF-IDF vectors for each note.
// It mirrors Reindex's walk rules: skip _inbox/, skip README.md, skip empty
// files, and process only .md files.
func buildTFIDFIndex(projectDir string) (*tfidfIndex, error) {
	memoryDir := filepath.Join(projectDir, "docs", "memory")

	idx := &tfidfIndex{
		df:  make(map[string]int),
		idf: make(map[string]float64),
	}

	err := filepath.Walk(memoryDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		if info.IsDir() {
			if info.Name() == "_inbox" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		if info.Name() == "README.md" {
			return nil
		}
		if info.Size() == 0 {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		body := stripFrontmatter(string(data))
		firstLine := firstNonEmptyLine(body)
		words := tokenize(body)

		rel, relErr := filepath.Rel(projectDir, path)
		if relErr != nil {
			rel = path
		}

		tf := make(map[string]float64, len(words))
		for _, w := range words {
			tf[w] = 1.0
		}

		idx.entries = append(idx.entries, tfidfEntry{
			path:      rel,
			firstLine: firstLine,
			modTime:   info.ModTime(),
			tf:        tf,
		})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	idx.totalDocs = len(idx.entries)

	// Document frequency: number of docs containing each term.
	for _, e := range idx.entries {
		for term := range e.tf {
			idx.df[term]++
		}
	}

	// IDF: smoothed log((N+1)/(df+1)) + 1. This keeps small corpora useful
	// and avoids zero/negative weights for common terms.
	for term, df := range idx.df {
		idx.idf[term] = idfWeight(idx.totalDocs, df)
	}

	// TF-IDF vectors and L2 norms.
	for i := range idx.entries {
		e := &idx.entries[i]
		e.vector = make(map[string]float64, len(e.tf))
		var sumSq float64
		for term, tfVal := range e.tf {
			w := tfVal * idx.idf[term]
			e.vector[term] = w
			sumSq += w * w
		}
		e.norm = math.Sqrt(sumSq)
	}

	return idx, nil
}

// searchTFIDF ranks notes against the query using cosine similarity over
// TF-IDF vectors, applying a freshness decay to the score.
func searchTFIDF(idx *tfidfIndex, query string) []SearchResult {
	if idx == nil || len(idx.entries) == 0 {
		return nil
	}

	queryWords := tokenize(query)
	if len(queryWords) == 0 {
		return nil
	}

	// Build the query vector, treating the query as a pseudo-document.
	queryVec := make(map[string]float64, len(queryWords))
	var queryNormSq float64
	for _, w := range queryWords {
		if _, ok := queryVec[w]; ok {
			continue
		}
		idf, ok := idx.idf[w]
		if !ok {
			idf = idfWeight(idx.totalDocs, 0)
		}
		weight := idf
		queryVec[w] = weight
		queryNormSq += weight * weight
	}
	queryNorm := math.Sqrt(queryNormSq)
	if queryNorm == 0 {
		return nil
	}

	now := time.Now()
	var results []SearchResult
	for _, e := range idx.entries {
		if e.norm == 0 {
			continue
		}
		// Cosine similarity = dot(q, d) / (|q| * |d|).
		var dot float64
		// Iterate over the smaller vector for efficiency.
		small, large := queryVec, e.vector
		if len(large) < len(small) {
			small, large = large, small
		}
		for term, w := range small {
			if dw, ok := large[term]; ok {
				dot += w * dw
			}
		}
		score := dot / (queryNorm * e.norm)
		if score <= 0 {
			continue
		}
		// Freshness decay: scale score by recency.
		score *= 0.7 + 0.3*freshnessFactor(e.modTime, now)
		results = append(results, SearchResult{
			Path:      e.path,
			FirstLine: e.firstLine,
			Score:     score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Path < results[j].Path
	})

	if len(results) > 10 {
		results = results[:10]
	}
	return results
}

// freshnessFactor returns a recency weight for a note based on its mod time.
// Notes younger than 30 days score 1.0, decaying linearly to 0.5 at 180 days,
// then flat at 0.5 beyond that.
func freshnessFactor(modTime, now time.Time) float64 {
	age := now.Sub(modTime)
	if age < 30*24*time.Hour {
		return 1.0
	}
	if age > 180*24*time.Hour {
		return 0.5
	}
	days := age.Hours() / 24
	return 1.0 - (days-30)/300.0
}

// tfidfFile is the diff-friendly JSON serialization of the TF-IDF index.
type tfidfFile struct {
	Built   string           `json:"built"`
	Entries []tfidfFileEntry `json:"entries"`
}

type tfidfFileEntry struct {
	Path      string             `json:"path"`
	FirstLine string             `json:"first_line"`
	Vector    map[string]float64 `json:"vector"`
	Norm      float64            `json:"norm"`
	ModTime   string             `json:"mod_time"`
}

// ReindexTFIDF rebuilds the TF-IDF index and writes it to
// .squadai/memory-tfidf.json. Returns the number of entries indexed.
func ReindexTFIDF(projectDir string) (int, error) {
	idx, err := buildTFIDFIndex(projectDir)
	if err != nil {
		return 0, err
	}

	file := tfidfFile{
		Built:   time.Now().UTC().Format(time.RFC3339),
		Entries: make([]tfidfFileEntry, 0, len(idx.entries)),
	}
	for _, e := range idx.entries {
		file.Entries = append(file.Entries, tfidfFileEntry{
			Path:      e.path,
			FirstLine: e.firstLine,
			Vector:    e.vector,
			Norm:      e.norm,
			ModTime:   e.modTime.UTC().Format(time.RFC3339),
		})
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return 0, err
	}

	destPath := filepath.Join(projectDir, tfidfPath)
	if _, err := fileutil.WriteAtomic(destPath, data, 0644); err != nil {
		return 0, err
	}

	return len(idx.entries), nil
}

// loadTFIDFIndex reads .squadai/memory-tfidf.json. Returns nil, nil if the
// file does not exist.
func loadTFIDFIndex(projectDir string) (*tfidfIndex, error) {
	path := filepath.Join(projectDir, tfidfPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var file tfidfFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}

	idx := &tfidfIndex{
		df:  make(map[string]int),
		idf: make(map[string]float64),
	}
	for _, fe := range file.Entries {
		modTime, perr := time.Parse(time.RFC3339, fe.ModTime)
		if perr != nil {
			modTime = time.Time{}
		}
		idx.entries = append(idx.entries, tfidfEntry{
			path:      fe.Path,
			firstLine: fe.FirstLine,
			modTime:   modTime,
			vector:    fe.Vector,
			norm:      fe.Norm,
		})
	}
	idx.totalDocs = len(idx.entries)

	// Reconstruct df and idf from the loaded vectors so that query vectors
	// can be built consistently with the corpus.
	for _, e := range idx.entries {
		for term := range e.vector {
			idx.df[term]++
		}
	}
	for term, df := range idx.df {
		idx.idf[term] = idfWeight(idx.totalDocs, df)
	}

	return idx, nil
}

func idfWeight(totalDocs, df int) float64 {
	if totalDocs <= 0 {
		return 0
	}
	return math.Log((float64(totalDocs)+1)/(float64(df)+1)) + 1
}

// SearchTFIDF is the public TF-IDF search API. It loads the index (building it
// on demand if missing or unreadable) and returns ranked results.
func SearchTFIDF(projectDir, query string) ([]SearchResult, error) {
	idx, err := loadTFIDFIndex(projectDir)
	if err != nil || idx == nil {
		idx, err = buildTFIDFIndex(projectDir)
		if err != nil {
			return nil, err
		}
	}
	return searchTFIDF(idx, query), nil
}
