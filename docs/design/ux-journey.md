# katharsis UX: player-journey evaluation

Status: findings, 2026-10-14. Built from the working copy at 21:03 that day.
Line numbers drift while other agents edit; each cite names the function too.
Owner of this lens: the journey (what the player knows, feels and decides,
moment to moment). Palette, typography and components belong to
`docs/design/ui-system.md`; findings here touch layout only where the journey
needs it.

## The short version

The moment-to-moment game is already good. The doors screen shows the
risk/reward trade, the crash screen carries the theme, `r` starts another boot
in one key, and the vim integration copes with `:wq`, `:q!` and unsaved
buffers. The loop breaks at the *long* timescales:

1. **Progress is invisible.** I beat the boss and nothing unlocked, with no
   reason given. My rating rose 108 and no screen said so.
2. **Bytes buy nothing.** Commit is always the right move and the bank is
   only a score.
3. **The game spends my lives without asking.** A failing-sample `:Submit` and
   a single `R` each cost a heart with no check and no message.
4. **State changes have no beat.** Lives, ducks, kexec, regressions and
   biome-ending ACs change silently.
5. **Inside a door, the statement and the judge share one pane**, and only
   one shows at a time.

## Method

I played the real binary in an isolated tmux server (`-L katharsis-ux-<pid>`,
`-f /dev/null`), with `XDG_DATA_HOME` and `--root` under `/tmp/ux-r`, vim
`-u NONE -N` and `--quiet`. I drove it with `send-keys` and read it with
`capture-pane`. I played 8 boots, about 70 minutes in all:

| Boot | Preset | What I did | End |
|---|---|---|---|
| 1 | standard | first launch on an empty save; CE, WA, 5 ACs, duck, kexec retreat, `:Debug`, boss | cleared, 10:35 |
| 2 | tiny (2-min biomes) | AC, then let the timer run out with 37 B uncommitted; upsolve | crash (timer), dump recovered |
| 3 | tiny | `:q!` on door 1 | crash (`:q!`) |
| 4 | tiny | cursed door, one wrong submit | crash (out of lives) |
| 5 | tiny | unsaved buffer left at timeout; 90x30, then 120x36 | crash (timer) |
| 6 | tiny | killed the server mid-door, relaunched | resumed, then crash |
| 7 | tiny | "3 days later" (due dates moved back in a copied DB); regression door; `:q!` on the boss with 183 B uncommitted | crash, dump left |
| 8 | tiny | HUD `R` without kexec, `Q`, `:wq` | dump from boot 7 overwritten |

I tested RTFM through `cmd/demo-learn` on the real guide snapshot; GRUB's
entry says "RTFM (learn mode) is not wired up yet". Trials are not wired.

Limits of this evidence:

- I am a proxy player who knew most 800-rated problems, so doors took 1–2
  minutes. Real struggle under a 15–50 min timer is underrepresented.
- Sound was off.
- I did not test the first-launch download (about 150 MB), because the
  problem cache is shared with real use. The pre-TUI stderr line is from a
  code read: `./cmd/katharsis/game.go`, `runGame`.
- The tiny preset speeds up the rating moves in J10.

Each finding says how far it is proven: **ran it** (reproduced in the running
game), **code** (read in source), or **inference**.

Severity is measured against D1, loop feel:

| Severity | Meaning |
|---|---|
| blocker | breaks a promise of the loop or misleads the player about progress |
| major | spends the player's life, time or learning without their choice, or removes feedback the loop needs |
| minor | friction or confusion with a workaround |
| polish | noticeable, cheap, low stakes |

## Keep these: what already feels great

- **Door cards make risk readable.** Rating, timer, bytes and `≈76% clear` sit
  on one line, and each modifier says what it does on the card
  (`☠ no ducks · 1 WA = crash · ×2`).
- **The boss card is an event:** a double frame, `H E I S E N B U G`, a flavor
  line, and +407 B against 40 B doors.
