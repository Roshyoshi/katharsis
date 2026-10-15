// demo-audio auditions katharsis's sound: every effect, every track, and
// the music cues of a boot, through the real engine and output device.
//
//	go run ./cmd/demo-audio sfx [name...]      each effect, named as it plays
//	go run ./cmd/demo-audio tour               the cues of a boot, compressed
//	go run ./cmd/demo-audio track <id|file>... a track from the start, then its loop seam
//	go run ./cmd/demo-audio pools              each pool's tracks, 15 s apiece
//	go run ./cmd/demo-audio analyze [file|dir...]  measure, never play (default: the cache)
//	go run ./cmd/demo-audio list               the manifest and what is downloaded
//
// Only analyze and list are silent; the rest play through the default
// output, at the volumes in ~/.config/katharsis/audio.toml.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"katharsis/internal/audio"
)

func main() {
	fs := flag.NewFlagSet("demo-audio", flag.ExitOnError)
	per := fs.Duration("for", 15*time.Second, "how long each track plays (track, pools)")
	seam := fs.Duration("seam", 6*time.Second, "how much before a track's loop point to jump to (track)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: demo-audio [-for d] [-seam d] sfx|tour|track|pools|analyze|list [args]")
		fs.PrintDefaults()
	}
	_ = fs.Parse(os.Args[1:])
	if fs.NArg() == 0 {
		fs.Usage()
		os.Exit(2)
	}
	set, lib, err := load()
	if err != nil {
		fail(err)
	}
	args := fs.Args()[1:]
	switch fs.Arg(0) {
	case "analyze":
		err = analyze(lib, args)
	case "list":
		list(lib)
	case "sfx":
		err = sfx(set, lib, args)
	case "tour":
		tour(set, lib)
	case "track":
		err = track(set, lib, args, *per, *seam)
	case "pools":
		pools(set, lib, *per)
	default:
		fs.Usage()
		os.Exit(2)
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "demo-audio:", err)
	os.Exit(1)
}

func load() (audio.Settings, *audio.Library, error) {
	dir, err := audio.ConfigDir()
	if err != nil {
		return audio.Settings{}, nil, err
	}
	set, err := audio.LoadSettings(filepath.Join(dir, "audio.toml"))
	if err != nil {
		return set, nil, err
	}
	lib, err := audio.DefaultLibrary(set.UserMusic)
	return set, lib, err
}

func list(lib *audio.Library) {
	fmt.Printf("cache %s\nyour music %s\n\n", lib.CacheDir, lib.UserDir)
	for _, t := range lib.Tracks {
		mark := "  "
		if lib.Have(t) {
			mark = "✓ "
		}
		fmt.Printf("%s%-22s %-28s %s\n", mark, t.ID, strings.Join(t.Pools, ","), t.Credit())
	}
}

func analyze(lib *audio.Library, args []string) error {
	type item struct {
		name, path string
		ls, le     float64
	}
	var items []item
	if len(args) == 0 {
		for _, t := range lib.Tracks {
			if lib.Have(t) {
				items = append(items, item{t.ID, lib.Path(t), t.LoopStart, t.LoopEnd})
			}
		}
		if len(items) == 0 {
			return fmt.Errorf("nothing downloaded; run `katharsis audio fetch` or name files")
		}
	}
	for _, a := range args {
		_ = filepath.WalkDir(a, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && audio.IsAudioName(p) {
				items = append(items, item{filepath.Base(p), p, 0, 0})
			}
			return nil
		})
	}
	fmt.Printf("%-34s %5s %7s %6s %6s %6s %5s %5s %6s %7s %6s %6s %5s\n",
		"track", "kHz", "secs", "LUFS", "peak", "rms", "clip", "lead", "tail", "seam×", "seamdB", "dec×", "trim")
	for _, it := range items {
		a, err := audio.Analyze(it.path, it.ls, it.le)
		if err != nil {
			fmt.Printf("%-34s %v\n", trunc(it.name, 34), err)
			continue
		}
		fmt.Printf("%-34s %5.1f %7.1f %6.1f %6.1f %6.1f %5d %5.2f %6.2f %7.1f %6.1f %6.0f %5d\n",
			trunc(it.name, 34), float64(a.Rate)/1000, float64(a.Frames)/float64(a.Rate), a.LUFS, a.PeakDB, a.RMSDB,
			a.Clipped, a.LeadSilence, a.TailSilence, a.SeamJump, a.SeamStepDB, math.Round(a.DecodeSpeed), a.Trimmed)
	}
	fmt.Println("\nseam× is the loop's wrap-around step over the median step (≈1 seamless, >10 a click);",
		"seamdB the last 100 ms against the first (≈0 a loop, very negative a fade-out).",
		"\nAn .mml song's seam is its decoder carrying on from the loop mark, the same samples the",
		"arranger's player makes, so its seam× is the music's own step there (a square wave's edge), not a cut.")
	return nil
}

