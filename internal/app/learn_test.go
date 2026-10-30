package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/curriculum"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/hud"
	"katharsis/internal/judge"
	"katharsis/internal/problem"
	"katharsis/internal/screens"
	"katharsis/internal/screens/rtfm"
)

// srcRunner writes each door's source the way the tmux runner does (libs
// pasted into a fresh template) and reports its path, so lib harvesting and
// linking run through the real app paths.
type srcRunner struct {
	*fakeRunner
	dir   string
	src   string
	libs  [][]game.Lib // Door.Libs per Open
	write string       // what the "player" saves into each new source
}

func (r *srcRunner) Open(ctx context.Context, d Door) error {
	if err := r.fakeRunner.Open(ctx, d); err != nil {
		return err
	}
	r.libs = append(r.libs, d.Libs)
	r.src = filepath.Join(r.dir, strings.ReplaceAll(string(d.Door.Problem), ":", "")+".cpp")
	return os.WriteFile(r.src, []byte(r.write), 0o644)
}

func (r *srcRunner) Source() string { return r.src }

func TestLibsHarvestedOnACAndLinkedNextBoot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var r *srcRunner
		h := newHarnessWith(t, func(f *fakeRunner) Runner {
			r = &srcRunner{fakeRunner: f, dir: t.TempDir(),
				write: "// lib: dsu\nstruct DSU {};\n// end lib\nint main() {}\n"}
			return r
		})
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		h.send(submitMsg{}) // the fake judge accepts
		if !slices.ContainsFunc(h.m.meta.Libs, func(l game.Lib) bool { return l.Name == "dsu" && l.Source == "struct DSU {};\n" }) {
			t.Fatalf("libs after an AC = %+v, want dsu", h.m.meta.Libs)
		}

		// A new boot offers the link screen; link dsu and open a door. The
		// boot in flight ends first, the hub asking.
		h.send(screens.StartBoot{Preset: "standard"})
		h.send(key("y"))
		if _, ok := h.m.top().(*linkScreen); !ok {
			t.Fatalf("top screen %T, want the link screen", h.m.top())
		}
		h.send(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
		h.send(tea.KeyPressMsg{Code: tea.KeyEnter})
		if !slices.Equal(h.m.state.Linked, []string{"dsu"}) {
			t.Fatalf("linked %v, want [dsu]", h.m.state.Linked)
		}
		h.enterDoor()
		got := r.libs[len(r.libs)-1]
		if len(got) != 1 || got[0].Name != "dsu" {
			t.Fatalf("door opened with libs %+v, want dsu", got)
		}
	})
}

func TestNoLinkScreenWithoutLibs(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.send(screens.StartBoot{Preset: "standard"})
		if _, ok := h.m.top().(*linkScreen); ok {
			t.Fatal("link screen shown with an empty lib bank")
		}
	})
}

// loadGuide gives the app the curriculum package's fixture guide as its RTFM
// curriculum.
func loadGuide(h *harness) *curriculum.Curriculum {
	h.t.Helper()
	cur, err := curriculum.Load(filepath.Join("..", "curriculum", "testdata", "guide"))
	if err != nil {
		h.t.Fatal(err)
	}
	// The fake catalog judges the guide's own Silver problems, as the real
	// one does, so its modules have trials of their own: a trial needs one.
	if c, ok := h.m.d.Catalog.(*fakeCatalog); ok {
		for _, id := range []catalog.ID{"usaco:572", "usaco:595", "usaco:691", "usaco:1471", "usaco:738", "usaco:643"} {
			if _, have := c.byID[id]; !have {
				p := catalog.Problem{ID: id, Title: string(id), Topic: "prefix-sums", Division: "silver", Rating: 1400, Judgeable: true}
				c.all, c.byID[id] = append(c.all, p), p
			}
		}
	}
	h.m.cur = cur
	return cur
}

