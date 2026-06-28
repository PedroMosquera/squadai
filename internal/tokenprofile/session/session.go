package session

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PedroMosquera/squadai/internal/tokenprofile/pricing"
)

// Usage holds aggregated token usage and cost for a single model or
// for the grand total across all models.
type Usage struct {
	Model         string  `json:"model"`
	InputTokens   int     `json:"input_tokens"`
	OutputTokens  int     `json:"output_tokens"`
	TotalTokens   int     `json:"total_tokens"`
	EstimatedCost float64 `json:"estimated_cost_usd"`
	SessionCount  int     `json:"session_count"`
}

// Aggregation is the result of Aggregate, broken down per model plus a
// grand total.
type Aggregation struct {
	ByModel map[string]Usage `json:"by_model"`
	Total   Usage            `json:"total"`
	Period  string           `json:"period"`
}

// AggregateOptions controls filtering of session files.
type AggregateOptions struct {
	Since      time.Duration
	ProjectDir string
}

// sessionDirs are the locations scanned for session files, relative to
// the user's home directory.
var sessionDirs = []string{
	".local/share/opencode/sessions",
	".config/opencode/sessions",
	".pi/agent/sessions",
}

// Aggregate scans known session directories under homeDir, parses each
// session file defensively, and aggregates token usage and cost per
// model. Missing directories and unparseable files are skipped
// gracefully. It only errors when homeDir cannot be resolved.
func Aggregate(homeDir string, opts AggregateOptions) (*Aggregation, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
	}

	agg := &Aggregation{ByModel: map[string]Usage{}}
	if opts.Since > 0 {
		agg.Period = opts.Since.String()
	} else {
		agg.Period = "all"
	}

	var cutoff time.Time
	if opts.Since > 0 {
		cutoff = time.Now().Add(-opts.Since)
	}

	for _, rel := range sessionDirs {
		walkSessions(filepath.Join(homeDir, rel), cutoff, opts.ProjectDir, agg)
	}

	for model, u := range agg.ByModel {
		u.Model = model
		u.TotalTokens = u.InputTokens + u.OutputTokens
		u.EstimatedCost = pricing.EstimateCost(model, u.InputTokens, u.OutputTokens)
		agg.ByModel[model] = u
		agg.Total.InputTokens += u.InputTokens
		agg.Total.OutputTokens += u.OutputTokens
		agg.Total.TotalTokens += u.TotalTokens
		agg.Total.EstimatedCost += u.EstimatedCost
		agg.Total.SessionCount += u.SessionCount
	}
	agg.Total.Model = "total"
	return agg, nil
}

// walkSessions walks dir recursively, accumulating any parseable
// session files into agg. Errors are swallowed so that a missing or
// unreadable directory never aborts aggregation.
func walkSessions(dir string, cutoff time.Time, projectDir string, agg *Aggregation) {
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".json") {
			return nil
		}
		if !cutoff.IsZero() {
			if info, err := d.Info(); err == nil && info.ModTime().Before(cutoff) {
				return nil
			}
		}
		if projectDir != "" && !strings.Contains(path, projectDir) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		model, input, output, ok := parseSession(data)
		if !ok || (input == 0 && output == 0) {
			return nil
		}
		if model == "" {
			model = "unknown"
		}
		u := agg.ByModel[model]
		u.Model = model
		u.InputTokens += input
		u.OutputTokens += output
		u.SessionCount++
		agg.ByModel[model] = u
		return nil
	})
}

// parseSession defensively extracts a model name and token counts from
// a session JSON blob. It tolerates a variety of field names and
// nesting shapes. ok is false when the blob is not valid JSON.
func parseSession(data []byte) (model string, input, output int, ok bool) {
	var top map[string]any
	if err := json.Unmarshal(data, &top); err != nil {
		return "", 0, 0, false
	}
	if m, ok := top["model"].(string); ok {
		model = m
	}
	input, output = extractTokens(top)
	return model, input, output, true
}

// extractTokens searches a session object (and common nested keys) for
// input/output token counts under a variety of field names.
func extractTokens(m map[string]any) (input, output int) {
	input = toInt(m["input_tokens"])
	output = toInt(m["output_tokens"])
	if input == 0 {
		input = toInt(m["prompt_tokens"])
	}
	if output == 0 {
		output = toInt(m["completion_tokens"])
	}
	for _, key := range []string{"usage", "tokens", "token_usage", "cost"} {
		sub, ok := m[key].(map[string]any)
		if !ok {
			continue
		}
		if input == 0 {
			input = toInt(sub["input_tokens"])
			if input == 0 {
				input = toInt(sub["input"])
			}
			if input == 0 {
				input = toInt(sub["prompt_tokens"])
			}
		}
		if output == 0 {
			output = toInt(sub["output_tokens"])
			if output == 0 {
				output = toInt(sub["output"])
			}
			if output == 0 {
				output = toInt(sub["completion_tokens"])
			}
		}
	}
	return
}

// toInt coerces a JSON-decoded numeric value to an int.
func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}