- **Route cards** carry flavor, the range and *your* per-topic rating
  (`your simulation rating 714`). This is the best "choose your build" moment
  in the game.
- **The crash screen is specific to the death**:
  `Attempted to kill init! exitcode=0x00000009` for `:q!`,
  `Cursed door: Killed process 1 (you) verdict:WA passed:7/28 submits:1`, and
  `Tainted: B (blind) C (cursed)`. Death reads as a story, which is the Dead
  Cells feel.
- **One key from death to the next boot.** `r` on the crash screen starts
  boot N+1 on the same preset.
- **Sample diffs are exact**: `#1 WA line 3 / want NO / got YES`.
- **The timer on the vim border** (`⏱ 14:41 · ♥♥♥ · 🦆1 · samples 0/1`)
  works without touching the player's status bar.
- **vim reflexes are safe.** `:q!` asks first (the cnoreabbrev), `:wq` gives
  `editor closed · e reopens it`, and unsaved edits are written at timeout.
  Ran it: the `// UNSAVED WORK LINE` I left in the buffer was on disk after
  the crash.
- **The fetch time is not charged**: `fetching door 1… its timer starts once
  it opens`.
- **Resume is correct.** Killing tmux mid-door and relaunching reopens the
  door, with the deadline kept.
- **The core-dump loop works**: `core dump recovered: +37 B banked` after an
  untimed upsolve.
- **The git log expanded view** is a complete boot story: per-door time,
  submits, ducks, and independent/hinted/retreated.
- **The RTFM module page** states the trial and its reward:
  `pass → ◆ CAP_PREFIX: The Accumulator joins your route choices`.

## Findings, ranked

### J1 · blocker · Beating the boss unlocks nothing, and no screen says why

**Where.** The clear screen, then GRUB, after boot 1.

**Evidence (ran it).**

```
                Boot #1 cleared · Bronze
                      ⚔ Heisenbug
       uptime      10:35
       committed   570 B · bank 570 B
       solved      5 doors · 1 hinted · 1 retreat
       lives left  ♥♥♥♥♥
        r reboot at Bronze · l git log · q quit
```

Relaunching shows `set prestige=0█   # 0–0 unlocked` under GRUB `e`.

**Why (code).** `finish` unlocks only once `rated(m) >= CalibAfter`, which is
60 rated doors (`./internal/game/engine.go:684`, `./internal/rules/rules.go:368`).
That is about 12 standard boots, or roughly 24 h. The reason is written with
`logf` (`engine.go:688`), and `State.Log` is rendered only by the crash screen
(`./internal/screens/boot/crash.go:229`). DESIGN.md:29 promises that "beating
the final boss unlocks the next". The boss kill is the Boss Cell moment; when
it pays nothing and says nothing, the player loses the long-term goal.

**Fix.** Show the gate wherever the player looks for progress. Before → after:

```
 lives left  ♥♥♥♥♥                      lives left  ♥♥♥♥♥
                                        rating      1000 → 1108  ▲108
                                        Silver      🔒 6/60 rated doors ▓▓░░░░░░░░
                                                    (still calibrating you)
 r reboot at Bronze · l git log ·       r reboot · g GRUB · l git log · q poweroff
 q quit
```

```
GRUB, before                            GRUB, after
 *katharsis, prestige 0 (Bronze)         *katharsis, prestige 0 (Bronze)
  katharsis (nightly build …)             katharsis, prestige 1 (Silver)  🔒 54 doors
                                          katharsis (nightly build …)
```

Whether 60 is the right gate is your decision (see "Decisions only you hold").

### J2 · blocker · Bytes buy nothing, so commit is never a decision

**Where.** The merge window, the GRUB footer, the clear screen.

**Evidence (ran it, code).**

```
commit   83 B uncommitted, lost if you crash  c commit
   ↓ c
commit   nothing uncommitted · 83 B committed this boot · bank 83 B
```