// readyTrial puts module id's trial in reach, as playing up to it would:
// the trial before it on the path passed, and its map's core solved.
func readyTrial(h *harness, id string) {
	h.t.Helper()
	a := h.m.cur.Atlas(h.m.judgeable)
	l, _, ok := a.Level(id)
	if !ok {
		h.t.Fatalf("no level %q", id)
	}
	if b, ok := a.Before(id); ok {
		h.m.meta.TrialsPassed = append(h.m.meta.TrialsPassed, b.Module)
	}
	for _, n := range l.Map.Nodes {
		if n.Core {
			h.m.meta.Solves = append(h.m.meta.Solves, game.Solve{Problem: n.ID, Kind: game.Independent, Upsolve: true})
		}
	}
}

func TestTrialGrantsItsCapability(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		cur := loadGuide(h)
		mod, ok := cur.Module("prefix-sums")
		if !ok {
			t.Fatal("fixture guide has no prefix-sums module")
		}
		want := curriculum.TrialFor(mod, h.m.judgeable, h.m.d.Catalog.Get).Capability
		if want == "" {
			t.Fatal("prefix-sums grants no capability")
		}

		readyTrial(h, "prefix-sums")
		h.send(screens.StartTrial{Module: "prefix-sums"})
		if h.m.state == nil || h.m.state.Trial == nil {
			t.Fatalf("no trial running: %+v", h.m.state)
		}
		if got := h.play(); got != game.PhaseCleared {
			t.Fatalf("trial ended %q, want cleared", got)
		}
		if !slices.Contains(h.m.meta.Capabilities, want) {
			t.Fatalf("capabilities %v after passing, want %s", h.m.meta.Capabilities, want)
		}
	})
}

// A page (General's fast-io) has no trial to start.
func TestPageHasNoTrial(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		h.send(screens.StartTrial{Module: "fast-io"})
		if h.m.state != nil && h.m.state.Trial != nil {
			t.Fatal("a page started a trial")
		}
		// A gated page refuses honestly: it does grant a capability, just
		// not through a trial of its own (Counting Minimums lists no problem
		// the game judges).
		h.send(screens.StartTrial{Module: "count-min"})
		if h.m.state != nil && h.m.state.Trial != nil {
			t.Fatal("a gated page started a trial")
		}
		if !strings.Contains(h.m.flash, "no trial of its own") || strings.Contains(h.m.flash, "grants no capability") {
			t.Errorf("count-min refuses with %q", h.m.flash)
		}
	})
}

// A trial's timer is soft: its door resumed past the deadline goes on in
// overtime, so it reopens; it used to wait, editorless, for an e.
func TestResumedTrialInOvertimeReopens(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		readyTrial(h, "prefix-sums")
		h.send(screens.StartTrial{Module: "prefix-sums"})
		h.clock.t = h.enterDoor().Deadline.Add(time.Minute)
		h.restart()
		if h.m.state.Phase != game.PhaseProblem || h.m.open == nil {
			t.Fatalf("phase %q open %v after resuming a trial door in overtime, want it open", h.m.state.Phase, h.m.open != nil)
		}
	})
}

// TestRebootAfterATrialRetriesIt: r on a failed trial's end screen runs it
// again, the trial rules being no boot preset, with doors drawn anew: one
// solved since is passed over. It needs no guide (a trial resumed after
// a restart has none loaded): the spec is the last try's.
func TestRebootAfterATrialRetriesIt(t *testing.T) {
	for _, loaded := range []bool{true, false} {
		synctest.Test(t, func(t *testing.T) {
			h := newHarness(t)
			loadGuide(h)
			readyTrial(h, "prefix-sums")
			h.send(screens.StartTrial{Module: "prefix-sums"})
			h.verdict = "WA"
			if got := h.play(); got != game.PhaseCrashed || h.m.state.Trial == nil {
				t.Fatalf("trial ended %q (trial %v), want crashed", got, h.m.state.Trial != nil)
			}
			if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "r retry the prefix-sums trial") {
				t.Fatalf("the end screen does not say r retries the trial:\n%s", v)
			}
			first, doors := h.m.state.Boot, slices.Concat(h.m.state.Trial.Plan...)
			// Solved somewhere before the retry.
			h.m.meta.Solves = append(h.m.meta.Solves, game.Solve{Problem: doors[0], Kind: game.Independent, Upsolve: true})
			if !loaded {
				h.m.cur = nil
			}
			h.send(key("r"))
			if s := h.m.state; s.Boot == first || s.Trial == nil || s.Trial.Module != "prefix-sums" || s.Phase == game.PhaseCrashed {
				t.Fatalf("loaded %v, after r: boot %d (was %d), trial %+v, phase %s; want prefix-sums again", loaded, s.Boot, first, s.Trial, s.Phase)
			}
			if got := slices.Concat(h.m.state.Trial.Plan...); slices.Contains(got, doors[0]) || len(got) != len(doors) {
				t.Errorf("loaded %v: retry's doors %v; want %d, without %s (of %v)", loaded, got, len(doors), doors[0], doors)
			}
		})
	}
}

