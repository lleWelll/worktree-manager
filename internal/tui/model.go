// Package tui is gwm's interactive worktree list.
package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleWelll/worktree-manager/internal/project"
)

// Project is the part of project.Project the TUI drives.
type Project interface {
	Worktrees() ([]project.Worktree, error)
	PlanCreate(branch, dirName string) (project.CreatePlan, error)
	Create(plan project.CreatePlan) error
	Delete(wt project.Worktree, force bool) (warning string, err error)
}

// StatusFunc reports whether the Worktree at path is Dirty and when it was last committed to.
type StatusFunc func(path string) (dirty bool, lastCommit time.Time, err error)

type mode int

const (
	modeList mode = iota
	modeFilter
	modeCreate
	modeConfirm
)

const (
	fieldBranch = iota
	fieldDir
)

type row struct {
	wt         project.Worktree
	loaded     bool
	dirty      bool
	lastCommit time.Time
}

type confirmation struct {
	prompt string
	onYes  tea.Cmd
	onNo   mode
}

// Model is the bubbletea model of the worktree list.
type Model struct {
	proj     Project
	mainPath string
	status   StatusFunc

	rows       []row
	loadedOnce bool
	cursor     int    // index into visible()
	keepPath   string // select this Worktree after the next reload
	generation int    // discards status results from older reloads

	mode    mode
	filter  string
	inputs  [2][]rune
	focus   int
	confirm confirmation

	message string
	isError bool
	busy    bool

	width, height  int
	currentDeleted bool
	quitting       bool

	// SwitchTo is the directory to Switch to after the program exits; empty for none.
	SwitchTo string
}

// New creates the model for proj, whose Main worktree is at mainPath.
func New(proj Project, mainPath string, status StatusFunc) *Model {
	return &Model{proj: proj, mainPath: mainPath, status: status}
}

type loadedMsg struct {
	list []project.Worktree
	err  error
}

type statusMsg struct {
	generation int
	path       string
	dirty      bool
	lastCommit time.Time
	err        error
}

type plannedMsg struct {
	plan project.CreatePlan
	err  error
}

type createdMsg struct {
	plan project.CreatePlan
	err  error
}

type deletedMsg struct {
	wt      project.Worktree
	warning string
	err     error
}

func (m *Model) Init() tea.Cmd { return m.load() }

func (m *Model) load() tea.Cmd {
	return func() tea.Msg {
		list, err := m.proj.Worktrees()
		return loadedMsg{list, err}
	}
}

func (m *Model) fetchStatus(generation int, path string) tea.Cmd {
	return func() tea.Msg {
		dirty, last, err := m.status(path)
		return statusMsg{generation, path, dirty, last, err}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		return m, m.handleKey(msg)
	case loadedMsg:
		return m, m.handleLoaded(msg)
	case statusMsg:
		if msg.generation == m.generation && msg.err == nil {
			for i := range m.rows {
				if m.rows[i].wt.Path == msg.path {
					m.rows[i].loaded, m.rows[i].dirty, m.rows[i].lastCommit = true, msg.dirty, msg.lastCommit
				}
			}
		}
		return m, nil
	case plannedMsg:
		return m, m.handlePlanned(msg)
	case createdMsg:
		m.busy = false
		m.mode = modeList
		if msg.err != nil {
			m.setError(msg.err)
			return m, nil
		}
		m.setInfo("Created worktree " + filepath.Base(msg.plan.Path))
		m.keepPath = msg.plan.Path
		return m, m.load()
	case deletedMsg:
		m.busy = false
		switch {
		case errors.Is(msg.err, project.ErrDirty):
			m.setError(fmt.Errorf("%w; press D to force delete", msg.err))
			return m, nil
		case msg.err != nil:
			m.setError(msg.err)
			return m, nil
		case msg.warning != "":
			m.setWarning(msg.warning)
		default:
			m.setInfo("Deleted worktree " + msg.wt.Name)
		}
		if msg.wt.Current {
			m.currentDeleted = true
		}
		return m, m.load()
	}
	return m, nil
}

