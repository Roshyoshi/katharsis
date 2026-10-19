package audio

import (
	"fmt"
	"slices"
	"strings"

	"katharsis/internal/audio/vgs"
)

// TouhouNotice is the label ZUN's fan-work guideline asks every Touhou
// fan work to carry ("東方Projectの二次創作である事を明記してください",
// https://touhou-project.news/guideline/), with what it covers here.
const TouhouNotice = "東方Project二次創作 — the Touhou music in katharsis is fan arrangements, " +
	"a fan work based on Touhou Project. Touhou Project and its original music are by ZUN " +
	"(上海アリス幻樂団, Team Shanghai Alice). katharsis is unofficial and not affiliated with " +
	"Team Shanghai Alice. Each arrangement belongs to its arranger and is used under the license shown."

// poolNames say where each pool plays, for the credits.
var poolNames = map[string]string{
	PoolGrub:     "Home (GRUB, RTFM, upsolves)",
	PoolUserland: "Userland (the first biome)",
	PoolAct1:     "Act I",
	PoolAct2:     "Act II",
	PoolAct3:     "Act III",
	PoolBoss1:    "Boss I",
	PoolBoss2:    "Boss II",
	PoolFinal:    "The Halting Problem",
	PoolMerge:    "The map between stages",
	PoolPanic:    "Kernel panic",
	PoolCleared:  "Boot cleared",
	PoolTrial:    "RTFM trials",
}

// Credits is the credits page: each source's license once, every track
// by where it plays, the Touhou notice, the player's folders, where the
// effects come from, and the notices the software asks for.
func Credits(l *Library, set Settings) string {
	var b strings.Builder
	b.WriteString("# Credits\n\n")
	var tracks []Track
	if l != nil {
		tracks = l.Tracks
	}
	have, touhou := 0, false
	var sources []*Source
	for _, t := range tracks {
		if l.Have(t) {
			have++
		}
		touhou = touhou || t.Touhou
		if t.From != nil && !slices.Contains(sources, t.From) {
			sources = append(sources, t.From)
		}
	}
	if touhou {
		fmt.Fprintf(&b, "> %s\n\n", TouhouNotice)
	}
	b.WriteString("## Music\n\n")
	switch {
	case len(tracks) == 0:
		b.WriteString("No tracks are listed yet.\n\n")
	case have < len(tracks):
		fmt.Fprintf(&b, "%d of %d tracks are downloaded. `katharsis audio fetch` downloads the rest "+
			"from the sources below; without them the game is silent but for its effects.\n\n", have, len(tracks))
	default:
		fmt.Fprintf(&b, "All %d tracks are downloaded.\n\n", len(tracks))
	}
	for _, s := range sources {
		b.WriteString(sourceEntry(s))
	}
	for _, p := range Pools {
		var in []Track
		for _, t := range tracks {
			if slices.Contains(t.Pools, p) {
				in = append(in, t)
			}
		}
		if p == PoolPanic && set.Panic == "" {
			fmt.Fprintf(&b, "### %s\n\nSilence: a halted kernel makes no sound.\n\n", poolNames[p])
			continue
		}
		if len(in) == 0 {
			continue
		}
		fmt.Fprintf(&b, "### %s\n\n", poolNames[p])
		for _, t := range in {
			b.WriteString(creditEntry(t))
		}
		b.WriteString("\n")
	}
	b.WriteString("## Your own music\n\n")
	if l != nil && l.UserDir != "" {
		fmt.Fprintf(&b, "Audio files (mp3, ogg, wav, or VGS mml) in `%s/<pool>/` play in that pool, and "+
			"`%s/<topic or boss>/` in that biome (`dp`, `graphs`, `heisenbug`, ...). With user_music = \"%s\" "+
			"they %s. Pools: %s.\n\n", l.UserDir, l.UserDir, set.UserMusic,
			map[string]string{"override": "replace the pool's tracks", "extend": "join the pool's tracks"}[set.UserMusic],
			strings.Join(Pools, ", "))
	}
	b.WriteString("## Sound effects\n\n")
	b.WriteString("Synthesized by katharsis at start from the parameters in `internal/audio/sfx.toml`: " +
		"pulse, triangle, saw and noise voices, no recordings, so they carry no license of their own.\n\n")
	b.WriteString("## Problems and text\n\n")
	b.WriteString("USACO problems, tests and analyses are from usaco.org. The problem index and learn mode " +
		"use the USACO Guide (CC BY-NC-SA 4.0). Codeforces problems, statements and editorials come from " +
		"the open-r1/codeforces dataset (CC BY 4.0); statements and editorials are by their Codeforces authors.\n\n")
	b.WriteString(StoryCredit + "\n\n")
	b.WriteString("## Software\n\n")
	b.WriteString("katharsis plays VGS songs through a Go port of SUZUKI PLAN's VGS MML compiler and BGM " +
		"decoder (https://github.com/suzukiplan/tohovgs-cli), under their license:\n\n")
	for _, line := range strings.Split(strings.TrimRight(vgs.License, "\n"), "\n") {
		fmt.Fprintf(&b, "    %s\n", line)
	}
	b.WriteString("\n## Controls\n\n")
	b.WriteString("`ctrl+t` turns the music on or off on any screen. Volumes, fades and the rest are in " +
		"`~/.config/katharsis/audio.toml`; `--quiet` silences everything, `--no-music` and `--no-sfx` one each.\n")
	return b.String()
}

