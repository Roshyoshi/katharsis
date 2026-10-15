package rules_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"katharsis/internal/rules"
)

func TestMapValid(t *testing.T) {
	t.Parallel()
	r := rules.Map()
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := r.WithDefaults().Validate(); err != nil {
		t.Fatal(err)
	}
	if len(r.Biomes) != 7 {
		t.Fatalf("%d tiers, want 7", len(r.Biomes))
	}
	for i, b := range r.Biomes {
		boss := i == rules.TierBossI || i == rules.TierBossII || i == rules.TierFinal
		if (b.Kind == rules.Boss) != boss || b.FailCycles <= 0 {
			t.Errorf("tier %d: %+v", i, b)
		}
	}
	// Every node of a tier shares its tier's aim, clock and price, so the
	// route screen's numbers for a tier are its exits'.
	for _, n := range r.Nodes {
		tb := r.Biomes[n.Tier]
		if n.TargetClear != tb.TargetClear || n.MinClear != tb.MinClear || n.Timer != tb.Timer || n.FailCycles != tb.FailCycles ||
			!n.Skip && n.Rooms != tb.Rooms {
			t.Errorf("node %s differs from tier %d: %+v vs %+v", n.ID, n.Tier, n, tb)
		}
	}
	if r.Cycles != 30 || r.MaxCycles != 50 || r.SubmitPrice() != 2 || r.RetreatPrice() != 5 || r.FsckHeal() != 10 || r.Life() != 10 ||
		r.OneShot != 1 || r.ECCPenalty.Duration != 5*time.Minute || r.RetreatLock.Duration != 150*time.Second ||
		r.OverclockMax != 2 || r.OverclockCycles != 5 || r.Clock() != 1 || !r.FailMoveOn || !r.O3OnBase || !r.KexecPerAct ||
		!r.FirstClockOwn || r.FountainCycles != 5 || r.FountainWhere != rules.FountainAct {
		t.Errorf("map numbers %+v", r)
	}
	if got := []int{r.Biomes[0].FailCycles, r.Biomes[1].FailCycles, r.Biomes[2].FailCycles, r.Biomes[3].FailCycles,
		r.Biomes[4].FailCycles, r.Biomes[5].FailCycles}; !slices.Equal(got, []int{10, 10, 20, 12, 20, 15}) {
		t.Errorf("fail prices %v, want 10 10 20 12 20 15 (x1, x1.25, x1.5 by act)", got)
	}
	if r.Biomes[0].Timer.Duration != 10*time.Minute {
		t.Errorf("Userland %s, want 10:00 (D50)", r.Biomes[0].Timer)
	}
	if r.TopPrestige(8) != 8 || rules.Legacy().TopPrestige(8) != 4 {
		t.Errorf("tops %d, %d", r.TopPrestige(8), rules.Legacy().TopPrestige(8))
	}
}

// TestMapLadder is map.md §(d)'s P1-P8, summed as a boot at each level
// plays it.
func TestMapLadder(t *testing.T) {
	t.Parallel()
	r := rules.Map()
	tests := []struct {
		p                                    int
		submit, heal, late, cycles, fountain int
		oneShot                              int
		first                                time.Duration
		timer                                float64
	}{
		{0, 0, 0, 0, 0, 0, 0, 0, 1},
		{1, 1, 0, 0, 0, 0, 0, 0, 0.95},
		{2, 1, -5, 0, 0, 0, 0, 0, 0.95},
		{3, 1, -5, 5, 0, 0, 0, 9 * time.Minute, 0.95 * 0.9},
		{4, 1, -5, 5, -5, 0, 0, 9 * time.Minute, 0.95 * 0.81},
		{5, 1, -5, 5, -5, -5, 0, 9 * time.Minute, 0.95 * 0.81},
		{6, 1, -5, 5, -5, -5, 2, 8 * time.Minute, 0.95 * 0.81},
		{7, 2, -5, 5, -10, -5, 2, 8 * time.Minute, 0.95 * 0.81},
		{8, 2, -5, 5, -10, -5, 3, 8 * time.Minute, 0.95 * 0.81},
	}
	for _, tc := range tests {
		a := r.Ascend(tc.p)
		if a.SubmitCycles != tc.submit || a.HealCycles != tc.heal || a.LateFailCycles != tc.late || a.Cycles != tc.cycles ||
			a.FountainCycles != tc.fountain || a.OneShot != tc.oneShot || a.FirstTimer.Duration != tc.first ||
			math.Abs(a.TimerMult-tc.timer) > 1e-9 {
			t.Errorf("P%d: %+v", tc.p, a)
		}
	}
	// FirstTimer is the highest level's, whatever the slice order.
	r.Ascensions = slices.Clone(r.Ascensions)
	slices.Reverse(r.Ascensions)
	if a := r.Ascend(8); a.FirstTimer.Duration != 8*time.Minute {
		t.Errorf("reversed ladder: first timer %s", a.FirstTimer)
	}
}

