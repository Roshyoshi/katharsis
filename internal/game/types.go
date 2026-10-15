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

	// Mult is what an upsolve made without the editorial multiplies Bytes
	// by: rules.DumpIndependentMult of the boot that dumped them
	// (retention RT4). 0, in a dump saved before it existed, reads as the
	// standard rules'.
	Mult float64 `json:",omitempty"`
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
	// Retired counts the regressions that came back at every stage and
	// were re-solved each time, so no longer return (the story's first one
	// is a milestone).
	Retired int `json:",omitempty"`
	Solves  []Solve
	Boots   int // boots started, for "Boot #N"; trials take numbers too
	// Trials counts the learn-mode trials among Boots, so a player who
	// opened RTFM before ever booting still gets boot #1's rules (RT11).
	Trials int `json:",omitempty"`

	// Seen lists the one-time story scenes already shown, each once, in
	// the order shown (MarkSeen). It is the story's reread log.
	Seen []SeenScene

	// TopicAttempts counts rated attempts per topic; a topic's rating aims
	// doors once it reaches rules.TopicRatingMin.
	TopicAttempts map[string]int

	// Rated counts every rated attempt; the first
	// rules.RatingColdAttempts move the ratings with rules.RatingKCold.
	Rated int
	// Prior is the answer to the first-launch rating question (D37,
	// prior.go), nil until it is asked.
	Prior *Prior `json:",omitempty"`
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

	// Governor counts finished boots in a row by how they ended, for the
	// easing (rules.GovernorAfter) and GRUB's prestige hint. Trials and
	// abandoned boots do not count.
	Governor Governor `json:",omitzero"`

	// LastPlayed is when the player last started or ended a boot or trial,
	// or upsolved: what a comeback (rules.ComebackDays) counts from.
	LastPlayed time.Time `json:",omitzero"`
}

// Governor is Meta's count of recent boot endings (ml-pipeline M4's streak
// governor; guardrail G2). The screens never show it as a count to keep
// up (retention GR1).
type Governor struct {
	// EarlyCrashes is how many boots in a row crashed in their first
	// biome, before any merge window; reaching one starts it over.
	EarlyCrashes int `json:",omitempty"`
	// FullClears is how many boots in a row were cleared; a crash starts it
	// over.
	FullClears int `json:",omitempty"`
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

	// TimerBuy is the multiplier the timer buy-back put on Timer (already
	// in it), 0 when none: the pool had no door near the aim, so the
	// engine bought the clear chance back with time (Ease, EaseFloor).
	// SolveP and Bytes are the door's without it.
	TimerBuy float64 `json:",omitempty"`

	// Floor is set when Rating is its source's floor (Codeforces rates
	// nothing under 800): the problem is that hard or less, and the engine
	// reads it as rules.FloorDrop easier on average (Difficulty).
	Floor bool `json:",omitempty"`

	// Twist is the boss whose rule this door bends (rules.Twists), its key,
	// "" for none: a boss modifier, shown on the boss card. The engine has
	// already applied what it carries out (Race Condition's Timer and
	// Bytes; Off-by-One's burn, State.SubmitCost); the fields below are for
	// the judge and the HUD.
	Twist string `json:",omitempty"`
	// MemMult multiplies the judge's memory limit (OOM Killer), 0 for 1.
	MemMult float64 `json:",omitempty"`
	// QuietSamples has sample runs report only pass or fail (Heisenbug).
	QuietSamples bool `json:",omitempty"`
	// NoSanitizers runs the samples without sanitizers, in the HUD and the
	// judge's profile (Undefined Behavior).
	NoSanitizers bool `json:",omitempty"`
	// HiddenClock hides the door's clock in the HUD; the strip's 5:00 and
	// 1:00 warnings still fire (The Halting Problem).
	HiddenClock bool `json:",omitempty"`
}

// TagsHidden reports whether the door hides its tags until a duck reveals
// them: a blind (or mystery) door, or one on a mastered topic.
func (d Door) TagsHidden() bool { return d.Blind || d.Mastered }

