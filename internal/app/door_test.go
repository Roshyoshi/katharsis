package app

import (
	"strings"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/judge"
	"katharsis/internal/rules"
	"katharsis/internal/screens"
	bootui "katharsis/internal/screens/boot"
	"katharsis/internal/session"
)

// The in-door half of cycles (D47-D54): the rules a door plays by, as the
// pane, the HUD and vim see them.

// cyclesBoot starts a boot on steady rules with r's changes applied and
// plays it to its first open door.
func cyclesBoot(t *testing.T, change func(*rules.Rules)) *harness {
	t.Helper()
	h := newHarness(t)
	r := steady()
	r.Name = "cycles"
	change(&r)
	h.m.d.Presets["cycles"] = r
	h.send(screens.StartBoot{Preset: "cycles"})
	h.enterDoor()
	return h
}

// pane is the open door's problem screen.
func (h *harness) pane() *problemScreen {
	h.t.Helper()
	for _, s := range h.m.stack {
		if p, ok := s.(*problemScreen); ok {
			return p
		}
	}
	h.t.Fatal("no problem screen")
	return nil
}

// TestTwistJudging: each boss's twist bends the HUD's judging the way its
// door says (D54): the OOM Killer's memory limit, on samples without ASan
// too, Undefined Behavior's samples without sanitizers, Heisenbug's
// pass-or-fail samples. A plain door and an upsolve judge as ever.
func TestTwistJudging(t *testing.T) {
	tc := judge.Toolchain{Samples: judge.Samples, Full: judge.Full, Debug: judge.Debug, Hardened: judge.HardenedSamples}
	door := func(d game.Door, mem int64) Door {
		return Door{Door: d, Problem: catalog.Problem{TimeLimit: time.Second, MemLimit: mem}}
	}
	prof, lim, quiet := judging(tc, door(game.Door{}, 256<<20), 0.8)
	if prof.Name != judge.Samples.Name || lim.Mem != 256<<20 || lim.Time != time.Second || lim.Scale != 0.8 || quiet {
		t.Errorf("plain door: %s, %+v, quiet %v", prof.Name, lim, quiet)
	}
	if prof, lim, _ := judging(tc, door(game.Door{Twist: "oom-killer", MemMult: 0.5}, 256<<20), 1); lim.Mem != 128<<20 || prof.Name != tc.Hardened.Name {
		t.Errorf("OOM Killer: memory %d on %q, want half of 256 MiB on %q", lim.Mem, prof.Name, tc.Hardened.Name)
	}
	if _, lim, _ := judging(tc, door(game.Door{Twist: "oom-killer", MemMult: 0.5}, 0), 1); lim.Mem != 128<<20 {
		t.Errorf("OOM Killer on a problem with no limit: %d, want half of the default", lim.Mem)
	}
	if prof, _, _ := judging(tc, door(game.Door{Twist: "undefined-behavior", NoSanitizers: true}, 0), 1); prof.Name != judge.Full.Name || prof.Sanitized {
		t.Errorf("Undefined Behavior: samples on %q", prof.Name)
	}
	if _, _, quiet := judging(tc, door(game.Door{Twist: "heisenbug", QuietSamples: true}, 0), 1); !quiet {
		t.Error("Heisenbug: samples not quiet")
	}
	up := door(game.Door{MemMult: 0.5, NoSanitizers: true, QuietSamples: true}, 256<<20)
	up.Upsolve = true
	if prof, lim, quiet := judging(tc, up, 1); prof.Name != judge.Samples.Name || lim.Mem != 256<<20 || quiet {
		t.Errorf("an upsolve kept a twist: %s, %d, %v", prof.Name, lim.Mem, quiet)
	}
}

