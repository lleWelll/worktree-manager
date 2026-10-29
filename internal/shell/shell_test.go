package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestScriptUnknownShell(t *testing.T) {
	if _, err := Script("fish"); err == nil {
		t.Fatal("expected error for fish")
	}
}

// The function must cd the calling shell into whatever gwm writes to GWM_CD_FILE,
// and pass through gwm's exit status.
func TestScriptSwitchesDirectory(t *testing.T) {
	for _, sh := range []string{"zsh", "bash"} {
		t.Run(sh, func(t *testing.T) {
			if _, err := exec.LookPath(sh); err != nil {
				t.Skip(sh + " not installed")
			}
			target, _ := filepath.EvalSymlinks(t.TempDir())
			bin := t.TempDir()
			fake := "#!/bin/sh\nprintf '%s' \"$GWM_TARGET\" > \"$GWM_CD_FILE\"\nexit 3\n"
			if err := os.WriteFile(filepath.Join(bin, "gwm"), []byte(fake), 0o755); err != nil {
				t.Fatal(err)
			}
			script, err := Script(sh)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(sh, "-c", script+"\ngwm; echo \"status=$?\"; pwd")
			cmd.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin", "GWM_TARGET="+target)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			if got, want := string(out), "status=3\n"+target+"\n"; got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestRCFile(t *testing.T) {
	t.Setenv("ZDOTDIR", "")
	cases := []struct{ sh, goos, want string }{
		{"zsh", "darwin", "/h/.zshrc"},
		{"zsh", "linux", "/h/.zshrc"},
		{"bash", "darwin", "/h/.bash_profile"},
		{"bash", "windows", "/h/.bash_profile"},
		{"bash", "linux", "/h/.bashrc"},
	}
	for _, c := range cases {
		if got, err := rcFile(c.sh, "/h", c.goos); err != nil || got != c.want {
			t.Errorf("%s on %s: got %q, %v", c.sh, c.goos, got, err)
		}
	}
	t.Setenv("ZDOTDIR", "/z")
	if got, _ := rcFile("zsh", "/h", "linux"); got != "/z/.zshrc" {
		t.Errorf("ZDOTDIR: got %q", got)
	}
}

func TestDetect(t *testing.T) {
	for shellEnv, want := range map[string]string{
		"/bin/zsh":          "zsh",
		"/usr/bin/bash":     "bash",
		"/usr/bin/bash.exe": "bash",
		"":                  ".",
	} {
		t.Setenv("SHELL", shellEnv)
		if got := Detect(); got != want {
			t.Errorf("SHELL=%q: got %q, want %q", shellEnv, got, want)
		}
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	if err := os.WriteFile(rc, []byte("export FOO=1"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i, wantAdded := range []bool{true, false} {
		added, err := Install(rc, "zsh")
		if err != nil || added != wantAdded {
			t.Fatalf("run %d: added=%v err=%v", i, added, err)
		}
	}
	data, _ := os.ReadFile(rc)
	content := string(data)
	if !strings.HasPrefix(content, "export FOO=1\n") {
		t.Errorf("existing content damaged:\n%s", content)
	}
	if n := strings.Count(content, `eval "$(command gwm init zsh)"`); n != 1 {
		t.Errorf("eval line appears %d times:\n%s", n, content)
	}
}
