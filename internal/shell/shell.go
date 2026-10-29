// Package shell provides the Shell integration: a shell function that lets a
// Switch change the directory of the calling shell (see docs/adr/0001).
package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// CDFileEnv names the file gwm writes the Switch target into.
const CDFileEnv = "GWM_CD_FILE"

const (
	beginMarker = "# >>> gwm shell integration >>>"
	endMarker   = "# <<< gwm shell integration <<<"
)

// The same function works in zsh and bash.
const function = `gwm() {
  local gwm_cd_file gwm_status gwm_dir
  gwm_cd_file="$(mktemp "${TMPDIR:-/tmp}/gwm.XXXXXX")" || return 1
  GWM_CD_FILE="$gwm_cd_file" command gwm "$@"
  gwm_status=$?
  gwm_dir="$(cat -- "$gwm_cd_file" 2>/dev/null)"
  rm -f -- "$gwm_cd_file"
  if [ -n "$gwm_dir" ] && [ -d "$gwm_dir" ]; then
    cd -- "$gwm_dir" || return
  fi
  return $gwm_status
}
`

// Supported reports whether sh is a supported shell name.
func Supported(sh string) bool {
	return sh == "zsh" || sh == "bash"
}

// Script returns the shell function definition for sh.
func Script(sh string) (string, error) {
	if !Supported(sh) {
		return "", fmt.Errorf("unsupported shell %q (supported: zsh, bash)", sh)
	}
	return function, nil
}

// Detect returns the user's login shell name from $SHELL. Git Bash passes $SHELL
// to native programs as a Windows path ending in bash.exe.
func Detect() string {
	return strings.TrimSuffix(filepath.Base(os.Getenv("SHELL")), ".exe")
}

// RCFile returns the startup file the Shell integration is installed into.
func RCFile(sh, home string) (string, error) {
	return rcFile(sh, home, runtime.GOOS)
}

// rcFile is RCFile for the given GOOS. bash uses .bash_profile on macOS and in
// Git Bash on Windows, whose terminals start login shells, and .bashrc elsewhere.
func rcFile(sh, home, goos string) (string, error) {
	switch sh {
	case "zsh":
		if dir := os.Getenv("ZDOTDIR"); dir != "" {
			return filepath.Join(dir, ".zshrc"), nil
		}
		return filepath.Join(home, ".zshrc"), nil
	case "bash":
		if goos == "darwin" || goos == "windows" {
			return filepath.Join(home, ".bash_profile"), nil
		}
		return filepath.Join(home, ".bashrc"), nil
	}
	return "", fmt.Errorf("unsupported shell %q (supported: zsh, bash)", sh)
}

// Install appends the Shell integration to rcFile unless it is already there.
func Install(rcFile, sh string) (added bool, err error) {
	data, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	content := string(data)
	if strings.Contains(content, beginMarker) {
		return false, nil
	}
	block := fmt.Sprintf("%s\neval \"$(command gwm init %s)\"\n%s\n", beginMarker, sh, endMarker)
	if content != "" && !strings.HasSuffix(content, "\n") {
		block = "\n" + block
	}
	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}
	if _, err := f.WriteString(block); err != nil {
		f.Close()
		return false, err
	}
	return true, f.Close()
}
