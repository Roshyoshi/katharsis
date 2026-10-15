package app

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/catalog"
	"katharsis/internal/hud"
	"katharsis/internal/judge"
	"katharsis/internal/libs"
	"katharsis/internal/problem"
	"katharsis/internal/session"
)

// TmuxRunner is the real Runner. Each door gets a directory under Root with
// the player's source, the samples and the statement; vim opens beside the
// game's pane; a HUD (embedded in the problem screen) rejudges the samples
// on every save and judges the full tests on :Submit. The app calls it from
// its Update only, so it needs no lock.
type TmuxRunner struct {
	Root      string   // solutions root, e.g. ~/cp/runs
	Editor    []string // e.g. ["vim"]
	Split     int      // editor width in percent
	Templates Templates
	Scale     float64 // CPU limit multiplier (judge.Limits.Scale)

	// Set by the app before the first Open.
	Send    func(tea.Msg)
	Conceal func() bool   // hide full tests (a boot without strace)
	Clock   func() string // the HUD clock and border: countdown, lives, ducks

	srv     *session.Server
	sock    string
	overlay string
	reaping sync.WaitGroup // closed editors' panes on their way out (reap)

	pane   string
	border *session.Border
	stop   context.CancelFunc
	hud    *hud.Model
	door   Door
	dir    string
	src    string
}

// Templates are C++ starting points; "" uses a built-in one. In a file-I/O
// template the word "problem" is replaced by the file stem, as the user's
// own ~/cp Makefile does.
type Templates struct {
	Stdio, FileIO string
}

// Listen starts the control socket the editor's commands reach the game
// through. Call once, after Send is set; Shutdown stops it.
func (r *TmuxRunner) Listen() error {
	// Best effort: what it misses is a dead game's files, and Listen
	// replaces a stale socket itself.
	_ = session.SweepRunDir()
	judge.Sweep() // judge temp and build dirs a killed game left behind
	sock, err := session.SocketPath()
	if err != nil {
		return err
	}
	srv, err := session.Listen(sock)
	if err != nil {
		return err
	}
	r.srv, r.sock = srv, sock
	go func() {
		if err := srv.Serve(func(req session.Request) { r.Send(req) }); err != nil {
			r.Send(errMsg{fmt.Errorf("control socket: %w; editor commands are off", err)})
		}
	}()
	return nil
}

// HUD is the open door's HUD, nil between doors.
func (r *TmuxRunner) HUD() *hud.Model { return r.hud }

func (r *TmuxRunner) Open(ctx context.Context, d Door) error {
	r.Close()
	dir, src, err := r.materialize(d)
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	overlay, err := session.OverlayPath()
	if err != nil {
		return err
	}
	// In a boot :q! is giving up; in an upsolve there is nothing to give up.
	if err := (session.Overlay{Ctl: []string{self, "ctl"}, QuitGivesUp: !d.Upsolve}).Write(overlay); err != nil {
		return err
	}
	r.overlay = overlay
	pane, err := session.SplitEditor(dir, src, session.EditorArgv(r.Editor, r.sock, overlay), r.Split)
	if err != nil {
		return fmt.Errorf("open editor: %w", err)
	}

	opt := hud.Options{
		Dir:     dir,
		Profile: judge.SampleProfile(ctx),
		Limits: judge.Limits{
			Time:           d.Problem.TimeLimit,
			Mem:            d.Problem.MemLimit,
			Output:         judge.DefaultOutputLimit,
			Scale:          r.Scale,
			FloatTolerance: d.Problem.FloatTolerance,
			FileIO:         d.Problem.FileIO, // a stem the source hides from detection
			AnyCase:        d.Problem.AnyCase,
		},
		SubmitProfile: judge.Full,
		FullTests:     d.Assets.Tests,
		EditorAlive:   func() bool { return session.PaneAlive(pane) },
		CloseEditor:   func() error { return session.CloseEditor(pane) },
		Clock:         r.Clock,
		// The problem screen draws the boot bar and the keys around it.
		Embedded: true,
	}
	if !d.Upsolve {
		opt.Conceal = r.Conceal
	}
	if win, err := session.WindowOf(os.Getenv("TMUX_PANE")); err == nil {
		if b, err := session.ShowBorder(win); err == nil {
			r.border = b
			// The text is computed in the HUD's Update; Set runs in a command
			// goroutine, so it must not read game state itself. A border that
			// fails to update only shows a stale clock.
			opt.Border = func(text string) { _ = b.Set(pane, text) }
		}
	}
	opt.Debug = func(t hud.DebugTarget) (func(), error) {
		dbg, err := session.SplitBelow(pane, t.Dir, session.LLDBArgs(session.LLDB, t.Binary, t.Stdin), 40)
		if err != nil {
			return nil, err
		}
		return func() { _ = session.KillPane(dbg) }, nil
	}

	wctx, stop := context.WithCancel(context.Background())
	go func() {
		err := session.Watch(wctx, src, 80*time.Millisecond, func() { r.Send(hud.SavedMsg{}) })
		if err != nil && wctx.Err() == nil {
			r.Send(hud.WatchErrMsg{Err: err})
		}
	}()

	r.pane, r.stop, r.door, r.dir, r.src = pane, stop, d, dir, src
	r.hud = hud.New(opt)
	return nil
}

