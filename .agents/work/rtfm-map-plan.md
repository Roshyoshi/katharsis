# RTFM world map: plan

Ask: "we should add a map sorta for RTFM ... can be like the new super mario
maps and like there can be levels and stuff inside ig".

Status: plan only. Nothing in ~/katharsis was changed. Snapshot read:
`~/Library/Caches/katharsis/guide/092bf547…/content` (commit 092bf547).
Scratch joins used for the counts: `/Users/roshanhegde/.claude/jobs/e2124f03/tmp/w/`
(`mods.txt` = division, chapter, id, topic, capability, playable, all problems;
`pre.txt` = prerequisites).

**Verdict up front.** The hypothesis holds: divisions are worlds, chapters
are regions and modules are levels. Two corrections change the design:

1. **Capabilities belong to topics, not modules.** 83 modules grant one of
   11 capabilities, so a single trial turns every □ of that topic to ■
   across all worlds. That is SMW's switch palace (one clear changes the
   whole world), not a castle. Castles should be the four `*-conclusion`
   modules.
2. **61 of 144 visible levels have no trial today.** `app/learn.go:155`
   refuses a trial that grants nothing. A Mario map where 42% of the
   nodes can never be cleared reads as broken. The map needs the open item
   the workfile already lists, "practice trials for the 33 capability-less
   modules" (`.agents/work/katharsis.md:442`), or it needs practice solves
   to count as the clear. See Q1.

---

## 1. RTFM today

### Data (internal/curriculum)

| Thing | Where | Facts |
|---|---|---|
| Division → Chapter → ModuleRef | `curriculum.go:27-45` | from `content/ordering.ts` (Section → Chapter → Module) |
| Module | `curriculum.go:50-75` | `Prerequisites []string` (ids or external text), `Frequency` 0-4 (-1 unset), `Topic`, `Problems []ProblemList`, `Meta` (pages about the guide itself, hidden) |
| Trial | `trial.go:19-46` | Unit = up to 4 module problems in round-robin by difficulty label (`trial.go:79`), plus 2 blind integration doors by topic and `DivisionBand`; `Capability = CapabilityFor(topic)` |
| Topic → capability | `game/capability.go:10-22` | 11 capabilities; complete-search, simulation, sorting-greedy, math and mixed are ungated |
| Problem refs | `problems.go:22-37` | only `usaco-N` and `cses-N` map to a catalog ID; everything else is an external link |

### Real counts from the snapshot

| World | Chapters | Modules (visible) | Grant a capability | Have a playable problem | Full 4-door unit | Playable refs | All refs | Distinct capabilities taught |
|---|---|---|---|---|---|---|---|---|
| General | 5 (4 visible) | 29 (24) | 0 | 4 | 1 | 9 | 14 | 0 |
| Bronze | 5 | 13 | 4 | 11 | 11 | 98 | 121 | 3 (DS, GEOM, GRAPH) |
| Silver | 5 | 14 | 10 | 14 | 13 | 119 | 264 | 6 |
| Gold | 7 | 29 | 24 | 28 | 23 | 194 | 438 | 8 |
| Platinum | 5 | 23 | 19 | 19 | 12 | 91 | 307 | 4 |
| Advanced | 8 | 41 | 26 | 25 | 4 | 55 | 398 | 7 |
| **Total** | 35 | 149 (144) | 83 | 101 | 64 | 566 | 1542 | **11** |

Notes on the counts:
- `TestFullGuide` (`full_test.go:105-111`) agrees: 149 modules, 0 conversion
  failures, 335 distinct USACO and 185 distinct CSES problems, 977 external
  refs, and 64 full 4-door units counting USACO and CSES.
- Chapter sizes (`ordering.ts`):

  | World | Chapter sizes |
  |---|---|
  | Bronze | 3/2/2/5/1 |
  | Silver | 2/6/4/1/1 |
  | Gold | 3/7/5/5/4/4/1 |
  | Platinum | 7/6/4/5/1 |
  | Advanced | 6/3/8/3/4/2/3/12 |

  Each of Bronze through Platinum ends with a 1-module "Conclusion" chapter.
