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

	"katharsis/internal/catalog"
	"katharsis/internal/rules"
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

	// Trial marks a solve made in a learn-mode trial. Timer fitting ignores
	// it too: a soft timer and a module just read are not boot pace.
	Trial bool
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

	// Seen lists the one-time story scenes already shown, each once, in
	// the order shown (MarkSeen). It is the story's reread log.
	Seen []SeenScene

	// TopicAttempts counts rated attempts per topic; a topic's rating aims
	// doors once it reaches rules.TopicRatingMin.
	TopicAttempts map[string]int

	// Rated counts every rated attempt; the first
	// rules.RatingColdAttempts move the ratings with rules.RatingKCold.
	Rated int
	// Calibration corrects door aim per tier, keyed by CalibrationTier.
	Calibration map[string]Calibration
	// TrialsPassed lists the modules (TrialSpec.Module) whose trial was
	// cleared, each once, in the order first passed.
	TrialsPassed []string

	// Installed lists the packages (rules.Package) bought with the bank,
	// each once, in the order installed; Theme is the installed banner theme
	// in use, "" for the default.
	Installed []string
	Theme     string
}

// Calibration is what one tier of biomes (see CalibrationTier) has learned
// about aiming its doors. The engine aims a door where the logistic says
// the player clears it with the biome's TargetClear; real solve curves are
// flatter than the logistic and have a ceiling (timers, wrong submits), so
// that aim is off by an amount that differs per tier. Offset is added to
// the tier's door targets to correct it; Observed and Expected are the
// recent clear rate of the tier's plain doors and what the engine expected
// of them, for display.
type Calibration struct {
	Offset   float64
	Doors    int // plain doors learned from
	Observed float64
	Expected float64
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

	// Mastered hides the tags because the player has mastered the door's
	// topic (rules.MasteryAttempts). Unlike Blind it is no modifier: the
	// door pays and teaches like a plain one. Never set with Blind.
	Mastered bool

	// Tier is the calibration tier the door's outcome teaches, "" when it
	// teaches none: a door with a modifier (blind, mystery, cursed), a
	// regression, a trial door, or one the pool could not place near its
	// target. SolveP already includes the tier's offset.
	Tier string
}

// TagsHidden reports whether the door hides its tags until a duck reveals
// them: a blind (or mystery) door, or one on a mastered topic.
func (d Door) TagsHidden() bool { return d.Blind || d.Mastered }

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

// Verdict is a full-test judge verdict, with judge.Verdict's values ("AC",
// "WA", "TLE", "RE", "MLE", "CE", ...). The engine acts on Accepted and on
// rules.FreeVerdicts; it keeps the rest for the screens and the crash
// quote.
type Verdict string

const Accepted Verdict = "AC"

type Attempt struct {
	Door     Door
	Opened   time.Time
	Deadline time.Time
	Closed   time.Time
	Outcome  Outcome // "" while open

	// Biome is the index into State.Biomes of the biome the door opened in.
	Biome int

	Submits     int
	Failed      int // submits that were not AC
	Quacks      int // ducks used on this door (1: tags, 2: technique)
	BestPass    int // most tests passed by one submit (the HP bar)
	Total       int // tests in the full set
	LastVerdict Verdict

	// Hint is the highest RevealTags level shown. On a door whose tags are
	// already visible the first quack goes straight to level 2, so Hint
	// can run ahead of Quacks.
	Hint int
	// O3 is set when the clear beat O3Frac of the timer (a free upgrade).
	O3 bool
	// Overtime is set when the timer ran out under rules.ExpireOvertime and
	// the attempt went on; a clear then pays and rates less.
	Overtime bool
	// SubmitAt is when the submit being judged was made, zero when none
	// is (Judging). Saved before the verdict, it lets a game that quit
	// mid-judge judge that submit again on resume, from that time.
	SubmitAt time.Time `json:",omitzero"`
}

// Upgrade is picked after each cleared door.
type Upgrade string

const (
	Overclock Upgrade = "overclock" // +OverclockFrac timer on later doors
	ECC       Upgrade = "ecc"       // +1 life
	Duck      Upgrade = "duck"      // +1 duck
	ShufPick  Upgrade = "shuf"      // +1 shuf (locked until coreutils)
)

// Daemon is a passive perk picked at a merge window.
type Daemon string

