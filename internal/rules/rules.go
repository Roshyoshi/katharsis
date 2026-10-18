// Package rules holds the tunable numbers of a boot. A preset is a TOML
// file; the engine snapshots the Rules into each boot, so editing a preset
// never changes a boot in progress. Every field has a default in
// Standard(); presets only need to override what they change.
package rules

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Duration struct{ time.Duration }

func (d *Duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	d.Duration = v
	return err
}

func (d Duration) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// OnExpire values. A soft timer (overtime) suits learn mode, where the point
// is to finish the problem, not to beat the clock.
const (
	ExpireCrash    = "crash"
	ExpireOvertime = "overtime"
)

// BiomeKind: a fixed biome has one topic chosen by the engine; a choice
// biome offers Choices topics at its entrance; a boss biome is a single
// named-bug room.
type BiomeKind string

const (
	Fixed  BiomeKind = "fixed"
	Choice BiomeKind = "choice"
	Boss   BiomeKind = "boss"
)

type Biome struct {
	Kind    BiomeKind `toml:"kind"`
	Name    string    `toml:"name"`    // "" lets the engine name it by topic
	Rooms   int       `toml:"rooms"`   // rooms (problems) in this biome
	Choices int       `toml:"choices"` // topics offered (Choice only)
	// TargetClear is the solve probability the engine aims each door at,
	// given the player's rating; it sets door difficulty.
	TargetClear float64  `toml:"target_clear"`
	Timer       Duration `toml:"timer"` // default timer before fitting
	// MinClear is the lowest solve probability the prestige offset may push
	// this biome's doors to: past it, a higher prestige stops raising them
	// and is harder only through its ascensions. It is also a floor for
	// each door: while the pool has a door the player clears with at least
	// MinClear, the biome offers none below it (D22). 0 sets no floor.
	MinClear float64 `toml:"min_clear"`

	// EaseClear is the solve probability an eased boot (game.State.Ease:
	// the governor after early crashes, a comeback after days away) aims
	// this biome's doors at instead of TargetClear. 0 leaves the aim alone.
	EaseClear float64 `toml:"ease_clear"`
	// Plain is how many doors of each room roll no risk modifier (blind,
	// mystery, cursed), the easiest on offer first: a safe door is always
	// there, so risk stays the player's choice. A run modifier
	// (blind_all) still applies.
	Plain int `toml:"plain"`
	// TimeoutLife makes a door whose timer runs out in this biome cost a
	// life (Rules.LifeCycles) instead of the boot while more is left, as a
	// retreat would: the door closes, rated as failed, and the room is
	// drawn again (D44). It is the rule of a boot saved before cycles;
	// FailCycles supersedes it.
	TimeoutLife bool `toml:"timeout_life"`

	// FailCycles, above 0, makes a timeout or a give-up (:q!) in this biome
	// burn this many cycles instead of ending the boot, while more than
	// that is left (D47: 10 a room, scaled by act; 20 a boss). The door
	// closes, rated as failed and back as a regression; Rules.FailMoveOn
	// says whether the room is lost or drawn again. A burn that would
	// leave none ends the boot, and so does a one-shot boss (Rules.OneShot).
	// 0 is the rule before cycles: a timeout ends the boot (TimeoutLife
	// aside) and so does a give-up.
	FailCycles int `toml:"fail_cycles"`

	// ---- a map node (docs/design/map.md §(c), §(h)) ------------------------
	//
	// A Biome in Rules.Nodes is a node of the boot's map. Zero values keep
	// a linear biome, so Rules.Biomes and old snapshots read as before.

	// ID names the node; Tier is the index into Rules.Biomes (the tier
	// skeleton) it plays at; Next lists the node IDs its exit leads to,
	// each of the next tier.
	ID   string   `toml:"id"`
	Tier int      `toml:"tier"`
	Next []string `toml:"next"`
	// Topic is the doors' topic, "" for mixed. A topic node is gated by
	// the topic's capability (game.TopicCapabilities), which Package, when
	// installed, also opens (git opens Bisect Bridge, `git bisect`).
	Topic   string `toml:"topic"`
	Package string `toml:"package"`
	// Boss is a boss node's key (game.BossKeys, game.FinalBoss); exactly
	// the nodes of a boss tier name one.
	Boss string `toml:"boss"`
	// MinPrestige gates the node behind a prestige level (Undefined
	// Behavior at 1, The Framebuffer at 2).
	MinPrestige int `toml:"min_prestige"`
	// Picks are upgrade picks owed when the node's last room is cleared:
	// a scroll at a gated biome's exit.
	Picks int `toml:"picks"`
	// Skip is an exit past its tier: no room, Picks owed, and the next tier
	// entered at once (Dead Cells' Guardian's Haven: two scrolls for the
	// level skipped).
	Skip bool `toml:"skip"`
	// Toll is what taking the node costs every door after it: their clocks
	// run Toll shorter (D62: the act III skip shortens the final by 0.2). 0
	// costs nothing.
	Toll float64 `toml:"toll"`

	// ---- a secret node (docs/design/achievements.md §(f)) -------------------

	// Hidden keeps the node's exit off the route until it opens, or until
	// the player has met its gate's hint (game.Meta.Capped for NeedFull):
	// the map never names it before.
	Hidden bool `toml:"hidden"`
	// NeedFull opens the node's exit only to a full cycle pool: the boot's
	// cycles at its cap (game.State.MaxCycles) as the tier is entered.
	NeedFull bool `toml:"need_full"`
	// Cursed makes every door of the node cursed (no ducks; one failed
	// submit ends the boot), as a door's roll would.
	Cursed bool `toml:"cursed"`
	// Ending, when set, ends the boot once the node's last room is
	// cleared: cleared with this ending (game.State.Ending), banking what
	// is uncommitted, unlocking no prestige and counting as no full clear.
	// Such a node has no exits.
	Ending string `toml:"ending"`
}

