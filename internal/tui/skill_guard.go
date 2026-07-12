package tui

import (
	"fmt"
	"regexp"
	"strings"
)

// defaultSkillInstallCmd mirrors the fallback install command used by the
// skill browser when the embedded catalog declares no install_command.
const defaultSkillInstallCmd = "npx skills add -y"

// safeSkillTokenRe matches argv tokens made only of characters the catalog
// legitimately uses (letters, digits, and @._/-), so no token can carry
// shell metacharacters, whitespace tricks, or path escapes.
var safeSkillTokenRe = regexp.MustCompile(`^[A-Za-z0-9@._/-]+$`)

// validateSkillInstallCmd validates a skill-install command line against the
// embedded skill catalog before it may be executed: the line must be exactly
// the catalog's install command (or the built-in default) followed by a skill
// identifier that appears in the catalog, and every argv token must match
// safeSkillTokenRe. It returns the argv tokens on success.
func validateSkillInstallCmd(cmdLine string) ([]string, error) {
	cat, err := loadSkillCatalog()
	if err != nil {
		return nil, err
	}
	base := strings.TrimSpace(cat.InstallCommand)
	if base == "" {
		base = defaultSkillInstallCmd
	}

	ident, ok := strings.CutPrefix(cmdLine, base+" ")
	if !ok {
		return nil, fmt.Errorf("refusing to run %q: not the catalog skill-install command", cmdLine)
	}
	ident = strings.TrimSpace(ident)

	found := false
	for _, category := range cat.Categories {
		for _, skill := range category.Skills {
			if skill.Install == ident {
				found = true
				break
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("refusing to run %q: skill %q is not in the embedded catalog", cmdLine, ident)
	}

	parts := strings.Fields(cmdLine)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	for _, p := range parts {
		if !safeSkillTokenRe.MatchString(p) {
			return nil, fmt.Errorf("refusing to run %q: token %q contains unexpected characters", cmdLine, p)
		}
	}
	return parts, nil
}
