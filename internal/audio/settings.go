package audio

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/BurntSushi/toml"
)

// Duration reads "3s" from TOML.
type Duration struct{ time.Duration }

func (d *Duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	d.Duration = v
	return err
}

func (d Duration) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// Settings are the player's audio preferences, read from
// ~/.config/katharsis/audio.toml; a file lists only what it changes.
// Unlike rules they are not locked per boot: they change no outcome.
type Settings struct {
	Music bool `toml:"music"` // music on at start (ToggleMusic flips it)
	SFX   bool `toml:"sfx"`   // effects on

	// Volumes run 0 to 1 on a squared taper (gain = v², so 0.5 is -12
	// dB), which tracks perceived loudness better than a linear one.
	MusicVolume float64 `toml:"music_volume"`
	SFXVolume   float64 `toml:"sfx_volume"`

	// MusicLUFS and SFXLUFS are the loudness each bus is normalized to at
	// volume 1, in LUFS (ITU-R BS.1770; see loudness.go). Effects sit 4 LU
	// above the music so a verdict is heard over it.
	MusicLUFS float64 `toml:"music_lufs"`
	SFXLUFS   float64 `toml:"sfx_lufs"`

	Crossfade Duration `toml:"crossfade"`  // a cue change: biome, merge window, boss
	TrackFade Duration `toml:"track_fade"` // the next track of the same pool
	// Repeats is how many times a track plays before the pool moves on;
	// a pool of one track loops it forever.
	Repeats int `toml:"repeats"`

	// DuckGain is the music's gain while a submission judges (0.5 is
	// -6 dB); DuckAttack and DuckRelease are how fast it dips and returns.
	DuckGain    float64  `toml:"duck_gain"`
	DuckAttack  Duration `toml:"duck_attack"`
	DuckRelease Duration `toml:"duck_release"`

	// WarnAt is how much of a door's timer is left when the warning
	// sounds; 0 turns it off.
	WarnAt Duration `toml:"warn_at"`
	// Toast is how long the "♪ title, arr. by" line stays; 0 hides it.
	Toast Duration `toml:"toast"`

	// UserMusic is what a non-empty music/<pool or key>/ folder does:
	// "override" plays only the player's tracks there, "extend" adds
	// them to the pool.
	UserMusic string `toml:"user_music"`
	// Panic is the pool the kernel panic plays, "" for silence: a halted
	// kernel makes no sound.
	Panic string `toml:"panic"`

	// DeviceBuffer is the output buffer: smaller is less latency, larger
	// is less CPU. IdleSuspend stops the output after this long with
	// nothing to play, which takes the idle cost from ~2% of a core to ~0.
	DeviceBuffer Duration `toml:"device_buffer"`
	IdleSuspend  Duration `toml:"idle_suspend"`
}

// DefaultSettings are the measured defaults (docs/research/audio.md has
// the numbers).
func DefaultSettings() Settings {
	return Settings{
		Music:        true,
		SFX:          true,
		MusicVolume:  0.7,
		SFXVolume:    0.8,
		MusicLUFS:    -20,
		SFXLUFS:      -16,
		Crossfade:    Duration{3 * time.Second},
		TrackFade:    Duration{1500 * time.Millisecond},
		Repeats:      2,
		DuckGain:     0.5,
		DuckAttack:   Duration{150 * time.Millisecond},
		DuckRelease:  Duration{800 * time.Millisecond},
		WarnAt:       Duration{60 * time.Second},
		Toast:        Duration{6 * time.Second},
		UserMusic:    "override",
		Panic:        "",
		DeviceBuffer: Duration{40 * time.Millisecond},
		IdleSuspend:  Duration{3 * time.Second},
	}
}

// Validate rejects settings the engine cannot honour.
func (s Settings) Validate() error {
	for _, v := range []struct {
		name string
		v    float64
	}{{"music_volume", s.MusicVolume}, {"sfx_volume", s.SFXVolume}, {"duck_gain", s.DuckGain}} {
		if math.IsNaN(v.v) || v.v < 0 || v.v > 1 {
			return fmt.Errorf("audio: %s %v is outside 0..1", v.name, v.v)
		}
	}
	for _, v := range []struct {
		name string
		v    float64
	}{{"music_lufs", s.MusicLUFS}, {"sfx_lufs", s.SFXLUFS}} {
		if math.IsNaN(v.v) || v.v < -60 || v.v > -6 {
			return fmt.Errorf("audio: %s %v is outside -60..-6", v.name, v.v)
		}
	}
	for _, d := range []struct {
		name string
		d    time.Duration
	}{
		{"crossfade", s.Crossfade.Duration}, {"track_fade", s.TrackFade.Duration},
		{"duck_attack", s.DuckAttack.Duration}, {"duck_release", s.DuckRelease.Duration},
		{"warn_at", s.WarnAt.Duration}, {"toast", s.Toast.Duration},
		{"idle_suspend", s.IdleSuspend.Duration},
	} {
		if d.d < 0 {
			return fmt.Errorf("audio: %s %v is negative", d.name, d.d)
		}
	}
	if b := s.DeviceBuffer.Duration; b < 5*time.Millisecond || b > time.Second {
		return fmt.Errorf("audio: device_buffer %v is outside 5ms..1s", b)
	}
	if s.Repeats < 1 {
		return fmt.Errorf("audio: repeats %d is below 1", s.Repeats)
	}
	if s.UserMusic != "override" && s.UserMusic != "extend" {
		return fmt.Errorf("audio: user_music %q is not override or extend", s.UserMusic)
	}
	if s.Panic != "" && !slices.Contains(Pools, s.Panic) {
		return fmt.Errorf("audio: panic pool %q is not one of %v", s.Panic, Pools)
	}
	return nil
}

// ConfigDir is $XDG_CONFIG_HOME/katharsis, else ~/.config/katharsis, as
// the presets use.
func ConfigDir() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" && filepath.IsAbs(d) {
		return filepath.Join(d, "katharsis"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "katharsis"), nil
}

// LoadSettings reads path over the defaults. A missing file is the
// defaults; a bad one is an error, so a typo is not silently ignored.
func LoadSettings(path string) (Settings, error) {
	s := DefaultSettings()
	md, err := toml.DecodeFile(path, &s)
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultSettings(), nil
	}
	if err != nil {
		return s, fmt.Errorf("audio settings %s: %w", path, err)
	}
	if un := md.Undecoded(); len(un) > 0 {
		return s, fmt.Errorf("audio settings %s: unknown key %q", path, un[0].String())
	}
	if err := s.Validate(); err != nil {
		return s, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

// gainOf maps a 0..1 volume to linear gain (squared taper).
func gainOf(v float64) float64 { return v * v }

// dbToGain converts decibels to linear gain.
func dbToGain(db float64) float64 { return math.Pow(10, db/20) }