// sourceEntry states one source's license, once for all its tracks.
func sourceEntry(s *Source) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### %s\n\n", s.Name)
	if s.License == "permission" {
		fmt.Fprintf(&b, "Used by the arranger's written permission (%s):\n\n", s.LicenseURL)
	} else {
		fmt.Fprintf(&b, "[%s](%s):\n\n", s.License, s.LicenseURL)
	}
	for _, line := range strings.Split(s.LicenseQuote, "\n") {
		fmt.Fprintf(&b, "> %s\n", line)
	}
	fmt.Fprintf(&b, "\nSource: %s\n\n", s.Page)
	if s.Notice != "" {
		fmt.Fprintf(&b, "%s\n\n", s.Notice)
	}
	return b.String()
}

func creditEntry(t Track) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- **%s**", t.Title)
	if t.TitleJA != "" && t.TitleJA != t.Title {
		fmt.Fprintf(&b, " (%s)", t.TitleJA)
	}
	if t.Original != "" {
		orig := t.Original
		if t.Role != "" {
			orig += ", " + t.Role
		}
		fmt.Fprintf(&b, ", arranged by %s (original: %s)", t.Artist, orig)
	} else {
		fmt.Fprintf(&b, ", by %s", t.Artist)
	}
	if t.From != nil {
		// The source's entry above states the license.
		b.WriteString("\n")
		return b.String()
	}
	if t.LicenseURL != "" {
		fmt.Fprintf(&b, ". [%s](%s)", t.License, t.LicenseURL)
	} else {
		fmt.Fprintf(&b, ". %s", t.License)
	}
	fmt.Fprintf(&b, ". Source: %s\n", t.Page)
	if t.LicenseQuote != "" {
		fmt.Fprintf(&b, "  > %s\n", strings.ReplaceAll(t.LicenseQuote, "\n", " "))
	}
	return b.String()
}

// StoryCredit is the story's sources, as the README and the RTFM
// COLOPHON credit them: Carroll is in the public domain, SICP under CC
// BY-SA 4.0, quoted verbatim and credited.
const StoryCredit = "The story quotes Lewis Carroll's *Alice's Adventures in Wonderland* (1865) and " +
	"*Through the Looking-Glass* (1871), which are in the public domain, and short passages of " +
	"*Structure and Interpretation of Computer Programs*, 2nd ed., by Harold Abelson and Gerald Jay " +
	"Sussman with Julie Sussman (MIT Press, 1996), licensed under " +
	"[CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/). Quotes are verbatim and credited."
