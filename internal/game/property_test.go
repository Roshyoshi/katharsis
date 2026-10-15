package game_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
)

// next lists the phases each phase may move to in one Step. Every phase
// but the end can crash: Abandon ends a suspended boot.
var next = map[game.Phase][]game.Phase{
	game.PhaseRoute:   {game.PhaseRoute, game.PhaseDoors, game.PhaseCrashed},
	game.PhaseDoors:   {game.PhaseDoors, game.PhaseProblem, game.PhaseCrashed},
	game.PhaseProblem: {game.PhaseProblem, game.PhaseDoors, game.PhaseMerge, game.PhaseCleared, game.PhaseCrashed},
	game.PhaseMerge:   {game.PhaseMerge, game.PhaseRoute, game.PhaseDoors, game.PhaseCrashed},
	game.PhaseCrashed: {game.PhaseCrashed},
	game.PhaseCleared: {game.PhaseCleared},
}

// checker holds what invariants compare against across one boot: Meta as
// it was when the boot started (nothing outside the boot touches it until
// the boot ends), and the previous step's phase and lives.
type checker struct {
	t         testing.TB
	bank0     int64
	lifetime0 int64
	rated0    int
	solves0   int
	dump0     string // meta.CoreDump as JSON
	dumpLost  int64  // what a crash must destroy: the dump's bytes, if any
	phase     game.Phase
	lives     int
	fountain  int // State.Fountained at the last step
	spent     int64
	installed []string // meta.Installed when the boot began
	theme     string
	prestige0 int           // meta.Prestige when the boot began
	gov0      game.Governor // meta.Governor when the boot began
	ended     bool          // the boot's end was checked
	merged    bool          // a merge window was reached (the early-crash count starts over)
}

// newChecker is called right after NewBoot or NewTrial, before the first
// Step: no door has closed, so Meta's bank, bytes, ratings and solves are
// still what the boot started from.
func newChecker(t testing.TB, s *game.State, m *game.Meta) *checker {
	c := &checker{
		t: t, bank0: m.Bank, lifetime0: m.LifetimeBytes, rated0: m.Rated, solves0: len(m.Solves),
		dump0: mustJSON(t, m.CoreDump), phase: s.Phase, lives: s.Cycles,
		installed: slices.Clone(m.Installed), theme: m.Theme,
		prestige0: m.Prestige,
	}
	if m.CoreDump != nil {
		c.dumpLost = m.CoreDump.Bytes
	}
	c.gov0 = m.Governor
	c.start(s, m)
	return c
}

// start checks a boot as NewBoot dealt it: its lives are the rules' (and
// the ascensions') plus what each easing added, and it has the easings
// Meta and the forecast call for.
func (c *checker) start(s *game.State, m *game.Meta) {
	t := c.t
	t.Helper()
	r := s.Rules
	if s.Trial != nil {
		if len(s.Ease) != 0 || s.Forecast != 0 {
			t.Fatalf("trial with easing %+v, forecast %v", s.Ease, s.Forecast)
		}
		return
	}
	lives, kinds := max(1, r.Cycles+r.Ascend(s.Prestige).Cycles), map[game.EaseKind]bool{}
	ducks := max(0, r.Ducks+r.Ascend(s.Prestige).Ducks) + r.Grants(s.Packages).Ducks
	for _, es := range s.Ease {
		if kinds[es.Kind] || es.Cycles < 0 || es.Ducks < 0 || es.Why == "" {
			t.Fatalf("easing %+v in %+v", es, s.Ease)
		}
		kinds[es.Kind] = true
		lives += es.Cycles
		ducks += es.Ducks
		for _, bad := range []string{"easy", "assist", "mercy"} {
			if strings.Contains(strings.ToLower(es.Why), bad) {
				t.Fatalf("easing %+v says %q (retention GR9)", es, bad)
			}
		}
	}
	if s.Cycles != lives || s.Ducks != ducks || s.Declined {
		t.Fatalf("boot starts with %d lives, %d ducks (declined %v), want %d and %d from the rules and %+v",
			s.Cycles, s.Ducks, s.Declined, lives, ducks, s.Ease)
	}
	// Boot #1 is the first boot that is not a trial (Meta.Trials).
	first := s.Boot == m.Trials+1
	if eased := first && (len(r.FirstBiomes) > 0 || r.FirstDoor || r.FirstEditorial); eased != kinds[game.EaseFirst] ||
		eased && kinds[game.EaseComeback] {
		t.Fatalf("boot %d (%d trials before) with easings %+v", s.Boot, m.Trials, s.Ease)
	}
	if first && len(r.FirstBiomes) > 0 && mustJSON(t, r.Biomes) != mustJSON(t, r.FirstBiomes) {
		t.Fatalf("boot #1 plays %+v, not its first biomes", r.Biomes)
	}
	if gov := r.GovernorAfter > 0 && m.Governor.EarlyCrashes >= r.GovernorAfter; gov != kinds[game.EaseGovernor] {
		t.Fatalf("governor eased %v after %d early crashes (rules %d)", kinds[game.EaseGovernor], m.Governor.EarlyCrashes, r.GovernorAfter)
	}
	if s.Forecast < 0 || s.Forecast > 1 || kinds[game.EaseFloor] && s.Forecast <= 1-math.Pow(s.AimClear(0)-r.BuybackMargin, float64(s.Biomes[0].Rooms))-1e-9 {
		t.Fatalf("forecast %v with easings %+v", s.Forecast, s.Ease)
	}
}

// active is whether s runs an easing of kind k (not declined), and it.
func active(s *game.State, k game.EaseKind) (game.Ease, bool) {
	if s.Declined {
		return game.Ease{}, false
	}
	i := slices.IndexFunc(s.Ease, func(es game.Ease) bool { return es.Kind == k })
	if i < 0 {
		return game.Ease{}, false
	}
	return s.Ease[i], true
}

