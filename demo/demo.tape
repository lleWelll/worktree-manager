# Records demo/demo.gif. Run from the repo root: vhs demo/demo.tape (needs gwm on PATH)
Output demo/demo.gif

Set Shell zsh
Set FontSize 16
Set Width 1000
Set Height 520
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 110ms

# Throwaway repo with three worktrees and the shell integration loaded, off-camera
Hide
Type `export PROMPT='%F{blue}%1~%f $ ' RPROMPT=''` Enter
Type `cd "$(mktemp -d)" && git init -q -b main shop && cd shop && echo .worktrees >> .git/info/exclude && git commit -q --allow-empty -m "Initial commit"` Enter
Type `for b in api-v2 feature-auth fix-payments; do git worktree add -q -b $b .worktrees/$b; done` Enter
Type `git -C .worktrees/api-v2 commit -q --allow-empty -m "Version the orders API"` Enter
Type `git -C .worktrees/feature-auth commit -q --allow-empty -m "Add login form"` Enter
Type `git -C .worktrees/fix-payments commit -q --allow-empty -m "Retry failed card payments" && touch .worktrees/fix-payments/notes.txt` Enter
Type `eval "$(command gwm init zsh)" && clear` Enter
Sleep 1s
Show

# Pick a worktree from the list
Type "gwm" Sleep 600ms Enter
Sleep 2.5s
Type "j" Sleep 900ms
Type "j" Sleep 1.5s
Enter
Sleep 1.5s
Type "git log --oneline -1" Sleep 500ms Enter
Sleep 2.5s

# Find one by typing
Type "gwm" Sleep 600ms Enter
Sleep 2.5s
Type "/" Sleep 800ms
Type "pay" Sleep 1.5s
Enter
Sleep 1.5s
Type "git status --short" Sleep 500ms Enter
Sleep 2.5s

# Back to the main worktree
Type "gwm" Sleep 600ms Enter
Sleep 2.5s
Type "k" Sleep 900ms
Type "k" Sleep 900ms
Type "k" Sleep 1.5s
Enter
Sleep 1.5s
Type "git branch --show-current" Sleep 500ms Enter
Sleep 3s
