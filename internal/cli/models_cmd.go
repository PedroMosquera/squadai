package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/modelcatalog"
)

// ─── models list ──────────────────────────────────────────────────────────────

// modelsListRow is one model row in `models list --json` output.
type modelsListRow struct {
	ID            string  `json:"id"`
	Provider      string  `json:"provider"`
	Display       string  `json:"display,omitempty"`
	InputPerMTok  float64 `json:"input_per_mtok"`
	OutputPerMTok float64 `json:"output_per_mtok"`
	ContextWindow int     `json:"context_window,omitempty"`
	Encoding      string  `json:"encoding,omitempty"`
	Legacy        bool    `json:"legacy,omitempty"`
	Source        string  `json:"source"`
	Tier          string  `json:"tier,omitempty"`
}

type modelsListOutput struct {
	SchemaVersion int               `json:"schema_version"`
	Updated       string            `json:"updated,omitempty"`
	Adapter       string            `json:"adapter,omitempty"`
	Tiers         map[string]string `json:"tiers,omitempty"`
	Models        []modelsListRow   `json:"models"`
}

// RunModelsList lists the effective model catalog. It is fully offline:
// embedded defaults layered with ~/.squadai/models.json and .squadai/models.json.
func RunModelsList(args []string, stdout io.Writer) error {
	jsonOut := false
	adapter := ""
	for _, arg := range args {
		switch {
		case arg == "--json":
			jsonOut = true
		case strings.HasPrefix(arg, "--adapter="):
			adapter = strings.TrimPrefix(arg, "--adapter=")
		case arg == "-h" || arg == "--help":
			fmt.Fprintln(stdout, "Usage: squadai models list [--json] [--adapter=<id>]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "List the effective model catalog (embedded defaults + overrides).")
			fmt.Fprintln(stdout, "Offline: reads ~/.squadai/models.json and .squadai/models.json layers only.")
			return nil
		default:
			return fmt.Errorf("unknown flag %q for models list", arg)
		}
	}

	cat, err := loadEffectiveCatalog()
	if err != nil {
		return err
	}

	tierByModel := map[string]string{}
	var tiers map[string]string
	if adapter != "" {
		entry, ok := cat.Adapter(adapter)
		if !ok {
			return fmt.Errorf("unknown adapter %q — known: %s", adapter, strings.Join(cat.AdapterIDs(), ", "))
		}
		tiers = entry.Tiers
		for tier, id := range entry.Tiers {
			tierByModel[modelcatalog.Normalize(id)] = tier
		}
	}

	out := modelsListOutput{
		SchemaVersion: cat.SchemaVersion(),
		Updated:       cat.UpdatedString(),
		Adapter:       adapter,
		Tiers:         tiers,
	}
	for _, id := range cat.ModelIDs() {
		if adapter != "" {
			if _, ok := tierByModel[id]; !ok {
				continue
			}
		}
		m, _ := cat.Model(id)
		out.Models = append(out.Models, modelsListRow{
			ID:            id,
			Provider:      m.Provider,
			Display:       m.Display,
			InputPerMTok:  m.InputPerMTok,
			OutputPerMTok: m.OutputPerMTok,
			ContextWindow: m.ContextWindow,
			Encoding:      m.Encoding,
			Legacy:        m.Legacy,
			Source:        cat.Source(id),
			Tier:          tierByModel[id],
		})
	}

	if jsonOut {
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal models list: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "Model catalog (schema v%d, updated %s)\n\n", out.SchemaVersion, orDash(out.Updated))
	if adapter != "" {
		fmt.Fprintf(stdout, "Adapter %s tiers:\n", adapter)
		for _, tier := range []string{"premium", "standard", "cheap"} {
			if id, ok := tiers[tier]; ok {
				fmt.Fprintf(stdout, "  %-8s → %s\n", tier, id)
			}
		}
		fmt.Fprintln(stdout)
	}
	fmt.Fprintf(stdout, "%-28s %-10s %10s %10s %10s  %-8s %s\n",
		"MODEL", "PROVIDER", "IN $/MTok", "OUT $/MTok", "CONTEXT", "SOURCE", "NOTES")
	for _, r := range out.Models {
		notes := ""
		if r.Legacy {
			notes = "legacy"
		}
		if r.Tier != "" {
			if notes != "" {
				notes += ", "
			}
			notes += "tier:" + r.Tier
		}
		fmt.Fprintf(stdout, "%-28s %-10s %10.2f %10.2f %10s  %-8s %s\n",
			r.ID, r.Provider, r.InputPerMTok, r.OutputPerMTok,
			formatContext(r.ContextWindow), r.Source, notes)
	}
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func formatContext(n int) string {
	switch {
	case n <= 0:
		return "-"
	case n >= 1000000 && n%1000000 == 0:
		return fmt.Sprintf("%dM", n/1000000)
	case n >= 1000 && n%1000 == 0:
		return fmt.Sprintf("%dK", n/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// loadEffectiveCatalog loads the layered catalog for the current process
// (home directory + working directory).
func loadEffectiveCatalog() (*modelcatalog.Catalog, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}
	return modelcatalog.Load(homeDir, projectDir)
}

// ─── models check ─────────────────────────────────────────────────────────────

// catalogDelta summarizes the differences between the local effective catalog
// and the remote published one.
type catalogDelta struct {
	LocalUpdated  string   `json:"local_updated,omitempty"`
	RemoteUpdated string   `json:"remote_updated,omitempty"`
	Stale         bool     `json:"stale"`
	Added         []string `json:"added,omitempty"`
	Changed       []string `json:"changed,omitempty"`
	LocalOnly     []string `json:"local_only,omitempty"`
}

// diffCatalog compares the local effective catalog against a remote document.
func diffCatalog(local *modelcatalog.Catalog, remote *modelcatalog.File) catalogDelta {
	d := catalogDelta{
		LocalUpdated:  local.UpdatedString(),
		RemoteUpdated: remote.Updated,
	}
	if rt, err := time.Parse("2006-01-02", remote.Updated); err == nil {
		d.Stale = rt.After(local.Updated())
	}

	remoteIDs := make([]string, 0, len(remote.Models))
	for id := range remote.Models {
		remoteIDs = append(remoteIDs, id)
	}
	sort.Strings(remoteIDs)

	for _, id := range remoteIDs {
		rm := remote.Models[id]
		lm, ok := local.Model(id)
		if !ok {
			d.Added = append(d.Added, fmt.Sprintf("%s (%.2f/%.2f $/MTok)", id, rm.InputPerMTok, rm.OutputPerMTok))
			continue
		}
		if lm.InputPerMTok != rm.InputPerMTok || lm.OutputPerMTok != rm.OutputPerMTok ||
			lm.ContextWindow != rm.ContextWindow || lm.Encoding != rm.Encoding || lm.Legacy != rm.Legacy {
			d.Changed = append(d.Changed, fmt.Sprintf("%s (%.2f/%.2f → %.2f/%.2f $/MTok)",
				id, lm.InputPerMTok, lm.OutputPerMTok, rm.InputPerMTok, rm.OutputPerMTok))
		}
	}
	for _, id := range local.ModelIDs() {
		if _, ok := remote.Models[id]; !ok {
			d.LocalOnly = append(d.LocalOnly, fmt.Sprintf("%s (source: %s)", id, local.Source(id)))
		}
	}
	return d
}

func (d catalogDelta) hasChanges() bool {
	return len(d.Added) > 0 || len(d.Changed) > 0
}

// RunModelsCheck fetches the published catalog and reports staleness and
// deltas without changing anything.
func RunModelsCheck(args []string, stdout io.Writer) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai models check [--json]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Fetch the published model catalog and report staleness and differences.")
			fmt.Fprintln(stdout, "Never modifies anything — run 'squadai models update' to apply.")
			return nil
		default:
			return fmt.Errorf("unknown flag %q for models check", arg)
		}
	}

	local, err := loadEffectiveCatalog()
	if err != nil {
		return err
	}
	remote, _, err := modelcatalog.FetchRemote(context.Background())
	if err != nil {
		return err
	}
	d := diffCatalog(local, remote)

	if jsonOut {
		data, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal models check: %w", err)
		}
		fmt.Fprintln(stdout, string(data))
		return nil
	}

	fmt.Fprintf(stdout, "Local catalog:  updated %s\n", orDash(d.LocalUpdated))
	fmt.Fprintf(stdout, "Remote catalog: updated %s\n\n", orDash(d.RemoteUpdated))
	printDelta(stdout, d)
	if !d.hasChanges() {
		fmt.Fprintln(stdout, "Catalog is up to date.")
	} else {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Run 'squadai models update' to review and apply these changes.")
	}
	return nil
}