- **Every capability can be earned by the end of Gold.** Bronze already
  grants DS (`intro-ds`, `intro-sets`), GRAPH (`intro-graphs`) and GEOM
  (`rect-geo`).
- **Prerequisites form a DAG that guide order already sorts.**
  - 147 internal edges: 72 within a division and 75 across divisions, plus
    3 external text prerequisites.
  - 0 edges point to a later division. Only 1 points later in the order
    (the meta `contributing` module).
  - 18 edges cross chapters inside a division.
  - So guide order is a valid topological order, and a linear spine never
    asks for something you have not reached.
- **Real "Star Road" edges** (Platinum or Advanced modules whose
  prerequisites all sit in Silver or earlier):
  - Silver `graph-traversal` → Adv `eulerian-tours`, `string-search`.
  - Silver `intro-tree` → Plat `centroid`.
  - Ten Plat/Adv modules list no prerequisites: `binary-jump`, `sqrt`,
    `DC-SRQ`, `geo-pri`, `fft`, `game-theory`, `random`, `interactive`,
    `vectorization`, `xor-basis`.
- **Optional modules.** 6 titles say "(Optional)": 5 in Gold, 1 in
  Platinum. `Frequency` 0 ("has not appeared") applies to 16 Advanced and
  2 Platinum modules.

### Screens and flows

| Screen | Anchor | What it shows | Keys |
|---|---|---|---|
| Browser (man index) | `screens/rtfm/browser.go:24,582` | divisions → chapters → modules; columns MODULE, BIOME, CAPABILITY (■/□), TRIAL (unit size), frequency ★; a division line shows `■ held/total` (`browser.go:674`) | j/k, enter/l open, tab fold, `/` filter, -/+ fold all, q |
| Module page | `screens/rtfm/module.go` | NAME, TRIAL (unit doors with label and rating, top-ups, blind doors, pass line), SEE ALSO (prerequisites, authors), attribution | r/enter read, t trial (only if `Capability != ""`), o open URL, q/h back |
| Reader | `app/learn.go:124` | module markdown | — |

Flows:
- **Enter RTFM.** `app/learn.go:39` openLearn: the Browser opens at a
  module, or at the topic's module nearest the player's band
  (`learnModule`, `learn.go:69`). The hub's `t RTFM` key and the route's
  locked exits link here (`app/boot.go:63` `Module: m.rtfmModule`).
- **Start a trial.** `learn.go:150` → `game.NewTrial` (`game/trial.go:11`)
  on `rules.Trial()` (`rules/rules.go:1035`): unit 4 rooms, integration 2,
  30 min, legacy lives.
- **On a clear.** The trial grants the capability and appends
  `Meta.TrialsPassed` (`game/trial.go:223`). `trialNext` (`learn.go:92`)
  offers the next module that grants a capability you lack.

### Progress data RTFM could show

| Fact | Source | Read by a UI today? |
|---|---|---|
| Capabilities held | `Meta.Capabilities` (`game/types.go:87`) | yes: ■/□ |
| Trials passed, by module | `Meta.TrialsPassed` (`types.go:119`) | **no: written only** (`trial.go:223`). The map would be its first reader |
| Problems solved | `Meta.Solves` (`types.go:95`): Problem, Kind, Trial/Upsolve flags | not in RTFM |
| Module opened or read | **nowhere**: no Meta field, no store event (`store/event.go:35-54`) | — |
| Setuid tie-in | P16 `stamped` = a capability (`achievements.md:274`); `setuid.Completion` counts capabilities (`setuid/completion.go:72`) | yes |

**An untimed practice door already exists.** `screens.Upsolve{Problem}` →
`startUpsolve` (`app/intents.go:268`) opens any catalog problem outside a
boot and records a `Solve{Upsolve: true}`. It pays bytes only to recover a
core dump (`app/debrief.go:271`), so module practice can use it without
adding a byte reward (GR4).

### Constraints that bind the design

