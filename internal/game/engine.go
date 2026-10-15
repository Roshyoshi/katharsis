package game

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/rules"
	"katharsis/internal/units"
)

// ErrInvalid wraps every error Step returns for an action the current phase
// does not allow. The state is unchanged when Step returns it.
var ErrInvalid = errors.New("invalid action")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalid}, args...)...)
}

// AllUpgrades and AllDaemons are what picks draw from.
var (
	AllUpgrades = []Upgrade{Overclock, ECC, Duck, ShufPick}
	AllDaemons  = []Daemon{Strace, Nice, Fork, DevRandom, Kexec, Cron, Chrt}
)

// engine is one call's worth of context. Handlers validate first and mutate
// after, so an error leaves State and Meta as they were; the RNG is only
// drawn from after validation for the same reason.
type engine struct {
	s    *State
	meta *Meta
	pool []catalog.Problem
	now  time.Time
	rng  *rand.Rand
	fx   []Effect

	// first is set in NewBoot for a new player's first boot, and comeback
	// is the days away when the boot is a comeback (Comeback), else 0.
	first    bool
	comeback int

	// draws are the doors drawDoors drew last, by problem, and setProp the
	// chance of that set: what the next offer logs (logOffer).
	draws   map[catalog.ID]drawn
	setProp float64
}

func newEngine(s *State, m *Meta, pool []catalog.Problem, now time.Time) *engine {
	return &engine{s: s, meta: m, pool: pool, now: now, rng: newRand(&s.RNG)}
}

func (e *engine) emit(fx ...Effect) { e.fx = append(e.fx, fx...) }

func (e *engine) logf(format string, args ...any) {
	e.s.Log = append(e.s.Log, LogLine{At: e.now, Text: fmt.Sprintf(format, args...)})
}

// NewBoot starts boot meta.Boots+1 at prestige. The rules are snapshotted
// (with defaults filled) into the State; meta is only touched once the
// arguments check out.
func NewBoot(r rules.Rules, meta *Meta, pool []catalog.Problem, prestige int, seed uint64, now time.Time) (*State, []Effect, error) {
	if meta == nil {
		return nil, nil, errors.New("game: nil meta")
	}
	r = r.WithDefaults()
	if err := r.Validate(); err != nil {
		return nil, nil, err
	}
	if prestige < 0 || prestige > MaxPrestige {
		return nil, nil, fmt.Errorf("game: prestige %d outside 0..%d", prestige, MaxPrestige)
	}
	if prestige > meta.Prestige {
		return nil, nil, fmt.Errorf("game: prestige %d is locked (highest unlocked %d)", prestige, meta.Prestige)
	}
	if !slices.ContainsFunc(pool, func(p catalog.Problem) bool { return p.Judgeable }) {
		return nil, nil, errors.New("game: no judgeable problems in the pool")
	}

	// Boot #1 plays its own shape (rules.FirstBiomes, RT11); the snapshot
	// records it as played.
	first := !booted(meta)
	if first && len(r.FirstBiomes) > 0 {
		r.Biomes = slices.Clone(r.FirstBiomes)
	}
	s := &State{Seed: seed, RNG: seed, Rules: r, Prestige: prestige, Started: now}
	e := newEngine(s, meta, pool, now)
	e.first = first
	e.comeback = Comeback(&r, meta, now) // before begin marks now as played
	e.begin()
	e.logf("boot #%d: prestige %d, rules %s, seed %#x, rating %.0f", s.Boot, prestige, r.Name, seed, meta.Rating)
	e.planAim()
	e.layout()
	e.planFloor()
	e.enterBiome(0)
	return s, e.fx, nil
}

// begin numbers the boot, takes a snapshot of its packages and deals its
// lives and ducks. It is the first write to Meta, so NewBoot and NewTrial
// call it once their arguments have checked out.
func (e *engine) begin() {
	s, m, r := e.s, e.meta, &e.s.Rules
	m.Boots++
	if s.Trial != nil {
		m.Trials++
	}
	if m.Rating <= 0 {
		m.Rating = r.ColdStartRating
	}
	m.LastPlayed = e.now
	s.Boot = m.Boots
	s.AtStart = startOf(s, m) // debrief.go
	s.Lives = max(1, r.Lives+r.Ascend(s.Prestige).Lives)
	s.MaxLives = max(r.MaxLives, s.Lives)
	s.Packages = slices.Clone(m.Installed)
	// A package's ducks come once, at the start; merge windows refill to
	// the base.
	s.Ducks = e.baseDucks() + r.Grants(s.Packages).Ducks
}

