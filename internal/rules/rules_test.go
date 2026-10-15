package rules_test

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"katharsis/internal/rules"
)

func TestStandard(t *testing.T) {
	t.Parallel()
	r := rules.Standard()
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := r.WithDefaults(); !reflect.DeepEqual(got, r) {
		t.Errorf("WithDefaults changed Standard():\n%+v\n%+v", got, r)
	}
}

func TestWithDefaults(t *testing.T) {
	t.Parallel()
	// Rules written before the engine's fields existed.
	old := rules.Rules{
		Name: "old", Lives: 2, DoorsPerRoom: 2, BaseBytes: 10,
		Biomes: []rules.Biome{{Kind: rules.Fixed, Rooms: 1, TargetClear: 0.8, Timer: rules.Duration{Duration: time.Minute}}},
	}
	got := old.WithDefaults()
	d := rules.Standard()
	if got.Lives != 2 || got.DoorsPerRoom != 2 || got.BaseBytes != 10 || len(got.Biomes) != 1 {
		t.Errorf("set fields changed: %+v", got)
	}
	if got.ForkDoors != d.ForkDoors || got.TimerGapScale != d.TimerGapScale || got.RatingK != d.RatingK ||
		got.RatingKCold != d.RatingKCold || got.CalibMax != d.CalibMax || got.OnExpire != rules.ExpireCrash ||
		!reflect.DeepEqual(got.RatingWindows, d.RatingWindows) || !reflect.DeepEqual(got.FreeVerdicts, d.FreeVerdicts) {
		t.Errorf("unset fields not defaulted: %+v", got)
	}
	// Zero turns these off, which is what such Rules did before the
	// fields existed.
	if got.BlindChance != 0 || got.PrestigeOffset != 0 || got.Ascensions != nil ||
		got.CalibRate != 0 || got.CalibWarm != 0 || got.CalibAfter != 0 || got.RatingColdAttempts != 0 ||
		got.TopicFollow != 0 || got.OvertimeScore != 0 || got.OvertimeBytesMult != 0 || got.ShowFailingInput ||
		got.MasteryAttempts != 0 || got.MasteryMargin != 0 || got.Biomes[0].MinClear != 0 {
		t.Errorf("zero-is-meaningful fields defaulted: %+v", got)
	}
	empty := old
	empty.FreeVerdicts = []string{}
	if got := empty.WithDefaults().FreeVerdicts; len(got) != 0 {
		t.Errorf("an explicit empty FreeVerdicts became %v", got)
	}
}

func TestTrial(t *testing.T) {
	t.Parallel()
	r := rules.Trial()
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := r.WithDefaults(); !reflect.DeepEqual(got, r) {
		t.Errorf("WithDefaults changed Trial():\n%+v\n%+v", got, r)
	}
	if r.Name != "trial" || r.OnExpire != rules.ExpireOvertime || !r.ShowFailingInput || r.DoorsPerRoom != 1 ||
		len(r.Biomes) != 2 || r.Biomes[0].Rooms != 4 || r.Biomes[1].Rooms != 2 ||
		r.BlindChance != 0 || r.MysteryChance != 0 || r.CursedChance != 0 || r.Ascensions != nil {
		t.Errorf("Trial() = %+v", r)
	}
	// Trial() is a copy: changing it leaves Standard() alone.
	r.Biomes[0].Rooms = 9
	if rules.Trial().Biomes[0].Rooms != 4 || len(rules.Standard().Biomes) != 3 {
		t.Error("Trial() shares its biomes")
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		edit func(*rules.Rules)
		err  string
	}{
		{"standard", func(*rules.Rules) {}, ""},
		{"overtime", func(r *rules.Rules) { r.OnExpire = rules.ExpireOvertime }, ""},
		{"empty on_expire", func(r *rules.Rules) { r.OnExpire = "" }, ""},
		{"bad on_expire", func(r *rules.Rules) { r.OnExpire = "pause" }, "on_expire"},
		{"negative calib_rate", func(r *rules.Rules) { r.CalibRate = -0.1 }, "calib_rate"},
		{"calib_rate of 1", func(r *rules.Rules) { r.CalibRate = 1 }, "calib_rate"},
		{"negative calib_after", func(r *rules.Rules) { r.CalibAfter = -1 }, "calib_after"},
		{"negative cold start", func(r *rules.Rules) { r.RatingColdAttempts = -1 }, "rating_cold_attempts"},
		// qa/chaos/presets/huge.toml: overflow bait.
		{"huge base_bytes", func(r *rules.Rules) { r.BaseBytes = math.MaxInt64 }, "base_bytes"},
		{"negative base_bytes", func(r *rules.Rules) { r.BaseBytes = -1 }, "base_bytes"},
		{"huge boss_mult", func(r *rules.Rules) { r.BossMult = 1e300 }, "boss_mult"},
		{"inf mystery_mult", func(r *rules.Rules) { r.MysteryMult = math.Inf(1) }, "mystery_mult"},
		{"nan cursed_mult", func(r *rules.Rules) { r.CursedMult = math.NaN() }, "cursed_mult"},
		{"negative blind_mult", func(r *rules.Rules) { r.BlindMult = -2 }, "blind_mult"},
		{"stacked pay", func(r *rules.Rules) {
			r.BaseBytes, r.MaxBytesMult, r.BossMult, r.CursedMult = rules.MaxBaseBytes, 100, 100, 100
		}, "a door could pay"},
		{"rich but bounded", func(r *rules.Rules) { r.BaseBytes, r.BossMult = 1<<20, 1000 }, ""},
		{"min_clear at target_clear", func(r *rules.Rules) { r.Biomes[1].MinClear = r.Biomes[1].TargetClear }, ""},
		{"min_clear above target_clear", func(r *rules.Rules) { r.Biomes[1].MinClear = 0.8 }, "min_clear"},
		{"negative min_clear", func(r *rules.Rules) { r.Biomes[0].MinClear = -0.1 }, "min_clear"},
		{"negative mastery_attempts", func(r *rules.Rules) { r.MasteryAttempts = -1 }, "mastery_attempts"},
	}
	for _, tc := range tests {
		r := rules.Standard()
		tc.edit(&r)
		err := r.Validate()
		if tc.err == "" && err != nil || tc.err != "" && (err == nil || !strings.Contains(err.Error(), tc.err)) {
			t.Errorf("%s: Validate = %v, want %q", tc.name, err, tc.err)
		}
	}
}

