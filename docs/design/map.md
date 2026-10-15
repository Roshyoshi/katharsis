# The run map: three acts on a Dead Cells-style graph

Map design thread (D47, D49), 2026-10-15. **Status: proposed.** It asks the
user for the decisions in §(j). Nothing here is merged. The prototype is rules
data and engine flags in the `map-design` workspace, evidence only (§(l)).
Baseline: the D47 cycles meta (`docs/research/meta.md`) with the user's
numbers, and the pre-D46 clock the user restored (fit floor 0.5, headroom
1.25, `timer_mult` 1).

**Evidence labels**, as in `meta.md`: *industry* (developer talk, interview,
store page, official wiki), *secondary*, *player report*. *(inference)* marks
my own reasoning. The Dead Cells wiki blocks scripts, so its facts were read
through a summarising fetcher; treat its exact counts as unverified.

## (a) Summary

**Recommendation: a seven-tier map in three acts, about nine doors, and the
trial-by-fire band met at P0.** Each act ends in a boss chosen by the route.
Each topic biome is one room, and every exit shows its topic, gate, reward
and the boss behind it. The preset is `rules.Map()`.

```
act I    Userland (2 rooms) → 1 of 4 topic biomes → ⚔ Off-by-One | OOM Killer
act II   1 of 3 topic biomes → ⚔ Heisenbug | Race Condition | Undefined Behavior
act III  a topic biome, or a skip for 2 picks → ⚔ The Halting Problem, one shot
```