// TestHiddenClock: The Halting Problem hides its countdown in the HUD
// bar, the editor's border and vim's message line (D54), and the 5:00
// warning still comes, in the strip and on the border.
func TestHiddenClock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := cyclesBoot(t, func(*rules.Rules) {})
		c := h.m.state.Current
		c.Door.HiddenClock = true
		left := c.Deadline.Sub(h.clock.t)
		count := countdown(left) // "15:00"
		for where, s := range map[string]string{
			"clock": h.m.Clock(), "border": h.m.BorderClock(), "status": h.m.statusLine(),
			"pane": ansi.Strip(h.pane().View(48, 30)),
		} {
			if strings.Contains(s, count) {
				t.Errorf("%s shows the countdown %s: %q", where, count, s)
			}
			if !strings.Contains(s, "??:??") && !strings.Contains(s, "clock hidden") {
				t.Errorf("%s does not say the clock is hidden: %q", where, s)
			}
		}
		h.clock.t = c.Deadline.Add(-5*time.Minute + time.Second)
		h.send(tickMsg(h.clock.t))
		if b := h.m.BorderClock(); !strings.Contains(b, "05:00 left") {
			t.Errorf("no 5:00 warning on the border: %q", b)
		}
		if v := ansi.Strip(h.pane().View(48, 30)); !strings.Contains(v, "left on this door") {
			t.Errorf("no 5:00 warning in the pane's strip:\n%s", v)
		}
	})
}

// TestRetreatLock: R is offered, with when it shuts, until the door's
// lock (rules.RetreatLock, 2:30 in); then the keys row says locked, R and
// :Retreat are refused with the strip's door.retreat.locked, and the door
// stays.
func TestRetreatLock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := cyclesBoot(t, func(r *rules.Rules) { r.RetreatLock = rules.Duration{Duration: 150 * time.Second} })
		h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
		door := h.m.state.Current.Door.Problem
		if last := h.lastLine(); !strings.Contains(last, "R retreat") {
			t.Fatalf("keys row before the lock: %q", last)
		}
		h.send(key("R"))
		if last := h.lastLine(); !strings.Contains(last, "retreat: burns 1 cycle") || !strings.Contains(last, "shuts in 02:30") {
			t.Fatalf("R before the lock asks %q", last)
		}
		h.send(key("x")) // let it go
		h.clock.t = h.m.state.Current.Opened.Add(150 * time.Second)
		h.send(tickMsg(h.clock.t))
		if last := h.lastLine(); !strings.Contains(last, "R locked") {
			t.Errorf("keys row past the lock: %q", last)
		}
		h.send(key("R"))
		h.send(key("R"))
		if c := h.m.state.Current; c == nil || c.Door.Problem != door {
			t.Fatalf("R R past the lock retreated: %+v", c)
		}
		if v := view(h); !strings.Contains(v, "retreat: locked 02:30 in") {
			t.Errorf("no door.retreat.locked line in the pane:\n%s", v)
		}
		req, reply := session.NewRequest(session.Command{Cmd: session.CmdRetreat})
		h.send(req)
		if rep, _ := answer(reply); rep.OK || !strings.Contains(rep.Text, "retreat locked 02:30 in") {
			t.Errorf(":Retreat past the lock answered %+v", rep)
		}
		if h.m.state.Current == nil || h.m.state.Current.Door.Problem != door {
			t.Fatal(":Retreat past the lock retreated")
		}
	})
}

