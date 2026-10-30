// Package project manages the worktrees of one git repository.
package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lleWelll/worktree-manager/internal/config"
	"github.com/lleWelll/worktree-manager/internal/git"
)

var (
	ErrNotRepo      = errors.New("not a git repository")
	ErrBareRepo     = errors.New("bare repositories are not supported")
	ErrMainWorktree = errors.New("the main worktree cannot be deleted")
	ErrLocked       = errors.New("worktree is locked; run `git worktree unlock <path>` first")
	ErrDirty        = errors.New("worktree has uncommitted changes")
)

// BranchInUseError reports a branch already checked out in another Worktree.
type BranchInUseError struct {
	Branch string
	Path   string
}

func (e *BranchInUseError) Error() string {
	return fmt.Sprintf("branch %q is already checked out in %s", e.Branch, e.Path)
}

// Project is a git repository with its worktrees.
type Project struct {
	MainPath      string
	WorktreesRoot string
	CurrentPath   string // the Current worktree; empty when gwm runs outside any worktree
	commonDir     string
}

// Worktree is a Worktree of the Project as shown to the user.
type Worktree struct {
	git.Worktree
	Name    string // Worktree directory name
	Main    bool
	Current bool
}

// Open finds the Project containing dir.
func Open(dir string, cfg config.Config) (*Project, error) {
	list, err := git.ListWorktrees(dir)
	if err != nil {
		if git.IsExitError(err) {
			return nil, ErrNotRepo
		}
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotRepo
	}
	if list[0].Bare {
		return nil, ErrBareRepo
	}
	commonDir, err := git.Run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	mainPath := realpath(list[0].Path)
	p := &Project{
		MainPath:      mainPath,
		WorktreesRoot: cfg.WorktreesRootFor(mainPath),
		commonDir:     commonDir,
	}
	if top, err := git.Run(dir, "rev-parse", "--show-toplevel"); err == nil {
		p.CurrentPath = realpath(top)
	}
	return p, nil
}

// Worktrees lists the Project's worktrees: the Main worktree first, the rest by name.
func (p *Project) Worktrees() ([]Worktree, error) {
	list, err := git.ListWorktrees(p.MainPath)
	if err != nil {
		return nil, err
	}
	result := make([]Worktree, 0, len(list))
	for i, wt := range list {
		result = append(result, Worktree{
			Worktree: wt,
			Name:     filepath.Base(wt.Path),
			Main:     i == 0,
			Current:  realpath(wt.Path) == p.CurrentPath,
		})
	}
	if len(result) > 1 {
		slices.SortFunc(result[1:], func(a, b Worktree) int { return strings.Compare(a.Name, b.Name) })
	}
	return result, nil
}

// Source is where the branch of a new Worktree comes from.
type Source int

const (
	SourceLocal  Source = iota // an existing local branch
	SourceRemote               // a new local branch tracking a remote one
	SourceNew                  // a new branch from the Start point
)

// CreatePlan describes a Worktree about to be created.
type CreatePlan struct {
	Branch     string
	Path       string
	Source     Source
	RemoteRef  string // for SourceRemote, e.g. origin/feat-x
	StartPoint string // for SourceNew, the Main worktree's branch (or short commit when detached)
}

// PlanCreate validates a request to create a Worktree for branch and decides where its branch comes from.
// An empty dirName defaults to the branch name with slashes replaced by dashes.
func (p *Project) PlanCreate(branch, dirName string) (CreatePlan, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return CreatePlan{}, errors.New("branch name is required")
	}
	if strings.HasPrefix(branch, "-") {
		return CreatePlan{}, fmt.Errorf("invalid branch name %q", branch)
	}
	if _, err := git.Run(p.MainPath, "check-ref-format", "--branch", branch); err != nil {
		return CreatePlan{}, fmt.Errorf("invalid branch name %q", branch)
	}

	dirName = strings.TrimSpace(dirName)
	if dirName == "" {
		dirName = strings.ReplaceAll(branch, "/", "-")
	}
	if strings.Contains(dirName, "/") || dirName == "." || dirName == ".." {
		return CreatePlan{}, fmt.Errorf("invalid directory name %q", dirName)
	}

	list, err := git.ListWorktrees(p.MainPath)
	if err != nil {
		return CreatePlan{}, err
	}
	for _, wt := range list {
		if wt.Branch == branch {
			return CreatePlan{}, &BranchInUseError{Branch: branch, Path: wt.Path}
		}
	}

	plan := CreatePlan{Branch: branch, Path: filepath.Join(p.WorktreesRoot, dirName)}
	if _, err := os.Lstat(plan.Path); err == nil {
		return CreatePlan{}, fmt.Errorf("directory %s already exists", plan.Path)
	}

	if _, err := git.Run(p.MainPath, "rev-parse", "--verify", "-q", "refs/heads/"+branch); err == nil {
		plan.Source = SourceLocal
		return plan, nil
	}
	if ref, err := p.remoteBranch(branch); err != nil {
		return CreatePlan{}, err
	} else if ref != "" {
		plan.Source = SourceRemote
		plan.RemoteRef = ref
		return plan, nil
	}
	plan.Source = SourceNew
	plan.StartPoint, err = p.mainHead()
	if err != nil {
		return CreatePlan{}, err
	}
	return plan, nil
}