`Meta.Bank` is commented "committed bytes available to spend"
(`./internal/game/types.go:78`), but outside display code it is only ever
added to: `bank()` and core-dump recovery (`./internal/game/meta.go:31`).
Commit has no downside, so it is a chore; skipping it is a misplay, never a
gamble. Leaving the merge window with 130 B uncommitted gave no warning, which
is consistent: nothing was at stake. The learning-science report reaches the
same finding (`docs/research/learning-science.md` §4 M2).

**Why.** In Dead Cells, cells at the Collector are the meta-progression and
the reason for one more run. Here bytes are the most-shown number on every
screen, and they mean nothing.

**Fix.** Turn the merge window into a real fork: spend now on this boot, or
commit to the bank for permanent unlocks.

```
before                                   after
commit  130 B uncommitted, lost if       bytes   130 B uncommitted · bank 607 B
        you crash  c commit                ▸ c  commit 130 B → bank      (safe, forever)
upgrade 2 to pick                            $  spend this boot:
                                                fsck   +1 ♥ now           60 B
                                                cron   +5:00 next door    40 B
                                                rr     re-roll next room  30 B
```

The bank then needs a sink at GRUB, for example an `apt` entry:
`apt install strace` puts the daemon in the offer pool for 400 B, and a third
lib slot costs 1 KiB. What the bank buys is your call (see "Decisions only
you hold").

### J3 · major · A failing-sample `:Submit` spends a life, or the whole boot on a cursed door, without a check

**Where.** In a door, vim `:Submit` or HUD `s`.

**Evidence (ran it).** On Matrix Rotation the samples were red, I submitted,
and a heart was gone:

```
#1 WA  line 3        want NO   got  YES          (samples 0/1)
  ↓ :Submit
⏱ 16:36  ♥♥♥♡♡  …   ▓▓▓▓░░░░░░░░░░░░░░░░ 6/30  1 submit · WA      (was ♥♥♥♥♡)
```

On the cursed door, with samples at 1/3, one submit ended the boot:

```
Cursed door: Killed process 1 (you) verdict:WA passed:7/28 submits:1
Kernel panic - not syncing: out of lives in "Make a triangle!"
```

**Code.** `command! -bar Submit` goes straight to `katharsis ctl submit`
(`./internal/session/overlay.go:161`); nothing checks the latest sample run.

**Why.** A submit that the visible samples already disprove is a misinput,
not a gamble. Dead Cells punishes mistakes in execution; it does not punish
pressing the wrong button.

**Fix.** Use the vim bang idiom. `:Submit` with failing samples echoes
`katharsis: samples 1/3 fail · :Submit! spends ♥ anyway` and does nothing;
on a cursed door it says `… :Submit! ends the boot on WA`. HUD `s` asks for
`s` again. CE stays free, as now (`FreeVerdicts`).

### J4 · major · One `R` in the HUD spends a life, with no confirmation and no message

**Where.** The HUD in a door, without kexec.

**Evidence (ran it).** One `R` changed the hearts from ♥♥♥♡♡ to ♥♥♡♡♡,
re-rolled all three doors, and printed nothing. `Q` needs a second press, `R`
does not (`./internal/app/problem.go:158` against `:162-168`). With kexec the
retreat was free, but nothing said a free swap had been used.

**Fix.** Make it `R R`, like `Q Q`, with
`retreat costs ♥ (2 left) · R again` or
`kexec: free swap (1 per biome) · R again`. Then log the result to the event
line (J5).

### J5 · major · State changes happen without a beat

**Where.** Every boot screen.

**Evidence (ran it).**

| Event | What the player saw |
|---|---|
| WA costs a life | the heart glyph changed; vim showed `katharsis: WA 6/30 · #1 line 3` (no mention of the life) |
| ECC +1 life | the hearts changed |
| ducks refilled at the merge window | 🦆1 became 🦆2 |
| kexec free swap used | nothing |
| AC on a biome's last room | went straight to the merge window; the only trace was `2 to pick (-O3 earned an extra)`. The `✓ AC … in 01:09 of 15:00` line the upgrade screen shows never appeared |
| regression cleared | went straight to the merge window, with no "remembered" moment and no next due date |

**Why.** Roguelike feel depends on feedback at every timescale. Each of these
changes the player's resources, and the player has to notice it by diffing
glyphs. The workfile already lists "WA feedback" for the integrator; this
widens it.

**Fix.** Add a `dmesg` strip of the last 1–3 events under the status line on
every boot screen, using the engine log the crash screen already renders. It
is never modal and it is themed:

```
Boot #1 · Bronze · merge window                                  uptime 02:46
♥♥♥♡♡  🦆1  83 B uncommitted · 0 B committed
[  166.2] door_2: AC "Matrix Rotation" 00:42 of 17:00 · -O3 · +41 B
[  166.2] merge window: ducks refilled 🦆0 → 🦆1
───────────────────────────────────────────────────────────────────────────────
```

Lines to emit:

- `WA 6/30 · −1 ♥ (3 left)`
- `ECC: ♥ 4/5`
- `kexec: free swap used (biome 2)`
- `↺ "Coder" remembered after 4 days · next check Oct 31`

### J6 · major · Clear and crash screens hide what changed

**Where.** The clear screen and the crash screen.

**Evidence (ran it).** No rating appears on either screen. It shows up only
in the GRUB footer after a relaunch: `rating 1000` became `rating 1108` after
the clear, then `886` after three crashes. The regression schedule is a raw
log line with an internal id:

```
[  200.328] regression: cf:384A due 2026-10-17
[  200.328] Kernel panic - not syncing: timer expired in "Coder"
```

**Why.** A boot costs up to 2 h. The end screen is the moment to show what
the time bought. The learning-science report makes the same point from
competence feedback (§4 M4).

**Fix.** Add a three-line summary under the quote. Before → after on the
crash screen:

```
core dumped: 37 B on "Coder" ·          core dumped: 37 B on "Coder" · upsolve it (untimed)
[u] upsolve [r] reboot [l] git log      to get them back; crash again first and it is gone
                                        ↺ "Coder" comes back as a regression on Oct 17
                                        rating 1108 → 1041 ▼67 · simulation 714 ▼40
                                        [u] upsolve  [e] editorial  [r] reboot  [g] GRUB
```

The clear-screen version is in J1.

### J7 · major · A core dump is overwritten silently

**Where.** The crash screen, then `r`, then the next crash.

**Evidence (ran it).** Boot 7 ended with
`core dumped: 183 B on "Karen and Morning"`. I pressed `r` and got no
warning. Boot 8's crash buried the loss in the kernel log:

```
[   70.463] coredump: 183B on cf:816A overwritten
```

**Code.** The overwrite is only `logf`'d (`./internal/game/engine.go:466`).

**Why.** The core dump is the one mechanic that pays loss aversion into
upsolving; the learning-science report says to keep it. Losing it silently
teaches nothing and feels like a bug.

**Fix.**

1. On the crash screen with a live dump, `r` asks:
   `reboot leaves 183 B at risk on "Karen and Morning" · r again · u upsolve first`.
2. During the next boot, the status line carries `☠ 183 B dump`.
3. When a crash overwrites a dump, the headline says
   `core dump lost: 183 B ("Karen and Morning")`.

(GRUB's due-upsolve list, which the meta-screens agent is building now, is
the fourth place it should appear.)

### J8 · major · The statement is hidden when a door opens, and reading it hides the judge

**Where.** The HUD pane in a door, at all sizes. At 90x30 the HUD is 31
columns wide.

**Evidence (ran it).** On open the HUD shows tags, the HP bar, the build line
and the sample table, but no statement. `t` replaces the entire HUD. My `:w`
while reading produced a CE that the pane never showed; only the vim border
changed, to `samples CE`.

```
 Make A Equal to B  ·  Codeforces Round 825 (Div. 2) · A · st…
  …
  #### Input
j/k scroll · space/b page · g/G ends · / search · o brow…  Top
```

At 90x30 the statement wraps at 27 characters (`Dora has a set s containing`)
while vim gets 58 columns. DESIGN.md:81-90 drew it the other way: a status
block on top and `── statement ──` below it.

**Why.** Reading the statement is the first minute of every door. Each
switch costs a pane hop (prefix + arrow) and a keystroke, and the player loses
sight of the judge.

**Fix.** Put the statement first and pin the status above it. Fold the
failure detail into one line, and expand it on `enter`.

```
before (62 cols)                         after (62 cols)
⏱ 13:56  ♥♥♥♡♡  🦆1  0 B +42 B          ⏱ 13:56  ♥♥♥♡♡  🦆1  0 B +42 B
tags brute force · greedy · sortings     S1 ✓ 77ms  S2 ✗ line 3 want NO got YES
░░░░░░░░░░░░░░░░ not submitted           ░░░░░░░░░░░░░░░░ not submitted
asan+ubsan (apple) · built 0.8s · 1/1    ─ Make A Equal to B · 800 · bf·greedy ─
──────────────────────────────           You are given two arrays a and b of n
▸ 1   AC      77ms    16M                elements, each element is either 0 or 1.
──────────────────────────────           …
#1 AC                                    (statement scrolls; j/k · enter = detail)
cpu 77ms · wall 125ms · peak 16M         s submit · ? duck · R retreat · h help
vim  :w samples · :Submit · …
s submit · t statement · ? duck · …
```

For narrow terminals, add `:Statement`, which opens the `statement.md`
already written in the door dir in a vim split, so the player reads in their
own vim with search.

### J9 · major · An unfetched door takes 34–75 s to open, with a static line and no way out

**Where.** The doors screen after `enter`.

**Evidence (ran it).** `fetching door 3… its timer starts once it opens`
stayed up for about 75 s. Other doors took 48 s, 34 s, and 1–14 s. The fetch
is a single 5-minute-timeout call with no progress
(`./internal/app/boot.go:387-390`). Doors the prefetch had already reached
opened in 1–4 s, so the cost lands only when the player chooses faster than
the prefetch.

**Why.** Fast choices are the ones that get punished. A minute of dead air
between "I pick this" and the editor kills momentum.

**Fix.** Show readiness on each card, show progress on the line, let `esc`
cancel, and let `enter` switch to a ready door.

```
┏━ ▸ 3 ━━━━━━━━━━━━━━━━━━━━━━━━━━━┓  ╭─ 1 ──────────────────── ● ready ─╮
┃ Dawid and Bags of Candies   ⤓41% ┃  │ Extremely Round                  │
 fetching door 3… 38 MB / 92 MB · 0:21 · timer not running · esc cancel · 1 is ready
```

### J10 · major · A few fast crashes make Userland read as hard

**Where.** The doors screen, boots 4–8.

**Evidence (ran it).** The rating went 1000, peaked at 1123, then fell to 886
and then about 820 over five quick failures. Userland, which aims at 90%,
then showed:

```
◆ 800  ⏱ 02:00  +280 B   ≈46% clear        (boot 7, room 1)
◆ 1000 ⏱ 02:00  +94 B    ≈34% clear        (boot 5, room 1)
```

The tiny preset sped this up (2-minute timers, and a `:q!` after 6 s still
counts as rated), but the mechanism is the one the learning-science report
measures: the target falls below the 800 pool floor (§1 P1.2), and a crash
streak drives churn (P1.3).

**Why.** The "cheap crashes" biome turning red after two deaths is a death
spiral the player can see.

**Fix.** This belongs to the engine owner and follows the science report:
buy back the clear chance with lives or time when the aim is below the floor.
The journey half is to *say so* on the room:
`Userland is at the content floor · +1 ♥ this biome`.

### J11 · major · The duck's second quack repeats the biome's name

**Where.** Brute Force Bunker (topic `complete-search`), Dawid and Bags of
Candies.

**Evidence (ran it).**
`katharsis: technique: complete-search · 39:36 left · ♥♥♥♥ · 🦆0`. The
engine skips level 1 only when the tags are visible
(`./internal/game/engine.go:298`). In a topic biome the route has already
named the technique.

**Fix.** When the technique equals the biome topic, level 2 reveals the
editorial's key observation, which is the "third duck level" of
learning-science §5. Also stop ducks stacking past their use: I held 🦆6 at
a boss with a cap of 2 hints per door (J18).

### J12 · minor · There is no way back to GRUB after a boot, and `q` quits the program

**Where.** The clear and crash screens.

**Evidence (ran it, code).** `q` on the clear screen ended the program and,
with it, the tmux session. The crash screen also takes `q`
(`./internal/screens/boot/crash.go:97`), but its footer does not list it.
`screens.Quit` exits (`./internal/app/intents.go:24`). To reach learn mode,
`top` or another prestige, the player has to quit and relaunch. A live boot
always resumes, so GRUB is unreachable until the boot ends.

**Fix.** Add `g GRUB` to both footers and make `q` go to GRUB, leaving
`poweroff` as the only exit. Keep `r` as the one-key reboot.

### J13 · minor · The template is judged on open and shows a red WA before the first keystroke

**Evidence (ran it, every door).** `▸ 1   WA      82ms    15M  line 1` with
`want 1 / got <EOF>`, then `samples 0/1` on the border, before I typed
anything.

**Fix.** Until the first save after open, show `S1 · not run · :w judges`.

### J14 · minor · The vim border, where the player is looking, never changes color as time runs out

**Evidence (ran it).** Under 30 s the HUD timer is bold, blinking red
(`ESC[1;5m ESC[31m ⏱ 00:29`). The vim border stayed plain
(`⏱ 00:28 · ♥♥♥♥ · 🦆2 · samples 0/1`).

**Fix.** Set a window-scoped `pane-border-style` (it does not touch the
player's status bar): yellow under 5 min, red under 1 min.

### J15 · minor · "≈% clear" ignores the curse and the timer

**Evidence (ran it).** Cursed doors showed the same `≈87% clear` as plain
doors in the same room, although `1 WA = crash`. A 2-minute door showed
`≈86%`.

**Fix.** Fold the curse into the estimate (it is the probability of no WA),
or show a second number: `≈87% · ☠ ≈60% first-try`.

### J16 · minor · A crash on a door with no editorial is a dead end

**Evidence (ran it).** In the upsolve HUD, `e` answered
` cf:384A has no editorial`: a raw id, after the footer offered the key. About
40% of Codeforces doors have no editorial (DESIGN.md "Content").

**Fix.** Mark it on the card (`📖✗`), hide `e` when there is none (the crash
screen already checks `hasEditorial`; the upsolve HUD does not), and fall
back to free tags plus the technique after a crash.

### J17 · minor · The HUD key legend is cut off at every width

**Evidence (ran it).** `s submit · t statement · ? duck · R retreat · Q :q! ·
r rerun…` at 180 columns; at 90 columns it ends at `? duc…`. `e` (reopen the
editor) and `j/k` never show.

**Fix.** Show the three keys that matter (`s submit · ? duck · h help`) and
put the rest in an `h` overlay.

### J18 · minor · Upgrade picks are low-information

**Evidence (ran it).**

- Every offer was the same three cards: Overclock, Rubber Duck, ECC.
- After a pick, the list reshuffles under the cursor (ECC moved from slot 2
  to slot 1), which invites a wrong second `enter`.
- Held upgrades are listed nowhere, while daemons are (`daemons kexec`).
- Ducks reached 🦆6 with a cap of 2 hints per door.

**Fix.**

- Keep the order stable after a pick.
- Show `ECC×2 OC×1` beside the daemons.
- When ducks are at least 2, replace the Duck card with a variant
  (`Rubber Duck+: a third quack shows the key observation`).

### J19 · minor · git log cannot act on a crash, and drops recovered bytes

**Evidence (ran it).** Boot 2's row shows `0 B`, though I recovered its 37 B
dump. Expanded attempts are read-only.

**Fix.** In the expanded view, `u` upsolves and `e` opens the editorial for
the attempt under the cursor. Count recovered bytes on the boot that dumped
them: `0 B (+37 B recovered)`.

### J20 · minor · top shows the weakest topic but not what to do about it

**Evidence (ran it).** `simulation  3  1  33%  729` is the weakest topic,
with no link onward.

**Fix.** Add a footer line:
`weakest: simulation 729 · RTFM Simulation (CAP_SIM) · or take The Emulator`.

### J21 · minor · The RTFM index opens on 29 meta modules (demo only)

**Evidence (ran it, `cmd/demo-learn` on the real guide).**

```
▾ GENERAL                                                     29 modules
   ▾ Getting Started
       Using This Guide          Userland        0+4 topped up
   ▾ Contributing
       Working With MDX          Userland        0+4 topped up
```

The Bronze modules start below the first screen, and "0+4 topped up" is
internal vocabulary (`./internal/screens/learn/learn.go:108`).

**Fix.** Collapse GENERAL, drop the Contributing modules, put the cursor on
the player's tier, and label the column `4 doors`.

### J22 · minor · `:Debug` stops before the program starts

**Evidence (ran it).** The lldb pane opened in about 20 s at
`breakpoint set --name main`, with the process not launched, under a target
named by hash (`f6733880…`). The README says "stopped at main".

**Fix.** Send `process launch` after the breakpoint, or change the README.
(Not re-run; it may depend on lldb's permissions in my environment.)

### J23 · minor · `<bits/stdc++.h>` fails to compile on the first save

**Evidence (ran it).**
`fatal error: 'bits/stdc++…  1 | #include <bits/stdc++.h>`. The built-in
template warns about this in its first line. This player's own template lists
headers, so it matters mostly for anyone else.

**Fix.** Add a `bits/stdc++.h` shim to the judge include path.

### J24 · minor · The route card's timer ignores Overclock

**Evidence (ran it).** The route card said `⏱ ~30:00`; the doors behind it
showed `⏱ 40:00` (30 × 1.15²).

**Fix.** Apply upgrades to the route card estimate.

### J25 · minor · Closing the game mid-door keeps the timer running, silently

**Evidence (ran it, code).** The deadline is absolute (`app.go` `Init`). A
relaunch resumed the door with 1:11 left and no notice. A door that ran out
while the game was closed crashes on launch. Closing a laptop at night
mid-door means a kernel panic in the morning.

**Fix.** On quit with a door open:
`the door's timer keeps running while katharsis is closed · q again`. On
resume: `resumed door_2 · 4:12 passed while away`. Whether a real suspend
exists is your decision.

### J26 · polish · `:q!` shows the editorial before the panic

**Evidence (ran it).** After the `:GiveUp` confirmation, the editorial opened
full-screen; the kernel panic appeared only after `q`. The death beat comes
second. This is already on the integrator's list ("panic before editorial").

### J27 · polish · The reader shows raw heading marks and loses subscripts

**Evidence (ran it).** `#### Input` and `## Resources` (twice) appear as raw
text, `arrᵢ` becomes `arri`, and footnotes render as `^†`.

**Fix.** Reader owner; style the headings in `ui-system.md`.

### J28 · polish · The same quote appeared on consecutive timer crashes

**Evidence (ran it).** `"It is more complicated than you think." — RFC 1925`
on boot 2, then again on boot 5.

**Fix.** Draw quotes per cause from a shuffled bag.

### J29 · polish · The custom-preset submenu lists names only

**Fix.** Show the preset's first comment line beside its name
(`short: a boot of about an hour`).

### J30 · polish · The card's first line is the source, not the title

**Evidence (ran it).** `Codeforces · Codeforces Round 825 (Div. 2) · A` sits
above `Make A Equal to B`. D6 calls the source "a label only". Hierarchy
belongs to `ui-system.md`.

### J31 · polish · The crash footer still offers `[u] upsolve` after recovery

**Evidence (ran it).** `core dump recovered: +37 B banked · [u] upsolve  [r]
reboot`.

## The first 10 minutes: onboarding proposal

The player has 400 h of Dead Cells and does CP: no tutorial wall. Dead
Cells teaches in the first biome by placing one thing at a time. Do the same
with **one-time `dmesg` lines** on the J5 strip. Each appears the first time
its mechanic does, is stored as seen in `Meta`, and is never modal.

```
t=0:00  GRUB        katharsis, prestige 0 (Bronze)            ← cursor here, as now
                    footer: "first boot? it explains itself in dmesg · RTFM is learn mode"
t=0:05  doors       [ 0.0] doors: pick one. ≈% is your clear chance in time.
                           bytes are lost on a crash until committed at a merge window.
t=0:10  door 1      statement shown (J8) · template not judged (J13)
                    [ 9.1] :w judges samples · :Submit judges all tests;
                           a failed submit costs ♥ · :q! gives up and opens the editorial
t≈3:00  first AC    "✓ AC … -O3" beat (J5)
                    [181.0] upgrades last this boot · -O3 (under half the timer) = 2 picks
t≈6:00  first WA    [362.4] WA 6/30 · −1 ♥ (3 left) · samples were red: next time :Submit! (J3)
t≈8:00  merge win.  [480.2] commit: bank it forever, or spend it on this boot (J2)
                    [480.2] daemons are passive perks for the rest of the boot
first crash         three-line summary (J6): what was lost, how to get it back, when it returns
next day            GRUB: "↺ 2 regressions due · ☠ 183 B dump on Karen and Morning · nightly 10-15"
```

Two content rules for boot 1 only:

- The first door of the first boot is a guaranteed AC at the pool floor, with
  tags shown and no curse, blind or mystery.
- The first boot's doors exclude problems without an editorial, so the first
  death always teaches (J16).

## Top 5 to ship first

| # | Change | Findings | Owner (by package) | Size |
|---|---|---|---|---|
| 1 | **End-of-boot truth**: rating delta, unlock progress, regression date and core-dump status on the clear and crash screens; `g GRUB` | J1, J6, J7, J12 | screens/boot, app | S |
| 2 | **`dmesg` event strip** plus the AC beat on biome-ending clears and regression clears | J5, J4 (message), J7 (overwrite line) | screens/boot, app; text from the engine log | S–M |
| 3 | **Ask before spending a life**: `:Submit!` when samples fail; `R R` to retreat | J3, J4 | session/overlay, app/problem | S |
| 4 | **Statement-first HUD** with a pinned status block; leave the template unjudged | J8, J13, J17 | hud, app | M |
| 5 | **Door readiness**: per-card fetch state, progress, `esc` to cancel, switch to a ready door | J9 | screens/boot, app | M |

J2 (a sink for bytes) matters as much as #1, but it needs your decision
first, so it is not on the ship list.

## Decisions only you hold

1. **What does the bank buy?** Recommended default: the merge window offers
   *spend now* (boot-scoped: fsck +♥, cron +time, re-roll) against *commit*
   (bank). At GRUB, `apt` turns the bank into permanent unlocks: daemons
   joining the offer pool, upgrade variants, a third lib slot, starting duck.
   Keep it tied to learning acts, as learning-science M2 asks.
2. **Prestige gate.** Is 60 rated doors (about 24 h) before Silver right?
   Recommended default: keep the gate so calibration stays honest, but show
   it (J1). If it still feels like a wall, unlock on the clear and calibrate
   inside Silver.
3. **Suspend.** Should a door survive closing the game? Recommended default:
   no suspend inside a door, which keeps the timer honest, but warn on quit
   and on resume (J25). The doors screen is already a safe room.
