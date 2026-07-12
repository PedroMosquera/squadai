package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/PedroMosquera/squadai/internal/governance"
)

// RunWatch monitors all managed files for drift and streams events to stdout.
// It runs until the user sends SIGINT/SIGTERM or cancels.
func RunWatch(args []string, stdout, stderr io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai watch [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Monitor all managed files for drift and stream events to stdout.")
			fmt.Fprintln(stdout, "Press Ctrl+C to stop. Events are also appended to .squadai/audit.log.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json  Output events as JSON lines instead of human-readable table.")
			return nil
		}
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	log := governance.OpenAuditLog(projectDir)

	onEvent := func(e governance.Event) {
		if jsonOut {
			data, _ := json.Marshal(e)
			fmt.Fprintln(stdout, string(data))
		} else {
			ts := e.Timestamp.Format("15:04:05")
			fmt.Fprintf(stdout, "%s  %-28s  %s  %s\n",
				ts, string(e.Kind), e.Path, e.Detail)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if !jsonOut {
		fmt.Fprintf(stdout, "watching %s — press Ctrl+C to stop\n\n", projectDir)
	}

	return governance.Watch(ctx, projectDir, governance.WatchOpts{
		AuditLog: log,
		OnEvent:  onEvent,
	})
}