type BiomeState struct {
	Name  string
	Topic string
	Kind  rules.BiomeKind
	Rooms int
	// Offers are the topics on offer at the entrance (Choice only). On a
	// map they are the topics of Exits, one each in the same order, for
	// the log and the screens that show topics.
	Offers []string

	// Boss is the boss key of a Boss biome (BossKeys or FinalBoss), set at
	// boot so the map can name it before it is reached. On a map it is set
	// when the tier's node is taken.
	Boss string

	// Node is the map node (rules.Rules.Nodes) this tier plays, "" until
	// its exit is taken and in a boot without a map.
	Node string `json:",omitempty"`
	// Exits are the exits the tier was entered by on a map: the nodes the
	// previous tier's node leads to, locked ones included, in the graph's
	// order. ChooseRoute takes one by index; a single open exit is taken
	// at once. They are kept after the choice, for the map's "not taken".
	Exits []Exit `json:",omitempty"`
}

// Exit is one way out of a map node into the next tier (map.md §(g)): the
// card the route screen shows. It is read off the rules snapshot,
// State.Caps, State.Packages and the prestige, and draws no random number.
type Exit struct {
	Node  string // rules.Biome.ID
	Name  string // the node's name
	Topic string // its doors' topic: Mixed for a mixed or boss node, "" for a skip
	Rooms int    // 0 for a skip
	// Boss is the node's key when it is a boss node; Bosses are the boss
	// keys its own exits lead to, for a topic node ("→ ⚔ Heisenbug").
	Boss   string   `json:",omitempty"`
	Bosses []string `json:",omitempty"`
	// Picks are the upgrade picks owed at its end: a gated biome's scroll,
	// or a skip's two.
	Picks int  `json:",omitempty"`
	Skip  bool `json:",omitempty"`
	// Gate is what the node needs, zero when nothing (always open).
	Gate Gate `json:",omitzero"`
	// Locked is why it cannot be taken, "" when it can: the gate's terms
	// ("CAP_BSEARCH or apt git", "prestige 1"), or "pool" when the pool
	// holds too few problems for its rooms near the player.
	Locked string `json:",omitempty"`
}

// Gate is what a map node needs to be taken: its topic's capability
// (earned in an RTFM trial), unless Package is installed; and a prestige.
type Gate struct {
	Capability string `json:",omitempty"`
	Package    string `json:",omitempty"`
	Prestige   int    `json:",omitempty"`
}

// Outcome of one attempt.
type Outcome string

const (
	Cleared  Outcome = "cleared"
	TimedOut Outcome = "timed-out"
	// OutOfLives is a failed submit that burned the last cycles, or set
	// off a curse. The stored value keeps its name from lives (D48), so
	// saved outcomes need no migration.
	OutOfLives Outcome = "out-of-lives"
	GaveUp     Outcome = "gave-up"   // :q!
	Retreated  Outcome = "retreated" // swapped the door for cycles
	// Abandoned is a boot ended because another started while it was
	// suspended between doors (Abandon): it counts as a crash.
	Abandoned Outcome = "abandoned"
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

	// Offer is the index in State.Offers of the offer the door was chosen
	// from; CyclesAtOpen and DucksAtOpen are the boot's cycles and ducks
	// when it opened; Events lists what happened on it, in order
	// (ml-pipeline §4.1). Attempts from before these fields read 0 and
	// none. CyclesAtOpen keeps its JSON name from lives.
	Offer        int
	CyclesAtOpen int `json:"LivesAtOpen"`
	DucksAtOpen  int
	// At is the time bought with at on this door, already in Door.Timer:
	// -O3 measures the timer without it (rules.O3OnBase).
	At     time.Duration  `json:",omitempty"`
	Events []AttemptEvent `json:",omitempty"`
}

// Upgrade is picked after each cleared door.
type Upgrade string

const (
	Overclock Upgrade = "overclock" // +OverclockFrac timer on later doors
	ECC       Upgrade = "ecc"       // a failed submit costs clock, not cycles (rules.ECCPenalty); else a heal
	Duck      Upgrade = "duck"      // +1 duck
	ShufPick  Upgrade = "shuf"      // +1 shuf (locked until coreutils)
)

