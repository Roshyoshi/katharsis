package session

import (
	"fmt"
	"strings"
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

// KillPane closes a pane, whatever runs in it. A pane already gone is not
// an error.
func KillPane(pane string) error {
	if !PaneAlive(pane) {
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
			b.Restore()
			return nil, err
		}
	}
	return b, nil
}

// Set shows text on pane's border; "" blanks it. tmux redraws the border
// when the option changes.
func (b *Border) Set(pane, text string) error {
	// The drawn border reads #[...] as a style and ## as #.
	_, err := tmux("set-option", "-p", "-t", pane, borderOption, strings.ReplaceAll(text, "#", "##"))
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
