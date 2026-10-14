package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/app"
	"katharsis/internal/catalog"
	"katharsis/internal/content/usaco"
	"katharsis/internal/flavor"
	"katharsis/internal/session"
	"katharsis/internal/store"
)

// runGame is `katharsis`: the GRUB menu, boots, learn mode.
func runGame(args []string) error {
	home, _ := os.UserHomeDir()
	fs := flag.NewFlagSet("katharsis", flag.ContinueOnError)
	root := fs.String("root", filepath.Join(home, "cp", "runs"), "where door solutions are kept")
	editor := fs.String("editor", "vim", "editor command; the source path is appended")
	split := fs.Int("split", 65, "editor width in percent")
	scale := fs.Float64("scale", 1, "multiply CPU time limits (below 1 is stricter than the judge)")
	quiet := fs.Bool("quiet", false, "no sounds")
	stdioT := fs.String("template", filepath.Join(home, "cp", "atcoder", "template.cpp"), "C++ template for stdin/stdout problems")
	fileT := fs.String("template-fileio", filepath.Join(home, "cp", "template.cpp"), "C++ template for file-I/O problems (\"problem\" becomes the file stem)")
	fs.Usage = usage
	if err := fs.Parse(args); err != nil {
		return err
	}

	// The game needs tmux to sit beside the editor. Outside it, restart in
	// a session of its own; starting again attaches to the running game.
	if !session.InTmux() {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		return session.ExecInTmux("katharsis", append([]string{self}, args...))
	}

	// Before the TUI: the first run downloads the USACO Guide snapshot
	// (about 150 MB) and builds the index, which can take a minute.
	fmt.Fprintln(os.Stderr, "katharsis: loading problems…")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	us, err := usaco.Open(ctx)
	cancel()
	if err != nil {
		return fmt.Errorf("USACO catalog: %w", err)
	}
	cat := catalog.NewUnion(us)

	path, err := store.DefaultPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	st, err := store.Open(path)
	if err != nil {
		return err
	}
	defer st.Close()

	runner := &app.TmuxRunner{
		Root:      *root,
		Editor:    strings.Fields(*editor),
		Split:     *split,
		Templates: app.Templates{Stdio: *stdioT, FileIO: *fileT},
		Scale:     *scale,
	}
	var sound func(string)
	if !*quiet {
		sound = play
	}
	m, err := app.New(app.Deps{
		Store:   st,
		Catalog: cat,
		Runner:  runner,
		Now:     time.Now,
		Sound:   sound,
	})
	if err != nil {
		return err
	}
	p := tea.NewProgram(m)
	runner.Send = p.Send
	runner.Conceal = m.Conceal
	runner.Clock = m.Clock
	if err := runner.Listen(); err != nil {
		return err
	}
	defer runner.Shutdown()

	// tmux sends SIGHUP when the game's pane or window is killed; quit
	// through Bubble Tea so the deferred cleanup still runs.
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	go func() {
		<-hup
		p.Quit()
	}()

	_, err = p.Run()
	return err
}

// play runs afplay in the background; a missing sound is silence.
func play(name string) {
	if f := flavor.SoundFile(name); f != "" {
		exec.Command("afplay", f).Start()
	}
}