// The first visit to RTFM plays the Mock Turtle's school over the browser,
// once, with the story on; the headless game plays none.
func TestFirstRTFMPlaysItsBeat(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		h.send(screens.OpenLearn{})
		if _, ok := h.m.top().(*rtfm.Map); !ok {
			t.Fatalf("the headless game shows %T over RTFM", h.m.top())
		}

		h = newStoryHarness(t)
		loadGuide(h)
		h.send(screens.OpenLearn{})
		if got, _ := h.watch(); !slices.Equal(got, []string{"story.first-rtfm"}) {
			t.Fatalf("first RTFM shows %v, want the Mock Turtle", got)
		}
		if _, ok := h.m.top().(*rtfm.Map); !ok || !slices.Equal(seenIDs(h.m.meta), []string{"story.first-rtfm"}) {
			t.Fatalf("after the scene: %T on top, seen %v", h.m.top(), seenIDs(h.m.meta))
		}
		h.send(screens.Back{})
		h.send(screens.OpenLearn{})
		if h.scene() != nil {
			t.Errorf("the second visit plays %q", h.sceneName())
		}
	})
}

// A trial starts in the Mock Turtle's words (rtfm.trial.start).
func TestTrialStartsWithItsLine(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		readyTrial(h, "prefix-sums")
		h.send(screens.StartTrial{Module: "prefix-sums"})
		if want := flavor.Story("rtfm.trial.start", 0); want == "" || h.m.flash != want {
			t.Errorf("trial start flashed %q, want %q", h.m.flash, want)
		}
	})
}

// graphGuide is a guide that teaches graphs in three divisions, each
// module's trial earning CAP_GRAPH: Bronze ◆1000–1300, Silver ◆1350–1650,
// Gold ◆1700–2000.
func graphGuide(t *testing.T) *curriculum.Curriculum {
	t.Helper()
	dir := t.TempDir()
	mod := func(id, title string) string {
		return "---\nid: " + id + "\ntitle: " + title + "\nauthor: A\ndescription: d\n---\n\nHi.\n"
	}
	for name, body := range map[string]string{
		"content/ordering.ts": `const MODULE_ORDERING = {
  bronze: [{ name: 'Graphs', items: ['intro-graphs'] }],
  silver: [{ name: 'Graphs', items: ['graph-traversal'] }],
  gold: [{ name: 'Graphs', items: ['toposort'] }],
};
`,
		"content/2_Bronze/Intro_Graphs.mdx":    mod("intro-graphs", "Introduction to Graphs"),
		"content/3_Silver/Graph_Traversal.mdx": mod("graph-traversal", "Graph Traversal"),
		"content/4_Gold/Toposort.mdx":          mod("toposort", "Topological Sort"),
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cur, err := curriculum.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return cur
}

// RTFM on a topic opens at the module nearest the player's rating among
// those whose trial earns the topic's capability, not the first in guide
// order: a 1365 player gets Silver's, a 1850 one Gold's, a 900 one
// Bronze's.
func TestRTFMOpensTheNearestModule(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.m.cur = graphGuide(t)
		for _, tc := range []struct {
			rating float64
			want   string
		}{{1365, "graph-traversal"}, {1850, "toposort"}, {900, "intro-graphs"}} {
			h.m.meta.Rating = tc.rating
			h.send(screens.OpenLearn{Topic: "graphs"})
			p, ok := h.m.top().(*rtfm.Map)
			if !ok {
				t.Fatalf("OpenLearn shows %T", h.m.top())
			}
			if got := p.Selected(); got != tc.want {
				t.Errorf("rating %v: RTFM opened at %s, want %s", tc.rating, got, tc.want)
			}
			h.send(screens.Back{})
		}
	})
}

