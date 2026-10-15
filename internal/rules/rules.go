// Package rules holds the tunable numbers of a boot. A preset is a TOML
// file; the engine snapshots the Rules into each boot, so editing a preset
// never changes a boot in progress. Every field has a default in
// Standard(); presets only need to override what they change.
package rules

import (
	"fmt"
	"os"
	"path/filepath"
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
	// and is harder only through its ascensions. 0 sets no floor.
	MinClear float64 `toml:"min_clear"`
}

type Rules struct {
	Name string `toml:"name"`

	Lives        int `toml:"lives"`
	MaxLives     int `toml:"max_lives"`
	Ducks        int `toml:"ducks"` // charges at boot start and per refill
	DoorsPerRoom int `toml:"doors_per_room"`

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

	// TimerFit: once the player has this many solves, door timers come from
	// their own solve times (quantile TimerQuantile, scaled by gap).
	TimerFitMin     int     `toml:"timer_fit_min"`
	TimerQuantile   float64 `toml:"timer_quantile"`
	RatingK         float64 `toml:"rating_k"` // Elo K for the player rating
	ColdStartRating float64 `toml:"cold_start_rating"`

	// ---- door selection ----------------------------------------------------

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
	FsckPrice int64 `toml:"fsck_price"` // +1 life, up to MaxLives
	DupPrice  int64 `toml:"dup_price"`  // +1 duck
	ShufPrice int64 `toml:"shuf_price"` // a re-roll of a room's doors, kept until used

	// ---- daemons -----------------------------------------------------------

	ForkDoors     int     `toml:"fork_doors"`      // doors per room after an AC
	NiceTimerFrac float64 `toml:"nice_timer_frac"` // timer bonus
	NiceLifeCost  int     `toml:"nice_life_cost"`  // lives per failed submit
	DevRandomMult float64 `toml:"dev_random_mult"` // extra pay on mystery doors

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
	// Ascensions add difficulty per prestige level on top of the rating
	// offset; a boot at prestige p applies every entry with Level <= p.
	Ascensions []Ascension `toml:"ascension"`
}

// Ascension is the extra difficulty of one prestige level. Levels stack like
// Dead Cells boss cells: prestige 3 plays with levels 1, 2 and 3.
type Ascension struct {
	Level         int     `toml:"level"`
	BlindChance   float64 `toml:"blind_chance"`   // added to the door roll
	MysteryChance float64 `toml:"mystery_chance"` // added to the door roll
	CursedChance  float64 `toml:"cursed_chance"`  // added to the door roll
	Lives         int     `toml:"lives"`          // added at boot (>= 1 left)
	Ducks         int     `toml:"ducks"`          // added at boot and refill
	TimerMult     float64 `toml:"timer_mult"`     // 0 means 1
}

// Ascend sums every ascension at or below prestige. TimerMult multiplies, so
// the result's TimerMult is never 0.
func (r Rules) Ascend(prestige int) Ascension {
	sum := Ascension{Level: prestige, TimerMult: 1}
	for _, a := range r.Ascensions {
		if a.Level < 1 || a.Level > prestige {
			continue
		}
		sum.BlindChance += a.BlindChance
		sum.MysteryChance += a.MysteryChance
		sum.CursedChance += a.CursedChance
		sum.Lives += a.Lives
		sum.Ducks += a.Ducks
		if a.TimerMult > 0 {
			sum.TimerMult *= a.TimerMult
		}
	}
	return sum
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
		{&r.MaxLives, &d.MaxLives}, {&r.TopicRatingMin, &d.TopicRatingMin},
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
		{&r.TimerHeadroom, &d.TimerHeadroom}, {&r.TimerFitLo, &d.TimerFitLo},
		{&r.TimerFitHi, &d.TimerFitHi}, {&r.BossTimerMult, &d.BossTimerMult},
		{&r.TimerQuantile, &d.TimerQuantile}, {&r.RatingK, &d.RatingK},
		{&r.ColdStartRating, &d.ColdStartRating}, {&r.RatingKCold, &d.RatingKCold},
		{&r.CalibMax, &d.CalibMax},
	}
	for _, f := range floats {
		if *f.v <= 0 {
			*f.v = *f.d
		}
	}
	if r.BaseBytes <= 0 {
		r.BaseBytes = d.BaseBytes
	}
	if r.OnExpire == "" {
		r.OnExpire = ExpireCrash
	}
	return r
}

