//go:build qa

package judgeqa

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"katharsis/internal/judge"
	"katharsis/internal/problem"
)

func loadAvg() string {
	out, _ := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	return strings.TrimSpace(string(out))
}

// How long a player waits for the verdict on an infinite loop, per
// profile, at the default 2 s limit (practice's -tl default and most CF
// problems). Nothing else runs meanwhile: samples are judged in order.
func TestQAInfiniteLoopLatency(t *testing.T) {
	needCompiler(t)
	for _, p := range profiles {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()
			b := build(t, "spin_latency", progSpin, p)
			c := writeCase(t, "", "")
			lim := judge.Limits{Time: 2 * time.Second, Mem: 256 << 20}
			r, err := judge.Run(context.Background(), b.Binary, c, lim, p)
			record("latency_infinite_loop_tl2s", p.Name, r, err)
			if err != nil {
				t.Fatal(err)
			}
			if r.Wall > 3*lim.Time+time.Second {
				t.Errorf("an infinite loop took %v to judge (TL %v, budget %v): the HUD says running… that long per test",
					r.Wall.Round(100*time.Millisecond), lim.Time, time.Duration(float64(lim.Time)*p.Slack))
			}
		})
	}
}

// Representative contest workloads: CPU at -O2 and under the sample
// profile, sequentially. The report compares them with what a
// Codeforces-class judge needs for the same work.
var benches = []struct {
	name, body string
	input      func(dir string) string // writes the input, returns its path
}{
	{"ops_1e8_simple", `#include <cstdio>
int a[1024];
int main() {
    for (int i = 0; i < 1024; i++) a[i] = i * 7;
    unsigned s = 0;
    for (int i = 0; i < 100000000; i++) s += (a[i & 1023] ^ i) & 7;
    printf("%u\n", s);
}
`, nil},
	{"ops_1e8_mod", `#include <cstdio>
int main() {
    long long s = 0, m = 1000000007;
    for (long long i = 1; i <= 100000000; i++) s = (s * 31 + i) % m;
    printf("%lld\n", s);
}
`, nil},
	{"set_2e5_insert_and_lower_bound", `#include <cstdio>
#include <set>
int main() {
    std::set<long long> s;
    unsigned long long x = 88172645463325252ULL;
    auto rnd = [&] { x ^= x << 13; x ^= x >> 7; x ^= x << 17; return (long long)(x % 1000000000); };
    for (int i = 0; i < 200000; i++) s.insert(rnd());
    long long acc = 0;
    for (int i = 0; i < 200000; i++) { auto it = s.lower_bound(rnd()); if (it != s.end()) acc += *it & 1; }
    printf("%lld\n", acc);
}
`, nil},
	{"map_2e5_string_keys", `#include <cstdio>
#include <map>
#include <string>
int main() {
    std::map<std::string, int> m;
    unsigned long long x = 88172645463325252ULL;
    for (int i = 0; i < 200000; i++) { x ^= x << 13; x ^= x >> 7; x ^= x << 17; m[std::to_string(x % 1000000)]++; }
    printf("%zu\n", m.size());
}
`, nil},
	{"sort_1e7", `#include <algorithm>
#include <cstdio>
#include <vector>
int main() {
    std::vector<int> v(10000000);
    unsigned x = 2463534242u;
    for (auto &e : v) { x ^= x << 13; x ^= x >> 17; x ^= x << 5; e = (int)x; }
    std::sort(v.begin(), v.end());
    printf("%d\n", v[5000000] & 1);
}
`, nil},
	{"umap_1e6", `#include <cstdio>
#include <unordered_map>
int main() {
    std::unordered_map<long long, int> m;
    unsigned long long x = 88172645463325252ULL;
    for (int i = 0; i < 1000000; i++) { x ^= x << 13; x ^= x >> 7; x ^= x << 17; m[(long long)(x % 100000000)]++; }
    printf("%zu\n", m.size());
}
`, nil},
	{"dijkstra_2e5_n_4e5_m", `#include <cstdio>
#include <queue>
#include <vector>
int main() {
    int n = 200000, m = 400000;
    std::vector<std::vector<std::pair<int, int>>> g(n);
    unsigned x = 2463534242u;
    auto r = [&] { x ^= x << 13; x ^= x >> 17; x ^= x << 5; return x; };
    for (int i = 1; i < n; i++) g[r() % i].push_back({i, (int)(r() % 1000 + 1)});
    for (int i = n - 1; i < m; i++) { int a = r() % n, b = r() % n; g[a].push_back({b, (int)(r() % 1000 + 1)}); }
    std::vector<long long> d(n, (long long)4e18);
    std::priority_queue<std::pair<long long, int>, std::vector<std::pair<long long, int>>, std::greater<>> pq;
    d[0] = 0; pq.push({0, 0});
    while (!pq.empty()) {
        auto [du, u] = pq.top(); pq.pop();
        if (du != d[u]) continue;
        for (auto [v, w] : g[u]) if (d[v] > du + w) { d[v] = du + w; pq.push({d[v], v}); }
    }
    long long s = 0; for (auto e : d) if (e < (long long)4e18) s += e & 1;
    printf("%lld\n", s);
}
`, nil},
	{"dp_lcs_5000x5000", `#include <cstdio>
#include <vector>
#include <algorithm>
int main() {
    int n = 5000;
    std::vector<int> a(n), b(n);
    unsigned x = 2463534242u;
    for (int i = 0; i < n; i++) { x ^= x << 13; x ^= x >> 17; x ^= x << 5; a[i] = x % 26; x ^= x << 13; x ^= x >> 17; x ^= x << 5; b[i] = x % 26; }
    std::vector<std::vector<int>> dp(n + 1, std::vector<int>(n + 1));
    for (int i = 1; i <= n; i++) for (int j = 1; j <= n; j++)
        dp[i][j] = a[i - 1] == b[j - 1] ? dp[i - 1][j - 1] + 1 : std::max(dp[i - 1][j], dp[i][j - 1]);
    printf("%d\n", dp[n][n]);
}
`, nil},
	{"brute_n2_pairs_1e5", `#include <cstdio>
#include <vector>
int main() {
    int n = 100000;
    std::vector<int> a(n);
    unsigned x = 2463534242u;
    for (auto &e : a) { x ^= x << 13; x ^= x >> 17; x ^= x << 5; e = x % 1000000; }
    long long cnt = 0;
    for (int i = 0; i < n; i++) for (int j = i + 1; j < n; j++) cnt += (a[i] + a[j] == 1000000);
    printf("%lld\n", cnt);
}
`, nil},
	{"cin_1e6_ints_sync_off", `#include <iostream>
int main() {
    std::ios::sync_with_stdio(false); std::cin.tie(nullptr);
    int n; std::cin >> n; long long s = 0;
    for (int i = 0; i < n; i++) { int v; std::cin >> v; s += v; }
    std::cout << s << "\n";
}
`, ints1e6},
	{"cin_1e6_ints_sync_on", `#include <iostream>
int main() {
    int n; std::cin >> n; long long s = 0;
    for (int i = 0; i < n; i++) { int v; std::cin >> v; s += v; }
    std::cout << s << "\n";
}
`, ints1e6},
	{"scanf_1e6_ints", `#include <cstdio>
int main() {
    int n; scanf("%d", &n); long long s = 0;
    for (int i = 0; i < n; i++) { int v; scanf("%d", &v); s += v; }
    printf("%lld\n", s);
}
`, ints1e6},
	{"cout_endl_1e6_lines", `#include <iostream>
int main() {
    for (int i = 0; i < 1000000; i++) std::cout << i << std::endl;
}
`, nil},
}

