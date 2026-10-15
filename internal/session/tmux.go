// Package session lays out a problem in tmux, with the editor beside the
// game's HUD, and tells the game when the solution is saved.
package session

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
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
// ($TMUX_PANE), with cwd dir, and returns the new pane's id (e.g. "%12").
// The new pane takes focus. pct is the editor's share of the window in
// percent; 0 gives the game's pane HUDWidth columns and the editor the
// rest.
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
	// With file appended argv has two words or more, which tmux execs
	// without a shell.
	argv := append(slices.Clone(editor), file)
	if pct != 0 {
		return splitPane(pane, dir, []string{"-h", "-b"}, pct, argv)
	}
	id, err := splitPane(pane, dir, []string{"-h", "-b"}, 65, argv)
	if err != nil {
		return "", err
	}
	// Best effort: a HUD that cannot be resized still works, only narrower.
	if out, err := tmux("display-message", "-p", "-t", pane, "#{window_width}"); err == nil {
		if w, err := strconv.Atoi(out); err == nil {
			_, _ = tmux("resize-pane", "-t", pane, "-x", strconv.Itoa(HUDWidth(w)))
		}
	}
	return id, nil
}

// HUDWidth is the game's pane width beside the editor in a window w columns
// wide (C8): 35% of it, at least 36 columns, so the HUD and its key rows
// read at 80, and at most 48, past which it only takes from the editor.
// The width is absolute, not what is left of the editor's share: 65% of 80
// left the HUD 27 columns (E12).
func HUDWidth(w int) int { return max(36, min(48, w*35/100)) }

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
	dead, ok := paneDead(paneID)
	return ok && !dead
}

// paneDead reports whether pane's command has exited, leaving the pane to
// remain-on-exit; ok is false when there is no such pane.
func paneDead(pane string) (dead, ok bool) {
	if pane == "" {
		return false, false
	}
	// display-message tolerates a missing target: tmux 3.6a prints empty
	// fields and exits 0, so only an echoed id proves the pane exists.
	out, err := tmux("display-message", "-p", "-t", pane, "#{pane_id} #{pane_dead}")
	id, state, _ := strings.Cut(out, " ")
	if err != nil || id != pane {
		return false, false
	}
	return state == "1", true
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
//
// argv runs with $KATHARSIS_OWN_SESSION set (OwnSession). A session whose
// game is gone (MarkGame's pid is dead: a kill -9, a panic) is not merely
// attached, which showed the game's orphan vim and no game: argv starts in
// a new window of it, and the game it starts sweeps the orphan.
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
	argv = append([]string{"env", ownSessionEnv + "=1"}, argv...)
	args := execArgs(name, argv)
	if gone(name) {
		if err := newWindow(name, argv); err == nil {
			args = append(serverArgs(name), "attach-session", "-t", name)
		}
	}
	return syscall.Exec(bin, args, os.Environ())
}

// ownSessionEnv marks the game ExecInTmux started in a session of its own.
const ownSessionEnv = "KATHARSIS_OWN_SESSION"

// OwnSession reports whether this process is the game ExecInTmux started:
// its session ends with it, taking with it whatever it printed last.
func OwnSession() bool { return os.Getenv(ownSessionEnv) == "1" && InTmux() }

// pidOption, a session option, holds the pid of the game playing in a
// session ExecInTmux made (MarkGame).
const pidOption = "@katharsis_pid"

// MarkGame records this process as the game of its own session, for the
// next ExecInTmux to tell a running game from a dead one. Outside its own
// session (the player's tmux) it does nothing.
func MarkGame() error {
	if !OwnSession() {
		return nil
	}
	_, err := tmux("set-option", "-t", os.Getenv("TMUX_PANE"), pidOption, strconv.Itoa(os.Getpid()))
	return err
}

// serverArgs is tmux's argv up to the subcommand for the server of the
// session called name. tmuxArgs follows -L name so a test's -L wins (tmux
// keeps the last one).
func serverArgs(name string) []string {
	return append([]string{"tmux", "-L", name}, tmuxArgs...)
}

// gone reports whether session name exists with a game, as MarkGame
// recorded it, that is no longer running. No server, no session or no pid
// is not gone: new-session -A makes or attaches as it always did.
func gone(name string) bool {
	args := append(serverArgs(name)[1:], "show-options", "-v", "-q", "-t", name+":", pidOption)
	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	return err == nil && pid > 0 && errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
}

// newWindow starts argv in a new window of session name, with this
// process's environment: the server's own is that of the game that died.
func newWindow(name string, argv []string) error {
	args := append(serverArgs(name)[1:], "new-window", "-t", name+":")
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "TMUX=") && !strings.HasPrefix(kv, "TMUX_PANE=") {
			args = append(args, "-e", kv)
		}
	}
	out, err := exec.Command("tmux", append(append(args, "--"), argv...)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux new-window: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// execArgs is the argv ExecInTmux hands to exec.
func execArgs(name string, argv []string) []string {
	args := serverArgs(name)
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
