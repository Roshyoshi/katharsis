package session

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	// Several times the game's 80 ms: on a loaded machine (load 30+) a
	// writer stalled mid-save for 50 ms split one save into two fires.
	testDebounce = 200 * time.Millisecond
	// Long enough for any straggler event to arm and fire the timer again.
	quiet = 4 * testDebounce
	// Upper bound on anything a test waits for: a fire, a pane, a reply.
	// Waits end as soon as the thing happens, so only a failure pays it;
	// under load lldb took 3 s to draw its first breakpoint.
	patience = 30 * time.Second
)

// startWatch runs watch on path until the test ends and returns a channel
// that receives once per onChange.
func startWatch(t *testing.T, path string) <-chan struct{} {
	t.Helper()
	fired, _ := startWatchCtx(t, context.Background(), path, testDebounce)
	return fired
}

// startWatchCtx is startWatch for tests that stop the watch themselves or
// want its result. At cleanup the watch is cancelled and, if nobody took its
// result, checked to be nil.
func startWatchCtx(t *testing.T, ctx context.Context, path string, debounce time.Duration) (<-chan struct{}, <-chan error) {
	t.Helper()
	return startWatching(t, ctx, func(ctx context.Context, onChange, ready func()) error {
		return watch(ctx, path, debounce, onChange, ready)
	})
}

// startWatching is startWatchCtx for any of the watches, which run calls.
func startWatching(t *testing.T, ctx context.Context, run func(ctx context.Context, onChange, ready func()) error) (<-chan struct{}, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(ctx)
	fired := make(chan struct{}, 16)
	ready := make(chan struct{})
	done := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		done <- run(ctx, func() { fired <- struct{}{} }, func() { close(ready) })
	}()
	t.Cleanup(func() {
		cancel()
		<-exited
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("watch: %v", err)
			}
		default: // the test took it
		}
	})
	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("watch did not start: %v", err)
	case <-time.After(patience):
		t.Fatal("watch did not start")
	}
	return fired, done
}

// expectFires waits for exactly n fires: each within patience, then none
// more for quiet.
func expectFires(t *testing.T, fired <-chan struct{}, n int) {
	t.Helper()
	for i := range n {
		select {
		case <-fired:
		case <-time.After(patience):
			t.Fatalf("fire %d of %d never came", i+1, n)
		}
	}
	select {
	case <-fired:
		t.Fatalf("fired more than %d times", n)
	case <-time.After(quiet):
	}
}

func setup(t *testing.T) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, "sol.cpp")
	write(t, path, "int main() {}\n")
	return dir, path
}

func write(t *testing.T, path, s string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWatchInPlaceWrite(t *testing.T) {
	t.Parallel()
	_, path := setup(t)
	fired := startWatch(t, path)
	write(t, path, "int main() { return 0; }\n")
	expectFires(t, fired, 1)
}

// What most editors' atomic save does.
func TestWatchRenameOver(t *testing.T) {
	t.Parallel()
	dir, path := setup(t)
	fired := startWatch(t, path)
	tmp := filepath.Join(dir, ".sol.cpp.tmp1234")
	write(t, tmp, "int main() { return 0; }\n")
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
	expectFires(t, fired, 1)
}

// What vim's default (backupcopy=auto, writebackup) save does.
func TestWatchBackupThenCreate(t *testing.T) {
	t.Parallel()
	_, path := setup(t)
	fired := startWatch(t, path)
	if err := os.Rename(path, path+"~"); err != nil {
		t.Fatal(err)
	}
	write(t, path, "int main() { return 0; }\n")
	if err := os.Remove(path + "~"); err != nil {
		t.Fatal(err)
	}
	expectFires(t, fired, 1)

	// The replacement is watched too: an in-place write to it still fires.
	write(t, path, "int main() { return 1; }\n")
	expectFires(t, fired, 1)
}

func TestWatchBurst(t *testing.T) {
	t.Parallel()
	_, path := setup(t)
	fired := startWatch(t, path)
	for i := range 5 {
		write(t, path, fmt.Sprintf("int main() { return %d; }\n", i))
		time.Sleep(testDebounce / 10)
	}
	expectFires(t, fired, 1)
}

func TestWatchIgnoresSiblings(t *testing.T) {
	t.Parallel()
	dir, path := setup(t)
	fired := startWatch(t, path)
	for _, name := range []string{".sol.cpp.swp", "4913", "sol.cpp~", "sol.cpp.bak", "other.cpp"} {
		p := filepath.Join(dir, name)
		write(t, p, "x")
		write(t, p, "xy")
		if name == "4913" {
			// vim deletes its probe right away
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
		}
	}
	expectFires(t, fired, 0)

	// Still watching: the silence above was not a dead watcher.
	write(t, path, "int main() { return 0; }\n")
	expectFires(t, fired, 1)
}

func TestWatchFileAppearsLater(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sol.cpp")
	fired := startWatch(t, path)
	write(t, path, "int main() {}\n")
	expectFires(t, fired, 1)

	// A delete arms the timer, but there is no file left to judge.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	expectFires(t, fired, 0)
}

func TestWatchCancel(t *testing.T) {
	t.Parallel()
	_, path := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	// A debounce the test never outlasts, so the save below is still
	// pending when the watch is cancelled, however slow the machine.
	fired, done := startWatchCtx(t, ctx, path, time.Minute)
	write(t, path, "int main() { return 0; }\n")
	time.Sleep(testDebounce)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("watch returned %v after cancel, want nil", err)
		}
	case <-time.After(patience):
		t.Fatal("watch did not return after cancel")
	}
	// onChange runs on the watch goroutine, so nothing can fire after return.
	write(t, path, "int main() { return 1; }\n")
	expectFires(t, fired, 0)
}

