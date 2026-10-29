package git

import (
	"reflect"
	"testing"
)

func TestParseWorktreeList(t *testing.T) {
	out := `worktree /repo
HEAD 1111111111111111111111111111111111111111
branch refs/heads/main

worktree /repo/.worktrees/feat-a
HEAD 2222222222222222222222222222222222222222
branch refs/heads/feat/a
locked
prunable gitdir file points to non-existent location

worktree /repo/.worktrees/detached
HEAD 3333333333333333333333333333333333333333
detached
locked some reason

`
	got := parseWorktreeList(out)
	want := []Worktree{
		{Path: "/repo", Head: "1111111111111111111111111111111111111111", Branch: "main"},
		{Path: "/repo/.worktrees/feat-a", Head: "2222222222222222222222222222222222222222", Branch: "feat/a", Locked: true, Missing: true},
		{Path: "/repo/.worktrees/detached", Head: "3333333333333333333333333333333333333333", Locked: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestParseWorktreeListBare(t *testing.T) {
	got := parseWorktreeList("worktree /repo.git\nbare\n\n")
	want := []Worktree{{Path: "/repo.git", Bare: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
