package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/anim"
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
	split := fs.Int("split", 0, "editor width in percent; 0 gives the HUD 35% of the window, 36 to 48 columns")
	scale := fs.Float64("scale", 1, "multiply CPU time limits (below 1 is stricter than the judge)")
	af := addAudioFlags(fs)
	noPrompts := fs.Bool("no-prompts", false, "never ask: no first-launch rating question, no line under a boot's end")
	motion := fs.String("motion", "full", "story animations: full, or reduced (every scene drawn at once)")
	fps := fs.Int("fps", 0, fmt.Sprintf("frame-rate cap for the story animations (0 is %d)", anim.DefaultFPS))
	budget := fs.Duration("session-budget", -1, "the day's play past which the end of a boot says so; 0 is off (default: settings.toml, else 3h)")
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
	mo, err := anim.ParseMotion(*motion, *fps)
	if err != nil {
		return err
	}
	anim.SetMotion(mo)

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
	warnKey("", ps.loadDir(dir), "Skipped those presets.")
	prefs := openSettings(*budget)
	sink, audioSet, credits, audioErrs := openAudio(af)
	defer closeAudio(sink, audioCloseWait) // every way out: quit, hangup, an error
	warnKey("audio: ", audioErrs, audioDefaults)

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
	logger, closeLog := openLog(filepath.Join(filepath.Dir(path), "katharsis.log"))
	defer closeLog()

	// Before the TUI: the first run downloads the USACO Guide snapshot
	// (about 150 MB) and builds the USACO index, which can take a minute.
	// Later runs read it from the cache.
	fmt.Fprintln(os.Stderr, "katharsis: loading problems… (the first run downloads the USACO Guide)")
	loadCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	done := showDownload()
	us, err := usaco.Open(loadCtx)
	done()
	cancel()
	if err != nil {
		return fmt.Errorf("USACO catalog: %w", err)
	}
	cats := catalog.NewUnion(us)
	cfNote := joinCodeforces(ctx, cats)

	runner := &app.TmuxRunner{
		Root:      *root,
		Editor:    *editor,
		Split:     *split,
		Templates: app.Templates{Stdio: *stdioT, FileIO: *fileT},
		Scale:     *scale,
	}
	m, err := app.New(app.Deps{
		Store:   st,
		Catalog: cats,
		Presets: ps.byName,
		Runner:  runner,
		Now:     time.Now,

		Audio:         sink,
		AudioSettings: audioSet,
		Credits:       credits,
		Log:           logger,
		Prompts:       !*noPrompts,
		Story:         true,
		Title:         true,
		Sources:       cfNote,
		Budget:        prefs.SessionBudget.Duration,
		SetuidNotices: prefs.SetuidNotices,
		Spoilers:      prefs.SetuidSpoilers,
	})
	if err != nil {
		return err
	}
	p := tea.NewProgram(m)
	runner.Send = p.Send
	runner.Conceal = m.Conceal
	runner.Clock = m.Clock
	runner.BorderClock = m.BorderClock
	runner.Best = m.Best
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
	if s := m.Farewell(); s != "" && err == nil {
		fmt.Println(s) // a boot suspended on the way out (map.suspend)
	}
	return err
}

// joinCodeforces adds Codeforces to cats without holding up the start.
// Codeforces is extra doors, not a requirement: from a cached index it
// joins at once (about 0.1 s); from a cold or stale cache the index builds
// in the background (20 s when measured, minutes on a stalled network) and
// joins when done, for the next boot. The note it returns is GRUB's line
// until then: the build's progress, or "Codeforces offline" once it fails.
// A failed build tries once more after codeforces.RetryWait.
func joinCodeforces(ctx context.Context, cats *catalog.Union) (note func() string) {
	var text atomic.Pointer[string]
	set := func(s string) { text.Store(&s) }
	note = func() string { return *text.Load() }
	if cf, err := codeforces.Cached(); err == nil {
		cats.Add(cf)
		set("")
		return note
	}
	set("Codeforces indexing… 0%")
	go func() {
		cf, err := codeforces.Build(ctx, func(done, total int) {
			set(fmt.Sprintf("Codeforces indexing… %d%%", done*100/total))
		}, func(error) { set("Codeforces offline") })
		if err != nil {
			set("Codeforces offline")
			return
		}
		cats.Add(cf)
		set("")
	}()
	return note
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

// logPath is katharsis.log in the XDG state dir, $XDG_STATE_HOME/katharsis
// (else ~/.local/state/katharsis), which the spec keeps for logs: they are
// not data to back up or move between machines, as the store is. XDG on
// macOS too, as the store and the presets are, not ~/Library/Logs.
func logPath() (string, error) {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" && filepath.IsAbs(d) {
		return filepath.Join(d, "katharsis", "katharsis.log"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "katharsis", "katharsis.log"), nil
}

// openLog appends to katharsis.log (logPath): what goes wrong where the
// player cannot see it, such as an event the store could not write. A log
// at old, beside the store where it used to be, moves there first, so
// what it holds stays in one file. The game plays without a log.
func openLog(old string) (*log.Logger, func()) {
	path, err := logPath()
	if err != nil {
		return nil, func() {}
	}
	// 0700, as the XDG spec asks of a dir it has to create.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, func() {}
	}
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		_ = os.Rename(old, path) // best effort: one left behind still reads where it is
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return nil, func() {}
	}
	return log.New(f, "", log.LstdFlags), func() { _ = f.Close() }
}
