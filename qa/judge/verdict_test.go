//go:build qa

package judgeqa

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"katharsis/internal/judge"
	"katharsis/internal/problem"
)

// expect is what a trustworthy judge reports. why says where the
// expectation comes from when it is not obvious.
type expect struct {
	v      judge.Verdict
	reason []string // each must appear in Reason (any of "a|b" alternatives)
	why    string
}

func ex(v judge.Verdict, reason ...string) *expect { return &expect{v: v, reason: reason} }

func (e *expect) because(why string) *expect { e.why = why; return e }

type vcase struct {
	name, body string
	in, out    string
	lim        judge.Limits // zero fields: Time 1s, Mem 256M, Output 64M
	// Per profile kind; nil skips that kind.
	full, asan, hard *expect
	// fileIO overrides the stem; "" derives it from the source, as
	// Judge and Submit do.
	fileIO string
	heavy  bool // allocates GBs: run alone
}

func kind(p judge.Profile) string {
	switch {
	case p.Name == judge.Full.Name, p.Name == judge.GCCFull.Name:
		return "full"
	case sanitizes(p, "address"):
		return "asan"
	}
	return "hard"
}

func (c vcase) expectFor(p judge.Profile) *expect {
	switch kind(p) {
	case "full":
		return c.full
	case "asan":
		return c.asan
	}
	return c.hard
}

func all(e *expect) (f, a, h *expect) { return e, e, e }

func sanOnly(a, h *expect) (f, aa, hh *expect) { return nil, a, h }

func mk(name, body, in, out string, f, a, h *expect) vcase {
	return vcase{name: name, body: body, in: in, out: out, full: f, asan: a, hard: h}
}

