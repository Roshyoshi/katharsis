package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestOverlayScript(t *testing.T) {
	o := Overlay{Ctl: []string{"/opt/it's here/katharsis", "ctl"}}
	s := o.script()
	if !strings.Contains(s, `let s:ctl = ['/opt/it''s here/katharsis', 'ctl']`) {
		t.Errorf("ctl argv not quoted for vim:\n%s", s)
	}
	for _, cmd := range []string{"Submit", "Debug", "Quack", "Retreat", "GiveUp"} {
		if !strings.Contains(s, "command! -bar "+cmd+" ") && !strings.Contains(s, "-nargs=? "+cmd+" ") {
			t.Errorf("no :%s in the overlay", cmd)
		}
	}
	if strings.Contains(s, "cnoreabbrev") {
		t.Error(":q! is rewritten without QuitGivesUp")
	}
	if o.QuitGivesUp = true; !strings.Contains(o.script(), "cnoreabbrev") {
		t.Error(":q! not rewritten with QuitGivesUp")
	}
}

func TestEditorArgv(t *testing.T) {
	for _, tc := range []struct {
		editor []string
		want   []string
	}{
		{[]string{"vim"}, []string{"env", "KATHARSIS_SOCK=/s", "vim", "-S", "/o.vim"}},
		{[]string{"/usr/local/bin/vim", "-p"}, []string{"env", "KATHARSIS_SOCK=/s", "/usr/local/bin/vim", "-p", "-S", "/o.vim"}},
		{[]string{"nvim"}, []string{"env", "KATHARSIS_SOCK=/s", "nvim"}},
		{[]string{"code", "-w"}, []string{"env", "KATHARSIS_SOCK=/s", "code", "-w"}},
	} {
		if got := EditorArgv(tc.editor, "/s", "/o.vim"); !slices.Equal(got, tc.want) {
			t.Errorf("EditorArgv(%q) = %q, want %q", tc.editor, got, tc.want)
		}
	}
}

// realVim is the user's vim, which must have +job for the overlay.
func realVim(t *testing.T) string {
	t.Helper()
	vim, err := exec.LookPath("vim")
	if err != nil {
		t.Skip("vim not on PATH")
	}
	out, err := exec.Command(vim, "--version").Output()
	if err != nil || !strings.Contains(string(out), "+job") {
		t.Skipf("%s has no +job", vim)
	}
	return vim
}

