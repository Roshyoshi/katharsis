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
const (
	PoolGrub     = "grub"     // the GRUB menu, git log, top
	PoolUserland = "userland" // the first biome
	PoolMid      = "mid"      // topic biomes
	PoolBoss     = "boss"     // named-bug boss rooms
	PoolFinal    = "final"    // The Halting Problem
	PoolMerge    = "merge"    // the merge window between biomes
	PoolPanic    = "panic"    // the kernel panic (silent by default)
	PoolCleared  = "cleared"  // after the last boss
	PoolRTFM     = "rtfm"     // learn mode
)

// Pools lists every pool, in the order the credits show them.
var Pools = []string{PoolGrub, PoolUserland, PoolMid, PoolBoss, PoolFinal, PoolMerge, PoolPanic, PoolCleared, PoolRTFM}

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

	Path string `toml:"-"` // the local file, once there is one
	// Gain is the linear gain that brings the track to the target
	// loudness; 0 means not measured yet (played at 1).
	Gain float64 `toml:"-"`
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
		b.WriteString(t.License)
	}
	return b.String()
}
