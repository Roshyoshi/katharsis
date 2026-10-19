# katharsis

A roguelike where the fights are competitive-programming problems. Pick a
door, solve it in your own vim before the timer runs out, and push through
biomes to the boss. Crash and you lose what you didn't commit, unless you go
back and upsolve the problem that killed you.

Design and vocabulary: [DESIGN.md](DESIGN.md).

## Run

```sh
nix run ~/katharsis                          # the game
nix profile install ~/katharsis              # or install `katharsis` on PATH
cd ~/katharsis && nix develop -c go run ./cmd/katharsis   # from source
```

The game runs inside tmux. Started outside tmux, it opens a tmux session of
its own, and starting it again attaches to that session. The first start
downloads the USACO Guide snapshot (about 150 MB) and builds the USACO index,
which takes a minute; later starts take seconds. The Codeforces index (about
17 MB) builds in the background while you play: GRUB shows "Codeforces
indexing… N%" until it joins, from the next boot on. Without network it shows
"Codeforces offline", the game plays USACO only, and it tries again next
start.

| Flag | Default | |
|---|---|---|
| `--root` | `~/cp/runs` | where door solutions are kept |
| `--editor` | `vim` | editor command |
| `--template` | `~/cp/atcoder/template.cpp` | stdin/stdout template |
| `--template-fileio` | `~/cp/template.cpp` | file-I/O template; `problem` becomes the file stem |
| `--scale` | `1` | multiply CPU time limits (below 1 is stricter) |
| `--quiet` | off | no sound at all; `--no-music` and `--no-sfx` silence one each |
| `--music-volume`, `--sfx-volume` | `audio.toml` | 0..1, over the settings file |
| `--motion` | `full` | `reduced` draws every story scene at once; any key skips a scene, esc leaves the story |
| `--fps` | `30` | frame-rate cap for the story animations |
| `--preset` | | also load a rules preset file (repeatable); presets in `~/.config/katharsis/presets` load automatically. `short` (4 doors) and `patch` (3 doors) are built in beside `standard` |
| `--session-budget` | `settings.toml`, else `3h` | past this much play in a day, the end of a boot says so in one line; `0` is off |

`katharsis practice <dir>` is the training room: any folder with
`<dir>.cpp` and `tests/N.in`/`tests/N.out`, no timer and no stakes.

`katharsis motd` prints one line when something is due (regressions, a core
dump, a nightly not yet played) and nothing otherwise. It reads the saves
without writing them. To see it in each new shell, add to `~/.zshrc`:

```sh
command -v katharsis >/dev/null && katharsis motd
```

## Play

```
GRUB ─enter→ boot ─→ exit ─→ doors ─enter→ problem (vim + HUD) ─AC→ upgrade ─→ next room
                                                │                         └ biome end → merge window
                                                ├ timer / :q! → burns cycles, the room is lost, the boot goes on
                                                └ 0 cycles → kernel panic → upsolve or reboot
```

A boot after the first is three acts on a map: Userland, a topic biome
and a boss, twice, then a third biome (or a skip) and The Halting
Problem. The exit you take picks the next biome and the boss behind it.
The pool is `♥ 30/50 cycles`. Between any two doors the boot can be left
(`q`) and picked up again on the map; inside a door there is no quitting.
At Boss I and Boss II the act is judged for two vaults, each an extra
upgrade pick: the timed door, for an act within 60% of its doors' clocks,
and the untouched door, for an act with no failed submit, timeout, `:q!` or
paid retreat. The doors screen shows the split as you go.

Each pick pool has ten from the first boot, more with apt, drawn by
rarity (common, uncommon, rare): daemons at merge windows, three seats,
and once they are full a swap for 64 B; an upgrade after every clear, or a
skip for 2 cycles; a shelf of five items a window, fsck always on it. The
rare ones change the rules: `selinux` fills your cycles and curses every
topic door with a longer clock; `cpupower` lengthens every clock and caps
cycles at 20, so every boss is one shot; `-Ofast` curses your next door
for two more picks. The full roster and its numbers:
[docs/research/perks.md](docs/research/perks.md).

