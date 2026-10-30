# Worktree Manager

A terminal tool (`gwm`) for listing, creating, switching between and deleting git worktrees of a single repository.

## Language

### Worktrees

**Project**:
The git repository whose worktrees are being managed, identified by its main worktree's path.
_Avoid_: Repo root, workspace

**Worktree**:
A working directory attached to the Project, as reported by git; one branch (or a detached commit) is checked out in it.
_Avoid_: Session, gw, checkout

**Main worktree**:
The Project's original working directory; it can never be deleted.
_Avoid_: Root worktree, primary

**Linked worktree**:
Any Worktree other than the Main worktree.

**Current worktree**:
The Worktree that contains the directory `gwm` was started from.

**Worktree directory name**:
The folder name of a Linked worktree inside the Worktrees root; defaults to the branch name when not given.
_Avoid_: Worktree name, alias

**Worktrees root**:
The directory under which new Linked worktrees are created for a Project.
_Avoid_: Worktrees dir, base dir

**Missing worktree**:
A Linked worktree git still knows about whose directory no longer exists.
_Avoid_: Prunable, stale

**Locked worktree**:
A Linked worktree that git protects from removal until it is unlocked.

**Start point**:
The commit a newly created branch begins from: the HEAD of the Main worktree.
_Avoid_: Base, base branch

### Actions

**Switch**:
Changing the user's shell working directory to a chosen Worktree.
_Avoid_: Resume, enter, checkout

**Shell integration**:
The shell function, installed in the user's shell startup file, that makes a Switch change the directory of the shell `gwm` was launched from.
_Avoid_: Wrapper, hook

**Safe delete**:
Removing a Linked worktree only if no uncommitted changes would be lost. Its local branch is preserved.

**Force delete**:
Removing a Linked worktree even when it has uncommitted changes. Its local branch is preserved.

**Dirty**:
A Worktree with uncommitted changes (modified, staged or untracked files).
