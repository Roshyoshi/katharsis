package rules

import "fmt"

// PerkRules are the pick pools past the first seven daemons, four upgrades
// and four items, each pool ten from the first boot with apt's on top
// (docs/research/perks.md). Pool 0, as in a boot saved before them, keeps
// the old pools and their uniform draw, so a boot goes on as its engine
// played it. Every number is a setting (D2); bytes are a byproduct, so the
// perks pay in cycles, clock, ducks, picks and doors.
type PerkRules struct {
	Pool int `toml:"pool"` // 1: the ten-a-pool roster

	// Weights are a draw's odds by rarity: common, uncommon, rare (Slay
	// the Spire's relics, Balatro's jokers: the dramatic picks rare).
	Weights [3]int `toml:"weights"`
	// Stock is how many items a merge window has on its shelf, fsck always
	// among them, the rest drawn: Dead Cells' shops, not a catalogue.
	Stock int `toml:"stock"`
	// SwapPrice is a daemon taken over one running once every seat is
	// full, so the later windows have a daemon decision too.
	SwapPrice int64 `toml:"swap_price"`
	// DaemonTimerMax caps the clock the daemons multiply together (nice,
	// quiet, cpupower, selinux): stacked, they would run a door to 3.5x.
	DaemonTimerMax float64 `toml:"daemon_timer_max"`

	// Daemons.
	CcachePicks      int      `toml:"ccache_picks"`      // a clean AC after Userland
	KsmdCycles       int      `toml:"ksmd_cycles"`       // freed by each Boss I or II clear
	BurstFrac        float64  `toml:"burst_frac"`        // of a clear's unused clock, onto the next door
	BurstMax         Duration `toml:"burst_max"`         // the most it carries
	WatchdogOvertime Duration `toml:"watchdog_overtime"` // a timeout's overtime, once an act
	WatchdogCycles   int      `toml:"watchdog_cycles"`   // what the overtime burns
	QuietPerDuck     float64  `toml:"quiet_per_duck"`    // clock per duck a window would refill
	QuietMax         float64  `toml:"quiet_max"`         // the most it adds
	CpupowerTimer    float64  `toml:"cpupower_timer"`    // every clock
	CpupowerCycles   int      `toml:"cpupower_cycles"`   // the cycles cap
	SelinuxTimer     float64  `toml:"selinux_timer"`     // a cursed topic door's clock
	DevRandomPicks   int      `toml:"dev_random_picks"`  // a mystery door's clear
	CronCycles       int      `toml:"cron_cycles"`       // a regression door's clear
	KdumpCycles      int      `toml:"kdump_cycles"`      // where the capture kernel boots
	TunedO3Frac      float64  `toml:"tuned_o3_frac"`     // -O3's share of the clock

	// Upgrades.
	HeatsinkCycles int      `toml:"heatsink_cycles"` // up to the starting cycles
	SnapshotHold   int      `toml:"snapshot_hold"`   // free failed submits held at most
	PrefetchTime   Duration `toml:"prefetch_time"`   // on the next door
	PrefetchHold   int      `toml:"prefetch_hold"`   // prefetches held at most
	TurboFrac      float64  `toml:"turbo_frac"`      // on the next boss door's clock
	SmtDoors       int      `toml:"smt_doors"`       // more doors in the next room
	MicrocodeOffer int      `toml:"microcode_offer"` // more offers at later picks
	OfastPicks     int      `toml:"ofast_picks"`     // a cursed door's clear
	SkipCycles     int      `toml:"skip_cycles"`     // a pick skipped, up to the starting cycles

	// Items, by price, and what they do.
	NohupPrice  int64    `toml:"nohup_price"`
	PopdPrice   int64    `toml:"popd_price"`
	DiffPrice   int64    `toml:"diff_price"`
	WhatisPrice int64    `toml:"whatis_price"`
	SudoPrice   int64    `toml:"sudo_price"`
	SudoCycles  int      `toml:"sudo_cycles"` // a cursed door's fail under sudo
	MakePrice   int64    `toml:"make_price"`
	MesgPrice   int64    `toml:"mesg_price"`
	MesgPerDuck Duration `toml:"mesg_per_duck"` // each duck held, on the next door
}