// RTFM opened at a link (a locked exit's t) goes straight back on q, not to
// the overworld first.
func TestRTFMLinkGoesBack(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.m.cur = graphGuide(t)
		from := h.m.top()
		h.send(screens.OpenLearn{Topic: "graphs"})
		if _, ok := h.m.top().(*rtfm.Map); !ok {
			t.Fatalf("OpenLearn shows %T", h.m.top())
		}
		h.send(key("q"))
		if h.m.top() != from {
			t.Errorf("q on the linked page shows %T, want %T", h.m.top(), from)
		}
	})
}

// A passed trial's next step is the next module in guide order whose
// trial earns a capability the player lacks, or back to RTFM when none is
// left; m (or enter) takes it, and r still retries the trial. It offered
// only the retry.
func TestPassedTrialOffersTheNextModule(t *testing.T) {
	for _, tc := range []struct {
		name  string
		held  []string
		check bool   // the catalog judges Matrix Exponentiation's problem
		line  string // the next step
		page  string // the module m opens at
	}{
		{"next module", nil, false, "m next module: Two Pointers", "two-pointers"},
		// With nothing left to grant, a trial that grants only its ✓.
		{"a check only", []string{"CAP_TWOPTR", "CAP_DS"}, true, "m next module: Matrix Exponentiation", "matrix-expo"},
		{"none left", []string{"CAP_TWOPTR", "CAP_DS"}, false, "m back to RTFM", "prefix-sums"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newHarness(t)
				loadGuide(h)
				for _, c := range tc.held {
					game.GrantCapability(&h.m.meta, c)
				}
				if tc.check {
					c := h.m.d.Catalog.(*fakeCatalog)
					p := catalog.Problem{ID: "usaco:746", Title: "USACO 746", Topic: "mixed", Rating: 2200, Judgeable: true}
					c.all, c.byID[p.ID] = append(c.all, p), p
				}
				readyTrial(h, "prefix-sums")
				h.send(screens.StartTrial{Module: "prefix-sums"})
				if got := h.play(); got != game.PhaseCleared {
					t.Fatalf("trial ended %q, want cleared", got)
				}
				v := ansi.Strip(h.m.top().View(100, 30)) // under the trial's flash
				if !strings.Contains(v, tc.line) || !strings.Contains(v, "r retry") {
					t.Fatalf("the passed trial's end offers no %q beside r retry:\n%s", tc.line, v)
				}
				for _, k := range []string{"m", "enter"} {
					h.send(key(k))
					p, ok := h.m.top().(*rtfm.Map)
					if !ok || p.Selected() != tc.page {
						t.Fatalf("%s opened %T, want RTFM at %s", k, h.m.top(), tc.page)
					}
					h.send(screens.Back{})
				}
				passed := h.m.state.Boot
				h.send(key("r"))
				if s := h.m.state; s.Boot == passed || s.Trial == nil || s.Trial.Module != "prefix-sums" {
					t.Errorf("r after a pass: boot %d (was %d), trial %+v; want prefix-sums again", s.Boot, passed, s.Trial)
				}
			})
		})
	}
}

// A trial that grants no capability still clears its level: casework's
// (an ungated topic) and the Bronze conclusion's (a castle) start, and
// passing one records the module and grants nothing.
func TestTrialWithoutCapabilityClearsOnly(t *testing.T) {
	for _, id := range []string{"casework", "bronze-conclusion"} {
		t.Run(id, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newHarness(t)
				loadGuide(h)
				// The synthetic pool judges none of casework's problems; one it
				// does makes casework a trial level, not a page.
				c := h.m.d.Catalog.(*fakeCatalog)
				p := catalog.Problem{ID: "usaco:915", Title: "Sleepy Cow Herding", Topic: "mixed", Rating: 1100, Judgeable: true}
				c.all, c.byID[p.ID] = append(c.all, p), p
				readyTrial(h, id)
				before := len(h.m.meta.TrialsPassed)
				h.send(screens.StartTrial{Module: id})
				if s := h.m.state; s == nil || s.Trial == nil || s.Trial.Module != id || s.Trial.Capability != "" {
					t.Fatalf("no capability-less trial of %s running: %+v", id, s)
				}
				if got := h.play(); got != game.PhaseCleared {
					t.Fatalf("trial ended %q, want cleared", got)
				}
				if !slices.Equal(h.m.meta.TrialsPassed[before:], []string{id}) || len(h.m.meta.Capabilities) != 0 || h.m.state.Trial.Granted != "" {
					t.Fatalf("passed %v, capabilities %v, granted %q; want %s passed and nothing granted",
						h.m.meta.TrialsPassed, h.m.meta.Capabilities, h.m.state.Trial.Granted, id)
				}
			})
		})
	}
}

