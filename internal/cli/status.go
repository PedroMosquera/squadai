package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/backup"
	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/planner"
	"github.com/PedroMosquera/squadai/internal/squadrefine"
	"github.com/PedroMosquera/squadai/internal/verify"
)

type squadRefinementInfo struct {
	Status    string   `json:"status"`
	Reasons   []string `json:"reasons"`
	LastRunAt string   `json:"last_run_at,omitempty"`
}

// RunStatus shows the current project configuration summary with health checks.
func RunStatus(args []string, stdout io.Writer) error {
	jsonOut := false
	fix := false
	daily := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "--fix":
			fix = true
		case "--daily":
			daily = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai status [--json] [--fix] [--daily]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Show the current project configuration summary: detected agents, active components,")
			fmt.Fprintln(stdout, "configured MCP servers, health checks, and the most recent backup.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --json   Output the status as JSON.")
			fmt.Fprintln(stdout, "  --fix    Run 'squadai apply' to fix any health issues found.")
			fmt.Fprintln(stdout, "  --daily  Show the daily-driver dashboard summary.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Examples:")
			fmt.Fprintln(stdout, "  squadai status")
			fmt.Fprintln(stdout, "  squadai status --json")
			fmt.Fprintln(stdout, "  squadai status --daily")
			fmt.Fprintln(stdout, "  squadai status --fix")
			return nil
		default:
			return fmt.Errorf("unknown flag %q for status", arg)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	mergedCfg, err := loadAndMerge(homeDir, projectDir)
	if err != nil {
		return err
	}
	// Keep a pre-profile view for config-level reporting (e.g. the memory
	// conflict note must fire on what the user configured, not on the
	// profile-filtered effective set). Shallow copy: applyProfileToConfig
	// replaces the MCP/Components maps rather than mutating them in place.
	cfgBeforeProfile := *mergedCfg
	applyDefaultProfile(mergedCfg)

	adapters := DetectAdapters(homeDir)

	p := planner.New()
	actions, err := p.Plan(mergedCfg, adapters, homeDir, projectDir)
	if err != nil {
		return exitcode.ErrPlanFailed(err)
	}

	// Run verify to get health results.
	v := verify.New()
	verifyReport, verifyErr := v.Verify(mergedCfg, adapters, homeDir, projectDir)
	if verifyErr != nil {
		verifyReport = nil
	}

	// Count managed files per component (non-skip actions grouped by component).
	compOrder := make([]string, 0)
	compMap := make(map[string]map[string]bool)
	for _, a := range actions {
		if a.Action == domain.ActionSkip {
			continue
		}
		if a.TargetPath == "" {
			continue
		}
		comp := string(a.Component)
		if _, exists := compMap[comp]; !exists {
			compMap[comp] = make(map[string]bool)
			compOrder = append(compOrder, comp)
		}
		compMap[comp][a.TargetPath] = true
	}

	// Also include components that only have skip actions (already managed).
	for _, a := range actions {
		if a.TargetPath == "" {
			continue
		}
		comp := string(a.Component)
		if _, exists := compMap[comp]; !exists {
			compMap[comp] = make(map[string]bool)
			compOrder = append(compOrder, comp)
		}
		compMap[comp][a.TargetPath] = true
	}

	// Deduplicate compOrder preserving first appearance.
	seenComp := make(map[string]bool)
	deduped := make([]string, 0, len(compOrder))
	for _, c := range compOrder {
		if !seenComp[c] {
			seenComp[c] = true
			deduped = append(deduped, c)
		}
	}
	compOrder = deduped
	sort.Strings(compOrder)

	// Get MCP server names.
	mcpNames := make([]string, 0, len(mergedCfg.MCP))
	for name := range mergedCfg.MCP {
		mcpNames = append(mcpNames, name)
	}
	sort.Strings(mcpNames)

	// Get most recent backup.
	backupDir := backup.ResolveBackupDir(mergedCfg.Paths.BackupDir, homeDir)
	store := backup.NewStore(backupDir)
	manifests, listErr := store.List()
	var lastManifest *backup.Manifest
	if listErr == nil && len(manifests) > 0 {
		lastManifest = &manifests[0]
	}

	// Compute squad refinement status (skip when project not initialized).
	var refInfo *squadRefinementInfo
	projectConfigPath := config.ProjectConfigPath(projectDir)
	if _, statErr := os.Stat(projectConfigPath); statErr == nil {
		refState, refExists, _ := squadrefine.Load(projectDir)
		if !refExists {
			refInfo = &squadRefinementInfo{Status: "never-refined", Reasons: []string{}}
		} else {
			signals := sampleDriftSignals(projectDir)
			reasons := squadrefine.DriftReasons(refState, signals)
			if len(reasons) == 0 {
				refInfo = &squadRefinementInfo{Status: "fresh", Reasons: []string{}, LastRunAt: refState.LastRunAt}
			} else {
				refInfo = &squadRefinementInfo{Status: "stale", Reasons: reasons, LastRunAt: refState.LastRunAt}
			}
		}
	}

	if jsonOut {
		type adapterStatus struct {
			ID         string `json:"id"`
			Delegation string `json:"delegation,omitempty"`
		}
		type componentStatus struct {
			ID           string `json:"id"`
			ManagedFiles int    `json:"managed_files"`
		}
		type backupInfo struct {
			ID        string `json:"id"`
			Timestamp string `json:"timestamp"`
			Files     int    `json:"files"`
		}
		type healthSummary struct {
			AllPass       bool `json:"all_pass"`
			TotalChecks   int  `json:"total_checks"`
			FailingChecks int  `json:"failing_checks"`
		}
		type refinementJSON struct {
			Status    string   `json:"status"`
			Reasons   []string `json:"reasons"`
			LastRunAt string   `json:"last_run_at,omitempty"`
		}
		type statusResult struct {
			ProjectDir     string              `json:"project_dir"`
			Language       string              `json:"language,omitempty"`
			Methodology    string              `json:"methodology,omitempty"`
			Mode           string              `json:"mode,omitempty"`
			Preset         string              `json:"preset,omitempty"`
			ContextProfile string              `json:"context_profile,omitempty"`
			Memory         domain.MemoryConfig `json:"memory,omitempty"`
			Usage          domain.UsageConfig  `json:"usage,omitempty"`
			ModelTier      string              `json:"model_tier,omitempty"`
			Adapters       []adapterStatus     `json:"adapters"`
			Components     []componentStatus   `json:"components"`
			MCPServers     []string            `json:"mcp_servers"`
			LastBackup     *backupInfo         `json:"last_backup,omitempty"`
			Health         *healthSummary      `json:"health,omitempty"`
			Refinement     *refinementJSON     `json:"refinement,omitempty"`
			Context        *contextHealth      `json:"context,omitempty"`
		}

		adapterList := make([]adapterStatus, 0, len(adapters))
		for _, a := range adapters {
			adapterList = append(adapterList, adapterStatus{
				ID:         string(a.ID()),
				Delegation: string(a.DelegationStrategy()),
			})
		}

		componentList := make([]componentStatus, 0, len(compOrder))
		for _, comp := range compOrder {
			componentList = append(componentList, componentStatus{
				ID:           comp,
				ManagedFiles: len(compMap[comp]),
			})
		}

		var lastBackupJSON *backupInfo
		if lastManifest != nil {
			lastBackupJSON = &backupInfo{
				ID:        lastManifest.ID,
				Timestamp: lastManifest.Timestamp.Format("2006-01-02T15:04:05Z"),
				Files:     len(lastManifest.AffectedFiles),
			}
		}

		var health *healthSummary
		if verifyReport != nil {
			failing := 0
			for _, r := range verifyReport.Results {
				if !r.Passed && r.Severity == domain.SeverityError {
					failing++
				}
			}
			health = &healthSummary{
				AllPass:       verifyReport.AllPass,
				TotalChecks:   len(verifyReport.Results),
				FailingChecks: failing,
			}
		}

		var refJSON *refinementJSON
		if refInfo != nil {
			refJSON = &refinementJSON{
				Status:    refInfo.Status,
				Reasons:   refInfo.Reasons,
				LastRunAt: refInfo.LastRunAt,
			}
		}
		result := statusResult{
			ProjectDir:     projectDir,
			Language:       mergedCfg.Meta.Language,
			Methodology:    string(mergedCfg.Methodology),
			Mode:           string(mergedCfg.Mode),
			Preset:         string(mergedCfg.Preset),
			ContextProfile: mergedCfg.Context.DefaultProfile,
			Memory:         mergedCfg.Memory,
			Usage:          mergedCfg.Usage,
			ModelTier:      string(mergedCfg.ModelTier),
			Adapters:       adapterList,
			Components:     componentList,
			MCPServers:     mcpNames,
			LastBackup:     lastBackupJSON,
			Health:         health,
			Refinement:     refJSON,
			Context:        collectContextHealth(homeDir, projectDir, mergedCfg, false),
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal status result: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	// Human-readable output.
	language := mergedCfg.Meta.Language
	if language == "" {
		language = "unknown"
	}
	methodology := string(mergedCfg.Methodology)
	if methodology == "" {
		methodology = "none"
	}
	mode := string(mergedCfg.Mode)
	if mode == "" {
		mode = "standard"
	}
	preset := string(mergedCfg.Preset)
	if preset == "" {
		preset = "custom"
	}
	contextProfile := mergedCfg.Context.DefaultProfile
	if contextProfile == "" {
		contextProfile = "default"
	}

	if daily {
		printDailyStatus(stdout, projectDir, language, methodology, mode, preset, contextProfile, &cfgBeforeProfile, adapters, mcpNames, lastManifest, verifyReport, refInfo)
		// Session aggregation can be slow, so real usage only appears in the
		// daily dashboard, not the fast default status.
		printContextSection(stdout, collectContextHealth(homeDir, projectDir, mergedCfg, true))
		return nil
	}

	fmt.Fprintf(stdout, "Project: %s (%s)\n", filepath.Base(projectDir), language)
	fmt.Fprintf(stdout, "Methodology: %s\n", methodology)
	fmt.Fprintf(stdout, "Mode: %s\n", mode)
	fmt.Fprintf(stdout, "Preset: %s\n", preset)
	fmt.Fprintf(stdout, "Context profile: %s\n", contextProfile)
	fmt.Fprintln(stdout)

	fmt.Fprintf(stdout, "Agents (%d enabled):\n", len(adapters))
	for _, a := range adapters {
		fmt.Fprintf(stdout, "  %-20s %-12s %s\n", string(a.ID()), string(a.Lane()), string(a.DelegationStrategy()))
	}
	fmt.Fprintln(stdout)

	fmt.Fprintf(stdout, "Components (%d active):\n", len(compOrder))
	for _, comp := range compOrder {
		fmt.Fprintf(stdout, "  %-20s %d files managed\n", comp, len(compMap[comp]))
	}
	fmt.Fprintln(stdout)

	mcpDisplay := "none"
	if len(mcpNames) > 0 {
		mcpDisplay = strings.Join(mcpNames, ", ")
	}
	fmt.Fprintf(stdout, "MCP servers: %s\n", mcpDisplay)
	if bothMemorySystemsEnabled(&cfgBeforeProfile) {
		fmt.Fprintln(stdout, memoryConflictNote)
	}
	fmt.Fprintln(stdout)

	if lastManifest != nil {
		fmt.Fprintf(stdout, "Last backup: %s (%d files)\n",
			lastManifest.Timestamp.Format("2006-01-02 15:04:05 UTC"),
			len(lastManifest.AffectedFiles))
	} else {
		fmt.Fprintln(stdout, "Last backup: none")
	}

	// Health section.
	if verifyReport != nil && len(verifyReport.Results) > 0 {
		fmt.Fprintln(stdout)
		failing := make([]domain.VerifyResult, 0)
		for _, r := range verifyReport.Results {
			if !r.Passed && r.Severity == domain.SeverityError {
				failing = append(failing, r)
			}
		}
		if len(failing) == 0 {
			fmt.Fprintf(stdout, "Health: OK (%d checks passed)\n", len(verifyReport.Results))
		} else {
			fmt.Fprintf(stdout, "Health: %d failing check(s) of %d\n", len(failing), len(verifyReport.Results))
			for _, r := range failing {
				msg := r.Check
				if r.Message != "" {
					msg = r.Message
				}
				fmt.Fprintf(stdout, "  [FAIL] %s\n", msg)
			}
			if fix {
				fmt.Fprintln(stdout)
				fmt.Fprintln(stdout, "Running 'squadai apply' to fix issues…")
				return RunApply([]string{}, stdout)
			}
			fmt.Fprintln(stdout, "\nRun 'squadai apply' to fix, or 'squadai status --fix' to fix automatically.")
		}
	}

	// Refinement section.
	if refInfo != nil {
		fmt.Fprintln(stdout)
		switch refInfo.Status {
		case "fresh":
			fmt.Fprintln(stdout, "Refinement: fresh")
		case "stale":
			fmt.Fprintf(stdout, "Refinement: stale: %s\n", strings.Join(refInfo.Reasons, ", "))
		default:
			fmt.Fprintln(stdout, "Refinement: never-refined")
		}
	}

	// Context health section (installed token cost vs profile cap).
	printContextSection(stdout, collectContextHealth(homeDir, projectDir, mergedCfg, false))

	return nil
}

// memoryConflictNote is shown when both the community knowledge-graph MCP
// server (config key "memory") and SquadAI Project Memory are enabled.
const memoryConflictNote = "note: both memory systems enabled — they don't share data"

// bothMemorySystemsEnabled reports whether the community knowledge-graph MCP
// server and the SquadAI Project Memory component are enabled at the same time.
func bothMemorySystemsEnabled(cfg *domain.MergedConfig) bool {
	if cfg == nil {
		return false
	}
	mcpDef, ok := cfg.MCP["memory"]
	if !ok || !mcpDef.Enabled {
		return false
	}
	comp, ok := cfg.Components[string(domain.ComponentMemory)]
	return ok && comp.Enabled
}

func printDailyStatus(stdout io.Writer, projectDir, language, methodology, mode, preset, contextProfile string, mergedCfg *domain.MergedConfig, adapters []domain.Adapter, mcpNames []string, lastManifest *backup.Manifest, verifyReport *domain.VerifyReport, refInfo *squadRefinementInfo) {
	fmt.Fprintf(stdout, "Daily Status: %s\n", filepath.Base(projectDir))
	fmt.Fprintf(stdout, "Project: %s (%s)\n", projectDir, language)
	fmt.Fprintf(stdout, "Setup: preset=%s methodology=%s mode=%s profile=%s\n", preset, methodology, mode, contextProfile)

	agentIDs := make([]string, 0, len(adapters))
	for _, a := range adapters {
		agentIDs = append(agentIDs, string(a.ID()))
	}
	sort.Strings(agentIDs)
	if len(agentIDs) == 0 {
		fmt.Fprintln(stdout, "Agents: none detected")
	} else {
		fmt.Fprintf(stdout, "Agents: %s\n", strings.Join(agentIDs, ", "))
	}

	mcpDisplay := "none"
	if len(mcpNames) > 0 {
		mcpDisplay = strings.Join(mcpNames, ", ")
	}
	fmt.Fprintf(stdout, "MCP: %s\n", mcpDisplay)
	if bothMemorySystemsEnabled(mergedCfg) {
		fmt.Fprintln(stdout, memoryConflictNote)
	}

	memoryBackend := mergedCfg.Memory.Backend
	if memoryBackend == "" {
		memoryBackend = "docs"
	}
	memoryExport := mergedCfg.Memory.ExportPath
	if memoryExport == "" {
		memoryExport = "docs/memory"
	}
	autoCapture := "off"
	if mergedCfg.Memory.AutoCapture {
		autoCapture = "on"
	}
	fmt.Fprintf(stdout, "Memory: backend=%s auto_capture=%s export=%s\n", memoryBackend, autoCapture, memoryExport)

	enforcement := mergedCfg.Usage.Enforcement
	if enforcement == "" {
		enforcement = "off"
	}
	fmt.Fprintf(stdout, "Usage: enforcement=%s session_tokens=%d daily_tokens=%d", enforcement, mergedCfg.Usage.SessionTokenBudget, mergedCfg.Usage.DailyTokenBudget)
	if mergedCfg.Usage.SessionCostBudget > 0 || mergedCfg.Usage.DailyCostBudget > 0 {
		currency := mergedCfg.Usage.Currency
		if currency == "" {
			currency = "USD"
		}
		fmt.Fprintf(stdout, " session_cost=%.2f%s daily_cost=%.2f%s", mergedCfg.Usage.SessionCostBudget, currency, mergedCfg.Usage.DailyCostBudget, currency)
	}
	fmt.Fprintln(stdout)

	if verifyReport != nil {
		failing := 0
		for _, r := range verifyReport.Results {
			if !r.Passed && r.Severity == domain.SeverityError {
				failing++
			}
		}
		if failing == 0 {
			fmt.Fprintf(stdout, "Health: OK (%d checks)\n", len(verifyReport.Results))
		} else if lastManifest == nil {
			fmt.Fprintf(stdout, "Health: setup pending (%d missing or stale check(s) of %d)\n", failing, len(verifyReport.Results))
			fmt.Fprintln(stdout, "Next: run squadai apply --no-review")
		} else {
			fmt.Fprintf(stdout, "Health: %d failing check(s) of %d\n", failing, len(verifyReport.Results))
		}
	} else {
		fmt.Fprintln(stdout, "Health: unavailable")
	}

	if refInfo != nil {
		refinement := refInfo.Status
		if len(refInfo.Reasons) > 0 {
			refinement += " (" + strings.Join(refInfo.Reasons, ", ") + ")"
		}
		fmt.Fprintf(stdout, "Refinement: %s\n", refinement)
	}

	if lastManifest != nil {
		fmt.Fprintf(stdout, "Last backup: %s (%d files)\n", lastManifest.Timestamp.Format("2006-01-02 15:04:05 UTC"), len(lastManifest.AffectedFiles))
	} else {
		fmt.Fprintln(stdout, "Last backup: none")
	}
}

// RunSquadInitStatus returns the current squad refinement status as JSON.
// It is used by the squad_init_status MCP tool and reads from the cwd.
func RunSquadInitStatus(_ []string, stdout io.Writer) error {
	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	type result struct {
		Status    string   `json:"status"`
		Reasons   []string `json:"reasons"`
		LastRunAt string   `json:"last_run_at,omitempty"`
	}

	// When project.json does not exist, return not-initialized status.
	projectConfigPath := config.ProjectConfigPath(projectDir)
	if _, statErr := os.Stat(projectConfigPath); os.IsNotExist(statErr) {
		r := result{Status: "not-initialized", Reasons: []string{}}
		data, _ := json.MarshalIndent(r, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	refState, exists, loadErr := squadrefine.Load(projectDir)
	if loadErr != nil {
		return fmt.Errorf("load squad-refined state: %w", loadErr)
	}

	if !exists {
		r := result{Status: "never-refined", Reasons: []string{}}
		data, _ := json.MarshalIndent(r, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	signals := sampleDriftSignals(projectDir)
	reasons := squadrefine.DriftReasons(refState, signals)
	if len(reasons) == 0 {
		r := result{Status: "fresh", Reasons: []string{}, LastRunAt: refState.LastRunAt}
		data, _ := json.MarshalIndent(r, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	r := result{Status: "stale", Reasons: reasons, LastRunAt: refState.LastRunAt}
	data, _ := json.MarshalIndent(r, "", "  ")
	fmt.Fprintln(stdout, string(data))
	return nil
}
