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
			ref("cf-9A", "Very Easy", true, "internal"), // external: never playable
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
