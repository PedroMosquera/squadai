package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PedroMosquera/squadai/internal/memory"
)

// RunMemoryCommand dispatches to memory sub-subcommands.
func RunMemoryCommand(args []string) error {
	if len(args) == 0 {
		printMemoryUsage()
		return nil
	}
	switch args[0] {
	case "add":
		return RunMemoryAdd(args[1:])
	case "search":
		return RunMemorySearch(args[1:])
	case "promote":
		return RunMemoryPromote(args[1:])
	case "reindex":
		return RunMemoryReindex(args[1:])
	case "status":
		return RunMemoryStatus(args[1:])
	case "gc":
		return RunMemoryGC(args[1:])
	default:
		printMemoryUsage()
		return fmt.Errorf("unknown memory subcommand %q", args[0])
	}
}

func printMemoryUsage() {
	fmt.Fprint(os.Stdout, `Usage: squadai memory <subcommand> [args]

Subcommands:
  add "<note>"           Save a note to docs/memory/_inbox/
  search <query>         Search indexed memory notes (--json for JSON output)
  promote <inbox-path>   Move an inbox note to categorized memory with frontmatter
  reindex                Rebuild the search index from docs/memory/
  gc                     Archive stale unreferenced notes (--older-than=180d, --dry-run)
  status                 Show inbox, total, and indexed counts

`)
}

// resolveProjectDir returns the working directory to use as the project root.
func resolveProjectDir() (string, error) {
	return os.Getwd()
}

// RunMemoryAdd saves a note to docs/memory/_inbox/.
func RunMemoryAdd(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("memory add: note text required\nUsage: squadai memory add \"<note>\"")
	}

	noteText := strings.Join(args, " ")

	projectDir, err := resolveProjectDir()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	rel, err := memory.AddInbox(projectDir, noteText)
	if err != nil {
		return fmt.Errorf("memory add: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Note saved to %s\n", rel)
	return nil
}

// RunMemorySearch queries the memory index.
func RunMemorySearch(args []string) error {
	jsonOut := false
	var queryParts []string

	for _, arg := range args {
		if arg == "--json" {
			jsonOut = true
		} else {
			queryParts = append(queryParts, arg)
		}
	}

	query := strings.Join(queryParts, " ")
	if query == "" {
		return fmt.Errorf("memory search: query required\nUsage: squadai memory search <query>")
	}

	projectDir, err := resolveProjectDir()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	results, err := memory.SearchTFIDF(projectDir, query)
	if err != nil {
		results, err = memory.Search(projectDir, query)
		if err != nil {
			return fmt.Errorf("memory search: %w", err)
		}
	}

	if jsonOut {
		if results == nil {
			results = []memory.SearchResult{}
		}
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal search results: %w", err)
		}
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	}

	if len(results) == 0 {
		fmt.Fprintf(os.Stdout, "No results for %q.\n", query)
		return nil
	}

	for _, r := range results {
		fmt.Fprintf(os.Stdout, "[%.2f] %s: %s\n", r.Score, r.Path, r.FirstLine)
	}
	return nil
}

// RunMemoryPromote moves an inbox note to a categorized directory.
func RunMemoryPromote(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("memory promote: inbox path required\nUsage: squadai memory promote <inbox-path>")
	}

	inboxPath := args[0]

	projectDir, err := resolveProjectDir()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	// Prompt for category.
	fmt.Fprint(os.Stdout, "Category [decisions/learnings/incidents/other]: ")
	reader := bufio.NewReader(os.Stdin)
	category, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read category: %w", err)
	}
	category = strings.TrimSpace(category)
	if category == "" {
		category = "other"
	}

	newRel, err := memory.Promote(projectDir, inboxPath, category)
	if err != nil {
		return fmt.Errorf("memory promote: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Promoted to %s\n", newRel)
	return nil
}

// RunMemoryReindex rebuilds the search index.
func RunMemoryReindex(args []string) error {
	_ = args

	projectDir, err := resolveProjectDir()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	n, err := memory.Reindex(projectDir)
	if err != nil {
		return fmt.Errorf("memory reindex: %w", err)
	}

	tfidfN, _ := memory.ReindexTFIDF(projectDir)

	fmt.Fprintf(os.Stdout, "Index rebuilt. %d entries indexed (TF-IDF: %d).\n", n, tfidfN)
	return nil
}

// RunMemoryStatus prints memory counts.
func RunMemoryStatus(args []string) error {
	_ = args

	projectDir, err := resolveProjectDir()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	s, err := memory.Status(projectDir)
	if err != nil {
		return fmt.Errorf("memory status: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Memory status:\n")
	fmt.Fprintf(os.Stdout, "  inbox:   %d notes\n", s.InboxCount)
	fmt.Fprintf(os.Stdout, "  total:   %d notes\n", s.TotalCount)
	fmt.Fprintf(os.Stdout, "  indexed: %d entries\n", s.IndexedCount)
	return nil
}

// RunMemoryGC archives stale unreferenced notes.
func RunMemoryGC(args []string) error {
	dryRun := false
	olderThanStr := "180d"

	for _, arg := range args {
		switch {
		case arg == "--dry-run":
			dryRun = true
		case strings.HasPrefix(arg, "--older-than="):
			olderThanStr = arg[len("--older-than="):]
		case arg == "-h", arg == "--help":
			fmt.Fprintln(os.Stdout, "Usage: squadai memory gc [--older-than=<dur>] [--dry-run]")
			fmt.Fprintln(os.Stdout)
			fmt.Fprintln(os.Stdout, "Archive stale unreferenced memory notes to docs/memory/.archive/")
			fmt.Fprintln(os.Stdout, "Notes referenced in decisions/ ADR files are exempt.")
			fmt.Fprintln(os.Stdout)
			fmt.Fprintln(os.Stdout, "Flags:")
			fmt.Fprintln(os.Stdout, "  --older-than=<dur>  Age threshold (e.g. 180d, 90d). Default: 180d")
			fmt.Fprintln(os.Stdout, "  --dry-run           Show what would be archived without moving files")
			return nil
		}
	}

	days, err := strconv.Atoi(strings.TrimSuffix(olderThanStr, "d"))
	if err != nil {
		return fmt.Errorf("invalid --older-than value %q (expected e.g. 180d)", olderThanStr)
	}

	projectDir, err := resolveProjectDir()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	result, err := memory.GC(projectDir, time.Duration(days)*24*time.Hour, dryRun)
	if err != nil {
		return fmt.Errorf("memory gc: %w", err)
	}

	if dryRun {
		fmt.Fprintf(os.Stdout, "Dry run — %d note(s) would be archived:\n", len(result.Archived))
	} else {
		fmt.Fprintf(os.Stdout, "Archived %d note(s) to docs/memory/.archive/.\n", len(result.Archived))
	}
	for _, p := range result.Archived {
		fmt.Fprintf(os.Stdout, "  %s\n", p)
	}
	fmt.Fprintf(os.Stdout, "Remaining: %d notes\n", result.Remaining)
	return nil
}