func (m *Model) handleLoaded(msg loadedMsg) tea.Cmd {
	if msg.err != nil {
		m.setError(msg.err)
		return nil
	}
	keep := m.keepPath
	if keep == "" && m.loadedOnce {
		keep = m.selected().Path
	}
	m.keepPath = ""
	m.generation++
	m.rows = make([]row, len(msg.list))
	cmds := make([]tea.Cmd, 0, len(msg.list))
	for i, wt := range msg.list {
		m.rows[i] = row{wt: wt}
		if !wt.Missing {
			cmds = append(cmds, m.fetchStatus(m.generation, wt.Path))
		}
	}

	m.cursor = min(m.cursor, max(len(m.visible())-1, 0))
	for i, r := range m.visible() {
		if (!m.loadedOnce && r.wt.Current) || (keep != "" && r.wt.Path == keep) {
			m.cursor = i
		}
	}
	m.loadedOnce = true
	return tea.Batch(cmds...)
}

func (m *Model) handlePlanned(msg plannedMsg) tea.Cmd {
	if msg.err != nil {
		m.busy = false
		m.setError(msg.err)
		return nil
	}
	plan := msg.plan
	if plan.Source != project.SourceLocal {
		m.busy = false
	}
	switch plan.Source {
	case project.SourceRemote:
		m.ask(fmt.Sprintf("Branch %q exists on %s. Create a local branch tracking it?", plan.Branch, remoteName(plan.RemoteRef)),
			m.create(plan), modeCreate)
		return nil
	case project.SourceNew:
		m.ask(fmt.Sprintf("Branch %q does not exist. Create it from %s?", plan.Branch, plan.StartPoint),
			m.create(plan), modeCreate)
		return nil
	}
	m.setInfo("Working…")
	return m.create(plan)
}

func (m *Model) create(plan project.CreatePlan) tea.Cmd {
	return func() tea.Msg {
		return createdMsg{plan, m.proj.Create(plan)}
	}
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	if msg.String() == "ctrl+c" {
		return m.quit()
	}
	if m.busy {
		return nil
	}
	switch m.mode {
	case modeFilter:
		return m.handleFilterKey(msg)
	case modeCreate:
		return m.handleCreateKey(msg)
	case modeConfirm:
		return m.handleConfirmKey(msg)
	}
	return m.handleListKey(msg)
}

func (m *Model) handleListKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "q", "esc":
		return m.quit()
	case "j", "down", "ctrl+n":
		m.move(1)
	case "k", "up", "ctrl+p":
		m.move(-1)
	case "enter":
		return m.switchToSelected()
	case "n":
		m.mode = modeCreate
		m.inputs = [2][]rune{}
		m.focus = fieldBranch
		m.clearMessage()
	case "/":
		m.mode = modeFilter
	case "r":
		m.clearMessage()
		return m.load()
	case "d":
		m.askDelete(false)
	case "D":
		m.askDelete(true)
	}
	return nil
}

func (m *Model) handleFilterKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.filter = ""
		m.mode = modeList
		m.cursor = 0
	case tea.KeyEnter:
		return m.switchToSelected()
	case tea.KeyDown, tea.KeyCtrlN:
		m.move(1)
	case tea.KeyUp, tea.KeyCtrlP:
		m.move(-1)
	case tea.KeyBackspace:
		m.filter = dropLast(m.filter)
		m.cursor = 0
	case tea.KeyRunes, tea.KeySpace:
		m.filter += string(msg.Runes)
		m.cursor = 0
	}
	return nil
}