// startVim opens path in the real vim, with no vimrc but the overlay, in a
// new window of the private tmux server, and waits for it to draw. The
// overlay's ctl is this test binary (see TestMain).
func startVim(t *testing.T, o Overlay, sock, path string) string {
	t.Helper()
	vim := realVim(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	o.Ctl = []string{self, "ctl"}
	ov := filepath.Join(sockDir(t), "o.vim")
	if err := o.Write(ov); err != nil {
		t.Fatal(err)
	}
	argv := EditorArgv([]string{vim, "-u", "NONE", "-i", "NONE", "-N"}, sock, ov)
	argv = append(append([]string{"env", ctlEnv + "=1"}, argv...), path)
	id, err := tmux(append([]string{"new-window", "-P", "-F", "#{pane_id}", "-c", filepath.Dir(path), "--"}, argv...)...)
	if err != nil {
		t.Fatal(err)
	}
	waitScreen(t, id, filepath.Base(path))
	return id
}

func capture(t *testing.T, pane string) string {
	t.Helper()
	out, err := tmux("capture-pane", "-p", "-t", pane)
	if err != nil {
		return ""
	}
	return out
}

// waitScreen waits until pane shows want.
func waitScreen(t *testing.T, pane, want string) {
	t.Helper()
	for deadline := time.Now().Add(patience); !strings.Contains(capture(t, pane), want); {
		if time.Now().After(deadline) {
			st, err := tmux("display-message", "-p", "-t", pane, "#{pane_id} dead=#{pane_dead} #{pane_current_command}")
			t.Fatalf("pane %s never showed %q (%s %v); screen:\n%s", pane, want, st, err, capture(t, pane))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// keys types a command line and Enter into pane.
func keys(t *testing.T, pane string, ks ...string) {
	t.Helper()
	if _, err := tmux(append([]string{"send-keys", "-t", pane}, ks...)...); err != nil {
		t.Fatal(err)
	}
}

func ex(t *testing.T, pane, cmd string) {
	t.Helper()
	if _, err := tmux("send-keys", "-t", pane, "-l", ":"+cmd); err != nil {
		t.Fatal(err)
	}
	keys(t, pane, "Enter")
}

func next(t *testing.T, reqs <-chan Request) Request {
	t.Helper()
	select {
	case r := <-reqs:
		return r
	case <-time.After(patience):
		t.Fatal("no request reached the game")
	}
	panic("unreachable")
}

func noRequest(t *testing.T, reqs <-chan Request) {
	t.Helper()
	select {
	case r := <-reqs:
		t.Fatalf("game got %+v, want nothing", r.Command)
	case <-time.After(300 * time.Millisecond):
	}
}

func solution(t *testing.T) string {
	t.Helper()
	path := filepath.Join(sockDir(t), "sol.cpp")
	write(t, path, "int main() {}\n")
	return path
}

func TestOverlayCommands(t *testing.T) {
	privateTmux(t)
	sock, reqs := serve(t)
	path := solution(t)
	pane := startVim(t, Overlay{}, sock, path)

	// :Submit writes the modified buffer before asking, and the answer
	// arrives in the message line while vim stays responsive.
	keys(t, pane, "o// edited", "Escape")
	ex(t, pane, "Submit")
	r := next(t, reqs)
	if r.Cmd != CmdSubmit || len(r.Args) != 0 {
		t.Errorf("got %+v, want submit", r.Command)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "// edited") {
		t.Errorf("submitted before saving; file is %q", b)
	}
	r.Answer(Reply{OK: true, Text: "AC 3/3"})
	waitScreen(t, pane, "katharsis: AC 3/3")

	for _, tc := range []struct {
		line string
		want Command
		rep  Reply
	}{
		{"Debug 3", Command{Cmd: CmdDebug, Args: []string{"3"}}, Reply{OK: true, Text: "lldb on #3"}},
		{"Debug", Command{Cmd: CmdDebug}, Reply{Text: "no failing test to debug"}},
		{"Quack", Command{Cmd: CmdQuack}, Reply{Text: "no ducks left"}},
		{"Retreat", Command{Cmd: CmdRetreat}, Reply{OK: true, Text: "retreated"}},
	} {
		ex(t, pane, tc.line)
		r := next(t, reqs)
		if r.Cmd != tc.want.Cmd || !slices.Equal(r.Args, tc.want.Args) {
			t.Errorf(":%s sent %+v, want %+v", tc.line, r.Command, tc.want)
		}
		r.Answer(tc.rep)
		waitScreen(t, pane, "katharsis: "+tc.rep.Text)
	}
	if !PaneAlive(pane) {
		t.Fatal("vim exited")
	}
}

// :q! asks; no leaves vim as it was, yes gives up (and vim stays: the game
// closes it).
func TestOverlayQuitGivesUp(t *testing.T) {
	privateTmux(t)
	sock, reqs := serve(t)
	path := solution(t)
	pane := startVim(t, Overlay{QuitGivesUp: true}, sock, path)
	keys(t, pane, "ounsaved", "Escape")

	ex(t, pane, "q!")
	waitScreen(t, pane, "Give up?")
	keys(t, pane, "n")
	noRequest(t, reqs)
	ex(t, pane, "echo 'modified=' . &modified . ' lines=' . line('$')")
	waitScreen(t, pane, "modified=1 lines=2")

	ex(t, pane, "q!")
	waitScreen(t, pane, "Give up?")
	keys(t, pane, "y")
	r := next(t, reqs)
	if r.Cmd != CmdGiveUp {
		t.Errorf("got %+v, want giveup", r.Command)
	}
	r.Answer(Reply{OK: true, Text: "gave up"})
	waitScreen(t, pane, "katharsis: gave up")
	if !PaneAlive(pane) {
		t.Fatal("vim quit on its own after giving up")
	}

	// Typing q! inside a longer command is not :q!.
	ex(t, pane, "echo 'q!'")
	waitScreen(t, pane, "q!")
	noRequest(t, reqs)

	ex(t, pane, "qa!")
	waitFor(t, "vim to quit on :qa!", func() bool { return !PaneAlive(pane) })
	if b, _ := os.ReadFile(path); string(b) != "int main() {}\n" {
		t.Errorf(":qa! saved the buffer: %q", b)
	}
}

// Every other way out of vim works as always, and without QuitGivesUp :q!
// is just :q!.
func TestOverlayOtherQuits(t *testing.T) {
	privateTmux(t)
	sock, reqs := serve(t)
	for _, tc := range []struct {
		name  string
		o     Overlay
		quit  string
		saved bool
	}{
		{"wq", Overlay{QuitGivesUp: true}, "wq", true},
		{"x", Overlay{QuitGivesUp: true}, "x", true},
		{"qa!", Overlay{QuitGivesUp: true}, "qa!", false},
		{"quit!", Overlay{QuitGivesUp: true}, "quit!", false},
		{"q! without QuitGivesUp", Overlay{}, "q!", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := solution(t)
			pane := startVim(t, tc.o, sock, path)
			keys(t, pane, "oadded", "Escape")
			ex(t, pane, tc.quit)
			waitFor(t, "vim to quit on :"+tc.quit, func() bool { return !PaneAlive(pane) })
			b, _ := os.ReadFile(path)
			if saved := strings.Contains(string(b), "added"); saved != tc.saved {
				t.Errorf(":%s saved=%v, want %v", tc.quit, saved, tc.saved)
			}
			noRequest(t, reqs)
		})
	}
}