func trunc(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func engine(set audio.Settings, lib *audio.Library) *audio.Engine {
	set.Music, set.SFX = true, true
	e := audio.New(set, lib)
	time.Sleep(1500 * time.Millisecond) // the device takes up to ~1.2 s to open
	return e
}

func sfx(set audio.Settings, lib *audio.Library, names []string) error {
	fx := audio.DefaultEffects()
	if len(names) == 0 {
		names = audio.Names(fx)
	}
	for _, n := range names {
		if _, ok := fx[n]; !ok {
			return fmt.Errorf("no effect %q; have %v", n, audio.Names(fx))
		}
	}
	e := engine(set, lib)
	defer e.Close()
	for _, n := range names {
		fmt.Printf("%-9s %s\n", n, fx[n].Note)
		e.Play(n)
		time.Sleep(1600 * time.Millisecond)
	}
	return nil
}

// tour plays a boot's cues in a few minutes: each step says what the game
// would be doing.
func tour(set audio.Settings, lib *audio.Library) {
	e := engine(set, lib)
	defer e.Close()
	type step struct {
		say  string
		cue  *audio.Cue
		sfx  []string
		duck int // 1 on, -1 off
		wait time.Duration
	}
	cue := func(p, key string, scene int) *audio.Cue { return &audio.Cue{Pool: p, Key: key, Scene: scene} }
	steps := []step{
		{say: "GRUB menu", cue: cue(audio.PoolGrub, "", 0), wait: 8 * time.Second},
		{say: "boot: Userland", cue: cue(audio.PoolUserland, "mixed", 0), sfx: []string{"door"}, wait: 6 * time.Second},
		{say: ":Submit — the music ducks while it judges", sfx: []string{"submit"}, duck: 1, wait: 3 * time.Second},
		{say: "wrong answer, a life lost", sfx: []string{"wa", "life"}, duck: -1, wait: 3 * time.Second},
		{say: ":Quack", sfx: []string{"quack"}, wait: 2 * time.Second},
		{say: "a minute left", sfx: []string{"warn"}, wait: 2 * time.Second},
		{say: ":Submit again — accepted", sfx: []string{"submit"}, duck: 1, wait: 2 * time.Second},
		{say: "AC, upgrade picked", sfx: []string{"ac", "pick"}, duck: -1, wait: 4 * time.Second},
		{say: "merge window: commit, buy", cue: cue(audio.PoolMerge, "", 0), sfx: []string{"commit"}, wait: 4 * time.Second},
		{say: "buy, then reroll", sfx: []string{"buy"}, wait: 1 * time.Second},
		{say: "", sfx: []string{"reroll"}, wait: 5 * time.Second},
		{say: "biome 2: The Cache (dp)", cue: cue(audio.PoolMid, "dp", 1), sfx: []string{"door"}, wait: 10 * time.Second},
		{say: "boss: Heisenbug", cue: cue(audio.PoolBoss, "heisenbug", 2), sfx: []string{"boss"}, wait: 10 * time.Second},
		{say: "final boss: The Halting Problem", cue: cue(audio.PoolFinal, "halting-problem", 3), wait: 10 * time.Second},
		{say: "cleared, prestige unlocked", cue: cue(audio.PoolCleared, "", 3), sfx: []string{"cleared", "prestige"}, wait: 8 * time.Second},
		{say: "another boot crashes: kernel panic", cue: cue(audio.PoolPanic, "", 4), sfx: []string{"crash"}, wait: 6 * time.Second},
		{say: "back to GRUB", cue: cue(audio.PoolGrub, "", 5), wait: 8 * time.Second},
	}
	var serial uint64
	for _, s := range steps {
		if s.say != "" {
			fmt.Println("▸", s.say)
		}
		if s.cue != nil {
			e.Cue(*s.cue)
		}
		for _, n := range s.sfx {
			e.Play(n)
		}
		switch s.duck {
		case 1:
			e.Duck(true)
		case -1:
			e.Duck(false)
		}
		for end := time.Now().Add(s.wait); time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
			if t, n := e.NowPlaying(); n != serial {
				serial = n
				fmt.Println("  ", t.Credit())
			}
		}
	}
}

func track(set audio.Settings, lib *audio.Library, args []string, per, seam time.Duration) error {
	if len(args) == 0 {
		return fmt.Errorf("name a track id or a file")
	}
	var ts []audio.Track
	for _, a := range args {
		i := slices.IndexFunc(lib.Tracks, func(t audio.Track) bool { return t.ID == a })
		switch {
		case i >= 0 && lib.Have(lib.Tracks[i]):
			t := lib.Tracks[i]
			t.Path = lib.Path(t)
			ts = append(ts, t)
		case i >= 0:
			return fmt.Errorf("%s is not downloaded; run `katharsis audio fetch`", a)
		default:
			if _, err := os.Stat(a); err != nil {
				return err
			}
			ts = append(ts, audio.Track{ID: a, Title: filepath.Base(a), Path: a})
		}
	}
	e := engine(set, lib)
	defer e.Close()
	for _, t := range ts {
		fmt.Println(t.Credit())
		fmt.Println("  from the start")
		if err := e.Audition(t, 0); err != nil {
			return err
		}
		time.Sleep(per)
		fmt.Printf("  %v before the loop point, then across it\n", seam)
		if err := e.Audition(t, -seam); err != nil {
			return err
		}
		time.Sleep(seam + 5*time.Second)
	}
	return nil
}

func pools(set audio.Settings, lib *audio.Library, per time.Duration) {
	e := engine(set, lib)
	defer e.Close()
	for _, p := range audio.Pools {
		ts := lib.Resolve(p, "")
		fmt.Printf("%s: %d tracks\n", p, len(ts))
		for _, t := range ts {
			fmt.Println("  ", t.Credit())
			if err := e.Audition(t, 0); err != nil {
				fmt.Println("   ", err)
				continue
			}
			time.Sleep(per)
		}
	}
}
