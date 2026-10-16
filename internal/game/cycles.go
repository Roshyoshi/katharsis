package game

import (
	"slices"
	"time"

	"katharsis/internal/rules"
	"katharsis/internal/units"
)

// Cycles (D47, D48; docs/research/meta.md §(b)). The pool is State.Cycles,
// and every failure burns from it at a price shown before it is paid:
//
//	door open ─┬─ :w                 samples, free
//	           ├─ :Submit fails a hidden test   SubmitCost (ECC: clock, not cycles, while
//	           │                                more than its 5:00 is left)
//	           ├─ timer runs out / :q!          FailCost: the room is lost, the boot goes on;
//	           │                                a boss draws a new boss door; a one-shot boss ends it
//	           ├─ R retreat                     RetreatCost, only before RetreatLockAt
//	           └─ AC                            pay and an upgrade pick, no heal
//	a burn that would leave 0 cycles or less ends the boot
//
// A boot saved before cycles has none of the rules' cycles fields, and its
// queries answer as lives did: one apiece, a timeout fatal outside D44's
// Userland rule, a give-up always fatal.

// FailCost is what a timeout or a give-up (o TimedOut or GaveUp) of the
// open door burns, and whether the boot survives it (ok). A one-shot boss
// (OneShot) and a biome without a failure price (rules.Biome.FailCycles,
// D44's TimeoutLife aside) end the boot whatever the cycles: cycles is 0
// then. When the burn would leave none, cycles is the price and ok false.
func (s *State) FailCost(o Outcome) (cycles int, ok bool) {
	r := &s.Rules
	if s.Trial != nil || s.Biome >= len(r.Biomes) || o != TimedOut && o != GaveUp {
		return 0, false
	}
	b := r.Biomes[s.Biome]
	switch {
	case b.FailCycles > 0:
		if s.OneShot() {
			return 0, false
		}
		cycles = b.FailCycles
		if s.afterFirstBoss() {
			cycles += r.Ascend(s.Prestige).LateFailCycles
		}
	case o == TimedOut && b.TimeoutLife:
		cycles = r.Life()
	default:
		return 0, false
	}
	return cycles, s.Cycles > cycles
}

// TimeoutCostsLife reports whether the open door's timer running out now
// would cost cycles rather than the boot (FailCost). The door screen says
// it in these terms. Before cycles it was D44's Userland rule alone.
func (s *State) TimeoutCostsLife() bool {
	_, ok := s.FailCost(TimedOut)
	return ok
}

// SubmitCost is what a charged failed submit (not CE, not a free verdict)
// of the open door costs at now, failing being how many of its tests fail
// (0 when not known yet): cycles burned, or, under ECC's conversion
// (rules.ECCPenalty), clock taken off the door. Off-by-One's twist
// multiplies either when exactly one test fails (rules.Twists.OffByOne);
// a screen showing the price before a submit asks with failing 1 for the
// worst. ECC pays in clock only while the door has more than that left
// (ECCShort): paid from less, it would run the clock out, and a timeout
// burns far more than the cycles. A cursed door's curse is apart: one
// failed submit ends the boot.
func (s *State) SubmitCost(failing int, now time.Time) (cycles int, clock time.Duration) {
	r := &s.Rules
	k := s.submitMult(failing)
	if s.ECCConverts() && !s.ECCShort(failing, now) {
		return 0, time.Duration(k) * r.ECCPenalty.Duration
	}
	cycles = max(1, r.SubmitPrice()+r.Ascend(s.Prestige).SubmitCycles)
	if slices.Contains(s.Daemons, Nice) {
		cycles *= r.NiceLifeCost
	}
	return k * cycles, 0
}

// submitMult is Off-by-One's multiplier on a failed submit of the open
// door with failing tests failing, 1 off its door.
func (s *State) submitMult(failing int) int {
	if m := s.Rules.Twists.OffByOne; m > 1 && failing == 1 && s.Current != nil && s.Current.Door.Twist == "off-by-one" {
		return m
	}
	return 1
}

// ECCConverts reports whether the boot holds ECC as the conversion: failed
// submits cost the door's clock, not cycles.
func (s *State) ECCConverts() bool {
	return s.Rules.ECCPenalty.Duration > 0 && slices.Contains(s.Upgrades, ECC)
}

// ECCShort reports whether, under ECC's conversion, the open door's clock
// at now holds no more than a failed submit's penalty plus ECCMargin
// (failing as for SubmitCost, Off-by-One's double included), so the
// submit burns cycles instead.
func (s *State) ECCShort(failing int, now time.Time) bool {
	if !s.ECCConverts() || s.Current == nil {
		return false
	}
	pen := time.Duration(s.submitMult(failing)) * s.Rules.ECCPenalty.Duration
	return s.Current.Deadline.Sub(now) <= pen+s.Rules.ECCMargin.Duration
}

// RetreatCost is what a retreat of the open door burns, 0 with free set
// when kexec's swap is unused. A retreat that would leave no cycles is
// refused, as is one at or past RetreatLockAt.
func (s *State) RetreatCost() (cycles int, free bool) {
	if slices.Contains(s.Daemons, Kexec) && !s.KexecUsed {
		return 0, true
	}
	return s.Rules.RetreatPrice(), false
}