// check asserts the invariants that hold after every Step.
func (c *checker) check(s *game.State, m *game.Meta, a game.Action, err error) {
	t := c.t
	t.Helper()
	fail := func(format string, args ...any) {
		t.Helper()
		t.Fatalf("boot %d after %T%+v (phase %s): %s", s.Boot, a, a, s.Phase, fmt.Sprintf(format, args...))
	}
	if err != nil && !errors.Is(err, game.ErrInvalid) {
		fail("error %v is not ErrInvalid", err)
	}
	if !slices.Contains(next[c.phase], s.Phase) {
		fail("%s -> %s is not a transition", c.phase, s.Phase)
	}
	c.phase = s.Phase
	c.merged = c.merged || s.Phase == game.PhaseMerge
	if s.Cycles < 0 || s.Cycles > s.MaxCycles || s.Ducks < 0 {
		fail("lives %d/%d ducks %d", s.Cycles, s.MaxCycles, s.Ducks)
	}
	if s.Cycles == 0 && s.Phase != game.PhaseCrashed {
		fail("no lives but not crashed")
	}
	// Cycles come back only by an ECC pick that heals, an fsck, or a merge
	// window's fountain, each by at most its heal (a cap can make it less);
	// nothing else raises them (D47: no heal on AC).
	gain := s.Cycles - c.lives
	fountain := s.Fountained - c.fountain
	switch {
	case gain <= 0:
	case a == game.PickUpgrade{Upgrade: game.ECC} && s.Rules.ECCPenalty.Duration <= 0 && gain <= s.ECCHeal():
	case a == game.Spend{Item: game.Fsck} && gain <= s.FsckHeal():
	case fountain > 0 && gain == fountain && s.Phase == game.PhaseMerge && s.Cycles <= s.StartCycles():
	default:
		fail("cycles %d -> %d (fountain %d)", c.lives, s.Cycles, fountain)
	}
	if fountain < 0 || fountain > 0 && s.Fountain != fountain {
		fail("fountain %d this window, %d -> %d in all", s.Fountain, c.fountain, s.Fountained)
	}
	c.lives, c.fountain = s.Cycles, s.Fountained
	if s.UpgradePicks < 0 || s.PendingUpgrade != (s.UpgradePicks > 0) || s.PendingUpgrade != (len(s.UpgradeOffers) > 0) {
		fail("picks %d pending %v offers %v", s.UpgradePicks, s.PendingUpgrade, s.UpgradeOffers)
	}
	if (s.Current != nil) != (s.Phase == game.PhaseProblem) {
		fail("current %v in phase %s", s.Current != nil, s.Phase)
	}
	if (s.Crash != nil) != (s.Phase == game.PhaseCrashed) {
		fail("crash %v in phase %s", s.Crash != nil, s.Phase)
	}
	if s.Phase == game.PhaseDoors {
		if len(s.Doors) == 0 {
			fail("no doors")
		}
		seen := map[catalog.ID]bool{}
		for _, d := range s.Doors {
			if seen[d.Problem] {
				fail("door %s twice in a room", d.Problem)
			}
			seen[d.Problem] = true
			if d.Bytes < 1 || d.Timer < time.Minute || d.SolveP <= 0 || d.SolveP >= 1 {
				fail("door %+v", d)
			}
			if d.Boss != "" && d.Cursed || d.Mystery && !d.Blind {
				fail("door modifiers %+v", d)
			}
			if d.Mastered && (d.Blind || s.Trial != nil) {
				fail("mastered door %+v", d)
			}
			// Effort pays the same at every difficulty (retention GR5): a
			// door without risk modifiers is expected to pay BaseBytes/2,
			// times only what multiplies every such door alike (a boss,
			// chrt, nice, cron on a regression), wherever the difficulty
			// factor is not clamped. Rounding a door's bytes moves it by at
			// most half a byte.
			if !d.Blind && !d.Cursed && 0.5/d.SolveP <= s.Rules.MaxBytesMult && 0.5/d.SolveP >= s.Rules.MinBytesMult {
				flat := 1.0
				if d.Boss != "" {
					flat *= s.Rules.BossMult
				}
				if slices.Contains(s.Daemons, game.Chrt) {
					flat *= s.Rules.ChrtBytesMult
				}
				if slices.Contains(s.Daemons, game.Nice) {
					flat *= 1 - s.Rules.NiceBytesFrac
				}
				if d.Regression && slices.Contains(s.Daemons, game.Cron) {
					flat *= s.Rules.CronMult
				}
				if want := float64(s.Rules.BaseBytes) / 2 * flat; math.Abs(d.SolveP*float64(d.Bytes)-want) > 0.5 {
					fail("door %+v is expected to pay %.1f B, want %.1f", d, d.SolveP*float64(d.Bytes), want)
				}
			}
		}
	} else if len(s.Doors) != 0 {
		fail("%d doors outside the doors phase", len(s.Doors))
	}
	if s.Phase != game.PhaseMerge && len(s.DaemonOffers) != 0 {
		fail("daemons %v on offer outside a merge window", s.DaemonOffers)
	}
	// A merge window or a clear sits past the last room of the biome it ends.
	// An abandoned boot stopped wherever it was suspended, a merge window
	// included.
	done := 0
	abandoned := s.Crash != nil && s.Crash.Cause == game.Abandoned
	if s.Phase == game.PhaseMerge || s.Phase == game.PhaseCleared || abandoned {
		done = 1
	}
	if s.Biome >= len(s.Biomes) || s.Room >= s.Biomes[s.Biome].Rooms+done || done == 1 && !abandoned && s.Room != s.Biomes[s.Biome].Rooms {
		fail("biome %d room %d out of the layout", s.Biome, s.Room)
	}
	if len(s.Daemons) > s.Rules.MaxDaemons || len(slices.Compact(slices.Sorted(slices.Values(s.Daemons)))) != len(s.Daemons) {
		fail("daemons %v", s.Daemons)
	}
	// A boot plays with the packages installed when it began, and nothing
	// in it installs any: no daemon a missing package locks runs or is
	// offered, and offers stop at the rules' count plus the packages'.
	if !slices.Equal(s.Packages, c.installed) || !slices.Equal(m.Installed, c.installed) || m.Theme != c.theme {
		fail("packages %v, installed %v theme %q, want %v and %q", s.Packages, m.Installed, m.Theme, c.installed, c.theme)
	}
	if slices.ContainsFunc(s.Daemons, s.DaemonLocked) || slices.ContainsFunc(s.DaemonOffers, s.DaemonLocked) ||
		len(s.DaemonOffers) > s.Rules.DaemonOffers+s.Rules.Grants(s.Packages).DaemonOffers {
		fail("daemons %v, offers %v with packages %v", s.Daemons, s.DaemonOffers, s.Packages)
	}
	if slices.ContainsFunc(s.UpgradeOffers, s.UpgradeLocked) || slices.ContainsFunc(s.Upgrades, s.UpgradeLocked) ||
		len(s.UpgradeOffers) > s.Rules.UpgradeOffers+s.Rules.Grants(s.Packages).UpgradeOffers {
		fail("upgrades %v, offers %v with packages %v", s.Upgrades, s.UpgradeOffers, s.Packages)
	}
	for _, it := range game.AllItems {
		if s.ItemLocked(it) && (s.Price(it) != 0 || s.CanSpend(it) == nil) {
			fail("%s is for sale without its package (%v)", it, s.Packages)
		}
	}
	if s.AtTime < 0 || s.AtTime > 0 && s.ItemLocked(game.At) {
		fail("at time %v with packages %v", s.AtTime, s.Packages)
	}
	// Prestige climbs one level, only by a clear at the top unlocked one
	// past the gate (D31), and the boot says so.
	switch g := game.NextPrestige(s.Rules, m); {
	case s.Unlocked == 0 && m.Prestige != c.prestige0:
		fail("prestige %d -> %d with nothing unlocked", c.prestige0, m.Prestige)
	case s.Unlocked != 0 && (s.Phase != game.PhaseCleared || s.Trial != nil || s.Prestige != c.prestige0 ||
		m.Prestige != c.prestige0+1 || s.Unlocked != m.Prestige || g.Rated < s.Rules.CalibAfter):
		fail("unlocked %d at prestige %d (top %d -> %d, %d rated of %d) in phase %s",
			s.Unlocked, s.Prestige, c.prestige0, m.Prestige, g.Rated, s.Rules.CalibAfter, s.Phase)
	}

	// Bytes are conserved: every byte a clear paid is committed, still
	// uncommitted, or lost to the crash, and only commits reach the bank.
	// A crash destroys the earlier dump first, then dumps what it lost.
	lost := int64(0)
	if s.Crash != nil {
		lost = s.Crash.Lost
		if s.Uncommitted != 0 {
			fail("uncommitted %d after a crash", s.Uncommitted)
		}
		d := m.CoreDump
		switch {
		case s.Trial != nil:
			// A failed trial leaves the dump a boot made, and no other.
			if got := mustJSON(t, d); got != c.dump0 || s.Crash.DumpLost != 0 {
				fail("trial crash changed the core dump %s to %s", c.dump0, got)
			}
		case s.Crash.DumpLost != c.dumpLost:
			fail("crash destroyed %d B of dump, want the %d B the boot started with", s.Crash.DumpLost, c.dumpLost)
		case s.Crash.Cause == game.Abandoned:
			// Abandoned between doors: no door to dump on, so the bytes
			// are lost outright.
			if d != nil || s.Crash.Door.Problem != "" {
				fail("abandoned boot left core dump %+v, door %+v", d, s.Crash.Door)
			}
		case (lost > 0) != (d != nil) || d != nil && (d.Problem != s.Crash.Door.Problem || d.Bytes != lost):
			fail("crash lost %d, core dump %+v", lost, d)
		}
	} else if got := mustJSON(t, m.CoreDump); got != c.dump0 {
		fail("core dump %s became %s without a crash", c.dump0, got)
	}
	if s.Crash != nil && s.Trial == nil && s.Crash.Cause != game.Abandoned {
		// The problem that crashed you comes back, from stage 0.
		i := slices.IndexFunc(m.Regressions, func(g game.Regression) bool { return g.Problem == s.Crash.Door.Problem })
		if i < 0 || m.Regressions[i].Stage != 0 || !m.Regressions[i].Due.Equal(s.Crash.At.AddDate(0, 0, s.Rules.RegressionDays[0])) {
			fail("crash on %s, regressions %+v", s.Crash.Door.Problem, m.Regressions)
		}
	}
	regs := map[catalog.ID]bool{}
	for _, g := range m.Regressions {
		if regs[g.Problem] || g.Stage < 0 || len(g.Days) > 0 && g.Stage >= len(g.Days) {
			fail("regression %+v in %+v", g, m.Regressions)
		}
		regs[g.Problem] = true
	}
	regDoors := 0
	for _, d := range s.Doors {
		if d.Regression {
			regDoors++
		}
	}
	if s.Current != nil && s.Current.Door.Regression {
		regDoors++
	}
	// Each attempt records the biome its door opened in, which is where
	// walking the attempts through the rooms puts it (a clear moves on a
	// room, and so does a door failed for cycles where the room is lost,
	// rules.FailMoveOn; a skipped tier has no room), and the open door is
	// in the current biome.
	biome, room := 0, 0
	last := len(s.Biomes) - 1
	for i, h := range s.History {
		if h.Biome != biome {
			fail("attempt %+v in biome %d, the walk says %d", h.Door, h.Biome, biome)
		}
		failed := (h.Outcome == game.TimedOut || h.Outcome == game.GaveUp) && (s.Crash == nil || i < len(s.History)-1)
		lost := failed && s.Rules.FailMoveOn && s.Biomes[biome].Kind != rules.Boss && (room+1 < s.Biomes[biome].Rooms || biome < last)
		if h.Outcome == game.Cleared || lost {
			for room++; room >= s.Biomes[biome].Rooms && biome < last; {
				biome, room = biome+1, 0
			}
		}
	}
	if s.Current != nil && s.Current.Biome != s.Biome {
		fail("open door in biome %d, boot in %d", s.Current.Biome, s.Biome)
	}
	cleared, rated := 0, 0
	for _, h := range s.History {
		if h.Door.Regression {
			regDoors++
		}
		if h.Outcome == game.Cleared {
			cleared++
		}
		if h.Outcome != game.Retreated {
			rated++
		}
	}
	if regDoors > 1 {
		fail("%d regression doors in one boot", regDoors)
	}
	// Every closed door but a retreat is rated, and every clear is a solve.
	if m.Rated != c.rated0+rated || len(m.Solves) != c.solves0+cleared {
		fail("rated %d solves %d, want %d and %d", m.Rated, len(m.Solves), c.rated0+rated, c.solves0+cleared)
	}
	for k, cal := range m.Calibration {
		limit := s.Rules.CalibMax
		if k != game.CalibrationAll {
			limit /= 2
		}
		if math.Abs(cal.Offset) > limit || cal.Observed < 0 || cal.Observed > 1 || cal.Expected < 0 || cal.Expected > 1 {
			fail("calibration %s %+v outside ±%.0f", k, cal, limit)
		}
	}
	if s.Trial != nil {
		if len(s.DaemonOffers) != 0 || len(s.Daemons) != 0 || len(s.Doors) > 1 {
			fail("trial with daemons %v offers %v doors %d", s.Daemons, s.DaemonOffers, len(s.Doors))
		}
		for _, d := range s.Doors {
			if d.Problem != s.Trial.Plan[s.Biome][s.Room] || d.Blind != (s.Biome > 0) || d.Cursed || d.Mystery || d.Tier != "" {
				fail("trial door %+v in biome %d room %d of plan %v", d, s.Biome, s.Room, s.Trial.Plan)
			}
		}
		if s.Phase == game.PhaseCleared && !s.Trial.Passed {
			fail("trial cleared but not passed")
		}
	}
	if s.Earned() != s.Committed+s.Uncommitted+s.Spent+lost {
		fail("earned %d != committed %d + uncommitted %d + spent %d + lost %d", s.Earned(), s.Committed, s.Uncommitted, s.Spent, lost)
	}
	// Only a Spend spends, by the item's price, and a trial sells nothing.
	if sp, ok := a.(game.Spend); s.Spent != c.spent && (!ok || s.Trial != nil || s.Spent-c.spent != s.Price(sp.Item)) {
		fail("spent %d -> %d", c.spent, s.Spent)
	}
	c.spent = s.Spent
	if s.Rerolls < 0 {
		fail("%d shufs", s.Rerolls)
	}
	if m.Bank < 0 {
		fail("bank %d", m.Bank)
	}
	if m.Bank != c.bank0+s.Committed || m.LifetimeBytes != c.lifetime0+s.Earned() {
		fail("bank %d lifetime %d, want %d + committed %d and %d + earned %d",
			m.Bank, m.LifetimeBytes, c.bank0, s.Committed, c.lifetime0, s.Earned())
	}
	if s.Phase == game.PhaseCleared && s.Uncommitted != 0 {
		fail("cleared with %d uncommitted", s.Uncommitted)
	}
	c.checkEase(s, m, a, err, fail)
	checkLog(s, fail)
}