func verdictCases() []vcase {
	var cs []vcase
	add := func(c vcase) { cs = append(cs, c) }
	tokensCF := "Codeforces' standard checkers (wcmp, ncmp) compare whitespace-separated tokens"

	// ---- AC / WA and the checker -------------------------------------
	f, a, h := all(ex(judge.AC))
	add(mk("ac_ab", progAB, "1 2\n", "3\n", f, a, h))
	f, a, h = all(ex(judge.WA, "line 3"))
	add(mk("wa_line3", progCat, "1\n2\n4\n", "1\n2\n3\n", f, a, h))
	f, a, h = all(ex(judge.AC))
	add(mk("ws_trailing_blank_lines", progCat, "3   \n\n\n", "3\n", f, a, h))
	add(mk("no_final_newline", progNoNewline, "1 2\n", "3\n", f, a, h))
	add(mk("crlf_output", progCat, "1\r\n2\r\n", "1\n2\n", f, a, h))
	f, a, h = all(ex(judge.WA, "line 2"))
	add(mk("eof_got_short", progCat, "1\n", "1\n2\n", f, a, h))
	add(mk("eof_got_long", progCat, "1\n2\n", "1\n", f, a, h))
	f, a, h = all(ex(judge.WA, "line 1"))
	add(mk("empty_output", progEmpty, "", "5\n", f, a, h))
	f, a, h = all(ex(judge.AC).because(tokensCF))
	add(mk("tokens_one_line_vs_per_line", progCat, "1 2 3\n", "1\n2\n3\n", f, a, h))
	add(mk("double_inner_space", progCat, "1  2\n", "1 2\n", f, a, h))
	add(mk("leading_space", progCat, " 1\n", "1\n", f, a, h))

	// ---- TLE -----------------------------------------------------------
	add(vcase{name: "tle_spin", body: progSpin, full: ex(judge.TLE), asan: ex(judge.TLE), hard: ex(judge.TLE)})
	add(mk("tle_burn_1.1x", progBurn, "1100\n", "ok\n",
		ex(judge.TLE), ex(judge.AC).because("Slack 4: sample timing is a hint"), ex(judge.AC).because("Slack 2")))
	add(mk("tle_burn_1.5x", progBurn, "1500\n", "ok\n",
		ex(judge.TLE), ex(judge.AC).because("Slack 4"), ex(judge.AC).because("Slack 2")))
	add(mk("tle_burn_3x", progBurn, "3000\n", "ok\n",
		ex(judge.TLE), ex(judge.AC).because("Slack 4"), ex(judge.TLE)))
	add(mk("sleep_1.5x", progSleep, "1500000\n", "ok\n",
		ex(judge.TLE).because("judges also bound wall time (inference: CF idleness/real-time limit)"), nil, nil))
	add(mk("sleep_3x", progSleep, "3000000\n", "ok\n",
		ex(judge.TLE, "wall").because("a wall-clock kill should not claim cpu > limit"), nil, nil))

	// ---- MLE (limit 256M) ----------------------------------------------
	design := "sanitized builds are held to HardMemCap only (by design)"
	add(vcase{name: "mle_grow_1G_touched", body: progGrow, in: "1024\n", out: "ok\n",
		full: ex(judge.MLE, "killed at"), asan: ex(judge.AC).because(design), hard: ex(judge.MLE), heavy: true})
	add(vcase{name: "mle_pushback_100M_ints", body: progPushBack, in: "100\n", out: "0\n",
		full: ex(judge.MLE), asan: ex(judge.AC).because(design), hard: ex(judge.MLE), heavy: true})
	add(vcase{name: "mle_huge_1G_touched", body: progHuge, in: "1024 1\n", out: "ok\n",
		full: ex(judge.MLE), asan: ex(judge.AC).because(design), hard: ex(judge.MLE), heavy: true})
	add(vcase{name: "mle_huge_1G_untouched", body: progHuge, in: "1024 0\n", out: "ok\n",
		full:  ex(judge.MLE).because("CF (Windows) counts committed memory; RLIMIT_AS judges fail the malloc"),
		heavy: true})
	add(vcase{name: "mle_reserve_800M_untouched", body: progReserve, in: "200\n", out: "0\n",
		full: ex(judge.MLE).because("CF counts committed memory; RLIMIT_AS judges throw bad_alloc"), heavy: true})
	add(vcase{name: "mle_static_1.2G_untouched", body: progStatic, in: "1000\n", out: "0\n",
		full: ex(judge.MLE).because("a 1.2 GB .bss is committed/mapped at load on CF and RLIMIT_AS judges"), heavy: true})
	add(vcase{name: "mle_peak_300M", body: progGrow, in: "300\n", out: "ok\n",
		full: ex(judge.MLE, "peak"), heavy: true})
	add(vcase{name: "mle_peak_270M", body: progGrow, in: "270\n", out: "ok\n",
		full: ex(judge.MLE, "peak"), heavy: true})

	// ---- RE --------------------------------------------------------------
	add(mk("re_segv_wild", progSegv, "16\n", "",
		ex(judge.RE, "segmentation fault|bus error"), ex(judge.RE, "SEGV", "sol.cpp:6"), ex(judge.RE)))
	add(mk("re_null", progSegv, "0\n", "",
		ex(judge.RE), ex(judge.RE, "null pointer", "sol.cpp:6"), ex(judge.RE, "null pointer", "sol.cpp:6")))
	f, a, h = all(ex(judge.RE, "abort"))
	add(mk("re_abort", progAbort, "1\n", "0\n", f, a, h))
	f, a, h = all(ex(judge.RE, "sol.cpp", "6").because("assert prints file and line; the player needs them"))
	add(mk("re_assert", progAssert, "7\n", "7\n", f, a, h))
	f, a, h = all(ex(judge.RE, "runtime_error", "boom").because("libc++abi prints the exception type and what()"))
	add(mk("re_uncaught_exception", progThrow, "1\n", "0\n", f, a, h))
	f, a, h = all(ex(judge.RE, "out_of_range"))
	add(mk("re_vector_at", progAt, "10\n", "0\n", f, a, h))
	add(vcase{name: "re_stack_overflow_1e7", body: progRecurse, in: "10000000\n", out: "10000000\n",
		full: ex(judge.RE, "stack").because("the player needs 'stack overflow', not a bare segfault"),
		asan: ex(judge.RE, "stack-overflow", "sol.cpp"), hard: ex(judge.RE, "stack"),
		lim: judge.Limits{Mem: 2 << 30}})
	sigfpe := "x86-64 judges raise SIGFPE on integer division by zero"
	add(mk("re_div_by_zero", progDiv, "1 0\n", "0\n",
		ex(judge.RE).because(sigfpe), ex(judge.RE, "division by zero", "sol.cpp:5"), ex(judge.RE, "division by zero", "sol.cpp:5")))
	add(mk("re_div_intmin_by_minus1", progDiv, "-2147483648 -1\n", "-2147483648\n",
		ex(judge.RE).because(sigfpe+" (and on INT_MIN / -1)"), ex(judge.RE, "cannot be represented"), ex(judge.RE, "cannot be represented")))
	add(mk("re_mod_by_zero", progMod, "5 0\n", "5\n",
		ex(judge.RE).because(sigfpe), ex(judge.RE, "division by zero"), ex(judge.RE, "division by zero")))
	add(mk("ub_signed_overflow", progOverflow, "2147483647\n", "-2147483648\n",
		nil, ex(judge.RE, "signed integer overflow", "sol.cpp:5"), ex(judge.RE, "signed integer overflow", "sol.cpp:5")))
	// Against libstdc++ (_GLIBCXX_ASSERTIONS) the library's bounds check
	// stops v[i] before ASan sees the access, and is the report.
	bounds := "|libstdc++: __n < this->size()"
	add(mk("re_heap_oob", progHeapOOB, "10\n", "1\n",
		nil, ex(judge.RE, "heap-buffer-overflow"+bounds, "sol.cpp:7"), ex(judge.RE, "out of bounds", "sol.cpp:7")))
	add(mk("re_container_oob", progContainerOOB, "6\n", "1 8\n",
		nil, ex(judge.RE, "overflow"+bounds, "sol.cpp:8"), ex(judge.RE, "out of bounds", "sol.cpp:8")))
	add(mk("re_global_oob", progGlobalOOB, "10\n", "1\n",
		nil, ex(judge.RE, "sol.cpp:6"), ex(judge.RE, "sol.cpp:6")))
	add(mk("re_use_after_free", progUAF, "0\n", "3\n",
		nil, ex(judge.RE, "heap-use-after-free", "sol.cpp:7"), nil))

	// ---- OLE, stderr ------------------------------------------------------
	f, a, h = all(ex(judge.OLE))
	add(mk("ole_infinite_print", progSpam, "", "", f, a, h))
	f, a, h = all(ex(judge.AC).because("stderr is not output"))
	// Writing 64 MiB costs about a second of CPU (1005-1136 ms at -O2
	// seen), so a 1 s limit would judge its speed; the case is about where
	// stderr goes.
	stderr := mk("stderr_64MiB_then_answer", progStderr, "64 1 2\n", "3\n", f, a, h)
	stderr.lim = judge.Limits{Time: 4 * time.Second}
	add(stderr)
	add(mk("re_after_20KB_debug_stderr", progDebugThenOOB, "800 10\n", "1\n",
		nil, ex(judge.RE, "heap-buffer-overflow"+bounds, "sol.cpp:8").because("debug prints to cerr are routine; the report must survive them"),
		ex(judge.RE, "out of bounds")))

	// ---- processes ---------------------------------------------------------
	f, a, h = all(ex(judge.AC).because("or RE (fork blocked); never a judge error, never a leaked child"))
	add(mk("fork_child_holds_stdout", progForkChild, "1 2\n", "3\n", f, a, h))
	add(mk("fork_detached_child", progForkDetached, "1 2\n", "3\n", f, a, h))

	// ---- file I/O: the problem is USACO "milk2" (file I/O) -------------------
	usacoOK := "USACO runs this with milk2.in present and reads milk2.out"
	add(vcase{name: "fileio_freopen", body: progFreopen, in: "1 2\n", out: "3\n",
		full: ex(judge.AC), asan: ex(judge.AC)})
	add(vcase{name: "fileio_usaco_guide_setIO", body: progSetIO, in: "1 2\n", out: "3\n",
		full: ex(judge.AC).because(usacoOK), asan: ex(judge.AC).because(usacoOK)})
	add(vcase{name: "fileio_fstream", body: progFstream, in: "1 2\n", out: "3\n",
		full: ex(judge.AC).because(usacoOK), asan: ex(judge.AC).because(usacoOK)})
	add(vcase{name: "fileio_wrong_stem_milk", body: progFreopenWrongStem, in: "1 2\n", out: "3\n",
		full: ex(judge.WA).because("USACO only provides milk2.in; milk.in does not exist")})
	add(vcase{name: "stdio_program_on_fileio_problem", body: progAB, in: "1 2\n", out: "3\n",
		full: ex(judge.WA).because("pre-2020 USACO reads milk2.out only")})
	// ---- a stdio problem (Codeforces / new USACO) ----
	add(vcase{name: "stdio_freopen_in_block_comment", body: progFreopenBlockComment, in: "1 2\n", out: "3\n",
		full: ex(judge.AC).because("the freopen is commented out; the program is plain stdio"), asan: ex(judge.AC)})
	add(vcase{name: "stdio_ifndef_ONLINE_JUDGE_template", body: progOnlineJudge, in: "1 2\n", out: "3\n",
		full: ex(judge.AC).because("Codeforces and AtCoder compile with -DONLINE_JUDGE"), asan: ex(judge.AC)})

	// ---- portability: what g++ on x86-64 accepts -----------------------------
	gnu := "g++/libstdc++ (every CF and USACO compiler) accepts it"
	for _, g := range []struct{ name, body string }{
		{"gnu_bits_stdcxx_h", progBits}, {"gnu_std___gcd", progGcd}, {"gnu_std___lg", progLg},
		{"gnu_pb_ds", progPbds}, {"gnu_pragma_target_avx2", progPragmaTarget},
	} {
		add(vcase{name: g.name, body: g.body, in: "1 2\n", out: "3\n",
			full: ex(judge.AC).because(gnu), asan: ex(judge.AC).because(gnu)})
	}
	x87 := "x86 long double has a 64-bit mantissa; Apple arm64 long double is double"
	add(vcase{name: "long_double_sqrtl_1e18", body: progSqrtl, in: "999999999999999999\n", out: "999999999\n",
		full: ex(judge.AC).because(x87)})
	add(vcase{name: "long_double_2p53_plus_1", body: progLongDoubleSum, in: "9007199254740992\n", out: "9007199254740993\n",
		full: ex(judge.AC).because(x87)})
	add(vcase{name: "long_is_32bit_on_cf_windows", body: progLong, in: "100000 100000\n", out: "10000000000\n",
		full: ex(judge.WA).because("on CF's Windows g++ long is 32 bits (LLP64), so this overflows there")})
	return cs
}

