package game

import (
	"time"

	"katharsis/internal/rules"
	"katharsis/internal/units"
)

// The vaults are two challenge doors at each boss but the last, Dead
// Cells' timed and no-hit doors in contest terms: speed and first-submit
// accuracy, the two things an ICPC penalty counts. Each open one owes
// rules.VaultPicks upgrade picks as the boss falls.
const (
	VaultTimed     = "timed"     // the act within rules.TimedVault of its clocks
	VaultUntouched = "untouched" // no door of the act hit (BiomeState.Hits)
)

// Vault is one vault as it was judged when its act's boss fell.
type Vault struct {
	Kind  string
	Act   int // index in State.Acts
	Open  bool
	Picks int // what it owed: rules.VaultPicks when Open, else 0
	// The act's split when it was judged (Split).
	Took, Limit time.Duration
	Hits        int
}

// Split is an act measured against its vaults: Took, the time spent in
// every door of the act, against Limit, rules.TimedVault of its cleared
// doors' timers without the time bought for them (Attempt.Base); and Hits,
// its doors' charged failures. A door left (retreated, timed out) adds its
// time and none of its clock.
type Split struct {
	Act         int
	Took, Limit time.Duration
	Hits        int
}

// Slack is how far ahead of the timed vault's budget the act is; under 0
// is behind.
func (s Split) Slack() time.Duration { return s.Limit - s.Took }

// ActSplit is the split of the act biome i belongs to, so far; ok is false
// for an act without vaults: the one that ends the boot (the final, or an
// ending's node), a trial, or rules without them.
func (s *State) ActSplit(i int) (sp Split, ok bool) {
	r := &s.Rules
	acts := s.Acts()
	a := s.ActOf(i)
	if s.Trial != nil || !s.vaulted() || a >= len(acts) {
		return Split{}, false
	}
	act := acts[a]
	if act.Last >= len(s.Biomes)-1 || s.Biomes[act.Last].Kind != rules.Boss ||
		act.Last < len(r.Biomes) && r.Biomes[act.Last].Ending != "" {
		return Split{}, false
	}
	sp.Act = a
	var clock time.Duration
	for _, h := range s.History {
		if h.Biome < act.First || h.Biome > act.Last {
			continue
		}
		sp.Took += max(h.Closed.Sub(h.Opened), 0)
		if h.Outcome == Cleared {
			clock += h.Base
		}
	}
	sp.Limit = scale(clock, r.TimedVault)
	for b := act.First; b <= act.Last && b < len(s.Biomes); b++ {
		sp.Hits += s.Biomes[b].Hits
	}
	return sp, true
}

// ActVaults are the vaults judged for the act biome i belongs to, nil
// before its boss falls.
func (s *State) ActVaults(i int) []Vault {
	a := s.ActOf(i)
	var out []Vault
	for _, v := range s.Vaults {
		if v.Act == a {
			out = append(out, v)
		}
	}
	return out
}

// judgeVaults opens or seals the vaults of the act the boss just cleared
// ends, and returns the picks the open ones owe.
func (e *engine) judgeVaults() int {
	s, r := e.s, &e.s.Rules
	sp, ok := s.ActSplit(s.Biome)
	if !ok || len(s.ActVaults(s.Biome)) > 0 {
		return 0
	}
	picks := 0
	judge := func(kind string, on, open bool, why string) {
		if !on {
			return
		}
		v := Vault{Kind: kind, Act: sp.Act, Open: open, Took: sp.Took, Limit: sp.Limit, Hits: sp.Hits}
		word := "sealed"
		if open {
			v.Picks = r.VaultPicks
			picks += v.Picks
			word = "open, " + units.Plural(v.Picks, "upgrade pick")
		}
		s.Vaults = append(s.Vaults, v)
		e.logf("vault: %s door %s (%s)", kind, word, why)
	}
	judge(VaultTimed, r.TimedVault > 0, sp.Took <= sp.Limit,
		units.Clock(sp.Took)+" in doors against "+units.Clock(sp.Limit))
	judge(VaultUntouched, r.UntouchedVault, sp.Hits == 0, units.Plural(sp.Hits, "hit"))
	return picks
}

// vaulted reports whether the boot's rules have vaults, which alone keep
// their bookkeeping (BiomeState.Hits, Attempt.Base): a boot saved before
// them goes on as its engine played it.
func (s *State) vaulted() bool {
	return s.Rules.VaultPicks > 0 && (s.Rules.TimedVault > 0 || s.Rules.UntouchedVault)
}

// hit counts a charged failure on the open door's tier (BiomeState.Hits).
func (e *engine) hit() {
	if s := e.s; s.vaulted() && s.Biome >= 0 && s.Biome < len(s.Biomes) {
		s.Biomes[s.Biome].Hits++
	}
}
