package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const maxWidth = 100

var (
	headerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	nameStyle     = lipgloss.NewStyle().Bold(true)
	ageStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	dirtyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("12"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	infoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	keyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("8"))
	modalStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("5")).Padding(0, 1)
	modalTitle    = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	labelStyle    = lipgloss.NewStyle().Width(11)
	focusLabel    = labelStyle.Foreground(lipgloss.Color("5")).Bold(true)
	tagStyles     = map[string]lipgloss.Style{
		"main":    lipgloss.NewStyle().Foreground(lipgloss.Color("12")),
		"current": lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		"missing": lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		"locked":  lipgloss.NewStyle().Foreground(lipgloss.Color("13")),
	}
)

func (m *Model) View() string {
	if m.quitting {
		return ""
	}
	width := maxWidth
	if m.width > 0 {
		width = min(m.width, maxWidth)
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render(" gwm") + dimStyle.Render(" · worktree manager · "+filepath.Base(m.mainPath)) + "\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", width)) + "\n")

	var below []string
	if m.mode == modeFilter || m.filter != "" {
		line := infoStyle.Render(" Filter: ") + m.filter
		if m.mode == modeFilter {
			line += "█"
		}
		below = append(below, line)
	}
	switch m.mode {
	case modeCreate:
		below = append(below, m.createView(width))
	case modeConfirm:
		below = append(below, modalStyle.Width(min(width-2, 70)).Render(m.confirm.prompt+dimStyle.Render("  [y/n]")))
	}
	below = append(below, dimStyle.Render(strings.Repeat("─", width)), m.messageView(), m.keysView())
	footer := strings.Join(below, "\n")

	maxRows := 0
	if m.height > 0 {
		maxRows = max(m.height-2-lipgloss.Height(footer), 1)
	}
	b.WriteString(m.tableView(width, maxRows))
	b.WriteString(footer)
	return b.String()
}

func (m *Model) tableView(width, maxRows int) string {
	rows := m.visible()
	if len(rows) == 0 {
		return dimStyle.Render("  no worktrees match") + "\n"
	}

	nameW, branchW := 4, 6
	for _, r := range rows {
		nameW = max(nameW, lipgloss.Width(r.wt.Name))
		branchW = max(branchW, lipgloss.Width(branchLabel(r)))
	}
	const fixed = 2 + 2 + 2 + 2 + 5 + 2 + 16 // cursor, dirty, gaps, age, tags
	for nameW+branchW+fixed > width && (nameW > 12 || branchW > 12) {
		if branchW >= nameW {
			branchW--
		} else {
			nameW--
		}
	}

	start := 0
	if maxRows > 0 && len(rows) > maxRows {
		start = min(max(m.cursor-maxRows/2, 0), len(rows)-maxRows)
		rows = rows[start : start+maxRows]
	}

	var b strings.Builder
	for i, r := range rows {
		selected := start+i == m.cursor
		dirty, age := " ", ""
		if r.loaded {
			age = formatAge(r.lastCommit)
			if r.dirty {
				dirty = "●"
			}
		}
		cells := []struct {
			text  string
			width int
			style lipgloss.Style
		}{
			{" ", 2, lipgloss.NewStyle()},
			{dirty, 2, dirtyStyle},
			{truncate(r.wt.Name, nameW), nameW + 2, nameStyle},
			{truncate(branchLabel(r), branchW), branchW + 2, dimStyle},
			{age, 7, ageStyle},
		}
		if selected {
			cells[0].text = "▎"
		}
		if r.wt.Missing {
			cells[2].style = dimStyle
		}

		var line strings.Builder
		for _, c := range cells {
			padded := c.text + strings.Repeat(" ", max(c.width-lipgloss.Width(c.text), 0))
			if selected {
				line.WriteString(padded)
			} else {
				line.WriteString(c.style.Render(padded))
			}
		}
		for _, tag := range tags(r) {
			if selected {
				line.WriteString(tag + " ")
			} else {
				line.WriteString(tagStyles[tag].Render(tag) + " ")
			}
		}
		text := line.String()
		if selected {
			text = selectedStyle.Render(text + strings.Repeat(" ", max(width-lipgloss.Width(text), 0)))
		}
		b.WriteString(text + "\n")
	}
	return b.String()
}

func (m *Model) createView(width int) string {
	fields := []struct {
		label, placeholder string
	}{
		{"Branch", "required"},
		{"Directory", strings.ReplaceAll(string(m.inputs[fieldBranch]), "/", "-")},
	}
	lines := []string{modalTitle.Render("New worktree")}
	for i, f := range fields {
		label := labelStyle.Render(f.label)
		value := string(m.inputs[i])
		if i == m.focus {
			label = focusLabel.Render(f.label)
		}
		if value == "" && f.placeholder != "" {
			value = dimStyle.Render(f.placeholder)
		}
		if i == m.focus {
			value = string(m.inputs[i]) + "█"
			if len(m.inputs[i]) == 0 && f.placeholder != "" {
				value = "█" + dimStyle.Render(f.placeholder)
			}
		}
		lines = append(lines, label+value)
	}
	return modalStyle.Width(min(width-2, 70)).Render(strings.Join(lines, "\n"))
}

func (m *Model) messageView() string {
	switch {
	case m.message == "":
		return ""
	case m.isError:
		return " " + errorStyle.Render(m.message)
	case strings.HasPrefix(m.message, "Warning:"):
		return " " + warningStyle.Render(m.message)
	}
	return " " + infoStyle.Render(m.message)
}

func (m *Model) keysView() string {
	var keys [][2]string
	switch m.mode {
	case modeFilter:
		keys = [][2]string{{"type", "search"}, {"↑↓", "move"}, {"enter", "switch"}, {"esc", "clear"}}
	case modeCreate:
		keys = [][2]string{{"tab", "next field"}, {"enter", "create"}, {"esc", "cancel"}}
	case modeConfirm:
		keys = [][2]string{{"y", "yes"}, {"n", "no"}}
	default:
		keys = [][2]string{{"enter", "switch"}, {"n", "new"}, {"d", "delete"}, {"D", "force delete"}, {"/", "filter"}, {"r", "refresh"}, {"q", "quit"}}
	}
	var parts []string
	for _, k := range keys {
		parts = append(parts, keyStyle.Render(" "+k[0]+" ")+dimStyle.Render(" "+k[1]))
	}
	return " " + strings.Join(parts, "  ")
}

func branchLabel(r row) string {
	if r.wt.Branch != "" {
		return r.wt.Branch
	}
	head := r.wt.Head
	if len(head) > 7 {
		head = head[:7]
	}
	return "detached @" + head
}

func tags(r row) []string {
	var t []string
	if r.wt.Main {
		t = append(t, "main")
	}
	if r.wt.Current {
		t = append(t, "current")
	}
	if r.wt.Missing {
		t = append(t, "missing")
	}
	if r.wt.Locked {
		t = append(t, "locked")
	}
	return t
}

func truncate(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	return string(r[:max(width-1, 0)]) + "…"
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/24/7))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/24/30))
	}
	return fmt.Sprintf("%dy", int(d.Hours()/24/365))
}