var heavyMu sync.Mutex

func TestQAVerdicts(t *testing.T) {
	needCompiler(t)
	for _, c := range verdictCases() {
		for _, p := range profiles {
			e := c.expectFor(p)
			if e == nil {
				continue
			}
			t.Run(c.name+"/"+p.Name, func(t *testing.T) {
				t.Parallel()
				if c.heavy {
					heavyMu.Lock()
					defer heavyMu.Unlock()
				}
				lim := c.lim
				if lim.Time == 0 {
					lim.Time = time.Second
				}
				if lim.Mem == 0 {
					lim.Mem = 256 << 20
				}
				if lim.Output == 0 {
					lim.Output = judge.DefaultOutputLimit
				}
				b := build(t, c.name, c.body, p)
				if !b.OK {
					note(c.name, p.Name, "CE "+firstLine(b.Log))
					if e.v != judge.CE {
						t.Errorf("CE, want %s (%s)\n%s", e.v, e.why, b.Log)
					}
					return
				}
				cs := writeCase(t, c.in, c.out)
				cs.FileIO = c.fileIO
				if cs.FileIO == "" {
					cs.FileIO = problem.FileIO(b.Src)
				}
				r, err := judge.Run(context.Background(), b.Binary, cs, lim, p)
				record(c.name, p.Name, r, err)
				if err != nil {
					t.Fatalf("Run error (not a verdict): %v", err)
				}
				if r.Verdict != e.v {
					t.Errorf("verdict %s %q, want %s (%s)\nstderr head:\n%s", r.Verdict, r.Reason, e.v, e.why, head(r.Stderr, 12))
					return
				}
				for _, want := range e.reason {
					if !anyOf(r.Reason, strings.Split(want, "|")...) {
						t.Errorf("Reason %q lacks %q (%s)\nstderr head:\n%s", r.Reason, want, e.why, head(r.Stderr, 12))
					}
				}
			})
		}
	}
}