// Opening a module's page records it in Meta.Opened, once, in the order
// first opened, and saves it: between boots and over one in flight.
func TestOpenModuleRecordsOpened(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		h.m.meta.TrialsPassed = []string{"bronze-conclusion"} // the path is at Prefix Sums
		h.send(screens.OpenModule{Module: "prefix-sums"})
		h.send(screens.Back{})
		h.send(screens.OpenModule{Module: "fast-io"})
		h.send(screens.Back{})
		h.send(screens.OpenModule{Module: "prefix-sums"})
		h.send(screens.Back{})
		want := []string{"prefix-sums", "fast-io"}
		if !slices.Equal(h.m.meta.Opened, want) {
			t.Fatalf("Opened %v, want %v", h.m.meta.Opened, want)
		}
		if m, err := h.st.LoadMeta(); err != nil || !slices.Equal(m.Opened, want) {
			t.Fatalf("saved Opened %v (%v), want %v", m.Opened, err, want)
		}
		h.send(screens.OpenModule{Module: "no-such-module"})
		h.send(screens.StartBoot{Preset: "standard"})
		if h.m.state == nil {
			t.Fatal("no boot in flight")
		}
		h.send(screens.OpenModule{Module: "casework"})
		want = append(want, "casework")
		if m, err := h.st.LoadMeta(); err != nil || !slices.Equal(m.Opened, want) {
			t.Errorf("saved over a boot: Opened %v (%v), want %v", m.Opened, err, want)
		}
	})
}

// The map's i opens the man index over it at the module, and q there comes
// back to the same map.
func TestRTFMIndexOverTheMap(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		h.m.meta.TrialsPassed = []string{"bronze-conclusion"} // the path is at Prefix Sums
		h.send(screens.OpenLearn{Module: "prefix-sums"})
		p, ok := h.m.top().(*rtfm.Map)
		if !ok {
			t.Fatalf("OpenLearn shows %T", h.m.top())
		}
		h.send(screens.OpenIndex{Module: "prefix-sums"})
		b, ok := h.m.top().(*rtfm.Browser)
		if !ok {
			t.Fatalf("OpenIndex shows %T", h.m.top())
		}
		// Its selection is the module: enter opens that page, q closes it.
		h.send(key("enter"))
		if v := ansi.Strip(b.View(100, 30)); !strings.Contains(v, "prefix-sums - ") {
			t.Errorf("enter on the index opens:\n%s", v)
		}
		h.send(key("q"))
		h.send(key("q"))
		if h.m.top() != p {
			t.Errorf("q on the index shows %T, want the map", h.m.top())
		}
	})
}

// RTFM's switch palace plays once per grant: not on the session's first
// map, then for what was granted since the map last showed, then not
// again.
func TestRTFMFlipsEachGrantOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		game.GrantCapability(&h.m.meta, "CAP_DS")
		if got := h.m.freshCaps(); got != nil {
			t.Errorf("the first map flips %v", got)
		}
		game.GrantCapability(&h.m.meta, "CAP_PREFIX")
		if got := h.m.freshCaps(); !slices.Equal(got, []string{"CAP_PREFIX"}) {
			t.Errorf("after a grant the map flips %v, want CAP_PREFIX", got)
		}
		if got := h.m.freshCaps(); got != nil {
			t.Errorf("the next map flips %v again", got)
		}
	})
}