func TestWatchDirGone(t *testing.T) {
	t.Parallel()
	dir, path := setup(t)
	_, done := startWatchCtx(t, context.Background(), path, testDebounce)
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("watch returned nil after its directory was removed")
		}
		t.Log(err)
	case <-time.After(patience):
		t.Fatal("watch kept running after its directory was removed")
	}
}

// TestWatchTests: a test saved, added or removed fires once per change, a
// case's input and answer saved together once; swap files, backups and
// other names do not.
func TestWatchTests(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	fired, _ := startWatching(t, context.Background(), func(ctx context.Context, onChange, ready func()) error {
		return watchTests(ctx, dir, testDebounce, onChange, ready)
	})
	write(t, filepath.Join(dir, "c1.in"), "3\n")
	expectFires(t, fired, 1)

	for _, name := range []string{".c1.in.swp", "c1.in~", "4913", "notes.txt"} {
		write(t, filepath.Join(dir, name), "x")
	}
	expectFires(t, fired, 0)

	write(t, filepath.Join(dir, "c2.in"), "4\n")
	write(t, filepath.Join(dir, "c2.out"), "10\n")
	expectFires(t, fired, 1)

	if err := os.Remove(filepath.Join(dir, "c1.in")); err != nil {
		t.Fatal(err)
	}
	expectFires(t, fired, 1)
}

func TestWatchMissingDir(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nope", "sol.cpp")
	err := Watch(context.Background(), path, testDebounce, func() {})
	if err == nil {
		t.Fatal("Watch on a missing directory returned nil")
	}
}

// TestWatchVim saves with the real vim, which is what the watcher is for.
// Run with -v to see the raw events of each save.
func TestWatchVim(t *testing.T) {
	vim, err := exec.LookPath("vim")
	if err != nil {
		t.Skip("vim not on PATH")
	}
	t.Parallel()
	for _, tc := range []struct {
		name string
		opts []string
	}{
		// vim skips the write backup under $TMPDIR and /private/tmp
		// ('backupskip'), where the tests live, and writes in place.
		{"in place", nil},
		// Everywhere else (backupcopy=auto, writebackup): probe with 4913,
		// rename the original to sol.cpp~, write a new sol.cpp, delete the
		// backup.
		{"backup rename", []string{"--cmd", "set backupskip="}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir, path := setup(t)
			logRaw(t, dir)
			fired := startWatch(t, path)

			ctx, cancel := context.WithTimeout(context.Background(), patience)
			defer cancel()
			// -u NONE -i NONE: no vimrc, plugins or viminfo. Two saves,
			// the second once the test has seen the first and written
			// next (a name the watch ignores): a fixed sleep between them
			// merged the two into one fire on a loaded machine.
			args := append([]string{"-u", "NONE", "-i", "NONE", "-N", "-es"}, tc.opts...)
			args = append(args,
				"-c", "call setline(1, 'int main() { return 0; }')", "-c", "w",
				"-c", "while !filereadable('next') | sleep 10m | endwhile",
				"-c", "call setline(1, 'int main() { return 1; }')", "-c", "w",
				"-c", "qa!", "sol.cpp")
			cmd := exec.CommandContext(ctx, vim, args...)
			cmd.Dir = dir
			// Anything vim would put under ~ goes to a scratch dir instead.
			cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
			var out bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &out
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			expectFires(t, fired, 1)
			write(t, filepath.Join(dir, "next"), "")
			expectFires(t, fired, 1)
			if err := cmd.Wait(); err != nil {
				t.Fatalf("vim: %v\n%s", err, out.Bytes())
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "int main() { return 1; }\n" {
				t.Fatalf("vim wrote %q", got)
			}
		})
	}
}

// logRaw logs every fsnotify event in dir until the test ends.
func logRaw(t *testing.T, dir string) {
	t.Helper()
	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Add(dir); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for ev := range w.Events {
			t.Logf("raw %6.1fms %-13s %s", float64(time.Since(start).Microseconds())/1000,
				ev.Op, filepath.Base(ev.Name))
		}
	}()
	t.Cleanup(func() {
		w.Close()
		<-stopped
	})
}