func TestAscend(t *testing.T) {
	t.Parallel()
	r := rules.Standard()
	tests := []struct {
		prestige            int
		blind, mystery, cur float64
		lives, ducks        int
		timer               float64
	}{
		{0, 0, 0, 0, 0, 0, 1},
		{1, 0.15, 0, 0, 0, 0, 0.95},
		{2, 0.15, 0.05, 0.04, 0, -1, 0.95},
		{3, 0.15, 0.05, 0.09, 0, -1, 0.855},
		{4, 0.15, 0.05, 0.13, -1, -1, 0.7695},
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-12 }
	for _, tc := range tests {
		a := r.Ascend(tc.prestige)
		if !near(a.BlindChance, tc.blind) || !near(a.MysteryChance, tc.mystery) || !near(a.CursedChance, tc.cur) ||
			a.Lives != tc.lives || a.Ducks != tc.ducks || !near(a.TimerMult, tc.timer) {
			t.Errorf("Ascend(%d) = %+v", tc.prestige, a)
		}
	}
}

func TestLoad(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		toml  string
		check func(rules.Rules) bool
		err   string
	}{
		{"overrides a scalar", `lives = 4`, func(r rules.Rules) bool {
			return r.Lives == 4 && len(r.Biomes) == 3 && len(r.Ascensions) == 4
		}, ""},
		{"biome tables replace Standard's", `
[[biome]]
kind = "boss"
rooms = 1
target_clear = 0.5
min_clear = 0.4
timer = "40m"
`, func(r rules.Rules) bool {
			return len(r.Biomes) == 1 && r.Biomes[0].Name == "" && r.Biomes[0].Timer.Duration == 40*time.Minute &&
				r.Biomes[0].MinClear == 0.4
		}, ""},
		{"ascension tables replace Standard's", `
[[ascension]]
level = 2
lives = -2
`, func(r rules.Rules) bool {
			return len(r.Ascensions) == 1 && r.Ascensions[0].BlindChance == 0 && r.Ascend(2).Lives == -2
		}, ""},
		{"bad kind", `
[[biome]]
kind = "bonus"
rooms = 1
target_clear = 0.5
timer = "40m"
`, nil, "not fixed, choice or boss"},
		{"bad ascension level", "[[ascension]]\nlevel = 0\n", nil, "ascension level"},
		{"soft timer and calibration knobs", "on_expire = \"overtime\"\ncalib_rate = 0.05\ncalib_after = 10\ntopic_follow = 0\n",
			func(r rules.Rules) bool {
				return r.OnExpire == rules.ExpireOvertime && r.CalibRate == 0.05 && r.CalibAfter == 10 &&
					r.TopicFollow == 0 && r.CalibWarm == rules.Standard().CalibWarm
			}, ""},
		{"bad on_expire", "on_expire = \"never\"\n", nil, "on_expire"},
	}
	for _, tc := range tests {
		path := filepath.Join(t.TempDir(), "preset.toml")
		if err := os.WriteFile(path, []byte(tc.toml), 0o644); err != nil {
			t.Fatal(err)
		}
		r, err := rules.Load(path)
		if tc.err != "" {
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("%s: err %v, want %q", tc.name, err, tc.err)
			}
			continue
		}
		if err != nil || !tc.check(r) {
			t.Errorf("%s: %v %+v", tc.name, err, r)
		}
	}
}