// baseDucks is what the boot starts with and merge windows refill to,
// before Duck upgrades.
func (e *engine) baseDucks() int {
	r := &e.s.Rules
	return max(0, r.Ducks+r.Ascend(e.s.Prestige).Ducks)
}

// Step applies one action. Tick is accepted in every phase (it only matters
// while a door is open); anything else the phase does not allow returns an
// ErrInvalid error and changes nothing.
func Step(s *State, meta *Meta, pool []catalog.Problem, a Action, now time.Time) ([]Effect, error) {
	if s == nil || meta == nil {
		return nil, errors.New("game: nil state or meta")
	}
	e := newEngine(s, meta, pool, now)
	var err error
	switch a := a.(type) {
	case Tick:
		err = e.tick()
	case ChooseRoute:
		err = e.chooseRoute(a.Index)
	case ChooseDoor:
		err = e.chooseDoor(a.Index)
	case SubmitResult:
		err = e.submit(a)
	case Judging:
		err = e.judging(a.On)
	case Quack:
		err = e.quack()
	case GiveUp:
		err = e.giveUp()
	case Retreat:
		err = e.retreat()
	case Commit:
		err = e.commit()
	case PickUpgrade:
		err = e.pickUpgrade(a.Upgrade)
	case PickDaemon:
		err = e.pickDaemon(a.Daemon)
	case LeaveMerge:
		err = e.leaveMerge()
	case Link:
		err = e.link(a.Libs)
	case Spend:
		err = e.spend(a.Item)
	case Reroll:
		err = e.reroll()
	case DeclineEase:
		err = e.declineEase()
	default:
		err = invalid("unknown action %T", a)
	}
	if err != nil {
		return nil, err
	}
	return e.fx, nil
}

func (e *engine) need(p Phase, what string) error {
	if e.s.Phase != p {
		return invalid("%s in phase %s", what, e.s.Phase)
	}
	return nil
}

// door names the open door in logs: door_N is the boot's Nth attempt.
func (e *engine) door() string { return fmt.Sprintf("door_%d", len(e.s.History)+1) }

// ---- route and doors -------------------------------------------------------

func (e *engine) chooseRoute(i int) error {
	if err := e.need(PhaseRoute, "choose route"); err != nil {
		return err
	}
	b := &e.s.Biomes[e.s.Biome]
	if i < 0 || i >= len(b.Offers) {
		return invalid("route %d of %d", i, len(b.Offers))
	}
	e.choose("route", i)
	b.Topic = b.Offers[i]
	e.logf("route: biome_%d takes %s", e.s.Biome, b.Topic)
	e.s.Phase = PhaseDoors
	e.rollRoom(true, "fresh")
	return nil
}

func (e *engine) chooseDoor(i int) error {
	s := e.s
	if err := e.need(PhaseDoors, "choose door"); err != nil {
		return err
	}
	if s.PendingUpgrade {
		return invalid("pick an upgrade first")
	}
	if i < 0 || i >= len(s.Doors) {
		return invalid("door %d of %d", i, len(s.Doors))
	}
	d := s.Doors[i]
	if s.AtTime > 0 { // bought with at: the next door, whichever it is
		d.Timer = s.OpenTimer(d)
		e.logf("at: +%s on the next door", units.Clock(s.AtTime))
		s.AtTime = 0
	}
	s.Current = &Attempt{
		Door: d, Opened: e.now, Deadline: e.now.Add(d.Timer), Biome: s.Biome,
		Offer: e.choose("doors", i), LivesAtOpen: s.Lives, DucksAtOpen: s.Ducks,
	}
	s.Doors = nil
	s.Phase = PhaseProblem
	mods := doorMods(d)
	if mods != "" {
		mods = ", " + mods
	}
	e.logf("%s: open %s (%d, %s, %s%s)", e.door(), d.Problem, d.Rating, units.Clock(d.Timer), units.Bytes(d.Bytes), mods)
	e.emit(OpenProblem{Door: d})
	return nil
}

// ---- inside a problem ------------------------------------------------------

