package sim_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"katharsis/internal/catalog"
	"katharsis/internal/game/sim"
	"katharsis/internal/rules"
)

// cachedPool is the judgeable problems of a content source's cached index
// (source "usaco" or "codeforces"), read straight from the cache file (no
// network, no guide snapshot), or nil when there is no cache.
// KATHARSIS_<SOURCE>_INDEX overrides the path.
func cachedPool(t *testing.T, source string) []catalog.Problem {
	t.Helper()
	path := os.Getenv("KATHARSIS_" + strings.ToUpper(source) + "_INDEX")
	if path == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			return nil
		}
		path = filepath.Join(dir, "katharsis", source, "index.json")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var idx struct{ Entries []catalog.Problem }
	if err := json.Unmarshal(b, &idx); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var ps []catalog.Problem
	for _, p := range idx.Entries {
		if p.Judgeable {
			ps = append(ps, p)
		}
	}
	return ps
}

// coldBoots is how many boots the engine's rating took to come within 100
// of where it settled (the mean of the last half of the run).
func coldBoots(rs []float64) int {
	if len(rs) == 0 {
		return 0
	}
	tail := rs[len(rs)/2:]
	sum := 0.0
	for _, r := range tail {
		sum += r
	}
	settled := sum / float64(len(tail))
	for i, r := range rs {
		if math.Abs(r-settled) < 100 {
			return i + 1
		}
	}
	return len(rs)
}

// blocks renders a run's time series, one line per block.
func blocks(rep sim.Report) string {
	var w strings.Builder
	at := 0
	for _, b := range rep.Blocks {
		var offs []string
		for _, k := range slices.Sorted(maps.Keys(b.Offsets)) {
			offs = append(offs, fmt.Sprintf("%s %+.0f", k, b.Offsets[k]))
		}
		fmt.Fprintf(&w, "  boots %3d-%3d: full clears %4.1f%%, off target %3.0f%%, rating %.0f, offsets [%s]\n",
			at+1, at+b.Boots, 100*float64(b.FullClears)/float64(b.Boots),
			100*float64(b.OffTarget)/float64(max(b.Opened, 1)), b.Rating, strings.Join(offs, ", "))
		at += b.Boots
	}
	return w.String()
}

// TestBalanceReport prints the numbers balance decisions are made from, on
// a synthetic pool large enough never to run dry and on the cached USACO
// pool: full clears, crashes and door clear rates per biome for the two
// reference players at prestige 0 and climbing, with the run's time series
// (cold start, calibration, pool exhaustion). It is a report, not a check;
// set KATHARSIS_BALANCE=1 and run with -v.
func TestBalanceReport(t *testing.T) {
	if os.Getenv("KATHARSIS_BALANCE") == "" {
		t.Skip("set KATHARSIS_BALANCE=1 for the balance report")
	}
	type pool struct {
		name string
		ps   []catalog.Problem
	}
	pools := []pool{{"synthetic 20000", sim.SyntheticPool(20000, 1)}}
	usaco, cf := cachedPool(t, "usaco"), cachedPool(t, "codeforces")
	if usaco != nil {
		pools = append(pools, pool{fmt.Sprintf("usaco %d", len(usaco)), usaco})
	}
	if usaco != nil && cf != nil {
		pools = append(pools, pool{fmt.Sprintf("usaco+codeforces %d", len(usaco)+len(cf)), slices.Concat(usaco, cf)})
	}
	if only := os.Getenv("KATHARSIS_BALANCE_POOL"); only != "" {
		pools = slices.DeleteFunc(pools, func(p pool) bool { return !strings.HasPrefix(p.name, only) })
	}
	// KATHARSIS_BALANCE_BOOTS lengthens the run, to see how long a pool
	// lasts; blocks grow with it.
	boots := 400
	if n, err := strconv.Atoi(os.Getenv("KATHARSIS_BALANCE_BOOTS")); err == nil && n > 0 {
		boots = n
	}
	for _, pool := range pools {
		for _, rating := range []float64{1000, 1600} {
			for _, prestige := range []int{0, -1} {
				rep := sim.Run(sim.Config{
					Rules:    rules.Standard(),
					Pool:     pool.ps,
					Player:   sim.DefaultPlayer(rating),
					Boots:    boots,
					Warmup:   50,
					Seed:     7,
					Prestige: prestige,
					Block:    max(50, (boots+50)/9),
				})
				t.Logf("%s, player %.0f, prestige %d (boots 51-%d; rating settled after %d boots):\n%s%s",
					pool.name, rating, prestige, boots+50, coldBoots(rep.Ratings), rep, blocks(rep))
			}
		}
	}
}
