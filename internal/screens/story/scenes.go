package story

import (
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/anim"
	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/ui"
)

// Timings. Every one is short enough that nobody waits for it, and any key
// cuts it short.
const (
	fallTime    = 1400 * time.Millisecond // the prologue's drop
	biomeFall   = 1000 * time.Millisecond // a drop between biomes
	cardTime    = 1200 * time.Millisecond // a boss card opening
	catIn       = 900 * time.Millisecond  // the Cat, grin first
	catOut      = 1500 * time.Millisecond // the Cat, tail first
	fadeTime    = 300 * time.Millisecond
	acWipe      = 360 * time.Millisecond // ACCEPTED: the banner, then the line
	acFade      = 240 * time.Millisecond
	biomeDwell  = 1500 * time.Millisecond // how long a biome card stays
	acDwell     = 900 * time.Millisecond  // how long ACCEPTED stays
	carveCPS    = 14                      // the old gods carve slowly
	cardWidth   = 64
	revealTime  = 600 * time.Millisecond  // the crash screen's reveal
	splashDwell = 1200 * time.Millisecond // how long a boot splash stays
)

// lines is s split into rows, nil for "".
func lines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// art is a flavor banner as rows.
func art(kind string) []string { return lines(flavor.Banner(kind)) }

// blank is one empty row: a gap between parts of a block.
var blank = anim.Hold{Lines: []string{""}}

// typed is narration typed out in the flavor voice, rewrapped to fit w
// cells when a line would not (story lines are 70 at most, so 80 columns
// never rewrap).
func typed(ls []string, w int) anim.Typewriter {
	return anim.Typewriter{Lines: fit(ls, w), Style: voice()}
}

// fit is ls with each line wider than w-2 word-wrapped, keeping a gutter.
func fit(ls []string, w int) []string {
	w = max(w-2, 20)
	var out []string
	for _, l := range ls {
		if ansi.StringWidth(l) <= w {
			out = append(out, l)
			continue
		}
		out = append(out, strings.Split(ansi.Wrap(l, w, ""), "\n")...)
	}
	return out
}

// fadeIn fades plain rows in, drawn in style.
func fadeIn(ls []string, style lipgloss.Style) anim.Fade {
	return anim.Fade{Lines: ls, From: anim.Gone, To: anim.Normal, D: fadeTime, Style: style}
}

// last is c's last frame.
func last(c anim.Clip) []string { return c.Frame(c.Len()) }

// NewPrologue is the first launch (story.prologue): the banner, then the
// prologue typed out. At "Down, down, down." you drop down the
// rabbit-hole, and land in the hall of doors for the rest.
func NewPrologue() *Scene {
	all := flavor.StoryBlock("story.prologue")
	k := len(all)
	for i, l := range all {
		if strings.HasPrefix(l, "Down, down, down") {
			k = i
			break
		}
	}
	above, below := all[:k], all[k:]
	first := func(w, _ int) anim.Clip {
		said := fit(above, w)
		return anim.Then(over(fadeIn(art("boot"), plain()), widest(said)), blank, typed(said, w))
	}
	pages := []page{first}
	if len(below) > 0 && len(above) > 0 {
		pages = append(pages, func(w, h int) anim.Clip {
			rest := typed(below, w)
			return anim.Seq(anim.Fall{
				From: place(last(first(w, h)), w, h), To: place(rest.Frame(0), w, h),
				W: w, H: h, D: fallTime, Label: "ring 3",
			}, rest)
		})
	}
	return newScene("story.prologue", 0, pages...)
}

