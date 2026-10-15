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
	"slices"
	"strings"
	"syscall"
	"time"

	"katharsis/internal/units"
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
	// FloatTolerance > 0 accepts numbers within that absolute or relative
	// error instead of exactly (Checker.Tol).
	FloatTolerance float64
	// AnyCase compares output tokens regardless of letter case, for
	// statements that accept answers in any case (Checker.AnyCase).
	AnyCase bool
	// FileIO is the stem the problem's statement names for file I/O
	// (catalog.Problem.FileIO; "paint" for paint.in), "" for none. With it
	// every run finds <stem>.in in its working directory, as on the real
	// grader, however the source opens its files; see Run.
	FileIO string
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

func (l Limits) checker() Checker { return Checker{Tol: l.FloatTolerance, AnyCase: l.AnyCase} }

type Result struct {
	Verdict Verdict
	CPU     time.Duration // user + system
	Wall    time.Duration
	MaxRSS  int64  // bytes
	Reason  string // one line: signal, sanitizer summary, limit hit
	Diff    *Diff  // set for WA
	Stderr  string // head and tail of stderr, bounded

	// Rerun is set when JudgeFull judged this test again alone after a
	// TLE in the parallel pass; this is the rerun's result.
	Rerun bool
}

// Case is one test: an input file and the expected output file. FileIO is
// the stem the source names for file I/O (problem.FileIO), "" when it
// names none.
type Case struct {
	In, Out string
	FileIO  string
}

// stderrCap bounds the stderr a Result keeps: this much of its head and as
// much of its tail, so a sanitizer report after megabytes of debug prints
// is still there.
const stderrCap = 16 << 10

// DefaultOutputLimit applies when Limits.Output is zero.
const DefaultOutputLimit = 64 << 20