func (e *engine) tick() error {
	if e.s.Phase == PhaseProblem && !e.expired() {
		e.overtime(e.s.Current.Deadline)
	}
	return nil
}

// expired crashes the open door if its deadline has passed, and reports
// whether it did. Every in-problem action checks it first, so a late Tick
// can't be outrun by a retreat or a quack. A soft timer never crashes; the
// deadline only marks the attempt overtime, which Tick and the judge
// result do (they cannot fail, so the mark never sticks to a refused
// action). In a biome with rules.Biome.TimeoutLife (D44) a timeout may
// cost a life instead (saveTimeout); the door is closed either way.
func (e *engine) expired() bool {
	if e.now.Before(e.s.Current.Deadline) || e.s.Rules.OnExpire == rules.ExpireOvertime {
		return false
	}
	if !e.saveTimeout() {
		e.crash(TimedOut, false)
	}
	return true
}

// overtime marks the open attempt overtime once now reaches after, under a
// soft timer.
func (e *engine) overtime(after time.Time) {
	a := e.s.Current
	if e.s.Rules.OnExpire != rules.ExpireOvertime || a.Overtime || e.now.Before(after) {
		return
	}
	a.Overtime = true
	e.event(AttemptEvent{Kind: "overtime"})
	e.logf("%s: timer expired, overtime", e.door())
}

func (e *engine) submit(v SubmitResult) error {
	s := e.s
	if err := e.need(PhaseProblem, "submit"); err != nil {
		return err
	}
	a := s.Current
	a.SubmitAt = time.Time{}
	late := a.Deadline.Add(s.Rules.SubmitGrace.Duration)
	if !e.now.Before(late) && s.Rules.OnExpire != rules.ExpireOvertime {
		if !e.saveTimeout() {
			e.crash(TimedOut, false)
		}
		return nil
	}
	e.overtime(late)
	a.Submits++
	a.LastVerdict = v.Verdict
	if v.Total > 0 {
		a.Total = v.Total
	}
	a.BestPass = max(a.BestPass, v.Passed)
	e.event(AttemptEvent{Kind: "submit", Verdict: v.Verdict, Passed: v.Passed, Total: v.Total})
	if v.Verdict == Accepted {
		e.clear()
		return nil
	}
	a.Failed++
	if slices.Contains(s.Rules.FreeVerdicts, string(v.Verdict)) {
		e.logf("%s: %s, no charge", e.door(), v.Verdict)
		return nil
	}
	if a.Door.Cursed {
		// The curse takes the boot, not a life: lives left stay on the
		// record for the crash screen.
		e.logf("%s: %s %d/%d on a cursed door, %d lives left", e.door(), v.Verdict, v.Passed, v.Total, s.Lives)
		e.crash(OutOfLives, true)
		return nil
	}
	cost := 1
	if slices.Contains(s.Daemons, Nice) {
		cost = s.Rules.NiceLifeCost
	}
	s.Lives = max(0, s.Lives-cost)
	e.logf("%s: %s %d/%d, -%d life, %d left", e.door(), v.Verdict, v.Passed, v.Total, cost, s.Lives)
	if s.Lives == 0 {
		e.crash(OutOfLives, false)
		return nil
	}
	e.emit(Sound{Name: "wa"})
	return nil
}

// judging records a submit in flight. It is no judge of time: a submit
// made past the deadline is the verdict's to refuse, as before.
func (e *engine) judging(on bool) error {
	if err := e.need(PhaseProblem, "judging"); err != nil {
		return err
	}
	e.s.Current.SubmitAt = time.Time{}
	if on {
		e.s.Current.SubmitAt = e.now
	}
	return nil
}

func (e *engine) quack() error {
	s := e.s
	if err := e.need(PhaseProblem, "quack"); err != nil {
		return err
	}
	if e.expired() {
		return nil
	}
	a := s.Current
	switch {
	case a.Door.Cursed:
		return invalid("no ducks on a cursed door")
	case s.Ducks <= 0:
		return invalid("no ducks left")
	case a.Hint >= 2:
		return invalid("the duck has nothing left to say")
	}
	level := a.Hint + 1
	if level == 1 && !a.Door.TagsHidden() {
		level = 2 // tags are already on screen; don't spend a duck on them
	}
	s.Ducks--
	a.Quacks++
	a.Hint = level
	e.event(AttemptEvent{Kind: "quack", Level: level})
	what := map[int]string{1: "tags", 2: "technique"}[level]
	e.logf("%s: quack %d (%s), %d ducks left", e.door(), a.Quacks, what, s.Ducks)
	e.emit(RevealTags{Problem: a.Door.Problem, Level: level})
	return nil
}

