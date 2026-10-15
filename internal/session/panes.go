package session

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// LLDB is the debugger :Debug runs; Xcode's, as `make lldb` in ~/cp uses.
var LLDB = "/usr/bin/lldb"

// LLDBArgs is the command line that debugs bin under lldb, stopped at a
// breakpoint on main and waiting for `run`. stdin, unless empty, is the file
// the program reads its standard input from (lldb's target.input-path), so
// typing `run` replays a test. For a file I/O program pass "" and start the
// pane in a directory holding <stem>.in instead.
//
// lldb runs with DEVELOPER_DIR and SDKROOT unset: /usr/bin/lldb is an xcrun
// shim, and with nix's dev shell values (which a game started from that
// shell passes to its tmux server, and so to every pane) it finds no lldb
// and exits with "error: tool 'lldb' not found".
func LLDBArgs(lldb, bin, stdin string) []string {
	args := []string{"env", "-u", "DEVELOPER_DIR", "-u", "SDKROOT", lldb}
	if stdin != "" {
		args = append(args, "-o", "settings set target.input-path "+lldbQuote(stdin))
	}
	return append(args, "-o", "breakpoint set --name main", "--", bin)
}

// lldbQuote double-quotes s for lldb's command parser, which would
// otherwise split a path at its spaces.
func lldbQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// KillPane closes a pane, whatever runs in it, including a dead one that
// remain-on-exit keeps on screen. A pane already gone is not an error.
func KillPane(pane string) error {
	if _, ok := paneDead(pane); !ok {
		return nil
	}
	_, err := tmux("kill-pane", "-t", pane)
	return err
}

// CloseEditor asks the vim in pane to quit the way `:confirm qa` does: at
// once when nothing is modified, otherwise asking per buffer whether to
// save, so unsaved work is never thrown away and the user can still cancel.
// The Escape first leaves whatever mode vim is in (insert, a half-typed
// command line, a confirm() prompt). It only types the keys; the pane
// closing is the answer (PaneAlive).
func CloseEditor(pane string) error {
	_, err := tmux("send-keys", "-t", pane, "Escape", ":confirm qa", "Enter")
	return err
}

// SaveAndCloseEditor writes every modified buffer, then quits as CloseEditor
// does. It is for a door that ends on its own (AC, a crash, quitting the
// game): nobody is at the keyboard to answer a save prompt, and one left
// behind eats the player's next keystroke and leaves a swap file. Resume and
// upsolve start from the source on disk, so saving is the safe answer. Only
// a buffer that cannot be written (no name, read-only) still asks.
func SaveAndCloseEditor(pane string) error {
	_, err := tmux("send-keys", "-t", pane, "Escape", ":silent! wall | confirm qa", "Enter")
	return err
}

// WindowOf returns the id (e.g. "@3") of the window holding pane.
func WindowOf(pane string) (string, error) {
	out, err := tmux("display-message", "-p", "-t", pane, "#{pane_id} #{window_id}")
	if err != nil {
		return "", err
	}
	id, win, ok := strings.Cut(out, " ")
	if !ok || id != pane || !strings.HasPrefix(win, "@") {
		return "", fmt.Errorf("session: no window for pane %s (tmux printed %q)", pane, out)
	}
	return win, nil
}

// borderOption holds a pane's border text; pane-border-format shows it.
const borderOption = "@katharsis_border"

// borderOptions are set on the game's window while the border shows.
var borderOptions = [][2]string{
	{"pane-border-status", "top"},
	{"pane-border-format", "#{" + borderOption + "}"},
}

// Border puts a line of text on the borders of the game's panes, so the
// timer stays in view while focus is in the editor. It sets
// pane-border-status and pane-border-format as options of the game's window
// only, never of the session or server, and Restore puts back what the
// window had. Each pane's text is a pane option; a pane with none shows a
// blank border.
type Border struct {
	window string
	saved  []savedOption
}

type savedOption struct {
	name, value string
	local       bool // set on the window itself rather than inherited
}

