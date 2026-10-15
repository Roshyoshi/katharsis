// demo-story plays every story scene with fixture text, so a human can
// watch them without playing to each moment: n and p switch scenes, r
// replays one, every other key goes to the scene, and what the scene emits
// is shown in the bottom line instead of being acted on. ctrl+c quits.
//
//	go run ./cmd/demo-story                  # every scene, from the title
//	go run ./cmd/demo-story title            # the title screen
//	go run ./cmd/demo-story boss             # every scene whose name has "boss"
//	go run ./cmd/demo-story -list            # the scene names
//	go run ./cmd/demo-story -reduced crash   # reduced motion: last frames at once
//	LC_ALL=C go run ./cmd/demo-story title   # ASCII glyphs; NO_COLOR=1: Mono
//	go run ./cmd/demo-story -print -at 0,300ms,end accepted
//	                                         # frames to stdout, no terminal
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/anim"
	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/screens/story"
	"katharsis/internal/ui"
)

// scene is one entry of the demo, built fresh each time it is shown.
type scene struct {
	name string
	make func() *story.Scene
}

func scenes() []scene {
	out := []scene{
		{"title", story.NewTitle},
		{"prologue", story.NewPrologue},
		{"boot splash", func() *story.Scene { return story.NewBootSplash(42, 0) }},
		{"boot splash, ring -3", func() *story.Scene { return story.NewBootSplash(318, game.MaxPrestige) }},
		{"first-door", story.NewFirstDoor},
		{"first-merge", story.NewFirstMerge},
		{"accepted", func() *story.Scene { return story.NewAccepted("AC · 12/12 tests · 08:41 / 15:00") }},
		{"crash", func() *story.Scene { return story.NewFirstCrash("15.2 KiB", "Bovine Shuffle") }},
		{"boss, first ever", func() *story.Scene { return story.NewBossIntro("off-by-one", 1, true) }},
	}
	for i, k := range game.BossKeys {
		out = append(out, scene{"boss " + k, func() *story.Scene { return story.NewBossIntro(k, uint64(i), false) }})
	}
	out = append(out,
		scene{"boss halting-problem, first", func() *story.Scene { return story.NewBossIntro(game.FinalBoss, 0, true) }},
		scene{"boss halting-problem", func() *story.Scene { return story.NewBossIntro(game.FinalBoss, 2, false) }},
	)
	from := []string{"Brute Force Bunker · room 3", "", "[  OK  ] door cleared"}
	for _, t := range catalog.Topics {
		out = append(out, scene{"biome " + t, func() *story.Scene { return story.NewBiome(t, from) }})
	}
	for _, id := range []string{
		"daemon.intro", "story.first-rtfm", "story.first-regression",
		"reveal.prestige.1", "reveal.prestige.2", "reveal.prestige.3", "reveal.prestige.4",
		"reveal.final-clear", "milestone.doors.10", "milestone.gate.half", "milestone.capability:CAP_DP",
		"milestone.mastered.1",
	} {
		out = append(out, scene{"beat " + id, func() *story.Scene { return story.NewBeat(id) }})
	}
	return out
}

func main() {
	reduced := flag.Bool("reduced", false, "reduced motion: every clip shows its last frame at once")
	fps := flag.Int("fps", 0, "frame-rate cap (0 is anim.DefaultFPS)")
	list := flag.Bool("list", false, "print the scene names and exit")
	printMode := flag.Bool("print", false, "print frames to stdout instead of playing")
	at := flag.String("at", "end", "with -print: comma-separated times into each page, or end")
	w := flag.Int("w", 80, "with -print: width")
	h := flag.Int("h", 24, "with -print: height")
	flag.Parse()
	anim.SetMotion(anim.Motion{Reduced: *reduced, FPS: *fps})
	// The glyph set and palette as the game picks them: ASCII under a
	// non-UTF-8 locale (LC_ALL=C), Mono under NO_COLOR.
	ui.SetGlyphs(ui.AutoGlyphs(os.Getenv))
	if os.Getenv("NO_COLOR") != "" {
		ui.SetPalette(ui.Mono)
	}

	all := scenes()
	if *list {
		for _, s := range all {
			fmt.Println(s.name)
		}
		return
	}
	var picked []scene
	for _, s := range all {
		if q := strings.Join(flag.Args(), " "); q == "" || strings.Contains(s.name, q) {
			picked = append(picked, s)
		}
	}
	if len(picked) == 0 {
		fmt.Fprintf(os.Stderr, "demo-story: no scene matches %q (try -list)\n", strings.Join(flag.Args(), " "))
		os.Exit(2)
	}
	if *printMode {
		if err := printFrames(picked, *at, *w, *h); err != nil {
			fmt.Fprintln(os.Stderr, "demo-story:", err)
			os.Exit(2)
		}
		return
	}
	if _, err := tea.NewProgram(&model{scenes: picked}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "demo-story:", err)
		os.Exit(1)
	}
}

// printFrames prints each scene's pages at the times in at, framed by a
// ruler so trailing blanks show.
func printFrames(ss []scene, at string, w, h int) error {
	var times []string
	for _, a := range strings.Split(at, ",") {
		if a = strings.TrimSpace(a); a != "end" {
			if _, err := time.ParseDuration(a); err != nil {
				return fmt.Errorf("-at %q: %w", a, err)
			}
		}
		times = append(times, a)
	}
	for _, s := range ss {
		sc := s.make()
		sc.View(w, h) // size the scene before looking at it
		for i := range sc.Pages() {
			for _, a := range times {
				t := sc.PageLen(i)
				if a != "end" {
					t, _ = time.ParseDuration(a)
				}
				sc.Seek(i, t)
				fmt.Printf("=== %s · page %d/%d · %v of %v\n", s.name, i+1, sc.Pages(), t, sc.PageLen(i))
				for _, l := range strings.Split(sc.View(w, h), "\n") {
					fmt.Println(l + "\x1b[m│")
				}
			}
		}
	}
	return nil
}

type model struct {
	scenes []scene
	i      int
	cur    *story.Scene
	w, h   int
	last   string // what the scene last emitted
}

func (m *model) open(i int) tea.Cmd {
	n := len(m.scenes)
	m.i = (i%n + n) % n
	m.cur = m.scenes[m.i].make()
	m.last = ""
	return m.cur.Init()
}

func (m *model) Init() tea.Cmd { return m.open(0) }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "n":
			return m, m.open(m.i + 1)
		case "p":
			return m, m.open(m.i - 1)
		case "r":
			return m, m.open(m.i)
		}
	case story.Done:
		m.last = fmt.Sprintf("story.Done%+v", msg)
		return m, nil
	}
	_, cmd := m.cur.Update(msg)
	return m, cmd
}

var bar = lipgloss.NewStyle().Reverse(true)

func (m *model) View() tea.View {
	w, h := m.w, m.h
	if w <= 0 || h <= 0 {
		w, h = 80, 24
	}
	status := fmt.Sprintf(" demo-story %d/%d %s · n/p scenes · r replay · ctrl+c quit", m.i+1, len(m.scenes), m.scenes[m.i].name)
	if m.last != "" {
		status += " · emitted " + m.last
	}
	status = ansi.Truncate(status, w, "…")
	status += strings.Repeat(" ", max(w-ansi.StringWidth(status), 0))
	v := tea.NewView(m.cur.View(w, h-1) + "\n" + bar.Render(status))
	v.AltScreen = true
	return v
}