type Rules struct {
	Name string `toml:"name"`

	// Cycles is the pool a boot starts with and MaxCycles its cap (D47,
	// D48: lives renamed cycles, drawn as ♥). The keys keep the names they
	// had as lives, so presets and saved boots read as they did.
	Cycles       int `toml:"lives" json:"Lives"`
	MaxCycles    int `toml:"max_lives" json:"MaxLives"`
	Ducks        int `toml:"ducks"` // charges at boot start and per refill
	DoorsPerRoom int `toml:"doors_per_room"`

	// Biomes are the boot's biomes in order. With Nodes set they are the
	// map's tier skeleton instead: each tier's kind, and the defaults the
	// map shows before an exit is taken.
	Biomes []Biome `toml:"biome"`

	// Door modifiers: chance per door, and the byte multiplier each pays.
	BlindChance   float64 `toml:"blind_chance"`
	MysteryChance float64 `toml:"mystery_chance"`
	CursedChance  float64 `toml:"cursed_chance"`
	BlindMult     float64 `toml:"blind_mult"`
	MysteryMult   float64 `toml:"mystery_mult"`
	CursedMult    float64 `toml:"cursed_mult"`
	// BlindAll hides tags on every door (a run modifier).
	BlindAll bool `toml:"blind_all"`
	// DuckTest makes the duck's second quack rubber-duck debugging (D64):
	// after a failed submit it copies the smallest test that submit failed
	// into the door's samples, so :w runs it and w shows it, where it named
	// the technique, which a topic biome already does. false is the
	// technique (a boot saved before D64).
	DuckTest bool `toml:"duck_test"`

	// MasteryAttempts and MasteryMargin hide the tags of doors on a topic
	// the player has mastered: one with MasteryAttempts rated attempts whose
	// rating is at least MasteryMargin above the door's. Choosing the
	// technique is what a known topic still trains; a topic being learned
	// keeps its tags. Such a door is not blind: it pays and teaches the
	// calibration like a plain door, and a duck still reveals its tags.
	// MasteryAttempts 0 turns it off.
	MasteryAttempts int     `toml:"mastery_attempts"`
	MasteryMargin   float64 `toml:"mastery_margin"`

	// BaseBytes pays for clearing a door the player clears half the time;
	// easier doors pay less and harder ones more, so a door's expected
	// payout stays near BaseBytes/2. Bosses pay BossMult on top.
	BaseBytes int64   `toml:"base_bytes"`
	BossMult  float64 `toml:"boss_mult"`
	// O3Frac: AC within this fraction of the timer grants a free upgrade.
	O3Frac float64 `toml:"o3_frac"`

	// Upgrade sizes.
	OverclockFrac float64 `toml:"overclock_frac"` // timer bonus per Overclock

	// RegressionDays schedules a problem that crashed you: it returns as a
	// door RegressionDays[0] days after the crash, RegressionDays[i] days
	// after its i-th re-solve, and retires after the last.
	RegressionDays []int `toml:"regression_days"`

	// DumpIndependentMult multiplies the core dump an upsolve recovers when
	// the editorial was never opened (retention RT4): the independent act
	// pays more. 1 pays the same either way.
	DumpIndependentMult float64 `toml:"dump_independent_mult"`

	// TimerFit: once the player has this many solves, door timers come from
	// their own solve times (quantile TimerQuantile, scaled by gap).
	TimerFitMin     int     `toml:"timer_fit_min"`
	TimerQuantile   float64 `toml:"timer_quantile"`
	RatingK         float64 `toml:"rating_k"` // Elo K for the player rating
	ColdStartRating float64 `toml:"cold_start_rating"`

	// ---- door selection ----------------------------------------------------

	// FloorRating is the lowest rating the sources in FloorSources give (a
	// problem's RatingSource): Codeforces rates nothing under 800, so a
	// problem rated there is that hard or less. The engine reads one at
	// that floor as FloorDrop easier on average, spread as an exponential
	// under it (game.Door.Difficulty). FloorDrop 0 reads floor ratings as
	// they are.
	FloorRating  int      `toml:"floor_rating"`
	FloorDrop    float64  `toml:"floor_drop"`
	FloorSources []string `toml:"floor_sources"`
	// RatingWindows are the ± bands around a door's target rating tried in
	// order; when none holds enough problems the engine takes any rating.
	RatingWindows []int `toml:"rating_windows"`
	// TopicRatingMin is how many attempts a topic needs before its own
	// rating replaces the overall one for aiming doors.
	TopicRatingMin int `toml:"topic_rating_min"`
	// TopicFollow is the share of every overall rating change that each
	// topic rating follows, so a topic rating is the overall plus what its
	// own doors taught (1) rather than a separate rating (0).
	TopicFollow float64 `toml:"topic_follow"`
	// RouteMinPool is how many eligible problems per room a topic needs
	// within the widest window to be offered as a biome.
	RouteMinPool int `toml:"route_min_pool"`
	// BossDoors is how many doors the boss room offers.
	BossDoors int `toml:"boss_doors"`
	// MinBytesMult and MaxBytesMult clamp the difficulty factor of a door's
	// reward, so a door far off target (sparse pool) pays a sane amount.
	MinBytesMult float64 `toml:"min_bytes_mult"`
	MaxBytesMult float64 `toml:"max_bytes_mult"`

	// ---- attempts ----------------------------------------------------------

	// HintedScore is the Elo score of a clear that used a duck (an
	// independent clear scores 1, anything else 0).
	HintedScore float64 `toml:"hinted_score"`
	// FreeVerdicts are submit verdicts that cost no life (a compile error
	// never ran a test).
	FreeVerdicts []string `toml:"free_verdicts"`
	// SubmitGrace accepts a judge result this long after the deadline: the
	// full tests take a while, and the submit happened before it.
	SubmitGrace Duration `toml:"submit_grace"`
	// UpgradeOffers and DaemonOffers size each pick; MaxDaemons caps how
	// many daemons one boot can run.
	UpgradeOffers int `toml:"upgrade_offers"`
	DaemonOffers  int `toml:"daemon_offers"`
	MaxDaemons    int `toml:"max_daemons"`

	// ---- merge window spending --------------------------------------------

	// What a merge window sells for uncommitted bytes (D25): spent on this
	// boot, where committing would bank them for packages at GRUB. A price
	// of 0 takes the item off sale. Prices are bytes, so a preset that
	// scales base_bytes should scale them too: a cleared door pays about
	// BaseBytes/2 in Userland and BaseBytes/1.4 in the mid biome.
	FsckPrice int64 `toml:"fsck_price"` // FsckCycles, up to MaxCycles
	DupPrice  int64 `toml:"dup_price"`  // +1 duck
	ShufPrice int64 `toml:"shuf_price"` // a re-roll of a room's doors, kept until used
	// at is sold only once its package is installed.
	AtPrice int64    `toml:"at_price"` // AtTime more on the next door's timer
	AtTime  Duration `toml:"at_time"`

	// ---- daemons -----------------------------------------------------------

	ForkDoors     int     `toml:"fork_doors"`      // doors per room after an AC
	NiceTimerFrac float64 `toml:"nice_timer_frac"` // timer bonus
	NiceBytesFrac float64 `toml:"nice_bytes_frac"` // pay cut on every door
	// NiceLifeCost multiplies what a failed submit burns while nice runs.
	// Nice pays for its time in bytes (NiceBytesFrac) since the alpha, so
	// this is 1, the normal charge: at 2 it traded survival for survival,
	// +20% time for a doubled charge on every miss. A preset may still set
	// it. A boot saved before the change keeps its snapshot's 2 and,
	// NiceBytesFrac being 0 there, its full pay.
	NiceLifeCost  int     `toml:"nice_life_cost"`
	DevRandomMult float64 `toml:"dev_random_mult"` // extra pay on mystery doors
	CronMult      float64 `toml:"cron_mult"`       // extra pay on a regression door
	ChrtTimerFrac float64 `toml:"chrt_timer_frac"` // timer cut
	ChrtBytesMult float64 `toml:"chrt_bytes_mult"` // extra pay on every door

	// ---- timer fitting -----------------------------------------------------

	// TimerGapScale is the rating gap that doubles a solve time; fitting
	// uses it to move a solve at one gap onto another.
	TimerGapScale float64 `toml:"timer_gap_scale"`
	// TimerHeadroom multiplies the fitted quantile. Only solves that beat
	// their timer are ever recorded, so the raw quantile shrinks every boot;
	// the headroom gives the fit a fixed point instead of a ratchet.
	TimerHeadroom float64 `toml:"timer_headroom"`
	// TimerFitLo and TimerFitHi clamp a fitted timer to this fraction of
	// the biome's default timer.
	TimerFitLo float64 `toml:"timer_fit_lo"`
	TimerFitHi float64 `toml:"timer_fit_hi"`
	// TimerFitHistory is how many recent solves the fit looks at, so the
	// timers follow the player as they improve.
	TimerFitHistory int `toml:"timer_fit_history"`
	// BossTimerMult stretches fitted boss timers beyond what the gap gives.
	// It is 1 by default: the default boss timer's extra length (50 vs 30
	// min) is the boss's harder gap at TimerGapScale, and the fit already
	// scales by gap, so a 1.5 here would count the stretch twice.
	BossTimerMult float64 `toml:"boss_timer_mult"`

	// ---- rating and aim calibration ----------------------------------------

	// RatingKCold is the Elo K for the player's first RatingColdAttempts
	// rated attempts. Everyone starts at ColdStartRating, and a clear of a
	// door aimed at 90% moves the rating by a tenth of K, so at the
	// steady-state K a strong newcomer spends boots on doors far below
	// them. RatingColdAttempts 0 turns the boost off.
	RatingKCold        float64 `toml:"rating_k_cold"`
	RatingColdAttempts int     `toml:"rating_cold_attempts"`
	// LinkSlots is how many libs a boot may link before its first door.
	LinkSlots int `toml:"link_slots"`
	// CalibRate is the gain of the aim calibration (game.Calibration): the
	// share of one plain door's surprise that moves its tier's offset, so a
	// tier forgets its past over about 1/CalibRate doors. 0 turns it off.
	CalibRate float64 `toml:"calib_rate"`
	// CalibWarm starts each offset on a faster gain, 1/(doors+CalibWarm),
	// until it falls to CalibRate: the first doors average like a running
	// mean, as if CalibWarm doors had agreed with the aim. 0 starts at
	// CalibRate.
	CalibWarm int `toml:"calib_warm"`
	// CalibAfter is how many rated attempts the player needs before doors
	// teach the calibration: the rating is still climbing out of the cold
	// start before that, and the offsets would learn the climb.
	CalibAfter int `toml:"calib_after"`
	// CalibMax bounds the aim correction, in rating points either way.
	CalibMax float64 `toml:"calib_max"`

	// ---- soft timers -------------------------------------------------------

	// OnExpire is what a door's timer running out does: ExpireCrash ends the
	// boot, ExpireOvertime lets the attempt go on, marked overtime (trials).
	OnExpire string `toml:"on_expire"`
	// OvertimeScore caps the Elo score of a clear made in overtime;
	// OvertimeBytesMult scales its bytes.
	OvertimeScore     float64 `toml:"overtime_score"`
	OvertimeBytesMult float64 `toml:"overtime_bytes_mult"`
	// ShowFailingInput shows every door's failing full test, as the strace
	// daemon does (see game.State.ShowsFailingInput).
	ShowFailingInput bool `toml:"show_failing_input"`

	// ---- prestige ----------------------------------------------------------

	// PrestigeOffset is added to every door's target rating per prestige
	// level.
	PrestigeOffset float64 `toml:"prestige_offset"`
	// TierBands are the door ratings each prestige keeps to, a USACO tier a
	// level (D63, the original idea: Dead Cells' boss cells as Bronze,
	// Silver, Gold, Platinum and Advanced). A door's aim at the player is
	// held inside its level's band, so a tier above the player is a wall
	// until they are better. Levels past the list keep its last; empty
	// holds no band (a boot saved before D63).
	TierBands [][2]int `toml:"tier_bands"`
	// TierWarmup lowers the first biome's band by that many points, so a
	// tier's Userland is a stepping stone from the tier below (Silver's
	// opens at 1150-1450) and the tier bites in its acts and bosses, as a
	// Dead Cells boss cell does. 0 keeps the band.
	TierWarmup int `toml:"tier_warmup"`
	// Ascensions add difficulty per prestige level on top of the rating
	// offset; a boot at prestige p applies every entry with Level <= p.
	Ascensions []Ascension `toml:"ascension"`

	// ---- packages ----------------------------------------------------------

	// Packages are what the bank buys at GRUB (see Package).
	Packages []Package `toml:"package"`

	// ---- easing (retention RT0, RT8; ml-pipeline M4, slice ML7) -----------

	// A boot eases off when its first biome would crash it too often: the
	// guardrail is P(crash before the first merge window) <= 0.2 (G1 in
	// docs/research/ml-pipeline.md). Every easing is announced in
	// game.State.Ease and can be declined before the first door opens
	// (retention GR9).

	// EaseAbove is the forecast chance of crashing in the first biome above
	// which a boot eases off there. The forecast is the engine's own: the
	// best clear chance the pool allows in each room of the biome, and
	// under cycles the chance its failures burn the pool. It binds for a
	// low-rated player, whose aim sits below the pool's floor (no local
	// problem is rated under 800). An eased boot gets lives (D34), each
	// LifeCycles cycles. EaseLives is the most lives it adds: D34's one,
	// and one more for each further EaseAbove of forecast crashes; and
	// EaseDucks more ducks (neuro P1.2 buys the clear chance back with
	// lives, time or ducks). 0 turns it off.
	EaseAbove float64 `toml:"ease_above"`
	EaseLives int     `toml:"ease_lives"`
	EaseDucks int     `toml:"ease_ducks"`
	// BuybackTimers are the timer multipliers that buy a floor door's
	// clear chance back: when the pool has no first-biome door near the
	// aim, each door gets the smallest multiplier that lifts its clear
	// chance to within BuybackMargin of the aim, or the largest.
	// BuybackBeta is what a longer timer is taken to be worth, in log-odds
	// per unit of ln(multiplier) (the ml-pipeline M1 prior). Empty turns the
	// buy-back off.
	BuybackTimers []float64 `toml:"buyback_timers"`
	BuybackBeta   float64   `toml:"buyback_beta"`
	BuybackMargin float64   `toml:"buyback_margin"`
	// GovernorAfter is how many boots in a row that crashed in the first
	// biome make the next aim every biome at its EaseClear, with
	// GovernorLives more lives, each LifeCycles cycles (the aim cannot move
	// below the pool's floor; a life can); reaching a merge window starts
	// the count over. 0 turns it off.
	GovernorAfter int `toml:"governor_after"`
	GovernorLives int `toml:"governor_lives"`
	// ComebackDays is how many days away make the next boot a comeback:
	// its first biome aims at its EaseClear (retention RT8). 0 turns it
	// off.
	ComebackDays int `toml:"comeback_days"`

	// ---- boot #1 (retention RT11) -------------------------------------------

	// A new player's first boot closes one whole loop in about an hour: it
	// plays FirstBiomes (empty: the preset's own), its first room offers
	// the pool's easiest doors with tags shown and no risk modifier
	// (FirstDoor), and its doors all have an editorial while the pool has
	// such (FirstEditorial), so the first death teaches.
	FirstBiomes    []Biome `toml:"first_biome"`
	FirstDoor      bool    `toml:"first_door"`
	FirstEditorial bool    `toml:"first_editorial"`

	// ---- cycles (D47, docs/research/meta.md) ---------------------------------
	//
	// Every price is in cycles. A zero price is one life, LifeCycles
	// cycles: the rule of a boot saved before cycles, whose lives are
	// cycles one apiece.

	// LifeCycles is what one life of the rules before cycles is worth: the
	// easings' lives (EaseLives, GovernorLives) are dealt in it, and an
	// unset price costs one. 0 means 1.
	LifeCycles int `toml:"life_cycles"`
	// SubmitCycles is what a charged failed submit (not CE, not a sample
	// failure: FreeVerdicts) burns, before the ascensions' SubmitCycles.
	SubmitCycles int `toml:"submit_cycles"`
	// RetreatCycles is what a retreat burns; a retreat that would leave
	// none is refused.
	RetreatCycles int `toml:"retreat_cycles"`
	// FsckCycles and ECCCycles are what an fsck and an ECC pick restore,
	// up to MaxCycles, before the ascensions' HealCycles. ECC restores
	// only while ECCPenalty is 0.
	FsckCycles int `toml:"fsck_cycles"`
	ECCCycles  int `toml:"ecc_cycles"`
	// FailMoveOn makes a room whose door failed without ending the boot
	// (Biome.FailCycles) lost: no pay, no upgrade pick, and the boot moves
	// to the next room (ICPC triage). A boss room is never lost: it draws
	// a new boss door. false draws the room again, as D44 did.
	FailMoveOn bool `toml:"fail_move_on"`
	// OneShot makes the boot's last OneShot bosses one-shot: a timeout or
	// give-up there ends the boot whatever its cycles (1: the final;
	// Ascension.OneShot raises it). The others burn their FailCycles and
	// draw a new boss door.
	OneShot int `toml:"one_shot"`
	// ECCPenalty, above 0, makes the ECC upgrade a conversion, offered
	// once a boot: from its pick on, a charged failed submit burns no
	// cycles and takes ECCPenalty off the open door's clock instead (the
	// user's 5:00), while the door has more than that left; with less it
	// burns the cycles (game.State.ECCShort).
	ECCPenalty Duration `toml:"ecc_penalty"`
	// ECCMargin is clock ECC leaves standing: it converts only while the
	// door holds more than ECCPenalty plus this, so a converted fail never
	// strands a door at a few seconds (5:10 left would leave 0:10). 0, as in
	// a boot saved before it, converts down to the penalty itself.
	ECCMargin Duration `toml:"ecc_margin"`
	// RetreatLock, above 0, refuses a retreat (kexec's free swap too) this
	// long into a door: a swap is for a statement read, not a door fought.
	RetreatLock Duration `toml:"retreat_lock"`
	// The vaults at each boss but the last (Dead Cells' timed and no-hit
	// doors; game.Split): judged as the boss falls, each open one owes
	// VaultPicks upgrade picks. The timed vault opens when the act's time in
	// its doors stays within TimedVault of its cleared doors' timers, the
	// time the player bought taken out (0 turns it off); the untouched vault
	// when no door of the act was hit (UntouchedVault). 0 picks, as in a
	// boot saved before them, is no vaults.
	TimedVault     float64 `toml:"timed_vault"`
	UntouchedVault bool    `toml:"untouched_vault"`
	VaultPicks     int     `toml:"vault_picks"`

	// D62, the boot's supply. PickPerBiome owes a biome's upgrade pick at
	// its exit, after its last room, not one for each door cleared;
	// O3NoPick pays a -O3 clear nothing more (the timed vault is speed's
	// reward); DaemonActs offers daemons only in the merge windows of the
	// first DaemonActs acts, the one after the act's boss included, so the
	// roster is set by the end of boss I at 1. Zero values are the old
	// supply: a pick a door, a -O3 pick, daemons in every window.
	PickPerBiome bool `toml:"pick_per_biome"`
	O3NoPick     bool `toml:"o3_no_pick"`
	DaemonActs   int  `toml:"daemon_acts"`

	// ---- the map (docs/design/map.md) -----------------------------------------

	// Nodes is the boot's map: a graph of biomes in the tiers of Biomes.
	// Tier 0 plays its one node; each later tier plays the node the player
	// takes from the last one's Next (game.Exit), gated ones shown locked.
	// Empty plays Biomes in order, as before the map.
	Nodes []Biome `toml:"node"`
	// FountainCycles is what a merge window's fountain restores, free, up
	// to the boot's starting cycles (not MaxCycles), where FountainWhere
	// puts one: FountainAct in the windows after a boss (between acts),
	// FountainBoss in those before one, FountainEvery in all. 0, with the
	// ascensions' FountainCycles, is no fountain.
	FountainCycles int    `toml:"fountain_cycles"`
	FountainWhere  string `toml:"fountain_where"`
	// KexecPerAct makes kexec's free swap once an act (the tiers up to and
	// including a boss) instead of once a biome.
	KexecPerAct bool `toml:"kexec_per_act"`

	// ---- time is bought (D50) -------------------------------------------------

	// TimerMult scales every door's timer: the base clock's setting. 0
	// means 1.
	TimerMult float64 `toml:"timer_mult"`
	// OverclockMax, above 0, makes Overclock additive (+OverclockFrac a
	// pick, not compounding) and stops offering it after OverclockMax
	// picks; 0 compounds without a cap, as before. OverclockCycles is what
	// a pick burns (it is offered only while more than that is left).
	OverclockMax    int `toml:"overclock_max"`
	OverclockCycles int `toml:"overclock_cycles"`
	// O3OnBase measures -O3 against the door's timer before the time the
	// player bought (at, Overclock, nice), so bought time never makes the
	// free upgrade easier.
	O3OnBase bool `toml:"o3_on_base"`
	// FirstClockOwn exempts the first biome from the ascensions' timer
	// multipliers: its clock per prestige is Ascension.FirstTimer's
	// (Userland 10:00, 9:00 at P3, 8:00 at P6).
	FirstClockOwn bool `toml:"first_clock_own"`

	// Twists are the rule each boss bends: a boss door's modifier, as
	// blind, mystery and cursed are a door's, shown on its boss card.
	Twists Twists `toml:"twists"`

	// Perk is the pick pools' roster and numbers (perk.go); its zero value
	// keeps the first pools.
	Perk PerkRules `toml:"perk"`
}