// RetreatLockAt is when the open door stops allowing a retreat
// (rules.RetreatLock), zero when it never does or no door is open.
func (s *State) RetreatLockAt() time.Time {
	lock := s.Rules.RetreatLock.Duration
	if lock <= 0 || s.Current == nil {
		return time.Time{}
	}
	return s.Current.Opened.Add(lock)
}

// OneShot reports whether the current biome is a boss that ends the boot on
// a timeout or give-up whatever the cycles: one of the boot's last
// rules.OneShot bosses (with the ascensions'), or any boss of a boot saved
// before cycles. The others draw a new boss door.
func (s *State) OneShot() bool {
	r := &s.Rules
	if s.Biome >= len(s.Biomes) || s.Biomes[s.Biome].Kind != rules.Boss {
		return false
	}
	if r.Biomes[s.Biome].FailCycles <= 0 {
		return true
	}
	return s.bossesLeft() <= max(r.OneShot, r.Ascend(s.Prestige).OneShot)
}

// bossesLeft counts the boss tiers from the current one on.
func (s *State) bossesLeft() int {
	n := 0
	for i := s.Biome; i < len(s.Biomes); i++ {
		if s.Biomes[i].Kind == rules.Boss {
			n++
		}
	}
	return n
}

// afterFirstBoss reports whether the current tier comes after the boot's
// first boss (acts II and III), where Ascension.LateFailCycles bites.
func (s *State) afterFirstBoss() bool {
	for i := range min(s.Biome, len(s.Biomes)) {
		if s.Biomes[i].Kind == rules.Boss {
			return true
		}
	}
	return false
}

// FsckHeal and ECCHeal are what an fsck and an ECC pick (when it heals:
// not ECCConverts) restore, the ascensions' cut taken, at least 1.
func (s *State) FsckHeal() int {
	return max(1, s.Rules.FsckHeal()+s.Rules.Ascend(s.Prestige).HealCycles)
}

func (s *State) ECCHeal() int {
	return max(1, s.Rules.ECCHeal()+s.Rules.Ascend(s.Prestige).HealCycles)
}

// StartCycles is the pool the boot was dealt before any easing: the rules'
// and the ascensions', at least 1. The fountain fills to it, not to
// MaxCycles.
func (s *State) StartCycles() int {
	return max(1, s.Rules.Cycles+s.Rules.Ascend(s.Prestige).Cycles)
}

// Suspendable reports whether the boot can be suspended now: it is between
// doors (a route, doors on offer, an upgrade pick, a merge window), not in
// one and not ended (meta.md §(c)). Inside a door there is no quitting,
// and the deadline holds if the game is closed.
func (s *State) Suspendable() bool {
	return s.Phase == PhaseRoute || s.Phase == PhaseDoors || s.Phase == PhaseMerge
}

// Act is one act of a boot: its tiers, from First to Last (indices into
// State.Biomes), the last a boss's (map.md §(a)). A linear boot of one boss
// is one act; tiers after the last boss, if a preset has any, close one
// more.
type Act struct{ First, Last int }

// Acts are the boot's acts in order.
func (s *State) Acts() []Act {
	var acts []Act
	first := 0
	for i, b := range s.Biomes {
		if b.Kind == rules.Boss || i == len(s.Biomes)-1 {
			acts = append(acts, Act{first, i})
			first = i + 1
		}
	}
	return acts
}

// ActOf is the index in Acts of the act tier i belongs to.
func (s *State) ActOf(i int) int {
	for a, act := range s.Acts() {
		if i <= act.Last {
			return a
		}
	}
	return max(len(s.Acts())-1, 0)
}

// abandon ends the boot because another starts while it is suspended
// (Abandon). With a door open it is that door's give-up, ending the boot
// as :q! did before cycles; a deadline already past is the timeout's.
func (e *engine) abandon() error {
	s, m := e.s, e.meta
	switch s.Phase {
	case PhaseCrashed, PhaseCleared:
		return invalid("abandon in phase %s", s.Phase)
	case PhaseProblem:
		// A deadline already past is the timeout's: under cycles it may
		// end only the door, and what is left is abandoned below.
		if !e.expired() {
			e.crash(GaveUp, false)
		}
		if s.Phase == PhaseCrashed || s.Phase == PhaseCleared {
			return nil
		}
	}
	lost := s.Uncommitted
	s.Uncommitted = 0
	var dumpLost int64
	if d := m.CoreDump; d != nil && s.Trial == nil {
		dumpLost = d.Bytes
		e.logf("coredump: %s on %s destroyed", units.Bytes(dumpLost), d.Problem)
		m.CoreDump = nil
	}
	s.Doors, s.DaemonOffers = nil, nil
	m.LastPlayed = e.now
	c := Crash{Cause: Abandoned, Lost: lost, At: e.now, Uptime: s.Uptime(e.now), DumpLost: dumpLost}
	s.Crash = &c
	s.Phase = PhaseCrashed
	e.logf("boot #%d abandoned for a new boot: %s lost", s.Boot, units.Bytes(lost))
	return nil
}