// checkLog asserts the offer and attempt logs (ml-pipeline §4.1): every
// offer is well formed, the doors on offer are the latest doors offer's,
// not yet chosen, and every attempt points at the item it was chosen as,
// with one event per submit and quack it counted.
func checkLog(s *game.State, fail func(string, ...any)) {
	lastDoors := -1
	for i, o := range s.Offers {
		if o.Seq != i || o.Kind != "doors" && o.Kind != "route" || o.Chosen < -1 || o.Chosen >= len(o.Items) ||
			!(o.SetProp > 0 && o.SetProp <= 1) || len(o.Items) == 0 {
			fail("offer %d %+v", i, o)
		}
		for _, it := range o.Items {
			if !(it.Propensity > 0 && it.Propensity <= 1) || it.Eligible < 1 {
				fail("offer %d item %+v", i, it)
			}
		}
		if o.Kind == "doors" {
			lastDoors = i
		}
	}
	if s.Phase == game.PhaseDoors {
		if lastDoors < 0 {
			fail("doors on offer with no doors offer logged")
		}
		o := s.Offers[lastDoors]
		if o.Chosen != -1 || len(o.Items) != len(s.Doors) || o.Biome != s.Biome || o.Room != s.Room {
			fail("latest offer %+v for doors %+v in room %d.%d", o, s.Doors, s.Biome, s.Room)
		}
		for j, d := range s.Doors {
			if o.Items[j].Problem != d.Problem || o.Items[j].Position != j {
				fail("offer item %d %+v for door %+v", j, o.Items[j], d)
			}
		}
	}
	atts := slices.Clone(s.History)
	if s.Current != nil {
		atts = append(atts, *s.Current)
	}
	for _, a := range atts {
		if a.Offer < 0 || a.Offer >= len(s.Offers) || s.Offers[a.Offer].Kind != "doors" || a.CyclesAtOpen < 1 {
			fail("attempt %+v: offer %d of %d, lives at open %d", a.Door, a.Offer, len(s.Offers), a.CyclesAtOpen)
		}
		if o := s.Offers[a.Offer]; o.Chosen < 0 || o.Items[o.Chosen].Problem != a.Door.Problem {
			fail("attempt %+v chosen from offer %+v", a.Door, o)
		}
		submits, quacks := 0, 0
		for j, ev := range a.Events {
			switch ev.Kind {
			case "submit":
				submits++
			case "quack":
				quacks++
			case "overtime":
			default:
				fail("attempt %+v event %+v", a.Door, ev)
			}
			if j > 0 && ev.At.Before(a.Events[j-1].At) {
				fail("attempt %+v events out of order %+v", a.Door, a.Events)
			}
		}
		if submits != a.Submits || quacks != a.Quacks {
			fail("attempt %+v: %d submit and %d quack events, counted %d and %d", a.Door, submits, quacks, a.Submits, a.Quacks)
		}
	}
}