func printDelta(w io.Writer, d catalogDelta) {
	if len(d.Added) > 0 {
		fmt.Fprintln(w, "New models:")
		for _, s := range d.Added {
			fmt.Fprintf(w, "  + %s\n", s)
		}
	}
	if len(d.Changed) > 0 {
		fmt.Fprintln(w, "Changed models:")
		for _, s := range d.Changed {
			fmt.Fprintf(w, "  ~ %s\n", s)
		}
	}
	if len(d.LocalOnly) > 0 {
		fmt.Fprintln(w, "Local-only models (kept as-is):")
		for _, s := range d.LocalOnly {
			fmt.Fprintf(w, "  = %s\n", s)
		}
	}
	if d.hasChanges() || len(d.LocalOnly) > 0 {
		fmt.Fprintln(w)
	}
}

// ─── models update ────────────────────────────────────────────────────────────

// RunModelsUpdate fetches the published catalog, shows the diff against the
// effective local catalog, asks for confirmation (unless --yes), and writes
// the remote document atomically to the override path. It never touches the
// embedded catalog or project.json, and never proceeds without explicit
// user consent.
func RunModelsUpdate(args []string, stdout io.Writer, stdin io.Reader) error {
	yes := false
	project := false
	for _, arg := range args {
		switch arg {
		case "--yes":
			yes = true
		case "--project":
			project = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: squadai models update [--yes] [--project]")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Fetch the published model catalog, show the diff, and after confirmation")
			fmt.Fprintln(stdout, "write it to ~/.squadai/models.json (or .squadai/models.json with --project).")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "Flags:")
			fmt.Fprintln(stdout, "  --yes      Skip the confirmation prompt")
			fmt.Fprintln(stdout, "  --project  Write the project override instead of the user override")
			return nil
		default:
			return fmt.Errorf("unknown flag %q for models update", arg)
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
	local, err := modelcatalog.Load(homeDir, projectDir)
	if err != nil {
		return err
	}

	remote, raw, err := modelcatalog.FetchRemote(context.Background())
	if err != nil {
		return err
	}

	d := diffCatalog(local, remote)
	if !d.hasChanges() && !d.Stale {
		fmt.Fprintln(stdout, "Catalog is already up to date — nothing to write.")
		return nil
	}

	targetPath := modelcatalog.UserOverridePath(homeDir)
	if project {
		targetPath = modelcatalog.ProjectOverridePath(projectDir)
	}

	fmt.Fprintf(stdout, "Local catalog:  updated %s\n", orDash(d.LocalUpdated))
	fmt.Fprintf(stdout, "Remote catalog: updated %s\n\n", orDash(d.RemoteUpdated))
	printDelta(stdout, d)
	fmt.Fprintf(stdout, "This will write the remote catalog to %s\n", targetPath)

	if !yes {
		fmt.Fprint(stdout, "Proceed? [y/N] ")
		reader := bufio.NewReader(stdin)
		line, _ := reader.ReadString('\n')
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(stdout, "Aborted — no changes written.")
			return nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("create override directory: %w", err)
	}
	if _, err := fileutil.WriteAtomic(targetPath, raw, 0644); err != nil {
		return fmt.Errorf("write %s: %w", targetPath, err)
	}
	fmt.Fprintf(stdout, "Wrote %s (updated %s).\n", targetPath, orDash(remote.Updated))
	return nil
}