func (e *engine) giveUp() error {
	if err := e.need(PhaseProblem, ":q!"); err != nil {
		return err
	}
	if e.expired() {
		return nil
	}
	e.crash(GaveUp, false)
	return nil
}

// retreat closes the door for a life (kexec: the first per biome is free)
// and re-rolls the room. It refuses rather than crash on the last life:
// retreating is a way out, not a way to die.
func (e *engine) retreat() error {
	s := e.s
	if err := e.need(PhaseProblem, "retreat"); err != nil {
		return err
	}
	if e.expired() {
		return nil
	}
	if s.Trial != nil {
		return invalid("a trial door can't be swapped")
	}
	free := slices.Contains(s.Daemons, Kexec) && !s.KexecUsed
	if !free && s.Lives <= 1 {
		return invalid("retreat needs a spare life")
	}
	name := e.door()
	a := e.close(Retreated)
	if free {
		s.KexecUsed = true
		e.logf("%s: kexec, free swap", name)
	} else {
		s.Lives--
		e.logf("%s: retreat, -1 life, %d left", name, s.Lives)
	}
	e.emit(CloseProblem{Door: a.Door})
	s.Phase = PhaseDoors
	e.rollRoom(false, "retreat")
	return nil
}

// close ends the open attempt with outcome, files it in History, teaches
// the aim calibration and rates it. A clear scores 1, a hinted or overtime
// one the lower of what each allows, a crash 0. A retreat is not rated: a
// swap is a choice of door (a free kexec swap two seconds in says nothing
// about skill), and rating it as a loss dropped every door's clear chance
// for taking it.
func (e *engine) close(o Outcome) Attempt {
	s := e.s
	a := s.Current
	a.Outcome, a.Closed = o, e.now
	a.SubmitAt = time.Time{} // a verdict still out is moot now
	score := 0.0
	if o == Cleared {
		score = 1
		if a.Quacks > 0 {
			score = s.Rules.HintedScore
		}
		if a.Overtime {
			score = min(score, s.Rules.OvertimeScore)
		}
	}
	calibrate(&s.Rules, e.meta, a.Door, o)
	if o != Retreated {
		rate(&s.Rules, e.meta, a.Door, score)
	}
	s.History = append(s.History, *a)
	s.Current = nil
	return *a
}

func (e *engine) clear() {
	s, m, r := e.s, e.meta, &e.s.Rules
	name := e.door()
	a := s.Current
	d := a.Door
	// A clock that stepped back (NTP, a hand-set date, a resume on another
	// machine) puts now before Opened. That is no evidence of speed: the
	// solve records 0, and -O3 wants a time that really passed.
	elapsed := max(e.now.Sub(a.Opened), 0)
	a.O3 = r.O3Frac > 0 && elapsed > 0 && elapsed <= scale(d.Timer, r.O3Frac)

	kind := Independent
	if a.Quacks > 0 {
		kind = Hinted
	}
	m.Solves = append(m.Solves, Solve{
		Problem: d.Problem,
		Kind:    kind,
		At:      e.now,
		Elapsed: elapsed,
		Boot:    s.Boot,
		Rating:  d.Rating,
		Gap:     d.Difficulty(r) - PlayerRating(r, m, d.Topic),
		Trial:   s.Trial != nil,
	})
	paid := s.Paid(*a)
	e.close(Cleared)
	advanceRegression(m, d.Problem, e.now)
	s.Uncommitted += paid
	m.LifetimeBytes += paid
	over := ""
	if a.Overtime {
		over = " (overtime)"
	}
	e.logf("%s: AC in %s, +%s%s", name, units.Clock(elapsed), units.Bytes(paid), over)
	e.emit(CloseProblem{Door: d})

	s.Room++
	more := s.Room < s.Biomes[s.Biome].Rooms
	if !more && s.Biome == len(s.Biomes)-1 {
		e.finish()
		return
	}
	picks := 1
	if a.O3 {
		picks++
		e.logf("%s: -O3, under %s, free upgrade", name, units.Clock(scale(d.Timer, r.O3Frac)))
	}
	e.emit(Sound{Name: "ac"})
	e.owe(picks)
	if more {
		s.Phase = PhaseDoors
		e.rollRoom(true, "fresh")
	} else {
		e.enterMerge()
	}
}

