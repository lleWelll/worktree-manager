package project

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"worktree-manager/internal/config"
)

func TestOpenFindsMainFromAnywhere(t *testing.T) {
	main := newRepo(t)
	sub := filepath.Join(main, "a", "b")
	mustMkdir(t, sub)
	linked := filepath.Join(t.TempDir(), "linked")
	gitIn(t, main, "worktree", "add", "-b", "linked", linked)

	for _, dir := range []string{main, sub, linked} {
		p, err := Open(dir, config.Config{})
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		if p.MainPath != main {
			t.Errorf("%s: MainPath = %q, want %q", dir, p.MainPath, main)
		}
		if p.WorktreesRoot != filepath.Join(main, ".worktrees") {
			t.Errorf("%s: WorktreesRoot = %q", dir, p.WorktreesRoot)
		}
	}

	p, _ := Open(sub, config.Config{})
	if p.CurrentPath != main {
		t.Errorf("CurrentPath from subdir = %q, want %q", p.CurrentPath, main)
	}
	p, _ = Open(linked, config.Config{})
	if p.CurrentPath != mustRealpath(t, linked) {
		t.Errorf("CurrentPath from linked = %q, want %q", p.CurrentPath, linked)
	}
}

func TestOpenOutsideRepo(t *testing.T) {
	if _, err := Open(t.TempDir(), config.Config{}); !errors.Is(err, ErrNotRepo) {
		t.Fatalf("err = %v, want ErrNotRepo", err)
	}
}

func TestOpenBareRepo(t *testing.T) {
	dir := tempDir(t)
	gitIn(t, dir, "init", "-q", "--bare")
	if _, err := Open(dir, config.Config{}); !errors.Is(err, ErrBareRepo) {
		t.Fatalf("err = %v, want ErrBareRepo", err)
	}
}

func TestWorktreesOrderAndFlags(t *testing.T) {
	main := newRepo(t)
	p := open(t, main)
	createBranch(t, p, "zeta")
	createBranch(t, p, "alpha")
	createBranch(t, p, "feat/mid")
	gitIn(t, main, "worktree", "lock", filepath.Join(p.WorktreesRoot, "zeta"))
	mustRemoveAll(t, filepath.Join(p.WorktreesRoot, "alpha"))

	list := worktrees(t, p)
	var names []string
	for _, wt := range list {
		names = append(names, wt.Name)
	}
	if got, want := strings.Join(names, ","), filepath.Base(main)+",alpha,feat-mid,zeta"; got != want {
		t.Fatalf("order = %s, want %s", got, want)
	}
	if !list[0].Main || !list[0].Current || list[0].Branch != "main" {
		t.Errorf("main entry = %+v", list[0])
	}
	if !list[1].Missing {
		t.Errorf("alpha should be missing: %+v", list[1])
	}
	if !list[3].Locked {
		t.Errorf("zeta should be locked: %+v", list[3])
	}
	if list[2].Branch != "feat/mid" || list[2].Main || list[2].Current {
		t.Errorf("feat-mid entry = %+v", list[2])
	}
}

func TestPlanCreateNewBranch(t *testing.T) {
	p := open(t, newRepo(t))
	plan, err := p.PlanCreate(" feat/x ", "")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Source != SourceNew || plan.Branch != "feat/x" || plan.StartPoint != "main" {
		t.Errorf("plan = %+v", plan)
	}
	if plan.Path != filepath.Join(p.WorktreesRoot, "feat-x") {
		t.Errorf("path = %q", plan.Path)
	}
}

func TestPlanCreateCustomDirName(t *testing.T) {
	p := open(t, newRepo(t))
	plan, err := p.PlanCreate("feat/x", "review")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Path != filepath.Join(p.WorktreesRoot, "review") {
		t.Errorf("path = %q", plan.Path)
	}
}

func TestPlanCreateLocalBranch(t *testing.T) {
	main := newRepo(t)
	gitIn(t, main, "branch", "existing")
	plan, err := open(t, main).PlanCreate("existing", "")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Source != SourceLocal {
		t.Errorf("source = %v", plan.Source)
	}
}

func TestPlanCreateRemoteBranch(t *testing.T) {
	main := newRepo(t)
	addRemoteBranch(t, main, "origin", "feat/remote")
	plan, err := open(t, main).PlanCreate("feat/remote", "")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Source != SourceRemote || plan.RemoteRef != "origin/feat/remote" {
		t.Errorf("plan = %+v", plan)
	}
}