func ints1e6(dir string) string {
	p := filepath.Join(dir, "ints.in")
	var b strings.Builder
	b.WriteString("1000000\n")
	for i := range 1000000 {
		fmt.Fprintf(&b, "%d ", (i*7919)%1000000000)
	}
	b.WriteString("\n")
	os.WriteFile(p, []byte(b.String()), 0o644)
	return p
}

func TestQABenchmarks(t *testing.T) {
	needCompiler(t)
	if os.Getenv("JUDGEQA_BENCH") == "" {
		t.Skip("set JUDGEQA_BENCH=1 (sequential, minutes)")
	}
	t.Logf("load before: %s", loadAvg())
	for _, bc := range benches {
		var fullCPU time.Duration
		for _, p := range profiles {
			b := build(t, "bench_"+bc.name, bc.body, p)
			if !b.OK {
				t.Fatalf("%s [%s] CE: %s", bc.name, p.Name, b.Log)
			}
			dir := t.TempDir()
			in := filepath.Join(dir, "empty.in")
			os.WriteFile(in, nil, 0o644)
			if bc.input != nil {
				in = bc.input(dir)
			}
			out := filepath.Join(dir, "x.out")
			os.WriteFile(out, nil, 0o644)
			lim := judge.Limits{Time: 30 * time.Second, Output: 1 << 30}
			// best of 3: the minimum is the least disturbed by other load
			best := judge.Result{CPU: time.Hour}
			for range 3 {
				r, err := judge.Run(context.Background(), b.Binary, judge.Case{In: in, Out: out}, lim, p)
				if err != nil {
					t.Fatal(err)
				}
				if r.Verdict == judge.RE || r.Verdict == judge.TLE || r.Verdict == judge.MLE || r.Verdict == judge.OLE {
					t.Fatalf("%s [%s]: %s %q", bc.name, p.Name, r.Verdict, r.Reason)
				}
				if r.CPU < best.CPU {
					best = r
				}
			}
			if kind(p) == "full" {
				fullCPU = best.CPU
			}
			ratio := ""
			if fullCPU > 0 && kind(p) != "full" {
				ratio = fmt.Sprintf(" (%.1fx O2, slack %.0f)", float64(best.CPU)/float64(fullCPU), p.Slack)
			}
			note("bench/"+bc.name, p.Name, fmt.Sprintf("cpu=%dms wall=%dms rss=%s%s",
				best.CPU.Milliseconds(), best.Wall.Milliseconds(), judge.Bytes(best.MaxRSS), ratio))
		}
	}
	t.Logf("load after: %s", loadAvg())
}

