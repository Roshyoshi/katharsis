package app

import (
	"context"
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
	h.m.cur = cur
	return cur
}

func TestTrialGrantsItsCapability(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		cur := loadGuide(h)
		mod, ok := cur.Module("prefix-sums")
		if !ok {
			t.Fatal("fixture guide has no prefix-sums module")
		}
		want := curriculum.TrialFor(mod, h.m.judgeable).Capability
		if want == "" {
			t.Fatal("prefix-sums grants no capability")
		}

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
	})
}

// A trial's timer is soft: its door resumed past the deadline goes on in
// overtime, so it reopens; it used to wait, editorless, for an e.
func TestResumedTrialInOvertimeReopens(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		h.send(screens.StartTrial{Module: "prefix-sums"})
		h.clock.t = h.enterDoor().Deadline.Add(time.Minute)
		h.restart()
		if h.m.state.Phase != game.PhaseProblem || h.m.open == nil {
			t.Fatalf("phase %q open %v after resuming a trial door in overtime, want it open", h.m.state.Phase, h.m.open != nil)
		}
	})
}

// r on a trial's end screen runs the same trial again: the trial rules
// are no boot preset, and the reboot fell back to a standard boot.
func TestRebootAfterATrialRetriesIt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		loadGuide(h)
		h.send(screens.StartTrial{Module: "prefix-sums"})
		h.verdict = "WA"
		if got := h.play(); got != game.PhaseCrashed || h.m.state.Trial == nil {
			t.Fatalf("trial ended %q (trial %v), want crashed", got, h.m.state.Trial != nil)
		}
		if v := ansi.Strip(h.m.View().Content); !strings.Contains(v, "r retry the prefix-sums trial") {
			t.Fatalf("the end screen does not say r retries the trial:\n%s", v)
		}
		first := h.m.state.Boot
		h.m.cur = nil // the curriculum is not needed again
		h.send(key("r"))
		if s := h.m.state; s.Boot == first || s.Trial == nil || s.Trial.Module != "prefix-sums" || s.Phase == game.PhaseCrashed {
			t.Fatalf("after r: boot %d (was %d), trial %+v, phase %s; want prefix-sums again", s.Boot, first, s.Trial, s.Phase)
		}
	})
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
		name string
		held []string
		line string // the next step
		page string // the module m opens at
	}{
		{"next module", nil, "m next module: Two Pointers", "two-pointers"},
		{"none left", []string{"CAP_TWOPTR", "CAP_DS"}, "m back to RTFM", "prefix-sums"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newHarness(t)
				loadGuide(h)
				for _, c := range tc.held {
					game.GrantCapability(&h.m.meta, c)
				}
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
				h.send(screens.StartTrial{Module: id})
				if s := h.m.state; s == nil || s.Trial == nil || s.Trial.Module != id || s.Trial.Capability != "" {
					t.Fatalf("no capability-less trial of %s running: %+v", id, s)
				}
				if got := h.play(); got != game.PhaseCleared {
					t.Fatalf("trial ended %q, want cleared", got)
				}
				if !slices.Equal(h.m.meta.TrialsPassed, []string{id}) || len(h.m.meta.Capabilities) != 0 || h.m.state.Trial.Granted != "" {
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
