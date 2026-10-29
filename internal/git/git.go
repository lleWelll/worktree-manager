// Package git is a thin wrapper over the git CLI.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Worktree is one entry of `git worktree list`.
type Worktree struct {
	Path    string
	Head    string
	Branch  string // short branch name; empty when HEAD is detached
	Bare    bool
	Locked  bool
	Missing bool // git reports the worktree as prunable
}

// Error carries git's stderr for a failed command.
type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	if e.Stderr != "" {
		msg := strings.TrimPrefix(e.Stderr, "fatal: ")
		return strings.TrimPrefix(msg, "error: ")
	}
	return fmt.Sprintf("git %s: %v", strings.Join(e.Args, " "), e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

// Run executes git in dir and returns trimmed stdout.
func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &Error{Args: args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return strings.TrimSpace(stdout.String()), nil
}

// IsExitError reports whether err is git exiting with a non-zero status
// (as opposed to git not being runnable at all).
func IsExitError(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}

// ListWorktrees returns the worktrees of the repository containing dir.
// The main worktree comes first, as git reports it.
func ListWorktrees(dir string) ([]Worktree, error) {
	out, err := Run(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktreeList(out), nil
}

func parseWorktreeList(out string) []Worktree {
	var list []Worktree
	var cur *Worktree
	for line := range strings.SplitSeq(out, "\n") {
		key, value, _ := strings.Cut(line, " ")
		if cur == nil && key != "worktree" {
			continue
		}
		switch key {
		case "worktree":
			list = append(list, Worktree{Path: value})
			cur = &list[len(list)-1]
		case "HEAD":
			cur.Head = value
		case "branch":
			cur.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "bare":
			cur.Bare = true
		case "locked":
			cur.Locked = true
		case "prunable":
			cur.Missing = true
		}
	}
	return list
}
