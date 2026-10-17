package game

import (
	"slices"
	"time"

	"katharsis/internal/rules"
	"katharsis/internal/units"
)

// A merge window is where bytes become a choice (D25): commit them to the
// bank, where they survive any crash and buy packages at GRUB, or spend
// them on this boot. Spending also takes them out of reach of the core
// dump, so it is the way to cash in bytes a crash would cost.

// AllItems is what a merge window sells, in shelf order.
var AllItems = []Item{Fsck, Dup, Shuf, At}

// Price is what it costs in this boot's merge windows, 0 when it is not
// for sale: the rules price it at 0, the package that sells it was not
// installed, or the boot is a trial, which sells nothing, as it offers no
// daemons: a trial is practice, not a run to equip.
func (s *State) Price(it Item) int64 {
	if s.Trial != nil || s.ItemLocked(it) || !s.Stocked(it) {
		return 0
	}
	return s.basePrice(it)
}

// basePrice is what the rules price it at, wherever it stands; the
// roster's items only under its rules.
func (s *State) basePrice(it Item) int64 {
	p := &s.Rules.Perk
	if s.perks() {
		switch it {
		case Nohup:
			return p.NohupPrice
		case Popd:
			return p.PopdPrice
		case Diff:
			return p.DiffPrice
		case Whatis:
			return p.WhatisPrice
		case Sudo:
			return p.SudoPrice
		case Make:
			return p.MakePrice
		case Mesg:
			return p.MesgPrice
		}
	}
	switch it {
	case Fsck:
		return s.Rules.FsckPrice
	case Dup:
		return s.Rules.DupPrice
	case Shuf:
		return s.Rules.ShufPrice
	case At:
		return s.Rules.AtPrice
	}
	return 0
}

// CanSpend says why it cannot be bought now, or nil when Spend would take
// it: the boot is in a merge window, it is for sale, the uncommitted bytes
// cover it, and it would do something (fsck needs cycles to restore).
func (s *State) CanSpend(it Item) error {
	price := s.Price(it)
	switch {
	case s.Phase != PhaseMerge:
		return invalid("spend in phase %s", s.Phase)
	case price <= 0:
		return invalid("%q is not for sale", it)
	case s.Uncommitted < price:
		return invalid("%s costs %s, %s uncommitted", it, units.Bytes(price), units.Bytes(s.Uncommitted))
	case it == Fsck && s.Cycles >= s.MaxCycles:
		return invalid("cycles are full")
	case it == Mesg && s.Ducks == 0:
		return invalid("no ducks to turn into clock")
	case it == Make && s.PendingUpgrade:
		return invalid("pick the upgrade owed first")
	}
	return nil
}

func (e *engine) spend(it Item) error {
	s := e.s
	if err := s.CanSpend(it); err != nil {
		return err
	}
	price := s.Price(it)
	s.Uncommitted -= price
	s.Spent += price
	switch it {
	case Fsck:
		s.Cycles = min(s.Cycles+s.FsckHeal(), s.MaxCycles)
	case Dup:
		s.Ducks++
	case Shuf:
		s.Rerolls++
	case At:
		s.AtTime += s.Rules.AtTime.Duration
	case Nohup:
		s.Nohup++
	case Popd:
		s.FreeRetreats++
	case Diff:
		s.Diffs++
	case Whatis:
		s.Whatis++
	case Sudo:
		s.Sudo++
	case Make:
		e.owe(1)
	case Mesg:
		s.AtTime += time.Duration(s.Ducks) * s.Rules.Perk.MesgPerDuck.Duration
		s.Ducks = 0
	}
	e.logf("%s: -%s, %s uncommitted (cycles %d/%d, ducks %d, shufs %d, at +%s)",
		it, units.Bytes(price), units.Bytes(s.Uncommitted), s.Cycles, s.MaxCycles, s.Ducks, s.Rerolls,
		units.Clock(s.AtTime))
	return nil
}

// reroll spends a shuf on the room on offer: fresh doors, drawn as a
// retreat's are but none of those on offer while the pool has others. A
// regression door on offer stays, since a boot offers it once. A boss room
// cannot be re-rolled: a boss is one door with no choice, and the boss
// tier's calibration counts on that (a player who skips the bosses they
// fear would clear more of the rest than they are aimed at).
func (e *engine) reroll() error {
	s := e.s
	if err := e.need(PhaseDoors, "shuf"); err != nil {
		return err
	}
	switch {
	case s.PendingUpgrade:
		return invalid("pick an upgrade first")
	case s.Rerolls <= 0:
		return invalid("no shuf left")
	case s.Biomes[s.Biome].Kind == rules.Boss:
		return invalid("a boss can't be shuffled")
	}
	i := slices.IndexFunc(s.Doors, func(d Door) bool { return d.Regression })
	var reg Door
	if i >= 0 {
		reg = s.Doors[i]
	}
	s.Rerolls--
	e.logf("shuf: room_%d.%d re-rolled, %d left", s.Biome, s.Room, s.Rerolls)
	rng := s.RNG
	doors := e.drawDoors(s.Doors)
	if i >= 0 {
		n := len(doors)
		doors = slices.DeleteFunc(doors, func(d Door) bool { return d.Problem == reg.Problem })
		if len(doors) == n {
			doors = doors[:n-1]
		}
		doors = append(doors, reg)
	}
	e.offer(doors, "reroll", rng)
	return nil
}
