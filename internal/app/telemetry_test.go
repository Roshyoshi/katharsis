package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/hud"
	"katharsis/internal/screens"
	"katharsis/internal/store"
)

// editorialCatalog serves an editorial beside each statement.
type editorialCatalog struct{ *fakeCatalog }

func (c editorialCatalog) Fetch(ctx context.Context, id catalog.ID) (catalog.Assets, error) {
	a, err := c.fakeCatalog.Fetch(ctx, id)
	if err != nil {
		return a, err
	}
	a.Editorial = filepath.Join(c.docs, "editorial.md")
	return a, os.WriteFile(a.Editorial, []byte("# Editorial\n\nSort, then sweep.\n"), 0o644)
}

// syncBuffer is a log sink the recorder's goroutine writes and the test
// reads.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func (h *harness) events() []store.Event {
	h.t.Helper()
	if !h.m.tele.rec.flush(5 * time.Second) {
		h.t.Fatal("events did not flush")
	}
	evs, err := h.st.Events(store.EventFilter{})
	if err != nil {
		h.t.Fatal(err)
	}
	return evs
}

func payload(t *testing.T, e store.Event) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(e.Data.(json.RawMessage), &v); err != nil {
		t.Fatalf("%s payload %s: %v", e.Kind, e.Data, err)
	}
	return v
}

// TestEventsFromADoorSession plays a scripted session, a door saved and
// given up, its editorial read, an upsolve failed then solved, and finds
// each event in the store, in order, under one session.
func TestEventsFromADoorSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.m.d.Catalog = editorialCatalog{h.m.d.Catalog.(*fakeCatalog)}
		h.send(screens.StartBoot{Preset: "standard"})
		cur := h.enterDoor()
		door, idx, boot := cur.Door.Problem, len(h.m.state.History), h.m.state.Boot
		// What the HUD sends when a :w's sample pass ends.
		h.send(hud.SamplesMsg{Compiled: true, Passed: 2, Total: 3, Wall: 1500 * time.Millisecond})
		h.send(game.GiveUp{})
		if h.m.state.Phase != game.PhaseCrashed {
			t.Fatalf("phase %q after :q!, want crashed", h.m.state.Phase)
		}
		h.clock.t = h.clock.t.Add(time.Minute)
		h.send(screens.OpenEditorial{Problem: door})
		h.clock.t = h.clock.t.Add(2 * time.Minute)
		h.send(screens.Back{})
		h.send(screens.Upsolve{Problem: door})
		h.verdict = "WA"
		h.send(submitMsg{})
		h.clock.t = h.clock.t.Add(10 * time.Minute)
		h.verdict = "AC"
		h.send(submitMsg{})
		if h.m.open != nil {
			t.Fatal("upsolve still open after AC")
		}
		h.send(screens.Quit{})

		evs, err := h.st.Events(store.EventFilter{}) // quit drained the queue
		if err != nil {
			t.Fatal(err)
		}
		var kinds []string
		for _, e := range evs {
			k := e.Kind
			if k == store.EventScreen {
				k += ":" + payload(t, e)["screen"].(string)
			}
			kinds = append(kinds, k)
			if e.Session != evs[0].ID {
				t.Errorf("%s in session %d, want %d", e.Kind, e.Session, evs[0].ID)
			}
		}
		got := strings.Join(kinds, " ")
		for _, want := range []string{
			"app_start screen:grub screen:",
			"screen:doors",
			"screen:problem samples_judged screen:crashed boot_end_action editorial_open editorial_close " +
				"boot_end_action upsolve_open upsolve_judged upsolve_judged upsolve_close app_quit", // debrief.go logs the end screen's choices
		} {
			if !strings.Contains(got, want) {
				t.Errorf("events\n  %s\nlack the run\n  %s", got, want)
			}
		}

		byKind := map[string][]store.Event{}
		for _, e := range evs {
			byKind[e.Kind] = append(byKind[e.Kind], e)
		}
		s := byKind[store.EventSamplesJudged][0]
		if s.Boot != boot || s.Attempt == nil || *s.Attempt != idx || s.Problem != door {
			t.Errorf("samples_judged at boot %d attempt %v problem %s, want %d %d %s",
				s.Boot, s.Attempt, s.Problem, boot, idx, door)
		}
		if p := payload(t, s); p["passed"] != 2.0 || p["total"] != 3.0 || p["wall_ms"] != 1500.0 || p["compiled"] != true {
			t.Errorf("samples_judged payload %v", p)
		}
		if p := payload(t, byKind[store.EventEditorialClose][0]); p["dwell_ms"] != float64(2*time.Minute/time.Millisecond) || p["in_boot"] != false {
			t.Errorf("editorial_close payload %v", p)
		}
		if e := byKind[store.EventEditorialOpen][0]; e.Problem != door || e.Attempt != nil || e.Boot != boot {
			t.Errorf("editorial_open %+v", e)
		}
		judged := byKind[store.EventUpsolveJudged]
		if p := payload(t, judged[0]); p["verdict"] != "WA" || p["editorial"] != true {
			t.Errorf("first upsolve_judged %v, want WA with the editorial seen", p)
		}
		if p := payload(t, byKind[store.EventUpsolveClose][0]); p["verdict"] != "AC" || p["elapsed_ms"] != float64(10*time.Minute/time.Millisecond) {
			t.Errorf("upsolve_close payload %v", p)
		}
		// The harness's preset set grows with other tests' needs; the payload
		// must list every one of them, standard included.
		if p := payload(t, evs[0]); p["tmux"] != false || !slices.Contains(p["presets"].([]any), any("standard")) {
			t.Errorf("app_start payload %v", p)
		}
	})
}