func anyOf(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func head(s string, n int) string {
	ls := strings.Split(s, "\n")
	if len(ls) > n {
		ls = ls[:n]
	}
	return strings.Join(ls, "\n")
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return l
}

// Children a solution forks must not outlive its run.
func TestQAForkLeak(t *testing.T) {
	needCompiler(t)
	for _, tc := range []struct{ name, body, marker string }{
		{"fork_child_holds_stdout", progForkChild, "sleep 6.4242"},
		{"fork_detached_child", progForkDetached, "sleep 7.4242"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer exec.Command("pkill", "-f", tc.marker).Run()
			b := build(t, tc.name+"_leak", tc.body, judge.Full)
			c := writeCase(t, "1 2\n", "3\n")
			start := time.Now()
			r, err := judge.Run(context.Background(), b.Binary, c, judge.Limits{Time: time.Second, Mem: 256 << 20}, judge.Full)
			el := time.Since(start)
			out, _ := exec.Command("pgrep", "-f", tc.marker).Output()
			left := strings.Fields(string(out))
			note("fork_leak/"+tc.name, "O2", fmt.Sprintf("verdict=%s reason=%q err=%v elapsed=%v leftover pids=%v",
				r.Verdict, r.Reason, err, el.Round(time.Millisecond), left))
			if err != nil {
				t.Errorf("Run returned an error instead of a verdict: %v", err)
			}
			if len(left) > 0 {
				t.Errorf("child %q still running after Run returned (pids %v)", tc.marker, left)
			}
		})
	}
}

