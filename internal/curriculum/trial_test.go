package curriculum

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

func ref(uid, diff string, starred bool, kind string) ProblemRef {
	return ProblemRef{UniqueID: uid, ID: CatalogID(uid), Difficulty: diff, Starred: starred, SolutionKind: kind}
}

var trialModule = Module{
	ID: "prefix-sums", Division: "silver", Topic: "prefix-sums",
	Problems: []ProblemList{
		{Key: "sample", Problems: []ProblemRef{ref("usaco-1", "Easy", false, "in-module")}},
		{Key: "general", Problems: []ProblemRef{
			ref("usaco-1", "Easy", true, "internal"), // focus problem again, starred here
			ref("usaco-2", "Easy", false, "internal"),
			ref("usaco-3", "Easy", true, "internal"),
			ref("ac-abc100_a", "Very Easy", true, "internal"), // external: never playable
			ref("cses-10", "Medium", false, "internal"),
			ref("usaco-4", "Hard", false, "internal"),
			ref("usaco-5", "Very Easy", false, "internal"),
			ref("usaco-6", "Insane", false, "internal"),
			ref("usaco-7", "Hard", true, "internal"),
			ref("usaco-8", "N/A", true, "internal"),
		}},
	},
}

func ids(s ...string) []catalog.ID {
	var out []catalog.ID
	for _, x := range s {
		out = append(out, catalog.ID(x))
	}
	return out
}

func TestTrialUnit(t *testing.T) {
	noCSES := func(id catalog.ID) bool { return id.Source() != catalog.CSES }
	for _, tc := range []struct {
		name      string
		judgeable func(catalog.ID) bool
		n         int
		want      []catalog.ID
	}{
		// One per label, easiest labels first; within Easy the starred
		// non-focus problem beats the focus problem; N/A sorts last.
		{"default", nil, DefaultUnitSize, ids("usaco:5", "usaco:3", "cses:10", "usaco:7")},
		{"second round", nil, 7, ids("usaco:5", "usaco:3", "usaco:1", "cses:10", "usaco:7", "usaco:6", "usaco:8")},
		{"judgeable filter", noCSES, 4, ids("usaco:5", "usaco:3", "usaco:7", "usaco:6")},
		{"everything, deduplicated", nil, 100, ids("usaco:5", "usaco:3", "usaco:1", "usaco:2", "cses:10", "usaco:7", "usaco:4", "usaco:6", "usaco:8")},
		{"nothing judgeable", func(catalog.ID) bool { return false }, 4, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := TrialForN(trialModule, tc.judgeable, tc.n)
			if !reflect.DeepEqual(got.Unit, tc.want) {
				t.Errorf("unit = %v, want %v", got.Unit, tc.want)
			}
		})
	}
}

func TestTrialIntegration(t *testing.T) {
	for _, tc := range []struct {
		m     Module
		topic string
		band  [2]int
		cap   string
	}{
		{trialModule, "prefix-sums", [2]int{1350, 1650}, "CAP_PREFIX"},
		{Module{Division: "bronze", Topic: "mixed"}, "mixed", [2]int{1000, 1300}, ""},
		{Module{Division: "general"}, "mixed", [2]int{1000, 1300}, ""},
		{Module{Division: "gold", Topic: "dp"}, "dp", [2]int{1700, 2000}, "CAP_DP"},
		{Module{Division: "plat", Topic: "trees"}, "trees", [2]int{2100, 2400}, "CAP_TREE"},
		{Module{Division: "adv", Topic: "strings"}, "strings", [2]int{2400, 2800}, "CAP_STRING"},
		{Module{Division: "bronze", Topic: "simulation"}, "simulation", [2]int{1000, 1300}, ""},
	} {
		got := TrialFor(tc.m, nil)
		if got.IntegTopic != tc.topic || got.IntegBand != tc.band || got.Capability != tc.cap {
			t.Errorf("%s/%q: topic=%q band=%v cap=%q, want %q %v %q",
				tc.m.Division, tc.m.Topic, got.IntegTopic, got.IntegBand, got.Capability, tc.topic, tc.band, tc.cap)
		}
	}
}