// A locked exit's t opens RTFM at a module with a trial of its own: Bronze's
// Introduction to Data Structures lists no problem, so for The Heap it
// sends the player on to Introduction to Sets & Maps, though the first
// comes first in guide order at the same band.
func TestRTFMOpensAModuleWithATrial(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		dir := t.TempDir()
		mod := func(id, title string) string {
			return "---\nid: " + id + "\ntitle: " + title + "\nauthor: A\ndescription: d\n---\n\nHi.\n"
		}
		for name, body := range map[string]string{
			"content/ordering.ts": `const MODULE_ORDERING = {
  bronze: [{ name: 'Data Structures', items: ['intro-ds', 'intro-sets'] }],
};
`,
			"content/2_Bronze/Intro_DS.mdx":   mod("intro-ds", "Introduction to Data Structures"),
			"content/2_Bronze/Intro_Sets.mdx": mod("intro-sets", "Introduction to Sets & Maps"),
			"content/2_Bronze/Intro_Sets.problems.json": `{"MODULE_ID": "intro-sets", "general": [
  {"uniqueId": "usaco-831", "name": "USACO 831", "url": "u", "source": "Bronze",
   "difficulty": "Easy", "isStarred": false, "tags": [], "solutionMetadata": {"kind": "internal"}}]}`,
		} {
			p := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cur, err := curriculum.Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		h.m.cur = cur
		c := h.m.d.Catalog.(*fakeCatalog)
		p := catalog.Problem{ID: "usaco:831", Title: "USACO 831", Topic: "data-structures", Rating: 1100, Judgeable: true}
		c.all, c.byID[p.ID] = append(c.all, p), p
		h.m.meta.Rating = 1000
		if got := h.m.learnModule("data-structures"); got != "intro-sets" {
			t.Errorf("The Heap's link opens %q, want intro-sets", got)
		}
	})
}

// CSES is the guide's practice, reached by id on RTFM's maps, not a door
// source: neither a boot nor a trial draws one.
func TestCSESDrawsNoDoors(t *testing.T) {
	setup := func(t *testing.T) *harness {
		h := newHarness(t)
		loadGuide(h)
		c := h.m.d.Catalog.(*fakeCatalog)
		for _, id := range []catalog.ID{"cses:1661", "cses:1662", "cses:9999"} {
			p := catalog.Problem{ID: id, Source: catalog.CSES, Title: string(id), Topic: "prefix-sums", Rating: 1400, Judgeable: true}
			c.all, c.byID[id] = append(c.all, p), p
		}
		return h
	}
	cses := func(h *harness) (ids []catalog.ID) {
		for _, p := range h.m.pool {
			if p.Source == catalog.CSES {
				ids = append(ids, p.ID)
			}
		}
		return ids
	}
	synctest.Test(t, func(t *testing.T) {
		h := setup(t)
		h.send(screens.StartBoot{Preset: "standard"})
		if h.m.state == nil {
			t.Fatal("no boot")
		}
		if got := cses(h); len(got) > 0 {
			t.Errorf("a boot's pool holds CSES problems %v", got)
		}
	})
	synctest.Test(t, func(t *testing.T) {
		h := setup(t)
		readyTrial(h, "prefix-sums")
		h.send(screens.StartTrial{Module: "prefix-sums"})
		if h.m.state == nil || h.m.state.Trial == nil {
			t.Fatalf("no trial: %s", h.m.flash)
		}
		if got := cses(h); len(got) > 0 {
			t.Errorf("a trial's pool holds CSES problems %v", got)
		}
	})
}

// The duck's debugging quack (D64) copies the smallest test the last
// submit failed into the door's tests as the next sample, so :w runs it.
func TestDuckCopiesAFailedTest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var r *srcRunner
		h := newHarnessWith(t, func(f *fakeRunner) Runner {
			r = &srcRunner{fakeRunner: f, dir: t.TempDir(), write: "int main() {}\n"}
			return r
		})
		h.send(screens.StartBoot{Preset: "standard"})
		h.m.state.Rules.DuckTest = true // the harness's steady preset predates D64
		h.enterDoor()
		hidden := t.TempDir()
		write := func(name, body string) string {
			p := filepath.Join(hidden, name)
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			return p
		}
		big := problem.Test{Name: "7", In: write("7.in", strings.Repeat("9 ", 500)), Out: write("7.out", "big\n")}
		small := problem.Test{Name: "9", In: write("9.in", "3\n1 2 3\n"), Out: write("9.out", "6\n")}
		h.send(hud.SubmittedMsg{Summary: judge.Summary{Verdict: judge.WA, Passed: 3, Total: 10, Done: 10, First: 6},
			Failed: []problem.Test{big, small}})
		samples := len(h.m.open.Assets.Samples)
		h.send(game.Quack{})
		n := samples + 1
		got, err := os.ReadFile(filepath.Join(filepath.Dir(r.Source()), "tests", fmt.Sprint(n)+".in"))
		if err != nil || string(got) != "3\n1 2 3\n" {
			t.Fatalf("sample %d is %q (%v), want the smallest failed test; flash %q", n, got, err, h.m.flash)
		}
		if len(h.m.open.Assets.Samples) != n || !strings.Contains(h.m.flash, "hidden test #9") {
			t.Errorf("samples %d, flash %q", len(h.m.open.Assets.Samples), h.m.flash)
		}
	})
}

