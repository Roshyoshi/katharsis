package game

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"deadcases/internal/catalog"
	"deadcases/internal/rules"
)

// ErrInvalid wraps every error Step returns for an action the current phase
// does not allow. The state is unchanged when Step returns it.
var ErrInvalid = errors.New("invalid action")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalid}, args...)...)
}

// AllUpgrades and AllDaemons are what picks draw from.
var (
	AllUpgrades = []Upgrade{Overclock, ECC, Duck}
	AllDaemons  = []Daemon{Strace, Nice, Fork, DevRandom, Kexec}
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

	meta.Boots++
	if meta.Rating <= 0 {
		meta.Rating = r.ColdStartRating
	}
	asc := r.Ascend(prestige)
	s := &State{
		Boot:     meta.Boots,
		Seed:     seed,
		RNG:      seed,
		Rules:    r,
		Prestige: prestige,
		Started:  now,
	}
	s.Lives = max(1, r.Lives+asc.Lives)
	s.MaxLives = max(r.MaxLives, s.Lives)
	s.Ducks = max(0, r.Ducks+asc.Ducks)

	e := newEngine(s, meta, pool, now)
	e.logf("boot #%d: prestige %d, rules %s, seed %#x, rating %.0f", s.Boot, prestige, r.Name, seed, meta.Rating)
	e.layout()
	e.enterBiome(0)
	return s, e.fx, nil
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
	b.Topic = b.Offers[i]
	e.logf("route: biome_%d takes %s", e.s.Biome, b.Topic)
	e.s.Phase = PhaseDoors
	e.rollRoom(true)
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
	s.Current = &Attempt{Door: d, Opened: e.now, Deadline: e.now.Add(d.Timer)}
	s.Doors = nil
	s.Phase = PhaseProblem
	mods := doorMods(d)
	if mods != "" {
		mods = ", " + mods
	}
	e.logf("%s: open %s (%d, %s, %s%s)", e.door(), d.Problem, d.Rating, clock(d.Timer), fmtBytes(d.Bytes), mods)
	e.emit(OpenProblem{Door: d})
	return nil
}

// ---- inside a problem ------------------------------------------------------

func (e *engine) tick() error {
	if e.s.Phase == PhaseProblem {
		e.expired()
	}
	return nil
}

// expired crashes the open door if its deadline has passed, and reports
// whether it did. Every in-problem action checks it first, so a late Tick
// can't be outrun by a retreat or a quack.
func (e *engine) expired() bool {
	if e.now.Before(e.s.Current.Deadline) {
		return false
	}
	e.crash(TimedOut)
	return true
}

func (e *engine) submit(v SubmitResult) error {
	s := e.s
	if err := e.need(PhaseProblem, "submit"); err != nil {
		return err
	}
	a := s.Current
	if !e.now.Before(a.Deadline.Add(s.Rules.SubmitGrace.Duration)) {
		e.crash(TimedOut)
		return nil
	}
	a.Submits++
	a.LastVerdict = v.Verdict
	if v.Total > 0 {
		a.Total = v.Total
	}
	a.BestPass = max(a.BestPass, v.Passed)
	if v.Verdict == "AC" {
		e.clear()
		return nil
	}
	a.Failed++
	if slices.Contains(s.Rules.FreeVerdicts, v.Verdict) {
		e.logf("%s: %s, no charge", e.door(), v.Verdict)
		return nil
	}
	cost := 1
	if slices.Contains(s.Daemons, Nice) {
		cost = s.Rules.NiceLifeCost
	}
	s.Lives = max(0, s.Lives-cost)
	e.logf("%s: %s %d/%d, -%d life, %d left", e.door(), v.Verdict, v.Passed, v.Total, cost, s.Lives)
	if a.Door.Cursed || s.Lives == 0 {
		e.crash(OutOfLives)
		return nil
	}
	e.emit(Sound{Name: "wa"})
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
	if level == 1 && !a.Door.Blind {
		level = 2 // tags are already on screen; don't spend a duck on them
	}
	s.Ducks--
	a.Quacks++
	a.Hint = level
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
	e.crash(GaveUp)
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
	e.rollRoom(false)
	return nil
}

// close ends the open attempt with outcome, files it in History and rates
// it. Everything but a clear scores 0.
func (e *engine) close(o Outcome) Attempt {
	s := e.s
	a := s.Current
	a.Outcome, a.Closed = o, e.now
	score := 0.0
	if o == Cleared {
		score = 1
		if a.Quacks > 0 {
			score = s.Rules.HintedScore
		}
	}
	rate(&s.Rules, e.meta, a.Door, score)
	s.History = append(s.History, *a)
	s.Current = nil
	return *a
}

