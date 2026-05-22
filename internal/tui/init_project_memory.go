package tui

import (
	"fmt"
	"strings"
)

// viewInitProjectMemory renders the project-memory enable screen.
func (m Model) viewInitProjectMemory() string {
	var b strings.Builder
	b.WriteString(headingStyle.Render("Project Memory"))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render(
		"SquadAI can consult and propose additions to a project knowledge\n" +
			"base. Agents read it before research and the librarian sub-agent\n" +
			"suggests new entries at task end.",
	))
	b.WriteString("\n\n")

	mark := "[ ]"
	if m.projectMemoryEnabled {
		mark = activeStyle.Render("[x]")
	}
	b.WriteString(fmt.Sprintf("%s Enable project memory  %s\n",
		mark, mutedStyle.Render("(space to toggle)")))

	if m.projectMemoryEnabled {
		b.WriteString("\n")
		b.WriteString(headingStyle.Render("Memory folder") +
			mutedStyle.Render("  docs/memory/  (relative to repo root)") + "\n")
		if m.projectMemoryPathExists {
			b.WriteString(activeStyle.Render("  folder already exists — scaffold step will be skipped\n"))
		}
	}

	b.WriteString("\n" + mutedStyle.Render("[space] toggle   [enter] continue   [esc] back"))
	return m.renderPanel(b.String())
}

// viewInitProjectMemoryScaffold renders the conditional "scaffold folder?" prompt.
func (m Model) viewInitProjectMemoryScaffold() string {
	var b strings.Builder
	b.WriteString(headingStyle.Render("Scaffold Memory Folder?"))
	b.WriteString("\n\n")
	b.WriteString(headingStyle.Render("docs/memory/") + mutedStyle.Render(" doesn't exist yet.\n\n"))
	b.WriteString("SquadAI can scaffold it during apply with:\n")
	b.WriteString(mutedStyle.Render("  - README.md (explains the format)\n"))
	b.WriteString(mutedStyle.Render("  - _index.md       (auto-maintained)\n"))
	b.WriteString(mutedStyle.Render("  - _metadata.json  (auto-maintained)\n"))
	b.WriteString(mutedStyle.Render("  - _inbox/         (librarian-proposed drafts)\n"))
	b.WriteString("\n")

	yMark, nMark := "[ ]", "[ ]"
	if m.projectMemoryScaffold {
		yMark = activeStyle.Render("[x]")
	} else {
		nMark = activeStyle.Render("[x]")
	}
	b.WriteString(fmt.Sprintf("%s Yes, scaffold it during apply\n", yMark))
	b.WriteString(fmt.Sprintf("%s No, leave it (memory will report empty until I create files)\n", nMark))

	b.WriteString("\n" + mutedStyle.Render("[y/n] choose   [esc] back"))
	return m.renderPanel(b.String())
}