**Where it lands** (simulator, P0, model players 1000 / 1300 / 1600, boots
51–250 of two seeds; the band is the coordinator's D49 default):

| Measure | Band | Map default | Today's shape with D47 cycles |
|---|---|---|---|
| Full clears, P0 | 40–55% | **45 / 45 / 47%** | 83 / 84 / 83% |
| Crash hazard by act, if reached | ~15 / 25 / 35% | **10–15 / 20–22 / 32–35%** | 0–2 / 2–3 / 13–14% |
| G1, a crash in Userland | ≤ 10% | **0–1.2%** (≤ 9% to P8) | 0.2–2.0% |
| Full clears P4 / P8 | ≤ 30% / ~10% | **21–22% / 7–10%** | n/a |
| Door time, mean boot | "2–3 h" | **2h52–3h07** | 2h16–2h21 |
| Door time, full clear (median) | | **2h44–3h08** | 1h40 |

Today's shape is `standard`'s three biomes with meta.md's D cycles, the
pre-D46 clock and Overclock as it is.

**What it changes, and why each change is there:**

1. **Three acts, one room a topic biome.** A full clear is about nine doors.
   At the model's pace a door takes about 20 minutes, so nine is what fits
   three hours (§(d), §(i)6).
2. **The final is one-shot; Bosses I and II retry at −20.** The one-shot is
   the act-III lever: it moves act III's hazard from 14–20% to 32–35%
   (§(i)2).
3. **kexec swaps once an act, not once a biome.** With seven tiers the
   per-biome free swap became the hidden net, worth 8–14 points of full
   clears (§(i)3).
4. **Overclock is additive, capped at +30%, and costs 5 cycles.**
   Free compounding picks stretched a three-act boot to 5 hours (§(i)4).
5. **Prestige bites late.** P3–P8 cut acts II–III and the cycles pool, not
   Userland, which keeps its own clock (10 → 9 → 8) outside the prestige
   timer cuts. Boss I has a 0.45 floor, so it does not become the wall
   (§(f)).

**What it costs.** The band is harsher by design. Over a learning player's
year (§(i)5):

- **Losing streaks double:** 29 against 14 runs of three crashes in a row.
- **Quits rise** from 73% to 77%, or from 70% to 77% if every boot plays
  the top level; 30 players carry about ±8 points of noise.
- **Durable skill per hour is about the same.**
- **The ladder is short:** 26 of 30 learning players reach P8 within the
  year.

## (b) What Dead Cells' map does

Facts from the wiki and Motion Twin, as the research agent read them
(sources in §(m)).

| Element | Dead Cells | Source |
|---|---|---|
| Graph | Prisoners' Quarters, then 2 biome tiers, boss I; 2 tiers, boss II; 1 tier, boss III; each biome exits to 1–4 of the next tier | [BIO] industry |
| Bosses | B1 Concierge, Conjunctivius, Mama Tick, Death; B2 Time Keeper, Giant, Scarecrow; B3 Hand of the King, Servants, Dracula; B4 Queen, Collector (5 BSC) | [BIO], [BSC] |
| Boss by route | the exit taken picks the boss: Ramparts → Black Bridge, Ancient Sewers → Insufferable Crypt, Morass → Nest | [BIO] |
| Passage (between biomes) | the Collector (cells to permanent unlocks), blacksmith, mutations, a fountain that "restores all of the player's Health Flask charges and any missing health"; cells not spent are lost on death | [PAS], [CEL] |
| Fountain by difficulty | BC1 every other passage; BC2 none, one flask recharge a passage; BC3 three charges a run; BC4–5 none | [BSC] |
| Gates | runes (Vine, Teleport, Ram, Spider) open exits; each rune drops in a biome the previous rune opens (inference from the requirements) | [RUN] |
| Other gates | Boss Stem Cell level (Cavern at 2, Crypt from Ramparts at 3, Astrolab at 5), keys, "after Dracula" | [BSC], [KEY] |
| Scrolls | about 24 a run; siblings in a tier carry about the same count and differ in delivery: free, or in a cursed chest (inference from the table) | [BIO], [CC] |
| Skip | Guardian's Haven exits straight to the Throne Room and "grants two Scrolls of Power for skipping one level" | [GH] |
| Risk branches | "Some paths will be shorter but harder, or maybe the foes there will be more suited to your playstyle"; cursed biomes from BC2, marked by "an icon above the exit… so that you can have the choice of taking this challenge" | [ITCH], [NL35] industry |
| Run length | timed doors at 2:00, 8:00, 15:00, 26:00 into the first four tiers; a full run 30–60 min | [TD] industry; [TWIN] secondary |
| What the player sees | the passage "will usually take the appearance of the biome it leads to"; a world map shows discovered biomes by name and "the path used in the current run"; no difficulty stars anywhere | [PAS], [U25]; no stars: inference from absence |

What the developers say makes it work:

- **Hand-made graph, random rooms.** "The overall world layout is fixed and
  designed by hand", and the keys to new paths never move with the seed
  ([DN], industry). In katharsis the graph is fixed and the doors are drawn.
- **Choice of path is the point.** "Instead of the regular, rigid
  progression between levels, we would rather leave some room for the player
  to choose their path" ([ITCH], industry). Early access promised "more
  levels organised into more branches to give you more choice" ([80LV]).
- **Mastery opens routes.** Runes are the metroidvania half of
  "roguevania" ([PCGI], [STEAM]). katharsis has the same half already:
  capabilities earned in RTFM add topic biomes (`game/capability.go`).

## (c) The graph

There are seven tiers in three acts, and a player takes one node a tier.

- `+1` is a scroll (one more upgrade pick) at the node's exit.
- The gate column lists the capability, apt package or prestige an exit
  needs.
- Bronze topics, The ALU and Kernel Space are always open. So a player with
  no capability always has a route, as Dead Cells always leaves a rune-less
  one.

The shape:

```
          act I                   act II                 act III
 Userland ─ 1 of 4 ─ ⚔ I ── 1 of 3 ─ ⚔ II ── 1 of 3 or skip ─ ⚔ Halting Problem
 2 rooms    1 room   2 kinds   1 room  3 kinds  1 room / +2 picks  one shot
```

Every node and where its exit leads (the bosses' costumes are the story
bible's):

```
 t  node                   +  topic            gate          exits to
 ACT I ────────────────────────────────────────────────────────────────────
 0  Userland                  mixed                          every tier-1 node
 1  Brute Force Bunker        complete-search                Off-by-One, OOM
 1  The Emulator              simulation                     OOM Killer
 1  The Scheduler             sorting-greedy                 Off-by-One
 1  The Accumulator       +1  prefix-sums      CAP_PREFIX    Off-by-One
 2  ⚔ Off-by-One                                             Bisect, Window, ALU
 2  ⚔ OOM Killer                                             ALU, Heap, Lexer
 ACT II ───────────────────────────────────────────────────────────────────
 3  Bisect Bridge         +1  binary-search    CAP_BSEARCH   Heisenbug
                                               or apt git
 3  The Sliding Window    +1  two-pointers     CAP_TWOPTR    Race Condition
 3  The ALU                   math                           Heisenbug, Race
 3  The Heap              +1  data-structures  CAP_DS        Heisenbug, UB
 3  The Lexer             +1  strings          CAP_STRING    Race, UB
 4  ⚔ Heisenbug                                              Routing, Cache
 4  ⚔ Race Condition                                         Union, Dijkstra
 4  ⚔ Undefined Behavior                       P≥1           Filesystem, Frame
    every Boss II also exits to Kernel Space and to the skip
 ACT III ──────────────────────────────────────────────────────────────────
 5  Routing Table         +1  graphs           CAP_GRAPH     The Halting Problem
 5  The Cache             +1  dp               CAP_DP        (every tier-5 node)
 5  Union Hall            +1  dsu              CAP_DSU
 5  Dijkstra's Reach      +1  shortest-paths   CAP_SSSP
 5  The Filesystem        +1  trees            CAP_TREE
 5  The Framebuffer       +1  geometry         CAP_GEOM, P≥2
 5  Kernel Space              mixed
 5  skip                  +2  no room
 6  ⚔ The Halting Problem                     one shot
```

Names and keys come from the story bible, except **Kernel Space** (a mixed
biome for act III; scene suggestion: the Queen's croquet-ground, where every
game is mixed). The storyteller owns that name.

Two gates beyond capabilities, as Dead Cells gates by Boss Stem Cell and by
key:

- **Prestige:** Undefined Behavior opens at P1 ("through the glass"), The
  Framebuffer at P2.
- **apt:** `git` opens Bisect Bridge without CAP_BSEARCH (`git bisect`). It
  replaces git's `route_offers`, which a fixed graph has no use for.

**The locked exit is the RTFM hook.** A locked exit stays on screen with its
capability and the module that grants it. That makes each trial a visible
new route, the "new toy" rhythm `meta.md` §(f) found missing (inference).

## (d) The numbers

### Tiers

| Tier | Rooms | Aim (floor) | Base clock | Timeout or give-up | Retry |
|---|---|---|---|---|---|
| 0 Userland | 2 | 0.92 | 10:00 (9 at P3, 8 at P6) | −10 | the room is lost, the boot goes on |
| 1 act I biome | 1 | 0.75 (0.6) | 21:00 | −10 | same |
| 2 Boss I | 1 | 0.55 (0.45) | 32:00 | −20 | a new boss door |
| 3 act II biome | 1 | 0.70 (0.6) | 24:00 | −12 (×1.25) | the room is lost |
| 4 Boss II | 1 | 0.45 | 36:00 | −20 | a new boss door |
| 5 act III biome | 1 or skip | 0.65 (0.6) | 28:00 | −15 (×1.5) | the room is lost |
| 6 The Halting Problem | 1 | 0.50 | 40:00 | the boot ends | none |

- **Cycles:** 30 at start, 50 max. A failed hidden-test submit costs 2
  (ECC turns it into 5:00 off the door's clock for the rest of the boot).
  Retreat costs 5 and locks 2:30 into a door.
- **Heals:** none on AC. Cycles come back through fsck (+10 for 64 B in any
  merge window) and the fountain (+5, free, only in the two windows between
  acts, up to 30).
- **Base clock:** about 0.8 of the model's 0.85 solve-time quantile at each
  aim. After 20 solves the timers fit the player's own pace (0.85 quantile ×
  1.25, clamped to 0.5–1.5 of the base).
- **Boss II at 0.45, the final at 0.50.** Boss II is the hardest single
  door, and the final is the one that cannot be retried.

### Time is bought, never free

The user's rule (10-15): each source of time has a price.

| Source | What it gives | Its price in the map | Today on main |
|---|---|---|---|
| Base clock | the tier timers above, fitted to the player's pace | none: the floor every other price is set against | 15 / 30 / 50 min |
| `timer_mult` | scales every timer | a setting, default 1 | none (new knob) |
| Prestige | ×0.95 (P1), ×0.9 (P3), ×0.9 (P4) on every tier but Userland; Userland 10 → 9 (P3) → 8 (P6) | — | the same multipliers on every tier; Userland fixed |
| `nice` | +20% on every door | doors pay ×0.8 (D46) | a WA costs 2 lives (D46 is not on main) |
| `at` | +5:00 on the next door | 40 B, once the `at` package is installed | same |
| Overclock | +15% on later doors | **5 cycles a pick, additive, two at most (+30%)** | a free pick that compounds without a cap |
| ECC | a WA costs 5:00 of the door's clock instead of 2 cycles | spends time to save cycles; once a boot | +1 life |
| Retreat | a new door with a fresh clock | 5 cycles, only in the first 2:30 | 1 life, at any time |
| kexec | one free swap | a daemon slot; **once an act** | once a biome |
| `chrt` | −20% on every door | sells time for ×1.5 pay | same |
| Skip exit | skips act III's room (about 25 min) | that room's bytes; pays 2 picks | none (new) |
| -O3 | a free upgrade for AC under half the timer | earned; **measured on the timer before bought time** | measured on the bought timer |
| Buy-back timers | longer timers on floor doors (easing) | free; stays off, and is announced and declinable if ever on (GR9) | off |
| Submit grace | 30 s for the judge after the deadline | — (judge latency, not thinking time) | same |

The audit found two free sources and fixed both: compounding Overclock, and
-O3 rewarding bought time.

**The clock sets boot length, not survival.** The aim calibration
(`game/calibration.go`) holds each tier's clear rate at its aim whatever the
clock. In the simulator, cutting the clock by a third cut mean door time by
1h30 and moved full clears by 6 points at most. The engine answered the
tighter clock with lower-rated doors (§(i)6). So a tighter clock means easier
problems solved faster, not more crashes. Harder boots come from aims, costs
and nets.

### Prestige across the three acts

Levels stack, as Dead Cells' Boss Stem Cells do. P1–P4 keep today's risk
modifiers and D47's cuts. The cuts that remain land on acts II–III and the
pool, so the hazard keeps rising by act. Results are full clears and G1,
seed 7, boots 51–250, players 1000 / 1600.

| Level | Adds | Full clears | G1 | Hazard I / II / III |
|---|---|---|---|---|
| P0 | the band above | 49 / 42% | 1.0 / 0.5% | 13–16 / 15–23 / 33–37% |
| P1 | blind +15%, timer ×0.95, WA 3 | 44 / 44% | 2.5 / 0.5% | |
| P2 | −1 duck, mystery +5%, cursed +4%, fsck heals 5 | 35 / 33% | 3.0 / 0.5% | |
| P3 | cursed +5%, timer ×0.9, Userland 9:00, acts II–III fails +5 | 31 / 28% | 3.5 / 1.5% | |
| P4 | cursed +4%, timer ×0.9, start 25 | **22 / 21%** | 7.0 / 3.0% | 32–34 / 38–46 / 42–46% |
| P5 | no fountain | 18 / 23% | 2.5 / 2.5% | |
| P6 | Userland 8:00, Boss II one-shot | 12 / 12% | 5.0 / 3.5% | |
| P7 | WA 4, start 20 | 15 / 8% | 4.5 / 5.5% | |
| P8 | every boss one-shot | **10 / 7%** | 9.0 / 3.0% | 37–45 / 67–68 / 46–68% |

- **The story's rings:** P5–P6 are ring −2 (SMM) and P7–P8 ring −3 (the
  ME). The story bible already calls these "rumour only".
- **Noise:** each cell is 200 boots, about ±3.5 points. P1 above P0 at 1600
  and P7 above P6 at 1000 are noise.
- **The timer multipliers skip Userland**, whose clock is set per level:
  10:00, then 9:00 at P3 and 8:00 at P6. Cutting it twice put G1 at 11.5%
  at P7.
- **P8 was softened.** With a further +5 on late fails it cleared 3–9%;
  with every boss one-shot alone, 7–10%. P4 sits at 21–22%, inside the
  band's 30% ceiling with room to spare.
- **Front-loaded cuts failed.** The first ladder started P4 at 20 cycles
  and put the fail cut on every act. It made Boss I a wall: 43–69% of boots
  crashed in act I from P4 on.

## (e) Transition areas: the merge window is the Collector and the fountain

- **The merge window keeps its job.** Commit banks bytes (the Collector:
  bytes left uncommitted are lost on a crash, as cells are). It also refills
  ducks, offers one daemon and sells items.
- **The fountain is a partial, flask-style refill, only between acts.** +5
  cycles, free, up to the starting 30, in the two windows that follow a boss.
  The other windows have none. It is gone from P5, as Dead Cells removes
  fountains with Boss Stem Cells.
- **It barely moves the band.** No fountain, +5 or +10 between acts all
  land at 45–56% full clears, within noise; +10 in every window adds 3–9
  points. The reason is that fsck sells the same heal: the model buys it
  1.2–1.4 times a boot, and without fsck full clears fall 2–9 points.
- **It is there for feel.** It marks the act boundaries, and it is the one
  free heal the brief allows ("at most a partial refill"). Decision 6 in
  §(j) is whether to keep it.

## (f) Boss rules under cycles

| Boss | Rule | Why |
|---|---|---|
| I (Off-by-One, OOM Killer) | retry at −20, a new boss door | an act-I death should be rare (band 15%); one retry is affordable from 30 |
| II (Heisenbug, Race Condition, Undefined Behavior) | retry at −20; one-shot from P6 | its retry is the act-II tension: fsck or not before it |
| III (The Halting Problem) | one-shot at every prestige | the lever for act III's 35%; with retries act III's hazard falls to 14–20% and full clears rise to 55–61% |

- **Boss I has a floor of 0.45.** Prestige stops raising its doors there
  (D22's rule, applied to a boss). Without the floor, act I's hazard at
  P4–P8 ran 34–62%; with it, 23–42% (both on the 0.85-quantile clock).
- **The final is The Halting Problem at every prestige** (default; decision
  1). The story bible's reveal at the first `halting-problem` door then
  fires at ring 3, so the storyteller needs a ring-3 Jabberwock beat. Alice
  meets the poem in chapter I of *Through the Looking-Glass*, mirrored and
  "rather hard to understand", before anyone explains it.

## (g) The hub screens

The map is the hub (D47). A boot can be suspended between doors only.

### The map, on launch with a boot in flight

```
 MAP · boot #42 · P1 ring 2 · suspended after ⚔ Off-by-One   ♥ 22/30   46 B
 ───────────────────────────────────────────────────────────────────────────
 act I    Userland ✓✓ ── The Scheduler ✓ ── ⚔ Off-by-One ✓ (retry −20: unused)
            not taken: Brute Force Bunker · The Emulator · The Accumulator
 act II   ● merge window: commit, fountain +5 ♥, then an exit
              The ALU             math            → ⚔ Heisenbug | ⚔ Race Cond.
              Bisect Bridge       binary-search   [CAP_BSEARCH or apt git] +1
              The Sliding Window  two-pointers    [CAP_TWOPTR] +1
 act III  ○ one room, or skip for 2 picks, then ⚔ The Halting Problem (one shot)
 ───────────────────────────────────────────────────────────────────────────
 core dump: 312 B on "Bovine Shuffle" (boot #41), at risk if #42 is abandoned
 enter resume · n new boot (ends #42 as a crash) · u upsolve · t RTFM · q quit
```

### The exit choice (today's route screen, one card per exit)

Dead Cells names exits by biome and shows no difficulty. katharsis keeps its
◆ band and aim, and adds the boss behind each exit and the gate.

```
 route · act II · after ⚔ Off-by-One                       ♥ 22/30  ⏱ ~24:00
      Pick an exit. It sets the next door and the boss behind it.
 ┌ ▸ 1 ─────────────────┐ ┌ 2 ───────────────────┐ ┌ 3 ───────────────────┐
 │ The ALU              │ │ Bisect Bridge        │ │ The Sliding Window   │
 │ math                 │ │ binary-search        │ │ two-pointers         │
 │ Ambition,            │ │ one side taller,     │ │ "let's all move one  │
 │ Distraction, ...     │ │ one shorter          │ │ place on"            │
 │                      │ │                      │ │                      │
 │ 1 room · ⏱ ~24:00    │ │ 1 room · ⏱ ~24:00    │ │ locked: CAP_TWOPTR   │
 │ ◆1150–1350 · aim 70% │ │ ◆1150–1350 · aim 70% │ │ RTFM › Silver ›      │
 │ a fail −12 ♥         │ │ +1 upgrade pick      │ │   Two Pointers  (t)  │
 │ → ⚔ Heisenbug        │ │ → ⚔ Heisenbug        │ │ → ⚔ Race Condition   │
 │   ⚔ Race Condition   │ │                      │ │                      │
 └──────────────────────┘ └──────────────────────┘ └──────────────────────┘
```

### The merge window between acts

```
 merge window · after ⚔ Off-by-One · act II next     ♥ 17/30 → 22  212 B
   "It's always six o'clock now."
   fountain  +5 ♥, free, between acts only          ✓ 17 → 22
   commit    bank 212 B                              c
   fsck      +10 ♥ for 64 B                          f
   daemon    kexec · strace · /dev/random            1–3
   leave     act II's exits                          enter
```

### The boss card (S9's, with the rule on it)

```
 ⚔ Off-by-One · Tweedledum and Tweedledee       act I boss · ◆1400 · ⏱ 32:00
   "Contrariwise"
   a fail or :q! costs 20 ♥ and draws a new boss door (22 left)
 ⚔ The Halting Problem · the Jabberwock         final · ◆1500 · ⏱ 40:00
   one shot: a fail or :q! ends the boot
```

**Story beats** (each once, recorded in `Meta.Seen`): first act II, first act
III, first skip, first locked exit (the Gryphon: "That's enough about
lessons", pointing at RTFM), each boss's card, and rings −2 and −3 at P5 and
P7. The merge-window lines stay the Hatter's.

## (h) The engine contract

The prototype keeps `State.Biomes` as one entry per tier, so every index the
engine, screens, store and debrief use stays valid. A tier's node is written
into the boot's rules snapshot when its exit is taken, as `trial.go:62`
already writes a trial's sizes.

| Where | Change | JSON / saves |
|---|---|---|
| `rules.Biome` | `id`, `tier`, `topic`, `next`, `boss`, `package`, `min_prestige`, `picks`, `skip` | TOML keys; zero values keep today's biomes |
| `rules.Rules` | `[[node]]` (the graph; `Biomes` becomes the tier skeleton), `fountain_hp`, `fountain_where`, `ecc_penalty`, `retreat_lock`, `one_shot`, `overclock_max`, `overclock_hp`, `timer_mult`, `o3_on_base`, `kexec_per_act`, `first_clock_own` | `Load` replaces `[[node]]` whole, as `[[biome]]`; `validateNodes` checks one start node, unique IDs, exits to the next tier, a boss name on boss tiers only |
| `rules.Ascension` | `one_shot`, `fountain_hp`, `first_timer`, `cycles`, `late_fail_hp` | sums like the rest; `first_timer` takes the highest level's |
| `game.BiomeState` | `Node string` | `omitempty`; "" in a boot without a map |
| `game.BiomeState` | `Exits []Exit` (node, name, topic, rooms, boss, picks, skip, and `Locked` as the gate's text) | replaces the prototype's node IDs in `Offers`; `Offers` keeps topics for the log and old screens |
| `game.State` | `Caps []string` (the capabilities at boot start, as `Packages` snapshots apt), `Fountained int` | `omitempty`; exits read the snapshot, so a trial passed while the boot is suspended cannot change its exits |
| `game.Attempt` | `At` (time bought with `at`) | `omitempty`; -O3 measures the timer without it |
| `game.MaxPrestige` | 4 → 8 | GRUB, `flavor/beats.go:137` and the story's ring names need P5–P8 |
| engine | `layoutMap`, `enterMapTier`, `takeNode`, the fountain in `enterMerge`, one-shot by bosses left in `failCost`, `KexecUsed` reset per act | none |

- **Deterministic.** Exits come from the rules snapshot, `State.Caps`,
  `State.Packages`, the prestige and pool counts. They draw no random
  number, so the RNG stream of a boot is unchanged. Route offers are logged
  with propensity 1, which the ML offer log reads as a fixed policy.
- **Old saves.** A `State.Rules` with no `Nodes` plays the linear path
  exactly as today; the prototype's engine and sim tests pass unchanged with
  `MaxPrestige` 4. The new state fields are `omitempty`. The new rules
  fields decode as zero from an old snapshot, and zero plays as today.
- **Order.** It builds on the D47 cycles fields (`meta.md` §(i)), which land
  first.
- **Boot #1.** Boot #1 stays the short linear shape. The map starts at
  boot 2, after one whole loop (RT11).
- **Presets.** `short` and `patch` stay linear and one-boss. `Map` is
  `standard`'s replacement.

## (i) Simulator evidence

**Method.** Union pool (USACO + Codeforces, 7,146 judgeable). `sim.Player` is
meta.md's model: `DefaultPlayer` with `GiveUpAt` 0.75, taking exits at
random. Static cells cover boots 51–250 only, before the floor problems run
out. Long horizons use the learning player (ML3, `DefaultLearning`). The
measures are:

- *hazard by act*: crashes in an act over the boots that reached it;
- *door time*: D46's uptime, in-door only;
- *full*: the share of boots cleared.

### 1. Each lever on its own, on the default (P0, 1000 / 1300 / 1600)

Each row changes one thing from `rules.Map()`. Full clears are for the
1000 / 1300 / 1600 players, two seeds, boots 51–250. Two seeds give about
±2.5 points of noise.

| Lever | Full clears | Hazard I / II / III | Door time, mean | Reading |
|---|---|---|---|---|
| **default** | **45 / 45 / 47%** | 10–15 / 20–22 / 32–35% | 2h52–3h07 | |
| fails 10 in every act (no act multiplier) | 49 / 45 / 45% | 11–15 / 17–23 / 30–36% | 2h56–3h08 | within noise |
| final retries at −20 | 61 / 58 / 55% | 13–14 / 19–21 / 14–20% | 3h07–3h17 | the act-III lever |
| no fountain | 46 / 56 / 50% | 12–14 / 17–22 / 24–32% | 2h46–3h00 | noise |
| fountain +10 between acts | 48 / 49 / 48% | 13–15 / 21–22 / 28–30% | 2h55–3h02 | noise |
| fountain +10 in every window | 49 / 55 / 52% | 10–12 / 18–20 / 25–31% | 3h01–3h15 | +3 to +9 |
| kexec once a biome | 58 / 59 / 55% | 8–11 / 14–17 / 23–26% | 2h44–3h06 | the hidden net |
| no fsck | 43 / 44 / 38% | 17–18 / 24–26 / 28–39% | 2h36–2h47 | −2 to −9: fsck is the real heal |
| no skip exit | 46 / 47 / 47% | 13–15 / 19–22 / 30–34% | 2h55–3h10 | noise; the skip is a choice, not a net |
| acts I and II at 2 rooms | 45 / 48 / 44% | 13–16 / 23–25 / 27–32% | 3h24–3h32 | act II's 25%, for +30 min |
| no act III biome | 51 / 55 / 49% | 13–15 / 20–22 / 21–26% | 2h42–2h49 | act III too short to rise |
| no Boss I floor | 45 / 45 / 48% | 14–18 / 17–24 / 30–35% | 2h50–3h15 | noise at P0; matters at P4+ |
| bosses 0.60 / 0.55 / 0.50 | 52 / 55 / 55% | 8–12 / 12–15 / 30–32% | 2h43–2h55 | softer acts I–II |
| Overclock free (cap kept) | 44 / 51 / 52% | 11–17 / 19–22 / 26–35% | 2h59–3h12 | noise |
| Overclock free and compounding (today) | 44 / 49 / 47% | 11–14 / 18–25 / 31–33% | **4h58–5h21** | boot length runs away |
| base clock 1.0 (the 0.85 quantile) | 53 / 50 / 51% | 11–16 / 15–18 / 26–31% | 3h15–3h34 | |
| `timer_mult` 0.9 | 48 / 51 / 47% | 13–15 / 18–22 / 26–33% | 2h25–2h43 | |
| `timer_mult` 0.8 | 51 / 47 / 47% | 15–18 / 16–22 / 26–31% | 1h47–2h08 | G1 rises to 3.2% |
| D47 cycles on the map, soft (`Acts()`) | 72 / 76 / 74% | 6–8 / 9–10 / 9–14% | 3h42–3h55 | fountain +10 everywhere, final retries, kexec a biome, 1–2 rooms |

### 2. The one-shot final is the act-III lever

| Final | Full clears | Act III hazard |
|---|---|---|
| one shot (default) | 45–47% | 32–35% |
| retry at −20 | 55–61% | 14–20% |

### 3. kexec once a biome was the hidden net

kexec's free swap reset in every biome. A seven-tier boot gave it seven
swaps, and the model takes kexec first. Once a biome clears 55–59% of boots
against the default's 45–47%. Once a boot cleared 36–38%, against 50–51%
for once an act, on an earlier base without the skip.

This overstates kexec's value: the model knows whether it can solve a door
before it swaps, and a person only guesses from the statement in 2:30.
Once an act still suits a map three times as long (inference).

### 4. Compounding Overclock made boots run away

| Overclock | Door time, mean | Clear median |
|---|---|---|
| free, compounding, no cap (today) | 4h58–5h21 | 3h57–4h28 |
| 5 cycles, additive, two at most (default) | 2h52–3h07 | 2h44–3h08 |

- **The loop:** more picks meant longer timers, longer timers made -O3
  easier, and -O3 paid more picks.
- **The cap matters for today's shape too.** It cut the one-boss boot's
  mean door time from 2h44 to 2h05, on the D46 clock.
- **The 5-cycle price moves no band measure.** The model then buys fsck
  1.2–1.4 times a boot instead of 0.4–0.6, so time is paid for in bytes,
  through cycles.

### 5. The long horizon: the learning player, a year each

The setup: 30 learning players rated 900 / 1100 / 1300, with sessions. Each
player chooses its level: the highest unlocked one where its last 10 boots
cleared at least 30%, retried 5 boots after it fell short. The baseline is
today's three biomes with D47 cycles (D1), whose ladder stops adding at P4.
Where two numbers are given, they are with churn / without churn.

| A year of play | Today's shape (D1) | Map default |
|---|---|---|
| Durable skill gained | +22 / +37 | +23 / +44 |
| Hours played | 152 / 286 | 177 / 381 |
| Quit for good (churn on) | 73% | 77% |
| Runs of three crashes in a row (churn on) | 14.3 | 29.3 |
| **G1 over the year** | **10.1 / 11.2%** | **2.7 / 3.1%** |
| Full clears over the year | 46 / 42% | 25 / 20% |
| Players at P8 by year end | 23 / 30 of 30 | 15 / 26 of 30 |

What the year shows:

- **G1 holds on the long horizon:** 2.7–3.1% for the map against 10–11%
  for D1. D1's own ladder ends at 20 cycles with Userland fails at full
  price.
- **Skill per hour is about the same.** Durable gain is +1 point with churn
  and +7 without, but the map is played 15–35% more hours. The learning
  simulator cannot separate gains this small (`meta.md` §(d)1).
- **The cost is losing streaks.** They double: 29 against 14 runs of three
  crashes a year. Quits rise from 73% to 77%, which is within the noise of
  30 players (about ±8 points). When every boot plays the top level, the
  streaks reach 37 against 15 and quits 77% against 70%. The cause is the
  band itself: P5–P8 clear 6–22%.
- **The ladder is short.** Without churn, 26 of 30 players reach P8 within
  the year and the rest reach P7. D31's gate (60 rated doors and one clear
  at the top level) opens a level every few boots. Decision 3 in §(j) is
  about this.

### 6. The clock sets length; the aims set the clear rate

| Clock (× the 0.85 quantile) | Full clears | Door time, mean | The final's clear rate a door |
|---|---|---|---|
| 1.0 | 50–53% | 3h15–3h34 | 47–51% |
| 0.8 (default) | 45–47% | 2h52–3h07 | 45–47% |
| 0.8 and `timer_mult` 0.9 | 47–51% | 2h25–2h43 | 46–51% |
| 0.8 and `timer_mult` 0.8 | 47–51% | 1h47–2h08 | 50–54% |

From the first row to the last, the clock falls by a third. Door time falls
by 1h30, and full clears move by 6 points at most.

## (j) Decisions for the user

Each has a default. They are independent.

1. **The final boss at every prestige.**
   - Default: The Halting Problem ends every boot, as the brief says. The
     storyteller adds a ring-3 Jabberwock beat (§(f)).
   - Alternative: Undefined Behavior ends P0–P3 and The Halting Problem
     takes over at P4. That keeps the story bible's ring −1 reveal as
     written.
2. **The band and the map's length.**
   - Default: accept where it lands. P0 clears 45–47%, the hazard by act is
     10–15 / 20–22 / 32–35%, P4 clears 21–22% and P8 7–10%. The map is
     one room a topic biome: about nine doors and three hours.
   - Act II's hazard is under the band's 25%. Acts I and II at 2 rooms
     reach 23–25%, for 30 more minutes a boot.
3. **The ladder's speed.**
   - Default: keep D31's gate (60 rated doors and a clear at the top level).
   - Under it, 26 of 30 learning players reach P8 within a year (without churn).
     They then play mostly at P5–P8, where clears are 7–23%, and that is
     where the doubled losing streaks come from (§(i)5).
   - Alternative: two clears at a level to unlock the next. This is
     untested.
4. **kexec once an act.** Default: yes. Once a biome is 8–14 points softer
   at P0 (§(i)3).
5. **Overclock's price.**
   - Default: 5 cycles a pick, +15% each, two at most.
   - Alternative: doors pay ×0.9 a stack, the way `nice` prices time in
     bytes. This is untested.
6. **The fountain.** Default: +5 between acts, gone from P5. Alternative:
   none, so fsck is the only heal. Either way the band moves less than the
   noise (§(e)).
7. **Boss aims.**
   - Default: 0.55 / 0.45 / 0.50. Boss II is the hardest door, and the
     final is the only one-shot.
   - Alternative: rising aims, 0.60 / 0.55 / 0.50. Full clears rise 5–10
     points, and act II's hazard falls to 12–15%.

Resolved without asking (engineering, not taste):

- -O3 ignores bought time.
- Prestige timer cuts skip Userland, whose clock is set per level.
- The fountain heals up to the starting 30, not to 50.
- Exits read a capability snapshot taken at boot start.
- Boot #1 stays linear.
- git's `route_offers` becomes the Bisect Bridge key.

## (k) Pitch (for the coordinator to show the user)

> Three acts on a Dead Cells map. After Userland you pick a topic biome,
> and the exit you take picks the boss: Off-by-One or the OOM Killer. Act
> II branches the same way, to Heisenbug, Race Condition or (from P1)
> Undefined Behavior. Act III is one more room, or a skip for two upgrade
> picks, and then The Halting Problem, one shot. Every exit shows its
> topic, the boss behind it and its lock. A locked exit names the RTFM
> trial that opens it, so learning a topic opens a route you have already
> seen.
>
> It is trial by fire. About 45% of P0 boots clear, and deaths come late:
> about 13% in act I, 21% in act II and 33% in act III. Userland almost
> never kills you. P4 clears about 20% and P8 about 10%. A full boot is
> about nine doors and three hours of door time, and you can stop between
> any two doors.
>
> Time is always bought. Overclock now costs 5 cycles and stops at +30%;
> free and compounding, it stretched boots to five hours. nice costs pay,
> at costs bytes, and ECC trades a WA's cycles for clock.
>
> One surprise: tightening the clock does not make boots deadlier. The game
> answers with easier doors. Harder boots come from costs and nets. kexec
> was the hidden net, so it now swaps once an act.
>
> The price is losing streaks: over a simulated year they double. Seven
> calls are yours, and each has a default.

## (l) The prototype (evidence, not a merge)

Workspace `map-design`, its own commit on top of this document's, which
also carries meta.md's D47 flags (`HPPerLife`, `SubmitHP`, `FailAdvance` and
the rest).

- `internal/rules/acts.go`: `Acts()` (meta D numbers on the map), `Map()`
  (the default) and `MapAscensions()`.
- `internal/rules/rules.go`: the fields in §(h).
- `internal/game/mapgraph.go`: layout, exits, gates, fountain, skip, -O3 base.
- Small hooks in `engine.go`, `doors.go`, `ease.go` and `timer.go`.
- `internal/game/sim/sim.go`: route policies, retreat lock, map counters,
  and `ClimbFloor`.
- `internal/game/sim/map_test.go`: every configuration above.

Shortcuts the real build should not keep:

- Exits are node IDs in `BiomeState.Offers`, and locked exits are not
  listed. The contract's `Exits` replaces both.
- Exits read `Meta.Capabilities` live, not a boot-start snapshot.
- No screen, story or store code is touched.

Every existing engine, sim and rules test passes with `MaxPrestige` 4. With 8,
`TestFullClear` and the doors golden fail, as expected.

```
KATHARSIS_MAP=1 KATHARSIS_MAP_PARTS=p0,levels KATHARSIS_MAP_LEVELS="Z map" \
KATHARSIS_MAP_ONLY="Z" KATHARSIS_MAP_BOOTS=200 [KATHARSIS_MAP_TIERS=1] \
go test ./internal/game/sim/ -run TestMapReport -v -timeout 60m
KATHARSIS_MAP=1 KATHARSIS_MAP_PARTS=learn KATHARSIS_MAP_PLAYERS=30 \
[KATHARSIS_MAP_CLIMB=0.3] go test ./internal/game/sim/ -run TestMapLearn -v
```

## (m) What remains uncertain, and sources

- **The model's clock is not the player's.** Door times are the model's
  (30 min median at gap 0). Boot #1's two real clears took 6:04 and 6:42.
  Read boot length as ±30 min until alpha boots give real times.
- **The model picks exits at random.** "long", "short" and "gated" policies
  moved full clears by 2–6 points. Real players will pick by topic.
- **kexec and retreat are overvalued**, because the model knows its chance
  up front.
- **Noise:** ±3.5 points a 200-boot cell, ±2.5 for two seeds.
- **Dead Cells' counts** came through a summarising fetcher (the wiki blocks
  scripts). Two conflicts were noted: late Dracula's Castle access, and the
  Distillery's enemy tier.

**Dead Cells sources** (industry unless marked):

- [BIO] https://deadcells.wiki.gg/wiki/Biomes
- [BSC] https://deadcells.wiki.gg/wiki/Boss_Stem_Cells
- [PAS] https://deadcells.wiki.gg/wiki/Passages
- [CEL] https://deadcells.wiki.gg/wiki/Cells
- [RUN] https://deadcells.wiki.gg/wiki/Runes
- [KEY] https://deadcells.wiki.gg/wiki/Key
- [CC] https://deadcells.wiki.gg/wiki/Cursed_chest
- [GH] https://deadcells.wiki.gg/wiki/Guardian%27s_Haven
- [TD] https://deadcells.wiki.gg/wiki/Timed_Door
- [NL35] https://nintendolife.com/news/2024/08/dead-cells-final-update-is-now-live-here-are-the-full-patch-notes
- [U25] https://mp1st.com/title-updates-and-patches/dead-cells-update-1-27-out-for-practice-makes-perfect-patch-notes-listed
- [ITCH] https://motiontwin.itch.io/dead-cells
- [DN] https://deepnight.net/tutorial/the-level-design-of-dead-cells-a-hybrid-approach/
- [PCGI] https://www.pcgamesinsider.biz/indie-interview/66988/how-motion-twin-stopped-dead-cells-being-just-another-roguelike-pixel-art-metroidvania-game/
- [80LV] https://80.lv/articles/interview-with-the-developers-of-dead-cells
- [STEAM] https://store.steampowered.com/app/588650/Dead_Cells/
- [TWIN] https://twinfinite.net/guides/dead-cells-long-levels-many-how/ (secondary)