func (e *engine) clear() {
	s, m, r := e.s, e.meta, &e.s.Rules
	name := e.door()
	a := s.Current
	d := a.Door
	elapsed := e.now.Sub(a.Opened)
	a.O3 = r.O3Frac > 0 && elapsed <= scale(d.Timer, r.O3Frac)

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
		Gap:     float64(d.Rating) - PlayerRating(r, m, d.Topic),
	})
	e.close(Cleared)
	advanceRegression(m, d.Problem, e.now)
	s.Uncommitted += d.Bytes
	m.LifetimeBytes += d.Bytes
	e.logf("%s: AC in %s, +%s", name, clock(elapsed), fmtBytes(d.Bytes))
	e.emit(CloseProblem{Door: d})

	picks := 1
	if a.O3 {
		picks++
		e.logf("%s: -O3, under %s, free upgrade", name, clock(scale(d.Timer, r.O3Frac)))
	}
	s.Room++
	switch {
	case s.Room < s.Biomes[s.Biome].Rooms:
		e.emit(Sound{Name: "ac"})
		e.owe(picks)
		s.Phase = PhaseDoors
		e.rollRoom(true)
	case s.Biome == len(s.Biomes)-1:
		e.finish()
	default:
		e.emit(Sound{Name: "ac"})
		e.owe(picks)
		e.enterMerge()
	}
}

// crash ends the boot. The order is the rule: a core dump left by an earlier
// boot is destroyed first, then this boot's uncommitted bytes (if any)
// become the new dump. Crashing with nothing uncommitted still costs the old
// dump.
func (e *engine) crash(cause Outcome) {
	s, m := e.s, e.meta
	name := e.door()
	a := e.close(cause)
	d := a.Door

	var dumpLost int64
	if m.CoreDump != nil {
		dumpLost = m.CoreDump.Bytes
		e.logf("coredump: %s on %s overwritten", fmtBytes(dumpLost), m.CoreDump.Problem)
		m.CoreDump = nil
	}
	lost := s.Uncommitted
	if lost > 0 {
		m.CoreDump = &CoreDump{Problem: d.Problem, Bytes: lost, Boot: s.Boot, At: e.now}
		e.logf("core dumped: %s on %s", fmtBytes(lost), d.Problem)
	}
	s.Uncommitted = 0
	if due, ok := scheduleRegression(m, d.Problem, e.now, s.Rules.RegressionDays); ok {
		e.logf("regression: %s due %s", d.Problem, due.Format(time.DateOnly))
	}

	c := Crash{
		Cause:    cause,
		Door:     d,
		Verdict:  a.LastVerdict,
		Lost:     lost,
		At:       e.now,
		Uptime:   e.now.Sub(s.Started),
		DumpLost: dumpLost,
	}
	s.Crash = &c
	s.Phase = PhaseCrashed
	e.dropPicks()
	why := map[Outcome]string{
		TimedOut:   "Kernel panic - not syncing: timer expired on " + name,
		OutOfLives: "Kernel panic - not syncing: out of lives on " + name,
		GaveUp:     "Kernel panic - not syncing: Attempted to kill init! (:q! on " + name + ")",
	}[cause]
	e.logf("%s", why)
	e.emit(CloseProblem{Door: d}, ShowCrash{Crash: c}, Sound{Name: "crash"})
	if cause == GaveUp {
		e.emit(OpenEditorial{Problem: d.Problem})
	}
}

// ---- upgrades --------------------------------------------------------------

func (e *engine) owe(n int) {
	s := e.s
	s.UpgradePicks += n
	s.PendingUpgrade = s.UpgradePicks > 0
	if s.PendingUpgrade && len(s.UpgradeOffers) == 0 {
		e.offerUpgrades()
	}
}

// offerUpgrades draws the pick. ECC is left out at full lives, where it
// would do nothing, so the pick can be one short.
func (e *engine) offerUpgrades() {
	s := e.s
	var pool []Upgrade
	for _, u := range AllUpgrades {
		if u != ECC || s.Lives < s.MaxLives {
			pool = append(pool, u)
		}
	}
	s.UpgradeOffers = sample(e.rng, pool, s.Rules.UpgradeOffers)
}

