# katharsis UI system: audit and proposal

Status: proposal, report only. No code was changed for this report.
Implemented in `internal/ui` (phase C), with the owner decisions of
2026-10-14 in place of §5: dark terminals and the 16 ANSI colors only, so
no `adaptive` palette (D26); difficulty stays `◆1300` in the rank ramp
everywhere, and capabilities move to `■`/`□` (D27).
Lens: the visual system (color, glyphs, borders, type, spacing, components).
The player journey is covered in `docs/design/ux-journey.md`; this report does
not repeat it.

Audited 2026-10-14. Other agents were editing `screens/boot`, `hud`, `reader`
and `meta` while this was written, so `./path:line` citations can be a few
lines off. Function names are given with each line number, so a search
still finds the spot.

## How the evidence was gathered

| What | How |
|---|---|
| Screens | `demo-boot` (13 pages), `demo-meta` (GRUB, GRUB edit, git log, expanded log, top, sorted top, statement, editorial, learn page), `demo-learn` (RTFM index, module page), and the real game: GRUB, door picker, two doors (CE, sample WA, sample AC, full-test WA, two quacks, statement in the pane, resize to 80x24), upgrade picker, `Q Q` crash and its reveal, git log, top, RTFM, practice HUD |
| Sizes | 80x24, 120x36, 180x48, plus 80x16 and 60x24 for the compact layouts |
| Isolation | a throwaway tmux server per capture (`tmux -L katharsis-ui-<pid> -f /dev/null`), `capture-pane -p` and `-p -e`; temp data dir, temp runs and templates; every server was killed afterwards |
| Color | SGR codes from `capture-pane -e`, shown in excerpts with the tags below |
| Light/dark | `capture-pane` records ANSI indices, not the RGB the terminal paints. Light-theme behavior is **computed** from the themes' published palettes with the WCAG 2 contrast formula. It was not observed on screen |
| Widths | `ansi.StringWidth` (what the layout code uses), tmux layout (seen in the captures), and vim's `strdisplaywidth()` under `vim -u NONE` |

Tags used in the excerpts: `«R»` red (1), `«G»` green (2), `«Y»` yellow (3),
`«B»` blue (4), `«M»` magenta (5), `«C»` cyan (6), `«K»` ANSI 8 (SGR 90),
`«d»` faint, `«b»` bold, `«i»` italic, `«v»` reverse, `«k»` blink,
`«256:n»` a fixed 256-color foreground, `«256-bg:n»` a fixed 256-color
background, `«C-bg»` cyan background, `«/»` reset.

## Frame: two skins, one meaning layer

The screens fall into two families. Most of the drift that matters runs
*across* them.

```
game skin (legible first)               parody skins (each mimics a real tool)
├─ boot header + footer                 ├─ GRUB menu + edit       grub
├─ route, doors, map, merge, upgrade    ├─ kernel panic           dmesg
├─ problem pane (HUD bar + HUD)         ├─ cleared                systemd shutdown
└─ practice HUD                         ├─ git log                git log --graph
                                        ├─ top                    htop
                                        └─ RTFM index/module      man(1)/less
                                           reader                 less
```

Proposed rule: a parody keeps its tool's own look: reverse rows in GRUB and
man, htop's function-key bar, man's `NAME`/`SEE ALSO`. That is theme, not
drift. What both families must share is the **meaning layer**: what each
color means, what each glyph means, how numbers are written, where the keys
sit, and how text is cut. The audit below scores against that layer.

---

## 1. Current state

### 1.1 Six palettes, one per package

Each package declares its own colors (`var red = ...`). None of them share
code.

| Package | Declared at | Colors and attributes |
|---|---|---|
| screens/boot | `./internal/screens/boot/style.go:15-32` | 1 2 3 5 6, faint, bold, italic; blink in `hudbar.go:28` |
| screens/meta | `./internal/screens/meta/meta.go:191-203` | 1–6, **8**, reverse, bg 6 + fg 0 (`field`); bg 2 + fg 0 in `top.go:256` |
| screens/learn | `./internal/screens/learn/learn.go:162-171` | a copy of meta's, with the comment "as in screens/meta" |
| hud | `./internal/hud/hud.go:380` (`green`, `red`, `yellow`, `faint`, `bold`) | 1 2 3 |
| app | `./internal/app/problem.go` (`faint`, `bold`, `red`, `green`) | 1 2, reverse flash |
| screens/reader | `./internal/screens/reader/reader.go:251,273-277` | 1, reverse, and Glamour's **"dark" style: fixed 256-color** (body 252, headings 39, H1 228 on 63, code 62/209/210/42/187) |