// Twists is one setting per boss (game.BossKeys, game.FinalBoss), each
// bending one rule on that boss's doors; a zero value bends nothing, as in a
// boot saved before them. The engine carries out the cycles, clock and pay;
// the door says the rest (game.Door's twist flags) for the judge and the
// HUD to carry out.
type Twists struct {
	// OffByOne multiplies what a failed submit burns (or, under ECC, the
	// clock it costs) when exactly one test fails: off-by-one punishes the
	// near miss. 0 and 1 bend nothing.
	OffByOne int `toml:"off_by_one"`
	// OOMMemory multiplies the judge's memory limit on the OOM Killer's
	// doors. 0 bends nothing.
	OOMMemory float64 `toml:"oom_memory"`
	// Heisenbug makes its doors' sample runs report only pass or fail, no
	// per-sample detail: the bug hides when looked at.
	Heisenbug bool `toml:"heisenbug"`
	// RaceClock multiplies Race Condition's door timers (0.8: the clock
	// runs 1.25 times fast) and RacePay replaces BossMult on its doors
	// (4 for 3). 0 bends nothing.
	RaceClock float64 `toml:"race_clock"`
	RacePay   float64 `toml:"race_pay"`
	// UBNoSanitizers runs Undefined Behavior's samples without sanitizers.
	UBNoSanitizers bool `toml:"ub_no_sanitizers"`
	// HaltingClock hides the clock on The Halting Problem's doors: you
	// cannot tell whether it will finish. The dmesg strip's 5:00 and 1:00
	// warnings still fire.
	HaltingClock bool `toml:"halting_clock"`
}