// MapPerks are the map's perks (rules.Map), the research's defaults.
func MapPerks() PerkRules {
	return PerkRules{
		Pool: 1, Weights: [3]int{6, 3, 1}, Stock: 5, SwapPrice: 64, DaemonTimerMax: 1.75,

		CcachePicks: 1, KsmdCycles: 10, BurstFrac: 0.25, BurstMax: minutes(5),
		WatchdogOvertime: minutes(3), WatchdogCycles: 5, QuietPerDuck: 0.2, QuietMax: 0.4,
		CpupowerTimer: 1.4, CpupowerCycles: 20, SelinuxTimer: 1.5,
		DevRandomPicks: 1, CronCycles: 5, KdumpCycles: 10, TunedO3Frac: 2.0 / 3,

		HeatsinkCycles: 4, SnapshotHold: 3, PrefetchTime: minutes(5), PrefetchHold: 2,
		TurboFrac: 0.2, SmtDoors: 1, MicrocodeOffer: 1, OfastPicks: 2, SkipCycles: 2,

		NohupPrice: 64, PopdPrice: 32, DiffPrice: 40, WhatisPrice: 24,
		SudoPrice: 96, SudoCycles: 10, MakePrice: 72, MesgPrice: 16, MesgPerDuck: minutes(4),
	}
}

// validate rejects perks the engine cannot honour.
func (p PerkRules) validate(name string) error {
	switch {
	case p.Pool < 0 || p.Pool > 1:
		return fmt.Errorf("rules %q: perk pool %d: want 0 or 1", name, p.Pool)
	case p.Pool == 0:
		return nil
	case p.Weights[0] < 0 || p.Weights[1] < 0 || p.Weights[2] < 0 || p.Weights[0]+p.Weights[1]+p.Weights[2] <= 0:
		return fmt.Errorf("rules %q: perk weights %v: want >= 0, not all 0", name, p.Weights)
	case p.Stock < 1 || p.DaemonTimerMax < 1 || p.CpupowerCycles < 1 || p.KdumpCycles < 1:
		return fmt.Errorf("rules %q: need perk stock, daemon_timer_max, cpupower_cycles, kdump_cycles >= 1", name)
	case p.BurstFrac < 0 || p.BurstFrac > 1 || p.TunedO3Frac < 0 || p.TunedO3Frac > 1:
		return fmt.Errorf("rules %q: need perk burst_frac, tuned_o3_frac in 0..1", name)
	}
	for _, n := range []int{p.CcachePicks, p.KsmdCycles, p.WatchdogCycles, p.DevRandomPicks, p.CronCycles,
		p.HeatsinkCycles, p.SnapshotHold, p.PrefetchHold, p.SmtDoors, p.MicrocodeOffer, p.OfastPicks,
		p.SkipCycles, p.SudoCycles} {
		if n < 0 {
			return fmt.Errorf("rules %q: perk counts and cycles must be >= 0", name)
		}
	}
	for _, f := range []float64{p.QuietPerDuck, p.QuietMax, p.CpupowerTimer - 1, p.SelinuxTimer - 1, p.TurboFrac} {
		if !(f >= 0) {
			return fmt.Errorf("rules %q: perk clock bonuses must be >= 0 (multipliers >= 1)", name)
		}
	}
	for _, b := range []int64{p.SwapPrice, p.NohupPrice, p.PopdPrice, p.DiffPrice, p.WhatisPrice, p.SudoPrice, p.MakePrice, p.MesgPrice} {
		if b < 0 {
			return fmt.Errorf("rules %q: perk prices must be >= 0", name)
		}
	}
	return nil
}
