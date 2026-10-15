# S9 Part 2: wiring the story (from the S9 Part 1 report, 10-15)

Part 1 is on main: `internal/flavor` story table and pacing (`Story`,
`StoryBlock`, `Fill`, `Ring`, `NextBeat`, `BeatLines`, `BeatTitle`,
`StoryLog`, `DefaultBeatGap = 8`), `game.Meta.Seen` (`HasSeen`, `MarkSeen`,
`RatedDoors`), `internal/anim` (Clip, Player, Motion, Typewriter, Reveal,
Fade, Wipe, Fall, Cheshire, Card, Hold/Stack/Then/Seq/Delay) and
`internal/screens/story` (scenes leave with `story.Done{Scene}`; the app
records `Scene` with `MarkSeen`). Preview: `go run ./cmd/demo-story -list`.

## Hook to screen (S9-P2 = story wiring; D = text into existing screens)

| hook | screen / moment | owner |
|---|---|---|
| story.prologue | NewPrologue before boot 1's first route, if not seen | S9-P2 |
| boot.splash.dmesg, boot.motd | NewBootSplash at boot start; motd also on the GRUB status line | S9-P2 / D (GRUB) |
| door.first.inscription | NewFirstDoor before boot 1 room 1 (instead of the doors header line) | S9-P2 |
| story.first-merge | NewFirstMerge before the first merge window | S9-P2 |
| daemon.intro | NewBeat, or a block above the daemon offers (merge.go:160) | D |
| story.first-boss, reveal.final-boss, boss.enter.* | NewBossIntro before boss rooms; boss.enter also on the boss door card (doors.go:160) | S9-P2 / D |
| boss.defeat.* | upgrade pick after a mid-boot boss; cleared screen after the last | D |
| story.first-crash | NextBeat at the crash, then NewFirstCrash | S9-P2 |
| reveal.prestige.*, reveal.final-clear, milestone.* | NextBeat at the boot end: teaser (first line) in RT1's kept block, a key opens NewBeat | D (+S9 NewBeat) |
| reread log | `/proc/story` reader page in GRUB from StoryLog | S9-P2 (screens/meta) |
| biome.intro.* | NewBiome at each biome start | S9-P2 |
| AC moment | NewAccepted before the upgrade pick | S9-P2 |
| story.first-rtfm, story.first-regression | RTFM browser / doors screen (or NewBeat) | D |
| grub.subtitle.*, merge.*, crash.epitaph.*, cleared.*, duck.*, daemon.*, upgrade.*, o3.bonus, regression.*, coredump.*, rtfm.* | the screens text.md names | D |
| verdict.<V> (the Queen's lines, D39), timer.warn, door.blind/mystery/cursed, lib.saved, quit.warn/resume.note | HUD, door cards, lib harvest; quit/resume with A2's D31 | D |

## Also

- Add the story funcs to `flavorSource` in `internal/screens/boot/env.go`.
- Carroll and SICP credit in the RTFM COLOPHON (`screens/rtfm/browser.go`).
- `anim.SetMotion` from the settings (`ui.motion`, `ui.fps`).
- Switch `story/style.go` and `anim/style.go` (SPONGE) to `internal/ui`.

## Defaults taken (10-15)

1. The 17 milestone beats in text.md are S9's draft (with a new "Turing
   note" thread); ship as drafts, storyteller revises.
2. NewFirstCrash plays by itself after the panic reveal on the first crash
   only; every other beat is RT6's teaser line plus a key.
3. Biome drops continue by themselves after about 2.5 s; any key goes
   sooner.
4. `grub.subtitle.nightly` now reads "...one seed for everyone, today"
   (RT15 acceptance: no "today only").
5. `regression.door` "Solve it to-day." is mild date pressure under GR2;
   storyteller decides.
6. Milestones "first regression retired" and "first topic mastered" need
   engine state on Meta first (A2 / S10 lane).
