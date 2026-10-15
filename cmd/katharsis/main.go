// katharsis is a competitive-programming roguelike for the terminal.
//
// `katharsis` plays: the GRUB menu, boots, learn mode. `katharsis practice
// <dir>` is the training room: the solution in vim beside a HUD that
// rejudges the samples on every save. In both, vim's :Submit, :Debug and
// the rest reach the game through `katharsis ctl`.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/hud"
	"katharsis/internal/judge"
	"katharsis/internal/problem"
	"katharsis/internal/session"
)

func main() {
	args := os.Args[1:]
	run := runGame
	if len(args) > 0 {
		switch args[0] {
		case "ctl":
			// vim runs `katharsis ctl` on every :Submit, so it goes first and
			// touches nothing but the control socket's client.
			os.Exit(session.CtlMain(args[1:], os.Stdout, os.Stderr))
		case "practice":
			run, args = runPractice, args[1:]
		case "audio":
			run, args = runAudio, args[1:]
		}
	}
	if err := run(args); errors.Is(err, flag.ErrHelp) {
		os.Exit(0) // -h printed the usage; that is not a failure
	} else if err != nil {
		fmt.Fprintln(os.Stderr, "katharsis:", err)
		if session.OwnSession() {
			waitEnter(os.Stdin, os.Stderr)
		}
		os.Exit(1)
	}
}

// waitEnter holds the error on screen until the player presses enter. In
// the session ExecInTmux made, the session ends with the game, and the
// player's terminal showed only "[exited]".
func waitEnter(in io.Reader, out io.Writer) {
	fmt.Fprint(out, "press enter to close")
	_, _ = bufio.NewReader(in).ReadString('\n')
}

// usage prints the commands and the flags of fs, the command's own flag
// set (the global one is empty).
func usage(fs *flag.FlagSet) {
	fmt.Fprint(os.Stderr, `usage: katharsis [flags]                 play (GRUB menu, boots, learn mode)
       katharsis practice [flags] <dir>
       katharsis ctl <submit|debug [n]|quack|giveup|retreat|status>
       katharsis audio fetch|list|credits

<dir> holds <dir>.cpp (or sol.cpp) and tests/N.in + tests/N.out.
ctl talks to the game whose socket $KATHARSIS_SOCK names; the game's vim
runs it for :Submit, :Debug and the rest.

Custom rules presets are $XDG_CONFIG_HOME/katharsis/presets/*.toml
(default ~/.config/katharsis/presets): each lists only what it changes
from the standard rules, and is named by its name key or file name.

`+fs.Name()+` flags:
`)
	fs.PrintDefaults()
}

// runPractice is `katharsis practice`; args follow the subcommand.
func runPractice(args []string) error {
	fs := flag.NewFlagSet("practice", flag.ContinueOnError)
	noTmux := fs.Bool("no-tmux", false, "HUD only; edit the source yourself")
	editor := fs.String("editor", "vim", "editor command line, run by sh; the source path is appended")
	split := fs.Int("split", 65, "editor width in percent")
	tl := fs.Duration("tl", problem.DefaultTimeLimit, "time limit per test")
	scale := fs.Float64("scale", 1, "multiply CPU time limits (below 1 is stricter than the judge)")
	fs.Usage = func() { usage(fs) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		usage(fs)
		return fmt.Errorf("expected one problem directory")
	}
	dir, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		return err
	}
	prob, err := problem.Load(dir)
	if err != nil {
		return err
	}

	if !*noTmux {
		if err := session.CheckEditor(*editor); err != nil {
			return err
		}
	}
	// Outside tmux, restart inside a tmux session so the editor and HUD can
	// sit side by side; a second start attaches to the first. Parsing
	// stopped at the one argument left, so it is the last one.
	if !*noTmux && !session.InTmux() {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		argv := append([]string{self, "practice"}, args[:len(args)-1]...)
		return session.ExecInTmux("katharsis-"+prob.Name, append(argv, dir))
	}
	// Best effort: unmarked, a relaunch after this HUD dies attaches to
	// its orphan editor, as it always did.
	_ = session.MarkGame()

	// tmux sends SIGHUP when the HUD's pane or window is killed. Bubble Tea
	// handles SIGINT and SIGTERM itself; without this the process would die
	// before the deferred cleanup (socket, border options) runs. It is
	// caught from before the editor opens, so a hangup during the probe
	// below still cleans up.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGHUP)
	defer stop()

	hud.SweepDebugDirs() // :Debug dirs a killed HUD left in $TMPDIR
	opt := hud.Options{
		Dir:    dir,
		Limits: judge.Limits{Time: *tl, Mem: prob.MemLimit, Output: 64 << 20, Scale: *scale},
	}
	var (
		editorPane string
		srv        *session.Server
	)
	if !*noTmux {
		var cleanup func()
		editorPane, srv, cleanup, err = openEditor(&opt, dir, prob.Source, *editor, *split)
		if err != nil {
			return err
		}
		defer cleanup()
	}
	// After the editor is up: on a cold cache the probe can take seconds
	// (a hanging ASan runtime times out at 3 s), and the player can start
	// typing meanwhile.
	opt.Profile = judge.SampleProfile(ctx)

	m := hud.New(opt)
	p := tea.NewProgram(m)
	go func() {
		<-ctx.Done() // a hangup, or stop on the way out
		p.Quit()
	}()
	if srv != nil {
		go func() { _ = srv.Serve(func(r session.Request) { p.Send(r) }) }()
	}
	go func() {
		err := session.Watch(ctx, prob.Source, 80*time.Millisecond, func() { p.Send(hud.SavedMsg{}) })
		if err != nil && ctx.Err() == nil {
			p.Send(hud.WatchErrMsg{Err: err})
		}
	}()

	_, err = p.Run()
	m.Cleanup()
	// Quitting normally waits for the editor to close. Any other exit
	// (a signal) leaves it open; ask it to quit as :confirm qa would, which
	// never discards unsaved work.
	if editorPane != "" && !m.EditorLeftOpen() && session.PaneAlive(editorPane) {
		_ = session.CloseEditor(editorPane) // best effort, like cleanup
	}
	return err
}