// checkEase asserts what the easing allows: only the floor easing's doors
// carry a timer buy-back, a timeout leaves the boot running only in a
// biome whose rule says so (D44), a decline takes what it added, and
// Meta.Governor counts how the boot ended.
func (c *checker) checkEase(s *game.State, m *game.Meta, a game.Action, err error, fail func(string, ...any)) {
	floor, eased := active(s, game.EaseFloor)
	doors := slices.Clone(s.Doors)
	if s.Current != nil {
		doors = append(doors, s.Current.Door)
	}
	for _, h := range s.History {
		doors = append(doors, h.Door)
	}
	for _, d := range doors {
		if d.Floor && (d.Rating > s.Rules.FloorRating || s.Rules.FloorDrop <= 0) {
			fail("door %+v at the floor of rules with floor %d, drop %v", d, s.Rules.FloorRating, s.Rules.FloorDrop)
		}
		if d.TimerBuy != 0 && (!slices.Contains(s.Rules.BuybackTimers, d.TimerBuy) || d.Tier != "" ||
			d.Regression || d.Boss != "" || len(s.Ease) == 0) {
			fail("door %+v bought back without a floor easing %+v", d, s.Ease)
		}
	}
	for i, h := range s.History {
		end := i == len(s.History)-1 && s.Crash != nil
		failed := h.Outcome == game.TimedOut || h.Outcome == game.GaveUp
		priced := s.Rules.Biomes[h.Biome].FailCycles > 0 || h.Outcome == game.TimedOut && s.Rules.Biomes[h.Biome].TimeoutLife
		if failed && !end && !priced {
			fail("door %+v %s in biome %d, which has no price for it, and the boot went on", h.Door, h.Outcome, h.Biome)
		}
		// A door failed for cycles comes back, as a crash's does.
		if failed && s.Trial == nil && !slices.ContainsFunc(m.Regressions, func(g game.Regression) bool { return g.Problem == h.Door.Problem }) {
			fail("door %+v %s with no regression scheduled", h.Door, h.Outcome)
		}
	}
	if _, ok := a.(game.DeclineEase); ok && err == nil {
		if !s.Declined || eased || len(s.History) > 0 {
			fail("declined %v, floor %+v, %d doors closed", s.Declined, floor, len(s.History))
		}
	}
	if done := s.Phase == game.PhaseCrashed || s.Phase == game.PhaseCleared; done && !c.ended {
		c.ended = true
		want := c.gov0
		switch {
		case s.Trial != nil:
		case s.Crash != nil && s.Crash.Cause == game.Abandoned:
			// Not counted, but a merge window reached starts the early
			// crashes over.
			if c.merged {
				want.EarlyCrashes = 0
			}
		case s.Phase == game.PhaseCleared:
			want = game.Governor{FullClears: want.FullClears + 1}
		case s.Biome == 0:
			want = game.Governor{EarlyCrashes: want.EarlyCrashes + 1}
		default:
			want = game.Governor{}
		}
		if m.Governor != want {
			fail("governor %+v after the boot, want %+v (from %+v)", m.Governor, want, c.gov0)
		}
	}
}

