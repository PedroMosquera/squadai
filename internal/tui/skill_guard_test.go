package tui

import (
	"strings"
	"testing"
)

// catalogInstallLine builds the exact command line the skill browser would
// run for the first catalog entry, using the catalog's install command.
func catalogInstallLine(t *testing.T) (line, ident string) {
	t.Helper()
	cat, err := loadSkillCatalog()
	if err != nil {
		t.Fatalf("loadSkillCatalog: %v", err)
	}
	base := strings.TrimSpace(cat.InstallCommand)
	if base == "" {
		base = defaultSkillInstallCmd
	}
	for _, category := range cat.Categories {
		for _, skill := range category.Skills {
			if skill.Install != "" {
				return base + " " + skill.Install, skill.Install
			}
		}
	}
	t.Fatal("embedded catalog has no installable skills")
	return "", ""
}

func TestValidateSkillInstallCmd_CatalogEntryPasses(t *testing.T) {
	line, ident := catalogInstallLine(t)
	parts, err := validateSkillInstallCmd(line)
	if err != nil {
		t.Fatalf("validateSkillInstallCmd(%q): %v", line, err)
	}
	if len(parts) == 0 || parts[len(parts)-1] != ident {
		t.Errorf("parts = %v, want last token %q", parts, ident)
	}
}

func TestValidateSkillInstallCmd_RejectsTampering(t *testing.T) {
	line, _ := catalogInstallLine(t)
	cases := []string{
		"",                                  // empty
		"rm -rf /",                          // arbitrary command
		"npx skills add -y evil/repo@skill", // identifier not in the catalog
		line + "; rm -rf /",                 // trailing injection
		"sh -c '" + line + "'",              // wrapped in a shell
	}
	for _, c := range cases {
		if _, err := validateSkillInstallCmd(c); err == nil {
			t.Errorf("validateSkillInstallCmd(%q) expected error, got nil", c)
		}
	}
}