// TestQADrift measures per-run CPU of a fixed amount of work alone and
// Workers-wide, and whether a submit that is comfortably inside the limit
// when run alone gets a false TLE from JudgeFull.
func TestQADrift(t *testing.T) {
	needCompiler(t)
	if os.Getenv("JUDGEQA_BENCH") == "" {
		t.Skip("set JUDGEQA_BENCH=1")
	}
	b := build(t, "drift_work", progWork, judge.Full)
	dir := t.TempDir()
	mkTests := func(n int, in string) []problem.Test {
		cmd := exec.Command(b.Binary)
		cmd.Stdin = strings.NewReader(in)
		want, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		sub := t.TempDir()
		var ts []problem.Test
		for i := range n {
			ip, op := filepath.Join(sub, fmt.Sprintf("%d.in", i)), filepath.Join(sub, fmt.Sprintf("%d.out", i))
			os.WriteFile(ip, []byte(in), 0o644)
			os.WriteFile(op, want, 0o644)
			ts = append(ts, problem.Test{Name: fmt.Sprint(i), In: ip, Out: op})
		}
		return ts
	}
	_ = dir
	ctx := context.Background()
	big := judge.Limits{Time: 30 * time.Second}
	stats := func(ds []time.Duration) string {
		s := slices.Sorted(slices.Values(ds))
		return fmt.Sprintf("min %dms med %dms p90 %dms max %dms", s[0].Milliseconds(), s[len(s)/2].Milliseconds(),
			s[len(s)*9/10].Milliseconds(), s[len(s)-1].Milliseconds())
	}
	ts := mkTests(3*judge.Workers, "150\n") // about 0.3 s on a P-core
	var seq []time.Duration
	for _, tc := range ts[:judge.Workers] {
		r, err := judge.Run(ctx, b.Binary, judge.Case{In: tc.In, Out: tc.Out}, big, judge.Full)
		if err != nil {
			t.Fatal(err)
		}
		seq = append(seq, r.CPU)
	}
	var par []time.Duration
	if _, err := judge.JudgeFull(ctx, ts, "", b, big, judge.Full, func(e judge.Event) {
		par = append(par, e.(judge.TestDone).Result.CPU)
	}); err != nil {
		t.Fatal(err)
	}
	note("drift/work150", fmt.Sprintf("seq x%d", len(seq)), stats(seq)+" load "+loadAvg())
	note("drift/work150", fmt.Sprintf("par W=%d x%d", judge.Workers, len(par)), stats(par))

	// A submit whose every test needs 70% of the limit alone.
	med := slices.Sorted(slices.Values(seq))[len(seq)/2]
	lim := judge.Limits{Time: time.Duration(float64(med) / 0.7)}
	for _, extra := range []int{0, 1} {
		label := fmt.Sprintf("submit at 0.7xTL, %d concurrent sample pass", extra)
		var stop func()
		if extra > 0 {
			stop = backgroundSamplePass(t)
		}
		var falseTLE, reruns int
		var cpus []time.Duration
		s, err := judge.JudgeFull(ctx, mkTests(4*judge.Workers, "150\n"), "", b, lim, judge.Full, func(e judge.Event) {
			r := e.(judge.TestDone).Result
			cpus = append(cpus, r.CPU)
			if r.Rerun {
				reruns++
			}
			if r.Verdict == judge.TLE {
				falseTLE++
			}
		})
		if stop != nil {
			stop()
		}
		if err != nil {
			t.Fatal(err)
		}
		note("drift/false_tle", label, fmt.Sprintf("TL %dms: verdict %s, %d false TLE, %d reruns, cpu %s",
			lim.Time.Milliseconds(), s.Verdict, falseTLE, reruns, stats(cpus)))
		if s.Verdict != judge.AC {
			t.Errorf("%s: verdict %s (%q); every test needs 70%% of the limit alone", label, s.Verdict, s.Reason)
		}
	}
}