// ShowBorder turns the border on for window.
func ShowBorder(window string) (*Border, error) {
	b := &Border{window: window}
	for _, o := range borderOptions {
		// -q prints nothing for an option the window does not set itself;
		// -v drops the name, so the value needs no unquoting.
		name, err := tmux("show-options", "-w", "-q", "-t", window, o[0])
		if err != nil {
			return nil, err
		}
		s := savedOption{name: o[0], local: name != ""}
		if s.local {
			if s.value, err = tmux("show-options", "-w", "-q", "-v", "-t", window, o[0]); err != nil {
				return nil, err
			}
		}
		b.saved = append(b.saved, s)
	}
	for _, o := range borderOptions {
		if _, err := tmux("set-option", "-w", "-t", window, o[0], o[1]); err != nil {
			_ = b.Restore() // best effort; err says why the border failed
			return nil, err
		}
	}
	return b, nil
}

// Set shows text on pane's border; "" blanks it. tmux redraws the border
// when the option changes. text is a tmux format: the drawn border reads
// #[...] as a style and ## as #, so the caller styles its text and escapes
// whatever it did not write itself (ui.TmuxEscape, ui.Token.Tmux).
func (b *Border) Set(pane, text string) error {
	_, err := tmux("set-option", "-p", "-t", pane, borderOption, text)
	return err
}

// Restore puts the window's border options back as they were. The panes'
// own text options go with the panes.
func (b *Border) Restore() error {
	var first error
	for _, s := range b.saved {
		args := []string{"set-option", "-w", "-t", b.window, s.name, s.value}
		if !s.local {
			args = []string{"set-option", "-w", "-u", "-t", b.window, s.name}
		}
		if _, err := tmux(args...); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// gameOption, a pane option, names the game process that opened an editor
// or lldb pane, and roleOption says which of the two it is (TagPane). A
// pane whose game is gone is an orphan: the game was killed (kill -9, a
// crash) before it could close it.
const (
	gameOption = "@katharsis_game"
	roleOption = "@katharsis_role"
)

// Pane roles, for TagPane.
const (
	RoleEditor = "editor"
	RoleLLDB   = "lldb"
)

// TagPane marks pane as this process's, in role, for SweepOrphans.
func TagPane(pane, role string) error {
	if _, err := tmux("set-option", "-p", "-t", pane, gameOption, strconv.Itoa(os.Getpid())); err != nil {
		return err
	}
	_, err := tmux("set-option", "-p", "-t", pane, roleOption, role)
	return err
}

// SweepOrphans closes the panes of games that are gone. A game killed
// mid-door left its vim, with the player's unsaved edits, and lldb
// running, and the resumed door opened a second vim on the same file
// (E325, swap file in use). An orphan editor is saved and closed as a door
// that ends does it (SaveAndCloseEditor), so its edits reach the file the
// resumed door opens; lldb is killed. It waits up to wait for the editors
// to go, killing each pane once its editor has quit (remain-on-exit would
// keep it); one still asking (a buffer it cannot write) is the player's to
// answer, and stays. It returns how many orphan panes it found.
func SweepOrphans(wait time.Duration) (int, error) {
	out, err := tmux("list-panes", "-a", "-F", "#{pane_id} #{"+gameOption+"} #{"+roleOption+"}")
	if err != nil {
		return 0, err
	}
	var editors []string
	found := 0
	for line := range strings.Lines(out) {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue // not a game's pane
		}
		pid, err := strconv.Atoi(f[1])
		if err != nil || pid == os.Getpid() || !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			continue
		}
		found++
		switch pane := f[0]; f[2] {
		case RoleEditor:
			if dead, ok := paneDead(pane); ok && !dead {
				// Best effort, like Close: a failure leaves it open.
				_ = SaveAndCloseEditor(pane)
			}
			editors = append(editors, pane)
		default:
			_ = KillPane(pane)
		}
	}
	for deadline := time.Now().Add(wait); len(editors) > 0; {
		editors = slices.DeleteFunc(editors, func(pane string) bool {
			if dead, ok := paneDead(pane); ok && !dead {
				return false
			}
			_ = KillPane(pane) // a dead pane remain-on-exit keeps
			return true
		})
		if len(editors) == 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return found, nil
}

// Zoom zooms pane to fill its window, or unzooms it, as `resize-pane -Z`
// does, for a pager opened in a pane too narrow to read in (ui-system.md
// §2.6). It is idempotent: tmux's zoom is a toggle, so the window's
// zoomed flag decides whether to send it.
func Zoom(pane string, on bool) error {
	flag, err := tmux("display-message", "-p", "-t", pane, "#{window_zoomed_flag}")
	if err != nil {
		return err
	}
	if (flag == "1") == on {
		return nil
	}
	_, err = tmux("resize-pane", "-Z", "-t", pane)
	return err
}