// Reopen brings the editor back after the player closed vim mid-door.
func (r *TmuxRunner) Reopen() error {
	if r.dir == "" {
		return errors.New("no door open")
	}
	return r.Open(context.Background(), r.door)
}

// Close ends the door's side: watcher, HUD, editor, border. The control
// socket stays up. The editor saves before it quits: a door ends without
// the player at the keyboard (AC, a timeout, quitting the game), and in
// play a "Save changes?" prompt left behind outlived the game, answered
// itself with the player's next keystroke, and on resume put a second vim
// with a swap-file warning beside it. The watcher is stopped first, so
// that last write judges nothing.
func (r *TmuxRunner) Close() {
	if r.stop != nil {
		r.stop()
	}
	if r.hud != nil {
		r.hud.Cleanup()
	}
	if pane := r.pane; pane != "" {
		alive := session.PaneAlive(pane)
		if alive {
			// A failed save-and-quit leaves the editor open, which reap
			// then leaves alone.
			_ = session.SaveAndCloseEditor(pane)
		}
		r.reaping.Go(func() { reap(pane, alive) })
	}
	if r.border != nil {
		// Best effort: the options it could not put back stay set on
		// the game's window only.
		_ = r.border.Restore()
	}
	r.stop, r.hud, r.pane, r.border = nil, nil, "", nil
}

// reap waits for a closed editor's pane to die, then kills it: with tmux's
// remain-on-exit on (some players set it globally) a dead pane stays on
// screen. A vim still asking after reapWait (a buffer it cannot write) is
// the player's to answer, and stays.
func reap(pane string, alive bool) {
	for deadline := time.Now().Add(reapWait); alive; alive = session.PaneAlive(pane) {
		if time.Now().After(deadline) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = session.KillPane(pane) // gone already is fine; a failure leaves it on screen
}

const reapWait = 3 * time.Second

// Shutdown closes the door and the control socket. It waits for the last
// editor's pane to go, which the program exiting would otherwise cut short.
func (r *TmuxRunner) Shutdown() {
	r.Close()
	if r.srv != nil {
		r.srv.Close() // what it leaves, the next game's SweepRunDir removes
	}
	if r.overlay != "" {
		os.Remove(r.overlay) // likewise
	}
	r.reaping.Wait()
}

// materialize lays out a door's directory and returns it and the source
// path. A source already there (a resumed door, a second attempt at a
// regression) is kept; samples and statement are rewritten. A new upsolve
// source starts from the seed boot's attempt when there is one.
func (r *TmuxRunner) materialize(d Door) (dir, src string, err error) {
	sub := fmt.Sprintf("boot-%d", d.Boot)
	if d.Upsolve {
		sub = cmp.Or(d.Sub, "upsolve")
	}
	name := slug(d)
	dir = filepath.Join(r.Root, sub, name)
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0o755); err != nil {
		return "", "", err
	}
	src = filepath.Join(dir, name+".cpp")
	if _, err := os.Stat(src); errors.Is(err, fs.ErrNotExist) {
		body, ok := r.seed(d)
		if !ok {
			body = []byte(libs.Inject(r.template(d.Problem), d.Libs))
		}
		if err := os.WriteFile(src, body, 0o644); err != nil {
			return "", "", err
		}
	}
	for i, t := range d.Assets.Samples {
		base := filepath.Join(dir, "tests", fmt.Sprint(i+1))
		if err := copyTo(t.In, base+".in"); err != nil {
			return "", "", err
		}
		if err := copyTo(t.Out, base+".out"); err != nil {
			return "", "", err
		}
	}
	if d.Assets.Statement != "" && !d.Door.Mystery {
		// A copy for reading beside the source; the game's own reader (t)
		// reads the cache.
		_ = copyTo(d.Assets.Statement, filepath.Join(dir, "statement.md"))
	}
	if _, err := problem.Load(dir); err != nil {
		return "", "", err
	}
	return dir, src, nil
}

