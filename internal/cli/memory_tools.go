package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/PedroMosquera/squadai/internal/memory"
)

// RunMemorySearchTool is the MCP tool handler for memory_search.
// It reads the cwd, calls memory.Search, and returns results as a JSON array.
func RunMemorySearchTool(args []string, stdout io.Writer) error {
	var query string
	for _, arg := range args {
		if strings.HasPrefix(arg, "--query=") {
			query = strings.TrimPrefix(arg, "--query=")
		}
	}
	if query == "" {
		return fmt.Errorf("memory_search: query is required")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	results, err := memory.Search(projectDir, query)
	if err != nil {
		return fmt.Errorf("memory search: %w", err)
	}

	if results == nil {
		results = []memory.SearchResult{}
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal search results: %w", err)
	}
	fmt.Fprintln(stdout, string(data))
	return nil
}

// RunMemoryAddTool is the MCP tool handler for memory_add.
// It reads the cwd, calls memory.AddInbox, and returns the saved path as JSON.
func RunMemoryAddTool(args []string, stdout io.Writer) error {
	var note string
	for _, arg := range args {
		if strings.HasPrefix(arg, "--note=") {
			note = strings.TrimPrefix(arg, "--note=")
		}
	}
	if note == "" {
		return fmt.Errorf("memory_add: note is required")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	savedPath, err := memory.AddInbox(projectDir, note)
	if err != nil {
		return fmt.Errorf("memory add: %w", err)
	}

	type result struct {
		Path string `json:"path"`
	}
	data, err := json.MarshalIndent(result{Path: savedPath}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal memory add result: %w", err)
	}
	fmt.Fprintln(stdout, string(data))
	return nil
}