// NewBossIntro is a boss's card before its room: the double frame opens,
// the name is stamped in, then its blurb and an entrance line from
// boss.enter.<key>, picked by seed. first marks the player's first boss
// (story.first-boss: the King's rule first) or first final boss
// (reveal.final-boss, which takes the place of the entrance line).
func NewBossIntro(key string, seed uint64, first bool) *Scene {
	label := ui.G.Boss + " BOSS"
	if key == game.FinalBoss {
		label = ui.G.Boss + " FINAL BOSS"
	}
	enter := flavor.Story("boss.enter."+key, seed)
	var pages []page
	id := ""
	if first {
		id = "story.first-boss"
		if key == game.FinalBoss {
			id, enter = "reveal.final-boss", ""
		}
		said := flavor.StoryBlock(id)
		pages = append(pages, func(w, _ int) anim.Clip { return typed(said, w) })
	}
	paras := []anim.Para{{Text: flavor.BossBlurb(key), Style: muted()}}
	if enter != "" {
		paras = append(paras, anim.Para{Text: enter, Style: voice()})
	}
	pages = append(pages, func(w, _ int) anim.Clip {
		return anim.Card{Label: label, Name: flavor.BossName(key), NameStyle: strong(),
			Paras: paras, W: min(w, cardWidth), D: cardTime}
	})
	return newScene(id, 0, pages...)
}

// NewFirstCrash is the Cheshire Cat's beat on the first crash
// (story.first-crash): the Cat appears grin first and says its piece,
// then vanishes tail first and leaves the grin, which is the core dump:
// bytes still on the problem titled title. The pacing shows it only after
// a crash that left bytes (flavor.NextBeat); bytes "" leaves the dump
// line out. The title is cut to 24 cells, as GRUB cuts the same line.
func NewFirstCrash(bytes, title string) *Scene {
	said := flavor.StoryBlock("story.first-crash")
	var dump []string
	if bytes != "" {
		dump = []string{flavor.Fill(flavor.Story("grub.subtitle.coredump", 0),
			"bytes", bytes, "title", ui.Clip(title, 24))}
	}
	return newScene("story.first-crash", 0, func(w, _ int) anim.Clip {
		speech := typed(said, w)
		cat := anim.Cheshire{Appear: true, D: catIn, Style: plain()}
		track := anim.Seq(cat, anim.Hold{Lines: last(cat), D: speech.Len()},
			anim.Cheshire{D: catOut, KeepGrin: true, Style: plain()})
		after := []anim.Clip{speech}
		if dump != nil {
			after = append(after, blank, anim.Delay(catOut, fadeIn(fit(dump, w), muted())))
		}
		return anim.Stack(over(track, widest(speech.Lines)), blank, anim.Delay(catIn, anim.Then(after...)))
	})
}

// widest is the widest row of ls, in cells.
func widest(ls []string) int {
	n := 0
	for _, l := range ls {
		n = max(n, ansi.StringWidth(l))
	}
	return n
}

// over centers c's rows over a block wide cells across.
func over(c anim.Clip, wide int) anim.Clip {
	return indent((wide-widest(last(c)))/2, c)
}

// NewBiome is the drop into a biome: from (the screen being left, nil for
// a blank one) slides up, the rabbit-hole goes by with the biome's name on
// its wall, and you land on its title and biome.intro.<topic>. It shows
// every biome start and leaves on its own after a moment.
func NewBiome(topic string, from []string) *Scene {
	name := flavor.BiomeName(topic)
	title := strings.ToUpper(name)
	if sp := strings.Join(strings.Split(title, ""), " "); ansi.StringWidth(sp) <= 70 {
		title = sp
	}
	intro := lines(flavor.Story("biome.intro."+topic, 0))
	return newScene("", biomeDwell, func(w, h int) anim.Clip {
		said := typed(intro, w)
		parts := []anim.Clip{over(fadeIn(fit([]string{title}, w), strong()), widest(said.Lines))}
		if len(intro) > 0 {
			parts = append(parts, blank, said)
		}
		land := anim.Then(parts...)
		return anim.Seq(anim.Fall{From: place(from, w, h), To: place(land.Frame(0), w, h),
			W: w, H: h, D: biomeFall, Label: name, Salt: uint64(len(topic))}, land)
	})
}

// door is the first door of the hall: old, shut, carved.
var door = []string{
	`.------------------.`,
	`|  .------------.  |`,
	`|  |            |  |`,
	`|  |            |  |`,
	`|  |          o |  |`,
	`|  |            |  |`,
	`|  |            |  |`,
	`'--'------------'--'`,
}

// inscription is what the old gods carved: K&R's first output, lowercase,
// no exclamation mark (story bible, tone rule 8).
const inscription = "hello, world"