// Daemon is a passive perk picked at a merge window.
type Daemon string

const (
	Strace    Daemon = "strace"     // failing test's input is shown
	Nice      Daemon = "nice"       // +20% timer, every door pays 0.8x
	Fork      Daemon = "fork"       // 4 doors per room after an AC
	DevRandom Daemon = "dev-random" // mystery doors pay 2x
	Kexec     Daemon = "kexec"      // one free door swap per biome (per act: rules.KexecPerAct)
	Cron      Daemon = "cron"       // a regression door pays 2x
	Chrt      Daemon = "chrt"       // -20% timer, every door pays 1.5x
)

// Item is what a merge window sells for uncommitted bytes (D25): bought
// for this boot with bytes that committing would have banked.
type Item string

const (
	Fsck Item = "fsck" // rules.FsckCycles, up to MaxCycles
	Dup  Item = "dup"  // +1 duck
	Shuf Item = "shuf" // one re-roll of a room's doors (Reroll), kept until used

	At Item = "at" // more time on the next door's timer; sold once installed
)

// Crash is how a boot ended badly. An Abandoned boot crashed between doors:
// its Door is the zero Door and it leaves no core dump of its own.
type Crash struct {
	Cause   Outcome // TimedOut, OutOfLives, GaveUp, Abandoned
	Door    Door
	Verdict Verdict // last verdict on the door, for quote matching
	Lost    int64   // uncommitted bytes moved into the core dump (Abandoned: lost outright)
	At      time.Time
	Uptime  time.Duration

	// DumpLost is the core dump from an earlier boot that this crash
	// destroyed before writing its own.
	DumpLost int64

	// Cursed is set when a failed submit on a cursed door ended the boot:
	// the curse, not the cycles, which it does not charge (State.Cycles
	// has what was left). Cause is OutOfLives then too, the failed-submit
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

	// Cycles is the pool (D47, D48: lives renamed cycles), MaxCycles its
	// cap. The JSON keys keep their names from lives.
	Cycles      int `json:"Lives"`
	MaxCycles   int `json:"MaxLives"`
	Ducks       int
	Uncommitted int64 // lost on crash
	Committed   int64 // banked this boot (also added to Meta.Bank)

	Upgrades       []Upgrade
	Daemons        []Daemon
	Linked         []string  // lib names linked this boot
	PendingUpgrade bool      // a cleared door earned an upgrade pick
	UpgradeOffers  []Upgrade // what the pending pick offers
	DaemonOffers   []Daemon  // what the current merge window offers
	KexecUsed      bool      // per biome, or per act (rules.KexecPerAct)

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
	// Caps is Meta.Capabilities when the boot began, as Packages snapshots
	// apt: a map boot's exits read it, so a trial passed while the boot is
	// suspended opens no route in it. nil in a boot saved before it.
	Caps []string `json:",omitempty"`
	// Fountain is what this merge window's fountain restored (0 outside a
	// merge window and in one without a fountain), Fountained what every
	// fountain of the boot restored (rules.FountainCycles).
	Fountain   int `json:",omitempty"`
	Fountained int `json:",omitempty"`
	// Unlocked is the prestige this boot unlocked by its clear, 0 if none.
	Unlocked int

	// Trial is set when this boot is a learn-mode trial (NewTrial).
	Trial *TrialState

	// AtStart is the player as the boot began, for the end screens'
	// before and after (Debrief); nil in a boot saved before it existed.
	AtStart *Start `json:",omitempty"`

	// Forecast is the engine's chance, at NewBoot, that this boot crashes
	// in its first biome (before the first merge window) as it is aimed,
	// before any lives an easing adds: the number guardrail G1 holds at
	// rules.EaseAbove. 0 for a trial.
	Forecast float64 `json:",omitempty"`
	// Ease lists the easings this boot runs with, in the order NewBoot
	// found them (retention RT8, GR9). The screens announce each; Declined
	// is set once the player turned them down (DeclineEase), which keeps
	// them listed for the record but turns them all off.
	Ease     []Ease `json:",omitempty"`
	Declined bool   `json:",omitempty"`

	// Offers logs every set of doors and every route the boot put on
	// offer, in order, with the propensity of each item: the data for
	// evaluating another door policy off this one's (ml-pipeline §4, §5.3).
	Offers []Offer `json:",omitempty"`
}