// TestSubmitReplyCosts: vim's :Submit reply carries what the verdict cost
// in the boot's terms: the cycles burned, ECC's clock instead (a hidden
// clock keeps its time left to itself) or why ECC did not pay, and the
// panic when it ended the boot; nothing when it cost nothing.
func TestSubmitReplyCosts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := cyclesBoot(t, func(r *rules.Rules) { r.ECCPenalty = rules.Duration{Duration: 5 * time.Minute} })
		s := h.m.state
		s.Cycles = 24
		before := h.m.doorLives()
		s.Cycles = 22
		if got := h.m.charged(before); got != "burned 2 cycles, 22 left" {
			t.Errorf("a burn: %q", got)
		}
		s.Cycles = 24
		s.Current.Deadline = s.Current.Deadline.Add(-5 * time.Minute)
		left := countdown(s.Current.Deadline.Sub(h.clock.t))
		if got, want := h.m.charged(before), "ECC: -05:00 on the clock, "+left+" left"; got != want {
			t.Errorf("ECC: %q, want %q", got, want)
		}
		s.Current.Door.HiddenClock = true
		if got := h.m.charged(before); got != "ECC: -05:00 on the clock" {
			t.Errorf("ECC on a hidden clock: %q", got)
		}
		s.Current.Deadline = before.deadline
		if got := h.m.charged(before); got != "" {
			t.Errorf("nothing charged: %q", got)
		}
		// The real engine under ECC with 04:00 left, too short to pay its
		// 5:00 in: the WA burns cycles, the door stays open, and the reply
		// says why.
		s.Current.Door.HiddenClock = false
		s.Upgrades = append(s.Upgrades, game.ECC)
		s.Current.Deadline = h.clock.t.Add(4 * time.Minute)
		before = h.m.doorLives()
		h.verdict = "WA"
		h.send(submitMsg{})
		got := h.m.charged(before)
		if s := h.m.state; s.Phase != game.PhaseProblem || s.Cycles >= before.cycles || !s.Current.Deadline.Equal(before.deadline) ||
			got != bootui.Burned(before.cycles-s.Cycles, s.Cycles)+" (ECC: clock too short)" {
			t.Errorf("ECC, the clock too short: phase %s, cycles %d → %d, %q", s.Phase, before.cycles, s.Cycles, got)
		}
		// The real engine's burn, to the end of the boot.
		h.m.state.Cycles = 1
		before = h.m.doorLives()
		h.send(submitMsg{})
		if got := h.m.charged(before); h.m.state.Phase != game.PhaseCrashed || got != "burned 1 cycle, 0 left (ECC: clock too short) · kernel panic" {
			t.Errorf("the last cycle: phase %s, %q", h.m.state.Phase, got)
		}
	})
}

// TestDoorQuitLine: ctrl+c inside a door does not quit it (D47); the
// pane says why and what each way out costs, the Rabbit's line over the
// rules, until the next key.
func TestDoorQuitLine(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := cyclesBoot(t, func(*rules.Rules) {})
		h.m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
		v := ansi.Strip(h.pane().View(48, 30))
		for _, want := range []string{"won't stop his watch", "no quitting inside a door", "R retreat −1", ":q! ends the boot", "does not stop the clock"} {
			if !strings.Contains(strings.Join(strings.Fields(v), " "), want) {
				t.Errorf("refusal missing %q:\n%s", want, v)
			}
		}
		for i, l := range strings.Split(v, "\n") {
			if w := ansi.StringWidth(l); w > 48 {
				t.Errorf("line %d is %d wide: %q", i, w, l)
			}
		}
		h.m.Update(key("j"))
		if v := ansi.Strip(h.pane().View(48, 30)); strings.Contains(v, "no quitting") {
			t.Errorf("the refusal outlived the next key:\n%s", v)
		}
	})
}

// countdown is a countdown as the reply prints it.
func countdown(d time.Duration) string { return strings.TrimSuffix(leftText(d), " left") }

// TestGiveUpAsks: vim's :q! asks the game what giving up costs now (ctl
// giveup ask), and asking gives nothing up: the room's burn while the
// boot can pay it, the end of the boot once it cannot.
func TestGiveUpAsks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := cyclesBoot(t, func(r *rules.Rules) {
			r.Cycles, r.MaxCycles, r.FailMoveOn = 30, 50, true
			r.Biomes[0].FailCycles = 10
		})
		ask := func() string {
			req, reply := session.NewRequest(session.Command{Cmd: session.CmdGiveUp, Args: []string{session.ArgAsk}})
			h.send(req)
			rep, _ := answer(reply)
			if !rep.OK {
				t.Fatalf("giveup ask answered %+v", rep)
			}
			return rep.Text
		}
		if got := ask(); got != "Give up? :q! burns 10 cycles, 20 left · room lost." {
			t.Errorf("with cycles to spare: %q", got)
		}
		if h.m.state.Phase != game.PhaseProblem || h.m.state.Current == nil {
			t.Fatal("asking gave the door up")
		}
		h.m.state.Cycles = 10
		if got := ask(); got != "Give up? :q! ends the boot." {
			t.Errorf("on the last cycles: %q", got)
		}
	})
}