**In vim** (the game adds these commands; your vimrc loads as usual):

| | |
|---|---|
| `:w` | rejudge the samples (the HUD updates in about a second) |
| `:Submit` | judge the full tests; the HP bar fills; a failure burns 2 cycles (a CE, or a fail on a sample, is free); another sent before the verdict is refused |
| `:Debug [n]` | lldb on sample n, stopped at `main` (any test with the `strace` daemon) |
| `:Quack` | rubber duck: first the tags, then the technique |
| `:Retreat` | swap the door for 5 cycles (`kexec`: free once an act), only in its first 2:30 |
| `:q!` | give up, after a confirmation: burns the act's price (10, 12, 15; a boss 20 and a new boss door), the room is lost and the boot goes on; The Halting Problem is one shot |

**Keys:**

| Screen | Keys |
|---|---|
| GRUB | `↑/↓` select · `enter` boot · `e` edit prestige, preset, seed · `upsolve…`: the core dump, regressions due, and the doors your boots lost, untimed · paused from a map: `continue boot #N` first (or `esc`), and a new boot asks before it ends the paused one |
| Map (the hub) | `enter` resume · `n` new boot (ends the one in flight as a crash) · `g` pause into GRUB · `t` RTFM · `q` quit |
| Exits | `←/→` or `1-4` select · `enter` take · `t` RTFM on a locked one · `m` map |
| Doors | `←/→` or `1-3` select · `enter` open · `m` map · `q` quit (the boot waits on the map) |
| Problem pane | `s` submit · `t` statement · `w` the samples in full · `?` duck · `R` retreat · `Q Q` give up · `e` reopen a closed editor |
| Samples (`w`) | input, expected output and your output side by side, stderr under them, the build's warnings above · `h/l` or `1-9` sample · `j/k` · `space` page · `g/G` · `w` or `q` back |
| Merge window | `c` commit bytes · `f` fsck · `↑/↓` + `enter` pick an upgrade or daemon, then leave |
| Kernel panic | `u` upsolve the problem · `e` editorial · `r` reboot (twice over a live core dump) · `s` short · `p` patch · `b` read the story beat · `l` git log · `g` GRUB |
| Cleared | `r` reboot · `s` short · `p` patch · `b` story beat · `l` git log · `g` GRUB |
| Reader | `j/k` · `space/b` · `g/G` · `/` search, `n/N` · `o` open in the browser · `q` back |
| git log | `j/k` · `enter` expand a boot · `r` play its seed again (a past nightly too) · `f` filter · `c` walk the calendar of days · `q` back |
| top | `s` sort topics · `q` back |

**Libs.** Mark a snippet in any solution and it becomes reusable:

```cpp
// lib: dsu
struct DSU { ... };
// end lib
```

**Judge.** `:Submit` judges with GCC 14 (`g++ -O2`, libstdc++, as on
Codeforces), and `:Debug` builds with the same g++ at `-O0 -g`. Saves run
the samples under clang's ASan and UBSan, built against that libstdc++.
Sample builds also warn (`-Wall -Wextra -Wshadow`): reading a variable
never set fails the build, since neither sanitizer sees it and `-O2` may
read a lucky 0; the other warnings show beside the samples. `:Submit`
builds without them, as the judge does.
`<bits/stdc++.h>`, `__gnu_pbds` and `std::__gcd` work as on the judge;
`#pragma GCC target(...)` lines are dropped, since arm64 g++ rejects x86
targets. Where the bundled g++ does not run, all three use clang and libc++.
In a tmux session the game starts, `g++` and `clang++` on PATH are these.

## Files

