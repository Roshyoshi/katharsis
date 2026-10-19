// Package audio is the game's sound: procedural 8-bit effects synthesized
// at start, and background music played from pools of downloaded tracks.
//
// The app never names a file or a sample. It tells a Sink which effect
// fired (Play) and which music the game state calls for (Cue); the Engine
// picks a track from the cue's pool, crossfades to it, loops it, and moves
// through the pool without repeats. Tracks are never bundled: `katharsis
// audio fetch` downloads them from the sources in tracks.toml into the
// cache, and ~/.config/katharsis/music/<pool or key>/ adds the player's
// own. With neither, or with no audio device, music is silence and the
// game plays on.
package audio

import (
	"fmt"
	"strings"
)

// Rate is the mixer's sample rate. Most downloadable music is 44.1 kHz, so
// the mixer runs at it and only other rates are resampled.
const Rate = 44100

// Music pools, by where the game is. A biome's cue names one of these and
// may add a finer Key (its topic or boss) for the player's own folders.
// Topic biomes and boss rooms climb by act as a Touhou run climbs by
// stage, so act III never draws a stage 3 theme.
const (
	PoolGrub     = "grub"     // home, outside a boot: GRUB, RTFM, upsolves
	PoolUserland = "userland" // the first biome: stage 1-2 themes
	PoolAct1     = "act1"     // act I's topic biomes: stage 3-4 themes
	PoolAct2     = "act2"     // act II's: stage 5-6 themes
	PoolAct3     = "act3"     // act III's: extra stage themes
	PoolBoss1    = "boss1"    // Boss I's rooms: stage 1-3 boss themes
	PoolBoss2    = "boss2"    // Boss II's: stage 4-5 and extra bosses
	PoolFinal    = "final"    // The Halting Problem
	PoolMerge    = "merge"    // the map between stages: merge window, route
	PoolPanic    = "panic"    // the kernel panic (silent by default)
	PoolCleared  = "cleared"  // after the last boss
	PoolTrial    = "trial"    // an RTFM trial
)

// Pools lists every pool, in the order the credits show them.
var Pools = []string{PoolGrub, PoolUserland, PoolAct1, PoolAct2, PoolAct3, PoolBoss1, PoolBoss2,
	PoolFinal, PoolMerge, PoolPanic, PoolCleared, PoolTrial}

// ActPools and BossPools are the topic and boss pools by act (the game's
// State.ActOf); an act past the last plays the last.
var (
	ActPools  = []string{PoolAct1, PoolAct2, PoolAct3}
	BossPools = []string{PoolBoss1, PoolBoss2}
)

// Cue is the music a game state calls for.
type Cue struct {
	Pool string // one of the Pool constants; "" is silence
	// Key is a finer folder for the player's own music: the biome's topic
	// ("dp") or boss ("heisenbug"). Tracks in music/<Key>/ take the place
	// of the pool's when the folder has any.
	Key string
	// Scene changes when a new stretch of the game starts in the same
	// pool, such as the next biome: the same pool then crossfades to a
	// new track instead of carrying on.
	Scene int
}

// Sink is what the game asks of the sound system. Every method returns at
// once and is safe from any goroutine.
type Sink interface {
	// Play starts a one-shot effect by name (see sfx.toml); an unknown
	// name, or effects turned off, is silence.
	Play(sfx string)
	// Cue sets the music the game state calls for. The same cue again
	// changes nothing; another one crossfades.
	Cue(c Cue)
	// Duck lowers the music while on, such as while a submission judges.
	Duck(on bool)
	// ToggleMusic turns music on or off and reports whether it is now on.
	ToggleMusic() bool
	// NowPlaying is the track playing and a serial that changes whenever
	// a new track starts; serial 0 means none has.
	NowPlaying() (Track, uint64)
}

// Null is the silent Sink, for tests, --quiet and systems without audio.
type Null struct{}

func (Null) Play(string)                 {}
func (Null) Cue(Cue)                     {}
func (Null) Duck(bool)                   {}
func (Null) ToggleMusic() bool           { return false }
func (Null) NowPlaying() (Track, uint64) { return Track{}, 0 }