// An install is an event too (retention RT3 times the apt ladder by it).
func TestEventAptInstall(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newHarness(t)
		h.m.meta.Bank = 1 << 20
		pkgs := h.m.packages()
		h.send(screens.Install{Package: pkgs[0].Name})
		evs := h.events()
		e := evs[len(evs)-1]
		if p := payload(t, e); e.Kind != store.EventAptInstall || p["package"] != pkgs[0].Name ||
			p["price"] != float64(pkgs[0].Price) {
			t.Errorf("last event %s %v, want the install", e.Kind, p)
		}
	})
}

// Events never hold up the UI: with another process holding the store's
// write lock, emitting a queue's worth and more returns at once, and the
// overflow is logged. A table the store cannot write is logged too, and
// the game's own saves go on.
func TestEventsNeverBlockOrCostASave(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		logs := &syncBuffer{}
		h := newHarness(t)
		h.events() // the recorder is idle, so its logger can change
		h.m.tele.log = log.New(logs, "", 0)
		h.m.tele.rec.log = h.m.tele.log
		db, err := sql.Open("sqlite", "file:"+h.path+"?_pragma=busy_timeout(5000)")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		conn, err := db.Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.ExecContext(context.Background(), `BEGIN IMMEDIATE`); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		for range eventQueue + 50 {
			h.m.event(store.EventScreen, "", kv{"screen": "spam"})
		}
		if d := time.Since(start); d > 500*time.Millisecond {
			t.Errorf("emitting with the store locked took %v", d)
		}
		if _, err := conn.ExecContext(context.Background(), `COMMIT`); err != nil {
			t.Fatal(err)
		}
		h.events()
		if !strings.Contains(logs.String(), "dropped") {
			t.Errorf("no drop logged; log:\n%s", logs)
		}

		if _, err := conn.ExecContext(context.Background(), `DROP TABLE events`); err != nil {
			t.Fatal(err)
		}
		h.send(screens.StartBoot{Preset: "standard"})
		h.enterDoor()
		if !h.m.tele.rec.flush(5 * time.Second) {
			t.Fatal("events did not flush")
		}
		if !strings.Contains(logs.String(), "no such table") {
			t.Errorf("failed event write not logged; log:\n%s", logs)
		}
		if h.m.saveErr != nil {
			t.Fatalf("save failed beside a failed event: %v", h.m.saveErr)
		}
		st, err := h.st.ActiveBoot()
		if err != nil || st == nil || st.Phase != game.PhaseProblem {
			t.Fatalf("saved boot %+v, %v; want the open door saved", st, err)
		}
	})
}
