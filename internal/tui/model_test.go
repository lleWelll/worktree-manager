package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"worktree-manager/internal/git"
	"worktree-manager/internal/project"
)

type fakeProject struct {
	list    []project.Worktree
	plan    project.CreatePlan
	planErr error
	created []project.CreatePlan
	deleted []string
	forced  []bool
}

func (f *fakeProject) Worktrees() ([]project.Worktree, error) { return f.list, nil }

func (f *fakeProject) PlanCreate(branch, dir string) (project.CreatePlan, error) {
	f.plan.Branch = branch
	return f.plan, f.planErr
}

func (f *fakeProject) Create(plan project.CreatePlan) error {
	f.created = append(f.created, plan)
	name := plan.Path[strings.LastIndex(plan.Path, "/")+1:]
	f.list = append(f.list, wt(name, plan.Branch))
	return nil
}

func (f *fakeProject) Delete(w project.Worktree, force bool) (string, error) {
	f.deleted = append(f.deleted, w.Name)
	f.forced = append(f.forced, force)
	var kept []project.Worktree
	for _, x := range f.list {
		if x.Path != w.Path {
			kept = append(kept, x)
		}
	}
	f.list = kept
	return "", nil
}

func wt(name, branch string) project.Worktree {
	return project.Worktree{Worktree: git.Worktree{Path: "/wt/" + name, Branch: branch}, Name: name}
}

func newFake() *fakeProject {
	main := wt("repo", "main")
	main.Path = "/repo"
	main.Main = true
	cur := wt("feat-b", "feat/b")
	cur.Current = true
	missing := wt("gone", "gone")
	missing.Missing = true
	return &fakeProject{list: []project.Worktree{main, wt("feat-a", "feat/a"), cur, missing}}
}

func start(t *testing.T, f *fakeProject) *Model {
	t.Helper()
	m := New(f, "/repo", func(string) (bool, time.Time, error) { return false, time.Time{}, nil })
	run(m, m.Init())
	return m
}

// run executes cmd and feeds resulting messages back into the model until none remain.
func run(m *Model, cmd tea.Cmd) {
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case nil:
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case tea.QuitMsg:
			m.quitting = true
		default:
			_, next := m.Update(msg)
			queue = append(queue, next)
		}
	}
}

func press(m *Model, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "backspace":
			msg = tea.KeyMsg{Type: tea.KeyBackspace}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		_, cmd := m.Update(msg)
		run(m, cmd)
	}
}

func TestCursorStartsOnCurrentWorktree(t *testing.T) {
	m := start(t, newFake())
	if got := m.selected().Name; got != "feat-b" {
		t.Fatalf("selected %q", got)
	}
}

func TestEnterSwitches(t *testing.T) {
	m := start(t, newFake())
	press(m, "k", "enter")
	if m.SwitchTo != "/wt/feat-a" || !m.quitting {
		t.Fatalf("SwitchTo=%q quitting=%v", m.SwitchTo, m.quitting)
	}
}

func TestEnterOnMissingDoesNotSwitch(t *testing.T) {
	m := start(t, newFake())
	press(m, "j", "enter")
	if m.SwitchTo != "" || m.quitting || !m.isError {
		t.Fatalf("SwitchTo=%q quitting=%v message=%q", m.SwitchTo, m.quitting, m.message)
	}
}

func TestQuitWithoutSwitch(t *testing.T) {
	m := start(t, newFake())
	press(m, "q")
	if m.SwitchTo != "" || !m.quitting {
		t.Fatalf("SwitchTo=%q quitting=%v", m.SwitchTo, m.quitting)
	}
}

func TestCreateNewBranchAsksConfirmation(t *testing.T) {
	f := newFake()
	f.plan = project.CreatePlan{Path: "/wt/feat-c", Source: project.SourceNew, StartPoint: "main"}
	m := start(t, f)
	press(m, "n", "f", "e", "a", "t", "/", "c", "enter")
	if m.mode != modeConfirm || len(f.created) != 0 {
		t.Fatalf("mode=%v created=%v", m.mode, f.created)
	}
	press(m, "y")
	if len(f.created) != 1 || f.created[0].Branch != "feat/c" {
		t.Fatalf("created=%v", f.created)
	}
	if m.mode != modeList || m.SwitchTo != "" || m.quitting {
		t.Fatalf("after create: mode=%v SwitchTo=%q", m.mode, m.SwitchTo)
	}
	if m.selected().Name != "feat-c" {
		t.Fatalf("selected %q, want the new worktree", m.selected().Name)
	}
}

