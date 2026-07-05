package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/modelcatalog"
)

const catConfig = "Project Configuration"

// runProjectConfig checks all configuration files.
func (d *Doctor) runProjectConfig(ctx context.Context) []CheckResult {
	return []CheckResult{
		d.checkProjectConfig(),
		d.checkPolicyConfig(),
		d.checkUserConfig(),
		d.checkStandards(),
		d.checkSquadRefined(ctx),
		d.checkModelsCatalogFreshness(),
		d.checkModelsKnown(),
		d.checkTokenBudgetUsage(),
	}
}

// modelsCatalogStaleAfter is how old the effective catalog data may be
// before doctor warns about staleness.
const modelsCatalogStaleAfter = 120 * 24 * time.Hour

// checkModelsCatalogFreshness warns when the effective model catalog data is
// older than 120 days. It never mutates anything — the fix is always an
// explicit, user-confirmed 'squadai models update'.
func (d *Doctor) checkModelsCatalogFreshness() CheckResult {
	cat, err := modelcatalog.Load(d.homeDir, d.projectDir)
	if err != nil {
		return warn(catConfig, "models-catalog-freshness",
			fmt.Sprintf("model catalog override could not be loaded: %v", err),
			"",
			"Fix or delete the invalid models.json override")
	}
	updated := cat.Updated()
	if updated.IsZero() {
		return warn(catConfig, "models-catalog-freshness",
			"model catalog has no updated date",
			"",
			"Run 'squadai models update' to refresh the catalog")
	}
	age := d.now().Sub(updated)
	if age > modelsCatalogStaleAfter {
		return warn(catConfig, "models-catalog-freshness",
			fmt.Sprintf("model catalog data is %d days old (updated %s)",
				int(age.Hours()/24), cat.UpdatedString()),
			"stale pricing and tier defaults may misprice sessions",
			"squadai models update")
	}
	return pass(catConfig, "models-catalog-freshness",
		fmt.Sprintf("model catalog data is current (updated %s)", cat.UpdatedString()),
		cat.UpdatedString())
}

// checkModelsKnown warns when the project config references concrete models
// the catalog does not know: those would price at $0 and tokenize with the
// chars/4 fallback.
func (d *Doctor) checkModelsKnown() CheckResult {
	proj, err := config.LoadProject(d.projectDir)
	if err != nil {
		return skip(catConfig, "models-known", "no project config — nothing to check")
	}
	cat, err := modelcatalog.Load(d.homeDir, d.projectDir)
	if err != nil {
		return skip(catConfig, "models-known", "model catalog unavailable — see models-catalog-freshness")
	}

	var configured []string
	for _, profile := range proj.Models.Profiles {
		for _, concreteModel := range profile.Adapters {
			if concreteModel != "" {
				configured = append(configured, concreteModel)
			}
		}
	}
	if len(configured) == 0 {
		return pass(catConfig, "models-known", "no concrete model overrides configured", "")
	}
	sort.Strings(configured)

	var unknown []string
	for _, m := range configured {
		if !cat.Known(m) {
			unknown = append(unknown, m)
		}
	}
	if len(unknown) > 0 {
		return warn(catConfig, "models-known",
			fmt.Sprintf("%d configured model(s) unknown to the catalog: %s",
				len(unknown), strings.Join(unknown, ", ")),
			"unknown models price at $0 and tokenize with the chars/4 fallback",
			"Run 'squadai models update' or fix the model IDs in project.json")
	}
	return pass(catConfig, "models-known",
		fmt.Sprintf("all %d configured model(s) known to the catalog", len(configured)),
		strings.Join(configured, ", "))
}

func (d *Doctor) checkProjectConfig() CheckResult {
	proj, err := config.LoadProject(d.projectDir)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return fail(catConfig, "project.json",
				".squadai/project.json missing",
				"",
				"Run 'squadai init' to create it")
		}
		return fail(catConfig, "project.json",
			fmt.Sprintf(".squadai/project.json parse error: %v", err),
			"",
			"Fix the JSON syntax in .squadai/project.json")
	}
	mode := "team"
	if proj.Methodology != "" {
		mode = string(proj.Methodology) + " methodology"
	}
	detail := fmt.Sprintf("v%d, %s", proj.Version, mode)
	return pass(catConfig, "project.json",
		fmt.Sprintf(".squadai/project.json valid (%s)", detail),
		detail)
}

func (d *Doctor) checkPolicyConfig() CheckResult {
	pol, err := config.LoadPolicy(d.projectDir)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return skip(catConfig, "policy.json", ".squadai/policy.json missing (optional for team mode)")
		}
		return fail(catConfig, "policy.json",
			fmt.Sprintf(".squadai/policy.json parse error: %v", err),
			"",
			"Fix the JSON syntax in .squadai/policy.json")
	}
	detail := fmt.Sprintf("%d locked field(s)", len(pol.Locked))
	return pass(catConfig, "policy.json",
		fmt.Sprintf(".squadai/policy.json valid (%s)", detail),
		detail)
}

func (d *Doctor) checkUserConfig() CheckResult {
	_, err := config.LoadUser(d.homeDir)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return CheckResult{
				Category:    catConfig,
				Name:        "config.json",
				Status:      CheckFail,
				Message:     "~/.squadai/config.json missing",
				FixHint:     "Create with default config (personal mode)",
				AutoFixable: true,
			}
		}
		return fail(catConfig, "config.json",
			fmt.Sprintf("~/.squadai/config.json parse error: %v", err),
			"",
			"Fix the JSON syntax in ~/.squadai/config.json")
	}
	path := config.UserConfigPath(d.homeDir)
	return pass(catConfig, "config.json",
		fmt.Sprintf("~/.squadai/config.json present at %s", path),
		path)
}

func (d *Doctor) checkStandards() CheckResult {
	path := filepath.Join(d.projectDir, config.ProjectConfigDir, "templates", "team-standards.md")
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return warn(catConfig, "team-standards.md",
				".squadai/templates/team-standards.md missing",
				"",
				"Run 'squadai init' to create it")
		}
		return warn(catConfig, "team-standards.md",
			fmt.Sprintf(".squadai/templates/team-standards.md error: %v", err),
			"", "")
	}
	if info.Size() == 0 {
		return warn(catConfig, "team-standards.md",
			".squadai/templates/team-standards.md is empty",
			"",
			"Add your team standards content")
	}
	return pass(catConfig, "team-standards.md",
		".squadai/templates/team-standards.md present",
		fmt.Sprintf("%d bytes", info.Size()))
}
