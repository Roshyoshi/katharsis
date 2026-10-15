package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// privateTmux starts a tmux server of the test's own and points $TMUX at
// it, which the session package's tmux calls follow when they name no
// socket. It returns a tmux command line bound to that server. The user's
// own server is never named: $TMUX_TMPDIR moves to a scratch dir too.
func privateTmux(t *testing.T) func(args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not on PATH")
	}
	// Not t.TempDir: macOS caps socket paths at 104 bytes.
	tmp, err := os.MkdirTemp("/tmp", "kqtmux")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	t.Setenv("TMUX_TMPDIR", tmp)
	t.Setenv("HOME", tmp)
	label := fmt.Sprintf("katharsis-cqa-%d", os.Getpid())
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("tmux", append([]string{"-L", label}, args...)...).Output()
		if err != nil {
			t.Fatalf("tmux %v: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}
	run("-f", "/dev/null", "new-session", "-d", "-s", "t", "-x", "120", "-y", "40", "sleep 300")
	// Registered after the Setenvs, so it runs before they are undone.
	t.Cleanup(func() { _ = exec.Command("tmux", "-L", label, "kill-server").Run() })
	t.Setenv("TMUX", run("display-message", "-p", "#{socket_path}")+",0,0")
	return run
}

// A door's editor pane goes when the door closes, also under tmux's
// remain-on-exit (some players set it globally), which used to keep vim's
// pane on screen dead after it quit.
func TestCloseRemovesTheEditorPane(t *testing.T) {
	tmux := privateTmux(t)
	vim, err := exec.LookPath("vim")
	if err != nil {
		t.Skip("vim not on PATH")
	}
	tmux("set-option", "-g", "remain-on-exit", "on")
	src := filepath.Join(t.TempDir(), "sol.cpp")
	if err := os.WriteFile(src, []byte("int main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gone := func(pane string) bool {
		return !strings.Contains(tmux("list-panes", "-a", "-F", "#{pane_id}")+"\n", pane+"\n")
	}
	for name, argv := range map[string][]string{
		"vim":        {vim, "-u", "NONE", "-i", "NONE", "-N", src},
		"exited vim": {"true"},
	} {
		pane := tmux(append([]string{"new-window", "-P", "-F", "#{pane_id}", "--"}, argv...)...)
		want := map[string]string{"vim": "sol.cpp", "exited vim": "Pane is dead"}[name]
		for deadline := time.Now().Add(5 * time.Second); !strings.Contains(tmux("capture-pane", "-p", "-t", pane), want); {
			if time.Now().After(deadline) {
				t.Fatalf("%s: pane never showed %q", name, want)
			}
			time.Sleep(20 * time.Millisecond)
		}
		r := &TmuxRunner{pane: pane}
		r.Shutdown()
		if !gone(pane) {
			t.Errorf("%s: pane %s still there after the door closed", name, pane)
		}
	}
}
