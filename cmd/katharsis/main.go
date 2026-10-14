// katharsis is a competitive-programming roguelike for the terminal.
//
// M0 ships the training room: `katharsis practice <dir>` opens the solution
// in vim beside a HUD that rejudges the samples on every save. In vim,
// :Submit judges every test at -O2 (the HP bar) and :Debug n opens lldb on
// test n under the editor; both reach the HUD through `katharsis ctl`.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/hud"
	"katharsis/internal/judge"
	"katharsis/internal/problem"
	"katharsis/internal/session"
)

func main() {
	// vim runs `katharsis ctl` on every :Submit, so it goes first and
	// touches nothing but the control socket's client.
	if len(os.Args) > 1 && os.Args[1] == "ctl" {
		os.Exit(session.CtlMain(os.Args[2:], os.Stdout, os.Stderr))
	}
	run := runGame
	if len(os.Args) > 1 && os.Args[1] == "practice" {
		run = runPractice
	}
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "katharsis:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: katharsis [flags]                 play (GRUB menu, boots, learn mode)
       katharsis practice [flags] <dir>
       katharsis ctl <submit|debug [n]|quack|giveup|retreat|status>

<dir> holds <dir>.cpp (or sol.cpp) and tests/N.in + tests/N.out.
ctl talks to the game whose socket $KATHARSIS_SOCK names; the game's vim
runs it for :Submit, :Debug and the rest.

flags:
`)
	flag.PrintDefaults()
}

func runPractice(args []string) error {
	fs := flag.NewFlagSet("practice", flag.ContinueOnError)
	noTmux := fs.Bool("no-tmux", false, "HUD only; edit the source yourself")
	editor := fs.String("editor", "vim", "editor command; the source path is appended")
	split := fs.Int("split", 65, "editor width in percent")
	tl := fs.Duration("tl", problem.DefaultTimeLimit, "time limit per test")
	scale := fs.Float64("scale", 1, "multiply CPU time limits (below 1 is stricter than the judge)")
	fs.Usage = usage
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		usage()
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

	// Outside tmux, restart inside a tmux session so the editor and HUD can
	// sit side by side; a second start attaches to the first.
	if !*noTmux && !session.InTmux() {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		argv := append([]string{self, "practice"}, args[1:len(args)-1]...)
		argv = append(argv, dir)
		return session.ExecInTmux("katharsis-"+prob.Name, argv)
	}

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
		editorPane, srv, cleanup, err = openEditor(&opt, dir, prob.Source, strings.Fields(*editor), *split)
		if err != nil {
			return err
		}
		defer cleanup()
	}
	// After the editor is up: on a cold cache the probe can take seconds
	// (a hanging ASan runtime times out at 3 s), and the player can start
	// typing meanwhile.
	opt.Profile = judge.SampleProfile(context.Background())

	m := hud.New(opt)
	p := tea.NewProgram(m)
	if srv != nil {
		go srv.Serve(func(r session.Request) { p.Send(r) })
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		err := session.Watch(ctx, prob.Source, 80*time.Millisecond, func() { p.Send(hud.SavedMsg{}) })
		if err != nil && ctx.Err() == nil {
			p.Send(hud.WatchErrMsg{Err: err})
		}
	}()
	// tmux sends SIGHUP when the HUD's pane or window is killed. Bubble Tea
	// handles SIGINT and SIGTERM itself; without this the process would die
	// before the deferred cleanup (socket, border options) runs.
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	go func() {
		<-hup
		p.Quit()
	}()

	_, err = p.Run()
	m.Cleanup()
	// Quitting normally waits for the editor to close. Any other exit
	// (a signal) leaves it open; ask it to quit as :confirm qa would, which
	// never discards unsaved work.
	if editorPane != "" && !m.EditorLeftOpen() && session.PaneAlive(editorPane) {
		session.CloseEditor(editorPane)
	}
	return err
}

// openEditor sets up what the editor pane needs: the control socket it
// reaches the HUD through, the vim overlay defining :Submit and the rest,
// the pane itself, the timer on its border, and :Debug's lldb pane under
// it. cleanup undoes all of it.
func openEditor(opt *hud.Options, dir, source string, editor []string, split int) (pane string, srv *session.Server, cleanup func(), err error) {
	var undo []func()
	cleanup = func() {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
	}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()

	session.SweepRunDir()
	sock, err := session.SocketPath()
	if err != nil {
		return "", nil, nil, err
	}
	if srv, err = session.Listen(sock); err != nil {
		return "", nil, nil, err
	}
	undo = append(undo, func() { srv.Close() })

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
	undo = append(undo, func() { os.Remove(overlay) })

	pane, err = session.SplitEditor(dir, source, session.EditorArgv(editor, sock, overlay), split)
	if err != nil {
		return "", nil, nil, fmt.Errorf("open editor: %w", err)
	}
	opt.EditorAlive = func() bool { return session.PaneAlive(pane) }
	opt.CloseEditor = func() error { return session.CloseEditor(pane) }

	// The border is a nicety: a window that refuses it still plays.
	var border *session.Border
	if win, err := session.WindowOf(os.Getenv("TMUX_PANE")); err == nil {
		if border, err = session.ShowBorder(win); err == nil {
			undo = append(undo, func() { border.Restore() })
			opt.Border = func(text string) { border.Set(pane, text) }
		}
	}
	opt.Debug = func(d hud.DebugTarget) (func(), error) {
		dbg, err := session.SplitBelow(pane, d.Dir, session.LLDBArgs(session.LLDB, d.Binary, d.Stdin), 40)
		if err != nil {
			return nil, err
		}
		if border != nil {
			border.Set(dbg, " lldb · #"+d.Test+" · stopped at main: run ")
		}
		return func() { session.KillPane(dbg) }, nil
	}
	return pane, srv, cleanup, nil
}