// fuzzer plays a boot with mostly sensible and some nonsense actions, on a
// clock that jumps around (sometimes past the deadline).
type fuzzer struct {
	rng *rand.Rand
	now time.Time
}

func (f *fuzzer) action(s *game.State) game.Action {
	r := f.rng
	if r.IntN(4) == 0 { // nonsense: any action, any index
		switch r.IntN(15) {
		case 0:
			return game.ChooseRoute{Index: r.IntN(5) - 1}
		case 1:
			return game.ChooseDoor{Index: r.IntN(6) - 1}
		case 2:
			return game.Retreat{}
		case 3:
			return game.Quack{}
		case 4:
			return game.Commit{}
		case 5:
			return game.PickUpgrade{Upgrade: game.AllUpgrades[r.IntN(3)]}
		case 6:
			return game.PickDaemon{Daemon: game.AllDaemons[r.IntN(5)]}
		case 7:
			return game.LeaveMerge{}
		case 8:
			return game.Tick{}
		case 9:
			return f.submit()
		case 10:
			if r.IntN(10) == 0 {
				return game.GiveUp{}
			}
			return game.Tick{}
		case 11:
			return game.Spend{Item: []game.Item{game.Fsck, game.Dup, game.Shuf, "bogus"}[r.IntN(4)]}
		case 12:
			return game.Reroll{}
		case 13:
			return game.DeclineEase{}
		case 14:
			if r.IntN(25) == 0 {
				return game.Abandon{}
			}
		}
		return nil
	}
	switch {
	case s.PendingUpgrade:
		return game.PickUpgrade{Upgrade: s.UpgradeOffers[r.IntN(len(s.UpgradeOffers))]}
	case s.Phase == game.PhaseRoute:
		return game.ChooseRoute{Index: r.IntN(len(s.Biomes[s.Biome].Offers))}
	case s.Phase == game.PhaseDoors:
		if len(s.Ease) > 0 && !s.Declined && len(s.History) == 0 && r.IntN(4) == 0 {
			return game.DeclineEase{}
		}
		if s.Rerolls > 0 && r.IntN(4) == 0 {
			return game.Reroll{}
		}
		return game.ChooseDoor{Index: r.IntN(len(s.Doors))}
	case s.Phase == game.PhaseMerge:
		switch {
		case r.IntN(3) == 0:
			return game.Spend{Item: game.AllItems[r.IntN(len(game.AllItems))]}
		case r.IntN(2) == 0 && s.Uncommitted > 0:
			return game.Commit{}
		case len(s.DaemonOffers) > 0 && r.IntN(2) == 0:
			return game.PickDaemon{Daemon: s.DaemonOffers[r.IntN(len(s.DaemonOffers))]}
		}
		return game.LeaveMerge{}
	case s.Phase == game.PhaseProblem:
		switch r.IntN(20) {
		case 0:
			return game.GiveUp{}
		case 1, 2:
			return game.Quack{}
		case 3, 4:
			return game.Retreat{}
		case 5, 6, 7, 8:
			return game.Tick{}
		}
		return f.submit()
	}
	return game.Tick{}
}