const (
	Strace    Daemon = "strace"     // failing test's input is shown
	Nice      Daemon = "nice"       // +20% timer, a failed submit costs 2 lives
	Fork      Daemon = "fork"       // 4 doors per room after an AC
	DevRandom Daemon = "dev-random" // mystery doors pay 2x
	Kexec     Daemon = "kexec"      // one free door swap per biome
	Cron      Daemon = "cron"       // a regression door pays 2x
	Chrt      Daemon = "chrt"       // -20% timer, every door pays 1.5x
)

// Item is what a merge window sells for uncommitted bytes (D25): bought
// for this boot with bytes that committing would have banked.
type Item string

const (
	Fsck Item = "fsck" // +1 life, up to MaxLives
	Dup  Item = "dup"  // +1 duck
	Shuf Item = "shuf" // one re-roll of a room's doors (Reroll), kept until used

	At Item = "at" // more time on the next door's timer; sold once installed
)

type Crash struct {
	Cause   Outcome // TimedOut, OutOfLives, GaveUp
	Door    Door
	Verdict Verdict // last verdict on the door, for quote matching
	Lost    int64   // uncommitted bytes moved into the core dump
	At      time.Time
	Uptime  time.Duration

	// DumpLost is the core dump from an earlier boot that this crash
	// destroyed before writing its own.
	DumpLost int64

	// Cursed is set when a failed submit on a cursed door ended the boot:
	// the curse, not the lives, which it does not charge (State.Lives has
	// what was left). Cause is OutOfLives then too, the failed-submit
	// family that screens and quotes group by.
	Cursed bool
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

	// Spent is the uncommitted bytes merge windows sold items for; every
	// byte earned is committed, uncommitted, spent or lost to the crash.
	Spent int64
	// Rerolls is how many shufs are bought and not yet used.
	Rerolls int
	// AtTime is the time at bought for the next door that opens.
	AtTime time.Duration
	// Packages is Meta.Installed when the boot began: what it plays with.
	Packages []string
	// Unlocked is the prestige this boot unlocked by its clear, 0 if none.
	Unlocked int

	// Trial is set when this boot is a learn-mode trial (NewTrial).
	Trial *TrialState
}

// TrialSpec is a learn-mode trial as the curriculum builds it
// (curriculum.Trial): the module's unit problems, Easy to Hard, and where
// to draw the blind integration doors from.
type TrialSpec struct {
	Module, Title string
	Unit          []catalog.ID
	IntegTopic    string // "" or Mixed: any topic
	IntegBand     [2]int // CF-scale ratings, inclusive; {0, 0}: any rating
	Capability    string // granted on a clear; "" grants nothing
}

// TrialState is a trial in progress. Biome 0 is the unit and biome 1 the
// integration; Plan holds each biome's problems, one per room, so the map
// can show the whole trial up front.
type TrialState struct {
	TrialSpec
	Plan [][]catalog.ID
	// TopUp lists the unit problems drawn from the pool because Unit was
	// shorter than the trial rules' unit biome.
	TopUp []catalog.ID
	// Passed is set on a clear; Granted is the capability it added to Meta
	// ("" when Capability was empty or already held).
	Passed  bool
	Granted string
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
	Verdict Verdict
	Passed  int
	Total   int
}

// Judging reports that a submit of the open door started being judged
// (On; Attempt.SubmitAt is now) or ended without a verdict (a judge
// error). A SubmitResult ends it too.
type Judging struct{ On bool }

type Tick struct{} // the clock moved; checks the deadline

// Merge window choices.
type Commit struct{} // bank Uncommitted
type PickUpgrade struct{ Upgrade Upgrade }
type PickDaemon struct{ Daemon Daemon }
type LeaveMerge struct{}

// Spend buys an item with uncommitted bytes (PhaseMerge).
type Spend struct{ Item Item }

// Reroll spends a shuf on the room on offer (PhaseDoors, not a boss room).
type Reroll struct{}

// Link chooses the libs (by name, from Meta.Libs) pasted into this boot's
// templates. Allowed until the first door opens, up to State.LinkSlots.
type Link struct{ Libs []string }

func (ChooseRoute) isAction()  {}
func (ChooseDoor) isAction()   {}
func (Retreat) isAction()      {}
func (Quack) isAction()        {}
func (GiveUp) isAction()       {}
func (SubmitResult) isAction() {}
func (Judging) isAction()      {}
func (Tick) isAction()         {}
func (Commit) isAction()       {}
func (PickUpgrade) isAction()  {}
func (PickDaemon) isAction()   {}
func (LeaveMerge) isAction()   {}
func (Link) isAction()         {}
func (Spend) isAction()        {}
func (Reroll) isAction()       {}

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
type Sound struct{ Name string }              // "ac", "wa", "crash", "commit", "boss", "cleared"
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
