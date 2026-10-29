package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathPrefersXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	t.Setenv("HOME", "/home/u")
	if got, want := Path(), "/xdg/worktree-manager/config.json"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPathFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/u")
	if got, want := Path(), "/home/u/.config/worktree-manager/config.json"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.WorktreesRootFor("/p"); got != "/p/.worktrees" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadInvalidJSONFails(t *testing.T) {
	path := writeConfig(t, `{"worktreesRoot": `)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error")
	}
}

func TestWorktreesRootResolution(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	path := writeConfig(t, `{
		"worktreesRoot": "../wt",
		"projects": {
			"/p/home": {"worktreesRoot": "~/trees/home"},
			"/p/abs": {"worktreesRoot": "/abs/trees"},
			"/p/empty": {}
		}
	}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"/p/other": "/p/wt",
		"/p/home":  "/home/u/trees/home",
		"/p/abs":   "/abs/trees",
		"/p/empty": "/p/wt",
	}
	for project, want := range cases {
		if got := cfg.WorktreesRootFor(project); got != want {
			t.Errorf("%s: got %q, want %q", project, got, want)
		}
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
