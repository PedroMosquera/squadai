package tui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PedroMosquera/squadai/internal/cli"
	"github.com/PedroMosquera/squadai/internal/doctor"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/governance"
	"github.com/PedroMosquera/squadai/internal/pipeline"
)

// commandResult carries the output of a CLI command execution.
type commandResult struct {
	output string
	err    error
}

// doctorCheckResult carries doctor check results from a tea.Cmd.
type doctorCheckResult struct {
	results []doctor.CheckResult
	err     error
}

// doctorFixDone carries the message to display after --fix attempt.
type doctorFixDone struct {
	msg string
}

// pipelineEventMsg wraps a single pipeline.Event received during apply.
type pipelineEventMsg struct {
	event pipeline.Event
}

// driftBadgeMsg carries the result of the background drift check for the menu badge.
type driftBadgeMsg struct {
	count int
	err   error
}

// watchDriftMsg carries a fresh drift check result for the watch screen.
type watchDriftMsg struct {
	results []governance.DriftResult
	at      time.Time
}

// watchTickMsg triggers the next drift poll on the watch screen.
type watchTickMsg struct{}

// auditLoadedMsg carries the audit log events loaded for the audit screen.
type auditLoadedMsg struct {
	events []governance.Event
	err    error
}

// listenForPipelineEvent returns a tea.Cmd that reads one event from ch and
// returns it as a pipelineEventMsg. When ch is closed the command returns nil.
func listenForPipelineEvent(ch <-chan pipeline.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return pipelineEventMsg{event: ev}
	}
}

func (m Model) runCommand(command string) tea.Cmd {
	return func() tea.Msg {
		var buf bytes.Buffer
		var err error

		switch command {
		case "plan":
			err = cli.RunPlan([]string{"--dry-run"}, &buf)
		case "apply":
			applyArgs := []string{}
			if m.setClaudeDefaultAgent {
				applyArgs = append(applyArgs, "--set-claude-default-agent")
			}
			err = cli.RunApply(applyArgs, &buf)
		case "verify":
			err = cli.RunVerify(nil, &buf)
		}

		return commandResult{output: buf.String(), err: err}
	}
}

// runApplyWithProgress starts the apply pipeline with a ChannelSink so the TUI
// can render per-step progress. It returns the two tea.Cmd values that must be
// batched: one that runs apply in a goroutine, and one that begins reading from
// the event channel. The Model's applyProgressCh field is set before returning.
func (m *Model) runApplyWithProgress() tea.Cmd {
	ch := make(chan pipeline.Event, 64)
	m.applyProgressCh = ch
	m.applyEvents = nil
	m.applyTotal = 0

	applyArgs := []string{}
	if m.setClaudeDefaultAgent {
		applyArgs = append(applyArgs, "--set-claude-default-agent")
	}

	sink := pipeline.NewChannelSink(ch, false) // non-blocking: TUI may lag

	runCmd := func() tea.Msg {
		var buf bytes.Buffer
		err := cli.RunApplyWithProgress(applyArgs, &buf, sink)
		close(ch) // signal that no more events are coming
		return commandResult{output: buf.String(), err: err}
	}

	return tea.Batch(runCmd, listenForPipelineEvent(ch))
}

// Run starts the TUI application.
func Run(version string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	// Load config to determine mode.
	merged, err := cli.LoadAndMerge(homeDir, "")
	if err != nil {
		// If config loading fails, use defaults.
		merged = &domain.MergedConfig{
			Mode: domain.ModePersonal,
		}
	}

	adapters := cli.DetectAdapters(homeDir)

	model := NewModel(version, merged.Mode, adapters, homeDir)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// runDoctorCmd returns a tea.Cmd that runs all doctor checks and returns results.
func (m Model) runDoctorCmd() tea.Cmd {
	homeDir := m.homeDir
	adapters := m.adapters
	return func() tea.Msg {
		projectDir, err := os.Getwd()
		if err != nil {
			return doctorCheckResult{err: fmt.Errorf("resolve working directory: %w", err)}
		}
		d := doctor.New(homeDir, projectDir, adapters, domain.DefaultMCPCatalog())
		results, err := d.Run(context.Background(), doctor.Options{})
		return doctorCheckResult{results: results, err: err}
	}
}

// runDoctorFixCmd returns a tea.Cmd that runs fixers for all auto-fixable failures.
func (m Model) runDoctorFixCmd() tea.Cmd {
	homeDir := m.homeDir
	adapters := m.adapters
	results := m.doctorResults
	return func() tea.Msg {
		projectDir, err := os.Getwd()
		if err != nil {
			return doctorFixDone{msg: fmt.Sprintf("Error: %v", err)}
		}
		d := doctor.New(homeDir, projectDir, adapters, domain.DefaultMCPCatalog())
		fixResults := d.Fix(context.Background(), results)

		var fixable int
		for _, r := range results {
			if r.AutoFixable && r.Status == doctor.CheckFail {
				fixable++
			}
		}
		if fixable == 0 {
			return doctorFixDone{msg: "No auto-fixable issues."}
		}

		var sb strings.Builder
		for _, fr := range fixResults {
			if fr.Err != nil {
				sb.WriteString(fmt.Sprintf("✗ %s — %v\n", fr.CheckResult.Name, fr.Err))
			} else {
				sb.WriteString(fmt.Sprintf("✓ %s — fixed\n", fr.CheckResult.Name))
			}
		}
		return doctorFixDone{msg: strings.TrimRight(sb.String(), "\n")}
	}
}

// runShellCommand executes a skill-install command line and returns the
// combined output as a commandResult, suitable for dispatch into the result
// screen.
func runShellCommand(cmdLine string) commandResult {
	parts, err := validateSkillInstallCmd(cmdLine)
	if err != nil {
		return commandResult{err: err}
	}
	// Invariant: validateSkillInstallCmd guarantees parts is exactly the
	// catalog's install command followed by a skill identifier listed in the
	// embedded catalog — never an arbitrary catalog-supplied string.
	cmd := exec.Command(parts[0], parts[1:]...) //nolint:gosec // argv validated against the embedded skill catalog allowlist
	out, err := cmd.CombinedOutput()
	return commandResult{output: string(out), err: err}
}

// ─── governance helpers ──────────────────────────────────────────────────────

// runDriftBadgeCmd runs CheckDrift in the background and returns a driftBadgeMsg.
func (m Model) runDriftBadgeCmd() tea.Cmd {
	return func() tea.Msg {
		projectDir, err := os.Getwd()
		if err != nil {
			return driftBadgeMsg{err: err}
		}
		results, err := governance.CheckDrift(projectDir)
		if err != nil {
			return driftBadgeMsg{err: err}
		}
		var count int
		for _, r := range results {
			if r.Drifted() {
				count++
			}
		}
		return driftBadgeMsg{count: count}
	}
}

// runWatchDriftCmd runs CheckDrift for the watch screen.
func (m Model) runWatchDriftCmd() tea.Cmd {
	return func() tea.Msg {
		projectDir, err := os.Getwd()
		if err != nil {
			return watchDriftMsg{at: time.Now()}
		}
		results, _ := governance.CheckDrift(projectDir)
		return watchDriftMsg{results: results, at: time.Now()}
	}
}

// loadAuditCmd reads the audit log for the audit screen.
func (m Model) loadAuditCmd() tea.Cmd {
	return func() tea.Msg {
		projectDir, err := os.Getwd()
		if err != nil {
			return auditLoadedMsg{err: err}
		}
		log := governance.OpenAuditLog(projectDir)
		events, err := log.Read(0, "")
		return auditLoadedMsg{events: events, err: err}
	}
}