// Fountain placements (Rules.FountainWhere).
const (
	FountainEvery = ""     // every merge window
	FountainAct   = "act"  // the windows after a boss: between acts
	FountainBoss  = "boss" // the windows before a boss
)

// Life is what one life is worth in cycles (LifeCycles), at least 1.
func (r Rules) Life() int { return max(1, r.LifeCycles) }

// price is p, or one life when p is unset.
func (r Rules) price(p int) int {
	if p > 0 {
		return p
	}
	return r.Life()
}

// SubmitPrice, RetreatPrice, FsckHeal and ECCHeal are SubmitCycles,
// RetreatCycles, FsckCycles and ECCCycles, or one life where unset; the
// ascensions' cuts are added by the caller (game.State knows the level).
func (r Rules) SubmitPrice() int  { return r.price(r.SubmitCycles) }
func (r Rules) RetreatPrice() int { return r.price(r.RetreatCycles) }
func (r Rules) FsckHeal() int     { return r.price(r.FsckCycles) }
func (r Rules) ECCHeal() int      { return r.price(r.ECCCycles) }

// Clock is TimerMult, 1 where unset.
func (r Rules) Clock() float64 {
	if r.TimerMult > 0 {
		return r.TimerMult
	}
	return 1
}

// Node returns the map node with id.
func (r Rules) Node(id string) (Biome, bool) {
	for _, n := range r.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Biome{}, false
}

// TopPrestige is the highest level the rules' ladder (Ascensions) reaches,
// capped at max: past it a level adds nothing. A linear boot meets The
// Halting Problem there. Rules without a ladder top out at max.
func (r Rules) TopPrestige(limit int) int {
	top := 0
	for _, a := range r.Ascensions {
		top = max(top, min(a.Level, limit))
	}
	if top <= 0 {
		return limit
	}
	return top
}

// Ascension is the extra difficulty of one prestige level. Levels stack like
// Dead Cells boss cells: prestige 3 plays with levels 1, 2 and 3.
type Ascension struct {
	Level         int     `toml:"level"`
	BlindChance   float64 `toml:"blind_chance"`   // added to the door roll
	MysteryChance float64 `toml:"mystery_chance"` // added to the door roll
	CursedChance  float64 `toml:"cursed_chance"`  // added to the door roll
	// Cycles is added to the starting pool (at least 1 is left). Its key
	// keeps its name from lives.
	Cycles    int     `toml:"lives" json:"Lives"`
	Ducks     int     `toml:"ducks"`      // added at boot and refill
	TimerMult float64 `toml:"timer_mult"` // 0 means 1

	// Cycle cuts (D47, map.md §(d)), summed over the levels: SubmitCycles
	// is added to a charged failed submit's price, HealCycles to what fsck
	// and ECC restore (each still restores 1), LateFailCycles to a
	// FailCycles burn in the tiers after the boot's first boss (acts II and
	// III, so prestige bites late), FountainCycles to each fountain (at
	// least 0).
	SubmitCycles   int `toml:"submit_cycles"`
	HealCycles     int `toml:"heal_cycles"`
	LateFailCycles int `toml:"late_fail_cycles"`
	FountainCycles int `toml:"fountain_cycles"`
	// OneShot raises Rules.OneShot: the boot's last OneShot bosses are
	// one-shot (2: Boss II and the final; 3: every boss of three). The
	// highest level's counts.
	OneShot int `toml:"one_shot"`
	// FirstTimer, when set, is the first biome's default timer from this
	// level on (the highest level's counts).
	FirstTimer Duration `toml:"first_timer"`
}

// Ascend sums every ascension at or below prestige. TimerMult multiplies, so
// the result's TimerMult is never 0.
func (r Rules) Ascend(prestige int) Ascension {
	sum := Ascension{Level: prestige, TimerMult: 1}
	first := 0 // the level FirstTimer was taken from
	for _, a := range r.Ascensions {
		if a.Level < 1 || a.Level > prestige {
			continue
		}
		sum.BlindChance += a.BlindChance
		sum.MysteryChance += a.MysteryChance
		sum.CursedChance += a.CursedChance
		sum.Cycles += a.Cycles
		sum.Ducks += a.Ducks
		if a.TimerMult > 0 {
			sum.TimerMult *= a.TimerMult
		}
		sum.SubmitCycles += a.SubmitCycles
		sum.HealCycles += a.HealCycles
		sum.LateFailCycles += a.LateFailCycles
		sum.FountainCycles += a.FountainCycles
		sum.OneShot = max(sum.OneShot, a.OneShot)
		if a.FirstTimer.Duration > 0 && a.Level >= first {
			sum.FirstTimer, first = a.FirstTimer, a.Level
		}
	}
	return sum
}

// Package is a permanent unlock bought at GRUB with banked bytes (D25),
// the way apt installs one: once, for every later boot. Packages are
// rules so a preset can price or remove them, but they are bought outside
// a boot, so GRUB sells the standard preset's. Each grants options or
// comfort more than power; the counts add up over installed packages.
type Package struct {
	Name     string   `toml:"name"`
	Price    int64    `toml:"price"`    // banked bytes
	Requires []string `toml:"requires"` // packages to install first

	// Daemon, Item and Upgrade are locked out of every boot (the merge
	// windows' daemon offers and shelf, the upgrade picks) until this
	// package is installed.
	Daemon  string `toml:"daemon"`
	Item    string `toml:"item"`
	Upgrade string `toml:"upgrade"`

	LinkSlots     int    `toml:"link_slots"`     // more libs linked per boot
	Ducks         int    `toml:"ducks"`          // more ducks at boot start
	DaemonOffers  int    `toml:"daemon_offers"`  // more daemons offered per merge window
	UpgradeOffers int    `toml:"upgrade_offers"` // more upgrades offered per pick
	RouteOffers   int    `toml:"route_offers"`   // more topics offered at each route
	Doors         int    `toml:"doors"`          // more doors per room, a boss's aside
	Theme         string `toml:"theme"`          // a GRUB banner theme, cosmetic
}

