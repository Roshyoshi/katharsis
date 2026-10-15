package game_test

import (
	"math"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

func TestCalibrationTier(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		b    rules.Biome
		want string
	}{
		{rules.Biome{Kind: rules.Fixed, TargetClear: 0.92}, "0.92"},
		{rules.Biome{Kind: rules.Choice, TargetClear: 0.7}, "0.70"},
		{rules.Biome{Kind: rules.Boss, TargetClear: 0.5}, "boss 0.50"},
	} {
		if got := game.CalibrationTier(tc.b); got != tc.want {
			t.Errorf("CalibrationTier(%+v) = %q, want %q", tc.b, got, tc.want)
		}
	}
}

// TestCalibrate checks one door's lesson against the update rule in
// calibrate's doc comment, and every case where a door teaches nothing.
func TestCalibrate(t *testing.T) {
	t.Parallel()
	const k = math.Ln10 / 400
	pool := dense(allTopics(), 800, 2600, 20)
	base := plain()
	base.Biomes = []rules.Biome{{Kind: rules.Fixed, Rooms: 2, TargetClear: 0.5, Timer: rules.Duration{Duration: 30 * time.Minute}}}
	tier := game.CalibrationTier(base.Biomes[0])
	edit := func(f func(*rules.Rules)) rules.Rules {
		r := base
		f(&r)
		return r
	}
	tests := []struct {
		name   string
		r      rules.Rules
		rated  int
		start  float64 // both offsets before the door
		end    game.Action
		y      float64 // the outcome taught; < 0: nothing taught
		capped bool
	}{
		{"clear", base, 100, 0, game.SubmitResult{Verdict: "AC"}, 1, false},
		{":q!", base, 100, 0, game.GiveUp{}, 0, false},
		{"retreat is a failure", base, 100, 0, game.Retreat{}, 0, false},
		{"clear with offsets", base, 100, 40, game.SubmitResult{Verdict: "AC"}, 1, false},
		{"capped", base, 100, 300, game.SubmitResult{Verdict: "AC"}, 1, true},
		{"cold start", base, 19, 0, game.SubmitResult{Verdict: "AC"}, -1, false},
		{"before CalibAfter", base, 59, 0, game.SubmitResult{Verdict: "AC"}, -1, false},
		{"blind door", edit(func(r *rules.Rules) { r.BlindAll = true }), 100, 0, game.SubmitResult{Verdict: "AC"}, -1, false},
		{"cursed door", edit(func(r *rules.Rules) { r.CursedChance = 1 }), 100, 0, game.SubmitResult{Verdict: "AC"}, -1, false},
		{"turned off", edit(func(r *rules.Rules) { r.CalibRate = 0 }), 100, 0, game.SubmitResult{Verdict: "AC"}, -1, false},
	}
	for _, tc := range tests {
		m := &game.Meta{Rating: 1500, Rated: tc.rated}
		if tc.start != 0 {
			m.Calibration = map[string]game.Calibration{
				game.CalibrationAll: {Offset: tc.start, Doors: 50},
				tier:                {Offset: min(tc.start, base.CalibMax/2), Doors: 50},
			}
		}
		h := newHarness(t, tc.r, m, pool, 0, 1)
		d := h.open(0)
		h.do(tc.end)
		got := m.Calibration
		if tc.y < 0 {
			if tc.start == 0 && got != nil {
				t.Errorf("%s: taught %+v", tc.name, got)
			}
			continue
		}
		p := min(max(d.SolveP, 0.2), 0.8)
		for _, s := range []struct {
			key              string
			slope, max, leak float64
		}{
			{game.CalibrationAll, 0.25 * k, base.CalibMax, 0},
			{tier, p * (1 - p) * k, base.CalibMax / 2, base.CalibRate / 10},
		} {
			start, n := 0.0, 0
			if tc.start != 0 {
				start, n = min(tc.start, s.max), 50
			}
			gain := max(base.CalibRate, 1/float64(n+base.CalibWarm))
			want := min(max(start*(1-s.leak)+gain*(tc.y-d.SolveP)/s.slope, -s.max), s.max)
			c := got[s.key]
			if math.Abs(c.Offset-want) > 1e-9 || c.Doors != n+1 {
				t.Errorf("%s: %s offset %.3f after %d doors, want %.3f after %d", tc.name, s.key, c.Offset, c.Doors, want, n+1)
			}
			if tc.capped && c.Offset != s.max {
				t.Errorf("%s: %s offset %.3f, want the cap %.0f", tc.name, s.key, c.Offset, s.max)
			}
			if n == 0 && (c.Observed != tc.y || c.Expected != d.SolveP) {
				t.Errorf("%s: %s observed %v expected %v, want %v and %v", tc.name, s.key, c.Observed, c.Expected, tc.y, d.SolveP)
			}
		}
	}
}

// TestTopicFollow: a topic rating moves with every change of the overall
// rating, and on its own doors also learns from its own surprise.
func TestTopicFollow(t *testing.T) {
	t.Parallel()
	pool := []catalog.Problem{{ID: "dp", Topic: "dp", Rating: 1400, Judgeable: true}}
	r := plain()
	r.DoorsPerRoom = 1
	r.Biomes = []rules.Biome{{Kind: rules.Fixed, Rooms: 1, TargetClear: 0.5, Timer: rules.Duration{Duration: 30 * time.Minute}}}
	for _, follow := range []float64{0, 1} {
		r.TopicFollow = follow
		m := &game.Meta{
			Rating: 1400, Rated: 100,
			TopicRating:   map[string]float64{"dp": 1200, "graphs": 1500},
			TopicAttempts: map[string]int{"dp": 2, "graphs": 9},
		}
		h := newHarness(t, r, m, pool, 0, 1)
		h.open(0)
		h.do(game.SubmitResult{Verdict: "AC"})
		moved := 40 * (1 - game.Expected(1400, 1400))
		if want := 1400 + moved; math.Abs(m.Rating-want) > 1e-9 {
			t.Errorf("follow %v: overall %v, want %v", follow, m.Rating, want)
		}
		if want := 1500 + follow*moved; math.Abs(m.TopicRating["graphs"]-want) > 1e-9 {
			t.Errorf("follow %v: graphs %v, want %v", follow, m.TopicRating["graphs"], want)
		}
		if want := 1200 + follow*moved + 40*(1-game.Expected(1200, 1400)); math.Abs(m.TopicRating["dp"]-want) > 1e-9 {
			t.Errorf("follow %v: dp %v, want %v", follow, m.TopicRating["dp"], want)
		}
	}
}