func (m *Model) handleCreateKey(msg tea.KeyMsg) tea.Cmd {
	in := &m.inputs[m.focus]
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeList
		m.clearMessage()
	case tea.KeyTab, tea.KeyShiftTab, tea.KeyUp, tea.KeyDown:
		m.focus = 1 - m.focus
	case tea.KeyEnter:
		m.busy = true
		branch, dir := string(m.inputs[fieldBranch]), string(m.inputs[fieldDir])
		return func() tea.Msg {
			plan, err := m.proj.PlanCreate(branch, dir)
			return plannedMsg{plan, err}
		}
	case tea.KeyBackspace:
		if len(*in) > 0 {
			*in = (*in)[:len(*in)-1]
		}
	case tea.KeyCtrlU:
		*in = nil
	case tea.KeyRunes:
		// Pasted text arrives as runes too; keep only its first line.
		text, _, _ := strings.Cut(string(msg.Runes), "\n")
		*in = append(*in, []rune(text)...)
	}
	return nil
}

func (m *Model) handleConfirmKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "Y":
		m.mode = modeList
		m.busy = true
		m.setInfo("Working…")
		return m.confirm.onYes
	case "n", "N", "esc":
		m.mode = m.confirm.onNo
		m.clearMessage()
	}
	return nil
}

func (m *Model) ask(prompt string, onYes tea.Cmd, onNo mode) {
	m.confirm = confirmation{prompt: prompt, onYes: onYes, onNo: onNo}
	m.mode = modeConfirm
}

func (m *Model) askDelete(force bool) {
	r, ok := m.selectedRow()
	if !ok {
		return
	}
	wt := r.wt
	switch {
	case wt.Main:
		m.setError(project.ErrMainWorktree)
		return
	case wt.Locked:
		m.setError(project.ErrLocked)
		return
	}
	what := "worktree " + wt.Name
	if wt.Branch != "" {
		what += fmt.Sprintf(" and branch %q", wt.Branch)
	}
	prompt := "Delete " + what + "?"
	if force {
		prompt = "Force delete " + what + "? Uncommitted changes and unmerged commits will be lost."
	}
	m.clearMessage()
	m.ask(prompt, func() tea.Msg {
		warning, err := m.proj.Delete(wt, force)
		return deletedMsg{wt, warning, err}
	}, modeList)
}

func (m *Model) switchToSelected() tea.Cmd {
	r, ok := m.selectedRow()
	if !ok {
		return nil
	}
	if r.wt.Missing {
		m.setError(fmt.Errorf("directory %s no longer exists", r.wt.Path))
		return nil
	}
	m.SwitchTo = r.wt.Path
	m.quitting = true
	return tea.Quit
}

func (m *Model) quit() tea.Cmd {
	if m.currentDeleted {
		m.SwitchTo = m.mainPath
	}
	m.quitting = true
	return tea.Quit
}

// visible returns the rows matching the filter.
func (m *Model) visible() []row {
	if m.filter == "" {
		return m.rows
	}
	q := strings.ToLower(m.filter)
	var out []row
	for _, r := range m.rows {
		if strings.Contains(strings.ToLower(r.wt.Name), q) || strings.Contains(strings.ToLower(r.wt.Branch), q) {
			out = append(out, r)
		}
	}
	return out
}

func (m *Model) selectedRow() (row, bool) {
	v := m.visible()
	if m.cursor < 0 || m.cursor >= len(v) {
		return row{}, false
	}
	return v[m.cursor], true
}

func (m *Model) selected() project.Worktree {
	r, _ := m.selectedRow()
	return r.wt
}

func (m *Model) move(delta int) {
	if n := len(m.visible()); n > 0 {
		m.cursor = (m.cursor + delta + n) % n
	}
}

func (m *Model) setInfo(s string)    { m.message, m.isError = s, false }
func (m *Model) setWarning(s string) { m.message, m.isError = "Warning: "+s, false }
func (m *Model) setError(err error)  { m.message, m.isError = "Error: "+err.Error(), true }
func (m *Model) clearMessage()       { m.message, m.isError = "", false }

func remoteName(ref string) string {
	name, _, _ := strings.Cut(ref, "/")
	return name
}

func dropLast(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}