func TestLoadStrictKeysAndFileName(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if _, err := rules.Load(write("typo.toml", "lifes = 5\n")); err == nil || !strings.Contains(err.Error(), "unknown keys lifes") {
		t.Errorf("misspelt key: err %v, want unknown keys lifes", err)
	}
	r, err := rules.Load(write("short.toml", "lives = 2\n"))
	if err != nil || r.Name != "short" || r.Lives != 2 {
		t.Errorf("unnamed preset: %q lives %d, %v; want short, 2", r.Name, r.Lives, err)
	}
	if r, err := rules.Load(write("x.toml", "name = \"speed\"\n")); err != nil || r.Name != "speed" {
		t.Errorf("named preset: %q, %v; want speed", r.Name, err)
	}
}

// The standard packages are a goal ladder (retention RT3): bought cheapest
// first, the first costs at most one boot's bank, every next one at most
// three, and the catalog lasts at least 40 boots. A capstone above three
// boots may come only once the ladder under it has lasted 40 boots of the
// weaker player. Boots are counted at what the sim's model players bank
// per boot over their first 40 boots at prestige 0 (commits and upsolved
// core dumps): sim TestAptLadder measures them (203 and 249 B when these
// were set, with RT0's floor reading and D44) and holds the catalog to the
// same rules at the real rates.
func TestPackageLadder(t *testing.T) {
	t.Parallel()
	const (
		weak   = 203 // B a boot, the 1000-rated model
		strong = 249 // B a boot, the 1600-rated model
	)
	ps := rules.Standard().Packages
	if len(ps) < 14 {
		t.Errorf("%d packages, want at least 14", len(ps))
	}
	prices := make([]int64, len(ps))
	for i, p := range ps {
		prices[i] = p.Price
	}
	slices.Sort(prices)
	if prices[0] > weak {
		t.Errorf("the first package costs %d B, more than a boot (%d B)", prices[0], weak)
	}
	var sum int64
	for _, p := range prices {
		if p > 3*weak && sum < 40*weak {
			t.Errorf("a %d B package after %d B of the ladder: more than three boots before boot 40", p, sum)
		}
		sum += p
	}
	if sum < 40*strong {
		t.Errorf("the catalog costs %d B, under 40 boots of the stronger player (%d B)", sum, 40*strong)
	}
}

// The built-in presets (RT7) are standard, short and patch, in that order,
// each valid and differing from standard only in name and biomes. Short
// is boot #1's shape (FirstBiomes, RT11); patch is RT7's: one Userland
// door whose timeout costs a life, one mid door, a mini-boss at 0.6.
func TestBuiltIns(t *testing.T) {
	t.Parallel()
	var names []string
	for _, r := range rules.BuiltIns() {
		names = append(names, r.Name)
		if err := r.Validate(); err != nil {
			t.Error(err)
		}
		std := rules.Standard()
		std.Name, std.Biomes = r.Name, r.Biomes
		if !reflect.DeepEqual(r, std) {
			t.Errorf("%s changes more than its name and biomes", r.Name)
		}
	}
	if want := []string{"standard", "short", "patch"}; !slices.Equal(names, want) {
		t.Errorf("built-ins %v, want %v", names, want)
	}
	if !reflect.DeepEqual(rules.Standard().FirstBiomes, rules.Short().Biomes) {
		t.Errorf("Short %+v, Standard's first %+v", rules.Short().Biomes, rules.Standard().FirstBiomes)
	}
	timers := func(r rules.Rules) (sum time.Duration, doors int) {
		for _, b := range r.Biomes {
			sum += b.Timer.Duration * time.Duration(b.Rooms)
			doors += b.Rooms
		}
		return sum, doors
	}
	if sum, doors := timers(rules.Short()); sum != 65*time.Minute || doors != 4 {
		t.Errorf("short: %d doors, %v of timers", doors, sum)
	}
	p := rules.Patch()
	if len(p.Biomes) != 3 {
		t.Fatalf("patch biomes %+v", p.Biomes)
	}
	sum, doors := timers(p)
	kinds := []rules.BiomeKind{p.Biomes[0].Kind, p.Biomes[1].Kind, p.Biomes[2].Kind}
	if doors != 3 || sum != 50*time.Minute ||
		!slices.Equal(kinds, []rules.BiomeKind{rules.Fixed, rules.Choice, rules.Boss}) ||
		!p.Biomes[0].TimeoutLife || p.Biomes[2].TargetClear != 0.6 {
		t.Errorf("patch: %d doors, %v of timers, biomes %+v", doors, sum, p.Biomes)
	}
}
