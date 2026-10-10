package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/scan"
)

const scanFailOnAllowed = "info, low, medium, high, none"

// RunScan runs the read-only security scan over the agent config in the
// current directory. It returns an exitcode.Policy error when any finding is
// at or above the --fail-on severity (default high).
func RunScan(args []string, stdout io.Writer) error {
	jsonOut := false
	failOn := "high"
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "--fail-on":
			if i+1 >= len(args) {
				return exitcode.ErrMissingArg("--fail-on value", "squadai scan --fail-on <"+strings.ReplaceAll(scanFailOnAllowed, ", ", "|")+">")
			}
			i++
			failOn = args[i]
		case strings.HasPrefix(arg, "--fail-on="):
			failOn = strings.TrimPrefix(arg, "--fail-on=")
		case arg == "-h" || arg == "--help":
			printScanUsage(stdout)
			return nil
		default:
			return exitcode.ErrUnknownValue("squadai scan", arg, "--json, --fail-on <severity>")
		}
	}

	threshold, gate := scan.SeverityHigh, true
	if strings.EqualFold(failOn, "none") {
		gate = false
	} else if s, ok := scan.ParseSeverity(failOn); ok {
		threshold = s
	} else {
		return exitcode.ErrUnknownValue("--fail-on", failOn, scanFailOnAllowed)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	report, err := scan.Scan(projectDir, scan.AllAdapters())
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	if jsonOut {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal scan report: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
	} else {
		printScanReport(stdout, report)
	}

	if !gate {
		return nil
	}
	failing := 0
	for _, f := range report.Findings {
		if f.Severity >= threshold {
			failing++
		}
	}
	if failing > 0 {
		return exitcode.New(exitcode.Policy, "E-203",
			fmt.Sprintf("scan found %d finding(s) at or above %s", failing, threshold),
			"Fix the findings, or change the gate with --fail-on <"+strings.ReplaceAll(scanFailOnAllowed, ", ", "|")+">.")
	}
	return nil
}

func printScanReport(w io.Writer, r scan.Report) {
	for _, s := range r.Skipped {
		fmt.Fprintf(w, "skipped %s: %s\n", s.File, s.Reason)
	}
	if len(r.Scanned) == 0 {
		fmt.Fprintln(w, "No agent config files found.")
		return
	}
	if len(r.Findings) == 0 {
		fmt.Fprintf(w, "No findings in %d config file(s).\n", len(r.Scanned))
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SEVERITY\tID\tLOCATION\tSUBJECT\tMESSAGE")
	counts := map[scan.Severity]int{}
	for _, f := range r.Findings {
		loc := f.File
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", f.Severity, f.ID, loc, f.Subject, f.Message)
		counts[f.Severity]++
	}
	tw.Flush()
	fmt.Fprintf(w, "\n%d finding(s): %d high, %d medium, %d low, %d info in %d config file(s)\n",
		len(r.Findings), counts[scan.SeverityHigh], counts[scan.SeverityMedium], counts[scan.SeverityLow], counts[scan.SeverityInfo], len(r.Scanned))
}

func printScanUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: squadai scan [--json] [--fail-on <info|low|medium|high|none>]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Read-only security scan of the agent config in the current project: MCP server")
	fmt.Fprintln(w, "definitions for every supported harness, Claude Code hooks, and project")
	fmt.Fprintln(w, ".claude/settings.json. Nothing is written or executed.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Rules:")
	for _, r := range scan.Rules() {
		fmt.Fprintf(w, "  %s  %-6s  %s\n", r.ID, r.Severity, r.Title)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --json      Output the report as JSON (scanned, skipped, findings).")
	fmt.Fprintln(w, "  --fail-on   Exit 3 when any finding is at or above this severity (default high).")
	fmt.Fprintln(w, "              'none' always exits 0 after a successful scan.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  squadai scan")
	fmt.Fprintln(w, "  squadai scan --json --fail-on medium")
}