// backgroundSamplePass keeps one CPU-bound sample-profile run going, as a
// save during :Submit does in practice mode.
func backgroundSamplePass(t *testing.T) func() {
	b := build(t, "bg_spin", progSpin, sample)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Go(func() {
		for ctx.Err() == nil {
			c := writeCase(t, "", "")
			judge.Run(ctx, b.Binary, c, judge.Limits{Time: 2 * time.Second}, sample)
		}
	})
	return func() { cancel(); wg.Wait() }
}

// TLE near the limit under Full, alone and inside JudgeFull.
func TestQATLERatios(t *testing.T) {
	needCompiler(t)
	b := build(t, "burn_ratio", progBurn, judge.Full)
	tl := 500 * time.Millisecond
	for _, ratio := range []float64{0.9, 1.1, 1.5, 3} {
		in := fmt.Sprintf("%.0f\n", ratio*float64(tl.Milliseconds()))
		dir := t.TempDir()
		var ts []problem.Test
		for i := range 2 * judge.Workers {
			ip, op := filepath.Join(dir, fmt.Sprintf("%d.in", i)), filepath.Join(dir, fmt.Sprintf("%d.out", i))
			os.WriteFile(ip, []byte(in), 0o644)
			os.WriteFile(op, []byte("ok\n"), 0o644)
			ts = append(ts, problem.Test{Name: fmt.Sprint(i), In: ip, Out: op})
		}
		start := time.Now()
		var reruns int
		var walls []time.Duration
		s, err := judge.JudgeFull(context.Background(), ts, "", b, judge.Limits{Time: tl}, judge.Full, func(e judge.Event) {
			r := e.(judge.TestDone).Result
			if r.Rerun {
				reruns++
			}
			walls = append(walls, r.Wall)
		})
		if err != nil {
			t.Fatal(err)
		}
		el := time.Since(start)
		note("tle_ratio_full_judgefull", fmt.Sprintf("%.1fx TL=500ms", ratio),
			fmt.Sprintf("verdict %s %q passed %d/%d reruns %d, pass took %v", s.Verdict, s.Reason, s.Passed, s.Total, reruns, el.Round(10*time.Millisecond)))
		want := judge.TLE
		if ratio < 1 {
			want = judge.AC
		}
		if s.Verdict != want {
			t.Errorf("%.1fx: verdict %s, want %s", ratio, s.Verdict, want)
		}
	}
}

