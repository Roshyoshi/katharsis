package curriculum

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

func ref(uid, diff string, starred bool, kind string) ProblemRef {
	return ProblemRef{UniqueID: uid, ID: CatalogID(uid), Difficulty: diff, Starred: starred, SolutionKind: kind}
}

func ids(s ...string) []catalog.ID {
	var out []catalog.ID
	for _, x := range s {
		out = append(out, catalog.ID(x))
	}
	return out
}

// TestTrialAim: the doors climb from the core's median rating to its
// hardest and a window past it. Side problems are stretch and aim
// nothing; CSES ratings count only when nothing else on the core is rated.
func TestTrialAim(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		rated map[catalog.ID]int // the catalog's ratings; nil is no lookup
		want  []int
	}{
		{"median, top, past it", []string{
			"core usaco-1 > usaco-2 > usaco-3",
			"core cses-10 > usaco-3",
			"side usaco-3 > usaco-4",
		}, map[catalog.ID]int{"usaco:1": 1200, "usaco:2": 1400, "usaco:3": 1600, "cses:10": 3000, "usaco:4": 2400},
			[]int{1400, 1600, 1700}},
		{"the lower median", []string{"core usaco-1 > usaco-2"},
			map[catalog.ID]int{"usaco:1": 1200, "usaco:2": 1400}, []int{1200, 1400, 1500}},
		{"one rated", []string{"core usaco-1 > usaco-2"},
			map[catalog.ID]int{"usaco:2": 1500}, []int{1500, 1500, 1600}},
		{"CSES alone", []string{"core cses-10 > cses-11"},
			map[catalog.ID]int{"cses:10": 1500, "cses:11": 1900}, []int{1500, 1900, 2000}},
		{"nothing rated", []string{"core usaco-1"}, map[catalog.ID]int{}, nil},
		{"no lookup", []string{"core usaco-1"}, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mp, err := ParseMap("m", strings.Join(tc.lines, "\n"))
			if err != nil {
				t.Fatal(err)
			}
			var lookup func(catalog.ID) (catalog.Problem, bool)
			if tc.rated != nil {
				lookup = func(id catalog.ID) (catalog.Problem, bool) {
					r, ok := tc.rated[id]
					return catalog.Problem{ID: id, Rating: r}, ok
				}
			}
			if got := aim(mp, lookup); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("aim = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTrialTopic(t *testing.T) {
	for _, tc := range []struct {
		m     Module
		topic string
		band  [2]int
		cap   string
	}{
		{Module{Division: "silver", Topic: "prefix-sums"}, "prefix-sums", [2]int{1350, 1650}, "CAP_PREFIX"},
		{Module{Division: "bronze", Topic: "mixed"}, "mixed", [2]int{1000, 1300}, ""},
		{Module{Division: "general"}, "mixed", [2]int{1000, 1300}, ""},
		{Module{Division: "gold", Topic: "dp"}, "dp", [2]int{1700, 2000}, "CAP_DP"},
		{Module{Division: "plat", Topic: "trees"}, "trees", [2]int{2100, 2400}, "CAP_TREE"},
		{Module{Division: "adv", Topic: "strings"}, "strings", [2]int{2400, 2800}, "CAP_STRING"},
		{Module{Division: "bronze", Topic: "simulation"}, "simulation", [2]int{1000, 1300}, ""},
	} {
		got := TrialFor(tc.m, nil, nil)
		if got.Topic != tc.topic || got.Band != tc.band || got.Capability != tc.cap {
			t.Errorf("%s/%q: topic=%q band=%v cap=%q, want %q %v %q",
				tc.m.Division, tc.m.Topic, got.Topic, got.Band, got.Capability, tc.topic, tc.band, tc.cap)
		}
	}
}

// TestTrialSpec: the curriculum's trial is what the engine plays: one
// door per target, on the topic first, from the pool it is given.
func TestTrialSpec(t *testing.T) {
	tr := Trial{
		Module: Module{ID: "prefix-sums", Title: "Prefix Sums"}, Topic: "prefix-sums",
		Band: DivisionBand("silver"), Targets: []int{1400, 1600, 1700}, Capability: "CAP_PREFIX",
	}
	spec := tr.Spec()
	if spec.Module != "prefix-sums" || spec.Topic != "prefix-sums" || spec.Band != [2]int{1350, 1650} ||
		spec.Capability != "CAP_PREFIX" || !reflect.DeepEqual(spec.Targets, tr.Targets) {
		t.Fatalf("spec %+v from trial %+v", spec, tr)
	}
	spec.Targets[0] = 0
	if tr.Targets[0] == 0 {
		t.Error("Spec shares the trial's targets")
	}

	// On the topic at 1400, 1600 and 1800; off it, right on the aims.
	var pool []catalog.Problem
	add := func(id, topic string, rating int) {
		pool = append(pool, catalog.Problem{ID: catalog.ID(id), Topic: topic, Rating: rating, Judgeable: true})
	}
	add("usaco:900", "prefix-sums", 1400)
	add("usaco:901", "prefix-sums", 1600)
	add("usaco:902", "prefix-sums", 1800)
	add("usaco:903", "graphs", 1400)
	add("usaco:904", "graphs", 1600)
	add("usaco:905", "graphs", 1700)
	s, _, err := game.NewTrial(rules.Trial(), &game.Meta{}, pool, tr.Spec(), 1, time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := slices.Concat(s.Trial.Plan...), ids("usaco:900", "usaco:901", "usaco:902"); !reflect.DeepEqual(got, want) {
		t.Errorf("doors %v, want %v", got, want)
	}
}

// The fixture's Prefix Sums plays the embedded map, cut to what the
// fixture lists and the catalog judges.
func TestTrialForFixture(t *testing.T) {
	m, _ := loadFixture(t).Module("prefix-sums")
	usaco := func(id catalog.ID) bool { return id.Source() == catalog.USACO }
	got := TrialFor(m, usaco, nil)
	if got.Map.Core() == 0 {
		t.Fatalf("map %+v has no core", got.Map)
	}
	listed := map[catalog.ID]bool{}
	for _, l := range m.Problems {
		for _, p := range l.Problems {
			listed[p.ID] = true
		}
	}
	for _, n := range got.Map.Nodes {
		if !listed[n.ID] || !usaco(n.ID) {
			t.Errorf("%s (%s): not a USACO problem the fixture lists", n.Problem, n.ID)
		}
	}
}

// Every level but a page has a trial: one that grants no capability for an
// ungated topic, a castle's for a conclusion, and none for General. The
// trial's kind is the atlas's.
func TestTrialKinds(t *testing.T) {
	c := loadFixture(t)
	usaco := func(id catalog.ID) bool { return id.Source() == catalog.USACO }
	for _, tc := range []struct {
		id    string
		kind  LevelKind
		topic string
		cap   string
	}{
		{"fast-io", PageLevel, catalog.Mixed, ""},
		{"casework", TrialLevel, "mixed", ""},
		{"bronze-conclusion", CastleLevel, catalog.Mixed, ""},
		{"prefix-sums", TrialLevel, "prefix-sums", "CAP_PREFIX"},
		{"matrix-expo", TrialLevel, "math", ""},
	} {
		m, ok := c.Module(tc.id)
		if !ok {
			t.Fatalf("fixture has no %s", tc.id)
		}
		got := TrialFor(m, usaco, nil)
		if got.Kind != tc.kind || got.Topic != tc.topic || got.Capability != tc.cap {
			t.Errorf("%s: kind %v topic %q cap %q, want %v %q %q", tc.id, got.Kind, got.Topic, got.Capability, tc.kind, tc.topic, tc.cap)
		}
		if l, _, _ := c.Atlas(usaco).Level(tc.id); l.Kind != got.Kind {
			t.Errorf("%s: trial kind %v, atlas kind %v", tc.id, got.Kind, l.Kind)
		}
	}
	// No judgeable problem: a page, even for a module that grants a
	// capability (it is earned in another module on the topic: a trial
	// with nothing to practise before it tests nothing the module taught);
	// a conclusion stays a castle.
	none := func(catalog.ID) bool { return false }
	for id, want := range map[string]LevelKind{"casework": PageLevel, "bronze-conclusion": CastleLevel, "prefix-sums": PageLevel} {
		m, _ := c.Module(id)
		if got := TrialFor(m, none, nil).Kind; got != want {
			t.Errorf("%s with nothing judgeable: kind %v, want %v", id, got, want)
		}
		if l, _, _ := c.Atlas(none).Level(id); l.Kind != want {
			t.Errorf("%s with nothing judgeable: atlas kind %v, want %v", id, l.Kind, want)
		}
	}
}

// A castle's trial is the division's gauntlet: every topic at the
// division's band, nothing granted, and the engine plays it as it plays
// any trial.
func TestCastleTrialSpec(t *testing.T) {
	m, _ := loadFixture(t).Module("bronze-conclusion")
	spec := TrialFor(m, func(id catalog.ID) bool { return id.Source() == catalog.USACO }, nil).Spec()
	if spec.Module != "bronze-conclusion" || spec.Topic != catalog.Mixed ||
		spec.Band != DivisionBand("bronze") || spec.Capability != "" {
		t.Errorf("spec %+v, want bronze-conclusion, mixed at Bronze's band, no capability", spec)
	}
	var pool []catalog.Problem
	for i, topic := range []string{"graphs", "dp", "simulation"} {
		pool = append(pool, catalog.Problem{ID: catalog.NewID(catalog.USACO, string(rune('1'+i))), Topic: topic, Rating: 1100, Judgeable: true})
	}
	s, _, err := game.NewTrial(rules.Trial(), &game.Meta{}, pool, spec, 1, time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got := slices.Concat(s.Trial.Plan...); len(got) != Doors {
		t.Errorf("doors %v, want %d from any topic", got, Doors)
	}
}
