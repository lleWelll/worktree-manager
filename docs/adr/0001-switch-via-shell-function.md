# Switch via a shell function, not a nested shell

A child process cannot change its parent shell's working directory, so a Switch needs help from the shell itself. We decided to ship a shell function (`gwm init zsh|bash`, installed by `gwm install`) that runs the binary and then `cd`s into the chosen Worktree in the *same* shell; the binary reports the target directory through a temp file named by `GWM_CD_FILE`, leaving stdout free for the TUI.

## Considered Options

- **`exec $SHELL` in the Worktree** (what `claude-worktree` does): zero setup, but every Switch nests a new shell inside the old one. `exit` returns to the previous Worktree instead of closing the tab, in-memory shell state (unexported variables, ad-hoc aliases, activated venv/nvm, jobs, `cd -`) is lost, and startup files are re-read on each Switch.
- **Printing the path on stdout for `cd "$(gwm)"`**: conflicts with the TUI and with subcommands that also write to stdout.

## Consequences

Without Shell integration a Switch cannot change directory; `gwm` then prints the path and suggests `gwm install` instead of silently falling back to a nested shell.