func TestValidateNodes(t *testing.T) {
	t.Parallel()
	edit := func(f func(r *rules.Rules)) rules.Rules {
		r := rules.Map()
		r.Nodes = slices.Clone(r.Nodes)
		f(&r)
		return r
	}
	at := func(r *rules.Rules, id string) *rules.Biome {
		i := slices.IndexFunc(r.Nodes, func(n rules.Biome) bool { return n.ID == id })
		return &r.Nodes[i]
	}
	tests := map[string]rules.Rules{
		"duplicate id":     edit(func(r *rules.Rules) { at(r, "alu").ID = "heap" }),
		"empty id":         edit(func(r *rules.Rules) { at(r, "alu").ID = "" }),
		"tier past end":    edit(func(r *rules.Rules) { at(r, "alu").Tier = 7 }),
		"two starts":       edit(func(r *rules.Rules) { at(r, "bunker").Tier = 0 }),
		"no start":         edit(func(r *rules.Rules) { at(r, "userland").Tier = 1 }),
		"exit skips tier":  edit(func(r *rules.Rules) { at(r, "userland").Next = []string{"alu"} }),
		"exit to nowhere":  edit(func(r *rules.Rules) { at(r, "alu").Next = []string{"nowhere"} }),
		"exit twice":       edit(func(r *rules.Rules) { at(r, "alu").Next = []string{"heisenbug", "heisenbug"} }),
		"dead end":         edit(func(r *rules.Rules) { at(r, "alu").Next = nil }),
		"final with exit":  edit(func(r *rules.Rules) { at(r, "halting-problem").Next = []string{"userland"} }),
		"boss unnamed":     edit(func(r *rules.Rules) { at(r, "heisenbug").Boss = "" }),
		"topic boss":       edit(func(r *rules.Rules) { at(r, "alu").Boss = "off-by-one" }),
		"boss skip":        edit(func(r *rules.Rules) { at(r, "heisenbug").Skip = true }),
		"no rooms":         edit(func(r *rules.Rules) { at(r, "alu").Rooms = 0 }),
		"bad aim":          edit(func(r *rules.Rules) { at(r, "alu").TargetClear = 1 }),
		"floor over aim":   edit(func(r *rules.Rules) { at(r, "alu").MinClear = 0.9 }),
		"no timer":         edit(func(r *rules.Rules) { at(r, "alu").Timer = rules.Duration{} }),
		"negative picks":   edit(func(r *rules.Rules) { at(r, "alu").Picks = -1 }),
		"negative fail":    edit(func(r *rules.Rules) { at(r, "alu").FailCycles = -1 }),
		"negative submit":  edit(func(r *rules.Rules) { r.SubmitCycles = -2 }),
		"bad fountain":     edit(func(r *rules.Rules) { r.FountainWhere = "always" }),
		"nan timer mult":   edit(func(r *rules.Rules) { r.TimerMult = math.NaN() }),
		"negative lock":    edit(func(r *rules.Rules) { r.RetreatLock = rules.Duration{Duration: -time.Second} }),
		"bad race pay":     edit(func(r *rules.Rules) { r.Twists.RacePay = math.Inf(1) }),
		"negative oneshot": edit(func(r *rules.Rules) { r.OneShot = -1 }),
	}
	for name, r := range tests {
		if err := r.Validate(); err == nil {
			t.Errorf("%s: valid", name)
		}
	}
	// A skip plays no room and still validates.
	if err := edit(func(r *rules.Rules) { at(r, "skip").Rooms = 0 }).Validate(); err != nil {
		t.Errorf("skip without rooms: %v", err)
	}
}

// TestKeysKept: lives were renamed cycles (D48) but their keys were not,
// so a preset and a saved boot read as before.
func TestKeysKept(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(rules.Map())
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{`"Lives":30`, `"MaxLives":50`, `"Lives":-5`} {
		if !strings.Contains(string(b), k) {
			t.Errorf("rules JSON has no %s", k)
		}
	}
	if strings.Contains(string(b), `"Cycles"`) || strings.Contains(string(b), `"MaxCycles"`) {
		t.Error("rules JSON names cycles; old saves name lives")
	}
	var old rules.Rules
	if err := json.Unmarshal([]byte(`{"Lives":3,"MaxLives":5,"Ascensions":[{"Level":4,"Lives":-1}]}`), &old); err != nil ||
		old.Cycles != 3 || old.MaxCycles != 5 || old.Ascend(4).Cycles != -1 {
		t.Errorf("old rules JSON reads %+v, %v", old, err)
	}
}

func TestLoadNodes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "tiny.toml")
	err := os.WriteFile(path, []byte(`
lives = 30
max_lives = 50
fail_move_on = true

[[biome]]
kind = "fixed"
rooms = 1
target_clear = 0.9
timer = "10m"
fail_cycles = 10

[[biome]]
kind = "boss"
rooms = 1
target_clear = 0.5
timer = "30m"
fail_cycles = 20

[[node]]
id = "start"
tier = 0
rooms = 1
target_clear = 0.9
timer = "10m"
next = ["boss"]

[[node]]
id = "boss"
tier = 1
boss = "off-by-one"
rooms = 1
target_clear = 0.5
timer = "30m"

[twists]
off_by_one = 3
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	r, err := rules.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Nodes) != 2 || r.Nodes[1].Boss != "off-by-one" || r.Cycles != 30 || !r.FailMoveOn || r.Twists.OffByOne != 3 ||
		r.Twists.RaceClock != rules.Standard().Twists.RaceClock {
		t.Errorf("loaded %+v", r)
	}
}