// Ladder is r's packages cheapest first, ties in the rules' order: the
// order GRUB shelves them in and the goal ladder they make (RT3).
func (r Rules) Ladder() []Package {
	ps := slices.Clone(r.Packages)
	slices.SortStableFunc(ps, func(a, b Package) int { return cmp.Compare(a.Price, b.Price) })
	return ps
}

// Package returns the package named name.
func (r Rules) Package(name string) (Package, bool) {
	for _, p := range r.Packages {
		if p.Name == name {
			return p, true
		}
	}
	return Package{}, false
}

// Grants sums what the installed packages among r's give a boot: link
// slots, ducks, doors, daemon, route and upgrade offers. Its Name and the
// rest are empty.
func (r Rules) Grants(installed []string) Package {
	var sum Package
	for _, p := range r.Packages {
		if slices.Contains(installed, p.Name) {
			sum.LinkSlots += p.LinkSlots
			sum.Ducks += p.Ducks
			sum.DaemonOffers += p.DaemonOffers
			sum.RouteOffers += p.RouteOffers
			sum.UpgradeOffers += p.UpgradeOffers
			sum.Doors += p.Doors
		}
	}
	return sum
}

// Locked lists the daemons, items and upgrades (by name; they are distinct
// types in game) that a package of r provides and that is not installed.
func (r Rules) Locked(installed []string) (daemons, items, upgrades []string) {
	for _, p := range r.Packages {
		if slices.Contains(installed, p.Name) {
			continue
		}
		for _, l := range []struct {
			name string
			to   *[]string
		}{{p.Daemon, &daemons}, {p.Item, &items}, {p.Upgrade, &upgrades}} {
			if l.name != "" {
				*l.to = append(*l.to, l.name)
			}
		}
	}
	return daemons, items, upgrades
}

// WithDefaults fills fields whose zero value means nothing (counts, scales,
// clamps) from Standard(), so Rules built in code before a field existed
// still play. Fields where zero is a real setting (chances, offsets, the
// calibration rate, the cold-start length, overtime penalties, Ascensions)
// are left alone, so such Rules keep the behavior they had before the
// field existed. An empty OnExpire becomes ExpireCrash.
func (r Rules) WithDefaults() Rules {
	d := Standard()
	if len(r.RatingWindows) == 0 {
		r.RatingWindows = d.RatingWindows
	}
	if r.FreeVerdicts == nil {
		r.FreeVerdicts = d.FreeVerdicts
	}
	if len(r.RegressionDays) == 0 {
		r.RegressionDays = d.RegressionDays
	}
	ints := []struct{ v, d *int }{
		{&r.MaxCycles, &d.MaxCycles}, {&r.TopicRatingMin, &d.TopicRatingMin},
		{&r.RouteMinPool, &d.RouteMinPool}, {&r.BossDoors, &d.BossDoors},
		{&r.UpgradeOffers, &d.UpgradeOffers}, {&r.DaemonOffers, &d.DaemonOffers},
		{&r.MaxDaemons, &d.MaxDaemons}, {&r.ForkDoors, &d.ForkDoors},
		{&r.NiceLifeCost, &d.NiceLifeCost}, {&r.TimerFitMin, &d.TimerFitMin},
		{&r.TimerFitHistory, &d.TimerFitHistory},
		{&r.LinkSlots, &d.LinkSlots},
	}
	for _, f := range ints {
		if *f.v <= 0 {
			*f.v = *f.d
		}
	}
	floats := []struct{ v, d *float64 }{
		{&r.BlindMult, &d.BlindMult}, {&r.MysteryMult, &d.MysteryMult},
		{&r.CursedMult, &d.CursedMult}, {&r.BossMult, &d.BossMult},
		{&r.MinBytesMult, &d.MinBytesMult}, {&r.MaxBytesMult, &d.MaxBytesMult},
		{&r.DevRandomMult, &d.DevRandomMult}, {&r.TimerGapScale, &d.TimerGapScale},
		{&r.CronMult, &d.CronMult}, {&r.ChrtBytesMult, &d.ChrtBytesMult},
		{&r.TimerHeadroom, &d.TimerHeadroom}, {&r.TimerFitLo, &d.TimerFitLo},
		{&r.TimerFitHi, &d.TimerFitHi}, {&r.BossTimerMult, &d.BossTimerMult},
		{&r.TimerQuantile, &d.TimerQuantile}, {&r.RatingK, &d.RatingK},
		{&r.ColdStartRating, &d.ColdStartRating}, {&r.RatingKCold, &d.RatingKCold},
		{&r.CalibMax, &d.CalibMax},
		{&r.DumpIndependentMult, &d.DumpIndependentMult},
	}
	for _, f := range floats {
		if *f.v <= 0 {
			*f.v = *f.d
		}
	}
	if r.BaseBytes <= 0 {
		r.BaseBytes = d.BaseBytes
	}
	if r.AtTime.Duration <= 0 {
		r.AtTime = d.AtTime
	}
	if r.OnExpire == "" {
		r.OnExpire = ExpireCrash
	}
	return r
}

// Standard is the default preset, the rules GRUB boots by name "standard":
// the three-act map (Map). map.md §(h): the map is standard's replacement,
// under its name.
func Standard() Rules { return Map() }