// Start is what the player had when a boot began (State.AtStart).
type Start struct {
	Rating      float64
	TopicRating map[string]float64 `json:",omitempty"`
	Bank        int64
	// Dump is the core dump the boot puts at risk: an earlier boot's, which
	// a crash in this one destroys. nil for none, and in a trial, whose
	// crash leaves the dump alone.
	Dump *CoreDump `json:",omitempty"`
}

// EaseKind is why a boot eases off.
type EaseKind string

const (
	// EaseFloor: the pool has no first-biome door near the aim (its floor
	// sits above a low-rated player's), so doors there may get longer
	// timers (rules.BuybackTimers) and, when the forecast crash chance is
	// above rules.EaseAbove, the boot gets lives (D34) and
	// rules.EaseDucks ducks.
	EaseFloor EaseKind = "floor"
	// EaseGovernor: the last rules.GovernorAfter boots crashed in their
	// first biome, so every biome aims at its EaseClear.
	EaseGovernor EaseKind = "governor"
	// EaseComeback: the player is back after rules.ComebackDays or more
	// away, so the first biome aims at its EaseClear (retention RT8: a
	// return gets an easier first door, never guilt, GR8).
	EaseComeback EaseKind = "comeback"
	// EaseFirst: boot #1 (retention RT11) plays rules.FirstBiomes, and its
	// first room offers the pool's easiest doors with tags shown and no
	// curse, blind or mystery (FirstDoor), all with an editorial
	// (Editorial), so the first death teaches.
	EaseFirst EaseKind = "first"
)

