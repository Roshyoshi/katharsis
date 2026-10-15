package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/app"
	"katharsis/internal/catalog"
	"katharsis/internal/content/codeforces"
	"katharsis/internal/content/guide"
	"katharsis/internal/content/usaco"
	"katharsis/internal/hud"
	"katharsis/internal/screens"
	"katharsis/internal/session"
	"katharsis/internal/store"
)

// runGame is `katharsis`: the GRUB menu, boots, learn mode.
func runGame(args []string) error {
	home, _ := os.UserHomeDir()
	fs := flag.NewFlagSet("katharsis", flag.ContinueOnError)
	root := fs.String("root", filepath.Join(home, "cp", "runs"), "where door solutions are kept")
	editor := fs.String("editor", "vim", "editor command line, run by sh; the source path is appended")
	split := fs.Int("split", 65, "editor width in percent")
	scale := fs.Float64("scale", 1, "multiply CPU time limits (below 1 is stricter than the judge)")
	af := addAudioFlags(fs)
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

	// Before tmux, so a missing editor is said in the player's terminal
	// instead of as a pane that dies.
	if err := session.CheckEditor(*editor); err != nil {
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
	sink, audioSet, credits, audioErrs := openAudio(af)
	warnAudio(audioErrs)

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
	// One game per store: a second one resumes the same boot and saves its
	// stale copy of it over this one's. The kernel drops the lock however
	// the process ends.
	lock, err := store.Acquire(path)
	if err != nil {
		return err
	}
	defer lock.Close()
	// After the lock, so a second game that is refused never claims the
	// session. Best effort: unmarked, a relaunch after this game dies
	// attaches to its orphan editor, as it always did.
	_ = session.MarkGame()
	st, err := store.Open(path)
	if err != nil {
		return err
	}
	defer st.Close() // every write already committed
	logger, closeLog := openLog(path)
	defer closeLog()

	// Before the TUI: the first run downloads the USACO Guide snapshot
	// (about 150 MB) and the Codeforces index (about 17 MB) and builds the
	// indexes, which can take a minute. Later runs read them from the cache.
	fmt.Fprintln(os.Stderr, "katharsis: loading problems… (the first run downloads the USACO Guide and Codeforces indexes)")
	loadCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	done := showDownload()
	us, err := usaco.Open(loadCtx)
	done()
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
		Editor:    *editor,
		Split:     *split,
		Templates: app.Templates{Stdio: *stdioT, FileIO: *fileT},
		Scale:     *scale,
	}
	m, err := app.New(app.Deps{
		Store:   st,
		Catalog: catalog.NewUnion(cats...),
		Presets: ps.byName,
		Runner:  runner,
		Now:     time.Now,

		Audio:         sink,
		AudioSettings: audioSet,
		Credits:       credits,
		Log:           logger,
	})
	if err != nil {
		return err
	}
	p := tea.NewProgram(m)
	runner.Send = p.Send
	runner.Conceal = m.Conceal
	runner.Clock = m.Clock
	hud.SweepDebugDirs() // :Debug dirs a killed game left in $TMPDIR
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

// showDownload prints the USACO Guide download as it moves, on one line
// redrawn per MB, until done is called. A stall the guide then retries
// shows as the count stopping and starting over.
func showDownload() (done func()) {
	var shown atomic.Int64
	shown.Store(-1)
	guide.Progress = func(n int64) {
		if mb := n >> 20; shown.Swap(mb) != mb {
			fmt.Fprintf(os.Stderr, "\rkatharsis: downloading the USACO Guide… %d MB", mb)
		}
	}
	return func() {
		guide.Progress = nil
		if shown.Load() >= 0 {
			fmt.Fprintln(os.Stderr)
		}
	}
}

// openLog appends to katharsis.log beside the store: what goes wrong where
// the player cannot see it, such as an event the store could not write.
// The game plays without it.
func openLog(db string) (*log.Logger, func()) {
	f, err := os.OpenFile(filepath.Join(filepath.Dir(db), "katharsis.log"),
		os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return nil, func() {}
	}
	return log.New(f, "", log.LstdFlags), func() { _ = f.Close() }
}