// crash ends the boot. The order is the rule: a core dump left by an earlier
// boot is destroyed first, then this boot's uncommitted bytes (if any)
// become the new dump. Crashing with nothing uncommitted still costs the old
// dump.
//
// A failed trial loses its uncommitted bytes but leaves no core dump and
// schedules no regression. A dump would destroy a real boot's shade over a
// practice failure. A regression would come back in boots, where a trial
// problem is rarely near the player's level, and take the boot's one
// regression slot; retrying the trial brings the unit back anyway.
//
// cursed marks a crash the curse caused rather than the lives running out;
// Cause is OutOfLives either way.
func (e *engine) crash(cause Outcome, cursed bool) {
	s, m := e.s, e.meta
	name := e.door()
	a := e.close(cause)
	d := a.Door

	var dumpLost int64
	lost := s.Uncommitted
	s.Uncommitted = 0
	if s.Trial != nil {
		if lost > 0 {
			e.logf("trial: %s lost, no core dump", units.Bytes(lost))
		}
	} else {
		if m.CoreDump != nil {
			dumpLost = m.CoreDump.Bytes
			e.logf("coredump: %s on %s overwritten", units.Bytes(dumpLost), m.CoreDump.Problem)
			m.CoreDump = nil
		}
		if lost > 0 {
			m.CoreDump = &CoreDump{Problem: d.Problem, Bytes: lost, Boot: s.Boot, At: e.now, Mult: s.Rules.DumpIndependentMult}
			e.logf("core dumped: %s on %s", units.Bytes(lost), d.Problem)
		}
		e.regress(d.Problem)
	}

	e.countEnd(true)
	m.LastPlayed = e.now
	c := Crash{
		Cause:    cause,
		Door:     d,
		Verdict:  a.LastVerdict,
		Lost:     lost,
		At:       e.now,
		Uptime:   e.now.Sub(s.Started),
		DumpLost: dumpLost,
		Cursed:   cursed,
	}
	s.Crash = &c
	s.Phase = PhaseCrashed
	why := map[Outcome]string{
		TimedOut:   "Kernel panic - not syncing: timer expired on " + name,
		OutOfLives: "Kernel panic - not syncing: out of lives on " + name,
		GaveUp:     "Kernel panic - not syncing: Attempted to kill init! (:q! on " + name + ")",
	}[cause]
	if cursed {
		why = "Kernel panic - not syncing: curse triggered on " + name
	}
	e.logf("%s", why)
	e.emit(CloseProblem{Door: d}, ShowCrash{Crash: c}, Sound{Name: "crash"})
	if cause == GaveUp {
		e.emit(OpenEditorial{Problem: d.Problem})
	}
}

// regress schedules id, which just cost the boot or a life, to return as
// a regression door.
func (e *engine) regress(id catalog.ID) {
	if due, ok := scheduleRegression(e.meta, id, e.now, e.s.Rules.RegressionDays); ok {
		e.logf("regression: %s due %s", id, due.Format(time.DateOnly))
	}
}

// ---- upgrades --------------------------------------------------------------

// owe adds n upgrade picks to what is owed (n < 0 pays) and draws the next
// pick's offers while any is owed. No pick is on offer when it is called.
func (e *engine) owe(n int) {
	s := e.s
	s.UpgradePicks += n
	s.PendingUpgrade = s.UpgradePicks > 0
	if s.PendingUpgrade {
		e.offerUpgrades()
	}
}

// offerUpgrades draws the pick. ECC is left out at full lives, where it
// would do nothing, so the pick can be one short, and so is an upgrade a
// package not installed locks. Packages can add offers.
func (e *engine) offerUpgrades() {
	s := e.s
	var pool []Upgrade
	for _, u := range AllUpgrades {
		if (u != ECC || s.Lives < s.MaxLives) && !s.UpgradeLocked(u) {
			pool = append(pool, u)
		}
	}
	s.UpgradeOffers = sample(e.rng, pool, s.Rules.UpgradeOffers+s.Rules.Grants(s.Packages).UpgradeOffers)
}