// A compile error must be readable in the HUD, a pane about 50 columns
// wide: the location should be short and relative.
func TestQACompileErrorReadable(t *testing.T) {
	needCompiler(t)
	for _, p := range profiles {
		t.Run(p.Name, func(t *testing.T) {
			b := build(t, "ce", progCE, p)
			if b.OK {
				t.Fatal("broken source compiled")
			}
			first := ""
			for l := range strings.Lines(b.Log) {
				if strings.Contains(l, "error:") {
					first = strings.TrimRight(l, "\n")
					break
				}
			}
			note("ce_first_error_line", p.Name, fmt.Sprintf("%q (%d cols)", first, len(first)))
			if i := strings.Index(first, "error:"); i > 20 {
				t.Errorf("first error line puts 'error:' at column %d: %q; a 50-column HUD cuts the message", i, first)
			}
			if strings.Contains(first, "/") {
				t.Errorf("error location is an absolute path: %q", first)
			}
		})
	}
}

// TestQASlowSymbolizer: a sanitizer report is symbolized by a helper
// process (llvm-symbolizer or atos) before the run exits. Observed at 1-4.6
// s wall under load; here a stand-in that takes 5 s. The run's crash must
// still be RE, not a wall-clock TLE.
func TestQASlowSymbolizer(t *testing.T) {
	needCompiler(t)
	real, err := exec.LookPath("llvm-symbolizer")
	if err != nil {
		t.Skip("no llvm-symbolizer on PATH")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nsleep 5\nexec " + real + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "llvm-symbolizer"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("ASAN_SYMBOLIZER_PATH", filepath.Join(dir, "llvm-symbolizer"))
	t.Setenv("UBSAN_SYMBOLIZER_PATH", filepath.Join(dir, "llvm-symbolizer"))
	for _, p := range profiles {
		if kind(p) == "full" {
			continue
		}
		b := build(t, "re_mod_by_zero", progMod, p)
		c := writeCase(t, "5 0\n", "5\n")
		r, err := judge.Run(context.Background(), b.Binary, c, judge.Limits{Time: time.Second}, p)
		record("slow_symbolizer_5s/mod_by_zero TL=1s", p.Name, r, err)
		if err == nil && r.Verdict != judge.RE {
			t.Errorf("%s: %s %q; the program crashed (stderr: %q)", p.Name, r.Verdict, r.Reason, firstLine(r.Stderr))
		}
	}
}
