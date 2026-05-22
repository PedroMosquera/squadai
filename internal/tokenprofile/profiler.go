package tokenprofile

import "os"

// ApproxTokens estimates the number of tokens in content using a 4 chars/token
// heuristic (ceiling division). Returns 0 for empty content.
func ApproxTokens(content []byte) int {
	n := len(content)
	if n == 0 {
		return 0
	}
	return (n + 3) / 4
}

// Entry holds per-file token data.
type Entry struct {
	Path     string
	Category string
	Bytes    int
	Tokens   int
}

// CategorySummary aggregates token data for a category.
type CategorySummary struct {
	Files  int
	Bytes  int
	Tokens int
}

// Report is the full output of a ScanPaths call.
type Report struct {
	Entries     []Entry
	ByCategory  map[string]CategorySummary
	TotalBytes  int
	TotalTokens int
	Missing     int // count of paths that did not exist on disk
}

// ScanPaths reads each file in paths (map[filepath]category) from disk,
// estimates tokens, and returns a Report.
// Missing files increment Report.Missing but are not an error.
func ScanPaths(paths map[string]string) (*Report, error) {
	r := &Report{
		ByCategory: make(map[string]CategorySummary),
	}
	for path, category := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				r.Missing++
				continue
			}
			return nil, err
		}
		tokens := ApproxTokens(data)
		r.Entries = append(r.Entries, Entry{
			Path:     path,
			Category: category,
			Bytes:    len(data),
			Tokens:   tokens,
		})
		sum := r.ByCategory[category]
		sum.Files++
		sum.Bytes += len(data)
		sum.Tokens += tokens
		r.ByCategory[category] = sum
		r.TotalBytes += len(data)
		r.TotalTokens += tokens
	}
	return r, nil
}
