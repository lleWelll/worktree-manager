// Command gwm is a TUI for managing the git worktrees of a project.
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"worktree-manager/internal/config"
	"worktree-manager/internal/project"
	"worktree-manager/internal/shell"
	"worktree-manager/internal/tui"
)

const usage = `Usage:
  gwm                   open the worktree manager
  gwm init <zsh|bash>   print the shell integration function
  gwm install           add the shell integration to your shell startup file
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gwm:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return runTUI()
	}
	switch args[0] {
	case "init":
		if len(args) != 2 {
			return errors.New("usage: gwm init <zsh|bash>")
		}
		script, err := shell.Script(args[1])
		if err != nil {
			return err
		}
		fmt.Print(script)
		return nil
	case "install":
		return install()
	case "-h", "--help", "help":
		fmt.Print(usage)
		return nil
	}
	return fmt.Errorf("unknown command %q\n\n%s", args[0], usage)
}

func runTUI() error {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	proj, err := project.Open(cwd, cfg)
	if err != nil {
		return err
	}

	model := tui.New(proj, proj.MainPath, status)
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		return err
	}
	if model.SwitchTo == "" {
		return nil
	}
	return switchTo(model.SwitchTo)
}

func status(path string) (bool, time.Time, error) {
	dirty, err := project.Dirty(path)
	if err != nil {
		return false, time.Time{}, err
	}
	last, err := project.LastCommit(path)
	return dirty, last, err
}

// switchTo hands the target directory to the Shell integration, or prints it when
// the integration is not installed.
func switchTo(dir string) error {
	if file := os.Getenv(shell.CDFileEnv); file != "" {
		return os.WriteFile(file, []byte(dir), 0o600)
	}
	fmt.Fprintln(os.Stderr, "gwm: shell integration is not installed, so the directory was not changed; run `gwm install`.")
	fmt.Println(dir)
	return nil
}

func install() error {
	sh := shell.Detect()
	if !shell.Supported(sh) {
		return fmt.Errorf("unsupported shell %q (supported: zsh, bash); add `eval \"$(command gwm init <shell>)\"` to your startup file manually", sh)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	rc, err := shell.RCFile(sh, home)
	if err != nil {
		return err
	}
	added, err := shell.Install(rc, sh)
	if err != nil {
		return err
	}
	if !added {
		fmt.Printf("Shell integration is already installed in %s.\n", rc)
		return nil
	}
	fmt.Printf("Added shell integration to %s. Restart your shell or run: source %s\n", rc, rc)
	return nil
}
