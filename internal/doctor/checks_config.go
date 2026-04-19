package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
)

const catConfig = "Project Configuration"

// runProjectConfig checks all configuration files.
func (d *Doctor) runProjectConfig(_ context.Context) []CheckResult {
	return []CheckResult{
		d.checkProjectConfig(),
		d.checkPolicyConfig(),
		d.checkUserConfig(),
		d.checkStandards(),
	}
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
