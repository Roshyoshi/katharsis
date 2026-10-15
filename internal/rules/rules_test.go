package rules_test

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
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
		got.TopicFollow != 0 || got.OvertimeScore != 0 || got.OvertimeBytesMult != 0 || got.ShowFailingInput {
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
		{1, 0.15, 0, 0, 0, 0, 1},
		{2, 0.15, 0.05, 0, 0, -1, 1},
		{3, 0.15, 0.05, 0.07, 0, -1, 0.9},
		{4, 0.15, 0.05, 0.07, -1, -1, 0.9},
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
timer = "40m"
`, func(r rules.Rules) bool {
			return len(r.Biomes) == 1 && r.Biomes[0].Name == "" && r.Biomes[0].Timer.Duration == 40*time.Minute
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