// Legacy is the standard preset as it was before the cycles meta (D47) and
// the map: three lives, Userland, a choice biome and one boss. A boot saved
// then carries these rules in its snapshot and plays them to its end
// (map.md §(h)); tests build such boots from here. Numbers come from the
// design discussion (DESIGN.md "A boot") and the balance simulation.
func Legacy() Rules {
	return Rules{
		Name:         "standard",
		Cycles:       3,
		MaxCycles:    5,
		Ducks:        1,
		DoorsPerRoom: 3,
		Biomes: []Biome{
			{Kind: Fixed, Name: "Userland", Rooms: 2, TargetClear: 0.92, Timer: Duration{15 * time.Minute}, EaseClear: 0.95, Plain: 1, TimeoutLife: true},
			// The mid biome is where learning happens, so prestige holds it at
			// 0.6 or above (D22) and adds risk instead.
			{Kind: Choice, Rooms: 2, Choices: 2, TargetClear: 0.7, MinClear: 0.6, Timer: Duration{30 * time.Minute}, EaseClear: 0.78},
			{Kind: Boss, Rooms: 1, TargetClear: 0.5, Timer: Duration{50 * time.Minute}, EaseClear: 0.55},
		},
		BlindChance:     0.25,
		MysteryChance:   0.08,
		CursedChance:    0.08,
		BlindMult:       1.5,
		MysteryMult:     2,
		CursedMult:      2,
		MasteryAttempts: 10,
		MasteryMargin:   100,
		BaseBytes:       64,
		BossMult:        3,
		O3Frac:          0.5,
		OverclockFrac:   0.15,
		RegressionDays:  []int{3, 17},
		TimerFitMin:     20,
		TimerQuantile:   0.85,
		RatingK:         40,
		ColdStartRating: 1000,

		// The user-held default (retention RT4): an upsolve without the
		// editorial recovers half as much again.
		DumpIndependentMult: 1.5,

		FloorRating:  800,
		FloorDrop:    250,
		FloorSources: []string{"cf"},

		RatingWindows:  []int{100, 250, 500},
		TopicRatingMin: 5,
		TopicFollow:    1,
		RouteMinPool:   6,
		BossDoors:      1,
		MinBytesMult:   0.25,
		MaxBytesMult:   4,

		HintedScore:   0.7,
		FreeVerdicts:  []string{"CE"},
		SubmitGrace:   Duration{30 * time.Second},
		UpgradeOffers: 3,
		DaemonOffers:  3,
		MaxDaemons:    3,

		FsckPrice: 64,
		DupPrice:  48,
		ShufPrice: 32,
		AtPrice:   40,
		AtTime:    Duration{5 * time.Minute},

		ForkDoors:     4,
		NiceTimerFrac: 0.2,
		NiceBytesFrac: 0.2,
		NiceLifeCost:  1,
		DevRandomMult: 2,
		CronMult:      2,
		ChrtTimerFrac: 0.2,
		ChrtBytesMult: 1.5,

		// 200 points per doubling makes the default 15/30/50 min timers
		// roughly the 0.85 quantile of one lognormal (sigma 0.5, median
		// 30 min at gap 0) across the three biome targets.
		TimerGapScale:   200,
		TimerHeadroom:   1.25,
		TimerFitLo:      0.5,
		TimerFitHi:      1.5, // 2 made calibrated bosses ~3 h; 1.5 keeps clears ~2 h
		TimerFitHistory: 200,
		BossTimerMult:   1,

		RatingKCold:        100,
		RatingColdAttempts: 20,
		LinkSlots:          2,
		CalibRate:          0.02,
		CalibWarm:          10,
		CalibAfter:         60,
		CalibMax:           300,

		OnExpire:          ExpireCrash,
		OvertimeScore:     0.5,
		OvertimeBytesMult: 0.5,

		PrestigeOffset: 25,
		// Prestige gets harder through these more than through the offset:
		// the mid biome's floor stops the offset there (D22), and each level
		// costs the model players a few points of full clears (sim
		// TestBalanceReport/prestige).
		Ascensions: []Ascension{
			{Level: 1, BlindChance: 0.15, TimerMult: 0.95},
			{Level: 2, Ducks: -1, MysteryChance: 0.05, CursedChance: 0.04},
			{Level: 3, CursedChance: 0.05, TimerMult: 0.9},
			{Level: 4, Cycles: -1, CursedChance: 0.04, TimerMult: 0.9},
		},

		// Easing (RT0). With the floor read as censored (FloorDrop) and
		// Userland's timeout rule (D44), the model players from 900 to 1600
		// crash in Userland 6 to 17% of the time without any of it
		// (TestBalanceReport/guardrails), so only the settled pieces are on:
		// D34's life past G1's 0.2 forecast, and neuro P1.3's governor after
		// two early crashes (G2), aiming at 0.95 / 0.78 / 0.55 (the biomes'
		// EaseClear) with a life. The second life, the duck and the timer
		// buy-back (ml-pipeline M4, at half its timer prior) are built and
		// off: in the simulator they moved G1 by 1 to 3 points.
		EaseAbove:     0.2,
		EaseLives:     1,
		EaseDucks:     0,
		BuybackTimers: nil,
		BuybackBeta:   0.5,
		BuybackMargin: 0.02,
		GovernorAfter: 2,
		GovernorLives: 1,
		ComebackDays:  3,

		// Boot #1 plays the short shape (RT7, RT11), the floor's doors first.
		FirstBiomes:    shortBiomes(),
		FirstDoor:      true,
		FirstEditorial: true,

		// What the bank buys at GRUB (D25, retention RT3): mostly options
		// (a daemon, an item or an upgrade joins its pool; one more route,
		// daemon or upgrade is on offer, one more door per room), banner
		// themes, a starting duck, and one capstone. Over its first 40
		// boots at prestige 0 the 1000-rated sim player banks about 200 B a
		// boot (commits and upsolved core dumps; 140 B before RT0's floor
		// reading and D44 let most of its boots past Userland), so bought
		// cheapest first the first package is one cleared door and every next
		// one is at most three boots away until binutils, about 42 boots in;
		// the catalog is about 62 boots of that player's bank and 49 of the
		// 1600-rated one's (263 B a boot). The measured rate moves by a
		// tenth with the prices themselves (when a package lands changes the
		// boots), so the ladder holds for 176 to 212 B. rules_test.go checks
		// the ladder at those rates and sim TestAptLadder against the
		// players; prices are settings like any other (D2).
		//
		// binutils is the one package of raw power: a lib slot pastes more
		// of the player's own code, which the door model does not see
		// (State.Linked is saved with every boot, so it can learn to). It is
		// the capstone, after the ladder, and the only one of its kind.
		Packages: []Package{
			{Name: "cowsay", Price: 32, Theme: "cowsay"},
			{Name: "tux", Price: 256, Theme: "tux"},
			{Name: "sl", Price: 384, Theme: "sl"},
			{Name: "rng-tools", Price: 448, Daemon: "dev-random"},
			{Name: "at", Price: 512, Item: "at"},
			{Name: "cron", Price: 528, Daemon: "cron"},
			{Name: "strace", Price: 544, Daemon: "strace"},
			{Name: "coreutils", Price: 544, Upgrade: "shuf"},
			{Name: "aptitude", Price: 544, UpgradeOffers: 1, Requires: []string{"coreutils"}},
			{Name: "rubber-duck", Price: 544, Ducks: 1},
			{Name: "git", Price: 544, RouteOffers: 1},
			{Name: "neofetch", Price: 544, Theme: "neofetch"},
			{Name: "util-linux", Price: 544, Daemon: "chrt"},
			{Name: "figlet", Price: 544, Theme: "figlet"},
			{Name: "tmux", Price: 544, Doors: 1},
			{Name: "cmatrix", Price: 544, Theme: "cmatrix"},
			{Name: "bastet", Price: 544, Theme: "bastet"},
			// A fourth daemon on offer means nothing while the pool holds
			// three, so procps waits on two of the locked ones.
			{Name: "procps", Price: 544, DaemonOffers: 1, Requires: []string{"rng-tools", "strace"}},
			// The roster's apt extras (perk.go): a net for one crash, -O3's
			// wider share, and the sanitizers back on a boss's samples. Late
			// on the ladder, under binutils.
			{Name: "kdump-tools", Price: 896, Daemon: "kdump"},
			{Name: "tuned", Price: 640, Daemon: "tuned"},
			{Name: "clang-tools", Price: 768, Upgrade: "fsanitize"},
			{Name: "binutils", Price: 4096, LinkSlots: 1},
		},
	}
}

// BuiltIns are the presets the game ships, in the order GRUB lists them:
// standard, then the two shorter boots offered where "one more" is
// decided (retention RT7). Their names are taken: a preset file cannot
// reuse one, so a name in the boots table always means these rules.
func BuiltIns() []Rules { return []Rules{Standard(), Short(), Patch()} }

// Short is the built-in short preset (retention RT7), the cycles meta on
// the shape boot #1 plays (ShortBiomes): two Userland doors, one mid door
// and the boss, on short timers, off the map. Its prices, ladder and twists
// are the map's; its one boss is the final, so it is one shot.
func Short() Rules {
	r := Map()
	r.Name = "short"
	r.Biomes, r.Nodes = ShortBiomes(), nil
	return r
}

// Patch is the built-in patch preset (retention RT7), the cheapest "one
// more": one Userland door, one mid door, and a mini-boss aimed at 0.6
// (PatchBiomes), off the map; every other number is the map's. Its doors
// are 3 against short's 4 and the map's 9. The timers fit to the player's
// solve times, so the end screens' ETAs are the player's own medians
// (GR3).
func Patch() Rules {
	r := Map()
	r.Name = "patch"
	r.Biomes, r.Nodes = PatchBiomes(), nil
	return r
}

// shortBiomes is the legacy short shape: boot #1's before cycles.
func shortBiomes() []Biome {
	return []Biome{
		{Kind: Fixed, Name: "Userland", Rooms: 2, TargetClear: 0.9, Timer: Duration{10 * time.Minute}, EaseClear: 0.95, Plain: 1, TimeoutLife: true},
		{Kind: Choice, Rooms: 1, Choices: 2, TargetClear: 0.7, MinClear: 0.6, Timer: Duration{20 * time.Minute}, EaseClear: 0.78},
		{Kind: Boss, Rooms: 1, TargetClear: 0.5, Timer: Duration{25 * time.Minute}, EaseClear: 0.55},
	}
}

// Trial is the learn-mode preset (DESIGN.md "Learn mode") that
// game.NewTrial plays: a unit biome of the module's own problems with tags
// shown, then blind integration doors, one door per room, on a soft timer.
//
// The two biomes are a shape, not a layout: the unit's Rooms is the
// minimum unit size (a short unit is topped up to it, a longer one plays
// in full) and the integration's Rooms is how many blind doors it draws.
// TargetClear is unused, since trial doors are the module's, not aimed at
// the player; it only has to pass Validate.
//
// Failing input is shown, as if strace ran: a trial is the first time the
// player meets the topic, and a hidden WA teaches nothing.
func Trial() Rules {
	// A trial is learn mode, not a boot: it keeps the lives the rules had
	// before cycles (three, a failed submit one), and no map.
	r := Legacy()
	r.Name = "trial"
	r.DoorsPerRoom = 1
	r.Biomes = []Biome{
		{Kind: Fixed, Name: "unit", Rooms: 4, TargetClear: 0.7, Timer: Duration{30 * time.Minute}},
		{Kind: Fixed, Name: "integration", Rooms: 2, TargetClear: 0.7, Timer: Duration{30 * time.Minute}},
	}
	r.BlindChance, r.MysteryChance, r.CursedChance = 0, 0, 0
	r.Ascensions = nil
	r.MasteryAttempts = 0 // the unit shows tags and the integration hides them
	r.OnExpire = ExpireOvertime
	r.ShowFailingInput = true
	// A trial is not aimed, so it has nothing to ease, and it is no boot.
	r.EaseAbove, r.GovernorAfter, r.ComebackDays = 0, 0, 0
	r.FirstBiomes, r.FirstDoor, r.FirstEditorial = nil, false, false
	return r
}

