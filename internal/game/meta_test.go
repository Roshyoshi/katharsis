package game_test

import (
	"slices"
	"testing"
	"time"

	"deadcases/internal/catalog"
	"deadcases/internal/game"
)

func TestUpsolve(t *testing.T) {
	t.Parallel()
	day := 24 * time.Hour
	reg := func(stage int, due time.Time, days []int) []game.Regression {
		return []game.Regression{{Problem: "x", Due: due, Stage: stage, Days: days}}
	}
	tests := []struct {
		name      string
		meta      game.Meta
		id        catalog.ID
		kind      game.SolveKind
		at        time.Time
		recovered int64
		dump      bool // the core dump survives
		regs      []game.Regression
	}{
		{"recovers the dump", game.Meta{Bank: 5, CoreDump: &game.CoreDump{Problem: "x", Bytes: 120}},
			"x", game.Editorial, t0, 120, false, nil},
		{"another problem leaves it", game.Meta{CoreDump: &game.CoreDump{Problem: "x", Bytes: 120}},
			"y", game.Independent, t0, 0, true, nil},
		{"before due: schedule kept", game.Meta{Regressions: reg(0, t0.Add(3*day), []int{3, 17})},
			"x", game.Independent, t0.Add(day), 0, false, reg(0, t0.Add(3*day), []int{3, 17})},
		{"after due: next stage", game.Meta{Regressions: reg(0, t0, []int{3, 17})},
			"x", game.Hinted, t0.Add(4 * day), 0, false, reg(1, t0.Add(21*day), []int{3, 17})},
		{"last stage: retired", game.Meta{Regressions: reg(1, t0, []int{3, 17})},
			"x", game.Independent, t0.Add(day), 0, false, nil},
		{"editorial keeps it scheduled", game.Meta{Regressions: reg(0, t0, []int{3, 17})},
			"x", game.Editorial, t0.Add(day), 0, false, reg(0, t0, []int{3, 17})},
		{"no snapshot: standard days", game.Meta{Regressions: reg(0, t0, nil)},
			"x", game.Independent, t0, 0, false, reg(1, t0.Add(17*day), nil)},
	}
	for _, tc := range tests {
		m := tc.meta
		bank := m.Bank
		got := game.Upsolve(&m, tc.id, tc.kind, 30*time.Minute, tc.at)
		if got != tc.recovered || m.Bank != bank+tc.recovered {
			t.Errorf("%s: recovered %d bank %d, want %d and %d", tc.name, got, m.Bank, tc.recovered, bank+tc.recovered)
		}
		if (m.CoreDump != nil) != tc.dump {
			t.Errorf("%s: core dump %+v", tc.name, m.CoreDump)
		}
		if mustJSON(t, m.Regressions) != mustJSON(t, tc.regs) {
			t.Errorf("%s: regressions %+v, want %+v", tc.name, m.Regressions, tc.regs)
		}
		s := m.Solves[len(m.Solves)-1]
		if !s.Upsolve || s.Kind != tc.kind || s.Elapsed != 30*time.Minute || !s.At.Equal(tc.at) {
			t.Errorf("%s: solve %+v", tc.name, s)
		}
	}
}

func TestDueRegressions(t *testing.T) {
	t.Parallel()
	m := &game.Meta{Regressions: []game.Regression{
		{Problem: "late", Due: t0.Add(time.Hour)},
		{Problem: "b", Due: t0.Add(-time.Hour)},
		{Problem: "now", Due: t0},
		{Problem: "a", Due: t0.Add(-time.Hour)},
		{Problem: "old", Due: t0.Add(-48 * time.Hour)},
	}}
	var got []string
	for _, g := range game.DueRegressions(m, t0) {
		got = append(got, string(g.Problem))
	}
	if want := []string{"old", "a", "b", "now"}; !slices.Equal(got, want) {
		t.Errorf("due %v, want %v", got, want)
	}
}

func TestCapabilitiesAndLibs(t *testing.T) {
	t.Parallel()
	m := &game.Meta{}
	if game.Unlocked(m, "dp") || !game.Unlocked(m, "simulation") || !game.Unlocked(m, game.Mixed) {
		t.Error("gating wrong for a new player")
	}
	game.GrantCapability(m, game.CapabilityFor("dp"))
	game.GrantCapability(m, "CAP_DP")
	game.GrantCapability(m, "")
	if !slices.Equal(m.Capabilities, []string{"CAP_DP"}) || !game.Unlocked(m, "dp") {
		t.Errorf("capabilities %v", m.Capabilities)
	}
	for topic, c := range game.TopicCapabilities {
		if !slices.Contains(catalog.Topics, topic) || c == "" {
			t.Errorf("capability %q for unknown topic %q", c, topic)
		}
	}

	game.AddLib(m, game.Lib{Name: "dsu", Source: "v1"})
	game.AddLib(m, game.Lib{Name: "segtree", Source: "s"})
	game.AddLib(m, game.Lib{Name: "dsu", Source: "v2"})
	if len(m.Libs) != 2 || m.Libs[0].Source != "v2" {
		t.Errorf("libs %+v", m.Libs)
	}
}
