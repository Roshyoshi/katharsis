package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"katharsis/internal/audio"
)

// audioFlags are the game's sound switches, over audio.toml.
type audioFlags struct {
	quiet, noMusic, noSFX *bool
	musicVol, sfxVol      *float64
}

func addAudioFlags(fs *flag.FlagSet) audioFlags {
	return audioFlags{
		quiet:    fs.Bool("quiet", false, "no sound at all (music and effects)"),
		noMusic:  fs.Bool("no-music", false, "no music (effects still play)"),
		noSFX:    fs.Bool("no-sfx", false, "no sound effects (music still plays)"),
		musicVol: fs.Float64("music-volume", -1, "music volume 0..1 (default: audio.toml, else 0.7)"),
		sfxVol:   fs.Float64("sfx-volume", -1, "effects volume 0..1 (default: audio.toml, else 0.8)"),
	}
}

// audioSettings reads audio.toml, or says why not and gives the defaults.
func audioSettings() (audio.Settings, error) {
	dir, err := audio.ConfigDir()
	if err != nil {
		return audio.DefaultSettings(), err
	}
	s, err := audio.LoadSettings(filepath.Join(dir, "audio.toml"))
	if err != nil {
		return audio.DefaultSettings(), err
	}
	return s, nil
}

// openAudio reads audio.toml and the flags and starts the engine. Sound
// is never a reason not to play: a bad file is reported and the defaults
// used; no device or no music downloaded is silence.
func openAudio(f audioFlags) (audio.Sink, audio.Settings, func() string, []error) {
	var errs []error
	set, err := audioSettings()
	if err != nil {
		errs = append(errs, err)
	}
	if *f.musicVol >= 0 {
		set.MusicVolume = min(*f.musicVol, 1)
	}
	if *f.sfxVol >= 0 {
		set.SFXVolume = min(*f.sfxVol, 1)
	}
	set.Music = set.Music && !*f.noMusic && !*f.quiet
	set.SFX = set.SFX && !*f.noSFX && !*f.quiet
	lib, err := audio.DefaultLibrary(set.UserMusic)
	if err != nil {
		errs = append(errs, err)
		return audio.Null{}, set, func() string { return audio.Credits(nil, set) }, errs
	}
	credits := func() string { return audio.Credits(lib, set) }
	// --quiet, or both buses off: no device is opened at all, so no CPU
	// goes to rendering silence. --no-music alone keeps the device for
	// effects, and ctrl+t can still turn the music on.
	if *f.quiet || (!set.SFX && *f.noMusic) {
		return audio.Null{}, set, credits, errs
	}
	return audio.New(set, lib), set, credits, errs
}

// audioDefaults is what the game does when audio.toml does not load.
const audioDefaults = "Playing with the default audio settings."

// audioCloseWait bounds the music's fade at exit (audio.Engine.Close): an
// output that hangs must not keep the game from exiting.
const audioCloseWait = time.Second

// closeAudio fades the music out and stops the engine, waiting at most
// wait. Without it the music was cut off mid-note at exit. The null sink
// has nothing to close.
func closeAudio(s audio.Sink, wait time.Duration) {
	c, ok := s.(interface{ Close() })
	if !ok {
		return
	}
	done := make(chan struct{})
	go func() { c.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(wait):
	}
}

// runAudio is `katharsis audio`: fetch the music, list it, show credits.
func runAudio(args []string) error {
	fs := flag.NewFlagSet("audio", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: katharsis audio fetch [pool|track...]   download the music (all of it by default)
       katharsis audio list                     the tracks, their pools, what is downloaded
       katharsis audio credits                  the credits page, as markdown

Music downloads from each track's own source into the user cache, checked
against its SHA-256; the game never bundles it. Your own files go in
~/.config/katharsis/music/<pool or topic>/. Settings: ~/.config/katharsis/audio.toml.
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return flag.ErrHelp
	}
	set, err := audioSettings()
	if err != nil {
		warnKey("audio: ", []error{err}, audioDefaults)
	}
	lib, err := audio.DefaultLibrary(set.UserMusic)
	if err != nil {
		return err
	}
	switch fs.Arg(0) {
	case "fetch":
		return fetchAudio(lib, fs.Args()[1:])
	case "list":
		have := 0
		for _, t := range lib.Tracks {
			mark := " "
			if lib.Have(t) {
				mark, have = "✓", have+1
			}
			fmt.Printf("%s %-22s %-20s %s\n", mark, t.ID, strings.Join(t.Pools, ","), t.Credit())
		}
		fmt.Printf("\n%d of %d downloaded, in %s\n", have, len(lib.Tracks), lib.CacheDir)
		return nil
	case "credits":
		fmt.Print(audio.Credits(lib, set))
		return nil
	}
	fs.Usage()
	return fmt.Errorf("unknown audio command %q", fs.Arg(0))
}

func fetchAudio(lib *audio.Library, only []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	var size int64
	n := 0
	for _, t := range lib.Tracks {
		if !lib.Have(t) {
			size += t.Size
			n++
		}
	}
	fmt.Fprintf(os.Stderr, "katharsis: %d of %d tracks to download (%.1f MiB) into %s\n",
		n, len(lib.Tracks), float64(size)/(1<<20), lib.CacheDir)
	fmt.Fprintf(os.Stderr, "\n%s\n\n", audio.TouhouNotice)
	r := audio.Fetch(ctx, lib, audio.FetchOptions{Out: os.Stderr, Only: only, Retries: 2, Gap: 500 * time.Millisecond})
	fmt.Fprintf(os.Stderr, "\n%d fetched (%.1f MiB), %d already here, %d failed. `katharsis audio credits` lists them all.\n",
		r.Fetched, float64(r.Bytes)/(1<<20), r.Had, r.Failed)
	if r.Failed > 0 {
		return fmt.Errorf("%d tracks failed; the game plays without them", r.Failed)
	}
	return nil
}