// Run executes bin on one case and judges the result.
//
// Input and output: the input is always on stdin. When a file I/O stem is
// known, from the source (c.FileIO) or the problem (lim.FileIO), the input
// is also <stem>.in in the run's working directory, and the output is
// <stem>.out if the program wrote anything there, else stdout. A program
// is judged on its answer, not on which I/O style its problem's era used.
//
// Time: TLE is decided on CPU time against lim.budget(p). The watchdog
// kills a run once its CPU time passes the budget under a sample profile
// (Slack > 1: timing there is a hint), or twice the budget under the
// judged one, so a slow-but-finishing program reports how slow it was.
// The wall clock (2x budget, plus 0.5 s) only stops a program that waits
// instead of computing.
//
// Memory: the watchdog kills a run whose memory grows past memCap (MLE); a
// run under that cap but over Mem is judged MLE on its peak afterwards, and
// a binary whose static data alone is over Mem is MLE without running.
//
// Processes: the program leads its own process group, which is killed when
// the run ends, so whatever it forked dies with it.
func Run(ctx context.Context, bin string, c Case, lim Limits, p Profile) (Result, error) {
	if lim.Output <= 0 {
		lim.Output = DefaultOutputLimit
	}
	mem := lim.Mem // what the run is held to; 0 for nothing
	if p.Sanitized {
		mem = 0
	}
	if mem > 0 {
		if n := staticData(bin); n > mem {
			return Result{Verdict: MLE, Reason: fmt.Sprintf("static data %s > %s", units.Mem(n), units.Mem(mem))}, nil
		}
	}

	work, err := os.MkdirTemp("", "katharsis-run-")
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = os.RemoveAll(work) }()
	stems := fileStems(c.FileIO, lim.FileIO)
	for _, s := range stems {
		if err := copyFile(c.In, filepath.Join(work, s+".in")); err != nil {
			return Result{}, err
		}
	}
	stdin, err := os.Open(c.In)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = stdin.Close() }()

	memLimit := memCap(mem)
	budget := lim.budget(p)
	cpuKill := 2 * budget
	if p.Slack > 1 {
		cpuKill = budget
	}
	wall := 2*budget + 500*time.Millisecond
	runCtx, cancel := context.WithTimeout(ctx, wall)
	defer cancel()

	stdout := &capWriter{max: lim.Output, onOverflow: cancel}
	stderr := &headTail{n: stderrCap}
	outPipe, err := newPipe(stdout)
	if err != nil {
		return Result{}, err
	}
	errPipe, err := newPipe(stderr)
	if err != nil {
		outPipe.finish(0)
		return Result{}, err
	}
	cmd := exec.CommandContext(runCtx, bin)
	cmd.Dir = work
	cmd.Stdin = stdin
	cmd.Stdout = outPipe.w
	cmd.Stderr = errPipe.w
	// Leak detection is unsupported on Apple silicon and noisy anyway. A
	// short allocation context keeps use-after-free reports quick to
	// symbolize. libstdc++'s assertions abort without saying where the
	// player's code called in; handle_abort has ASan print the stack.
	asan := "ASAN_OPTIONS=detect_leaks=0:halt_on_error=1:print_summary=1:malloc_context_size=5"
	if p.Libstdcxx {
		asan += ":handle_abort=1"
	}
	cmd.Env = append(os.Environ(), asan, "UBSAN_OPTIONS=print_stacktrace=1:halt_on_error=1")
	cmd.WaitDelay = time.Second
	inGroup(cmd)

	start := time.Now()
	err = cmd.Start()
	outPipe.closeWrite()
	errPipe.closeWrite()
	if err != nil {
		outPipe.finish(0)
		errPipe.finish(0)
		return Result{}, err
	}
	stopWatch := watch(cmd.Process, memLimit, cpuKill)
	runErr := cmd.Wait()
	killed := stopWatch()
	res := Result{Wall: time.Since(start)}
	_ = killGroup(cmd.Process.Pid) // whatever it forked, and its hold on the pipes
	outPipe.finish(time.Second)
	errPipe.finish(time.Second)
	res.Stderr = stderr.String()
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
		res.Verdict, res.Reason = OLE, fmt.Sprintf("output over %s", units.Mem(lim.Output))
		return res, nil
	case killed.Mem > 0:
		limit := units.Mem(memLimit) + " cap"
		if mem > 0 {
			limit = units.Mem(mem)
		}
		res.Verdict, res.Reason = MLE, fmt.Sprintf("killed at %s > %s", units.Mem(killed.Mem), limit)
		return res, nil
	case res.CPU > budget:
		res.Verdict, res.Reason = TLE, fmt.Sprintf("cpu %s > %s", ms(res.CPU), ms(budget))
		return res, nil
	case killed.CPU:
		res.Verdict, res.Reason = TLE, fmt.Sprintf("cpu over %s", ms(budget))
		return res, nil
	case runCtx.Err() != nil && sanitizerReport.MatchString(res.Stderr):
		// It crashed within its CPU budget and was still writing the
		// report (symbolizing a stack can take seconds) when the wall
		// clock ran out.
		res.Verdict, res.Reason = RE, crashReason(res.Stderr, cmd.ProcessState, p)
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
		res.Verdict, res.Reason = RE, crashReason(res.Stderr, cmd.ProcessState, p)
		return res, nil
	case mem > 0 && res.MaxRSS > mem:
		res.Verdict, res.Reason = MLE, fmt.Sprintf("peak %s > %s", units.Mem(res.MaxRSS), units.Mem(mem))
		return res, nil
	}

	got, wrote := stdout.buf.Bytes(), false
	for _, s := range stems {
		b, err := readCapped(filepath.Join(work, s+".out"), lim.Output)
		switch {
		case errors.Is(err, os.ErrNotExist):
			continue
		case err != nil:
			res.Verdict, res.Reason = OLE, err.Error()
			return res, nil
		}
		wrote = true
		if len(bytes.TrimSpace(b)) > 0 {
			got = b
			break
		}
	}
	want, err := os.ReadFile(c.Out)
	if err != nil {
		return res, err
	}
	if ok, d := lim.checker().Check(want, got); ok {
		res.Verdict = AC
	} else {
		res.Verdict, res.Diff = WA, d
		res.Reason = fmt.Sprintf("line %d", d.Line)
		if len(stems) > 0 && !wrote && len(bytes.TrimSpace(got)) == 0 {
			res.Reason = "no " + stems[0] + ".out written"
		}
	}
	return res, nil
}