// A trial waits on the path and on its map: one the path has not reached
// is locked until the trial before it is passed, and one reached opens
// once its map's core is solved. Each refusal says what is left.
func TestTrialWaitsOnThePath(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		started := func() bool { return h.m.state != nil && h.m.state.Trial != nil }
		h.send(screens.StartTrial{Module: "prefix-sums"})
		if started() || !strings.Contains(h.m.flash, "is locked: pass Additional Practice for USACO Bronze first") {
			t.Fatalf("before Bronze's castle: trial %v, flash %q", started(), h.m.flash)
		}
		h.m.meta.TrialsPassed = append(h.m.meta.TrialsPassed, "bronze-conclusion")
		h.send(screens.StartTrial{Module: "prefix-sums"})
		if started() || !strings.Contains(h.m.flash, "trial opens once its core is solved: 0 of") {
			t.Fatalf("with the core unsolved: trial %v, flash %q", started(), h.m.flash)
		}
		readyTrial(h, "prefix-sums")
		h.send(screens.StartTrial{Module: "prefix-sums"})
		if !started() {
			t.Fatalf("with the core solved, no trial: %q", h.m.flash)
		}
	})
}

// A module's page waits on the path as its trial does: no reader, nothing
// recorded opened, and the flash says what to pass.
func TestReadWaitsOnThePath(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		h.send(screens.OpenModule{Module: "prefix-sums"})
		if len(h.m.meta.Opened) > 0 || !strings.Contains(h.m.flash, "is locked: pass Additional Practice for USACO Bronze first") {
			t.Fatalf("before Bronze's castle: opened %v, flash %q", h.m.meta.Opened, h.m.flash)
		}
		h.m.meta.TrialsPassed = append(h.m.meta.TrialsPassed, "bronze-conclusion")
		h.send(screens.OpenModule{Module: "prefix-sums"})
		if !slices.Equal(h.m.meta.Opened, []string{"prefix-sums"}) {
			t.Fatalf("once the path is there: opened %v, flash %q", h.m.meta.Opened, h.m.flash)
		}
	})
}

// RTFM and the doors share no problem: neither a boot's pool nor the
// trials' holds one the guide lists, and no problem is in both.
func TestPoolsApart(t *testing.T) {
	c := newFakeCatalog()
	n := 0
	for id := range curriculum.Listed() {
		if n++; n > 200 {
			break
		}
		p := catalog.Problem{ID: id, Topic: "graphs", Rating: 1500, Judgeable: true}
		c.all, c.byID[id] = append(c.all, p), p
	}
	trials := poolFor(c, &game.State{Trial: &game.TrialState{}})
	boots := poolFor(c, nil)
	if len(trials) == 0 || len(boots) == 0 {
		t.Fatalf("%d trial problems, %d boot ones", len(trials), len(boots))
	}
	held := map[catalog.ID]bool{}
	for _, p := range trials {
		held[p.ID] = true
		if curriculum.Listed()[p.ID] {
			t.Errorf("the trials draw %s, on RTFM's maps", p.ID)
		}
	}
	for _, p := range boots {
		if curriculum.Listed()[p.ID] || held[p.ID] {
			t.Errorf("boots draw %s, listed %v or a trial's %v", p.ID, curriculum.Listed()[p.ID], held[p.ID])
		}
	}
}
