package judge_test

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"katharsis/internal/judge"
)

// gnuCompiler reports whether g++ on PATH is GCC; macOS's /usr/bin/g++ is
// clang under that name.
func gnuCompiler() bool {
	out, err := exec.Command(judge.GXX, "--version").Output()
	return err == nil && strings.Contains(string(out), "Free Software Foundation")
}

// gnuToolchain is judge.Profiles without PCHs, skipping t where g++ does
// not judge.
func gnuToolchain(t *testing.T) judge.Toolchain {
	t.Helper()
	needCompiler(t)
	tc := judge.Profiles(context.Background())
	if !tc.Full.Libstdcxx {
		t.Skipf("g++ does not judge here (Full is %q)", tc.Full.Name)
	}
	tc.Samples, tc.Full = noPCH(tc.Samples), noPCH(tc.Full)
	return tc
}

// Where g++ is GCC it judges, and every build uses libstdc++.
func TestToolchain(t *testing.T) {
	needCompiler(t)
	t.Parallel()
	tc := judge.Profiles(context.Background())
	t.Logf("samples %q, full %q, debug %q", tc.Samples.Name, tc.Full.Name, tc.Debug.Name)
	if !gnuCompiler() {
		if tc.Full.Name != judge.Full.Name || tc.Samples.Libstdcxx {
			t.Errorf("without GCC: full %q, samples %q; want clang's", tc.Full.Name, tc.Samples.Name)
		}
		return
	}
	if tc.Full.Name != judge.GCCFull.Name || tc.Debug.Name != judge.GCCDebug.Name || !tc.Samples.Libstdcxx {
		t.Errorf("with GCC: samples %q, full %q, debug %q; want libstdc++ throughout",
			tc.Samples.Name, tc.Full.Name, tc.Debug.Name)
	}
}

// What contest code takes from g++ builds under every profile of the
// toolchain, the sanitized one included: a save must compile what :Submit
// does.
func TestGNUSources(t *testing.T) {
	tc := gnuToolchain(t)
	for _, tt := range []struct{ src, in, out string }{
		{"pbds.cpp", "5\n", "20 3\n"},
		{"pragma_target.cpp", "1 2\n", "3\n"},
		{"gnu.cpp", "12 18\n", "6 10 12\n"},
	} {
		for _, p := range []judge.Profile{tc.Samples, tc.Full, tc.Debug} {
			t.Run(tt.src+"/"+p.Name, func(t *testing.T) {
				t.Parallel()
				b := build(t, tt.src, p)
				c := writeCase(t, tt.in, tt.out)
				r, err := judge.Run(context.Background(), b.Binary, c, judge.Limits{Time: 2 * time.Second}, p)
				if err != nil || r.Verdict != judge.AC {
					t.Fatalf("%v %s %q\n%s", err, r.Verdict, r.Reason, r.Stderr)
				}
			})
		}
	}
}

// Dropping a target pragma must not move the lines g++ reports.
func TestTargetPragmaKeepsLines(t *testing.T) {
	tc := gnuToolchain(t)
	t.Parallel()
	b, err := judge.Compile(context.Background(), fixture("pragma_ce.cpp"), tc.Full)
	if err != nil {
		t.Fatal(err)
	}
	if b.OK || !strings.Contains(b.Log, "pragma_ce.cpp:7:") || strings.Contains(b.Log, "target(") {
		t.Errorf("OK = %v, log:\n%s\nwant one error, at pragma_ce.cpp:7", b.OK, b.Log)
	}
}

// Why g++ judges (D28): through libc++, cin >> int reads 1e6 ints ~16x
// slower than scanf even synced off, and correct solutions TLE; through
// libstdc++ it beats scanf.
func TestCinSpeed(t *testing.T) {
	tc := gnuToolchain(t)
	const n = 1_000_000
	var in strings.Builder
	var sum int64
	fmt.Fprintf(&in, "%d\n", n)
	for i := range n {
		x := int64(i) * 7919 % 1_000_000_007
		sum += x
		fmt.Fprintf(&in, "%d ", x)
	}
	c := writeCase(t, in.String()+"\n", fmt.Sprintf("%d\n", sum))
	cpu := map[string]time.Duration{}
	for _, src := range []string{"cin.cpp", "scanf.cpp"} {
		b := build(t, src, tc.Full)
		r, err := judge.Run(context.Background(), b.Binary, c, judge.Limits{Time: 20 * time.Second}, tc.Full)
		if err != nil || r.Verdict != judge.AC {
			t.Fatalf("%s: %v %s %q", src, err, r.Verdict, r.Reason)
		}
		cpu[src] = r.CPU
	}
	t.Logf("1e6 ints under %s: cin %v, scanf %v", tc.Full.Name, cpu["cin.cpp"], cpu["scanf.cpp"])
	if cpu["cin.cpp"] > 2*cpu["scanf.cpp"] {
		t.Errorf("cin %v > 2 x scanf %v: libc++'s istream speed, not libstdc++'s", cpu["cin.cpp"], cpu["scanf.cpp"])
	}
}
