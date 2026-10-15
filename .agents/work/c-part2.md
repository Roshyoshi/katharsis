# Phase C part 2: migration checklist

Spec: docs/design/ui-system.md §2 (system), §3.2 (per package), §3.3 step 1
and 2, §4 (fixes), with D26 (ANSI 16, dark only: no `adaptive`) and D27
(`◆1300` in the rank ramp everywhere, capabilities `■`/`□`) in place of §5.

Line numbers are at fa91db5b, which Part 1 does not touch for these
packages. Other lanes edit most of these files (table below), so after the
rebase re-run the survey and trust the function names over the numbers:

    cd .agents/ws/c-visual && grep -n 's\(Bold\|Faint\|Italic\|Red\|Green\|Yellow\|Magenta\|Cyan\)\|frame[A-Z]\|lipgloss\.\|hint(\|hearts(\|[▸◆◇◌☠⚔↺↩●○⏱✓✗★▓░]' \
      internal/screens/boot/*.go internal/screens/meta/*.go internal/hud/*.go internal/app/*.go | grep -v _test

Goldens are ANSI-stripped (boot_test.go:204, meta_test.go:54), so they show
text and layout only. A color-only change shows no golden diff; prove it
with a capture (end of file).

## Lanes that touch Part 2 files (snapshots of 10-14)

| Lane | Part 2 files it edits | Consequence |
|---|---|---|
| a2-economy | boot doors.go (shuf key, `Mastered` hiddenMod in sFaint), env.go, hudbar.go, merge.go (spend section, +2 rows), crash.go; merge goldens; app boot.go, views.go, learn.go; game | migrate boot after it lands; new sites to convert: `hiddenMod` Mastered, the spend section, the `s shuf (n)` footer pair |
| r1-robust | app app.go, boot.go, problem.go (`editorErr` red lines, `overlayBanner` reverse+red), runner.go, intents.go; session panes.go (TagPane, SweepOrphans), tmux.go (ExecInTmux); hud sweep.go | no overlap with Border.Set or splitPane bodies; `overlayBanner` and `editorErr` need Danger/Alert |
| audio | meta grub.go (+1 line), grub goldens; app app.go, boot.go, intents.go | do grub after it lands |
| main (GCC) | hud command.go, hud.go (profile wiring), app runner.go | different functions; no visual overlap |
| s9-story | game only | none |

The c-visual base is 4 commits behind the main tip (retention doc, GCC
flake, judge, docs). Only DESIGN.md overlaps, in disjoint hunks (trunk
@@ -201, Part 1 @@ -138): the rebase is clean.

## Order

Each step is one commit, gated (gofmt, vet, golangci-lint, go test), with
goldens updated only where listed.

### P0 ui additions (internal/ui, nobody else edits it)

Gaps the survey found; everything else Part 2 needs already exists.

- `Voice` token: italic, the flavor voice (§2.6). Replaces `sItalic` at
  boot crash.go:417, doors.go:334, route.go:156 and :177, and the italic of
  doors.go:237 ("tags hidden": `Voice.Render(Muted.Render(..))`).
- `FrameKind.Render(s)`: the frame's own paint, for the `▸ N` title that
  must match its edge. Replaces `f.paint.Render` at doors.go:155 and the
  hand-copied `sCyan.Bold(true)` at route.go:104, upgrade.go:58.
- `Alert` token: Danger + reverse, the look `timerCrit` already has
  (tokens.go). For r1's `overlayBanner` and the `:q!` confirm row.
- Catalogue golden gains the three rows; TestOneMeaning/TestGlyphWidths
  unchanged.

### P1 boot: style.go onto ui, golden-neutral (D1 lane)

Proof step: style.go's helpers become one-line forwards, and every boot
golden stays byte-identical. That shows the lift in Part 1 was faithful
before any visual change lands.

- style.go:15-32 palette and s* styles: keep the names for now, defined
  from tokens (`sFaint = ui.Muted.Style()` ...). Not golden-neutral in
  color for sCyan/sYellow semantics; that is P2's job, so P1 maps
  one-to-one (sRed→Danger, sGreen→OK, sYellow→Warn, sMagenta→Special,
  sCyan→Focus, sBold→Strong, sFaint→Muted, sItalic→Voice).
- style.go:37-51 frame vars → `ui.FrameKind`; style.go:57-81 `box` →
  `ui.Frame`.
- style.go:84-207 fit, clip, spread, center, wrapN, joinH, padLines,
  layout → ui.Fit, Clip, Spread, Center, WrapN, JoinH, PadLines, Layout.
  Behavior deltas, all invisible after Layout clips: ui.Center clips,
  ui.Frame skips an empty fill render.
- style.go:209-214 hearts → `ui.Hearts`.
- Keep boot-local (layout policy, not system): indent, centerBlock,
  maxWidth, centered, concat, fmtMult (spell it with `ui.G.Times`).
- `hint` (style.go:175) stays until P2, because ui.Footer needs a width.

Expected golden diff: none.

### P2 boot per screen, visual fixes (one commit per file group)

| Site | Change | ui API | Golden diff (text) |
|---|---|---|---|
| hudbar.go:15-48 tone, toneStyle, timerTone | delete; countdown via ui.Timer (no blink; bold at low, reverse at crit) | `ui.Timer(left, door.Timer)` | none |
| hudbar.go:68, 73-76 | `⏱ --:--` Muted; ducks `G.Duck`, cursed `G.Duck+G.Fail` Muted | G | none |
| hudbar.go:77-80 | uncommitted bytes fg (was green); `+N B` reward OK (was faint) | OK | none |
| hudbar.go:86-99 mods | `ui.Modifier(ModBoss, label)`, ModCursed "cursed: 1 WA = crash", ModMystery, ModBlind (Muted, was cyan), ModRegression (glyph only) | Modifier | none |
| hudbar.go:115-121 tags | tags fg (was cyan), "tags " label Muted | Muted | none |
| hudbar.go:127, 134-164 hpBar | HUDBar stops drawing the HP row: the embedded HUD draws the one bar (C6, fix 2). Headless pane draws it in app (A1) | `ui.HP` | hudbar.golden: last row of every case goes |
| doors.go:248-252 doorStats | `ui.Rating(door.Rating)`, ⏱ fg, `+N B` OK without bold (R3) | Rating | doors_*: `◆ 1300` → `◆1300` |
| doors.go:172-186 tag | `ui.Modifier(..)`; blind Muted | Modifier | none |
| doors.go:141-166 card | frame kinds; `▸ N` via `FrameKind.Render`; boss label `ui.Modifier(ModBoss, "BOSS")` without bold; source `Muted(G.Sep+src)` | Frame, FrameKind.Render | none |
| doors.go:192-225 body | pad the side-by-side card **above** the stats row, so stats rows line up (E13); the wide body's mod line drops "tags hidden" (tag slot already says it): `◌ blind · ×1.5`, `? mystery · ×2` | — | doors_140x40, doors_fork_140x40: blank rows move above the stats row; mod lines shorten |
| doors.go:227-246 titleStyle, tagLines | mystery title Special without bold; tags fg (was faint) | Special, Voice | none |
| doors.go:270-313 hiddenMod, modifiers | `mod.style` becomes `ui.Mod`; regression paints the glyph only (R2) | Modifier | none |
| doors.go:315-323 daysAgo | `units.Plural(n, "day") + " ago"` | units | none |
| doors.go:327-349 bossBody | name Special (R3), blurb Voice | Voice | none |
| doors.go:355-385 compact | glyphs from `ui.Mod.Glyph()`/`Token()`, joined by spaces (`☠ ?`, §2.4 spacing rule); cursor `G.Cursor` Focus | Mod | only goldens that reach the compact tier (none today) |
| doors.go:64, 72, 85-93 | footers → `ui.Footer(width, ...)`; boss `where` via Modifier | Footer | none at 80/140 |
| env.go:242-255 sourceLabel | drop the repeated site name: `Codeforces · Codeforces Round 690 (Div. 3) · A` → `CF Round 690 (Div. 3) · A` (C3); add a unit test (fixtures are all USACO) | — | none |
| env.go:274-300 header (C1) | `Boot #N` Strong, seps `Muted(G.Sep)`, uncommitted fg (was green), `G.Duck`, rule `ui.Rule(width)` | Hearts, Rule | none |
| route.go:99-107 card | Frame + `FrameKind.Render` | Frame | none |
| route.go:117-147 describe | rooms `units.Plural`; band `ui.RatingBand(mid-100, mid+100)`; "your dp rating" numeral `ui.Rank(own)` (a player rating is a bare numeral) | RatingBand, Rank | route*_: `◆ 900–1100` → `◆900–1100` |
| route.go:149-194 | blurb Voice; compact cursor Focus | Voice | none |
| bootmap.go:27-39 glyph | here `G.Here` Focus (was bold yellow); crash Danger; done OK; boss Special; ahead Muted | G | none |
| bootmap.go:89-131 boxMap | links `G.Rule`; `FrameHere` is now Focus, thin | Frame | none |
| bootmap.go:154 | crumb `Muted(G.Crumb)` | G | none |
| bootmap.go:249-275 history table | rating cell rank-colored, right-aligned by cells (`ui.AlignRight`; `%*s` pads bytes and breaks with SGR); header stays `◆`, cells stay bare numerals (default, see user calls) | Rank, AlignRight | none |
| bootmap.go:278-296 outcomeLabel | `✓ AC`: glyph OK, word fg (R2); retreat `↺` → `↩` (`G.Retreat`, fix 4); open `G.Here` Focus | G | map_*, map_crashed_*, doors_map_*: `↺ retreat` → `↩ retreat` |
| merge.go:138-183 (after A2) | commit bytes fg (was bold green); drop the inline `c commit` (C2: it is in the footer); leave row cursor `G.Cursor` Focus + Strong "leave"; spend section from A2 onto tokens | Footer | merge_*: `  c commit` leaves row "commit" |
| upgrade.go:35-95 | cards via Frame; list rows C4 (selected name Strong, not bold cyan) | Frame | none |
| upgrade.go:167-194 | `✓ AC` glyph OK + Strong title; `-O3` OK without bold (it is a reward; default, R3) | OK | none |
| cleared.go:83-86 | `✓ Boot #N cleared`: glyph OK, words Strong (R2, R3) | OK, Strong | cleared_*: `Boot #42 cleared` → `✓ Boot #42 cleared` |
| cleared.go:89-101 | `units.Plural` for doors and retreats | units | none |
| cleared.go:105, 118-120 | committed OK without bold; `★` Warn glyph, Strong words | Warn | none |
| cleared.go:130 | footer left-aligned (C2) | Footer | cleared_*: footer row loses its centering |
| crash.go:123, 144, 191, 200, 235, 429-461 | dline.style → `ui.Token`; panic line Danger bold (allowed); quote Voice; core dump bytes OK without bold; `[u] upsolve` brackets stay (console idiom, C2p) | Token | none |

### P3 boot: delete the forwards (mechanical)

Remove the s* names, `hint`, `box` and the helper forwards from style.go;
call ui directly. Expected golden diff: none.

### P4 meta (D2 lane; grub after audio lands)

| Site | Change | ui API | Golden diff |
|---|---|---|---|
| meta.go:157-170 styles | tokens; `gray` (ANSI 8, R4) → Muted; `field` → `ui.Field` | tokens | none |
| meta.go:174-222 frame, pad, padLeft, center, spread | → `ui.Layout(w, h, lines, "")`, Fit, AlignRight, Center, Spread. Spread now drops the **right** half (§2.6) where meta's cut the left; at 80 and 140 no fixture line overflows | Layout, Fit, AlignRight, Spread | none |
| meta.go:224-241 ratingStyle | → `ui.Rank(r)`; chart.go:19, 90 take `func(float64) lipgloss.Style` = `ui.Rank(r).Style()` | Rank | none |
| meta.go:243-251 keyBar | keep (htop parody, R8); `field` stays top-only | Field | none |
| log.go:126-131 | filter name Focus; keys → `ui.Footer(w, "j/k","move","enter","expand","f","filter","g/G","ends","q","back")` | Footer | none |
| log.go:175 | selection `ui.Select` | Select | none |
| log.go:191-198, 231-243 | star glyph carries the outcome color; words "cleared"/"abandoned" fg (R2); "crashed" Danger words (allowed); hash keeps git's yellow (parody exception, R8) | OK, Warn, Danger | none |
| log.go:206-208 | `+N B` OK (gain) | OK | none |
| log.go:291-299 attemptLine marks | open `…` → `G.Here` Focus; ✓/✗/↩ via G | G | none in fixtures |
| log.go:301-303 | door difficulty `ui.Rating(a.Rating)` (D27) | Rating | log_expanded_*: `simulation 1150` → `simulation ◆1150` |
| log.go:312-315, 335 | `%d✗` → `G.Fail`; `🦆%d` → `G.Duck` | G | none |
| top.go:139 | drop `.Bold(true)` on the rank color (R3) | Rank | none |
| top.go:295 | player rating, bare numeral `ui.Rank` | Rank | none |
| top.go:298 | selection `ui.Field` | Field | none |
| top.go:316-318 clears bar | R1: clears `\|` OK, fails `#` Danger, so the split survives without color and on the selected row. Precedent: htop's monochrome meter draws a second segment with `#` (BarMeterMode_characters `"\|#*@$%&."`, Meter.c; from memory) | OK, Danger | top_*: the fail part of each `[\|\|\| ]` becomes `#` |
| top.go:245 head | htop's green header row: keep (parody) | — | none |
| grub.go:319 | one cursor: reverse the cell under it, drop the extra `█` | Select | grub_edit_*: `8f3a█` → `8f3a ` (reversed space) |
| grub.go:465 | `ui.Select` | Select | none |

### P5 hud + session (E lane)

| Site | Change | ui API | Test or golden diff |
|---|---|---|---|
| hud.go:385-401 styles, verdictStyle | tokens; `ui.Verdict(string(v))` | Verdict | none |
| command.go:201-209 hpBar | delete | — | TestHPBar (command_test.go:101) goes; ui_test covers HP |
| command.go:211-250 submitLines | `ui.HP(w, sum.Passed, sum.Total, note)`; note = `judging d/t…` Muted, or the verdict token + `#name` + reason, plus `· best b/N` when the best submit beats this one (C6, fix 2) | HP, Verdict | command_test.go:136 `submit ▓` → `HP ▓` |
| hud.go Options (:54-75) | add `Best func() (passed, total int)` (the engine's BestPass, so a resumed door keeps it) and `BorderClock func() string` (a tmux format) | — | — |
| hud.go:341-355 updateBorder | border gets a tmux format: `BorderClock()`, or `G.Timer`-less practice clock, then `ui.TmuxEscape` on the samples text and `Verdict(v).Tmux(..)` on the HP verdict | Token.Tmux, TmuxEscape | command_test.go:331, :339 expect the format |
| hud.go:357-383 statusText | stays plain: it is also `ctl status` (command.go:95) | — | command_test.go:161 holds |
| hud.go:416-426 header | practice clock `up 12:34` (elapsed has no ⏱, §2.4); `ui.Spread(Strong(name), clock, w)` | Spread | hud_test.go:143, command_test.go:321 |
| hud.go:410, 429, 437-446, 464-471 | `ui.Rule`, Warn closing line, Danger errors, Muted notes | tokens | none |
| hud.go:493 footer | `ui.Footer(w, "w","rejudge",":Submit","judge",":Debug n","debug","r","rerun","j/k","move","q","quit")` | Footer | hud_test.go:212, :280 hold (`q quit`) |
| hud.go:511-537 statusLine | `✓ ALL PASS`: glyph OK, words Strong; C8 order `samples n/m · profile · built 1.2s` | OK, Strong | hud_test.go:60, :277 hold (Contains) |
| hud.go:539-555 rowLine | cursor `G.Cursor` Focus; pending `·` Muted | G | hud_test.go:76 `▸ 2` holds |
| session/panes.go:132-138 Border.Set | stop escaping: the text is a tmux format and its writer escapes user text (`ui.TmuxEscape`, `Token.Tmux`). Verified 10-14 in nested `-L` servers: `#[fg=colour1,bold]` inside `@katharsis_border` draws styled, `##` draws `#`; today's blanket escape would print the directives literally | — | panes_test.go:239-243 passes a pre-escaped format |
| cmd/katharsis/main.go:222 | practice border closure unchanged (the hud now hands it a format) | — | — |

### P6 app, integrator (§3.3 step 2)

| Site | Change | ui API | Test diff |
|---|---|---|---|
| app.go:330-345 Clock | add `BorderClock()`: `ui.BorderTimer(left, door.Timer) + " · " + ui.BorderHearts(..) + " · " + TmuxEscape(ducks)`; soft-timer overrun `+mm:ss over` as `ui.Danger.Tmux` (fix 3, E15) | BorderTimer, BorderHearts | — |
| runner.go:131-144, cmd/katharsis/game.go:143 | wire `BorderClock` and `Best` into hud.Options | — | — |
| problem.go:149-154 styles | tokens | — | — |
| problem.go:156-203 View (C8) | vitals (HUDBar, no HP) + `ui.Rule(w)`, then the HUD (HP first), then two footer rows. ≥52 cols: vim row `ui.Footer(w, ":w","samples",":Submit","judge",":Debug n","debug",":Quack","duck",":Retreat","retreat",":q!","give up")`; <52: bare names, Muted, and the pane row drops `Q :q!` | Footer, Rule | app_test.go:962 (`vim ` / `s submit` prefixes) |
| problem.go:165-167 confirm | replaces the pane-keys row instead of pushing the pane down; `ui.Alert` | Alert | — |
| problem.go:168-170 (+ r1 editorErr, overlayBanner) | Danger; banner `ui.Alert` | Danger, Alert | — |
| problem.go:206-226 headless | the one HP bar when no HUD runs: `ui.HP(w, att.BestPass, att.Total, verdict)` | HP | — |
| problem.go:314-320 overlayFlash | `ui.Select`, `ui.Clip` | Select | — |
| learn.go:191-213 linkScreen | tokens; cursor `G.Cursor`; footer `ui.Footer(w, "space","toggle","enter","link","esc","none")`; `[x]`/`[ ]` stay (a checkbox, not a capability) | Footer | none |
| cmd/katharsis main.go:137, game.go:140 (before `tea.NewProgram`) | `ui.SetGlyphs(ui.AutoGlyphs(os.Getenv))`; `ui.SetPalette(ui.Mono)` when `NO_COLOR` is set (Mono is for what Bubble Tea does not strip: the tmux border). No `adaptive` (D26), no new flag | SetGlyphs, SetPalette | — |
| session/tmux.go:23-40, 53-80 SplitEditor (C8, fix 5) | HUD gets `max(36, min(48, 35% of window width))` columns, absolute: split, then `resize-pane -t $TMUX_PANE -x <cols>`. `-split` (game.go:33, main.go:75, default 65) becomes an explicit override; new default 0 = the C8 rule | — | session tests |
| pager zoom (§2.6, E14) | reader opened from a pane under 60 cols: `resize-pane -Z` on push (intents.go:55), unzoom on pop; new `session.Zoom(pane, on)` checking `#{window_zoomed_flag}` | — | — |

### P7 next tier (only if cheap; else hand to D)

- Crash bottom-anchored, keys on the last row, free rows filled with log
  tail (E16): crash.go:105-118 `layout(.., "")` → anchor to the bottom.
  Every crash_* golden changes.
- Cleared banner in a double frame (E17): cleared.go:78-84.

## Gate and proof

    cd .agents/ws/c-visual && jj st && \
    nix develop path:. -c sh -c 'gofmt -l .; go vet ./... && golangci-lint run ./... && go test ./...' 2>&1 | grep -v '^warning: Git tree'

Judge TLE timing flakes under load; rerun `go test -count=1 ./internal/judge`
alone. Show each golden diff with `jj diff --git <golden>`.

Color proof (goldens strip SGR): build the demo binary before and after
(`jj new <base>` → `go build -o /tmp/<x>-before ./cmd/demo-boot` → back),
then capture in an isolated server and tag the SGR codes:

    S=katharsis-cv-$$; tmux -L $S -f /dev/null new-session -d -s s -x 80 -y 24 "env TERM=xterm-256color /tmp/x-after"
    # wait for the screen, send keys, then:
    tmux -L $S capture-pane -p -e -t s | sed -E $'s/\x1b\\[([0-9;:]*)m/«\\1»/g'
    tmux -L $S kill-server

The tmux border is not in capture-pane: run the game's tmux client inside
a second isolated server's pane and capture that pane (the border check
above was done this way).

## User calls (defaults apply if unanswered)

1. The optional 600 ms `ACCEPTED` beat on AC (§4 next tier, marked taste):
   default, leave it to the S9 animations lane, not Part 2.
2. Door-history table: rank-colored bare numerals under a `◆` header
   (default; no width churn), or `◆1300` in every cell.
