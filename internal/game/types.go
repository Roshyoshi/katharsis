// Package game is the rules engine: pure, deterministic for a given seed,
// and free of I/O. The app feeds it Actions (player choices, judge results,
// clock ticks) and carries out the Effects it returns (open a problem, show
// the crash screen, play a sound). Screens read State and Meta; they never
// mutate them.
//
// This file is the contract shared by the engine, the screens and the store.
// Changes must be additive; renaming or removing a field breaks other
// packages built against it.
package game

import (
	"time"

	"deadcases/internal/catalog"
	"deadcases/internal/rules"
)

// ---- persistent player state (survives boots) ------------------------------

type SolveKind string

const (
	Independent SolveKind = "independent" // no ducks, no editorial
	Hinted      SolveKind = "hinted"      // used a duck
	Editorial   SolveKind = "editorial"   // after :q! or upsolving a crash
)

type Solve struct {
	Problem catalog.ID
	Kind    SolveKind
	At      time.Time
	Elapsed time.Duration // from door open to AC
	Boot    int

	// Rating is the problem's rating and Gap that minus the player's
	// rating when it was solved (0 when unknown); timer fitting moves
	// solve times between gaps with them. Upsolve marks a solve made
	// outside a boot, which timer fitting ignores.
	Rating  int
	Gap     float64
	Upsolve bool
}

// CoreDump is the shade: bytes left on the problem that crashed you. Upsolve
// Problem to recover Bytes; crashing in a later boot first loses them.
type CoreDump struct {
	Problem catalog.ID
	Bytes   int64
	Boot    int
	At      time.Time
}

// Regression is a problem due to come back as a door.
type Regression struct {
	Problem catalog.ID
	Due     time.Time
	Stage   int // index into rules.RegressionDays

	// Days is the boot's RegressionDays when this was scheduled, so an
	// upsolve between boots (no rules at hand) can advance it.
	Days []int
}

type Lib struct {
	Name   string // "dsu", "segtree", ...
	Source string // C++ snippet, pasted into the template when linked
}

type Meta struct {
	Rating        float64            // CF-scale estimate of the player
	TopicRating   map[string]float64 // per catalog topic, same scale
	Prestige      int                // highest unlocked prestige level
	Bank          int64              // committed bytes available to spend
	LifetimeBytes int64

	Capabilities []string // e.g. "CAP_DP"; unlock topic biomes
	Libs         []Lib
	CoreDump     *CoreDump
	Regressions  []Regression
	Solves       []Solve
	Boots        int // boots started, for "Boot #N"

	// TopicAttempts counts rated attempts per topic; a topic's rating aims
	// doors once it reaches rules.TopicRatingMin.
	TopicAttempts map[string]int
}

// ---- one boot ----------------------------------------------------------------

type Phase string

const (
	PhaseRoute   Phase = "route"   // choosing the next biome's topic
	PhaseDoors   Phase = "doors"   // choosing a door
	PhaseProblem Phase = "problem" // a door is open, timer running
	PhaseMerge   Phase = "merge"   // between biomes: commit, upgrade, daemon
	PhaseCrashed Phase = "crashed"
	PhaseCleared Phase = "cleared" // beat the last boss
)

type Door struct {
	Problem catalog.ID
	Topic   string
	Rating  int
	Timer   time.Duration
	Bytes   int64   // reward on AC, multipliers applied
	SolveP  float64 // engine's estimate of the player's chance

	Blind      bool   // tags hidden
	Mystery    bool   // title hidden too
	Cursed     bool   // no ducks; one failed submit crashes
	Regression bool   // a past crash returning
	Boss       string // key from BossKeys (or FinalBoss), "" for normal doors
}

type BiomeState struct {
	Name   string
	Topic  string
	Kind   rules.BiomeKind
	Rooms  int
	Offers []string // topics on offer at the entrance (Choice only)

	// Boss is the boss key of a Boss biome (BossKeys or FinalBoss), set at
	// boot so the map can name it before it is reached.
	Boss string
}

// Outcome of one attempt.
type Outcome string

const (
	Cleared    Outcome = "cleared"
	TimedOut   Outcome = "timed-out"
	OutOfLives Outcome = "out-of-lives"
	GaveUp     Outcome = "gave-up"   // :q!
	Retreated  Outcome = "retreated" // swapped the door for a life
)

type Attempt struct {
	Door     Door
	Opened   time.Time
	Deadline time.Time
	Closed   time.Time
	Outcome  Outcome // "" while open

	Submits     int
	Failed      int // submits that were not AC
	Quacks      int // ducks used on this door (1: tags, 2: technique)
	BestPass    int // most tests passed by one submit (the HP bar)
	Total       int // tests in the full set
	LastVerdict string

	// Hint is the highest RevealTags level shown. On a door whose tags are
	// already visible the first quack goes straight to level 2, so Hint
	// can run ahead of Quacks.
	Hint int
	// O3 is set when the clear beat O3Frac of the timer (a free upgrade).
	O3 bool
}

// Upgrade is picked after each cleared door.
type Upgrade string