func TestPlanCreateRejects(t *testing.T) {
	main := newRepo(t)
	p := open(t, main)
	createBranch(t, p, "taken")
	mustMkdir(t, filepath.Join(p.WorktreesRoot, "occupied"))

	if _, err := p.PlanCreate("", ""); err == nil {
		t.Error("empty branch accepted")
	}
	if _, err := p.PlanCreate("bad..name", ""); err == nil {
		t.Error("invalid branch accepted")
	}
	if _, err := p.PlanCreate("ok", "a/b"); err == nil {
		t.Error("dir name with slash accepted")
	}
	if _, err := p.PlanCreate("ok", ".."); err == nil {
		t.Error("dir name .. accepted")
	}
	if _, err := p.PlanCreate("occupied", ""); err == nil {
		t.Error("existing directory accepted")
	}
	var inUse *BranchInUseError
	if _, err := p.PlanCreate("main", ""); !errors.As(err, &inUse) || inUse.Path != main {
		t.Errorf("main branch: err = %v", err)
	}
	if _, err := p.PlanCreate("taken", "other"); !errors.As(err, &inUse) {
		t.Errorf("taken branch: err = %v", err)
	}
}

func TestCreateNewBranchStartsFromMainHead(t *testing.T) {
	main := newRepo(t)
	p := open(t, main)
	createBranch(t, p, "side")
	side := filepath.Join(p.WorktreesRoot, "side")
	commit(t, side, "side.txt")

	// Running from the side worktree must still branch off main's HEAD.
	p = open(t, side)
	createBranch(t, p, "fresh")
	fresh := filepath.Join(p.WorktreesRoot, "fresh")
	if got, want := gitIn(t, fresh, "rev-parse", "HEAD"), gitIn(t, main, "rev-parse", "HEAD"); got != want {
		t.Fatalf("fresh HEAD = %s, want main HEAD %s", got, want)
	}
}

func TestCreateExcludesRootInsideProjectOnce(t *testing.T) {
	main := newRepo(t)
	p := open(t, main)
	createBranch(t, p, "one")
	createBranch(t, p, "two")

	exclude := readFile(t, filepath.Join(main, ".git", "info", "exclude"))
	if n := strings.Count(exclude, "/.worktrees/"); n != 1 {
		t.Fatalf("exclude has %d entries:\n%s", n, exclude)
	}
	if status := gitIn(t, main, "status", "--porcelain"); status != "" {
		t.Fatalf("main worktree not clean:\n%s", status)
	}
}

func TestCreateOutsideProjectLeavesExcludeAlone(t *testing.T) {
	main := newRepo(t)
	outside := tempDir(t)
	p, err := Open(main, config.Config{WorktreesRoot: outside})
	if err != nil {
		t.Fatal(err)
	}
	before := readFile(t, filepath.Join(main, ".git", "info", "exclude"))
	createBranch(t, p, "x")
	if _, err := os.Stat(filepath.Join(outside, "x")); err != nil {
		t.Fatal(err)
	}
	if after := readFile(t, filepath.Join(main, ".git", "info", "exclude")); after != before {
		t.Fatalf("exclude changed:\n%s", after)
	}
}

func TestCreateFromRemoteTracks(t *testing.T) {
	main := newRepo(t)
	addRemoteBranch(t, main, "origin", "feat/remote")
	p := open(t, main)
	createBranch(t, p, "feat/remote")
	if got := gitIn(t, main, "rev-parse", "--abbrev-ref", "feat/remote@{upstream}"); got != "origin/feat/remote" {
		t.Fatalf("upstream = %q", got)
	}
}

func TestSafeDeleteMergedRemovesBranch(t *testing.T) {
	main := newRepo(t)
	p := open(t, main)
	createBranch(t, p, "done")
	warning, err := p.Delete(find(t, p, "done"), false)
	if err != nil || warning != "" {
		t.Fatalf("warning=%q err=%v", warning, err)
	}
	assertGone(t, p, "done")
	if branchExists(t, main, "done") {
		t.Error("branch kept")
	}
}

func TestSafeDeleteUnmergedKeepsBranch(t *testing.T) {
	main := newRepo(t)
	p := open(t, main)
	createBranch(t, p, "wip")
	commit(t, filepath.Join(p.WorktreesRoot, "wip"), "wip.txt")
	warning, err := p.Delete(find(t, p, "wip"), false)
	if err != nil || warning == "" {
		t.Fatalf("warning=%q err=%v", warning, err)
	}
	assertGone(t, p, "wip")
	if !branchExists(t, main, "wip") {
		t.Error("unmerged branch deleted")
	}
}

func TestSafeDeleteDirtyFails(t *testing.T) {
	p := open(t, newRepo(t))
	createBranch(t, p, "dirty")
	writeFile(t, filepath.Join(p.WorktreesRoot, "dirty", "new.txt"))
	if _, err := p.Delete(find(t, p, "dirty"), false); err == nil {
		t.Fatal("dirty worktree deleted")
	}
	find(t, p, "dirty")
}