func (f *fuzzer) submit() game.SubmitResult {
	v := []game.Verdict{"AC", "AC", "AC", "WA", "TLE", "RE", "MLE", "CE"}[f.rng.IntN(8)]
	return game.SubmitResult{Verdict: v, Passed: f.rng.IntN(11), Total: 10}
}

func (f *fuzzer) tick() {
	f.now = f.now.Add(time.Duration(f.rng.IntN(int(12 * time.Minute))))
}

// move is one fuzzed action with its time, for replay.
type move struct {
	A   game.Action
	At  time.Time
	Err bool
}

// startBoot starts a boot at a random unlocked prestige.
func startBoot(t testing.TB, r rules.Rules, m *game.Meta, ps []catalog.Problem, f *fuzzer) *game.State {
	t.Helper()
	s, _, err := game.NewBoot(r, m, ps, f.rng.IntN(m.Prestige+1), f.rng.Uint64(), f.now)
	if err != nil {
		t.Fatalf("NewBoot: %v", err)
	}
	return s
}

// startTrial starts a learn-mode trial on a spec drawn from ps.
func startTrial(t testing.TB, m *game.Meta, ps []catalog.Problem, f *fuzzer) *game.State {
	t.Helper()
	spec := fuzzSpec(f.rng, ps)
	s, _, err := game.NewTrial(rules.Trial(), m, ps, spec, f.rng.Uint64(), f.now)
	if err != nil {
		t.Fatalf("NewTrial(%+v): %v", spec, err)
	}
	return s
}

// fuzzBoot plays one boot to its end (or a step cap).
func fuzzBoot(t testing.TB, r rules.Rules, m *game.Meta, ps []catalog.Problem, f *fuzzer, verify bool) (*game.State, []move) {
	t.Helper()
	s := startBoot(t, r, m, ps, f)
	return s, newChecker(t, s, m).play(s, m, ps, f, verify)
}

// fuzzTrial is fuzzBoot for a trial.
func fuzzTrial(t testing.TB, m *game.Meta, ps []catalog.Problem, f *fuzzer, verify bool) (*game.State, []move) {
	t.Helper()
	s := startTrial(t, m, ps, f)
	return s, newChecker(t, s, m).play(s, m, ps, f, verify)
}

// fuzzSpec is a trial on a random topic and band, with a unit of 0 to 5 of
// that topic's problems and sometimes an id the pool lacks.
func fuzzSpec(rng *rand.Rand, ps []catalog.Problem) game.TrialSpec {
	topic := catalog.Topics[rng.IntN(len(catalog.Topics))]
	lo := 800 + 100*rng.IntN(16)
	spec := game.TrialSpec{Module: "m-" + topic, IntegTopic: topic, IntegBand: [2]int{lo, lo + 300}, Capability: game.CapabilityFor(topic)}
	for range rng.IntN(6) {
		p := ps[rng.IntN(len(ps))]
		spec.Unit = append(spec.Unit, p.ID)
	}
	if rng.IntN(4) == 0 {
		spec.Unit = append(spec.Unit, "gone:1")
	}
	return spec
}

// play fuzzes s to its end (or a step cap), checking c after every step,
// and returns the actions with their times, for replay.
func (c *checker) play(s *game.State, m *game.Meta, ps []catalog.Problem, f *fuzzer, verify bool) []move {
	t := c.t
	t.Helper()
	c.check(s, m, nil, nil)
	var moves []move
	for n := 0; n < 400 && s.Phase != game.PhaseCrashed && s.Phase != game.PhaseCleared; n++ {
		f.tick()
		a := f.action(s)
		var before, meta string
		if verify {
			before, meta = mustJSON(t, s), mustJSON(t, m)
		}
		_, err := game.Step(s, m, ps, a, f.now)
		if err != nil && verify && (mustJSON(t, s) != before || mustJSON(t, m) != meta) {
			t.Fatalf("%T%+v failed (%v) but changed the state", a, a, err)
		}
		c.check(s, m, a, err)
		moves = append(moves, move{a, f.now, err != nil})
	}
	return moves
}

// TestFuzzTrials plays fuzzed trials between fuzzed boots, so trials meet
// core dumps and regressions a boot left, under every invariant plus the
// trial's own (one planned door per room, no daemons, the dump untouched).
func TestFuzzTrials(t *testing.T) {
	t.Parallel()
	ps := sim.SyntheticPool(600, 3)
	m := &game.Meta{Prestige: game.MaxPrestige}
	f := &fuzzer{rng: rand.New(rand.NewPCG(21, 22)), now: t0}
	ends := map[game.Phase]int{}
	runs := 2000
	if testing.Short() {
		runs = 200
	}
	for i := range runs {
		if i%3 == 2 { // two trials first: boot #1 comes after them
			fuzzBoot(t, rules.Standard(), m, ps, f, false)
		} else {
			s, _ := fuzzTrial(t, m, ps, f, i%10 == 1)
			ends[s.Phase]++
		}
		f.now = f.now.Add(time.Duration(f.rng.IntN(int(72 * time.Hour))))
	}
	t.Logf("trial ends %v, %d modules passed, capabilities %v", ends, len(m.TrialsPassed), m.Capabilities)
	if ends[game.PhaseCleared] == 0 || ends[game.PhaseCrashed] == 0 {
		t.Errorf("ends %v: the fuzzer should reach both", ends)
	}
}