func TestCreateDeclinedReturnsToForm(t *testing.T) {
	f := newFake()
	f.plan = project.CreatePlan{Path: "/wt/x", Source: project.SourceRemote, RemoteRef: "origin/x"}
	m := start(t, f)
	press(m, "n", "x", "enter", "n")
	if m.mode != modeCreate || string(m.inputs[0]) != "x" || len(f.created) != 0 {
		t.Fatalf("mode=%v input=%q created=%v", m.mode, string(m.inputs[0]), f.created)
	}
}

func TestCreateLocalBranchNeedsNoConfirmation(t *testing.T) {
	f := newFake()
	f.plan = project.CreatePlan{Path: "/wt/x", Source: project.SourceLocal}
	m := start(t, f)
	press(m, "n", "x", "enter")
	if len(f.created) != 1 {
		t.Fatalf("created=%v", f.created)
	}
}

func TestCreatePlanErrorStaysInForm(t *testing.T) {
	f := newFake()
	f.planErr = errors.New("branch in use")
	m := start(t, f)
	press(m, "n", "x", "enter")
	if m.mode != modeCreate || !m.isError || !strings.Contains(m.message, "branch in use") {
		t.Fatalf("mode=%v message=%q", m.mode, m.message)
	}
}

func TestDirectoryFieldViaTab(t *testing.T) {
	f := newFake()
	f.plan = project.CreatePlan{Path: "/wt/r", Source: project.SourceLocal}
	m := start(t, f)
	press(m, "n", "b", "tab", "r", "backspace", "d")
	if string(m.inputs[0]) != "b" || string(m.inputs[1]) != "d" {
		t.Fatalf("inputs=%q,%q", string(m.inputs[0]), string(m.inputs[1]))
	}
}

func TestSafeAndForceDelete(t *testing.T) {
	f := newFake()
	m := start(t, f)
	press(m, "k", "d")
	if m.mode != modeConfirm || len(f.deleted) != 0 {
		t.Fatalf("mode=%v deleted=%v", m.mode, f.deleted)
	}
	press(m, "y")
	press(m, "D", "y")
	if strings.Join(f.deleted, ",") != "feat-a,feat-b" || f.forced[0] || !f.forced[1] {
		t.Fatalf("deleted=%v forced=%v", f.deleted, f.forced)
	}
}

func TestDeleteMainRefused(t *testing.T) {
	f := newFake()
	m := start(t, f)
	press(m, "k", "k", "d")
	if m.mode != modeList || !m.isError || len(f.deleted) != 0 {
		t.Fatalf("mode=%v message=%q", m.mode, m.message)
	}
}

func TestDeletingCurrentSwitchesToMainOnQuit(t *testing.T) {
	m := start(t, newFake())
	press(m, "d", "y", "q")
	if m.SwitchTo != "/repo" {
		t.Fatalf("SwitchTo=%q", m.SwitchTo)
	}
}

func TestFilter(t *testing.T) {
	m := start(t, newFake())
	press(m, "/", "t", "-", "a")
	if got := len(m.visible()); got != 1 {
		t.Fatalf("visible=%d", got)
	}
	press(m, "enter")
	if m.SwitchTo != "/wt/feat-a" {
		t.Fatalf("SwitchTo=%q", m.SwitchTo)
	}
}

func TestFilterEscClears(t *testing.T) {
	m := start(t, newFake())
	press(m, "/", "z", "z", "esc")
	if m.mode != modeList || m.filter != "" || len(m.visible()) != 4 {
		t.Fatalf("mode=%v filter=%q visible=%d", m.mode, m.filter, len(m.visible()))
	}
}