// NewFirstDoor is the first door's inscription (door.first.inscription):
// the door fades in, hello, world is carved over it slowly, and the line
// says who carved it.
func NewFirstDoor() *Scene {
	line := []string{flavor.Story("door.first.inscription", 0)}
	return newScene("door.first.inscription", 0, func(w, _ int) anim.Clip {
		said := fit(line, w)
		wide := max(widest(said), widest(door))
		shut := over(fadeIn(door, plain()), wide)
		carve := over(anim.Typewriter{Lines: []string{inscription}, CPS: carveCPS, Style: strong()}, wide)
		return anim.Stack(
			anim.Delay(shut.Len(), carve), blank, shut, blank,
			anim.Delay(shut.Len()+carve.Len(), over(fadeIn(said, voice()), wide)),
		)
	})
}

// teaParty is the merge window's table: always tea-time.
var teaParty = []string{
	`          .------------------.`,
	`          |   MERGE WINDOW   |`,
	`          '--------+---------'`,
	`                   |`,
	`.------------------------------------.`,
	`|  c[_]    c[_]    c[_]    c[_]      |`,
	`'------------------------------------'`,
	`   ||                            ||`,
}

// NewFirstMerge is the first merge window (story.first-merge): the long
// table under the tree, then the Hatter and the March Hare.
func NewFirstMerge() *Scene {
	said := flavor.StoryBlock("story.first-merge")
	return newScene("story.first-merge", 0, func(w, _ int) anim.Clip {
		speech := typed(said, w)
		return anim.Then(over(fadeIn(teaParty, plain()), widest(speech.Lines)), blank, speech)
	})
}

// NewAccepted is the beat an AC gets before the upgrade pick, about 600 ms
// (ui-system.md §1.7, E17: punishment had a beat, reward had none): the
// ACCEPTED banner wipes in, then line, the verdict in figures, fades in
// under it. It leaves on its own.
func NewAccepted(line string) *Scene {
	banner := acBanner(art("ac"))
	wide := max(widest(banner), ansi.StringWidth(line))
	parts := []anim.Clip{over(anim.Wipe{Lines: banner, D: acWipe}, wide)}
	if line != "" {
		parts = append(parts, blank, over(anim.Fade{Lines: []string{line}, From: anim.Gone, To: anim.Normal,
			D: acFade, Style: muted()}, wide))
	}
	clip := anim.Then(parts...)
	return newScene("", acDwell, func(int, int) anim.Clip { return clip })
}

// NewBeat is a page for any one-time beat id (flavor.BeatLines): its
// title, then its text typed out. A prestige reveal, a milestone or a
// first visit reads this way when the player opens it from the end of a
// boot, and the reread log can reuse it.
func NewBeat(id string) *Scene {
	title, said := []string{flavor.BeatTitle(id)}, flavor.BeatLines(id)
	return newScene(id, 0, func(w, _ int) anim.Clip {
		return anim.Then(fadeIn(fit(title, w), strong()), blank, typed(said, w))
	})
}

// NewBootSplash is the start of boot number boot at prestige p
// (boot.splash.dmesg, boot.motd): the banner, then the dmesg block printed
// a line at a time the way the crash screen prints, then one line of the
// motd. It shows every boot and leaves on its own; the app may replace it
// sooner, once door 1 is fetched.
func NewBootSplash(boot, p int) *Scene {
	ring, where := flavor.Ring(p)
	var dmesg []string
	for _, l := range flavor.StoryBlock("boot.splash.dmesg") {
		if where == "" && strings.Contains(l, "{place}") {
			continue // a ring the story has no name for
		}
		dmesg = append(dmesg, flavor.Fill(l, "boot", strconv.Itoa(boot), "ring", strconv.Itoa(ring), "place", where))
	}
	motd := lines(flavor.Story("boot.motd", uint64(max(boot, 0))))
	return newScene("", splashDwell, func(w, _ int) anim.Clip {
		log := fit(dmesg, w)
		return anim.Then(over(fadeIn(art("boot"), plain()), widest(log)), blank,
			anim.Reveal{Lines: log, Total: revealTime, Style: muted()}, blank, typed(motd, w))
	})
}
