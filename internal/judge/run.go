package judge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

type Verdict string

const (
	AC  Verdict = "AC"
	WA  Verdict = "WA"
	TLE Verdict = "TLE"
	MLE Verdict = "MLE"
	OLE Verdict = "OLE" // output limit exceeded
	RE  Verdict = "RE"
	CE  Verdict = "CE"
)

type Limits struct {
	Time time.Duration
	// Mem is in bytes. Sanitizer builds are not held to it (shadow memory)
	// and only HardMemCap stops them.
	Mem    int64
	Output int64 // bytes of stdout or output file

	// Scale multiplies the CPU budget; zero means 1. An M4 runs contest
	// code faster than judge hardware, so a game that wants a local AC to
	// predict a judge AC sets Scale below 1 for a stricter local limit.
	// JudgeFull's parallel runs read about 15% more CPU than sequential
	// ones (see Workers), which already eats into that headroom.
	Scale float64
	// FloatTolerance > 0 checks output with CheckFloat at that absolute or
	// relative error instead of exactly.
	FloatTolerance float64
	// AnyCase folds yes/no answer words to one case on both sides before
	// checking, for statements that accept them in any case. Other words
	// still compare exactly.
	AnyCase bool
}

// budget is the CPU time a run may use before it is TLE: Time × Slack ×
// Scale.
func (l Limits) budget(p Profile) time.Duration {
	slack, scale := p.Slack, l.Scale
	if slack <= 0 {
		slack = 1
	}
	if scale <= 0 {
		scale = 1
	}
	return time.Duration(float64(l.Time) * slack * scale)
}

type Result struct {
	Verdict Verdict
	CPU     time.Duration // user + system
	Wall    time.Duration
	MaxRSS  int64  // bytes
	Reason  string // one line: signal, sanitizer summary, limit hit
	Diff    *Diff  // set for WA
	Stderr  string // head of stderr, bounded

	// Rerun is set when JudgeFull judged this test again alone after a
	// borderline TLE in the parallel pass; this is the rerun's result.
	Rerun bool
}

// Case is one test: an input file and the expected output file. FileIO is
// the stem for freopen-style programs ("" means stdin/stdout).
type Case struct {
	In, Out string
	FileIO  string
}

const stderrCap = 16 << 10

// DefaultOutputLimit applies when Limits.Output is zero.
const DefaultOutputLimit = 64 << 20

// Run executes bin on one case and judges the result.
//
// TLE is decided on CPU time against lim.budget(p); the wall-clock kill
// happens later (2x that, plus 0.5 s) so a slow-but-finishing program
// reports how slow it was instead of just "killed". A watchdog kills a run
// whose memory grows past memCap (MLE) while it runs; a run under that cap
// but over Mem is judged MLE on its peak afterwards.
func Run(ctx context.Context, bin string, c Case, lim Limits, p Profile) (Result, error) {
	work, err := os.MkdirTemp("", "katharsis-run-")
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = os.RemoveAll(work) }()

	inPath := c.In
	if c.FileIO != "" {
		inPath = filepath.Join(work, c.FileIO+".in")
		if err := copyFile(c.In, inPath); err != nil {
			return Result{}, err
		}
	}
	stdin, err := os.Open(inPath)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = stdin.Close() }()

	if lim.Output <= 0 {
		lim.Output = DefaultOutputLimit
	}
	mem := lim.Mem // what the run is held to; 0 for nothing
	if p.Sanitized {
		mem = 0
	}
	memLimit := memCap(mem)
	budget := lim.budget(p)
	wall := 2*budget + 500*time.Millisecond
	runCtx, cancel := context.WithTimeout(ctx, wall)
	defer cancel()

	stdout := &capWriter{max: lim.Output, onOverflow: cancel}
	stderr := &capWriter{max: stderrCap}
	cmd := exec.CommandContext(runCtx, bin)
	cmd.Dir = work
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(),
		// Leak detection is unsupported on Apple silicon and noisy anyway.
		"ASAN_OPTIONS=detect_leaks=0:halt_on_error=1:print_summary=1",
		"UBSAN_OPTIONS=print_stacktrace=1:halt_on_error=1",
	)
	cmd.WaitDelay = time.Second

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return Result{}, err
	}
	stopWatch := watchMemory(cmd.Process, memLimit)
	runErr := cmd.Wait()
	killedAt := stopWatch()
	res := Result{Wall: time.Since(start), Stderr: stderr.buf.String()}
	if ps := cmd.ProcessState; ps != nil {
		if ru, ok := ps.SysUsage().(*syscall.Rusage); ok {
			res.CPU = tv(ru.Utime) + tv(ru.Stime)
			res.MaxRSS = ru.Maxrss
			if runtime.GOOS == "linux" {
				res.MaxRSS *= 1024 // KiB on Linux, bytes on macOS
			}
		}
	}
	if ctx.Err() != nil {
		return res, ctx.Err() // superseded (e.g. the file was saved again)
	}

	switch {
	case stdout.overflow:
		res.Verdict, res.Reason = OLE, fmt.Sprintf("output over %s", Bytes(lim.Output))
		return res, nil
	case killedAt > 0:
		limit := Bytes(memLimit) + " cap"
		if mem > 0 {
			limit = Bytes(mem)
		}
		res.Verdict, res.Reason = MLE, fmt.Sprintf("killed at %s > %s", Bytes(killedAt), limit)
		return res, nil
	case res.CPU > budget:
		res.Verdict, res.Reason = TLE, fmt.Sprintf("cpu %s > %s", ms(res.CPU), ms(budget))
		return res, nil
	case runCtx.Err() != nil:
		// Killed on the wall clock within its CPU budget: blocked, or
		// starved by a busy machine.
		res.Verdict, res.Reason = TLE, fmt.Sprintf("wall %s > %s, cpu %s", ms(res.Wall), ms(wall), ms(res.CPU))
		return res, nil
	case runErr != nil:
		var exit *exec.ExitError
		if !errors.As(runErr, &exit) {
			return res, runErr
		}
		res.Verdict, res.Reason = RE, crashReason(exit, res.Stderr)
		return res, nil
	case mem > 0 && res.MaxRSS > mem:
		res.Verdict, res.Reason = MLE, fmt.Sprintf("peak %s > %s", Bytes(res.MaxRSS), Bytes(mem))
		return res, nil
	}

	got := stdout.buf.Bytes()
	if c.FileIO != "" {
		got, err = readCapped(filepath.Join(work, c.FileIO+".out"), lim.Output)
		if errors.Is(err, os.ErrNotExist) {
			res.Verdict, res.Reason = WA, "no "+c.FileIO+".out written"
			return res, nil
		}
		if err != nil {
			res.Verdict, res.Reason = OLE, err.Error()
			return res, nil
		}
	}
	want, err := os.ReadFile(c.Out)
	if err != nil {
		return res, err
	}
	if lim.AnyCase {
		want, got = FoldYesNo(want), FoldYesNo(got)
	}
	if ok, d := CheckFloat(want, got, lim.FloatTolerance); ok {
		res.Verdict = AC
	} else {
		res.Verdict, res.Diff = WA, d
		res.Reason = fmt.Sprintf("line %d", d.Line)
	}
	return res, nil
}

