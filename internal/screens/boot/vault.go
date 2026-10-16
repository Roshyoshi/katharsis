package boot

import (
	"fmt"
	"strings"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// roman names an act by its index: "I", "II", "III".
func roman(i int) string {
	if r := []string{"I", "II", "III", "IV", "V"}; i >= 0 && i < len(r) {
		return r[i]
	}
	return fmt.Sprint(i + 1)
}

// slack is a split's lead or lag, signed, toned: +6:40 ahead, −2:10
// behind.
func slack(d time.Duration) string {
	if d < 0 {
		return ui.Danger.Render("−" + units.Clock(-d))
	}
	return ui.OK.Render("+" + units.Clock(d))
}

// vaultSplit is the act's split against its vaults while its boss stands,
// for the doors screen, a speedrun's split line: "act I vaults · timed
// +6:40 · untouched ✓". "" for an act without vaults or once judged.
func vaultSplit(st *game.State) string {
	sp, ok := st.ActSplit(st.Biome)
	if !ok || len(st.ActVaults(st.Biome)) > 0 {
		return ""
	}
	r := st.Rules
	var parts []string
	if r.TimedVault > 0 {
		parts = append(parts, ui.Muted.Render("timed ")+slack(sp.Slack()))
	}
	if r.UntouchedVault {
		if sp.Hits == 0 {
			parts = append(parts, ui.Muted.Render("untouched ")+ui.OK.Render(ui.G.Pass))
		} else {
			parts = append(parts, ui.Muted.Render("untouched ")+ui.Danger.Render(ui.G.Fail+" "+units.Plural(sp.Hits, "hit")))
		}
	}
	return ui.Muted.Render("act "+roman(sp.Act)+" vaults"+ui.G.Sep) + strings.Join(parts, ui.Muted.Render(ui.G.Sep))
}

// vaultRows are the merge window's rows for the vaults its boss judged,
// under label: each door open or sealed, and why.
//
//	vaults   ✓ timed      38:12 in doors, within 45:00 · +1 pick
//	         ✗ untouched  2 hits
func vaultRows(st *game.State, label func(string) string) []string {
	if st.Phase != game.PhaseMerge {
		return nil
	}
	vs := st.ActVaults(st.Biome)
	if len(vs) == 0 || st.Biome >= len(st.Biomes) || st.ActOf(st.Biome) != vs[0].Act || !atActEnd(st) {
		return nil
	}
	var out []string
	for i, v := range vs {
		head := strings.Repeat(" ", widthOf(label("")))
		if i == 0 {
			head = label("vaults")
		}
		mark := ui.Danger.Render(ui.G.Fail)
		if v.Open {
			mark = ui.OK.Render(ui.G.Pass)
		}
		why := ""
		switch v.Kind {
		case game.VaultTimed:
			within := "over"
			if v.Took <= v.Limit {
				within = "within"
			}
			why = fmt.Sprintf("%s in doors, %s %s", units.Clock(v.Took), within, units.Clock(v.Limit))
		case game.VaultUntouched:
			why = "no door hit"
			if v.Hits > 0 {
				why = units.Plural(v.Hits, "hit")
			}
		}
		line := head + mark + " " + fmt.Sprintf("%-10s", v.Kind) + ui.Muted.Render(why)
		if v.Open {
			line += ui.Muted.Render(ui.G.Sep) + ui.OK.Render(fmt.Sprintf("+%d", v.Picks)) + ui.Muted.Render(" "+pluralWord(v.Picks, "pick"))
		}
		out = append(out, line)
	}
	return out
}

// atActEnd reports whether the boot stands at its act's last tier, the
// boss: its merge window is the one after the vaults were judged.
func atActEnd(st *game.State) bool {
	acts := st.Acts()
	a := st.ActOf(st.Biome)
	return a < len(acts) && acts[a].Last == st.Biome
}

func pluralWord(n int, w string) string {
	if n == 1 {
		return w
	}
	return w + "s"
}

// openVaults names the vaults of the act just judged that owe picks, for
// the upgrade screen's count: "the timed door".
func openVaults(st *game.State) []string {
	if st.Phase != game.PhaseMerge || !atActEnd(st) {
		return nil
	}
	var out []string
	for _, v := range st.ActVaults(st.Biome) {
		if v.Open && v.Picks > 0 {
			out = append(out, "the "+v.Kind+" door")
		}
	}
	return out
}
