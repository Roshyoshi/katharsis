package game

import (
	"math"
	"testing"
	"time"

	"katharsis/internal/catalog"
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
	// 30 solves near gap 0 at 20 min, 30 far away (gap -600) that would
	// move onto gap 0 at 80 min.
	var mixed []Solve
	for range 30 {
		mixed = append(mixed, solves(1, 0, 20*time.Minute)[0], solves(1, -600, 10*time.Minute)[0])
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
		{"trials ignored", append(solves(19, 0, 20*time.Minute), Solve{Kind: Independent, Elapsed: time.Hour, Trial: true}), 0, 0},
		{"the nearest half decide", mixed, 0, 25 * time.Minute},
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

// A door the floor easing gave a longer timer is rated against the clear
// chance the timer bought (rules.BuybackBeta), not the plain door's: its
// clear lifts the rating less and its failure drops it more.
func TestRateBoughtBack(t *testing.T) {
	t.Parallel()
	r := rules.Standard()
	r.RatingColdAttempts = 0
	d := Door{Problem: "t:800", Topic: Mixed, Rating: 800}
	bought := d
	bought.TimerBuy = 2
	for _, score := range []float64{0, 1} {
		plain, eased := &Meta{Rating: 1000}, &Meta{Rating: 1000}
		rate(&r, plain, d, score)
		rate(&r, eased, bought, score)
		p := lift(Expected(1000, 800), r.BuybackBeta*math.Log(2))
		if want := 1000 + r.RatingK*(score-p); math.Abs(eased.Rating-want) > 1e-9 || eased.Rating >= plain.Rating {
			t.Errorf("score %v: bought-back door rated %.2f (plain %.2f), want %.2f", score, eased.Rating, plain.Rating, want)
		}
	}
}

// A floor-rated door is read as easier than its rating: its clear chance
// is the logistic averaged over an exponential drop under the floor, so it
// lies between the chance at the rating and the chance at the mean drop,
// and is the plain chance when the rules read floors as they are.
func TestClearChanceAtTheFloor(t *testing.T) {
	t.Parallel()
	r := rules.Standard()
	for _, R := range []float64{700, 900, 1000, 1300, 2000} {
		at, mean := Expected(R, 800), Expected(R, 800-r.FloorDrop)
		got := clearChance(&r, R, 800, 0, true)
		if got <= at || got >= mean {
			t.Errorf("player %.0f: floor door %.4f, want between %.4f and %.4f", R, got, at, mean)
		}
		if plain := clearChance(&r, R, 800, 0, false); plain != at {
			t.Errorf("player %.0f: plain door %.4f, want %.4f", R, plain, at)
		}
	}
	if got := clearChance(&r, 1000, 800, 0, true); math.Abs(got-0.891) > 0.001 {
		t.Errorf("1000 on a floor door: %.4f, want 0.891", got)
	}
	cf := catalog.Problem{Rating: 800, RatingSource: "cf"}
	usaco := catalog.Problem{Rating: 800, RatingSource: "division"}
	above := catalog.Problem{Rating: 900, RatingSource: "cf"}
	if !floored(&r, &cf) || floored(&r, &usaco) || floored(&r, &above) {
		t.Error("only a Codeforces 800 is at the floor")
	}
	r.FloorDrop = 0
	if floored(&r, &cf) {
		t.Error("FloorDrop 0 still reads the floor")
	}
}