func TestForceDeleteDirtyUnmerged(t *testing.T) {
	main := newRepo(t)
	p := open(t, main)
	createBranch(t, p, "mess")
	dir := filepath.Join(p.WorktreesRoot, "mess")
	commit(t, dir, "a.txt")
	writeFile(t, filepath.Join(dir, "b.txt"))
	if _, err := p.Delete(find(t, p, "mess"), true); err != nil {
		t.Fatal(err)
	}
	assertGone(t, p, "mess")
	if branchExists(t, main, "mess") {
		t.Error("branch kept")
	}
}

func TestDeleteRefusesMainAndLocked(t *testing.T) {
	main := newRepo(t)
	p := open(t, main)
	if _, err := p.Delete(worktrees(t, p)[0], true); !errors.Is(err, ErrMainWorktree) {
		t.Errorf("main: err = %v", err)
	}
	createBranch(t, p, "locked")
	gitIn(t, main, "worktree", "lock", filepath.Join(p.WorktreesRoot, "locked"))
	if _, err := p.Delete(find(t, p, "locked"), true); !errors.Is(err, ErrLocked) {
		t.Errorf("locked: err = %v", err)
	}
}

func TestDeleteMissingPrunes(t *testing.T) {
	main := newRepo(t)
	p := open(t, main)
	createBranch(t, p, "gone")
	mustRemoveAll(t, filepath.Join(p.WorktreesRoot, "gone"))
	if _, err := p.Delete(find(t, p, "gone"), false); err != nil {
		t.Fatal(err)
	}
	assertGone(t, p, "gone")
	if branchExists(t, main, "gone") {
		t.Error("branch kept")
	}
}

func TestDirtyAndLastCommit(t *testing.T) {
	main := newRepo(t)
	if dirty, err := Dirty(main); err != nil || dirty {
		t.Fatalf("clean repo: dirty=%v err=%v", dirty, err)
	}
	writeFile(t, filepath.Join(main, "x.txt"))
	if dirty, err := Dirty(main); err != nil || !dirty {
		t.Fatalf("dirty repo: dirty=%v err=%v", dirty, err)
	}
	if ts, err := LastCommit(main); err != nil || ts.IsZero() {
		t.Fatalf("last commit: %v %v", ts, err)
	}
}

// --- helpers ---

func newRepo(t *testing.T) string {
	t.Helper()
	dir := tempDir(t)
	gitIn(t, dir, "init", "-q", "-b", "main")
	commit(t, dir, "README")
	return dir
}

func open(t *testing.T, dir string) *Project {
	t.Helper()
	p, err := Open(dir, config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func createBranch(t *testing.T, p *Project, branch string) {
	t.Helper()
	plan, err := p.PlanCreate(branch, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Create(plan); err != nil {
		t.Fatal(err)
	}
}

func worktrees(t *testing.T, p *Project) []Worktree {
	t.Helper()
	list, err := p.Worktrees()
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func find(t *testing.T, p *Project, name string) Worktree {
	t.Helper()
	for _, wt := range worktrees(t, p) {
		if wt.Name == name {
			return wt
		}
	}
	t.Fatalf("worktree %q not found", name)
	return Worktree{}
}

func assertGone(t *testing.T, p *Project, name string) {
	t.Helper()
	for _, wt := range worktrees(t, p) {
		if wt.Name == name {
			t.Fatalf("worktree %q still listed", name)
		}
	}
	if _, err := os.Stat(filepath.Join(p.WorktreesRoot, name)); !os.IsNotExist(err) {
		t.Fatalf("directory %q still exists", name)
	}
}

func addRemoteBranch(t *testing.T, repo, remote, branch string) {
	t.Helper()
	bare := filepath.Join(tempDir(t), "remote.git")
	gitIn(t, repo, "clone", "-q", "--bare", repo, bare)
	gitIn(t, bare, "branch", branch, "main")
	gitIn(t, repo, "remote", "add", remote, bare)
	gitIn(t, repo, "fetch", "-q", remote)
}

func branchExists(t *testing.T, repo, branch string) bool {
	t.Helper()
	return exec.Command("git", "-C", repo, "rev-parse", "--verify", "-q", "refs/heads/"+branch).Run() == nil
}

func commit(t *testing.T, dir, file string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, file))
	gitIn(t, dir, "add", file)
	gitIn(t, dir, "commit", "-q", "-m", "add "+file)
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func tempDir(t *testing.T) string {
	t.Helper()
	return mustRealpath(t, t.TempDir())
}

func mustRealpath(t *testing.T, path string) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(path), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustRemoveAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
}
