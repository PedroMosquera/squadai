package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/PedroMosquera/squadai/internal/governance"
)

// RunAudit renders the audit log for the current project.
func RunAudit(args []string, stdout io.Writer) error {
	jsonOut := false
	var sinceAge time.Duration
	filterKind := ""

	for _, arg := range args {
		switch {
		case arg == "--json":
			jsonOut = true
		case strings.HasPrefix(arg, "--since="):
			val := strings.TrimPrefix(arg, "--since=")
			d, err := time.ParseDuration(val)
			if err != nil {
				return fmt.Errorf("invalid --since value %q: %w", val, err)
			}
			sinceAge = d
		case strings.HasPrefix(arg, "--filter="):
			filterKind = strings.TrimPrefix(arg, "--filter=")
		case arg == "-h" || arg == "--help":
			fmt.Fprintln(stdout, "Usage: squadai audit [--json] [--since=<duration>] [--filter=<kind-prefix>]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Render the governance audit log (.squadai/audit.log).")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json             Output raw JSON lines.")
			fmt.Fprintln(stdout, "  --since=<duration> Show events from the last duration (e.g. 24h, 30m).")
			fmt.Fprintln(stdout, "  --filter=<prefix>  Show only events whose kind starts with prefix (e.g. drift).")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai audit")
			fmt.Fprintln(stdout, "  squadai audit --since=24h --filter=drift")
			fmt.Fprintln(stdout, "  squadai audit --json")
			return nil
		}
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	log := governance.OpenAuditLog(projectDir)
	events, err := log.Read(sinceAge, filterKind)
	if err != nil {
		return fmt.Errorf("read audit log: %w", err)
	}

	if len(events) == 0 {
		fmt.Fprintln(stdout, "No events found.")
		return nil
	}

	if jsonOut {
		for _, e := range events {
			data, _ := json.Marshal(e)
			fmt.Fprintln(stdout, string(data))
		}
		return nil
	}

	// Table header.
	fmt.Fprintf(stdout, "%-20s  %-30s  %-30s  %s\n", "TIMESTAMP", "KIND", "PATH", "DETAIL")
	fmt.Fprintln(stdout, strings.Repeat("-", 100))
	for _, e := range events {
		ts := e.Timestamp.Format("2006-01-02 15:04:05")
		fmt.Fprintf(stdout, "%-20s  %-30s  %-30s  %s\n",
			ts, string(e.Kind), e.Path, e.Detail)
	}
	fmt.Fprintf(stdout, "\n%d event(s)\n", len(events))
	return nil
}
