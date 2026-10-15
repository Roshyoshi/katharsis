package game

import (
	"fmt"
	"strconv"
	"time"
)

// NightlySeed is the nightly build's seed for the day t falls on in t's own
// location (hand it local time): the date's decimal digits read as hex, so
// 2026-10-14 is seed 0x20261014. Each day has one seed, git log shows it as
// the date, and it is never 0 (which StartBoot takes as random). The engine
// mixes its seed through splitmix64, so neighbouring days still roll
// unrelated boots. The seed fixes the engine's draws, but doors are still
// aimed at the player's rating and history, so a replay can meet other
// doors.
func NightlySeed(t time.Time) uint64 {
	y, mo, d := t.Date()
	seed, _ := strconv.ParseUint(fmt.Sprintf("%04d%02d%02d", y, int(mo), d), 16, 64)
	return seed
}

// Nightly reports whether the boot played its own day's nightly seed, the
// day it started on in the location it started in.
func (s *State) Nightly() bool { return s.Seed != 0 && s.Seed == NightlySeed(s.Started) }
