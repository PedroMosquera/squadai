package tui

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/assets"
)

// skillEntry is a single skill in the curated catalog.
type skillEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Install     string `json:"install"` // owner/repo@skill identifier passed to `npx skills add`
}

// skillCategory groups related skills.
type skillCategory struct {
	Name   string       `json:"name"`
	Skills []skillEntry `json:"skills"`
}

// skillCatalog is the top-level structure of skills/catalog.json.
type skillCatalog struct {
	Categories     []skillCategory `json:"categories"`
	InstallCommand string          `json:"install_command"`
	SearchCommand  string          `json:"search_command"`
	BrowseURL      string          `json:"browse_url"`
}

// loadSkillCatalog reads and parses the embedded skills/catalog.json.
func loadSkillCatalog() (skillCatalog, error) {
	raw, err := assets.Read("skills/catalog.json")
	if err != nil {
		return skillCatalog{}, fmt.Errorf("load skill catalog: %w", err)
	}
	var cat skillCatalog
	if err := json.Unmarshal([]byte(raw), &cat); err != nil {
		return skillCatalog{}, fmt.Errorf("parse skill catalog: %w", err)
	}
	// Sort categories and their skills deterministically.
	sort.Slice(cat.Categories, func(i, j int) bool {
		return cat.Categories[i].Name < cat.Categories[j].Name
	})
	for ci := range cat.Categories {
		sort.Slice(cat.Categories[ci].Skills, func(i, j int) bool {
			return cat.Categories[ci].Skills[i].Name < cat.Categories[ci].Skills[j].Name
		})
	}
	return cat, nil
}

// viewSkillBrowser renders the community skill browser screen.
func (m Model) viewSkillBrowser() string {
	var content strings.Builder
	content.WriteString(headingStyle.Render("Community Skills (skills.sh)") + "\n\n")

	if m.skillCatErr != nil {
		content.WriteString(errorStyle.Render("Could not load catalog: "+m.skillCatErr.Error()) + "\n")
		var b strings.Builder
		b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("esc/q: back to menu"))
		return b.String()
	}

	if len(m.skillCat.Categories) == 0 {
		content.WriteString(mutedStyle.Render("No skills found in catalog.") + "\n")
		var b strings.Builder
		b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("esc/q: back to menu"))
		return b.String()
	}

	// Category tab bar.
	for i, cat := range m.skillCat.Categories {
		if i > 0 {
			content.WriteString("  ")
		}
		if i == m.skillCatCursor {
			content.WriteString(activeStyle.Render("[" + cat.Name + "]"))
		} else {
			content.WriteString(mutedStyle.Render(" " + cat.Name + " "))
		}
	}
	content.WriteString("\n\n")

	// Skills list for the selected category.
	currentCat := m.skillCat.Categories[m.skillCatCursor]
	for si, skill := range currentCat.Skills {
		if si == m.skillScrollIndex {
			content.WriteString(activeStyle.Render("> "+skill.Name) + "\n")
		} else {
			content.WriteString("  " + skill.Name + "\n")
		}
		content.WriteString(mutedStyle.Render("    "+skill.Description) + "\n")
	}

	// Install hint footer.
	content.WriteString("\n")
	installCmd := m.skillCat.InstallCommand
	if installCmd == "" {
		installCmd = "npx skills add -y"
	}
	browseURL := m.skillCat.BrowseURL
	if browseURL == "" {
		browseURL = "https://skills.sh"
	}
	selectedInstall := ""
	if len(currentCat.Skills) > 0 && m.skillScrollIndex < len(currentCat.Skills) {
		selectedInstall = " " + currentCat.Skills[m.skillScrollIndex].Install
	}
	content.WriteString(mutedStyle.Render(
		"Install: "+installCmd+selectedInstall+"  |  Browse more: "+browseURL,
	) + "\n")

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("tab/←/→: category  ↑/↓: skill  enter: install  esc/q: back"))
	return b.String()
}

// viewSkillInstallConfirm renders the install confirmation prompt for a
// community skill selected in the browser.
func (m Model) viewSkillInstallConfirm() string {
	var content strings.Builder
	content.WriteString(headingStyle.Render("Install community skill") + "\n\n")
	content.WriteString("Skill:   " + activeStyle.Render(m.pendingSkillName) + "\n")
	content.WriteString("Command: " + mutedStyle.Render(m.pendingSkillCmd) + "\n\n")

	if _, err := exec.LookPath("npx"); err != nil {
		content.WriteString(errorStyle.Render("npx not found on PATH.") + "\n")
		content.WriteString(mutedStyle.Render("Install Node.js (https://nodejs.org) and try again.") + "\n\n")
		content.WriteString(mutedStyle.Render("esc: back"))
	} else {
		content.WriteString(mutedStyle.Render("This will run the command above in the current directory.") + "\n\n")
		content.WriteString(mutedStyle.Render("y: install   n/esc: cancel"))
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	return b.String()
}
