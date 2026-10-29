# gwm

A terminal UI for listing, creating, switching between and deleting the git worktrees of a repository.

Inspired by [claude-worktree](https://github.com/bucket-robotics/claude-worktree).

## Install

```sh
go install ./cmd/gwm
gwm install   # add the shell integration to ~/.zshrc or ~/.bash_profile
```

The shell integration lets `gwm` change the directory of your current shell when you switch worktrees. Without it, `gwm` prints the target path instead.

## Usage

Run `gwm` inside a git repository.

| Key     | Action       |
|---------|--------------|
| `enter` | switch       |
| `n`     | new worktree |
| `d`     | delete       |
| `D`     | force delete |
| `/`     | filter       |
| `r`     | refresh      |
| `q`     | quit         |

## Configuration

Optional, at `$XDG_CONFIG_HOME/worktree-manager/config.json` (defaults to `~/.config/...`):

```json
{
  "worktreesRoot": ".worktrees",
  "projects": {
    "/path/to/repo": { "worktreesRoot": "~/worktrees/repo" }
  }
}
```

Relative paths are resolved against the repository's main worktree.