// Load reads a preset on top of Standard(), so a preset file lists only
// what it changes.
func Load(path string) (Rules, error) {
	r := Standard()
	b, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	// toml decodes an array of tables into an existing slice element by
	// element, so a preset's [[biome]] would inherit Standard's fields by
	// index (the first custom biome named "Userland"). A preset that lists
	// tables replaces the whole slice instead.
	var keys map[string]any
	if err := toml.Unmarshal(b, &keys); err != nil {
		return r, fmt.Errorf("%s: %w", path, err)
	}
	if _, ok := keys["biome"]; ok {
		r.Biomes = nil
	}
	if _, ok := keys["ascension"]; ok {
		r.Ascensions = nil
	}
	if _, ok := keys["package"]; ok {
		r.Packages = nil
	}
	if _, ok := keys["first_biome"]; ok {
		r.FirstBiomes = nil
	}
	// So does [[node]]; and a preset that lists biomes but no nodes is a
	// linear shape, off the map.
	_, biomes := keys["biome"]
	if _, ok := keys["node"]; ok || biomes {
		r.Nodes = nil
	}
	md, err := toml.Decode(string(b), &r)
	if err != nil {
		return r, fmt.Errorf("%s: %w", path, err)
	}
	// A key the rules do not have is an error, not a silent no-op: a
	// misspelt override would play as standard while the player believes
	// it changed.
	if keys := md.Undecoded(); len(keys) > 0 {
		names := make([]string, len(keys))
		for i, k := range keys {
			names[i] = k.String()
		}
		return r, fmt.Errorf("%s: unknown keys %s", path, strings.Join(names, ", "))
	}
	// Without a name key the preset is named after its file, so an error
	// from Validate names the preset rather than "standard".
	if !md.IsDefined("name") {
		r.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return r, r.Validate()
}

func (r Rules) Validate() error {
	if r.Cycles < 1 || r.DoorsPerRoom < 1 || len(r.Biomes) == 0 {
		return fmt.Errorf("rules %q: need lives, doors_per_room and at least one biome", r.Name)
	}
	for i, b := range r.Biomes {
		if b.Rooms < 1 || b.TargetClear <= 0 || b.TargetClear >= 1 || b.Timer.Duration <= 0 {
			return fmt.Errorf("rules %q: biome %d needs rooms >= 1, 0 < target_clear < 1, timer > 0", r.Name, i)
		}
		if b.MinClear < 0 || b.MinClear > b.TargetClear {
			return fmt.Errorf("rules %q: biome %d needs 0 <= min_clear <= target_clear", r.Name, i)
		}
		if b.Kind == Choice && b.Choices < 2 {
			return fmt.Errorf("rules %q: choice biome %d needs choices >= 2", r.Name, i)
		}
		if b.Kind != Fixed && b.Kind != Choice && b.Kind != Boss {
			return fmt.Errorf("rules %q: biome %d kind %q is not fixed, choice or boss", r.Name, i, b.Kind)
		}
		if b.EaseClear != 0 && (b.EaseClear < b.TargetClear || b.EaseClear >= 1) || b.Plain < 0 {
			return fmt.Errorf("rules %q: biome %d needs ease_clear 0 or in target_clear..1, plain >= 0", r.Name, i)
		}
	}
	if !(r.FloorDrop >= 0 && r.FloorDrop <= 1000) || !(r.EaseAbove >= 0 && r.EaseAbove < 1) || r.EaseLives < 0 || r.EaseDucks < 0 || r.GovernorAfter < 0 || r.GovernorLives < 0 ||
		!(r.BuybackBeta >= 0) || !(r.BuybackMargin >= 0 && r.BuybackMargin < 1) {
		return fmt.Errorf("rules %q: need 0 <= floor_drop <= 1000, 0 <= ease_above < 1, ease_lives, ease_ducks, governor_after, governor_lives, buyback_beta >= 0, 0 <= buyback_margin < 1", r.Name)
	}
	for _, k := range r.BuybackTimers {
		// The negated test also catches NaN.
		if !(k >= 1 && k <= MaxBuyback) {
			return fmt.Errorf("rules %q: buyback_timers %v not in 1..%d", r.Name, k, MaxBuyback)
		}
	}
	if r.ComebackDays < 0 {
		return fmt.Errorf("rules %q: need comeback_days >= 0", r.Name)
	}
	if len(r.FirstBiomes) > 0 {
		first := r
		first.Biomes, first.FirstBiomes, first.Nodes = r.FirstBiomes, nil, nil
		if err := first.Validate(); err != nil {
			return fmt.Errorf("first_biome: %w", err)
		}
	}
	if r.TimerFitLo > r.TimerFitHi {
		return fmt.Errorf("rules %q: timer_fit_lo above timer_fit_hi", r.Name)
	}
	if r.OnExpire != "" && r.OnExpire != ExpireCrash && r.OnExpire != ExpireOvertime {
		return fmt.Errorf("rules %q: on_expire %q is not %q or %q", r.Name, r.OnExpire, ExpireCrash, ExpireOvertime)
	}
	if r.FsckPrice < 0 || r.DupPrice < 0 || r.ShufPrice < 0 || r.AtPrice < 0 {
		return fmt.Errorf("rules %q: need fsck_price, dup_price, shuf_price, at_price >= 0", r.Name)
	}
	if r.ChrtTimerFrac < 0 || r.ChrtTimerFrac >= 1 {
		return fmt.Errorf("rules %q: need 0 <= chrt_timer_frac < 1", r.Name)
	}
	// The negated test also catches NaN.
	if !(r.NiceBytesFrac >= 0 && r.NiceBytesFrac < 1) {
		return fmt.Errorf("rules %q: need 0 <= nice_bytes_frac < 1", r.Name)
	}
	if r.MasteryAttempts < 0 {
		return fmt.Errorf("rules %q: need mastery_attempts >= 0", r.Name)
	}
	if r.CalibRate < 0 || r.CalibRate >= 1 || r.CalibWarm < 0 || r.CalibAfter < 0 || r.RatingColdAttempts < 0 {
		return fmt.Errorf("rules %q: need 0 <= calib_rate < 1 and calib_warm, calib_after, rating_cold_attempts >= 0", r.Name)
	}
	for _, a := range r.Ascensions {
		if a.Level < 1 {
			return fmt.Errorf("rules %q: ascension level %d < 1", r.Name, a.Level)
		}
	}
	for i, p := range r.Packages {
		if p.Name == "" || p.Price < 0 || slices.ContainsFunc(r.Packages[:i], func(q Package) bool { return q.Name == p.Name }) {
			return fmt.Errorf("rules %q: package %d needs a unique name and price >= 0", r.Name, i)
		}
		for _, req := range p.Requires {
			if _, ok := r.Package(req); !ok || req == p.Name {
				return fmt.Errorf("rules %q: package %q requires %q, which is not another package", r.Name, p.Name, req)
			}
		}
	}
	if err := r.validateCycles(); err != nil {
		return err
	}
	if err := r.validateNodes(); err != nil {
		return err
	}
	return r.validateBytes()
}

// validateCycles checks the cycles and time fields: no negative price,
// burn or duration, a known fountain placement, a finite clock.
func (r Rules) validateCycles() error {
	if r.LifeCycles < 0 || r.SubmitCycles < 0 || r.RetreatCycles < 0 || r.FsckCycles < 0 ||
		r.ECCCycles < 0 || r.OneShot < 0 || r.FountainCycles < 0 || r.OverclockMax < 0 || r.OverclockCycles < 0 {
		return fmt.Errorf("rules %q: need life_cycles, submit_cycles, retreat_cycles, fsck_cycles, "+
			"ecc_cycles, one_shot, fountain_cycles, overclock_max, overclock_cycles >= 0", r.Name)
	}
	if r.ECCPenalty.Duration < 0 || r.ECCMargin.Duration < 0 || r.RetreatLock.Duration < 0 {
		return fmt.Errorf("rules %q: need ecc_penalty, ecc_margin, retreat_lock >= 0", r.Name)
	}
	for i, b := range r.TierBands {
		if b[0] <= 0 || b[0] >= b[1] {
			return fmt.Errorf("rules %q: tier_bands[%d] %v needs 0 < low < high", r.Name, i, b)
		}
	}
	if r.TierWarmup < 0 {
		return fmt.Errorf("rules %q: tier_warmup %d is negative", r.Name, r.TierWarmup)
	}
	if !(r.TimedVault >= 0 && r.TimedVault <= 2) || r.VaultPicks < 0 || r.DaemonActs < 0 {
		return fmt.Errorf("rules %q: need 0 <= timed_vault <= 2, vault_picks >= 0, daemon_acts >= 0", r.Name)
	}
	if err := r.Perk.validate(r.Name); err != nil {
		return err
	}
	// The negated test also catches NaN.
	if !(r.TimerMult >= 0 && r.TimerMult <= MaxTimerMult) {
		return fmt.Errorf("rules %q: timer_mult %v not in 0..%d", r.Name, r.TimerMult, MaxTimerMult)
	}
	if w := r.FountainWhere; w != FountainEvery && w != FountainAct && w != FountainBoss {
		return fmt.Errorf("rules %q: fountain_where %q is not %q, %q or %q", r.Name, w, FountainEvery, FountainAct, FountainBoss)
	}
	for _, bs := range [][]Biome{r.Biomes, r.Nodes} {
		for i, b := range bs {
			if b.FailCycles < 0 || b.Picks < 0 || b.MinPrestige < 0 || !(b.Toll >= 0 && b.Toll < 1) {
				return fmt.Errorf("rules %q: biome %d needs fail_cycles, picks, min_prestige >= 0 and 0 <= toll < 1", r.Name, i)
			}
		}
	}
	for _, a := range r.Ascensions {
		if a.OneShot < 0 || a.FirstTimer.Duration < 0 {
			return fmt.Errorf("rules %q: ascension %d needs one_shot, first_timer >= 0", r.Name, a.Level)
		}
	}
	// The negated tests also catch NaN.
	tw := r.Twists
	if tw.OffByOne < 0 || !(tw.OOMMemory >= 0 && tw.OOMMemory <= MaxTimerMult) || !(tw.RaceClock >= 0 && tw.RaceClock <= MaxTimerMult) ||
		!(tw.RacePay >= 0 && tw.RacePay <= MaxBytesMultiplier) {
		return fmt.Errorf("rules %q: twists need off_by_one >= 0, oom_memory and race_clock in 0..%d, race_pay in 0..%d",
			r.Name, MaxTimerMult, MaxBytesMultiplier)
	}
	return nil
}

// MaxTimerMult bounds timer_mult: past it a door's timer is hours.
const MaxTimerMult = 10

// validateNodes checks the map: every node a playable biome of its tier
// (rooms, aim and timer as a biome's; a skip plays no room), IDs unique,
// one node in tier 0, a boss key on the nodes of boss tiers and no other,
// and every exit to a node of the next tier, which the last tier's nodes
// and an ending have none of.
func (r Rules) validateNodes() error {
	if len(r.Nodes) == 0 {
		return nil
	}
	ids := map[string]Biome{}
	starts := 0
	last := len(r.Biomes) - 1
	for _, n := range r.Nodes {
		if _, dup := ids[n.ID]; n.ID == "" || dup || n.Tier < 0 || n.Tier > last {
			return fmt.Errorf("rules %q: node %q needs a unique id and a tier in 0..%d", r.Name, n.ID, last)
		}
		if n.Skip {
			n.Rooms = max(n.Rooms, 1) // a skip plays no room; the rest must still check out
		}
		if n.Rooms < 1 || n.TargetClear <= 0 || n.TargetClear >= 1 || n.Timer.Duration <= 0 ||
			n.MinClear < 0 || n.MinClear > n.TargetClear || n.EaseClear != 0 && (n.EaseClear < n.TargetClear || n.EaseClear >= 1) {
			return fmt.Errorf("rules %q: node %q needs rooms >= 1, 0 <= min_clear <= target_clear < 1, ease_clear 0 or in target_clear..1, timer > 0",
				r.Name, n.ID)
		}
		if boss := r.Biomes[n.Tier].Kind == Boss; boss != (n.Boss != "") || boss && n.Skip {
			return fmt.Errorf("rules %q: node %q: the nodes of a boss tier, and only they, name a boss (and none is a skip)", r.Name, n.ID)
		}
		if n.Tier == 0 && n.Skip {
			return fmt.Errorf("rules %q: node %q: the start is no skip", r.Name, n.ID)
		}
		if (n.Hidden || n.Ending != "") && (n.Tier == 0 || n.Skip) {
			return fmt.Errorf("rules %q: node %q: the start and a skip can be neither hidden nor an ending", r.Name, n.ID)
		}
		ids[n.ID] = n
		if n.Tier == 0 {
			starts++
		}
	}
	if starts != 1 {
		return fmt.Errorf("rules %q: the map needs one tier-0 node, has %d", r.Name, starts)
	}
	for _, n := range r.Nodes {
		// An ending ends the boot where it stands: no exits.
		if len(n.Next) == 0 && n.Tier < last && n.Ending == "" || len(n.Next) > 0 && (n.Tier == last || n.Ending != "") {
			return fmt.Errorf("rules %q: node %q of tier %d has %d exits", r.Name, n.ID, n.Tier, len(n.Next))
		}
		for i, id := range n.Next {
			if m, ok := ids[id]; !ok || m.Tier != n.Tier+1 || slices.Contains(n.Next[:i], id) {
				return fmt.Errorf("rules %q: node %q exits to %q, not a node of tier %d", r.Name, n.ID, id, n.Tier+1)
			}
		}
	}
	return nil
}

// MaxBaseBytes bounds base_bytes, MaxBytesMultiplier each multiplier of a
// door's pay, and MaxDoorBytes what one door can pay with every multiplier
// stacked, 18 KiB under Standard (no door stacks them all: a boss or a
// regression is never cursed). The bounds keep the sums over a lifetime of
// boots (the bank, lifetime bytes) far from int64's limit: a preset with
// base_bytes near it overflowed on the first clear.
const (
	MaxBaseBytes       = 1 << 30
	MaxBytesMultiplier = 1000
	MaxDoorBytes       = 1 << 40
)

// MaxBuyback bounds a timer buy-back multiplier: past it a door's timer is
// hours, which no clear chance needs.
const MaxBuyback = 10

func (r Rules) validateBytes() error {
	if r.BaseBytes < 0 || r.BaseBytes > MaxBaseBytes {
		return fmt.Errorf("rules %q: base_bytes %d not in 0..%d", r.Name, r.BaseBytes, MaxBaseBytes)
	}
	mults := []struct {
		key string
		v   float64
	}{
		{"blind_mult", r.BlindMult}, {"mystery_mult", r.MysteryMult},
		{"cursed_mult", r.CursedMult}, {"boss_mult", r.BossMult},
		{"min_bytes_mult", r.MinBytesMult}, {"max_bytes_mult", r.MaxBytesMult},
		{"dev_random_mult", r.DevRandomMult}, {"overtime_bytes_mult", r.OvertimeBytesMult},
		{"cron_mult", r.CronMult}, {"chrt_bytes_mult", r.ChrtBytesMult},
		{"dump_independent_mult", r.DumpIndependentMult},
	}
	for _, m := range mults {
		// TOML has inf and nan; the negated test also catches NaN.
		if m.v < 0 || !(m.v <= MaxBytesMultiplier) {
			return fmt.Errorf("rules %q: %s %v not in 0..%d", r.Name, m.key, m.v, MaxBytesMultiplier)
		}
	}
	// The richest door stacks every multiplier as doorBytes in
	// internal/game does: the difficulty factor at its cap, mystery (with
	// dev-random) or blind, cursed, a boss, a regression with cron, chrt,
	// and overtime pay. A zero is the default WithDefaults fills in.
	d := r.WithDefaults()
	worst := float64(d.BaseBytes) * max(d.MaxBytesMult, d.MinBytesMult) *
		max(d.MysteryMult*d.DevRandomMult, d.BlindMult, 1) * max(d.CursedMult, 1) *
		max(d.BossMult, d.Twists.RacePay, 1) * max(d.CronMult, 1) * max(d.ChrtBytesMult, 1) * max(d.OvertimeBytesMult, 1)
	if worst > MaxDoorBytes {
		return fmt.Errorf("rules %q: a door could pay %.3g bytes, over the %d allowed; lower base_bytes or the *_mult keys",
			r.Name, worst, int64(MaxDoorBytes))
	}
	return nil
}

// TierBand is the door ratings prestige p keeps to (TierBands, its last
// for a level past them); ok is false when the rules hold no band.
func (r *Rules) TierBand(p int) (band [2]int, ok bool) {
	if len(r.TierBands) == 0 {
		return [2]int{}, false
	}
	return r.TierBands[min(max(p, 0), len(r.TierBands)-1)], true
}