func (e *engine) dropPicks() {
	e.s.UpgradePicks, e.s.PendingUpgrade, e.s.UpgradeOffers = 0, false, nil
	e.s.Doors, e.s.DaemonOffers = nil, nil
}

func (e *engine) pickUpgrade(u Upgrade) error {
	s := e.s
	if !s.PendingUpgrade || s.Phase == PhaseProblem || s.Phase == PhaseCrashed || s.Phase == PhaseCleared {
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
	}
	e.logf("upgrade: %s (lives %d, ducks %d)", u, s.Lives, s.Ducks)
	s.UpgradePicks--
	s.UpgradeOffers = nil
	s.PendingUpgrade = s.UpgradePicks > 0
	if s.PendingUpgrade {
		e.offerUpgrades()
	}
	return nil
}

// ---- merge window ----------------------------------------------------------

// enterMerge opens the window between biomes: ducks refill to the boot's
// base plus Duck upgrades, and daemons not yet running are offered.
func (e *engine) enterMerge() {
	s, r := e.s, &e.s.Rules
	s.Phase = PhaseMerge
	s.Doors = nil
	refill := max(0, r.Ducks+r.Ascend(s.Prestige).Ducks)
	for _, u := range s.Upgrades {
		if u == Duck {
			refill++
		}
	}
	s.Ducks = max(s.Ducks, refill)
	s.DaemonOffers = nil
	if len(s.Daemons) < r.MaxDaemons {
		var pool []Daemon
		for _, d := range AllDaemons {
			if !slices.Contains(s.Daemons, d) {
				pool = append(pool, d)
			}
		}
		s.DaemonOffers = sample(e.rng, pool, r.DaemonOffers)
	}
	e.logf("merge window: %s uncommitted, ducks %d, daemons on offer %v",
		fmtBytes(s.Uncommitted), s.Ducks, s.DaemonOffers)
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
	e.logf("commit: %s banked, bank %s", fmtBytes(n), fmtBytes(e.meta.Bank))
}

func (e *engine) pickDaemon(d Daemon) error {
	s := e.s
	if err := e.need(PhaseMerge, "pick daemon"); err != nil {
		return err
	}
	if !slices.Contains(s.DaemonOffers, d) {
		return invalid("daemon %q not on offer", d)
	}
	if len(s.Daemons) >= s.Rules.MaxDaemons {
		return invalid("already running %d daemons", len(s.Daemons))
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
		e.logf("merge window: leaving %s uncommitted", fmtBytes(s.Uncommitted))
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
	rb := s.Rules.Biomes[i]
	label := b.Name
	if label == "" {
		label = string(b.Kind)
	}
	e.logf("biome_%d: %s, %d rooms, aim %.0f%%", i, label, b.Rooms, rb.TargetClear*100)
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
	e.rollRoom(true)
}

// finish clears the boot: what is still uncommitted is banked, and beating
// the top unlocked prestige unlocks the next.
func (e *engine) finish() {
	s, m := e.s, e.meta
	if s.Uncommitted > 0 {
		e.bank()
	}
	s.Phase = PhaseCleared
	e.dropPicks()
	if s.Prestige == m.Prestige && m.Prestige < MaxPrestige {
		m.Prestige++
		e.logf("prestige %d unlocked", m.Prestige)
	}
	e.logf("boot #%d: cleared in %s, %s committed", s.Boot, clock(e.now.Sub(s.Started)), fmtBytes(s.Committed))
	e.emit(Sound{Name: "cleared"}, ShowCleared{})
}

// ---- small helpers ---------------------------------------------------------

// Earned is every byte cleared doors paid this boot: Committed + Uncommitted
// + the crash's Lost, always.
func (s *State) Earned() int64 {
	var n int64
	for _, a := range s.History {
		if a.Outcome == Cleared {
			n += a.Door.Bytes
		}
	}
	return n
}

// clock formats a duration as m:ss or h:mm:ss, like the HUD.
func clock(d time.Duration) string {
	d = d.Round(time.Second)
	h, m, sec := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%d:%02d", m, sec)
}

// fmtBytes shows bytes in B, KiB or MiB (DESIGN.md's currency).
func fmtBytes(n int64) string {
	switch {
	case n < 1<<10:
		return fmt.Sprintf("%dB", n)
	case n < 1<<20:
		return fmt.Sprintf("%.1fKiB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%.1fMiB", float64(n)/(1<<20))
}