// TestFuzzBoots plays thousands of fuzzed boots with meta carried across,
// checking every invariant after every step.
func TestFuzzBoots(t *testing.T) {
	t.Parallel()
	pools := map[string][]catalog.Problem{
		"synthetic": sim.SyntheticPool(600, 3),
		"tiny":      dense([]string{"dp"}, 1000, 1100, 50), // exhausted at once
	}
	for _, r := range presets() {
		for name, ps := range pools {
			t.Run(r.Name+"/"+name, func(t *testing.T) {
				t.Parallel()
				fuzzBoots(t, r, name, ps)
			})
		}
	}
}

// fuzzBoots plays thousands of fuzzed boots of r on ps with meta carried
// across, and checks the fuzzer met what the invariants guard.
func fuzzBoots(t *testing.T, r rules.Rules, name string, ps []catalog.Problem) {
	m := &game.Meta{Prestige: game.MaxPrestige}
	f := &fuzzer{rng: rand.New(rand.NewPCG(11, 12)), now: t0}
	ends := map[game.Phase]int{}
	bought := map[string]int{} // accepted actions by type
	installs := 0
	eases := map[string]int{} // boots by easing; failures the boot survived
	boots := 3000
	if testing.Short() {
		boots = 300
	}
	for i := range boots {
		s, moves := fuzzBoot(t, r, m, ps, f, i%10 == 0)
		ends[s.Phase]++
		for _, es := range s.Ease {
			eases[string(es.Kind)]++
		}
		for j, h := range s.History {
			if (h.Outcome == game.TimedOut || h.Outcome == game.GaveUp) && (s.Crash == nil || j < len(s.History)-1) {
				eases["saved "+string(h.Outcome)]++
				if h.Door.Boss != "" {
					eases["saved boss"]++
				}
			}
		}
		if s.Crash != nil {
			eases["crash "+string(s.Crash.Cause)]++
		}
		for _, mv := range moves {
			if !mv.Err {
				bought[fmt.Sprintf("%T", mv.A)]++
			}
		}
		if f.rng.IntN(3) == 0 && m.CoreDump != nil {
			game.Upsolve(m, m.CoreDump.Problem, game.Editorial, time.Hour, f.now)
		}
		installs += fuzzInstall(t, r, m, f)
		f.now = f.now.Add(time.Duration(f.rng.IntN(int(72 * time.Hour))))
	}
	t.Logf("%s %s: boot ends %v, meta rating %.0f, %d solves, %d regressions, %d spends, %d shufs, installed %v, easings %v, %d declined, %d abandoned",
		r.Name, name, ends, m.Rating, len(m.Solves), len(m.Regressions), bought["game.Spend"], bought["game.Reroll"], m.Installed,
		eases, bought["game.DeclineEase"], bought["game.Abandon"])
	if name == "synthetic" && (eases["governor"] == 0 || eases["saved timed-out"] == 0 || bought["game.DeclineEase"] == 0 ||
		bought["game.Abandon"] == 0) {
		t.Errorf("easings %v, %d declined, %d abandoned: the fuzzer should meet the governor, a saved timeout, a decline and an abandon",
			eases, bought["game.DeclineEase"], bought["game.Abandon"])
	}
	bosses := 0
	for _, b := range r.Biomes {
		if b.Kind == rules.Boss {
			bosses++
		}
	}
	if name == "synthetic" && r.FailMoveOn && (eases["saved gave-up"] == 0 || bosses > max(r.OneShot, 1) && eases["saved boss"] == 0) {
		t.Errorf("easings %v: under cycles the fuzzer should survive a give-up, and a boss failure where a boss retries", eases)
	}
	if ends[game.PhaseCleared] == 0 || ends[game.PhaseCrashed] == 0 {
		t.Errorf("ends %v: the fuzzer should reach both", ends)
	}
	if bought["game.Spend"] == 0 || bought["game.Reroll"] == 0 {
		t.Errorf("accepted %v: the fuzzer should buy and use items", bought)
	}
	if installs == 0 {
		t.Errorf("no package installed in %d boots", boots)
	}
}

// fuzzInstall tries, between boots, to install a few packages of r at
// random (one already installed, one not yet allowed or affordable) and to
// switch banner themes. A refusal changes nothing; an install takes the
// price, once, and the bank never goes negative. It returns how many
// installs went through.
func fuzzInstall(t *testing.T, r rules.Rules, m *game.Meta, f *fuzzer) int {
	t.Helper()
	n := 0
	for range f.rng.IntN(3) {
		p := r.Packages[f.rng.IntN(len(r.Packages))]
		before, bank, count := mustJSON(t, m), m.Bank, len(m.Installed)
		err := game.Install(m, p)
		switch {
		case err != nil && (!errors.Is(err, game.ErrInvalid) || mustJSON(t, m) != before):
			t.Fatalf("refused Install(%s) (%v) changed meta", p.Name, err)
		case err == nil && (m.Bank != bank-p.Price || len(m.Installed) != count+1 || m.Installed[count] != p.Name):
			t.Fatalf("Install(%s): bank %d -> %d, installed %v", p.Name, bank, m.Bank, m.Installed)
		case err == nil:
			n++
		}
		if themes := []string{"", "cowsay", "tux", "bogus"}; f.rng.IntN(4) == 0 {
			theme := m.Theme
			if game.UseTheme(r, m, themes[f.rng.IntN(len(themes))]) != nil && m.Theme != theme {
				t.Fatalf("refused theme changed %q to %q", theme, m.Theme)
			}
		}
	}
	if m.Bank < 0 || len(slices.Compact(slices.Sorted(slices.Values(m.Installed)))) != len(m.Installed) {
		t.Fatalf("bank %d, installed %v", m.Bank, m.Installed)
	}
	return n
}