- **Glyphs, as the code has them** (`ui/glyphs.go`):

  | Glyph | ASCII | Meaning |
  |---|---|---|
  | ✓ | `+` | pass |
  | ○ | `.` | ahead |
  | ● | `@` | here |
  | ⚔ | `X` | boss |
  | ■ / □ | `[x]` / `[ ]` | capability held / locked |
  | ▸ | `>` | cursor |
  | ★ / ☆ | `*` / `.` | achievement, frequency level |
  | ▓ / ░ | `#` / `.` | bar fill / empty |
  | `─` | `-` | horizontal rule |
- **The code and `ui-system.md:466` disagree on the capability glyph.** The
  doc says ◆/◇; the code uses ■/□, because D27 kept ◆ for ratings. Follow
  the code.
- **No path-line glyphs exist.** The boot hub draws its map as columns
  without connectors (`testdata/hub_140x40.golden`).
- **Taken in reader content:** ◇, ≡ and ↗ appear in `reader/tex.go` (TeX
  math). That is content, not UI chrome, but avoid them for map meanings.
- **Rules R1 and R7** (`ui-system.md:378-384`): every state needs a glyph or
  word besides colour, and one hue has one meaning.
- **Goldens.** Both sizes are golden-tested (`rtfm/testdata/*_{80x24,140x40}.golden`).
  ASCII goldens exist in boot, meta and story tests but not in rtfm.
- **Guardrails:** GR1 no streaks; GR2 no date pressure; GR3 no fabrication;
  GR4 bytes only for learning acts; GR7 no engagement steering; GR8 no
  guilt; GR10 no debt counters (`docs/research/retention.md:1189-1200`).

---

## 2. Design principles from the research

A subagent did the research with WebSearch and WebFetch. Its uncertainties
are marked.

| # | Principle | Source | Implies here |
|---|---|---|---|
| P1 | **A spine with free pockets.** Wonder is path-and-node with "open areas" where you choose the order, so a stuck player plays something else. Duolingo's 2022 linear path cut choice paralysis but was criticised for removing choice | mariowiki.com/Super_Mario_Bros._Wonder; blog.duolingo.com/new-duolingo-home-screen-design; nbcnews (von Ahn interview) | Guide order is the spine (it is a valid topological order, §1). Chapters are pockets: any level is enterable. Nothing locks reading |
| P2 | **Gate on a count, not on one level.** Wonder Piranhas want N seeds from anywhere | mariowiki.com/Wonder_Seed (the exact per-Piranha counts were not verified) | If anything gates, gate the castle on "k trials in this world", never on one module. Recommend no gates (Q2) |
| P3 | **Say a secret exists, never where.** SMW's red dot; Wonder's "all collectibles and no check means a secret exit" | mariowiki.com/Secret_exit; nintendo.com Wonder tips | A ↗ under a node whose clear is the last prerequisite of a later-world module. Derived from real edges (GR3) |
| P4 | **Secrets pay as shortcuts.** Star Road, warp whistles, the Special World | mariowiki.com/Warp_Whistle; mariopartylegacy Special World | A warp moves the cursor to that later-world level. The Advanced world is the "Special World" |
| P5 | **One clear changes the whole world.** Switch palaces turn on blocks everywhere; the SMB3 hammer opens paths | mariowiki.com/Super_Mario_World, /Super_Mario_Bros._3 | A capability trial flips every □ of its topic to ■ across all worlds and opens boot-map exits. Show that moment |
| P6 | **Layered completion per node.** Clear → gold flag → green check → world check → save-file medals and SMW's 96 | gamereactor.eu Wonder medals; en.wikibooks.org/wiki/Super_Mario_World | Per level: page opened, practice k/n, trial ✓, capability ■. Per world: counts with denominators. No single % on the map (Wonder shows none either; that was the subagent's finding) |
| P7 | **Milestones survive failure.** SMB3 locked doors stay open after a Game Over | mariowiki.com/Locked_door | Capabilities and trials passed never expire. Nothing decays (GR2, GR10) |
| P8 | **Practice off the hot path costs nothing.** Badge Challenges cost no lives | mariowiki Wonder | Practice uses the untimed upsolve door: no lives, no clock, no bytes |
| P9 | **Difficulty on the node.** Wonder rates every course with 1-5 stars | mariowiki Wonder | The card shows the trial band (`*1300–1600` rank colour) and the guide's frequency stars. Labels inside a module are relative (`problems.go:31`), so only bands go on the map |
| P10 | **Name nodes by the skill.** Duolingo: "get directions", not "City 3" | duolingo blog | Labels are module titles, never "S-4" |
| P11 | **Hide a post-game.** Special World and Special Zone | mariowiki SMW, Wonder | Advanced is always open but sits past the Platinum castle. The Platinum castle reveals nothing that does not exist |

