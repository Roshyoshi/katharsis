// Package rules holds the tunable numbers of a boot. A preset is a TOML
// file; the engine snapshots the Rules into each boot, so editing a preset
// never changes a boot in progress. Every field has a default in
// Standard(); presets only need to override what they change.
package rules

import (
	"fmt"
	"os"
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

	// Bytes for clearing a door at TargetClear 0.5; easier doors pay less
	// (scaled by difficulty relative to the player), bosses pay BossMult.
	BaseBytes int64   `toml:"base_bytes"`
	BossMult  float64 `toml:"boss_mult"`
	// O3Frac: AC within this fraction of the timer grants a free upgrade.
	O3Frac float64 `toml:"o3_frac"`

	// Upgrade sizes.
	OverclockFrac float64 `toml:"overclock_frac"` // timer bonus per Overclock

	// Regressions: days after a crash or editorial-assisted solve when the
	// problem returns.
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
			{Kind: Choice, Rooms: 2, Choices: 2, TargetClear: 0.7, Timer: Duration{30 * time.Minute}},
			{Kind: Boss, Rooms: 1, TargetClear: 0.5, Timer: Duration{50 * time.Minute}},
		},
		BlindChance:     0.25,
		MysteryChance:   0.08,
		CursedChance:    0.08,
		BlindMult:       1.5,
		MysteryMult:     2,
		CursedMult:      2,
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
		CalibRate:          0.02,
		CalibWarm:          10,
		CalibAfter:         60,
		CalibMax:           300,

		OnExpire:          ExpireCrash,
		OvertimeScore:     0.5,
		OvertimeBytesMult: 0.5,

		PrestigeOffset: 25,
		Ascensions: []Ascension{
			{Level: 1, BlindChance: 0.15},
			{Level: 2, Ducks: -1, MysteryChance: 0.05},
			{Level: 3, CursedChance: 0.07, TimerMult: 0.9},
			{Level: 4, Lives: -1},
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
	if err := toml.Unmarshal(b, &r); err != nil {
		return r, fmt.Errorf("%s: %w", path, err)
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
	if r.CalibRate < 0 || r.CalibRate >= 1 || r.CalibWarm < 0 || r.CalibAfter < 0 || r.RatingColdAttempts < 0 {
		return fmt.Errorf("rules %q: need 0 <= calib_rate < 1 and calib_warm, calib_after, rating_cold_attempts >= 0", r.Name)
	}
	for _, a := range r.Ascensions {
		if a.Level < 1 {
			return fmt.Errorf("rules %q: ascension level %d < 1", r.Name, a.Level)
		}
	}
	return nil
}