// crashReason prefers the sanitizer's one-line summary over the raw exit
// status, since "exit 1" says nothing about a heap overflow.
func crashReason(exit *exec.ExitError, stderr string) string {
	// SUMMARY comes last, so a long report (a 255-frame stack overflow) can
	// push it past the stderr cap; the ERROR header near the top names the
	// kind of fault and is the fallback.
	header := ""
	for line := range strings.Lines(stderr) {
		line = strings.TrimSpace(line)
		if s, ok := strings.CutPrefix(line, "SUMMARY: "); ok {
			return s
		}
		if i := strings.Index(line, "runtime error: "); i >= 0 {
			return "UB: " + line[i+len("runtime error: "):]
		}
		if _, s, ok := strings.Cut(line, "libc++ Hardening assertion "); ok {
			if _, msg, ok := strings.Cut(s, " failed: "); ok {
				s = msg
			}
			return "libc++: " + s
		}
		if i := strings.Index(line, "ERROR: AddressSanitizer: "); i >= 0 && header == "" {
			header = line[i+len("ERROR: "):]
		}
	}
	if header != "" {
		kind, _, _ := strings.Cut(header, " on ")
		return kind
	}
	if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return ws.Signal().String()
	}
	return fmt.Sprintf("exit %d", exit.ExitCode())
}

// capWriter keeps the first max bytes written to it and drops the rest,
// calling onOverflow, if set, at the first byte dropped. Only exec's copying
// goroutine writes to it, and Wait returns after that goroutine does, so buf
// is safe to read after Wait.
type capWriter struct {
	buf        bytes.Buffer
	max        int64
	overflow   bool
	onOverflow func()
}

func (w *capWriter) Write(p []byte) (int, error) {
	room := w.max - int64(w.buf.Len())
	if int64(len(p)) <= room {
		return w.buf.Write(p)
	}
	w.buf.Write(p[:room])
	if !w.overflow && w.onOverflow != nil {
		w.onOverflow()
	}
	w.overflow = true
	return len(p), nil
}

func readCapped(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err == nil && int64(len(b)) > max {
		return nil, fmt.Errorf("%s over %s", filepath.Base(path), Bytes(max))
	}
	return b, err
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

func tv(t syscall.Timeval) time.Duration {
	return time.Duration(t.Sec)*time.Second + time.Duration(t.Usec)*time.Microsecond
}

func ms(d time.Duration) string { return fmt.Sprintf("%dms", d.Milliseconds()) }

// Bytes formats a size the way the HUD shows it: 312B, 1.2K, 48M.
func Bytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0fM", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%dB", n)
}
