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
downloads the USACO Guide snapshot (about 150 MB) and the Codeforces index
(about 17 MB) and builds the problem indexes. That takes a minute; later
starts take seconds. Without network on that first start, the game plays
USACO only and tries Codeforces again next time.

| Flag | Default | |
|---|---|---|
| `--root` | `~/cp/runs` | where door solutions are kept |
| `--editor` | `vim` | editor command |
| `--template` | `~/cp/atcoder/template.cpp` | stdin/stdout template |
| `--template-fileio` | `~/cp/template.cpp` | file-I/O template; `problem` becomes the file stem |
| `--scale` | `1` | multiply CPU time limits (below 1 is stricter) |
| `--quiet` | off | no sounds |
| `--preset` | | also load a rules preset file (repeatable); presets in `~/.config/katharsis/presets` load automatically. `presets/short.toml` is a ~1 h boot |

`katharsis practice <dir>` is the training room: any folder with
`<dir>.cpp` and `tests/N.in`/`tests/N.out`, no timer and no stakes.

## Play

```
GRUB ─enter→ boot ─→ doors ─enter→ problem (vim + HUD) ─AC→ upgrade ─→ next room
                                        │                            └ biome end → merge window
                                        └ timer / lives / :q! → kernel panic → upsolve or reboot
```

**In vim** (the game adds these commands; your vimrc loads as usual):

| | |
|---|---|
| `:w` | rejudge the samples (the HUD updates in about a second) |
| `:Submit` | judge the full tests; the HP bar fills, a failure costs a life |
| `:Debug [n]` | lldb on sample n, stopped at `main` (any test with the `strace` daemon) |
| `:Quack` | rubber duck: first the tags, then the technique |
| `:Retreat` | swap the door for a life |
| `:q!` | give up, after a confirmation: the boot crashes and the editorial opens |

**Keys:**

| Screen | Keys |
|---|---|
| GRUB | `↑/↓` select · `enter` boot · `e` edit prestige, preset, seed |
| Doors | `←/→` or `1-3` select · `enter` open · `m` map |
| Problem pane | `s` submit · `t` statement · `?` duck · `R` retreat · `Q Q` give up · `e` reopen a closed editor |
| Merge window | `c` commit bytes · `↑/↓` + `enter` pick an upgrade or daemon, then leave |
| Kernel panic | `u` upsolve the problem · `r` reboot · `l` git log · `q` quit |
| Reader | `j/k` · `space/b` · `g/G` · `/` search, `n/N` · `o` open in the browser · `q` back |
| git log | `j/k` · `enter` expand a boot · `f` filter · `q` back |
| top | `s` sort topics · `q` back |

**Libs.** Mark a snippet in any solution and it becomes reusable:

```cpp
// lib: dsu
struct DSU { ... };
// end lib
```

## Files

| | |
|---|---|
| `~/cp/runs/boot-N/<problem>/` | your solutions; the samples are in `tests/` |
| `~/.local/share/katharsis/katharsis.db` | saves: boots, attempts, rating, bank |
| `~/Library/Caches/katharsis/` | problems, tests, editorials, builds |
| `~/.config/katharsis/presets/*.toml` | custom rules (each overrides the standard rules) |

## Content

USACO problems, with test data and analyses from usaco.org. The problem
index and learn mode use the USACO Guide (CC BY-NC-SA 4.0).

Codeforces problems, with statements, editorials and tests, come from the
[open-r1/codeforces](https://huggingface.co/datasets/open-r1/codeforces)
dataset (CC BY 4.0); statements and editorials are by their Codeforces
authors. Each statement and editorial carries that credit
(`codeforces.Attribution`).