// pickUpgrade takes one owed pick. Picks are owed only outside a problem:
// a door cannot open with one pending, and the boot's last clear owes none.
func (e *engine) pickUpgrade(u Upgrade) error {
	s := e.s
	if !s.PendingUpgrade {
		return invalid("no upgrade pick pending")
	}
	if !slices.Contains(s.UpgradeOffers, u) {
		return invalid("upgrade %q not on offer", u)
	}
	s.Upgrades = append(s.Upgrades, u)
	switch u {
	case Overclock:
		// "Later doors" includes the ones on screen: the pick happens
		// before the door does.
		for i := range s.Doors {
			s.Doors[i].Timer = roundTimer(scale(s.Doors[i].Timer, 1+s.Rules.OverclockFrac))
		}
	case ECC:
		s.Lives = min(s.Lives+1, s.MaxLives)
	case Duck:
		s.Ducks++
	case ShufPick:
		s.Rerolls++
	}
	e.logf("upgrade: %s (lives %d, ducks %d, shufs %d)", u, s.Lives, s.Ducks, s.Rerolls)
	s.UpgradeOffers = nil
	e.owe(-1)
	return nil
}

// ---- merge window ----------------------------------------------------------

// enterMerge opens the window between biomes: ducks refill to the boot's
// base plus Duck upgrades, and daemons not yet running are offered, but
// not those a package not installed locks (rules.Package.Daemon) or that
// would do nothing (idle), and more with packages that add offers. A
// trial offers none: fork and kexec need rooms that re-roll, which a
// trial's do not, and the rest are boot perks.
func (e *engine) enterMerge() {
	s, r := e.s, &e.s.Rules
	s.Phase = PhaseMerge
	e.reachMerge()
	refill := e.baseDucks()
	for _, u := range s.Upgrades {
		if u == Duck {
			refill++
		}
	}
	s.Ducks = max(s.Ducks, refill)
	if len(s.Daemons) < r.MaxDaemons && s.Trial == nil {
		var pool []Daemon
		for _, d := range AllDaemons {
			if !slices.Contains(s.Daemons, d) && !s.DaemonLocked(d) && !s.idle(d) {
				pool = append(pool, d)
			}
		}
		s.DaemonOffers = sample(e.rng, pool, r.DaemonOffers+r.Grants(s.Packages).DaemonOffers)
	}
	e.logf("merge window: %s uncommitted, ducks %d, daemons on offer %v",
		units.Bytes(s.Uncommitted), s.Ducks, s.DaemonOffers)
}

// idle reports whether daemon d would change nothing in this boot: fork
// once the rooms hold its ForkDoors without it (tmux's door; the two do
// not stack, roomSize). In the simulator (every package installed, the
// 1000 and 1600 models at prestige 0 and 4, 1600 boots each) such a fork
// took a daemon slot from kexec or /dev/random: leaving it off the offer
// raised full clears by 3 to 9 points, while letting it stack a fifth
// door moved them by -1 to +3, within noise.
func (s *State) idle(d Daemon) bool {
	r := &s.Rules
	return d == Fork && r.DoorsPerRoom+r.Grants(s.Packages).Doors >= r.ForkDoors
}

func (e *engine) commit() error {
	if err := e.need(PhaseMerge, "commit"); err != nil {
		return err
	}
	if e.s.Uncommitted > 0 {
		e.bank()
		e.emit(Sound{Name: "commit"})
	}
	return nil
}

func (e *engine) bank() {
	s := e.s
	n := s.Uncommitted
	s.Committed += n
	e.meta.Bank += n
	s.Uncommitted = 0
	e.logf("commit: %s banked, bank %s", units.Bytes(n), units.Bytes(e.meta.Bank))
}

func (e *engine) pickDaemon(d Daemon) error {
	s := e.s
	if err := e.need(PhaseMerge, "pick daemon"); err != nil {
		return err
	}
	// A window offers daemons only below MaxDaemons, and only one pick.
	if !slices.Contains(s.DaemonOffers, d) {
		return invalid("daemon %q not on offer", d)
	}
	s.Daemons = append(s.Daemons, d)
	s.DaemonOffers = nil
	e.logf("daemon: %s started", d)
	return nil
}

