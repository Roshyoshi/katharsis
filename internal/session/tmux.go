// Package session lays out a problem in tmux, with the editor beside the
// game's HUD, and tells the game when the solution is saved.
package session

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
)

// tmuxArgs goes before every tmux subcommand. The game leaves it empty and
// reaches its server through $TMUX; tests point it at a private server
// (-L name) so they never touch the user's.
var tmuxArgs []string

// InTmux reports whether we run inside tmux.
func InTmux() bool { return os.Getenv("TMUX") != "" }

// SplitEditor opens `editor... file` in a new pane left of the game's pane
// ($TMUX_PANE), with cwd dir, pct% of the window wide, and returns the new
// pane's id (e.g. "%12"). The new pane takes focus.
//
// The pane gets the tmux server's environment, not the game's, so the editor
// starts the way the user's shell would start it.
func SplitEditor(dir, file string, editor []string, pct int) (paneID string, err error) {
	pane := os.Getenv("TMUX_PANE")
	switch {
	case pane == "":
		return "", errors.New("session: $TMUX_PANE not set")
	case len(editor) == 0:
		return "", errors.New("session: no editor command")
	}
	// With file appended there are always two words; see splitPane.
	return splitPane(pane, dir, []string{"-h", "-b"}, pct, append(slices.Clone(editor), file))
}

// SplitBelow opens argv in a new pane below pane, pct% of its height, with
// cwd dir, and returns the new pane's id. The new pane takes focus. argv
// needs two or more words to be exec'd as is; one word goes through the
// user's shell.
func SplitBelow(pane, dir string, argv []string, pct int) (paneID string, err error) {
	if len(argv) == 0 {
		return "", errors.New("session: no command for the pane")
	}
	return splitPane(pane, dir, []string{"-v"}, pct, argv)
}

func splitPane(target, dir string, flags []string, pct int, argv []string) (string, error) {
	if pct < 1 || pct > 99 {
		return "", fmt.Errorf("session: pane size %d%% not in 1..99", pct)
	}
	// tmux falls back to $HOME when -c does not exist, and the editor would
	// then open a fresh file there instead of the solution.
	if fi, err := os.Stat(dir); err != nil {
		return "", err
	} else if !fi.IsDir() {
		return "", fmt.Errorf("session: %s is not a directory", dir)
	}
	args := append([]string{"split-window"}, flags...)
	args = append(args,
		"-t", target,
		// tmux expands formats in -c, so "a##b" would become "a#b".
		"-c", strings.ReplaceAll(dir, "#", "##"),
		"-l", fmt.Sprintf("%d%%", pct),
		"-P", "-F", "#{pane_id}",
		// Two or more words are exec'd as they are; a single word would go
		// through the user's shell.
		"--",
	)
	id, err := tmux(append(args, argv...)...)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(id, "%") {
		return "", fmt.Errorf("session: tmux split-window printed %q, not a pane id", id)
	}
	return id, nil
}

// PaneAlive reports whether the pane exists and its command still runs. A
// pane kept on screen by remain-on-exit after its command exited counts as
// gone.
func PaneAlive(paneID string) bool {
	if paneID == "" {
		return false
	}
	// display-message tolerates a missing target: tmux 3.6a prints empty
	// fields and exits 0, so only an echoed id proves the pane exists.
	out, err := tmux("display-message", "-p", "-t", paneID, "#{pane_id} #{pane_dead}")
	return err == nil && out == paneID+" 0"
}

// ExecInTmux replaces the process with tmux running argv in a new session
// called name, for when the game starts outside tmux. If that session is
// already running (the same game started twice), tmux attaches to it
// instead (new-session -A) and argv is ignored. It returns only if it cannot
// exec.
//
// The session gets a server of its own (-L name). A session added to a
// server that is already running inherits that server's environment, not
// ours, and the judge finds the dev shell's pinned clang++ through PATH.
func ExecInTmux(name string, argv []string) error {
	if len(argv) == 0 {
		return errors.New("session: no command for the tmux session")
	}
	if InTmux() {
		return errors.New("session: already inside tmux")
	}
	bin, err := exec.LookPath("tmux")
	if err != nil {
		return err
	}
	return syscall.Exec(bin, execArgs(name, argv), os.Environ())
}

// execArgs is the argv ExecInTmux hands to exec. tmuxArgs follows -L name so
// a test's -L wins (tmux keeps the last one).
func execArgs(name string, argv []string) []string {
	args := append([]string{"tmux", "-L", name}, tmuxArgs...)
	args = append(args, "new-session", "-A", "-s", name, "--")
	if len(argv) == 1 {
		// tmux runs a lone word with `$SHELL -c`; quote it so a path with
		// spaces stays one word.
		return append(args, shellQuote(argv[0]))
	}
	return append(args, argv...)
}

// tmux runs one tmux command and returns its trimmed stdout. Errors carry
// tmux's own message ("can't find pane: %9").
func tmux(args ...string) (string, error) {
	out, err := exec.Command("tmux", append(slices.Clone(tmuxArgs), args...)...).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			return "", fmt.Errorf("tmux %s: %s", args[0], strings.TrimSpace(string(exit.Stderr)))
		}
		return "", fmt.Errorf("tmux %s: %w", args[0], err)
	}
	return strings.TrimSpace(string(out)), nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