// TestQAWorkerSweep: the per-run CPU of a fixed 0.7 s job at several
// JudgeFull widths. On an M4 (4 P + 6 E cores) a run placed on an E-core
// reads far more CPU time than one on a P-core.
func TestQAWorkerSweep(t *testing.T) {
	needCompiler(t)
	if os.Getenv("JUDGEQA_BENCH") == "" {
		t.Skip("set JUDGEQA_BENCH=1")
	}
	b := build(t, "drift_work", progWork, judge.Full)
	in := "450\n"
	cmd := exec.Command(b.Binary)
	cmd.Stdin = strings.NewReader(in)
	want, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	old := judge.Workers
	defer func() { judge.Workers = old }()
	big := judge.Limits{Time: 30 * time.Second}
	var seqMed time.Duration
	for _, w := range []int{1, 4, 5, 8, 10} {
		judge.Workers = w
		dir := t.TempDir()
		var ts []problem.Test
		for i := range max(2*w, 5) {
			ip, op := filepath.Join(dir, fmt.Sprintf("%d.in", i)), filepath.Join(dir, fmt.Sprintf("%d.out", i))
			os.WriteFile(ip, []byte(in), 0o644)
			os.WriteFile(op, want, 0o644)
			ts = append(ts, problem.Test{Name: fmt.Sprint(i), In: ip, Out: op})
		}
		var cpus []time.Duration
		start := time.Now()
		if _, err := judge.JudgeFull(context.Background(), ts, "", b, big, judge.Full, func(e judge.Event) {
			cpus = append(cpus, e.(judge.TestDone).Result.CPU)
		}); err != nil {
			t.Fatal(err)
		}
		s := slices.Sorted(slices.Values(cpus))
		if w == 1 {
			seqMed = s[len(s)/2]
		}
		note("worker_sweep/work450", fmt.Sprintf("W=%02d n=%d", w, len(s)),
			fmt.Sprintf("cpu min %dms med %dms max %dms (max %+.0f%% vs W=1 median) wall %v load %s",
				s[0].Milliseconds(), s[len(s)/2].Milliseconds(), s[len(s)-1].Milliseconds(),
				100*(float64(s[len(s)-1])/float64(seqMed)-1), time.Since(start).Round(10*time.Millisecond), loadAvg()))
	}
}

// TestQAStderrCost: the same debug-printing program judged through Run
// (stderr is a pipe read by Go) and with stderr sent to a file, as judges
// that keep stderr do.
func TestQAStderrCost(t *testing.T) {
	needCompiler(t)
	b := build(t, "stderr_cost", progStderr, judge.Full)
	for _, mib := range []int{4, 16} {
		in := fmt.Sprintf("%d 1 2\n", mib)
		c := writeCase(t, in, "3\n")
		r, err := judge.Run(context.Background(), b.Binary, c, judge.Limits{Time: 30 * time.Second}, judge.Full)
		record(fmt.Sprintf("stderr_cost/%dMiB", mib), "Run (pipe to Go)", r, err)
		f, _ := os.Create(filepath.Join(t.TempDir(), "err.txt"))
		cmd := exec.Command(b.Binary)
		cmd.Stdin = strings.NewReader(in)
		cmd.Stderr = f
		start := time.Now()
		cmd.Run()
		wall := time.Since(start)
		f.Close()
		ru := cmd.ProcessState.SysUsage().(*syscall.Rusage)
		cpu := time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
		note(fmt.Sprintf("stderr_cost/%dMiB", mib), "stderr to a file", fmt.Sprintf("cpu=%dms wall=%dms", cpu.Milliseconds(), wall.Milliseconds()))
	}
}