// Track is one piece of music: its credit and where it comes from. The
// manifest (tracks.toml) is the credits data; a file from the player's
// own folder is a Track with only Title and Path.
type Track struct {
	ID       string `toml:"id"`
	Title    string `toml:"title"`
	Artist   string `toml:"artist"`   // who made this recording (the arranger)
	Original string `toml:"original"` // what it arranges: "ZUN, Touhou 6 (EoSD), stage 3"; "" for originals
	// Touhou marks an arrangement of ZUN's music, which ZUN's guideline
	// requires to be labelled as a Touhou Project fan work.
	Touhou     bool   `toml:"touhou"`
	License    string `toml:"license"` // short name, e.g. "CC BY 4.0"
	LicenseURL string `toml:"license_url"`
	// LicenseQuote is where the source states the license, verbatim.
	LicenseQuote string   `toml:"license_quote"`
	Page         string   `toml:"page"` // the source page
	URL          string   `toml:"url"`  // the file, fetched as is
	SHA256       string   `toml:"sha256"`
	Size         int64    `toml:"size"`
	Format       string   `toml:"format"` // "mp3", "ogg", "wav", or "mml" (VGS MML, rendered as it plays)
	Seconds      float64  `toml:"seconds"`
	Pools        []string `toml:"pools"`
	// LoopStart and LoopEnd, in seconds, bound the part that repeats; 0
	// and 0 repeat the whole file, or a VGS song from its own loop mark.
	// A track with an intro starts at 0 and loops back to LoopStart.
	LoopStart float64 `toml:"loop_start"`
	LoopEnd   float64 `toml:"loop_end"`
	Note      string  `toml:"note"` // loop suitability, for the curious

	TitleJA string `toml:"title_ja"` // the original's Japanese title, for the credits
	Role    string `toml:"role"`     // where the original plays: "Stage 1 theme"
	// Source names a [source.<name>] table that fills in what the entry
	// leaves out (artist, license, page, format, touhou); File is appended
	// to its url_prefix to give URL.
	Source string `toml:"source"`
	File   string `toml:"file"`
	// Once marks a song with an ending: it plays once a turn instead of
	// Settings.Repeats times, so its ending is not followed by its start.
	Once bool `toml:"once"`
	// Keys are cue keys (a boss: "heisenbug") this track leads for: a cue
	// with that key starts with one of its leads, then plays its pool.
	Keys []string `toml:"keys"`

	From *Source `toml:"-"` // the Source table, once loaded
	Path string  `toml:"-"` // the local file, once there is one
	// Gain is the linear gain that brings the track to the target
	// loudness; 0 means not measured yet (played at 1).
	Gain float64 `toml:"-"`
}

// Source is one arranger's or site's terms, shared by its tracks so the
// credits state them once.
type Source struct {
	Name         string `toml:"name"` // who and what, for the credits heading
	Artist       string `toml:"artist"`
	Touhou       bool   `toml:"touhou"`
	License      string `toml:"license"`
	LicenseURL   string `toml:"license_url"`
	LicenseQuote string `toml:"license_quote"`
	Page         string `toml:"page"`
	URLPrefix    string `toml:"url_prefix"`
	Format       string `toml:"format"`
	// Notice is anything else the source asks the game to show, such as
	// a decoder's copyright line.
	Notice string `toml:"notice"`
}

// licenseLabel is a license's short name as a credit shows it.
func licenseLabel(l string) string {
	if l == "permission" {
		return "used by permission"
	}
	return l
}

// Credit is the one-line toast shown when the track starts:
// "♪ Title, arr. by X (orig. ZUN, Touhou 6), CC BY 4.0".
func (t Track) Credit() string {
	var b strings.Builder
	b.WriteString("♪ ")
	b.WriteString(t.Title)
	if t.Artist != "" {
		if t.Original != "" {
			b.WriteString(", arr. by ")
		} else {
			b.WriteString(", by ")
		}
		b.WriteString(t.Artist)
	}
	if t.Original != "" {
		fmt.Fprintf(&b, " (orig. %s)", t.Original)
	}
	if t.License != "" {
		b.WriteString(", ")
		b.WriteString(licenseLabel(t.License))
	}
	return b.String()
}