// saveFail closes the open door with o (TimedOut or GaveUp) for its price
// (State.FailCost) when the boot survives it, and reports whether it did;
// otherwise the caller crashes the boot. The door is rated as failed and
// comes back as a regression, as a crash's would. Then a boss draws a new
// boss door; with rules.FailMoveOn any other room is lost (no pay, no
// upgrade pick) and the boot moves to the next room or merge window;
// otherwise the room is drawn again, as D44 did.
func (e *engine) saveFail(o Outcome) bool {
	s := e.s
	cost, ok := s.FailCost(o)
	if !ok {
		return false
	}
	b := s.Biomes[s.Biome]
	boss := b.Kind == rules.Boss
	moveOn := !boss && s.Rules.FailMoveOn && (s.Room+1 < b.Rooms || s.Biome < len(s.Biomes)-1)
	name := e.door()
	a := e.close(o)
	s.Cycles -= cost
	e.hit()
	what := map[Outcome]string{TimedOut: "timer expired", GaveUp: ":q!"}[o]
	then := ""
	switch {
	case boss:
		then = ", a new boss door"
	case moveOn:
		then = ", room lost"
	}
	e.logf("%s: %s, -%s, %d left%s", name, what, units.Plural(cost, "cycle"), s.Cycles, then)
	e.regress(a.Door.Problem)
	e.emit(CloseProblem{Door: a.Door}, Sound{Name: "wa"})
	switch {
	case boss:
		s.Phase = PhaseDoors
		e.rollRoom(false, "retry")
	case moveOn:
		if s.Room++; s.Room < b.Rooms {
			s.Phase = PhaseDoors
			e.rollRoom(true, "fresh")
		} else {
			e.enterMerge()
		}
	default:
		s.Phase = PhaseDoors
		e.rollRoom(false, map[Outcome]string{TimedOut: "timeout", GaveUp: "gave-up"}[o])
	}
	return true
}

// offerable reports whether upgrade u would do something now, and can be
// paid for. ECC as the conversion (rules.ECCPenalty) is offered until the
// boot holds it, ECC as a heal while cycles are short of MaxCycles;
// Overclock until its cap (rules.OverclockMax) and while more cycles are
// left than it burns.
func (s *State) offerable(u Upgrade) bool {
	r := &s.Rules
	switch u {
	case Overclock:
		return (r.OverclockMax <= 0 || s.count(Overclock) < r.OverclockMax) && s.Cycles > r.OverclockCycles
	case ECC:
	default:
		return true
	}
	if s.Rules.ECCPenalty.Duration > 0 {
		return !slices.Contains(s.Upgrades, ECC)
	}
	return s.Cycles < s.MaxCycles
}

// twist bends the rule boss door d's boss bends (rules.Twists), where the
// rules turn it on: Race Condition's clock runs fast (its pay is doorBytes'),
// and the rest are flags the judge and the HUD carry out, or a price
// (Off-by-One, SubmitCost).
func (e *engine) twist(d *Door) {
	tw := e.s.Rules.Twists
	switch d.Boss {
	case "off-by-one":
		if tw.OffByOne > 1 {
			d.Twist = d.Boss
		}
	case "oom-killer":
		if tw.OOMMemory > 0 {
			d.Twist, d.MemMult = d.Boss, tw.OOMMemory
		}
	case "heisenbug":
		if tw.Heisenbug {
			d.Twist, d.QuietSamples = d.Boss, true
		}
	case "race-condition":
		if tw.RaceClock > 0 {
			d.Twist, d.Timer = d.Boss, roundTimer(scale(d.Timer, tw.RaceClock))
		} else if tw.RacePay > 0 {
			d.Twist = d.Boss
		}
	case "undefined-behavior":
		if tw.UBNoSanitizers {
			d.Twist, d.NoSanitizers = d.Boss, true
		}
	case FinalBoss:
		if tw.HaltingClock {
			d.Twist, d.HiddenClock = d.Boss, true
		}
	}
}

// fountain restores this merge window's free cycles (rules.FountainCycles,
// with the ascensions' cut), up to the pool the boot was dealt
// (StartCycles), where rules.FountainWhere puts a fountain: between acts by
// default, gone from P5 (map.md §(e)).
func (e *engine) fountain() {
	s, r := e.s, &e.s.Rules
	s.Fountain = 0
	gain := r.FountainCycles + r.Ascend(s.Prestige).FountainCycles
	next := s.Biome + 1
	switch {
	case gain <= 0, s.Trial != nil,
		r.FountainWhere == rules.FountainAct && s.Biomes[s.Biome].Kind != rules.Boss,
		r.FountainWhere == rules.FountainBoss && (next >= len(s.Biomes) || s.Biomes[next].Kind != rules.Boss):
		return
	}
	gain = min(gain, s.StartCycles()-s.Cycles)
	if gain <= 0 {
		return
	}
	s.Cycles += gain
	s.Fountain, s.Fountained = gain, s.Fountained+gain
	e.logf("fountain: +%s, %d", units.Plural(gain, "cycle"), s.Cycles)
}
