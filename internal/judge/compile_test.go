package judge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// hangingCompiler is a compiler that never finishes, like one stuck on a
// template blowup or a wedged toolchain: it says it is working, starts a
// temp file in $TMPDIR, as cc1plus starts its assembly, then waits on a
// child that holds its stderr and stdout, as g++ waits on cc1plus. The
// child sleeps for mark seconds, so pgrep can find it.
func hangingCompiler(t *testing.T, mark string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hang++")
	script := fmt.Sprintf("#!/bin/sh\necho 'hang++: thinking' >&2\n: > \"${TMPDIR:-/tmp}/hang++-$$.s\"\nsleep %s &\nwait\n", mark)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// lingers reports whether hangingCompiler's child for mark still runs,
// and kills it if so.
func lingers(mark string) bool {
	out, _ := exec.Command("pgrep", "-f", "sleep "+mark).Output()
	if len(out) == 0 {
		return false
	}
	_ = exec.Command("pkill", "-f", "sleep "+mark).Run()
	return true
}

// within fails t unless f returns within d. What it tests would
// otherwise hang the test binary.
func within(t *testing.T, d time.Duration, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("still running after %v", d)
	}
}

// A compile that hangs ends at compileTimeout as a compile error that
// says so, the compiler's children with it: a :Submit stuck on one would
// otherwise hold the door until its grace ran out (a :Submit while one
// judges is refused). A precompiled header's build, and a build loading
// one, end the same way. A cancel ends a hanging compile at once.
//
// Not parallel: it lowers compileTimeout while the parallel tests are
// parked.
func TestCompileTimeout(t *testing.T) {
	saved := compileTimeout
	compileTimeout = time.Second
	t.Cleanup(func() { compileTimeout = saved })
	t.Setenv("TMPDIR", t.TempDir()) // for hang++ --version
	const body = "#include <cstdio>\nint main() {}\n"
	src := filepath.Join(t.TempDir(), "sol.cpp")
	if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	timedOut := func(t *testing.T, b Build) {
		t.Helper()
		if b.OK || !strings.HasPrefix(b.Log, "error: compile timed out after 1s\n") ||
			!strings.Contains(b.Log, "hang++: thinking") {
			t.Errorf("OK %v, log:\n%s\nwant a compile error that names the timeout, then the compiler's output", b.OK, b.Log)
		}
	}

	t.Run("submit", func(t *testing.T) {
		const mark = "3141.5926501"
		p := Profile{Name: "hang", Compiler: hangingCompiler(t, mark)}
		var evs []Event
		within(t, 10*time.Second, func() {
			Submit(context.Background(), src, nil, p, Limits{Time: time.Second}, func(e Event) { evs = append(evs, e) })
		})
		if len(evs) != 2 {
			t.Fatalf("events %#v, want BuildDone and Finished", evs)
		}
		if bd, ok := evs[0].(BuildDone); !ok {
			t.Errorf("first event %#v, want BuildDone", evs[0])
		} else {
			timedOut(t, bd.Build)
		}
		if f, ok := evs[1].(Finished); !ok || f.Err != nil || f.Summary == nil || f.Summary.Verdict != CE {
			t.Errorf("last event %#v, want Finished with a CE summary", evs[1])
		}
		if lingers(mark) {
			t.Error("the compiler's child outlived the timeout")
		}
	})

	// A killed compiler cannot remove its temp files: they go in the
	// build's own dir, which goes with the build.
	t.Run("cancelled", func(t *testing.T) {
		const mark = "3141.5926502"
		p := Profile{Name: "hang", Compiler: hangingCompiler(t, mark)}
		tmp := t.TempDir()
		t.Setenv("TMPDIR", tmp)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		time.AfterFunc(200*time.Millisecond, cancel)
		var b Build
		var err error
		start := time.Now()
		within(t, 10*time.Second, func() { b, err = Compile(ctx, src, p) })
		if err == nil || b.OK {
			t.Errorf("cancelled: err %v, OK %v; want an error", err, b.OK)
		}
		if el := time.Since(start); el > 900*time.Millisecond {
			t.Errorf("returned after %v, want soon after the cancel at 200ms, before the timeout", el)
		}
		if lingers(mark) {
			t.Error("the compiler's child outlived the cancel")
		}
		if left, _ := os.ReadDir(tmp); len(left) > 0 {
			t.Errorf("the killed compiler left %s in TMPDIR", left[0].Name())
		}
	})

	t.Run("pch", func(t *testing.T) {
		const mark = "3141.5926503"
		p := Profile{Name: "hang-pch", Compiler: hangingCompiler(t, mark), Libstdcxx: true, GNU: true, PCH: true}
		var c pch
		// pchOf asks the compiler its --version.
		within(t, 10*time.Second, func() { c = pchOf(t, p, body) })

		// No PCH yet: the build starts one in the background.
		var b Build
		var err error
		within(t, 10*time.Second, func() { b, err = Compile(context.Background(), src, p) })
		if err != nil {
			t.Fatal(err)
		}
		timedOut(t, b)
		within(t, 10*time.Second, func() { waitPCH(c) })
		if why, err := os.ReadFile(c.failed()); err != nil || !strings.Contains(string(why), "compile timed out") {
			t.Errorf("PCH build: failed %q (%v), want the timeout", why, err)
		}

		// A PCH in place: the build loads it.
		if err := os.Remove(c.failed()); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(c.file(), []byte("not really a PCH"), 0o644); err != nil {
			t.Fatal(err)
		}
		within(t, 10*time.Second, func() { b, err = Compile(context.Background(), src, p) })
		if err != nil {
			t.Fatal(err)
		}
		if !b.PCH {
			t.Error("build with a PCH in place did not load it")
		}
		timedOut(t, b)
		if lingers(mark) {
			t.Error("the compiler's child outlived the timeout")
		}
	})
}