| | |
|---|---|
| `~/cp/runs/boot-N/<problem>/` | your solutions; the samples are in `tests/` |
| `~/.local/share/katharsis/katharsis.db` | saves: boots, attempts, rating, bank |
| `~/Library/Caches/katharsis/` | problems, tests, editorials, builds |
| `~/.config/katharsis/presets/*.toml` | custom rules: each lists only what it changes from the standard rules ([rules.go](internal/rules/rules.go)), a `[[biome]]` list replaces them all, and an unknown key is an error; `standard`, `short` and `patch` are built-in names |
| `~/.config/katharsis/audio.toml` | volumes, fades, ducking, the timer warning ([settings](internal/audio/settings.go)) |
| `~/.config/katharsis/settings.toml` | `session_budget = "3h"` (see `--session-budget`); `sample_warnings = [...]`, the sample builds' warning flags (`[]` for none) |
| `~/.config/katharsis/music/<pool or topic>/` | your own mp3, ogg, wav or VGS mml files for that pool or biome |
| `~/Library/Caches/katharsis/music/` | downloaded music |

## Content

USACO problems, with test data and analyses from usaco.org. The problem
index and learn mode use the USACO Guide (CC BY-NC-SA 4.0).

Codeforces problems, with statements, editorials and tests, come from the
[open-r1/codeforces](https://huggingface.co/datasets/open-r1/codeforces)
dataset (CC BY 4.0); statements and editorials are by their Codeforces
authors. Each statement and editorial carries that credit
(`codeforces.Attribution`).

The story quotes Lewis Carroll's *Alice's Adventures in Wonderland* (1865)
and *Through the Looking-Glass* (1871), which are in the public domain, and
short passages of *Structure and Interpretation of Computer Programs*, 2nd
ed., by Harold Abelson and Gerald Jay Sussman with Julie Sussman (MIT Press,
1996), licensed under
[CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/). Quotes
are verbatim and credited; the story bible and its sources are in
[docs/story/](docs/story/story-bible.md).

## Music and sound

```sh
katharsis audio fetch     # download the music once (176 files, 5 MiB)
katharsis audio credits   # every track, its arranger and license
```

The music is Touhou chiptune: SUZUKI PLAN's 東方BGM on VGS, 175 chiptune
arrangements of ZUN's music, used with the arranger's written permission
for Touhou Project fan works, plus SwapXFO's NES "Misty Lake" (CC BY 4.0).
Each biome draws from the matching part of the games and climbs by act
as a run climbs by stage: title themes on GRUB, stage 1-2 themes in
Userland, stage 3-4 in act I, 5-6 in act II, extra stages in act III;
stage 1-3 bosses for Boss I, 4-5 and extra bosses for Boss II, and
final-boss themes for The Halting Problem. A pool plays every track once
before any plays again, across biomes and launches (the cache's
`rotation.json` keeps its place), and GRUB and RTFM play their own music
over a paused boot. A toast credits each track as it starts; GRUB →
Credits lists them all. The game never bundles the
music: `audio fetch` downloads it from each source, checked against
pinned SHA-256 hashes. Without it the game plays on with its effects
only. `ctrl+t` turns the music on or off.

Sound effects are synthesized at start (`internal/audio/sfx.toml`).
Licensing evidence, the stage-to-pool mapping and the engine's numbers:
[docs/research/audio.md](docs/research/audio.md).

> 東方Project二次創作: the Touhou music in katharsis is fan arrangements, a
> fan work based on Touhou Project. Touhou Project and its original music
> are by ZUN (上海アリス幻樂団, Team Shanghai Alice). katharsis is unofficial
> and not affiliated with Team Shanghai Alice. Each arrangement belongs to
> its arranger and is used under the license shown on the credits page.

VGS songs play through a Go port of SUZUKI PLAN's VGS MML compiler and
BGM decoder, Copyright (c) 2016, Yoji Suzuki, under the BSD 2-Clause
License (full text in `internal/audio/vgs/vgs.go` and on the credits
page).