// openEditor sets up what the editor pane needs: the control socket it
// reaches the HUD through, the vim overlay defining :Submit and the rest,
// the pane itself, the timer on its border, and :Debug's lldb pane under
// it. cleanup undoes all of it, best effort: the process is on its way out
// and has no one left to report to.
func openEditor(opt *hud.Options, dir, source, editor string, split int) (pane string, srv *session.Server, cleanup func(), err error) {
	var undo []func() error
	undoAll := func() {
		for i := len(undo) - 1; i >= 0; i-- {
			_ = undo[i]()
		}
	}
	// The error returns below zero the named results before this runs, so
	// neither it nor the closures in undo may read them: a nil cleanup or
	// srv here panicked and hid the error.
	defer func() {
		if err != nil {
			undoAll()
		}
	}()

	_ = session.SweepRunDir() // best effort: leftovers only take space
	judge.Sweep()
	sock, err := session.SocketPath()
	if err != nil {
		return "", nil, nil, err
	}
	s, err := session.Listen(sock)
	if err != nil {
		return "", nil, nil, err
	}
	undo = append(undo, s.Close)

	self, err := os.Executable()
	if err != nil {
		return "", nil, nil, err
	}
	overlay, err := session.OverlayPath()
	if err != nil {
		return "", nil, nil, err
	}
	// Practice has no door to give up, so :q! stays :q!.
	if err := (session.Overlay{Ctl: []string{self, "ctl"}}).Write(overlay); err != nil {
		return "", nil, nil, err
	}
	undo = append(undo, func() error { return os.Remove(overlay) })

	// The editor and lldb panes killed games left in this tmux server,
	// so this HUD's file opens in one vim (session.SweepOrphans).
	_, _ = session.SweepOrphans(3 * time.Second)
	pane, err = session.SplitEditor(dir, source, session.EditorArgv(editor, sock, overlay), split)
	if err != nil {
		return "", nil, nil, fmt.Errorf("open editor: %w", err)
	}
	_ = session.TagPane(pane, session.RoleEditor) // best effort, for the sweep
	opt.EditorAlive = func() bool { return session.PaneAlive(pane) }
	opt.CloseEditor = func() error { return session.CloseEditor(pane) }

	// The border is a nicety: a window that refuses it still plays.
	var border *session.Border
	if win, err := session.WindowOf(os.Getenv("TMUX_PANE")); err == nil {
		if border, err = session.ShowBorder(win); err == nil {
			undo = append(undo, border.Restore)
			opt.Border = func(text string) { _ = border.Set(pane, text) }
		}
	}
	opt.Debug = func(d hud.DebugTarget) (func(), error) {
		dbg, err := session.SplitBelow(pane, d.Dir, session.LLDBArgs(session.LLDB, d.Binary, d.Stdin), 40)
		if err != nil {
			return nil, err
		}
		_ = session.TagPane(dbg, session.RoleLLDB)
		if border != nil {
			_ = border.Set(dbg, " lldb · #"+d.Test+" · stopped at main: run ")
		}
		return func() { _ = session.KillPane(dbg) }, nil
	}
	return pane, s, undoAll, nil
}