// fileStems is the stems a run provides <stem>.in for: the source's, then
// the problem's, each once. A stem must be a plain file name.
func fileStems(stems ...string) []string {
	var out []string
	for _, s := range stems {
		ok := s != "" && s != "." && s != ".." && !strings.ContainsAny(s, "/\\\x00")
		if ok && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// pipe carries one of a run's output streams to dst. The program gets the
// write end as a plain file, so exec starts no copying goroutine of its
// own and Wait returns when the program exits, not when the last process
// holding the pipe lets go (a forked child; see killGroup).
type pipe struct {
	r, w *os.File
	done chan struct{}
}

func newPipe(dst io.Writer) (*pipe, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	p := &pipe{r: r, w: w, done: make(chan struct{})}
	go func() {
		defer close(p.done)
		_, _ = io.Copy(dst, r)
	}()
	return p, nil
}

// closeWrite drops the parent's copy of the write end once the child has
// its own, so the read end sees EOF when the child's side closes.
func (p *pipe) closeWrite() { _ = p.w.Close() }

// finish waits up to d for the stream to end, then closes the read end,
// which stops the copy: a process outside the run's group (one that called
// setsid) may hold the pipe open for good. dst is safe to read after.
func (p *pipe) finish(d time.Duration) {
	_ = p.w.Close()
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-p.done:
	case <-t.C:
	}
	_ = p.r.Close()
	<-p.done
}

// capWriter keeps the first max bytes written to it and drops the rest,
// calling onOverflow, if set, at the first byte dropped. Only the pipe's
// copying goroutine writes to it, and pipe.finish returns after that
// goroutine does, so buf is safe to read after finish.
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

// headTail keeps the first n bytes written to it and the last n, so a
// crash report survives any amount of debug output before it.
type headTail struct {
	n     int
	head  []byte
	tail  []byte // the bytes after head; trimmed to the last n now and then
	total int64
}

func (h *headTail) Write(p []byte) (int, error) {
	h.total += int64(len(p))
	q := p
	if room := h.n - len(h.head); room > 0 {
		k := min(room, len(q))
		h.head = append(h.head, q[:k]...)
		q = q[k:]
	}
	if len(q) >= h.n {
		h.tail = append(h.tail[:0], q[len(q)-h.n:]...)
		return len(p), nil
	}
	h.tail = append(h.tail, q...)
	if len(h.tail) > 2*h.n {
		h.tail = append(h.tail[:0], h.tail[len(h.tail)-h.n:]...)
	}
	return len(p), nil
}

func (h *headTail) String() string {
	tail := h.tail
	if len(tail) > h.n {
		tail = tail[len(tail)-h.n:]
	}
	cut := h.total - int64(len(h.head)) - int64(len(tail))
	if cut == 0 {
		return string(h.head) + string(tail)
	}
	return fmt.Sprintf("%s\n… %s of stderr cut …\n%s", h.head, units.Mem(cut), tail)
}

func readCapped(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err == nil && int64(len(b)) > max {
		return nil, fmt.Errorf("%s over %s", filepath.Base(path), units.Mem(max))
	}
	return b, err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

func tv(t syscall.Timeval) time.Duration {
	return time.Duration(t.Sec)*time.Second + time.Duration(t.Usec)*time.Microsecond
}

func ms(d time.Duration) string { return fmt.Sprintf("%dms", d.Milliseconds()) }

// Bytes formats a size the way the HUD shows it: 312B, 1.2K, 48M.
//
// Deprecated: use units.Mem, which this is.
func Bytes(n int64) string { return units.Mem(n) }