// TestTrialSpec: the curriculum's trial is what the engine plays, and a
// short unit is the engine's to top up.
func TestTrialSpec(t *testing.T) {
	tr := TrialForN(trialModule, nil, 2)
	spec := tr.Spec()
	if spec.Module != "prefix-sums" || spec.IntegTopic != "prefix-sums" || spec.IntegBand != [2]int{1350, 1650} ||
		spec.Capability != "CAP_PREFIX" || !reflect.DeepEqual(spec.Unit, tr.Unit) {
		t.Fatalf("spec %+v from trial %+v", spec, tr)
	}
	spec.Unit[0] = "x"
	if tr.Unit[0] == "x" {
		t.Error("Spec shares the trial's unit")
	}
	var pool []catalog.Problem
	for i, id := range append(slices.Clone(tr.Unit), ids("usaco:900", "usaco:901", "usaco:902", "usaco:903", "usaco:904")...) {
		pool = append(pool, catalog.Problem{ID: id, Topic: "prefix-sums", Rating: 1360 + 50*i, Judgeable: true})
	}
	m := &game.Meta{}
	s, _, err := game.NewTrial(rules.Trial(), m, pool, tr.Spec(), 1, time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Trial.Plan[0]) != 4 || len(s.Trial.TopUp) != 2 || len(s.Trial.Plan[1]) != 2 {
		t.Errorf("plan %v, top-ups %v: want a unit of 4 (2 topped up) and 2 integration doors", s.Trial.Plan, s.Trial.TopUp)
	}
}

func TestTrialForFixture(t *testing.T) {
	m, _ := loadFixture(t).Module("prefix-sums")
	got := TrialFor(m, func(id catalog.ID) bool { return id.Source() == catalog.USACO })
	// The fixture's USACO problems: 572, 691 (Very Easy), 595 (Easy), 1471
	// (Medium, starred). One per label, then a second Very Easy.
	want := ids("usaco:572", "usaco:691", "usaco:595", "usaco:1471")
	if !reflect.DeepEqual(got.Unit, want) {
		t.Errorf("unit = %v, want %v", got.Unit, want)
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
		got := TrialFor(m, usaco)
		if got.Kind != tc.kind || got.IntegTopic != tc.topic || got.Capability != tc.cap {
			t.Errorf("%s: kind %v topic %q cap %q, want %v %q %q", tc.id, got.Kind, got.IntegTopic, got.Capability, tc.kind, tc.topic, tc.cap)
		}
		if l, _, _ := c.Atlas(usaco).Level(tc.id); l.Kind != got.Kind {
			t.Errorf("%s: trial kind %v, atlas kind %v", tc.id, got.Kind, l.Kind)
		}
	}
	// No judgeable problem: a page, but a conclusion stays a castle and a
	// module that grants a capability keeps its trial, all topped up.
	none := func(catalog.ID) bool { return false }
	for id, want := range map[string]LevelKind{"casework": PageLevel, "bronze-conclusion": CastleLevel, "prefix-sums": TrialLevel} {
		m, _ := c.Module(id)
		if got := TrialFor(m, none).Kind; got != want {
			t.Errorf("%s with nothing judgeable: kind %v, want %v", id, got, want)
		}
		if l, _, _ := c.Atlas(none).Level(id); l.Kind != want {
			t.Errorf("%s with nothing judgeable: atlas kind %v, want %v", id, l.Kind, want)
		}
	}
}

// A castle's trial is the division's gauntlet: the conclusion's own
// problems ramped, the integration doors on every topic at the division's
// band, nothing granted, and the engine plays it as it plays any trial.
func TestCastleTrialSpec(t *testing.T) {
	m, _ := loadFixture(t).Module("bronze-conclusion")
	tr := TrialFor(m, func(id catalog.ID) bool { return id.Source() == catalog.USACO })
	spec := tr.Spec()
	// The conclusion's USACO problems: 642, 509 (Medium), 1181, 1085 (Hard).
	if want := ids("usaco:642", "usaco:509", "usaco:1181", "usaco:1085"); !reflect.DeepEqual(spec.Unit, want) {
		t.Errorf("unit %v, want %v", spec.Unit, want)
	}
	if spec.Module != "bronze-conclusion" || spec.IntegTopic != catalog.Mixed ||
		spec.IntegBand != DivisionBand("bronze") || spec.Capability != "" {
		t.Errorf("spec %+v, want bronze-conclusion, mixed at Bronze's band, no capability", spec)
	}

	// Two of the unit judgeable, the rest of the pool on other topics:
	// the engine tops the unit up and draws integration from any topic.
	spec.Unit = spec.Unit[:2]
	var pool []catalog.Problem
	topics := []string{"graphs", "dp", "simulation", "math", "strings", "trees"}
	for i, id := range append(slices.Clone(spec.Unit), ids("usaco:900", "usaco:901", "usaco:902", "usaco:903")...) {
		pool = append(pool, catalog.Problem{ID: id, Topic: topics[i], Rating: 1000 + 50*i, Judgeable: true})
	}
	meta := &game.Meta{}
	s, _, err := game.NewTrial(rules.Trial(), meta, pool, spec, 1, time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Trial.Plan[0]) != 4 || len(s.Trial.TopUp) != 2 || len(s.Trial.Plan[1]) != 2 {
		t.Errorf("plan %v, top-ups %v: want a unit of 4 (2 topped up) and 2 integration doors", s.Trial.Plan, s.Trial.TopUp)
	}
}