const (
	Overclock Upgrade = "overclock" // +OverclockFrac timer on later doors
	ECC       Upgrade = "ecc"       // +1 life
	Duck      Upgrade = "duck"      // +1 duck
)

// Daemon is a passive perk picked at a merge window.
type Daemon string

const (
	Strace    Daemon = "strace"     // failing test's input is shown
	Nice      Daemon = "nice"       // +20% timer, a failed submit costs 2 lives
	Fork      Daemon = "fork"       // 4 doors per room after an AC
	DevRandom Daemon = "dev-random" // mystery doors pay 2x
	Kexec     Daemon = "kexec"      // one free door swap per biome
)

type Crash struct {
	Cause   Outcome // TimedOut, OutOfLives, GaveUp
	Door    Door
	Verdict string // last verdict on the door, for quote matching
	Lost    int64  // uncommitted bytes moved into the core dump
	At      time.Time
	Uptime  time.Duration

	// DumpLost is the core dump from an earlier boot that this crash
	// destroyed before writing its own.
	DumpLost int64
}

type State struct {
	Boot     int
	Seed     uint64
	Rules    rules.Rules // snapshot; never re-read during a boot
	Prestige int
	Started  time.Time

	Phase  Phase
	Biomes []BiomeState
	Biome  int // index into Biomes
	Room   int // room index within the biome

	Doors   []Door // doors on offer (PhaseDoors)
	Current *Attempt
	History []Attempt

	Lives, MaxLives int
	Ducks           int
	Uncommitted     int64 // lost on crash
	Committed       int64 // banked this boot (also added to Meta.Bank)

	Upgrades       []Upgrade
	Daemons        []Daemon
	Linked         []string  // lib names linked this boot
	PendingUpgrade bool      // a cleared door earned an upgrade pick
	UpgradeOffers  []Upgrade // what the pending pick offers
	DaemonOffers   []Daemon  // what the current merge window offers
	KexecUsed      bool      // per biome

	Crash *Crash
	Log   []LogLine // dmesg

	// RNG is the PRNG state (splitmix64). Every random draw advances it,
	// so a boot saved mid-run resumes the same sequence.
	RNG uint64
	// UpgradePicks is how many upgrade picks are owed (an -O3 clear owes
	// two); PendingUpgrade is UpgradePicks > 0.
	UpgradePicks int
	// RegressionOffered: a boot offers at most one regression door.
	RegressionOffered bool
}

type LogLine struct {
	At   time.Time
	Text string
}

// ---- inputs ------------------------------------------------------------------

type Action interface{ isAction() }

type ChooseRoute struct{ Index int } // PhaseRoute
type ChooseDoor struct{ Index int }  // PhaseDoors
type Retreat struct{}                // PhaseProblem: swap the door, costs a life (kexec: free once)
type Quack struct{}                  // PhaseProblem: use a duck
type GiveUp struct{}                 // PhaseProblem: :q!

// SubmitResult reports a full-test judge of the open door.
type SubmitResult struct {
	Verdict string // judge.Verdict as string
	Passed  int
	Total   int
}

type Tick struct{} // the clock moved; checks the deadline

// Merge window choices.
type Commit struct{} // bank Uncommitted
type PickUpgrade struct{ Upgrade Upgrade }
type PickDaemon struct{ Daemon Daemon }
type LeaveMerge struct{}

func (ChooseRoute) isAction()  {}
func (ChooseDoor) isAction()   {}
func (Retreat) isAction()      {}
func (Quack) isAction()        {}
func (GiveUp) isAction()       {}
func (SubmitResult) isAction() {}
func (Tick) isAction()         {}
func (Commit) isAction()       {}
func (PickUpgrade) isAction()  {}
func (PickDaemon) isAction()   {}
func (LeaveMerge) isAction()   {}

// ---- outputs -----------------------------------------------------------------

type Effect interface{ isEffect() }

type OpenProblem struct{ Door Door }  // materialize + open editor/HUD
type CloseProblem struct{ Door Door } // tear down editor/HUD
type RevealTags struct {
	Problem catalog.ID
	Level   int
} // 1 tags, 2 technique
type OpenEditorial struct{ Problem catalog.ID }
type ShowCrash struct{ Crash Crash }
type ShowCleared struct{}
type Sound struct{ Name string }              // "ac", "wa", "crash", "commit", "boss"
type Prefetch struct{ Problems []catalog.ID } // doors on offer: fetch ahead

func (OpenProblem) isEffect()   {}
func (CloseProblem) isEffect()  {}
func (RevealTags) isEffect()    {}
func (OpenEditorial) isEffect() {}
func (ShowCrash) isEffect()     {}
func (ShowCleared) isEffect()   {}
func (Sound) isEffect()         {}
func (Prefetch) isEffect()      {}

// BossKeys name the bosses of non-final boss biomes; flavor maps a key to a
// display name and blurb. FinalBoss is the last boss at the highest
// prestige.
var BossKeys = []string{"off-by-one", "heisenbug", "oom-killer", "race-condition", "undefined-behavior"}

const FinalBoss = "halting-problem"

// MaxPrestige is the highest prestige level (0 Bronze ... 4 Advanced).
const MaxPrestige = 4