// TestQAIOSweep: like TestQAWorkerSweep for syscall-heavy programs (debug
// lines to stderr, endl-flushed stdout), whose system time may not scale
// like compute does.
func TestQAIOSweep(t *testing.T) {
	needCompiler(t)
	if os.Getenv("JUDGEQA_BENCH") == "" {
		t.Skip("set JUDGEQA_BENCH=1")
	}
	endl := `#include <iostream>
int main() { int n; std::cin >> n; for (int i = 0; i < n; i++) std::cout << i % 10 << std::endl; }
`
	old := judge.Workers
	defer func() { judge.Workers = old }()
	for _, pc := range []struct{ name, body, in string }{
		{"stderr_16MiB", progStderr, "16 1 2\n"},
		{"endl_3e5", endl, "300000\n"},
	} {
		b := build(t, "iosweep_"+pc.name, pc.body, judge.Full)
		cmd := exec.Command(b.Binary)
		cmd.Stdin = strings.NewReader(pc.in)
		want, _ := cmd.Output()
		var seqMed time.Duration
		for _, w := range []int{1, 5, 8} {
			judge.Workers = w
			dir := t.TempDir()
			var ts []problem.Test
			for i := range max(2*w, 5) {
				ip, op := filepath.Join(dir, fmt.Sprintf("%d.in", i)), filepath.Join(dir, fmt.Sprintf("%d.out", i))
				os.WriteFile(ip, []byte(pc.in), 0o644)
				os.WriteFile(op, want, 0o644)
				ts = append(ts, problem.Test{Name: fmt.Sprint(i), In: ip, Out: op})
			}
			var cpus []time.Duration
			if _, err := judge.JudgeFull(context.Background(), ts, "", b, judge.Limits{Time: 30 * time.Second, Output: 1 << 30}, judge.Full, func(e judge.Event) {
				cpus = append(cpus, e.(judge.TestDone).Result.CPU)
			}); err != nil {
				t.Fatal(err)
			}
			s := slices.Sorted(slices.Values(cpus))
			if w == 1 {
				seqMed = s[len(s)/2]
			}
			note("io_sweep/"+pc.name, fmt.Sprintf("W=%02d n=%d", w, len(s)),
				fmt.Sprintf("cpu min %dms med %dms max %dms (max %+.0f%% vs W=1 median) load %s",
					s[0].Milliseconds(), s[len(s)/2].Milliseconds(), s[len(s)-1].Milliseconds(),
					100*(float64(s[len(s)-1])/float64(seqMed)-1), loadAvg()))
		}
	}
}

// TestQAFalseTLEIOHeavy: a solution that prints one line per query with
// endl, 0.5 s alone on a 1 s limit, judged by JudgeFull at the default
// Workers, as :Submit does.
func TestQAFalseTLEIOHeavy(t *testing.T) {
	needCompiler(t)
	endl := `#include <iostream>
int main() { int n; std::cin >> n; for (int i = 0; i < n; i++) std::cout << i % 10 << std::endl; }
`
	b := build(t, "false_tle_endl", endl, judge.Full)
	in := "1000000\n"
	cmd := exec.Command(b.Binary)
	cmd.Stdin = strings.NewReader(in)
	want, _ := cmd.Output()
	dir := t.TempDir()
	var ts []problem.Test
	for i := range 2 * judge.Workers {
		ip, op := filepath.Join(dir, fmt.Sprintf("%d.in", i)), filepath.Join(dir, fmt.Sprintf("%d.out", i))
		os.WriteFile(ip, []byte(in), 0o644)
		os.WriteFile(op, want, 0o644)
		ts = append(ts, problem.Test{Name: fmt.Sprint(i), In: ip, Out: op})
	}
	lim := judge.Limits{Time: time.Second, Mem: 256 << 20}
	alone, err := judge.Run(context.Background(), b.Binary, judge.Case{In: ts[0].In, Out: ts[0].Out}, lim, judge.Full)
	if err != nil {
		t.Fatal(err)
	}
	var reruns int
	var cpus []time.Duration
	s, err := judge.JudgeFull(context.Background(), ts, "", b, lim, judge.Full, func(e judge.Event) {
		r := e.(judge.TestDone).Result
		cpus = append(cpus, r.CPU)
		if r.Rerun {
			reruns++
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	note("false_tle_endl_1e6/TL=1s", fmt.Sprintf("W=%d", judge.Workers),
		fmt.Sprintf("alone %s cpu=%dms; JudgeFull verdict %s %q passed %d/%d reruns %d cpus %v load %s",
			alone.Verdict, alone.CPU.Milliseconds(), s.Verdict, s.Reason, s.Passed, s.Total, reruns, cpus, loadAvg()))
	if alone.Verdict == judge.AC && s.Verdict != judge.AC {
		t.Errorf("AC alone at %v CPU, %s in JudgeFull (%q)", alone.CPU.Round(time.Millisecond), s.Verdict, s.Reason)
	}
}
