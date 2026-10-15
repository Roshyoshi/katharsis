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
	"katharsis/internal/content/codeforces"
	"katharsis/internal/content/usaco"
	"katharsis/internal/flavor"
	"katharsis/internal/screens"
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
	var presetFiles []string
	fs.Func("preset", "also load the rules preset in `file` (repeatable); unlike the presets dir, a bad one stops the game",
		func(f string) error { presetFiles = append(presetFiles, f); return nil })
	fs.Usage = func() { usage(fs) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	// A misspelt subcommand must fail here, not start the game.
	if fs.NArg() > 0 {
		usage(fs)
		return fmt.Errorf("unknown command %q", fs.Arg(0))
	}

	// Presets load before tmux, so a --preset that fails ends the game in
	// the player's own terminal, where the error stays on screen. The
	// flag's files go first: a dir preset with the same name is the one
	// skipped.
	ps := newPresets()
	for _, f := range presetFiles {
		if err := ps.add(f); err != nil {
			return fmt.Errorf("preset %w", err)
		}
	}
	dir, err := presetDir()
	if err != nil {
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
	warnPresets(ps.loadDir(dir))

	// tmux sends SIGHUP when the game's pane or window is killed; quit
	// through Bubble Tea so the deferred cleanup still runs. Before the
	// TUI, a hangup cancels the catalog downloads instead.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGHUP)
	defer stop()

	// Before the catalogs, so a database this build cannot use fails
	// without the wait.
	path, err := store.DefaultPath()
	if err != nil {
		return err
	}
	st, err := store.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }() // every write already committed

	// Before the TUI: the first run downloads the USACO Guide snapshot
	// (about 150 MB) and the Codeforces index (about 17 MB) and builds the
	// indexes, which can take a minute. Later runs read them from the cache.
	fmt.Fprintln(os.Stderr, "katharsis: loading problems… (the first run downloads the USACO Guide and Codeforces indexes)")
	loadCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	us, err := usaco.Open(loadCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("USACO catalog: %w", err)
	}
	cats := []catalog.Catalog{us}
	// Codeforces is extra doors, not a requirement: a cold cache with no
	// network still plays USACO. Its build took 20 s when measured; the
	// timeout only bounds a stalled download.
	loadCtx, cancel = context.WithTimeout(ctx, 3*time.Minute)
	cf, err := codeforces.Open(loadCtx)
	cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "katharsis: Codeforces catalog unavailable, playing USACO only: %v\n", err)
	} else {
		cats = append(cats, cf)
	}

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
		Catalog: catalog.NewUnion(cats...),
		Presets: ps.byName,
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
	go func() {
		<-ctx.Done() // a hangup, or stop on the way out
		// Quit through the app, so it cancels its fetches and closes the
		// editor; Send is a no-op once the program has exited. Force it if
		// that stalls.
		p.Send(screens.Quit{})
		time.Sleep(3 * time.Second)
		p.Quit()
	}()

	_, err = p.Run()
	return err
}

// play runs afplay in the background; a missing sound is silence.
func play(name string) {
	if f := flavor.SoundFile(name); f != "" {
		start(exec.Command("afplay", f))
	}
}

// start runs cmd without waiting on it, but reaps it once it exits: a
// child never waited on stays a zombie until the game exits, one per sound
// played.
func start(cmd *exec.Cmd) {
	if cmd.Start() == nil {
		go func() { _ = cmd.Wait() }()
	}
}