**Uncertain:** whether the SMW path-drawing animation is sourced (it is the
subagent's memory), Duolingo's "jump here" feature, Khan Academy's mastery
rule (sources conflict), and Brilliant (not researched).

---

## 3. The design

### 3.1 Structure (from the real data, nothing invented)

```
Overworld (the RTFM hub)
├─ General        library: 24 page nodes in 4 shelves, no trials (side island)
├─ 1 Bronze       5 regions · 13 levels · castle bronze-conclusion
├─ 2 Silver       5 regions · 14 levels · castle silver-conclusion
├─ 3 Gold         7 regions · 29 levels · castle gold-conclusion
├─ 4 Platinum     5 regions · 23 levels · castle plat-conclusion
└─ 5 Advanced     8 regions · 41 levels · no castle (the Special World)
```

### 3.2 Node types (each has its own glyph, per R1)

| Node | Which modules | Glyph (Unicode / ASCII) | When cleared |
|---|---|---|---|
| trial level | the module has a trial (today: it grants a capability; with Q1, any module with ≥ 1 playable problem) | `○` / `.` → `✓` / `+` | `TrialsPassed ∋ id` |
| page | no playable problem and no trial (mostly General) | new `Page`: `▪` / `=` | never "cleared"; the card says "opened" when Q4 lands (no inflation) |
| castle | `*-conclusion` (4) | `⚔` / `X` → `✓` | its trial passed (Q1/Q3). SMW's ruined castle becomes ✓ |
| capability mark (under a node) | the topic is gated | `□` / `[ ]` → `■` / `[x]` | `Capabilities ∋ cap` (it flips everywhere at once) |
| warp mark (under a node) | real cross-world edges (§1) | new `Warp`: `↑` / `^` | shown from the start, because it names a fact about the guide. `w` on the node jumps to the target |
| optional | "(Optional)" titles and Frequency 0 | path drawn `muted`, dotted `┄` / `:` | same as its type |
| cursor | — | `▸` / `>` before the node | — |

**Why ▸ and not ●.** "Here" is the boot's room marker (`bootmap.go:37`).
On this map the cursor must keep the node's own state glyph visible, and ▸
is already the only cursor glyph (`ui-system.md:470`).

### 3.3 Unlock and secret rules

| Rule | Decision | Why |
|---|---|---|
| Can I enter any level? | **Yes, always.** Reading and practice are never locked | The guide is free reading. Locking it fights the player's agency, and D37's self-reported rating and `learnModule` already send a strong player to Gold |
| Path "lit" | A path segment is solid when the node before it on the spine is cleared, muted when not | reveal by clears (SMW) without blocking |
| Prerequisites | Shown for the **selected** node: its prerequisite nodes get the focus colour on the map, and the card lists them with their state in words (R1) | 147 edges cannot be drawn legibly in 80 columns. Highlighting the selection's edges stays honest |
| Castle | Enterable any time. The card shows the world's count ("trials 3/13") but gates nothing (Q2) | count gates are the motivating part of Wonder; a hard lock would be a debt framing |
| Secret (warp) | ↑ under Silver `graph-traversal` (→ Adv `eulerian-tours`, `string-search`) and `intro-tree` (→ Plat `centroid`); the 10 prerequisite-free Plat/Adv modules appear as ↑ targets from the overworld | real edges only (GR3) |
| Special World | Advanced is open from the start, and the overworld draws it after Platinum on a dotted path | P11 without hiding content |
| Switch-palace moment | when a trial grants a capability and RTFM is next shown, the □ → ■ flips animate once across the visible world (`internal/anim`, reduced motion = an instant swap) and a flash names the boot-map exit it opened (from `map.md` §c: e.g. "CAP_BSEARCH: Bisect Bridge is on your routes") | P5. The text exists today as `passLine` (`module.go:255`) |

### 3.4 Inside a level (enter on a node)

Three panels in Wonder's seed shape: **READ · PRACTICE · TRIAL**.

- **READ** opens the reader (today's `r`, `OpenModule`). With Q4 it also
  records "opened".
- **PRACTICE** lists the module's playable problems: title, guide label,
  `*rating`, ✓ if `Meta.Solves` holds that ID from any context. Enter opens
  an untimed door through `screens.Upsolve`. External problems are listed
  muted with `o` to open the URL.
- **TRIAL** is today's TRIAL section unchanged (`module.go:134-146`): the
  unit, top-ups, 2 blind doors, and the pass line.
- **Footer:** SEE ALSO (prerequisites with their states) and the CC BY-NC-SA
  attribution (required, `curriculum.go:9`).

### 3.5 Completion and rewards (no bytes)

| Level | Shown | Data |
|---|---|---|
| node | glyph ○/✓, ■/□, ↑ | TrialsPassed, Capabilities |
| card | practice `k/n solved` (n = playable distinct), trial ✓, frequency stars | Solves ∩ module IDs |
| region | `3/6` after the region label at 140 cols | derived |
| world | `trials k/n · ■ h/t · solved k/n` in the header, a ▓░ bar on the overworld | derived; denominators are real counts |
| overworld | `■ h/11 held` | Capabilities |
| meta | P16 `stamped` and the capabilities part of `setuid.Completion` stay as they are. New setuid bits (e.g. a castle cleared, all of a world's trials) are Q5 | — |

Rewards are the capability (it opens boot-map exits) and the ✓ marks. RTFM
pays no bytes (GR4: trials keep their door economy unchanged). Nothing
counts days (GR1, GR2). There is no "N left to do" debt line, only "k of n"
(GR10).

---

## 4. Mockups

Glyph style follows the code. Numbers are illustrative of a mid-Silver
player; the layout is the real Silver data.

### 4.1 Overworld, 80x24

```
RTFM(7)                  USACO Guide · CC BY-NC-SA 4.0                   RTFM(7)
────────────────────────────────────────────────────────────────────────────────

      ▪  General     24 pages · library                       opened  6/24
      │
      ✓  Bronze      13 levels  ▓▓▓▓▓▓▓▓▓░░░   9/12 trials  ⚔ ✓   ■ 3/3
      │
    ▸ ○  Silver      14 levels  ▓▓▓░░░░░░░░░   3/13 trials  ⚔     ■ 2/6
      │                ↑ 3 levels past Gold open from here
      ○  Gold        29 levels  ░░░░░░░░░░░░   0/28 trials  ⚔     ■ 0/8
      │
      ○  Platinum    23 levels  ░░░░░░░░░░░░   0/19 trials  ⚔     ■ 0/4
      ┆
      ○  Advanced    41 levels  ░░░░░░░░░░░░   0/25 trials        ■ 0/7
                     the special world · 10 levels need nothing first


  ■ 5/11 capabilities held · each opens topic exits on the boot map
────────────────────────────────────────────────────────────────────────────────
 ↑/↓ world · enter open · i index · / find · q quit
```

### 4.2 World, 80x24 (Silver, serpentine along guide order)

Row 1 runs left to right. Row 2 comes back right to left, the way Mario's
paths fold on screen.

```
RTFM › Silver                          trials 3/13 · ■ 2/6 · solved 18/119
────────────────────────────────────────────────────────────────────────────────
  Prefix Sums       Sorting & Searching
  ✓ ──── ○ ──────── ✓ ── ▸ ○ ──── ○ ──── ✓ ──── ○ ──── ○ ─┐
  ■      ■          ■      □      □                    ■   │
                                                           │
     ⚔ ──── ○ ──────── ○ ──── ○ ──── ○ ──── ○ ─────────────┘
                       □      □ ↑    □      □ ↑
  Conclusion Additional  Graphs ◂ (read right to left)

╭─ Binary Search on a Sorted Array ──────────── Silver › Sorting & Searching ─╮
│ □ CAP_BSEARCH → Bisect Bridge · *1100–1400 · ★★★☆ somewhat frequent         │
│ needs  Introduction to Sorting (Bronze) · practice 3/4 ✓                    │
│ read · practice 1/3 solved · trial 4 doors + 2 blind                        │
╰─────────────────────────────────────────────────────────────────────────────╯
 ←/→ path · enter level · r read · t trial · w warp · tab world · i index · q
```

A mark row holds ■/□ and ↑ only, each followed by a space
(`ui-system.md:478`). The `◂` is annotation for this plan, not UI.

Graphs reads right to left in row 2:
- `graph-traversal` (□ CAP_GRAPH, ↑ to 2 Advanced levels)
- `flood-fill` (□)
- `intro-tree` (□ CAP_TREE, ↑ to Plat `centroid`)
- `func-graphs` (□)

`intro-bitwise` (math, ungated) is a trial level only if Q1 lands;
otherwise it is a page node `▪`.

Fit check for the largest world (Advanced, 41 nodes, 8 regions):
- At 80 columns a slot is 7 cells, so a row holds 10-11 nodes and Advanced
  needs 4 rows of 3 lines each = 12 lines.
- Header 2 + map 12 + card 5 + footer 1 = 20 lines, which fits 24.
- Regions longer than a row (Misc. Topics, 12) wrap onto the next row with
  their label repeated in `muted`.

### 4.3 World, 140x40 (Silver)

Labels sit under the nodes, and the card moves into a right-hand panel.

```
RTFM › Silver                                                                         trials 3/13 · ■ 2/6 · solved 18/119 · 5 regions
────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────

  PREFIX SUMS  1/2          SORTING & SEARCHING  1/6                                      ╭─ Binary Search on a Sorted Array ──────────╮
                                                                                          │ Silver › Sorting & Searching               │
  ✓ ────────── ○ ────────── ✓ ──────── ▸ ○ ────────── ○ ────────── ✓ ────────── ○ ──┐   │                                            │
  ■            ■            ■            □            □                         ■     │   │ □ CAP_BSEARCH → Bisect Bridge joins routes │
  Prefix Sums  More Prefix  Two Pointer  Binary Sea…  Binary Sea…  Custom Com…  Greedy  │   │ *1100–1400 · ★★★☆ somewhat frequent        │
                                                                                      │   │                                            │
                                                                       ○ ─────────────┘   │ NEEDS                                      │
                                                                       ■                  │   Introduction to Sorting (Bronze)         │
                                                                       Priority Q…        │     practice 3/4 ✓ · no trial              │
                                                                       │                  │                                            │
  GRAPHS  0/4                                                          │                  │ READ        opens the module in the reader │
                                                                       │                  │ PRACTICE    1/3 solved                     │
  ○ ────────── ○ ────────── ○ ────────── ○ ────────────────────────────┘                  │ TRIAL       4 doors + 2 blind, 30:00 each  │
  □ ↑          □            □ ↑          □                                                │             pass → ■ CAP_BSEARCH           │
  Graph Trav…  Flood Fill   Intro Tree   Functional…                                      │                                            │
  │                                                                                       │ ↑ none from here                           │
  │   ADDITIONAL  0/1        CONCLUSION                                                   ╰────────────────────────────────────────────╯
  └── ○ ────────────────────── ⚔
                               trial: 7 USACO problems
      Intro Bitwise            Silver Conclusion





────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
 ←/→ path · ↑/↓ row · enter level · r read · t trial · p practice · w warp · tab next world · shift+tab previous · i index · q back
```

### 4.4 Inside a level, 80x24

```
RTFM › Silver › Graphs › Graph Traversal                          □ CAP_GRAPH
────────────────────────────────────────────────────────────────────────────────
 Graph Traversal  ★★★★ very frequent · *1100–1400 · Routing Table
 Traversing a graph with depth first search and breadth first search.

 ▸ READ      the module, in the reader                              not opened
   PRACTICE  4/14 solved
              ✓ Building Roads         CSES     Easy      *1200
              ✓ Moocast                Silver   Easy      *1400
                Message Route          CSES     Easy      *1300
                Wormhole Sort          Silver   Medium    *1600   +10 more
   TRIAL     4 doors (Easy → Hard) + 2 blind · pass → ■ CAP_GRAPH
              Routing Table joins your route choices in act III

 ↑ also opens: Eulerian Tours, String Searching (Advanced)
 needs  Intro to Graphs (Bronze) ✓ · Complete Search with Recursion (Bronze) ✓
 “Graph Traversal” by …, USACO Guide (https://usaco.guide/silver/graph-…
────────────────────────────────────────────────────────────────────────────────
 ↑/↓ choose · enter go · r read · p practice · t trial · o usaco.guide · q map
```

ASCII skin: identical layout with `+ . X [x] [ ] > ^ = :`, `-` and `|`
paths, `+` corners, and `*` stars from `ui.ASCII`. It gets its own goldens.

---

## 5. Data model and contract

```
internal/curriculum   (pure, reads the snapshot)          NEW: atlas.go
  type Atlas  struct{ Worlds []World }
  type World  struct{ ID, Title string; Regions []Region; Castle string }
  type Region struct{ Title string; Levels []Level }
  type Level  struct{
      Module   string        // id
      Kind     LevelKind     // TrialLevel | PageLevel | CastleLevel
      Optional bool          // "(Optional)" title or Frequency == 0
      Warps    []string      // later-world modules whose last internal prereq is this
      Prereqs  []string      // internal ids only (externals stay on the page)
      Playable []catalog.ID  // distinct judgeable ids, for practice k/n
  }
  func (c *Curriculum) Atlas(judgeable func(catalog.ID) bool) Atlas
  type Progress …; func (a Atlas) Progress(m game.Meta) Progress
      // per level: Passed, CapHeld, Solved k/n; per world and region: folds
      // curriculum already imports game (trial.go:8), so no new edge

internal/game          unchanged, apart from (Q4) Meta.Opened []string `json:",omitempty"`
                       and (Q1/Q3) nothing: NewTrial already takes Capability ""

internal/ui            glyphs.go: Page ▪/=, Warp ↑/^, path runes PathH ─/-,
                       PathV │/|, PathDot ┄/:, corners from G.Thin

internal/screens/rtfm  NEW: worldmap.go (Overworld and World), level.go
                       Env{Atlas, Progress, Caps}; View(w,h) is a pure function
                       of env and cursor. Intents: OpenModule, StartTrial,
                       Upsolve{Problem}, reader.OpenURL, OpenIndex, Back.
                       Browser and Module stay (index on `i`; Module page
                       becomes level.go's TRIAL panel source)

internal/app           learn.go: openLearn pushes the World screen at the
                       module (same Open(id) contract as Browser.Open), index
                       one key away; startTrial allows Castle and Q1 kinds;
                       records Opened (Q4); a granted capability queues the
                       switch-palace flash
```

| Shown | Derived or stored |
|---|---|
| worlds, regions, levels, kinds, warps, prerequisites | derived from the snapshot (Atlas) |
| trial ✓, ■/□ | derived from Meta (`TrialsPassed`, `Capabilities`) |
| practice k/n | derived from `Meta.Solves` ∩ `Level.Playable` (a boot solve counts: it is solved) |
| "opened" | **new**, only with Q4: `Meta.Opened` (Meta migrates by JSON omitempty, no schema change, as D39 did for Seen) |
| cursor and last world | app memory per session, or `localStorage` style settings: not stored (Q6) |

Contract tests: `Atlas` over the fixture and over `TestFullGuide`:
- 6 worlds, 144 visible levels, and 4 castles.
- The warp set equals {graph-traversal → eulerian-tours, string-search;
  intro-tree → centroid}.
- No level's prerequisite comes later in guide order.

---

## 6. Phases

| # | Phase | Size | Tests |
|---|---|---|---|
| 1 | `curriculum.Atlas` + `Progress` (pure) | S | fixture unit tests; `TestFullGuide` asserts the §1 counts and the warp set; Progress on a fixture Meta (TrialsPassed, Caps, Solves incl. a boot solve) |
| 2 | ui glyphs (Page, Warp, Path*) + ASCII pairs | S | glyph width test (each 1 cell); `ui-system.md` §2.4 rows added; fix its ◆/◇ vs ■/□ drift |
| 3 | World screen: serpentine layout, card, cursor, region wrap | M-L | goldens at 80x24 and 140x40 for Bronze, Silver and Advanced (largest), in Unicode and ASCII; `TestFits` over every world of the full guide at 80x24 (no line > w, no row overflow); key tests (←/→ along the spine, ↑/↓ rows, w warp) |
| 4 | Overworld + app wiring (openLearn → map, `i` index, hub `t` and locked-exit links land on the node) | S | goldens; app tests: a locked route exit opens the map on the granting module; q returns |
| 5 | Level screen with PRACTICE via `screens.Upsolve` | M | golden; intents test (enter on a problem emits Upsolve); app test: a practice AC records `Solve{Upsolve:true}`, banks 0 B without a core dump (GR4) |
| 6 | Castles: conclusion trials (mixed, division band, grant nothing) | S | curriculum: castle trial spec; app: startTrial accepts castles; game: TrialsPassed gets the conclusion |
| 7 | Q1: trials for capability-less modules with ≥ 1 playable problem | S | trial specs for the 33; `module.go` pass line "grants no capability, ✓ only" |
| 8 | Switch-palace moment (anim + flash naming the boot exit) | S | anim golden first/last frame; reduced motion = instant; shown once per grant |
| 9 | (Q4) `Meta.Opened` and (Q5) setuid bits | S | old-save load test; setuid table tests; backfill from TrialsPassed |

Lane: one UI owner for `screens/rtfm` plus `app/learn.go`, with phase 1
contract first (D30 workspace per agent). Phases 6 and 7 touch app and
curriculum only.

---

## 7. Open taste questions (each has a recommended default)

| # | Question | Default |
|---|---|---|
| Q1 | Give the 33 capability-less modules (and the 4 conclusions) a trial that grants only ✓? Without it, 61 of 144 nodes cannot be cleared | **Yes**, for modules with ≥ 1 playable problem. General stays page-only |
| Q2 | Hard locks on levels or castles (Wonder Piranhas)? | **No locks.** Prerequisites light up but never block; castles show "k/n trials" only |
| Q3 | Castle glyph: ⚔ (the existing boss glyph; a conclusion trial is a division-wide gauntlet) or a new flag ⚑? | **⚔**: it already exists, is width-tested, and the meaning is close. ⚑ risks emoji width on some fonts |
| Q4 | Track "opened" for page nodes? It is weak evidence of reading | **Yes, but label it "opened"**, never "read". It is in the card only, never ✓ |
| Q5 | New setuid bits (castle cleared, a world's trials all passed, a warp taken)? | **Two:** "castle" (any conclusion trial) and "every trial in a world". Warps get none (no steering, GR7) |
| Q6 | RTFM opens on the map or the man index? | **The map**, at the nearest-band module (`learnModule`). The man index stays one key away (`i`) for search |
| Q7 | Practice solves from RTFM: pay bytes? | **No.** They pay what an upsolve pays today (0 B unless they recover a core dump). Trials keep their door economy |
| Q8 | Show external (non-judgeable) problems in PRACTICE? | **Yes, muted with `o`**, and never counted in k/n (n counts playable problems only) |