// TestModelInvariants runs the sim's model player through thousands of
// boots under the same invariants.
func TestModelInvariants(t *testing.T) {
	t.Parallel()
	for _, r := range presets() {
		for _, rating := range []float64{900, 1500, 2300} {
			t.Run(fmt.Sprint(r.Name, rating), func(t *testing.T) {
				modelInvariants(t, r, rating)
			})
		}
	}
}

// modelInvariants plays the model at rating through 1500 boots of r,
// climbing, with every invariant checked after every call.
func modelInvariants(t *testing.T, r rules.Rules, rating float64) {
	t.Parallel()
	var c *checker
	rep := sim.Run(sim.Config{
		Rules:    r,
		Pool:     sim.SyntheticPool(600, 4),
		Player:   sim.DefaultPlayer(rating),
		Boots:    1500,
		Seed:     uint64(rating),
		Prestige: -1,
		Check: func(s *game.State, m *game.Meta, a game.Action, _ []game.Effect, err error) {
			if a == nil { // NewBoot
				if err != nil {
					t.Fatalf("NewBoot: %v", err)
				}
				c = newChecker(t, s, m)
			}
			c.check(s, m, a, err)
		},
	})
	if rep.Errors != 0 {
		t.Errorf("%d model actions failed", rep.Errors)
	}
}

// TestDeterminism: the same seed and the same actions at the same times give
// byte-identical state, meta and effects.
func TestDeterminism(t *testing.T) {
	t.Parallel()
	ps := sim.SyntheticPool(600, 5)
	run := func(r rules.Rules) string {
		m := &game.Meta{Prestige: 2}
		f := &fuzzer{rng: rand.New(rand.NewPCG(1, 2)), now: t0}
		var out []string
		for range 50 {
			s, moves := fuzzBoot(t, r, m, ps, f, false)
			out = append(out, mustJSON(t, s), fmt.Sprint(len(moves)))
		}
		return mustJSON(t, out) + mustJSON(t, m)
	}
	for _, r := range presets() {
		if a, b := run(r), run(r); a != b {
			t.Fatalf("%s: two runs from the same seed diverged", r.Name)
		}
	}

	// Different seeds give different boots.
	m1, m2 := &game.Meta{}, &game.Meta{}
	s1, _, _ := game.NewBoot(rules.Standard(), m1, ps, 0, 1, t0)
	s2, _, _ := game.NewBoot(rules.Standard(), m2, ps, 0, 2, t0)
	if mustJSON(t, s1.Doors) == mustJSON(t, s2.Doors) {
		t.Error("seeds 1 and 2 offered the same doors")
	}
}

// TestJSONRoundTrip saves boots and trials mid-run, loads them, and plays
// the same actions on both copies: they must end identical. Replaying the
// first half on a fresh start must also accept and refuse what the original
// run did.
func TestJSONRoundTrip(t *testing.T) {
	t.Parallel()
	ps := sim.SyntheticPool(600, 6)
	for _, tc := range []struct {
		name       string
		seeds      uint64
		minResumed int
		start      func(*game.Meta, *fuzzer) *game.State
	}{
		{"boot", 300, 1000, func(m *game.Meta, f *fuzzer) *game.State { return startBoot(t, rules.Legacy(), m, ps, f) }},
		{"cycles", 300, 1000, func(m *game.Meta, f *fuzzer) *game.State { return startBoot(t, cyclesLinear(), m, ps, f) }},
		{"trial", 150, 300, func(m *game.Meta, f *fuzzer) *game.State { return startTrial(t, m, ps, f) }},
	} {
		resumed := 0
		for seed := range tc.seeds {
			fresh := func() (*game.State, *game.Meta, *fuzzer) {
				m := &game.Meta{Prestige: game.MaxPrestige}
				f := &fuzzer{rng: rand.New(rand.NewPCG(seed, 9)), now: t0}
				return tc.start(m, f), m, f
			}
			s, m, f := fresh()
			moves := newChecker(t, s, m).play(s, m, ps, f, false)
			if len(moves) < 4 {
				continue
			}
			cut := len(moves) / 2

			s2, m2, _ := fresh()
			for _, mv := range moves[:cut] {
				if _, err := game.Step(s2, m2, ps, mv.A, mv.At); (err != nil) != mv.Err {
					t.Fatalf("%s seed %d: replayed %T%+v: %v", tc.name, seed, mv.A, mv.A, err)
				}
			}
			var s3 game.State
			var m3 game.Meta
			if err := json.Unmarshal([]byte(mustJSON(t, s2)), &s3); err != nil {
				t.Fatalf("unmarshal state: %v", err)
			}
			if err := json.Unmarshal([]byte(mustJSON(t, m2)), &m3); err != nil {
				t.Fatalf("unmarshal meta: %v", err)
			}
			if mustJSON(t, &s3) != mustJSON(t, s2) {
				t.Fatalf("%s seed %d: state JSON does not round-trip", tc.name, seed)
			}
			for _, mv := range moves[cut:] {
				fx2, err2 := game.Step(s2, m2, ps, mv.A, mv.At)
				fx3, err3 := game.Step(&s3, &m3, ps, mv.A, mv.At)
				if (err2 == nil) != (err3 == nil) || (err2 != nil) != mv.Err || fmt.Sprint(fx2) != fmt.Sprint(fx3) {
					t.Fatalf("%s seed %d: %T%+v diverged after the reload: %v / %v", tc.name, seed, mv.A, mv.A, err2, err3)
				}
			}
			if mustJSON(t, s2) != mustJSON(t, &s3) || mustJSON(t, m2) != mustJSON(t, &m3) {
				t.Fatalf("%s seed %d: reloaded run ended differently", tc.name, seed)
			}
			resumed += len(moves) - cut
		}
		if resumed < tc.minResumed {
			t.Errorf("%s: only %d moves played after a reload", tc.name, resumed)
		}
	}
}