// seed reads the source an upsolve starts from: the player's attempt in
// boot d.SeedBoot, under either name the door could have had there (the
// titled one, or the mystery one).
func (r *TmuxRunner) seed(d Door) ([]byte, bool) {
	if !d.Upsolve || d.SeedBoot <= 0 {
		return nil, false
	}
	boot := filepath.Join(r.Root, fmt.Sprintf("boot-%d", d.SeedBoot))
	for _, mystery := range []bool{false, true} {
		e := d
		e.Door.Mystery = mystery
		name := slug(e)
		if b, err := os.ReadFile(filepath.Join(boot, name, name+".cpp")); err == nil {
			return b, true
		}
	}
	return nil, false
}

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

// slug names a door's directory and source. Mystery doors get no title in
// the name, since the HUD shows the directory name.
func slug(d Door) string {
	id := strings.ReplaceAll(string(d.Door.Problem), ":", "")
	if d.Door.Mystery {
		return "mystery_" + id
	}
	t := strings.Trim(nonWord.ReplaceAllString(strings.ToLower(d.Problem.Title), "_"), "_")
	if len(t) > 32 {
		t = strings.TrimRight(t[:32], "_")
	}
	if t == "" {
		return id
	}
	return id + "_" + t
}

func (r *TmuxRunner) template(p catalog.Problem) string {
	if p.FileIO != "" {
		if b, err := os.ReadFile(r.Templates.FileIO); err == nil {
			return strings.ReplaceAll(string(b), "problem", p.FileIO)
		}
		return strings.ReplaceAll(fileIOTemplate, "problem", p.FileIO)
	}
	if b, err := os.ReadFile(r.Templates.Stdio); err == nil {
		return string(b)
	}
	return stdioTemplate
}

// cppHeaders opens both built-in templates: libc++ has no <bits/stdc++.h>,
// so they list headers as the ~/cp templates do.
const cppHeaders = `#include <algorithm>
#include <array>
#include <cmath>
#include <cstdint>
#include <cstdio>
#include <functional>
#include <iostream>
#include <map>
#include <numeric>
#include <queue>
#include <set>
#include <string>
#include <unordered_map>
#include <vector>
using namespace std;
using ll = long long;
`

const stdioTemplate = "// libc++ has no <bits/stdc++.h>; list headers as the ~/cp templates do.\n" +
	cppHeaders + `
int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);

    return 0;
}
`

const fileIOTemplate = cppHeaders + `
int main() {
    freopen("problem.in", "r", stdin);
    freopen("problem.out", "w", stdout);
    ios::sync_with_stdio(false);
    cin.tie(nullptr);

    return 0;
}
`

func copyTo(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

// Source is the open door's source file, "" between doors.
func (r *TmuxRunner) Source() string {
	if r.hud == nil {
		return ""
	}
	return r.src
}