// Standard is the default preset. Numbers come from the design discussion
// (DESIGN.md "A boot"); balance simulation will tune them.
func Standard() Rules {
	return Rules{
		Name:         "standard",
		Lives:        3,
		MaxLives:     5,
		Ducks:        1,
		DoorsPerRoom: 3,
		Biomes: []Biome{
			{Kind: Fixed, Name: "Userland", Rooms: 2, TargetClear: 0.92, Timer: Duration{15 * time.Minute}},
			// The mid biome is where learning happens, so prestige holds it at
			// 0.6 or above (D22) and adds risk instead.
			{Kind: Choice, Rooms: 2, Choices: 2, TargetClear: 0.7, MinClear: 0.6, Timer: Duration{30 * time.Minute}},
			{Kind: Boss, Rooms: 1, TargetClear: 0.5, Timer: Duration{50 * time.Minute}},
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

		ForkDoors:     4,
		NiceTimerFrac: 0.2,
		NiceLifeCost:  2,
		DevRandomMult: 2,

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
			{Level: 4, Lives: -1, CursedChance: 0.04, TimerMult: 0.9},
		},
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
	r := Standard()
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
	if r.Lives < 1 || r.DoorsPerRoom < 1 || len(r.Biomes) == 0 {
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
	}
	if r.TimerFitLo > r.TimerFitHi {
		return fmt.Errorf("rules %q: timer_fit_lo above timer_fit_hi", r.Name)
	}
	if r.OnExpire != "" && r.OnExpire != ExpireCrash && r.OnExpire != ExpireOvertime {
		return fmt.Errorf("rules %q: on_expire %q is not %q or %q", r.Name, r.OnExpire, ExpireCrash, ExpireOvertime)
	}
	if r.FsckPrice < 0 || r.DupPrice < 0 || r.ShufPrice < 0 {
		return fmt.Errorf("rules %q: need fsck_price, dup_price, shuf_price >= 0", r.Name)
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
	return r.validateBytes()
}

// MaxBaseBytes bounds base_bytes, MaxBytesMultiplier each multiplier of a
// door's pay, and MaxDoorBytes what one door can pay with every multiplier
// stacked; Standard's richest door pays 6 KiB. The bounds keep the sums
// over a lifetime of boots (the bank, lifetime bytes) far from int64's
// limit: a preset with base_bytes near it overflowed on the first clear.
const (
	MaxBaseBytes       = 1 << 30
	MaxBytesMultiplier = 1000
	MaxDoorBytes       = 1 << 40
)

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
	}
	for _, m := range mults {
		// TOML has inf and nan; the negated test also catches NaN.
		if m.v < 0 || !(m.v <= MaxBytesMultiplier) {
			return fmt.Errorf("rules %q: %s %v not in 0..%d", r.Name, m.key, m.v, MaxBytesMultiplier)
		}
	}
	// The richest door stacks every multiplier as doorBytes in
	// internal/game does: the difficulty factor at its cap, mystery (with
	// dev-random) or blind, cursed, a boss, and overtime pay. A zero is
	// the default WithDefaults fills in.
	d := r.WithDefaults()
	worst := float64(d.BaseBytes) * max(d.MaxBytesMult, d.MinBytesMult) *
		max(d.MysteryMult*d.DevRandomMult, d.BlindMult, 1) * max(d.CursedMult, 1) *
		max(d.BossMult, 1) * max(d.OvertimeBytesMult, 1)
	if worst > MaxDoorBytes {
		return fmt.Errorf("rules %q: a door could pay %.3g bytes, over the %d allowed; lower base_bytes or the *_mult keys",
			r.Name, worst, int64(MaxDoorBytes))
	}
	return nil
}