The game skin keeps to the 16 ANSI colors, as `style.go:13-14` intends ("so
every screen follows the user's theme"). Two places break that: the reader
(fixed 256 colors) and meta/learn's `gray` (ANSI 8).

### 1.2 What each hue means today

| Hue | Meanings found | Verdict |
|---|---|---|
| red | cursed, crash, WA/RE/CE, timer under 20%, panic line, `:q!` confirm, **hearts**, "editorial" count in top, CF grandmaster rating | coherent: loss and danger. Hearts are the iconic exception |
| green | AC, ✓, cleared, pass, HP fill, rewards `+80 B`, **uncommitted bytes (the at-risk ones)**, committed bytes on cleared, core dump, a calm timer, CF pupil 1200–1399, "independent" count | overloaded: success, gain, "safe time" and the money you can still lose |
| yellow | **◆ difficulty glyph**, timer warn, **"you are here"** on the map, retreat, regression, `-O3`, ★ prestige, TLE/MLE/OLE, git hash, "abandoned", "hinted", CF master, the current search hit, the closing-vim notice | the most overloaded hue, and the weakest on light backgrounds |
| cyan | **selection** (card frame, list cursor), **blind ◌**, **tags** in the HUD, the git log filter value, htop labels, top's selected row, CF specialist, sparkline | focus and two unrelated door facts |
| magenta | boss ⚔, mystery ?, CF candidate master | "special" door, acceptable |
| blue | CF expert 1600–1899 only | fine; weak on dark themes |
| ANSI 8 | CF newbie <1200 (log, top, RTFM, rating chart) | **invisible on Solarized Dark**, where ANSI 8 is the background (§1.4) |

### 1.3 Exhibits

Each exhibit gives the claim, then the capture, then the code.

**E1. Two HP bars for one submit, in two colors.** Real game, HUD pane 41
columns, after a full-test WA:

```
«G»⏱ 16:28«/»  «R»♥♥«d»♡♡♡«/»  🦆2  «G»42 B«d» +41 B«/»
«d»tags «C»implementation · two pointers«/»
«G»▓▓▓▓▓▓▓▓▓▓«d»░░░░░░░░░░«/» 16/30«d»  1 submit · WA          ← HUD bar: green, 20 cells
«d»asan+ubsan · built 1.2s · «/»0/1
submit «R»▓▓▓▓▓▓▓▓▓░░░░░░░░«/» 16/30  «R»WA«/» #1 «d»li…       ← HUD: same result, red, 17 cells
```

The boot HUD bar draws the best submit (`hpBar`,
`./internal/screens/boot/hudbar.go:135`). The embedded HUD draws the latest
one (`submitLines`, `./internal/hud/command.go:191`, with its own `hpBar` at
`:181`). The problem pane stacks both (`./internal/app/problem.go:195`).

**E2. The reader is fixed dark-theme 256-color.** Learn page, demo-meta:

```
  «b»«256:39»## Introduction«/»
  «256:252»Given an array a[1], a[2], …, a[N], a «b»prefix sum«/»«256:252» array stores …
```

Body text 252 (#d0d0d0) on a white background is **1.5:1**. Headings 39
(#00afff) are 2.4:1. `reader.go` hard-codes `glamour.WithStandardStyle("dark")`
(`:251`, default `style: "dark"`), and nothing calls `WithStyle`. Statements,
editorials and every learn module go through this one renderer, so on a
light background most of the reading in the game sits at 1.5:1.

**E3. One glyph, two or three meanings.**

| Glyph | Meaning A | Meaning B | Meaning C |
|---|---|---|---|
| ◆ | difficulty: `«Y»◆«/» 1300` (`doors.go:252`, `route.go:143`, map header `bootmap.go`) | held capability: `◆ 1/3 held`, `«G»◆ CAP_TWOPTR«/»` (`learn/module.go:227-229`, `browser.go`) | |
| ↺ | regression: `«Y»↺ regression«/»` (`doors.go`, `hudbar.go`) | retreat: `«Y»↺ retreat«/»` (`bootmap.go:291`) | git log uses `«Y»↩«/»` for retreat (`log.go:295`) |
| ▸ | cursor: `▸ 1`, `▸ Overclock`, `▸ 1   WA` | folded section: `▸ ` (`learn/browser.go:638`) | breadcrumb: `RTFM ▸ Silver ▸ Prefix Sums` (`learn/module.go:92`) |
| ? | mystery door | unknown biome on the map: `╭─ ? ─╮` (`env.go:147`) | unreliable call-trace frame `? door_2` (dmesg's own idiom, fine) |
| ⏱ | countdown in a boot, colored by urgency | **elapsed** time in practice: `prac  ⏱ 00:33`, uncolored | |

**E4. A rating's color depends on the screen.** The same door rating is
drawn three ways:

```
doors (boot):    «Y»◆«/» 1300  ⏱ 30:00  «b»«G»+80 B«/»          plain number, yellow glyph
git log (meta):  ✓ The Bucket List    sorting-greedy «G»1280«/» 13:02   CF rank color
RTFM (learn):    1  Breed Counting     Very Easy  «G»1250«/»          CF rank color
```

meta and learn color ratings by Codeforces rank (`ratingStyle`,
`meta.go:258`, `learn.go:238`). The boot screens never do.

**E5. Four byte formatters.** In the real crash screen, two lines apart:

```
[  122.175] door_2: open cf:1462A (800, 17:00, 41B)        ← engine fmtBytes: "41B"
  core dumped: «b»«G»42 B«/» on "Favorite Sequence"         ← boot FormatBytes: "42 B"
```

| Formatter | 12 800 bytes renders as |
|---|---|
| `boot.FormatBytes` (`style.go:221`) | `12.5 KiB` |
| `meta.Bytes` (`meta.go:103`) | `12 KiB` (no decimal at 10 or more, rounds half to even) |
| `game.fmtBytes` (`engine.go:745`) | `12.5KiB` (no space) |
| `judge.Bytes` (`judge/run.go:312`) | `12.5K` (this one is RSS, not the currency) |

These values are computed from the code, not captured. A bank of 12.5 KiB
reads `12.5 KiB` in the boot header and `12 KiB` in top.

**E6. Four clock formatters.** The map shows `09:20 / 15:00`, the expanded
git log shows `8:41`, and the engine log writes `17:00`. Minutes are
zero-padded in boot (`fmtClock`, `style.go:244`, which truncates) and in hud
(`hud.go:589`, which rounds). They are unpadded in meta (`meta.go:126`) and
the engine (`engine.go:735`).

**E7. Eight key-help dialects.** Every capture, last row:

| Screen | Footer as captured | Style |
|---|---|---|
| route, doors, map, merge, upgrade | `«b»←/→«/» «d»select · «/»«b»1-3«/» «d»jump · …` | `hint()`: bold key, faint label |
| cleared | same, but **centered** (`cleared.go:129`) | hint, centered |
| crash | `«b»[u]«/» upsolve  «b»[e]«/» editorial  «b»[r]«/» reboot …` | bracketed keys, plain labels |
| RTFM module | `[r] read  [t] start trial  [o] usaco.guide  [q] back` | brackets, no emphasis |
| RTFM index | `«v» rtfm(7) line 1/30 · ◆ 1/3 held   enter open · tab fold …«/»` | man's reverse status bar |
| top | `j/k«C-bg»Move  «/»s«C-bg»Sort RATING «/»q«C-bg»Back«/»` | htop's function-key bar |
| git log, reader, problem pane, practice HUD, `ld` link screen | `«d»j/k move · enter expand · f filter · g/G ends · q back«/»` | all faint, keys not emphasized |
| GRUB | "Use the ↑ and ↓ keys to select which entry is highlighted…" | GRUB's own prose |

GRUB, top and man are parodies, so their footers are right. The rest is
drift: three styles for the game skin and four within meta and learn. The
RTFM index and RTFM module are in one package and use two different footers.

**E8. Five selection idioms.** Card: heavy frame + cyan + `▸ N`. Boot list:
cyan bold `▸ name`. Meta/learn list: reverse row with its colors stripped
(`log.go:174`, `browser.go:601`). top: cyan background with colors stripped
(`top.go:297`). HUD test rows: `▸ ` only. Stripping is why a selected git
log row or top row loses its state color (see E11).

**E9. Bold on weak hues.** `sGreen.Bold` (`+80 B` reward, `✓ AC`, `ALL
PASS`, core dump, `Boot #42 cleared`), `sYellow.Bold` (`●` here, `-O3`, `★
prestige unlocked`) and `sCyan.Bold` (the selected `▸ 1`, the heavy frame).
Terminals that "brighten bold" turn these into bright green, yellow and cyan,
which reach 1.4–1.9:1 on white (§1.4). iTerm2 and Terminal.app ship with that
option on (inference, from their default profiles).

**E10. Blink on the critical timer.** `toneCrit: sRed.Bold(true).Blink(true)`
(`hudbar.go:28`) shows as `«b»«k»«R»⏱ 01:40«/»`. The HUD sits in the
player's peripheral vision while they type in vim. For two minutes, a blink
there competes with the code they are writing.

**E11. Encoded by color alone.** Real top, with colors stripped (monochrome
or a red-green deficit):

```
  TOPIC             ATT   CLR  CLR%  RATING  CLEARS / ATTEMPTS
  strings             1     1  100%     946  [|||||||||||||||||||||||||||||||]
  two-pointers        1     0    0%     946  [|||||||||||||||||||||||||||||||]
```

100% and 0% look identical: both segments use `|`, told apart only by
green versus red (`top.go:316`). The timer's calm, warn and low states are
also color only. Hearts (♥/♡), map nodes (✓●○☠⚔), frames (thin/heavy/double)
and verdict words are redundant with shape or text. Those are good.

**E12. The HUD pane is too narrow at 80 columns.** The editor takes 65% of
the width (`Split`, `./cmd/katharsis/game.go:120`). That leaves the HUD 41
columns at 120 and about 27 at 80 (21 after a live resize). `HUDBar`
documents "35 to 60 columns". At 80x24:

```
⏱ 15:59  ♥♥♡♡♡  🦆1 …
tags implementation ·
     two pointers
technique two-pointe…
submit ▓▓▓░░░ 16/30 …
vim  :w samples · :S…
s submit · t stateme…
```

Even at 41 columns, both help rows are cut: `vim  :w samples · :Submit ·
:Debug n · :…` and `s submit · t statement · ? duck · R retr…`.

**E13. Door cards.** In real data:

```
 ┏━ ▸ 1 ━━━━━━━━━━━━━━━━━━━━ ◌ blind ━┓  ╭─ 2 ────────────────────────────────╮  ╭─ 3 ────────────────────────────────╮
 ┃ Codeforces · Codeforces Round 172… ┃  │ Codeforces · Codeforces Round 666… │  │ Codeforces · Codeforces Round 921… │
 ┃ Word Capitalization                ┃  │ Juggling Letters                   │  │ We Got Everything Covered!         │
 ┃ tags hidden                        ┃  │ greedy · strings                   │  │ constructive algorithms · greedy · │
 ┃                                    ┃  │                                    │  │ strings                            │
 ┃ ◆ 800  ⏱ 15:00  +63 B              ┃  │ ◆ 800  ⏱ 15:00  +42 B              │  │                                    │
 ┃ ≈76% clear                         ┃  │ ≈76% clear                         │  │ ◆ 800  ⏱ 15:00  +42 B              │
 ┃ ◌ blind: tags hidden · ×1.5        ┃  │                                    │  │ ≈76% clear                         │
 ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛  ╰────────────────────────────────────╯  ╰────────────────────────────────────╯
```

- The stats row floats. When one card's tags wrap, its stats drop a row, so
  the three cards' stats rows do not line up.
- The source says its name twice. `sourceLabel` prefixes the contest name
  with the site name (`env.go:243`), which gives `Codeforces · Codeforces
  Round 172…`, and the clip removes the useful part, the round number.
- A blind card says "tags hidden" twice: the tag line (`doors.go:240`) and
  the modifier line (`doors.go:272`).

**E14. The statement opens in the 41-column HUD pane.** The line length is
37 characters. The title shows twice: the reader header, then Glamour's H1
(`«b»«256:228»«256-bg:63» Favorite Sequence «/»`). Line breaks land
mid-phrase: `Time limit: 2 s · Memory limit: 256` / `MB`.

**E15. The vim pane border, the timer the player actually watches, is
plain.** `@katharsis_border " ⏱ 16:10 · ♥♥ · 🦆1 · samples 0/1 · HP 16/30
WA "`. It has no tone, even in the last two minutes, and it draws hearts as
`♥♥` where the HUD draws `♥♥♡♡♡`. `Border.Set` escapes every `#`
(`./internal/session/panes.go:136`), which also rules out tmux `#[fg=…]`
styles.

**E16. The crash prints from the top, like a file, not like a console.**
At 120x36, the panic, quote and keys fill 24 rows and 11 empty rows sit
below. The key line lands mid-screen, while every other screen pins its keys
to the last row (`crash.go:119` passes no footer to `layout`). Older log
lines are capped at 4.

**E17. Cleared.** The CLEARED banner and `[  OK  ]` render in default
color. systemd paints `OK` green, so the parody leaves its brightest
moment unused. ★ is bold yellow (E9). The footer is centered (E7). The
flavor package draws an `ACCEPTED … exit 0` banner (`flavor/banner.go:10-16`)
that nothing uses.

**E18. Smaller drift.**

- Separator spacing: the reader uses `"  ·  "` (`reader.go:286`), everything
  else `" · "`.
- `spread` cuts opposite ends. In boot it drops the right side
  (`style.go:102`). In meta and learn it truncates the left (`meta.go:246`).
- The expanded git log glues rating to topic, so the column floats:
  `simulation 1150      8:41` vs `sorting-greedy 1280 13:02` (`log.go:301`).
- GRUB edit shows two cursors: a reverse value plus an appended `█`
  (`grub.go:318`).
- Plurals: `1 boots`, `1 crashes` (git log, top).
- The `:q!` confirm line pushes the whole HUD down one row while it shows.

### 1.4 Light and dark backgrounds (computed)

WCAG 2 contrast of the ANSI hues this code relies on, against the theme's
own background (4.5:1 is the AA bar for text, 3:1 for UI glyphs):

| Theme (bg) | green 2 | yellow 3 | cyan 6 | red 1 | magenta 5 | blue 4 | bold-bright green / yellow / cyan |
|---|---|---|---|---|---|---|---|
| Terminal.app Basic (white) | 3.3 | 3.0 | 3.0 | 8.9 | 5.9 | 12.8 | 1.9 / 1.4 / 1.6 |
| xterm defaults (white) | 2.2 | 1.7 | 2.0 | 5.8 | — | — | — |
| Solarized Light (#fdf6e3) | 3.0 | 3.0 | 2.9 | 4.3 | 4.2 | 3.4 | — |
| xterm defaults (black) | — | — | — | 3.6 | — | **2.2** | — |
| Solarized Dark (#002b36) | — | 4.7 | — | — | — | — | ANSI 8 = **1.0** |
| Glamour "dark" body 252 on white | 1.5 | | | | | | |

The palette values come from the themes' published values, recalled from
memory; they are cheap to check. Faint black on white is about 4:1, so
`faint` holds up in both modes.

Takeaways:

- Red, magenta and blue survive light themes.
- Green, yellow and cyan sit at 2–3.3:1 on light themes, and at 1.4–1.9 when
  bolded on a bold-brightening terminal.
- ANSI 8 vanishes on Solarized Dark.
- The reader is the only surface that is unreadable on light themes, not
  just weak.

### 1.5 Glyph widths

| Glyph | Go (`ansi.StringWidth`) | tmux (layout in the captures) | vim `strdisplaywidth` |
|---|---|---|---|
| ⏱ U+23F1 | 1 | 1 | **2** |
| 🦆 U+1F986 | 2 | 2 | 2 |
| ♥ ♡ ☠ ⚔ ↩ ↺ ◆ ◌ ✓ ✗ ★ ● | 1 | 1 | 1 |

Only ⏱ disagrees, and only in vim. `statusLine` already keeps ⏱ out of
vim's message line (`./internal/app/problem.go:324`, see the comment above
it). ⏱, ☠, ⚔, ↩ and ♥ are emoji that render as text by default. A terminal
without a text glyph for them substitutes a color emoji, which can paint
over the next cell (inference: this depends on the font). The code already
follows each of them with a space, except the compact door rows: `4 ??? ☠?`
(`doors.go:356` compact).

### 1.6 What the eye hits first

| Screen | First | Then | Problem |
|---|---|---|---|
| GRUB | banner | the reverse row | none; authentic |
| route / doors | the selected card's heavy cyan frame | bold titles, red cursed frame, **bold green rewards** | the reward is louder than difficulty and time; three bold-green `+N B` compete |
| merge | the map | bold section labels | flat but clear |
| upgrade | `✓ AC Moocast … -O3` | the cards | fine |
| problem pane | green `⏱` | then **everything at equal weight**: tags, two HP bars, sample status, rows | no grouping; E1 |
| crash | red bold panic line | italic quote, core-dump line | keys mid-screen at large heights (E16) |
| cleared | uncolored banner | green `Boot #42 cleared`, yellow ★ | the climax is the least colored line |
| git log / top / RTFM | yellow hashes / cyan labels / the man header | | authentic |
| reader | Glamour H1 block (yellow on purple) | blue `##` headings | off-palette (E2) |

### 1.7 Truncation, empty states, motion

- **Truncation** always marks cuts with `…` at cell width. Good: `clip` and
  `ansi.Truncate` everywhere, and the card top edge clips the title before
  the danger tag (`style.go:53-81`). Bad: footers and the source label lose
  their useful part mid-word (E12, E13), and `spread` disagrees on which
  side to drop (E18).
- **Empty states** are consistent: faint, lowercase, no period. "no doors on
  offer", "no doors opened yet", "no boots match filter: …", "not enough
  rated attempts for a trend", "no tags", "not submitted". Keep them. Only
  the indent varies (0 or 2 cells).
- **Motion.**
  - The crash reveal: 600 ms, line by line, any key skips (`crash.go:16-48`).
    The best piece of motion in the game. It prints top-down (E16).
  - The 1 Hz countdown. Blink from 2:00 (E10).
  - The live `judging n/N…` counter.
  - AC cuts straight to the upgrade picker: punishment gets a beat, reward
    gets none (E17).

---

## 2. Proposed visual system

### 2.1 Rules

| # | Rule | Why |
|---|---|---|
| R1 | Every state shown in color is also shown by a glyph, a word or a shape | color-blind and NO_COLOR safety (E11) |
| R2 | The weak hues (green, yellow, cyan) paint glyphs, frames, bars and numerals only; words in those states stay `fg`/`strong`. Red and magenta may paint words | light themes (§1.4) |
| R3 | Never bold a colored token, except `danger`. Emphasis is bold on the default foreground | bold-as-bright (E9) |
| R4 | No ANSI 8 anywhere; "gray" is `faint` | Solarized Dark (§1.4) |
| R5 | No fixed 256-color values by default; the reader uses a katharsis Glamour style built from tokens | follow the user's theme (E2) |
| R6 | No blink; critical states use reverse video | peripheral vision (E10) |
| R7 | One hue, one meaning. The CF rank ramp is the one sanctioned overload, and it paints rating numerals only | E3, E4 |
| R8 | Parody skins keep their tool's idiom; tokens, glyphs, formats and the footer row are shared | §Frame |

### 2.2 Color tokens

Default palette `ansi` follows the terminal theme. Optional palette
`adaptive` (a setting, `ui.palette`) replaces the three weak hues with darker
256-color values when Bubble Tea's `BackgroundColorMsg.IsDark()` reports a
light background. The `tea.RequestBackgroundColor` and `IsDark` APIs exist in
bubbletea v2.1.0. Whether tmux 3.6a passes OSC 11 through is unverified;
`COLORFGBG` is the fallback.

| Token | Means, and only this | `ansi` | `adaptive`, light bg | `adaptive`, dark bg | Paints |
|---|---|---|---|---|---|
| `fg` | body text | default | default | default | words |
| `strong` | names, titles, values, key caps | default + bold | same | same | words |
| `muted` | secondary text, labels, idle frames, hidden info | faint | faint | faint | anything |
| `focus` | cursor, selected frame, "you are here", active filter | 6 | 30 `#008787` (4.4:1) | 6 | glyphs, frames |
| `ok` | success and gain: AC, ✓, cleared, HP fill, `+bytes` | 2 | 28 `#008700` (4.7:1) | 2 | glyphs, bars, numerals |
| `warn` | time pressure, soft caution: timer ≤50%, TLE/MLE/OLE, regression, retreat, hinted | 3 | 94 `#875f00` (5.7:1) | 3 | glyphs, numerals |
| `danger` | failure, loss, lethal: WA/RE/CE, crash, cursed, timer ≤20%, `:q!` confirm | 1 | 1 | 1 | words allowed; bold allowed |
| `special` | boss and mystery doors | 5 | 5 | 5 | words allowed |
| `life` | ♥ (lost ♡ is `muted`) | 1 | 1 | 1 | glyph only |
| `select` | parody list selection (GRUB, git log, man, RTFM) | reverse | reverse | reverse | the row; keep its glyphs |
| `field` | htop's bar, **top only** | bg 6 + fg 0 | same | same | top |

The `adaptive` light values were checked with the same formula as §1.4: on
white, 30 = 4.4:1, 28 = 4.7:1, 94 = 5.7:1. On #fdf6e3 they are 4.0, 4.4 and
5.3.

Moves this table implies:

| Today | Proposed | Reason |
|---|---|---|
| ◌ blind in cyan | `muted` | cyan is focus; a dotted glyph in faint already reads as "hidden" |
| HUD tags in cyan | `fg` (label `muted`) | tags are content, not focus |
| map "here" ● and frame in bold yellow | `focus` | "where you are" is focus |
| uncommitted bytes in green | `fg`; only `+N B` gains are `ok` | the at-risk balance should not read as "good" |
| ◆ difficulty glyph in yellow | rating token (§2.3) | E3, E4 |

### 2.3 Rating token

One token for every rating, everywhere: `*1300`, Codeforces' own notation
for a problem-difficulty tag. It is ASCII and one cell per character, and it
frees ◆/◇ for capabilities, exactly as DESIGN.md draws them. The numeral
takes the CF rank color:

| Rank | Range | Paint |
|---|---|---|
| newbie | <1200 | `muted` (not ANSI 8) |
| pupil | 1200–1399 | 2 |
| specialist | 1400–1599 | 6 |
| expert | 1600–1899 | 4 (`adaptive` dark: 12) |
| candidate master | 1900–2099 | 5 |
| master | 2100–2399 | 3 |
| grandmaster | ≥2400 | 1 |

Rank colors appear on rating numerals and nowhere else. Players read the
convention natively, and the number carries the value, so R1 holds.

### 2.4 Glyph set

The ASCII fallbacks apply under a setting (`ui.glyphs = unicode | ascii`),
switched on automatically when the locale is not UTF-8 or `TERM=linux`. Door
modifiers fall back to the kernel taint letters the crash screen already
prints (`crash.go:340-361`), so the same letter means the same modifier on
every screen.

| Meaning | Glyph | ASCII | Token | Notes |
|---|---|---|---|---|
| countdown | `⏱ 23:41` | `T-23:41` | timer tone (§2.8) | never sent to vim (2 cells there) |
| elapsed / uptime | `up 00:33` | same | `muted` label | practice HUD, headers; no ⏱ |
| life / life lost | `♥` / `♡` | `#` / `-` | `life` / `muted` | border text shows both, like the HUD |
| duck | `🦆1`, `🦆✗` | `duck 1`, `duck x` | `fg` | the only true emoji; 2 cells everywhere |
| pass, AC, cleared | `✓` | `+` | `ok` | |
| fail | `✗` | `x` | `danger` | |
| crash, lethal | `☠` | `!` | `danger` | also cursed: "this kills you" |
| boss | `⚔` | `X` | `special` | taint letter (`X (boss)` on the crash screen); on the RTFM map, a division's conclusion (its castle) |
| mystery | `?` | `M` | `special` | taint letter |
| blind | `◌` | `B` | `muted` | taint letter |
| cursed | `☠` | `C` | `danger` | taint letter |
| regression | `↺` | `R` | `warn` | taint letter; **only** meaning of ↺ |
| retreat | `↩` | `<` | `warn` | NetHack's up-stairs; **only** meaning of ↩ |
| you are here / ahead | `●` / `○` | `@` / `.` | `focus` / `muted` | `@` is the roguelike player |
| capability held / locked | `■` / `□` | `[x]` / `[ ]` | `ok` / `muted` | **only** meaning of ■/□; ◆ stays difficulty (D27) |
| difficulty | `*1300` | same | rank ramp | §2.3 |
| reward, multiplier, chance | `+80 B`, `×1.5`, `≈74%` | `+80 B`, `x1.5`, `~74%` | `ok`, `fg`, `muted` | |
| cursor | `▸` | `>` | `focus` | **only** meaning of ▸ |
| fold open / shut | `▾` / `▹` | `-` / `+` | `muted` | not ▸ |
| RTFM page (reading only) | `▪` | `=` | `muted`, `fg` once opened | the map's node for a level with no trial; never ticked |
| RTFM warp | `↑` | `^` | `special` | a shortcut to a later world (a real prerequisite edge) |
| RTFM path | `─` `│` `╭╮╰╯` | `-` `\|` `+` | `fg` lit, `muted` ahead | Rule's cell and Thin's sides and corners |
| RTFM optional path | `┄` / `┆` | `:` | as the path | into an optional or never-asked level |
| chain, breadcrumb | ` › ` | ` / ` | `muted` | the map's line view already uses › (`bootmap.go:155`) |
| separator | ` · ` | ` \| ` | `muted` | exactly one space each side |
| bar fill / empty | `▓` / `░` | `#` / `.` | `ok` / `muted` | |
| achievement | `★` | `*` | `warn` glyph, `strong` words | |
| truncation | `…` | `...` | — | |

Spacing rule for glyphs: a non-ASCII glyph is always followed by a space, a
digit or the end of its token. It is never directly followed by another
glyph or a border character (fixes `☠?` in the compact door rows).

### 2.5 Borders

This codifies what `style.go:34-51` already does almost everywhere.

| Border | Means | Color |
|---|---|---|
| rounded `╭─╮` | an idle, choosable card, or a region (map biome) | `muted`; `danger` if cursed; `ok` if done |
| heavy `┏━┓` | **focus**: the selected card | `focus`; `danger` if the card is cursed |
| double `╔═╗` | climax: boss door (and, proposed, the cleared banner) | `special` |
| plain `┌─┐` | parody chrome (GRUB) | default |
| none | text surfaces: reader, dmesg, logs, man pages | — |
| rule `────` | header/body and section separators, full width | `muted` |

Selection changes the border's shape as well as its color, so it reads in
monochrome. ASCII mode keeps that: idle `+-|`, focus `#=#`.

### 2.6 Type and spacing

- **Attributes.** `bold` marks names, titles, values and key caps. `faint`
  marks secondary text. `italic` is the flavor voice only: route blurbs, boss
  lines, quotes, "tags hidden". It already is; keep it that way.
  `underline` is reserved for search hits other than the current one.
- **Screen anatomy (game skin).** Rows 0–2 are the header (§2.8 C1), then the
  body, then the footer on the last row. Parody skins name their tool on row
  0 and also put their keys on the last row, which fixes E16.
- **Horizontal.**
  - One cell of padding inside frames.
  - Two-cell gutters between cards.
  - Two-cell indent for free text outside frames.
  - Table columns are separated by two or more spaces. Numbers align right,
    text aligns left, and every value has its own column (fixes E18's
    floating rating).
- **Vertical.**
  - One blank row between groups, never two (except for centering).
  - Choice screens (route, doors, upgrade, merge, cleared) center their block
    between header and footer, with at least one blank row under the header.
  - List and text screens (map, git log, reader) anchor to the top.
  - The crash anchors to the **bottom**, like a console: older lines scroll
    off the top, and the free rows fill with more log tail.
- **Width tiers.**

  | Width | Doors | Route |
  |---|---|---|
  | ≥90 | side by side | side by side |
  | 60–89 | stacked full width | stacked |
  | otherwise | one line per door | one line per route |

  This is already the logic in `doors.go` and `route.go`; keep it.

  The HUD pane gets a floor of 36 columns and a target of 40–48 (§2.8 C8).
  Text measure is 60–80 columns. A pager in a narrower pane zooms it
  (`resize-pane -Z`) while reading and unzooms on `q`, which fixes E14.
- **Truncation.**
  - `clip` with `…` at cell width.
  - In a two-sided line the right, lower-priority half drops first,
    everywhere (one `Spread`).
  - In tables the text column is cut; numerals never are.
  - Footers drop whole key pairs from the end, never half a label.

### 2.7 Number formats

One leaf package with no dependencies, so the engine can use it too:

| Value | Format | Examples |
|---|---|---|
| currency | `FormatBytes` as in `style.go:221`: integer B under 1 KiB, one decimal of KiB/MiB (`.0` trimmed), no decimal at 100 or more, always a space before the unit | `42 B`, `12.5 KiB`, `312 KiB` |
| memory (RSS) | `judge.Bytes` stays: `9M`, `1.2K` | it must not look like currency |
| countdown | `mm:ss`, then `h:mm:ss`, rounded **up** (`fmtCountdown`) | `14:51`, `1:12:30` |
| durations in tables and logs | `mm:ss` / `h:mm:ss`, zero-padded minutes | `08:41 / 15:00` |
| TIME+ (lifetime time inside doors) | top: as durations (`units.Clock`); GRUB's status line: `units.HMShort`; a boot's debrief: `units.HM`, as its budget line | `61:20:44`, `61h20m`, `29:47 → 30:46` |
| counts | `16/30` (no spaces); time pairs `09:20 / 15:00` (spaced) | |
| plurals | `plural(n, noun)` everywhere | `1 boot`, `2 boots` |

### 2.8 Components

**C1. Header bar (game skin).** Three rows. On the right, the less important
half drops first.

```
Boot #42 · Silver · The Heap · room 2/2                         uptime 47:12
♥♥♡  🦆1  96 B uncommitted · 112 B committed                daemons strace
────────────────────────────────────────────────────────────────────────────
```

- Row 1: `strong` "Boot #42"; `muted` separators; the location in `fg`. Boss
  rooms show `⚔ Heisenbug` in `special`.
- Row 2: `life` hearts; bytes in `fg` with `muted` labels (no green).
- Rule: `muted`.

**C2. Key-help footer.** The last row, left-aligned on every game-skin and
non-parody screen: git log, reader, problem pane, practice, `ld`, cleared.

```
←/→ select · 1-3 jump · enter open door · m map
```

- Key: `strong`. Label: `muted`. Separator ` · `: `muted`.
- Order: navigate, act, secondary, then back or quit last.
- When narrow, drop pairs from the end.
- No inline key hints in the body when the footer already lists the key.
  Merge's `c commit` appears twice today (`merge.go:140`).

**C2p. Pager status bar (parody: man/less).** RTFM index, RTFM module **and**
the reader share it, one idiom per skin. The RTFM module's bracket footer
(`module.go:235`) and the reader's faint footer both move to it.

```
 Manual page rtfm(7) line 1/30 · ◆ 1/3 held              enter open · tab fold · / filter · q quit 
```

- Reverse, full width. Position on the left; keys on the right.
- The crash keeps its `[u] upsolve` brackets: a console prompt is its idiom.

**C3. Card (door, route, upgrade).**

```
┏━ ▸ 1 · CF Round 690 (Div. 3) · A ━━━━━ ↺ regression ━┓
┃ Favorite Sequence                                    ┃
┃ implementation · two pointers                        ┃
┃                                                      ┃
┃ *800  ⏱ 17:00  +41 B                      ≈78% clear ┃
┃ ↺ crashed you 3 days ago                             ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
╭─ 2 · CF Round 921 (Div. 1) · A ─────────── ☠ cursed ─╮
│ ???                                                  │
│ tags hidden                                          │
│                                                      │
│ *1500  ⏱ 17:00  +210 B                    ≈52% clear │
│ ? mystery · ×2                                       │
│ ☠ no ducks · 1 WA = crash · ×2                       │
╰──────────────────────────────────────────────────────╯
```

Card 2 is a mystery door: `???` says the title is hidden, the tags slot
says the tags are hidden, so the `?` modifier line only carries the
multiplier.

- **Top edge, left:** index, then the source as `site · contest · index`,
  without repeating the site name. `CF` abbreviates Codeforces in the edge,
  because the round number is the information. **Top edge, right:** the most
  dangerous modifier.
- **Body, fixed slots:**
  1. title (`strong`; `special` for mystery `???`);
  2. tags (`fg`, or one `muted` *italic* "tags hidden" line, never twice);
  3. blank;
  4. stats row (`*rating` ramp, `⏱ timer` `fg`, `+bytes` `ok`, `≈clear`
     `muted`);
  5. modifier lines, glyph colored, words `fg`, except `☠`/`⚔` lines, which
     may be colored words.
- **Alignment:** pad cards *above* the stats row, so stats rows line up
  across a row of cards (E13).
- **Selection:** heavy frame + `focus` + `▸ N`. Cursed keeps `danger` in
  both states. Boss: double frame + `special`.

**C4. List row.**

```
  ▸ Overclock    +15% timer on later doors.
    ECC          +1 life.
    Rubber Duck  +1 duck (one hint charge).
```

- Two-cell cursor gutter: `▸ ` in `focus`, or two spaces.
- Name column padded to the longest name, `strong` when selected.
- Description `muted`.
- Parody lists use `select` (reverse) on the whole row and keep the row's
  glyphs. Stripping to plain text is fine only once R1 holds, because the
  words and glyphs then still carry the state.

**C5. Section panel (merge window).**

```
commit   176 B uncommitted · lost if you crash
upgrade  a clear earned one; pick it before you leave
           ▸ Overclock    +15% timer on later doors.
             ECC          +1 life.
daemon   pick one · held 1/3: strace
leave    → biome 3: route choice
```

The label column is 9 cells, `strong`. The body is `fg` and `muted`. Nested
rows are C4 rows.

**C6. HP bar.** Exactly one per pane.

```
HP ░░░░░░░░░░░░░░░░░░░░ not submitted
HP ▓▓▓▓▓▓▓▓▓░░░░░░░░░░░ 14/30  judging 22/30…
HP ▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░ 16/30  WA #17 · line 1
HP ▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░ 16/30  WA #17 · best 20/30
HP ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ 30/30  AC
```

- Fill `ok` (passed tests are progress). Remainder `muted`. The verdict
  token carries failure (`danger` WA/RE/CE, `warn` TLE/MLE).
- Width: `min(20, w − label)`.
- It lives in the HUD's submit line, which is live while judging. The boot
  HUD bar stops drawing its own bar when a HUD is embedded and appends
  `best n/N` when the best submit beats the latest one (fixes E1).

**C7. Timer.**

```
calm  ⏱ 23:41       ok on ⏱ and digits
warn  ⏱ 12:05       warn (≤ 50% left)
low   ⏱ 05:12       danger + bold (≤ 20%)        ← shape cue: bold
crit  ⏱ 01:40       danger + reverse (< 2:00)    ← shape cue: block, no blink
```

The tmux pane border uses the same tones with tmux styles (`#[fg=red,bold]`,
`#[reverse]`). Only the text gets `#` escaped, not the style directives
(`panes.go:136`). The border shows `♥♥♡♡♡` like the HUD (fixes E15).

**C8. Problem pane (assembled).** At 41 columns:

```
⏱ 16:28  ♥♥♡♡♡  🦆1  42 B  +41 B
↺ regression
tags implementation · two pointers
─────────────────────────────────────────
HP ▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░ 16/30  WA #17
samples 0/1 · asan+ubsan · built 1.2s
─────────────────────────────────────────
▸ 1   WA      90ms     9M  line 1
─────────────────────────────────────────
#1 WA  line 1
want 3 1 4 1 5 9 2
got  3 4 5 2 9 1 1

:w :Submit :Debug n :Quack :Retreat :q!
s submit · t stmt · ? duck · R retreat
```

- Three groups, separated by rules: vitals, judgement, detail.
- Below 52 columns, the vim row uses the bare command names, and the pane
  row drops `Q Q give up` (`:q!` is on the vim row).
- The `:q!` confirm replaces the pane-keys row instead of pushing the pane
  down.
- The split gives the HUD `max(36, min(48, 35% of the width))` columns (an
  absolute `-l`), not "the 35% left over from the editor's 65%".

**C9. Modifier tags.** `☠ cursed` (`danger` word), `⚔ Heisenbug`
(`special` word), `? mystery` (`special`), `◌ blind` (`muted`), `↺
regression` (`warn` glyph, `fg` word). Compact rows show glyphs separated
by spaces. ASCII mode shows the taint letters: `R M B C X`.

---

## 3. Migration map

### 3.1 New packages (one owner, contract first)

The boot helpers are the strongest code here: frames that change by glyph,
`layout` that pins the footer, and clipping that respects cell width.
Promote them rather than meta's copies.

```
internal/ui/text/    no deps beyond x/ansi; the engine may import it
  Bytes  Clock  Countdown  Plural
  Clip  Fit  Spread (drop right)  Center  WrapN  JoinH  PadLines
internal/ui/         lipgloss + ui/text
  tokens.go   FG Strong Muted Focus OK Warn Danger Special Life Select Field
              Rank(r) ; SetPalette(ansi | adaptive(dark bool))
  glyphs.go   G.{Timer Life LifeLost Duck Pass Fail Crash Boss Mystery Blind
              Cursed Regression Retreat Here Ahead Cap CapLocked Cursor Fold
              Unfold Crumb Sep Fill Empty Star} ; Taint letters ; SetGlyphs(ascii)
  frame.go    Frame(kind, w, title, tag, body)       ← boot box() + frame vars
  layout.go   Layout(w, h, body, footer)             ← boot layout()
  footer.go   Footer(w, pairs...)  StatusBar(w, left, right)
  hp.go       HP(passed, total, w, verdict, best)
  timer.go    Timer(left, total) ; BorderTimer(left, total) (tmux #[...] form)
  token.go    Hearts(l, max)  Rating(r)  Modifier(m)  Verdict(v)
  glamour.go  StyleConfig() — Glamour style built from tokens, Document color unset
```

### 3.2 Per package

| Package (owner) | Function / place | Adopts | Change |
|---|---|---|---|
| screens/boot (D1) | `style.go` vars, `box`, `layout`, `hint`, `hearts`, `FormatBytes`, `fmtClock`, `fmtCountdown`, `spread`, `clip`, `fit` | ui, ui/text | delete the local copies; keep thin aliases while migrating |
| | `hudbar.go` `toneStyle`, `timerTone` | `ui.Timer` | drop blink; add the bold/reverse cues |
| | `hudbar.go` `hpBar`, `HUDBar` | `ui.HP` | no bar when a HUD is embedded; `best n/N` text; tags in `fg` |
| | `doors.go` stats row (`:252`), edge tag, mods (`:270-318`), compact flags (`:356`) | `ui.Rating`, `ui.Modifier`, C3 | `*1300`; ◌ `muted`; one "tags hidden"; bottom-align stats; spaces between compact glyphs |
| | `env.go` `sourceLabel` (`:243`), `header` (`:287`) | C1, C3 | drop the repeated site name; uncommitted bytes in `fg` |
| | `route.go` band (`:143`) | `ui.Rating` | `*850–1050` in the rank ramp |
| | `bootmap.go` `glyph`, frames, `outcomeLabel` (`:291`) | glyphs, frames | here → `focus`; retreat ↺ → ↩ |
| | `merge.go:140`, `upgrade.go:172`, `cleared.go:117-129`, `crash.go:119,445` | C2, C5, R3 | drop the inline `c commit`; `✓` glyph in `ok` with `strong` words; cleared footer left; `[  OK  ]` OK in `ok`; crash bottom-anchored with keys on the last row |
| screens/meta (D2) | `meta.go` vars, `frame`, `pad`, `spread`, `center`, `Bytes`, `clock`, `ratingStyle` | ui, ui/text | `gray` → `muted`; `Bytes` → `text.Bytes`; `Spread` drops right |
| | `log.go:130` keys, `:174` selection, `:295` ↩, `:301` rating | C2, C4, Rating | footer `ui.Footer`; rating in its own right-aligned column; plurals |
| | `top.go:316` clears bar | R1 | clears `\|` vs fails `·` (or `█`/`░`), so state survives without color and on the selected row |
| | `top.go:132` `keyBar`, `field` | keep | parody; `field` stays top-only |
| | `grub.go:318` | keep reverse | one cursor: reverse the cell, drop the extra `█` |
| screens/learn (G) | `learn.go:162-260` copied helpers and `ratingStyle` | ui, ui/text | delete the copies |
| | `module.go:92` crumb, `:235` keys | glyphs, C2p | `RTFM › Silver › Prefix Sums`; pager status bar |
| | `browser.go:638` fold, `:661-680` | glyphs, C2p | `▸` → `▹`; already the reference status bar |
| screens/reader (D2) | `reader.go:251` `WithStandardStyle("dark")`, `:277` current hit, `:286` separator | `ui.StyleConfig()`, C2p | ANSI-token Glamour style (body = terminal fg; headings `strong`; no H1 block; code in ANSI hues); current hit = reverse + bold, other hits underline; ` · ` |
| hud (E) | `hud.go` style vars (`:380`), `verdictStyle` (`:387`), `fmtClock` (`:589`), footer (`:487`), `ALL PASS` (`:526`) | ui, ui/text, C2 | `✓ ALL PASS` with an `ok` glyph and `strong` words; one clock |
| | `command.go` `hpBar` (`:181`), `submitLines` (`:191`) | `ui.HP` | the single HP bar |
| | `statusText` (`:353`) | `ui.BorderTimer` | toned border text with ♡ |
| session (E) | `panes.go:136` `Border.Set` | — | accept pre-styled text: escape `#` in the user text, not in the `#[...]` directives |
| | `tmux.go` `SplitEditor` | C8 | size the HUD absolutely: `-l <cols>` on the HUD side |
| app (integrator) | `problem.go` styles, `View` footers (`:203`), confirm line, statement open | C2, C8 | compact help below 52 columns; confirm replaces the keys row; zoom the pane for the pager |
| | `learn.go` `linkScreen.View` footer | C2 | `ui.Footer` |
| | `app.go` theme | `SetPalette`, `SetGlyphs` | read the `ui.palette` and `ui.glyphs` settings; optional `RequestBackgroundColor` |
| game (engine) | `engine.go:735` `clock`, `:745` `fmtBytes` | ui/text | `41 B`, `17:00`, the same as the screens |
| flavor | `banner.go` | — | unchanged; `ac` banner available for the optional AC beat |
| cmd/demo-* | status bars (`reverse` in demo-boot, `faint` in meta/learn) | `ui.StatusBar` | low priority; not player-facing |

### 3.3 Order

```
step 0  one owner      internal/ui + internal/ui/text, lifted from boot/style.go,
                       with golden tests (half a day; additive contract, like W1)
step 1  in parallel    D1 boot · D2 meta + reader · G learn · E hud + session · engine
step 2  integrator     app: problem pane composition (C8), settings, pager zoom
```

Fixes 1–3 below do not need step 0. If the UI packages are still busy, land
them in place first and fold them into step 1 later.

---

## 4. Top 10 visual fixes, by impact over effort

| # | Fix | Impact | Effort | Where |
|---|---|---|---|---|
| 1 | Reader: replace Glamour "dark" with a token-built ANSI style (body = terminal fg) | **High**: all reading text is 1.5:1 on light themes; the H1 block is off-palette | S | `reader.go:251` |
| 2 | One HP bar per pane (C6) | **High**: the hottest pane shows the same result green and red | S | `hud/command.go:181-230`, `hudbar.go:135` |
| 3 | Toned pane-border timer and no blink (C7) | **High**: the border is the timer seen while typing; blink pulls the eye for 2 minutes | S | `panes.go:136`, `hud.go:353`, `hudbar.go:24-29` |
| 4 | Glyph collisions: `*1300` difficulty, ◆/◇ caps only, ↺ regression vs ↩ retreat, ▸ cursor only (› crumbs, ▹ folds) | Med-high: each glyph means one thing | S | `doors.go`, `route.go`, `bootmap.go`, `learn/module.go`, `learn/browser.go` |
| 5 | HUD pane width floor (36–48 cols) and compact two-row help | Med-high at 80 columns (E12) | S-M | `session/tmux.go`, `app/problem.go` |
| 6 | No bold on green, yellow or cyan; no ANSI 8 | Med: light themes, bold-bright terminals, Solarized Dark | S | `style.go`, `meta.go:200`, `learn.go:171`, `hud.go` |
| 7 | Remove color-only encodings: top's clears/attempts bar; timer bold/reverse cues | Med: color-blind and mono safety | S | `top.go:316`, `hudbar.go` |
| 8 | Door cards: bottom-aligned stats, no repeated site name, one "tags hidden", spaced compact glyphs | Med: the core choice screen | S | `doors.go`, `env.go:243` |
| 9 | One byte format, one clock format (`41B`/`42 B`, `12.5 KiB`/`12 KiB`, `8:41`/`09:20`) | Med: trust in the numbers | S-M | `engine.go:735-753`, `meta.go:103-133`, `hud.go:589` |
| 10 | `internal/ui` tokens and components; footers unified (C2, C2p), crash keys on the last row | High long-term; removes six palettes and four formatters | M | §3 |

Next tier: crash bottom-anchoring with a fuller log tail (E16), a green
`[  OK  ]` and a framed cleared banner (E17), an optional 600 ms `ACCEPTED`
beat on AC that any key skips (E17; taste), the pager zoom for statements
(E14), separators and plurals (E18).

## 5. Decisions for the owner

These need your call. Code cannot settle them, and each recommendation
below is the default if you don't pick.

1. **Light backgrounds.** Do you ever play on a light terminal theme?
   Recommended: default `ui.palette = ansi` with rules R1–R6, plus
   `adaptive` as an opt-in setting. If you play on light themes, make
   `adaptive` the default. Fix 1 is needed either way.
2. **Difficulty glyph.** Recommended: `*1300` (Codeforces' own tag notation),
   keeping ◆/◇ for capabilities as DESIGN.md draws them. Alternative: keep
   `◆ 1300` and move capabilities to ■/□, which changes the DESIGN.md learn
   mock-up.
3. **Rank colors in the boot.** Recommended: CF rank colors on every rating
   numeral, door cards included. Alternative: plain numerals everywhere,
   with rank color removed from git log, top and RTFM.