func (e *engine) leaveMerge() error {
	s := e.s
	if err := e.need(PhaseMerge, "leave merge window"); err != nil {
		return err
	}
	if s.PendingUpgrade {
		return invalid("pick an upgrade first")
	}
	if s.Uncommitted > 0 {
		e.logf("merge window: leaving %s uncommitted", units.Bytes(s.Uncommitted))
	}
	s.DaemonOffers = nil
	e.enterBiome(s.Biome + 1)
	return nil
}

// ---- biomes and the end ----------------------------------------------------

func (e *engine) enterBiome(i int) {
	s := e.s
	s.Biome, s.Room, s.KexecUsed = i, 0, false
	b := &s.Biomes[i]
	label := b.Name
	if label == "" {
		label = string(b.Kind)
	}
	aim := fmt.Sprintf(", aim %.0f%%", s.AimClear(i)*100)
	if s.Trial != nil {
		aim = "" // trial doors are not aimed
	}
	if s.Trial == nil && s.Rules.Biomes[i].TimeoutLife {
		aim += ", a timeout costs a life, not the boot" // D44, announced
	}
	e.logf("biome_%d: %s, %d rooms%s", i, label, b.Rooms, aim)
	if b.Kind == rules.Choice && b.Topic == "" {
		s.Phase = PhaseRoute
		e.logf("route: biome_%d offers %v", i, b.Offers)
		return
	}
	s.Phase = PhaseDoors
	if b.Kind == rules.Boss {
		e.logf("boss: %s", b.Boss)
		e.emit(Sound{Name: "boss"})
	}
	e.rollRoom(true, "fresh")
}

// finish clears the boot: what is still uncommitted is banked, and beating
// the top unlocked prestige unlocks the next. A trial unlocks no prestige;
// it grants its capability instead.
func (e *engine) finish() {
	s, m := e.s, e.meta
	if s.Uncommitted > 0 {
		e.bank()
	}
	s.Phase = PhaseCleared
	e.countEnd(false)
	m.LastPlayed = e.now
	switch {
	case s.Trial != nil:
		e.passTrial()
	// Not before the engine knows the player (PrestigeGate).
	case s.Prestige == m.Prestige && NextPrestige(s.Rules, m).Open():
		m.Prestige++
		s.Unlocked = m.Prestige
		e.logf("prestige %d unlocked", m.Prestige)
	case s.Prestige == m.Prestige && m.Prestige < MaxPrestige:
		g := NextPrestige(s.Rules, m)
		e.logf("prestige %d locked until %d rated doors (%d so far)", g.Next, g.Need, g.Rated)
	}
	e.logf("boot #%d: cleared in %s, %s committed", s.Boot, units.Clock(e.now.Sub(s.Started)), units.Bytes(s.Committed))
	e.emit(Sound{Name: "cleared"}, ShowCleared{})
}

// ---- small helpers ---------------------------------------------------------

// Earned is every byte cleared doors paid this boot: Committed + Uncommitted
// + Spent + the crash's Lost, always.
func (s *State) Earned() int64 {
	var n int64
	for _, a := range s.History {
		if a.Outcome == Cleared {
			n += s.Paid(a)
		}
	}
	return n
}

// Paid is what clearing a's door pays (or paid): its Bytes, scaled by
// OvertimeBytesMult when the clear came in overtime.
func (s *State) Paid(a Attempt) int64 {
	if !a.Overtime {
		return a.Door.Bytes
	}
	return max(1, int64(math.Round(float64(a.Door.Bytes)*s.Rules.OvertimeBytesMult)))
}

// SoftTimer reports whether door timers are soft (rules.ExpireOvertime):
// past the deadline the HUD counts overtime up instead of crashing.
func (s *State) SoftTimer() bool { return s.Rules.OnExpire == rules.ExpireOvertime }

// ShowsFailingInput reports whether the HUD may show a failing full test
// (input, expected and actual output) and :Debug may run it. A running boot
// hides it unless strace runs or the rules show it (rules.Trial): a failing
// case on screen turns a wrong answer into a diff to match, which is the
// perk strace sells. A nil State (practice, an upsolve) and an ended boot
// show it, as nothing is at stake.
func (s *State) ShowsFailingInput() bool {
	if s == nil || s.Phase == PhaseCrashed || s.Phase == PhaseCleared {
		return true
	}
	return s.Rules.ShowFailingInput || slices.Contains(s.Daemons, Strace)
}
