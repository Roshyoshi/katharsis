# katharsis

A competitive-programming roguelike that runs in the terminal. Each run is a
sequence of locally judged problems. Problems are picked by difficulty, solved
in your own vim, judged on save, and lost to a timer.

North star: the loop should feel like a roguelike (Dead Cells is the
reference). Every number in this document is a setting with a default, not a
fixed rule. Settings are locked for the length of a boot.

## Stack

| Layer | Choice |
|---|---|
| Language | Go 1.26 |
| TUI | Bubble Tea v2, Bubbles v2, Lip Gloss v2, Glamour v2 (`charm.land/*`) |
| Judge compiler | clang 22 (pinned in `flake.nix`; 21.1.8's ASan hangs on macOS 26.5) |
| Editor, layout, debugger | the user's vim, tmux and `/usr/bin/lldb` (not pinned) |
| Storage | SQLite via `modernc.org/sqlite` (no cgo) |
| Settings | TOML rule presets |
| Sound | `afplay` |

## Vocabulary

| Mechanic | Name | Behavior |
|---|---|---|
| run | boot | "Boot #42"; run length is `uptime` |
| start screen | GRUB menu | pick prestige, preset, seed |
| difficulty tier | prestige 0–4 | Bronze, Silver, Gold, Platinum, Advanced; beating the final boss unlocks the next |
| currency | bytes | shown as B, KiB, MiB |
| bank between biomes | merge window | `commit` bytes; uncommitted bytes are lost on a crash |
| death | crash | kernel panic screen |
| shade | core dump | uncommitted bytes stay on the problem that killed you; upsolving it recovers them; crashing again first loses them |
| give up | `:q!` | voluntary crash; the editorial opens (RTFM); the problem stays solvable |
| hint charge | rubber duck | 1st quack reveals tags, 2nd the technique; refills at merge windows |
| lives | lives | a failed submit costs one |
| scroll | upgrade | after each clear pick one: Overclock (+15% timer), ECC (+1 life), Duck (+1 duck) |
| mutation | daemon | passive perk picked at a merge window |
| blueprint / weapon | lib | a snippet you implemented in a trial; link 2 per boot, pasted into the template |
| rune | capability | e.g. `CAP_DP`; earned in learn mode; adds that topic's biomes to route choices |
| elite | regression | a problem that crashed you returns after 3 and 17 days with a blank file |
| boss | named bug | stretch problem, 1.5x timer: Off-by-One, Heisenbug, OOM Killer, Race Condition; final boss The Halting Problem |
| timed door | -O3 bonus | AC under half the timer grants a free upgrade |
| cursed chest | cursed door | no ducks, one WA = crash, 2x bytes |
| daily run | nightly build | fixed seed per day |
| history | `git log` | every boot and attempt |
| stats | `top` | rating trend, per-topic radar |

Daemons (v1): `strace` (failing test input shown; hidden by default),
`nice -20` (+20% timer, a WA costs 2 lives), `fork` (4 doors after an AC),
`/dev/random` (mystery doors pay 2x), `kexec` (one free door swap per biome).

## A boot

```
GRUB ▸ prestige 1 · standard · seed 8f3a
 ├─ Userland     door, door        short timers; cheap crashes
 ├─ merge window commit · ducks refill · pick a daemon
 ├─ biome 2      route choice between two topic biomes
 │               door, door (+ a regression if one is due)
 ├─ merge window
 └─ boss         named bug; at max prestige, The Halting Problem
```

- Full clear about 2 h; typical crash about 1 h 15 m (fitted timers cap at
  1.5x the biome default).
- Door difficulty is chosen from the player's rating estimate to hit target
  clear rates: about 92% early, 70% mid, 50% boss (about 20% full clears).
- Default timers: 15 / 30 / 50 min (early / mid / boss), then fitted to the
  player's own solve times.

## Doors

Each door shows title, tags, difficulty, timer and reward. The source is a
label only; doors draw from every local source by difficulty.

- Blind: tags hidden, 1.5x bytes.
- Mystery: title also hidden (titles can name the technique).
- Cursed: see vocabulary.
- Regression: a past crash, due for re-solve.

## Inside a problem

```
┌─ vim sol.cpp ─────────────────────────┬─ HUD ─────────────────┐
│                                       │ ⏱ 23:41  ♥♥♡  🦆1  46B │
│                                       │ S1 ✓ 3ms  S2 ✗ line 2 │
│                                       │ ▓▓▓▓▓▓░░ 37/52        │
│                                       │ ── statement ──       │
└───────────────────────────────────────┴───────────────────────┘
```

```
:w sol.cpp
└─ file watcher → judge samples
   ├─ clang++ -std=gnu++23 -O1 -g -fsanitize=address,undefined -Wl,-stack_size,0x20000000
   ├─ run each sample (wall-clock kill, wait4 → cpu ms, peak RSS)
   └─ HUD: S1 ✓ 3ms   S2 ✗ line 2: want 7, got 6
:Submit  (vim command → control socket)
└─ judge full tests: -O2, parallel → HP bar; first failure costs a life
:Debug S2 → lldb pane under vim, stdin = S2 (or copied to <name>.in)
:q!      → crash, editorial opens
```

- Inside tmux the game pane shrinks to the HUD and vim opens beside it.
  Outside tmux the game starts its own tmux session.
- The timer also appears on the editor pane's border (a window-scoped
  tmux option, so the player's own status bar is never touched).
- File I/O problems (old USACO) run in a temp dir with `<name>.in`/`.out`.

## Crash screen

```
[ 5421.337] Kernel panic - not syncing: timer expired in "Bovine Shuffle"
[ 5421.337] CPU: 0 PID: 1 Comm: you Tainted: B (blind) C (cursed)
[ 5421.337] Call Trace:
[ 5421.337]  door_4+0x1e/0x30 [The Heap]
[ 5421.337]  boot_42+0x2c/0x40
[ 5421.337] ---[ end Kernel panic ]---

  "It always takes longer than you expect, even when you take into
   account Hofstadter's Law."            — Douglas Hofstadter, GEB (1979)

  core dumped: 312 B on "Bovine Shuffle" · [u] upsolve  [r] reboot  [l] git log
```

Quotes are matched to the cause of the crash and must have a known source.

| Cause | Pool |
|---|---|
| timer | Hofstadter's Law |
| out of lives | Dijkstra, "presence of bugs, never their absence"; Knuth, "only proved it correct, not tried it" |
| `:q!` | Torvalds, "Talk is cheap. Show me the code."; Unix V6, `/* You are not expected to understand this */` |
| runtime error | Kernighan, "debugging is twice as hard"; Hoare, "billion-dollar mistake" |
| TLE | Pike, "Fancy algorithms are slow when n is small, and n is usually small." |
| filler | Thompson, "When in doubt, use brute force."; Torvalds, "C++ is a horrible language." |

## Learn mode (RTFM)

```
RTFM › Silver › Prefix Sums                         □ CAP_PREFIX
  read   USACO Guide module in the TUI (o = open in browser)
  unit   4 doors from the module list, Easy→Hard, tags shown, soft timer
  integ  2 blind doors at the same difficulty
  pass   ■ CAP_PREFIX → topic biomes join route choices; lib unlocked
```

## Content

Local judging only until the game is complete. Doors pick by difficulty
across all sources.

| Source | Size | Difficulty signal | Editorial | When |
|---|---|---|---|---|
| USACO | ~720 | division + problem position | official analysis | v1 |
| CSES | 400 | solver counts | USACO Guide (119) | v1, if login test download works |
| Codeforces (open-r1 dataset) | ~10k | exact ratings | ~60% | after the game is complete |
| Codeforces / AtCoder platforms | — | — | — | last |

v1 excludes interactive problems and problems needing a custom checker for
multiple valid answers. Float tolerance uses a built-in checker.

## Milestones

Built in waves. Contracts first, then independent packages in parallel,
then one integrator.

```
W0  M0 training room       judge, tmux/vim split, file watcher, HUD
W1  contracts              catalog, game types, rules, flavor and screen APIs; deps pinned
W2  parallel build         A content/usaco + content/guide   B game engine + sim
                           C store                           D1 screens/boot
                           D2 screens/meta + reader          F flavor
                           G learn (RTFM)                    E judge+control (after W0)
W3  integrate              internal/app wires engine, store, catalog, screens, session
W4  hardening              balance sim tuning, end-to-end tests, fixes → game done
W5  content                CSES (needs login), Codeforces dataset
W6  platforms              CF sync/submit, AtCoder browser submit
```

Package ownership during W2 (one owner per directory; contract files change
only additively, via the engine owner or the integrator):

```
internal/
├─ catalog/        contract: Problem, Catalog, Topics
├─ rules/          contract + presets (engine owner)
├─ game/           contract types.go + engine (engine owner); sim/
├─ flavor/         flavor owner
├─ screens/        contract screen.go; boot/ (D1); meta/, reader/ (D2)
├─ content/        guide/, usaco/ (content owner)
├─ learn/          learn owner
├─ store/          store owner
├─ judge/ problem/ session/ hud/   W0, then E
└─ app/            integrator (W3)
```

## Platform facts (checked 2026-10-14)

- macOS `ld` caps `-stack_size` at 512 MB; the 8 MB default segfaults at
  1e6-deep recursion.
- macOS rejects `setrlimit(RLIMIT_AS)` and `RLIMIT_DATA` (returns -1); memory
  limits need a watchdog plus `ru_maxrss` (bytes on macOS).
- Apple's libc++ reads ints through `cin` ~25x slower than libstdc++, synced
  off: 1e6 ints in ~970 ms against ~37 ms (scanf ~60 ms), so the judge times
  with GCC 14 (D28, `docs/research/gcc-toolchain.md`).
- nixpkgs' `gcc14` runs natively on aarch64-darwin with its own libstdc++; an
  `exec format error` from a nix g++ means an aarch64-linux store path. GCC
  has no ASan or UBSan runtime there (`-lasan` not found);
  `-fsanitize-trap=undefined` works.
- arm64 g++ rejects x86 `#pragma GCC target("avx2")` with an error no flag
  downgrades; clang ignores it.
- libstdc++'s vector annotations poison spare capacity only after `reserve()`,
  and its string and deque have none, so ASan misses indexing past `size()`;
  `_GLIBCXX_ASSERTIONS` catches it.
- USACO: tests at `usaco.org/current/data/{stem}.zip`, analysis at
  `sol_{stem}.html`; stems come from USACO Guide's `id_to_sol.json`. File I/O
  through the 2020 US Open, stdin/stdout from Dec 2020. Zips use
  `N.in`/`N.out` (`I.N`/`O.N` only Nov–Dec 2011).
- USACO Guide: `content/**/*.problems.json`, order in `content/ordering.ts`,
  CC BY-NC-SA 4.0. Difficulty labels are module-relative.
- CSES: tests need a login (`/problemset/tests/{id}/`, POST with
  `csrf_token`); unverified.
- Codeforces: HTML returns Cloudflare 403 to scripts; `/api` works, 1 call per
  2 s. open-r1/codeforces has statements, editorials, tests, Python checkers.
- AtCoder: login and practice submits are CAPTCHA-gated since spring 2025;
  public test data withdrawn Dec 2024. Difficulty and solve-time models from
  kenkoooo AtCoder Problems.
- jj: a new file is invisible to nix flakes until a jj command snapshots the
  working copy (`jj st`).