// remoteBranch returns the remote-tracking ref for branch, preferring origin; empty if none.
func (p *Project) remoteBranch(branch string) (string, error) {
	out, err := git.Run(p.MainPath, "for-each-ref", "--format=%(refname:strip=2)", "refs/remotes")
	if err != nil {
		return "", err
	}
	var found string
	for ref := range strings.SplitSeq(out, "\n") {
		remote, name, _ := strings.Cut(ref, "/")
		if name != branch {
			continue
		}
		if remote == "origin" {
			return ref, nil
		}
		if found == "" {
			found = ref
		}
	}
	return found, nil
}

func (p *Project) mainHead() (string, error) {
	head, err := git.Run(p.MainPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || head != "HEAD" {
		return head, err
	}
	return git.Run(p.MainPath, "rev-parse", "--short", "HEAD")
}

// Create creates the Worktree described by plan.
func (p *Project) Create(plan CreatePlan) error {
	if err := p.ensureExcluded(); err != nil {
		return err
	}
	if err := os.MkdirAll(p.WorktreesRoot, 0o755); err != nil {
		return err
	}
	var args []string
	switch plan.Source {
	case SourceLocal:
		args = []string{"worktree", "add", plan.Path, plan.Branch}
	case SourceRemote:
		args = []string{"worktree", "add", "--track", "-b", plan.Branch, plan.Path, plan.RemoteRef}
	case SourceNew:
		// Run in the Main worktree, so HEAD is the Start point.
		args = []string{"worktree", "add", "-b", plan.Branch, plan.Path, "HEAD"}
	}
	_, err := git.Run(p.MainPath, args...)
	return err
}

// ensureExcluded hides a Worktrees root that lives inside the Main worktree via .git/info/exclude.
func (p *Project) ensureExcluded() error {
	rel, err := filepath.Rel(p.MainPath, p.WorktreesRoot)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return nil
	}
	entry := "/" + filepath.ToSlash(rel) + "/"
	path := filepath.Join(p.commonDir, "info", "exclude")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		entry = "\n" + entry
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(entry + "\n"); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Delete removes a Linked worktree, preserving its local branch.
// A Safe delete (force=false) refuses to lose uncommitted changes.
func (p *Project) Delete(wt Worktree, force bool) (warning string, err error) {
	switch {
	case wt.Main:
		return "", ErrMainWorktree
	case wt.Locked:
		return "", ErrLocked
	}

	if wt.Missing {
		_, err = git.Run(p.MainPath, "worktree", "prune")
	} else if force {
		_, err = git.Run(p.MainPath, "worktree", "remove", "--force", wt.Path)
	} else {
		_, err = git.Run(p.MainPath, "worktree", "remove", wt.Path)
		if err != nil && strings.Contains(err.Error(), "modified or untracked files") {
			err = ErrDirty
		}
	}
	return "", err
}

// Dirty reports whether the Worktree at path has uncommitted changes.
func Dirty(path string) (bool, error) {
	out, err := git.Run(path, "--no-optional-locks", "status", "--porcelain")
	return out != "", err
}

// LastCommit returns the time of the latest commit checked out in the Worktree at path.
func LastCommit(path string) (time.Time, error) {
	out, err := git.Run(path, "log", "-1", "--format=%ct")
	if err != nil {
		return time.Time{}, err
	}
	sec, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(sec, 0), nil
}

func realpath(path string) string {
	if p, err := filepath.EvalSymlinks(path); err == nil {
		return p
	}
	return path
}

// Branch is a local branch or a cached remote-tracking branch.
// Remote is empty for local branches.
type Branch struct {
	Name   string
	Remote string
}

// Branches lists local and remote-tracking branches without contacting remotes.
func (p *Project) Branches() ([]Branch, error) {
	out, err := git.Run(p.MainPath, "for-each-ref", "--sort=refname", "--format=%(refname)%09%(symref)", "refs/heads/", "refs/remotes/")
	if err != nil {
		return nil, err
	}
	var branches []Branch
	for line := range strings.SplitSeq(out, "\n") {
		ref, symbolic, _ := strings.Cut(line, "\t")
		if ref == "" || symbolic != "" {
			continue
		}
		if name, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
			branches = append(branches, Branch{Name: name})
		} else if ref, ok := strings.CutPrefix(ref, "refs/remotes/"); ok {
			remote, name, ok := strings.Cut(ref, "/")
			if ok {
				branches = append(branches, Branch{Name: name, Remote: remote})
			}
		}
	}
	return branches, nil
}