// Ease is one easing a boot runs with. It is announced in task terms and
// can be declined (retention GR9): DeclineEase turns every easing of the
// boot off until the first door opens.
type Ease struct {
	Kind EaseKind
	// Why names the doors, boots or days behind it, never the player:
	// "Userland's easiest doors in the pool are rated 800", "the last 2
	// boots ended in Userland", "first boot in 5 days".
	Why string

	// Cycles is how many cycles it added at the boot's start; its JSON
	// key keeps its name from lives.
	Cycles int `json:"Lives,omitempty"`
	// Ducks is how many ducks it added at the boot's start.
	Ducks int `json:",omitempty"`
	// Clear is, per biome, the clear chance its doors are aimed at instead
	// of the biome's TargetClear; 0 leaves that biome's aim alone.
	Clear []float64 `json:",omitempty"`
	// Buyback gives the first biome's doors longer timers where the pool
	// has none near the aim (Door.TimerBuy).
	Buyback bool `json:",omitempty"`
	// FirstDoor and Editorial are boot #1's rules (EaseFirst).
	FirstDoor bool `json:",omitempty"`
	Editorial bool `json:",omitempty"`
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
type Retreat struct{}                // PhaseProblem: swap the door for cycles (kexec: free once), early in a door
type Quack struct{}                  // PhaseProblem: use a duck
type GiveUp struct{}                 // PhaseProblem: :q!

// SubmitResult reports a full-test judge of the open door. SampleFailed is
// set when a test that failed is one of the samples :w runs: such a submit
// costs nothing, as a CE does (meta.md §(b)), since the samples already
// told the player.
type SubmitResult struct {
	Verdict      Verdict
	Passed       int
	Total        int
	SampleFailed bool
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

// DeclineEase turns the boot's easing (State.Ease) off: the lives and ducks
// it added go, and the room on offer is drawn again without it. Allowed
// once, until the first door opens.
type DeclineEase struct{}

// Abandon ends the boot as a crash because the player starts another while
// it is suspended (meta.md §(c)): the uncommitted bytes are lost, the core
// dump an earlier boot left is destroyed, and the boot is logged as
// abandoned (Crash.Cause). Between doors it is allowed in every phase but
// the end; with a door open it is a give-up that ends the boot, as :q!
// did before cycles. The governor does not count it.
type Abandon struct{}

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
func (DeclineEase) isAction()  {}
func (Abandon) isAction()      {}

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

// MaxPrestige is the highest prestige level a boot can play: the map's
// ladder runs to P8 (map.md §(d), rules.MapAscensions). The story names
// P5-P8 by ring (flavor.Ring, cleared.N, grub.subtitle.prestige.N), and
// their division names fall back to "Prestige N". A rules ladder can stop
// short of it (rules.Rules.TopPrestige).
const MaxPrestige = 8

// ---- logging (ml-pipeline §4.1, slice ML1) ----------------------------------

// AttemptEvent is one thing that happened on an open door: a judged
// submit, a quack, the timer running out under a soft timer.
type AttemptEvent struct {
	At        time.Time
	Kind      string  // "submit", "quack", "overtime"
	Verdict   Verdict `json:",omitempty"`
	Passed    int     `json:",omitempty"`
	Total     int     `json:",omitempty"`
	FirstFail int     `json:",omitempty"` // 1-based first failing test; 0 unknown
	Level     int     `json:",omitempty"` // quack level
}

// Offer is one set of doors, or of route topics, the engine put on offer,
// with what weighing it later needs: each item's propensity (the chance
// the draw offered it, given the state it was drawn in) and that state.
type Offer struct {
	Seq    int    // index in State.Offers
	Kind   string // "doors" | "route"
	Biome  int
	Room   int // -1 for a route offer
	At     time.Time
	Policy string // how it was drawn: "elo-window/1"
	// Reason is why it was drawn: "fresh" (a room or route reached),
	// "retreat", "reroll" (a shuf), "decline" (DeclineEase), "timeout" and
	// "gave-up" (a door failed for cycles, its room drawn again), "retry"
	// (a boss door failed for cycles: a new boss door).
	Reason string
	RNG    uint64  // State.RNG before the draw, for audits
	Target float64 // the clear chance the biome aims at (AimClear)
	Floor  float64 // the biome's MinClear
	Mu, SD float64 // the rating its topic was aimed with, and its SD (0 under Elo)
	Items  []OfferItem
	// SetProp is the probability of the set, unordered: the product over
	// the draw's steps of 1/C(eligible, drawn).
	SetProp  float64
	Chosen   int       // index into Items, -1 until chosen
	ChosenAt time.Time `json:",omitzero"`
}

// OfferItem is one door, or route topic, of an Offer.
type OfferItem struct {
	Problem   catalog.ID     `json:",omitempty"`
	Source    catalog.Source `json:",omitempty"`
	RatingSrc string         `json:",omitempty"` // the problem's RatingSource
	Topic     string
	Rating    int `json:",omitempty"`
	// Slot is how the draw found it: a rating window of the biome's topic
	// or of any ("w100", "any250"), the nearest left past the windows
	// ("near", "near-any"), or "review", "trial", "route".
	Slot string
	// P is the clear chance the engine expected with every stake (the
	// timer bought back included), PBase without the buy-back.
	P, PBase float64 `json:",omitempty"`
	// Propensity is the chance the draw put it on offer, given the state:
	// drawn / eligible within its step, 1 for a review or trial door.
	Propensity float64
	Eligible   int  // candidates in its step
	Tier       int  // its step's index in the draw's order; -1 none
	Blind      bool `json:",omitempty"`
	Mystery    bool `json:",omitempty"`
	Cursed     bool `json:",omitempty"`
	Review     bool `json:",omitempty"`
	// BlindWhy is why its tags are hidden: "" (they are not), "mastery",
	// "roll" (the blind chance, prestige's included), "run" (BlindAll),
	// "trial".
	BlindWhy string        `json:",omitempty"`
	Timer    time.Duration `json:",omitempty"`
	Bytes    int64         `json:",omitempty"`
	Position int           // display index
}
