package game

import (
	"math"
	"testing"
	"time"

	"katharsis/internal/rules"
)

func TestSplitmixKnownValues(t *testing.T) {
	t.Parallel()
	// Reference outputs of SplitMix64 seeded with 0 (Vigna's splitmix64.c).
	want := []uint64{0xe220a8397b1dcdaf, 0x6e789e6aa1b965f4, 0x06c45d188009454f}
	var s uint64
	m := splitmix{&s}
	for i, w := range want {
		if got := m.Uint64(); got != w {
			t.Errorf("draw %d = %#x, want %#x", i, got, w)
		}
	}
}

func TestSample(t *testing.T) {
	t.Parallel()
	var st uint64 = 42
	rng := newRand(&st)
	xs := []int{1, 2, 3, 4, 5}
	for n := range 7 {
		got := sample(rng, xs, n)
		if len(got) != min(n, len(xs)) {
			t.Fatalf("sample(%d) has %d elements", n, len(got))
		}
		seen := map[int]bool{}
		for _, x := range got {
			if seen[x] {
				t.Fatalf("sample(%d) = %v repeats %d", n, got, x)
			}
			seen[x] = true
		}
	}
	if xs[0] != 1 || xs[4] != 5 {
		t.Fatalf("sample reordered its input: %v", xs)
	}
}

func TestQuantile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		xs   []float64
		q    float64
		want float64
	}{
		{[]float64{5}, 0.85, 5},
		{[]float64{1, 2, 3, 4, 5}, 0, 1},
		{[]float64{1, 2, 3, 4, 5}, 1, 5},
		{[]float64{1, 2, 3, 4, 5}, 0.5, 3},
		{[]float64{5, 1, 4, 2, 3}, 0.85, 4.4}, // h = 3.4
		{[]float64{10, 20}, 0.25, 12.5},
	}
	for _, tc := range tests {
		if got := quantile(append([]float64(nil), tc.xs...), tc.q); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("quantile(%v, %v) = %v, want %v", tc.xs, tc.q, got, tc.want)
		}
	}
}

func TestFitTimer(t *testing.T) {
	t.Parallel()
	r := rules.Standard()
	solves := func(n int, gap float64, d time.Duration) []Solve {
		var ss []Solve
		for range n {
			ss = append(ss, Solve{Kind: Independent, Elapsed: d, Gap: gap})
		}
		return ss
	}
	tests := []struct {
		name   string
		solves []Solve
		gap    float64
		want   time.Duration // 0: not fitted
	}{
		{"too few", solves(19, 0, 20*time.Minute), 0, 0},
		{"same gap", solves(20, 0, 20*time.Minute), 0, 25 * time.Minute},
		// One doubling (TimerGapScale) easier halves the time.
		{"easier door", solves(20, 0, 20*time.Minute), -200, 12*time.Minute + 30*time.Second},
		{"solves were easier", solves(20, -200, 10*time.Minute), 0, 25 * time.Minute},
		{"upsolves ignored", append(solves(19, 0, 20*time.Minute), Solve{Kind: Independent, Elapsed: time.Hour, Upsolve: true}), 0, 0},
		{"editorial ignored", append(solves(19, 0, 20*time.Minute), Solve{Kind: Editorial, Elapsed: time.Hour}), 0, 0},
	}
	for _, tc := range tests {
		got, ok := fitTimer(&r, tc.solves, tc.gap)
		if tc.want == 0 {
			if ok {
				t.Errorf("%s: fitted %v, want no fit", tc.name, got)
			}
			continue
		}
		if !ok || (got-tc.want).Abs() > time.Second {
			t.Errorf("%s: fitTimer = %v, %v; want %v", tc.name, got, ok, tc.want)
		}
	}
}

func TestFitTimerNearestGaps(t *testing.T) {
	t.Parallel()
	r := rules.Standard()
	// 30 solves near gap 0 at 20 min, 30 far away (gap -600) that would
	// normalize to 80 min. The nearest half decide.
	var ss []Solve
	for range 30 {
		ss = append(ss, Solve{Kind: Independent, Elapsed: 20 * time.Minute, Gap: 0})
		ss = append(ss, Solve{Kind: Independent, Elapsed: 10 * time.Minute, Gap: -600})
	}
	got, ok := fitTimer(&r, ss, 0)
	if !ok || (got-25*time.Minute).Abs() > time.Second {
		t.Fatalf("fitTimer = %v, %v; want 25m from the near solves only", got, ok)
	}
}
