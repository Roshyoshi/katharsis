# Achievements: setuid bits

Achievements thread, 2026-10-15. **Status: proposed.** It asks the user for
the decisions in §(l). No code. It builds on the map (`map.md`, D52), the
cycles meta (`meta.md`), D53 (story only at game events), D55 (TIME+) and
the retention guardrails GR1–GR12 (`docs/research/retention.md` §(d)), which
bind every line below.

**Evidence labels**, as in `map.md`: *industry* (developer statement, store
page, official wiki, platform docs), *PR* (peer reviewed), *secondary*,
*player report*. *(inference)* marks my own reasoning. Steam percentages are
a snapshot taken on 2026-10-15.

## (a) Summary

**Recommendation: 61 achievements called setuid bits, in four kinds.**

- **They pay nothing.** They are listed on a `find / -perm -4000` screen
  reached from GRUB.
- **Every visible bit states its barrier.** The text names a boss, a count
  or a time, the way Hollow Knight's do.
- **Secrets show as `-?????????` rows.** Each has an optional hint.
- **One secret is an alternate ending in act II, *Dull Reality*.** Arrive
  at the act II bosses with a full cycle pool and a hidden exit opens: the
  Court of Hearts. Win its one-shot cursed door and you wake on the
  riverbank. The boot ends there without a prestige unlock.

| Kind | Count | Dir | What sets it | Example |
|---|---|---|---|---|
| progression | 23 | `/bin` | playing on: each boss, each ring, the first of each learning act | `snicker-snack`: defeat The Halting Problem |
| skill | 16 | `/sbin` | a run you set out to do: door-time speedruns, clean and constrained clears, one twist per boss, two speed completions on boot time | `stopped-looking`: defeat Heisenbug without running the samples |
| secret | 10 | `/home/you/.*` | curiosity: a hidden route, a hidden state, a clutch moment | `two-miles-high`: hit the cycle cap |
| completion | 12 | `/usr/bin` | counters with progress shown, then 100% and 112% | `regular-course`: hold all 11 capabilities |

**The user's request, corrected by the sources (§(b)):**

- **Neither game has a 112% speedrun achievement.**
  - Hollow Knight's Speed Completion is 100% in under 20 h.
  - Silksong's is 100% in under 30 h.
  - Hollow Knight's 112% achievement, *Pure Completion*, has no clock.
  - 112% is a speedrun.com category, not an achievement. That is from memory and unverified.
  - katharsis gets both bits anyway: `speed-completion` (100%) and `speed-completion-112`.
- **Silksong's act II ending has no "twisted root".**
  - The ending is *Twisted Child*: defeat Grand Mother Silk while cursed.
  - You get the curse by giving the **Twisted Bud** to **Greyroot**, and the ending needs you to refuse the cure (Yarnaby's Witch Crest).
  - "Beat the game in act 2" is right.

**Two places where I push back:**

1. **TIME+ should not be the clock for the completion speedruns.**
   - D55's TIME+ counts upsolves and RTFM trials.
   - So a TIME+ ceiling would charge for the two acts the game exists to encourage.
   - The speed completions measure **boot time**: TIME+ minus upsolves and trials (§(d) S15, S16).
2. **Achievements are feedback here, not a learning lever.**
   - Points and badges raised output but not competence or intrinsic motivation (Mekler et al. 2017, PR).
   - Completion-contingent rewards lowered free-choice motivation, d = −0.36. Positive feedback raised it, d = +0.33 (Deci, Koestner & Ryan 1999, PR meta-analysis).
   - So bits never pay and never steer doors (GR4, GR7). Their text reads as a record of what you did.

## (b) What the role models do

### Hollow Knight (Team Cherry, 2017)

63 achievements, 24 hidden on Steam ([S-HK], [W-HK]).

| Kind | Examples, verbatim | Steam % |
|---|---|---|
| boss, visible | Falsehood (False Knight); Test of Resolve; Respect (Mantis Lords) | 71.3; 62.7; 46.6 |
| boss, hidden | Strength (Failed Champion); Obsession (Collector); Banishment | —; —; 8.2 |
| collection tiers | Protected "Acquire 4 Mask Shards" → Masked (all); Charmed "Acquire your first Charm" → Blessed | —; 20.4; 76.7 → 20.2 |
| endings, hidden | The Hollow Knight; Sealed Siblings; Dream No More; Passing of the Age | 24.5; 18.9; 20.5; 7.0 |
| pantheons, hidden | Brotherhood … Embrace the Void "Ascend the Pantheon of Hallownest and take your place at its peak" | 17.4 … 5.4 |
| completion and skill | Completion; Pure Completion "Achieve 112% game completion and finish the game."; Speedrun 1 "Complete the game in under 10 hours"; Speedrun 2 "…under 5 hours"; Speed Completion "Achieve 100% game completion and finish the game in under 20 hours"; Steel Soul; Steel Heart (100% in Steel Soul) | 19.1; 7.0; 6.6; 5.4; 5.6; 4.7; 3.9 |

- **How 112% is counted.** The base game adds to 100%: charms 36, bosses
  14, equipment 14, warrior dreams 7, and so on. The DLC adds 12. The
  Pantheon of Hallownest counts 0 ([W-HKC], industry).
- **When the percentage shows.** It is hidden until *World Sense*, which a
  tablet grants after all three Dreamers die ([W-HKWS]).
- **What makes each barrier clear** *(inference from the verbatim text)*:
  - Each visible description names one countable thing ("4 Mask Shards") or one time ("under 5 hours").
  - The first tier of a collection equals one upgrade: 4 shards make a mask.
  - Hidden ones are just as literal ("Defeat the Collector"). They hide *what exists*, not how hard it is.
- **Speedrun 1 and 2 sit within 1.2 points of each other** (6.6% and 5.4%).
  Most players likely get both in one dedicated run *(inference)*.
- **No developer statement on achievement design was found.** That covers
  Gibson, Pellen and Griffin, and the Team Cherry blog.

### Silksong (Team Cherry, 2025)

52 achievements, 16 hidden; all five ending achievements are hidden ([S-SS],
[W-SS]).

| Achievement | Verbatim | Steam % |
|---|---|---|
| Speedrunner | "Complete the game in under 5 hours" | 5.0 |
| Speed Completion | "Achieve 100% game completion and finish the game in under 30 hours" | 4.0 |
| Steel Soul / Steel Heart | "Finish the game in Steel Soul mode" / "…100% game completion…" | 3.8 / 3.0 |
| Weaver Queen (act II) | "Defeat Grand Mother Silk and bind her power" | 41.0 |
| **Twisted Child (act II)** | "Defeat Grand Mother Silk while cursed" | **17.3** |
| Sister of the Void (act III) | "Defeat Lost Lace and free Pharloom" | 29.8 |

**The act II alternate ending, verified** ([W-SSE], [W-TB], [PCG]):

1. Finish Greyroot's *Rite of the Pollip* (6 Pollip Hearts).
2. Find the **Twisted Bud** behind a breakable wall, reached from a hidden
   drop at the east of the Whispering Vaults. Its description: "Contorted
   mass of ashen vine. Cries out incessantly."
3. Give it to Greyroot. *Rite of Rebirth*: "Greyroot seizes Hornet's
   shell".
   - Hornet wakes cursed: no Bind (no heal), no tools, no Silk Skills, at most 3 Silk, slower, no crest change.
4. **Do not take the cure.** *Infestation Operation* leads to Yarnaby, whose
   Witch Crest ends the curse. Getting it closes the ending on that save.
5. Defeat Grand Mother Silk while cursed.
   - The scene: vines burst from Hornet's shell, and Hornet, Grand Mother Silk and the Cradle all become roots.

**Why it lands** (press, labelled; no community data could be read):

- PC Gamer: "one of the darker ones… it consumes both of you".
- PCGamesN: "a truly spooky affair".

My reading *(inference)*:

- The key is a handicap you choose. You refuse a cure the game keeps pointing you to.
- The hints are thematic, not mechanical: the Bud cries, and Pavo talks of "Offerings they'd call the crying things".
- After the ending the game reloads you before the boss, so trying it costs little.
- It is rarer than the act III ending (17.3% against 29.8%). That points to discoverability as the gate, not difficulty.

Gibson on endings [B]: "It is just an ending." Pellen [B]: "We don't
reveal percentages or anything like that until you go beyond Act 2."

### Star of Providence (Team D-13, 2017; Steam app 603960)

52 achievements, 18 hidden ([S-SoP], [W-SoP]).

- **Difficulty sits on its own axis, Lethality.** It has three levels:
  - Mild: 10 HP.
  - Intense: 5 HP at 125% damage.
  - Sudden Death: 1 HP at 200% damage.

  Lethality changes only the player, and each level has a named achievement for a Floor 6 win: Intense (2.8%), Death, Suddenly (1.5%).
- **Constraint runs:**
  - Peashooter, no weapons (1.8%).
  - Power Internal, no upgrades (1.5%).
  - Speed Runner, under 20 minutes (5.1%).
- **Loop achievements fire on reaching a loop, "victory or loss".**
- **The Floor 6 ending needs four seals.** The seal door shows its empty
  slots before you can open it. Players report the "top seal" missing
  *(player report)*. So the gate is seen long before it opens.
- **NPCs give hints:** Kleines says "Some walls down there are pretty thin."
- **The jukebox lists unheard songs as "???"**, which works as a completion
  meter.

### What motivates, and what grinds

| Finding | Source | Grade |
|---|---|---|
| Completion-contingent rewards lowered free-choice intrinsic motivation (d = −0.36); positive feedback raised it (+0.33); unexpected rewards, no effect (0.01) | Deci, Koestner & Ryan 1999 | PR, meta-analysis |
| Points and leaderboards raised output, not competence or intrinsic motivation | Mekler et al. 2017 | PR |
| Badges raised activity in a field study (N ≈ 1,400 before, 1,600 after) | Hamari 2017 | PR |
| "If description is vague enough, it functions more as a teaser"; unbounded multipliers ("1 million fire damage") are grind | Hamari & Eranti 2011 | PR (DiGRA) |
| Public records "easily turns the most fun filled activities into chores" | Jakobsson 2011 | PR |
| Players felt pressure to collect everything, disagreed whether badges show skill or time spent, and said badges can pull them out of the game | Cruz, Hanus & Fox 2017 | PR |
| "Use measurement achievements instead of completion achievements"; moderate difficulty gives the best gains | Blair (Game Developer) | industry |
| Hide only "spoiler-related or unmissable content", otherwise "make it clear in the description how to get it"; players "look up the answer anyway"; a little grind is fine with a progress tracker | ID@Xbox, Xbox Wire 2020 | industry |
| Give one achievement "in the first 10 minutes"; about 50% "off path"; brutal ones please "an extremely small part" of players | Charla 2018 | industry |
| Show progress partway (10 of 20 wins) | Steamworks docs | industry |
| Dead Cells' Boss Stem Cell ladder falls off steadily: 40.4, 22.9, 14.7, 11.4, 8.5% | [S-DC] | industry |

**The rules this doc follows** *(inference from the table, squared with the
guardrails)*:

- **R1 · State the barrier.** Visible text names the boss, the count or the
  time. Hide only what would spoil (Xbox, Hollow Knight).
- **R2 · Measure, don't reward.** No bytes, no items, no themes. The bit is
  feedback (Deci; GR4).
- **R3 · Skill bits are runs you set out to do.** Each is a constraint you
  pick at GRUB or the first route, like Peashooter or Speed Runner. A
  normal boot rarely sets one by accident.
- **R4 · Counters are bounded and tracked.** The biggest counter is 100
  rated doors. Every counter shows its progress (Steamworks, Hamari &
  Eranti).
- **R5 · Secrets reward curiosity about systems already there.** Every hint
  is a game event (D53). There is no lore page.
- **R6 · Nothing about the calendar.**
  - No streaks, no "in a row", no "today" (GR1, GR2).
  - The nightly bit takes any date.
  - TIME+ only ever appears as a ceiling, never as hours to pile up (GR4: "never for … hours played").
- **R7 · No win cue on a crash.** A bit earned in a boot that crashes shows
  at the next GRUB, never on the panic screen (GR3).
- **R8 · Bits never steer the game.** The engine never reads them. Easing
  never disqualifies a bit, because an easing is the doors' state, not the
  player's (GR7, GR9).

## (c) The name: setuid bits

| Candidate | For | Against |
|---|---|---|
| **setuid bits** | privilege is the story's spine ("Nobody checks your privileges"; rings count backwards); `ls -l` gives the screen its grammar; `chmod u+s` is the unlock verb; `-?????????` is how `ls` draws an entry it cannot stat; GNU `ls --color` paints setuid files white on red (`su=37;41`) | sits next to the CAP_* capabilities, which are also privileges |
| CTF flags | secrets map well (`flag{…}`) | "capture" implies an opponent; collides with CLI flags; progression and completion don't fit |
| badges in /etc | plain | "badge" is the gamification word the literature doubts; /etc holds config, not deeds; no verb |

**Pick: setuid bits.** The CAP_* overlap becomes a joke the game can make on
purpose:

- **Capabilities** are the modern, fine-grained privilege. They open routes.
- **A setuid bit** is the old, coarse one, and here it opens nothing.

The screen footer says so: "setuid bits open nothing. They record what you
did, and when." The copy never calls a bit a capability.

Vocabulary in use:

- A bit is **set** (never "unlocked") and **not set**.
- The screen is `find / -perm -4000`.
- The verb on the strip is `chmod u+s /bin/<id>`.
- **Paths by kind:**
  - progression lives in `/bin`
  - skill in `/sbin`, the superuser's directory
  - completion in `/usr/bin`
  - secrets are dotfiles in `/home/you`, which `ls` hides by default

## (d) The list

**Helpers.** Each condition below is a pure function of `Meta`, one boot's
`State` and `Meta.Record`, the fold over finished boots described in §(i).

```
B           a boot: the one in play (live bits) or a finished one (end bits)
built(B)    B.Trial = nil ∧ B.Rules.Name ∈ {standard, short, patch}
              (built-in names are reserved: ./cmd/katharsis/presets.go:72)
map(B)      built(B) ∧ len(B.Rules.Nodes) > 0           a three-act boot
clear(B)    B.Phase = cleared ∧ B.Ending = ""
full(B)     map(B) ∧ clear(B)
beat(B,k)   ∃a ∈ B.History: a.Door.Boss = k ∧ a.Outcome = Cleared
door(B,k)   that attempt a
burn(a)     #{e ∈ a.Events : e.Kind = submit ∧ e.Verdict ∉ {AC} ∪ FreeVerdicts}
node(a)     B.Biomes[a.Biome].Node
up(B)       B.Uptime(end of B)                                    door time
P(B)        B.Prestige
R           Meta.Record (§(i)): Nodes, Bosses, Verdicts, Daemons, Unlocked,
            Endings, BootTime, Recovered, RecoveredFree, Nightly
```

Abandoned boots set nothing. Easing never counts against a bit (R8).

**Text conventions.**

- *Text* is the mechanical barrier, shown on the screen. It may name
  numbers, as mechanic text does (tone rule 5).
- The unlock line in the story's voice is a separate hook,
  `setuid.<id>`, written by the storyteller (§(h)).
- Names that quote Carroll keep each quote within two hooks (tone
  rule 11). The new quotes are listed in §(f).

### Progression · `/bin` · 23

| # | id | Name | Text | Condition |
|---|---|---|---|---|
| P1 | `hello-world` | hello, world | Clear a door. | ∃B built, ∃a ∈ B.History: a.Outcome = Cleared |
| P2 | `exit-0` | exit 0 | Clear a boot. | ∃B: built(B) ∧ clear(B) |
| P3 | `two-twins` | Two Twins, One Gap | Defeat Off-by-One. | ∃B built: beat(B, off-by-one) |
| P4 | `there-goes-bill` | There Goes Bill | Defeat the OOM Killer. | beat(B, oom-killer) |
| P5 | `shelf-stays-full` | The Shelf Stays Full | Defeat Heisenbug. | beat(B, heisenbug) |
| P6 | `caucus-race` | The Caucus-Race | Defeat Race Condition. | beat(B, race-condition) |
| P7 | `handsome-pig` | A Handsome Pig | Defeat Undefined Behavior. It opens from prestige 1. | beat(B, undefined-behavior) |
| P8 | `snicker-snack` | Snicker-Snack | Defeat The Halting Problem: clear a three-act boot. | ∃B: full(B) |
| P9 | `through-the-glass` | Through the Glass | Unlock Silver: clear at Bronze once 60 doors are rated. | ∃B built: B.Unlocked = 1 |
| P10 | `nameless-wood` | The Nameless Wood | Unlock Gold. | ∃B built: B.Unlocked = 2 |
| P11 | `eighth-square` | Eighth Square | Unlock Platinum: ring 0. | B.Unlocked = 3 |
| P12 | `red-kings-dream` | The Red King's Dream | Unlock Advanced: ring -1. | B.Unlocked = 4 |
| P13 | `sunny-at-midnight` | Sunny at Midnight | Unlock prestige 5: ring -2. | B.Unlocked = 5 |
| P14 | `where-the-fall-began` | Where the Fall Began | Unlock prestige 7: ring -3. | B.Unlocked = 7 |
| P15 | `you-halted` | You Halted | Clear a three-act boot at prestige 8. | ∃B: full(B) ∧ P(B) = 8 |
| P16 | `stamped` | Stamped by the Gryphon | Pass an RTFM trial that grants a capability. | len(M.Capabilities) ≥ 1 |
| P17 | `another-road` | Another Road Opens | Clear a room behind a locked exit (a capability, or `apt git`). | ∃B map, ∃a Cleared: the node of a is gated (`Gate.Capability` or `Gate.Package` ≠ "") |
| P18 | `grin-returns` | The Grin Comes Back | Recover a core dump by upsolving its problem. | R.Recovered ≥ 1 |
| P19 | `struck-off` | Struck Off in Ink | Retire a regression: re-solve it at every stage. | M.Retired ≥ 1 |
| P20 | `humpty` | Humpty's First Word | Save a lib (a `// lib:` block in a cleared door). | len(M.Libs) ≥ 1 |
| P21 | `one-topic` | One Topic Knows You | Master a topic: 10 rated doors on it. | ∃t ≠ mixed: M.TopicAttempts[t] ≥ Rules.MasteryAttempts (as `milestone.mastered.1`) |
| P22 | `by-railway` | By Railway | Clear a boot that took act III's skip. | ∃B full: ∃i B.Biomes[i].Node = skip |
| P23 | `un-birthday` | An Un-Birthday Present | Clear a nightly build, any date. | ∃B full: B.Seed = NightlySeed(local date of B.Started) |

The ring bits read `B.Unlocked` on a built-in boot, not `Meta.Prestige`. A
custom preset can unlock prestige (`./internal/game/engine.go:774`, which
does not check the preset), and a custom preset's clear sets nothing.

### Skill · `/sbin` · 16

Each one is a run you set out to do (R3). A three-act boot ("map") is
`standard` from D52 on.

| # | id | Name | Text | Condition |
|---|---|---|---|---|
| S1 | `speedrun-1` | Late, but Inside the Limit | Clear a three-act boot in under 1:30:00 of door time. | full(B) ∧ up(B) < 1h30 |
| S2 | `speedrun-2` | Faster Than the Rabbit | Clear a three-act boot in under 1:00:00 of door time. | full(B) ∧ up(B) < 1h |
| S3 | `all-o3` | Twice as Fast as That! | Clear a three-act boot with every door -O3. Retreats are allowed; a lost room is not. | full(B) ∧ ∀a: (a.Outcome = Cleared ∧ a.O3) ∨ a.Outcome = Retreated |
| S4 | `finds-none` | Finds None. Sulks. | Clear a three-act boot with no failed hidden-test submit. | full(B) ∧ ∀a: burn(a) = 0 |
| S5 | `catless` | Not Even the Grin | Clear a three-act boot without a duck. | full(B) ∧ Σ a.Quacks = 0 |
| S6 | `essence` | Time Is of the Essence | Clear a three-act boot without buying time: no Overclock, ECC, nice, at or retreat (kexec swaps count as retreats). | full(B) ∧ Overclock, ECC ∉ B.Upgrades ∧ nice ∉ B.Daemons ∧ ∀a: a.At = 0 ∧ a.Outcome ≠ Retreated |
| S7 | `doesnt-blink` | She Doesn't Blink | Clear at prestige 8 with no failed hidden-test submit and no duck. | full(B) ∧ P(B) = 8 ∧ ∀a: burn(a) = 0 ∧ Σ a.Quacks = 0 |
| S8 | `ditto-ditto` | Ditto, Ditto | Defeat Off-by-One with no failed hidden-test submit on its door. | a = door(B, off-by-one) ∧ burn(a) = 0 |
| S9 | `rss-modest` | RSS: Modest | Defeat the OOM Killer on your first :Submit. | a = door(B, oom-killer) ∧ a.Submits = 1 |
| S10 | `stopped-looking` | You Stopped Looking Hard | Defeat Heisenbug without running the samples (`:w`) on its door. | a = door(B, heisenbug) ∧ a.Samples = 0 (new field, §(i)) |
| S11 | `every-thread` | Every Thread Joined | Defeat Race Condition with -O3, under half its fast clock. | a = door(B, race-condition) ∧ a.O3 |
| S12 | `bounds-checked` | Bounds-Checked the Soup | Defeat Undefined Behavior with no failed hidden-test submit. | a = door(B, undefined-behavior) ∧ burn(a) = 0 |
| S13 | `decided-this-one` | You Decided This One | Defeat The Halting Problem with -O3, under half its hidden clock. | a = door(B, halting-problem) ∧ a.O3 |
| S14 | `independent` | The Cat Kept a Copy | Recover a core dump with an upsolve made without the editorial or a duck. | R.RecoveredFree ≥ 1 |
| S15 | `speed-completion` | Speed Completion | Reach 100% with under 60 h of boot time (TIME+ less upsolves and trials). | the first check that finds Completion(M) ≥ 100 also finds R.BootTime < 60h |
| S16 | `speed-completion-112` | Pure Speed | Reach 112% with under 150 h of boot time. | the same at 112 and 150h |

**Boss bits (S8–S13)** are each the boss's own twist, mastered (`map.md`
§(f)). Off-by-One burns double on a WA, so S8 asks for none. Heisenbug says
only pass or fail on `:w`, so S10 asks you not to look at all. That line
comes from its defeat text: "You stopped looking hard."

**The speedrun thresholds are provisional.**

- **Door time.** The model's full-clear median is 2h44–3h08 (`map.md`
  §(a)). S1 is about half of that and S2 about a third; Hollow Knight's
  two speedruns are about 1/3 and 1/6 of a first playthrough *(inference)*.
  Half of every base timer on the skip route adds up to 1h27, so S1 is
  about "-O3 pace" and S2 is beyond it.
- **The model's clock is not the player's** (`map.md` §(m)). Decision 4
  sets both from the first ten real map clears, then freezes them.
- **Boot time.** 60 h and 150 h are placeholders. Decision 5 sets them
  from the learning simulator (ML3) at the 25th percentile of hours to
  100% and to 112%.
- **What these are anchored to.** Hollow Knight's 20 h is about 0.4 of a
  typical 100% run *(inference)*. In `map.md` §(i)5, 26 of 30 learning
  players reach P8 within 381 hours a year, churn off.
- **They are missable on a save.** Like Hollow Knight's Speed Completion,
  they belong to the store they began in; a fresh store is a fresh try.
- **Neither ever shows a countdown** (§(g)).

### Secret · `/home/you` · 10

They show as `-?????????` rows. **Hint** is what `?` reveals on the selected
row; **Text** is shown once the bit is set.

| # | id | Name | Hint | Text once set | Condition |
|---|---|---|---|---|---|
| X1 | `.dull-reality` | Dull Reality | Who Stole the Tarts? | Win the Trial and wake on the bank. | ∃B: B.Ending = wake (§(f)) |
| X2 | `.two-miles-high` | Nearly Two Miles High | EAT ME | Reach the cycle cap (♥ 50/50) in a boot. | ∃B map: B.Cycles ≥ B.MaxCycles at a safe point, or ∃a: a.CyclesAtOpen ≥ B.MaxCycles (backfill) |
| X3 | `.stuff-and-nonsense` | Stuff and Nonsense! | The court sits in the garden. | Find the Trial. | ∃B map, ∃i: B.Biomes[i].Node = trial |
| X4 | `.then-it-ends` | Then It Ends | The Rabbit cries twice. | Defeat The Halting Problem in its last minute. | a = door(B, halting-problem) ∧ ¬a.Overtime ∧ 0 ≤ a.Deadline − a.Closed < 1:00 |
| X5 | `.galumphing-back` | Galumphing Back | The Rose knew a shortcut. | Walk away from the Jabberwock, then slay it. | ∃B map: ∃a (Boss = halting-problem ∧ Retreated) before door(B, halting-problem) |
| X6 | `.consider-your-verdict` | Consider Your Verdict | The jury runs out of slate. | Hear every verdict on boot doors: AC, WA, TLE, MLE, RE, CE, OLE. | R.Verdicts ⊇ {AC, WA, TLE, MLE, RE, CE, OLE} |
| X7 | `.every-seat` | Every Seat Taken | Three seats taken. | Run every daemon seat a boot has. | ∃B built: len(B.Daemons) ≥ B.Rules.MaxDaemons |
| X8 | `.counted-again` | Counted Again | Count them. Count again. | Defeat Off-by-One after its double burn. | a = door(B, off-by-one) ∧ ∃e ∈ a.Events: submit WA with Passed = Total − 1 |
| X9 | `.size-of-a-door` | Exactly the Size of a Door | Under the doormat in the hall. | Read the note to its last fold. | HasSeen(M, milestone.doors.500) |
| X10 | `.which-dreamed-it` | Which Do You Think It Was? | Two endings. One dreamer. | See both endings. | `snicker-snack` ∧ `.dull-reality` |

**Notes on the secrets:**

- **X2 and X3 are the trail to X1** (§(f)).
- **X5 depends on the engine.** `retreat` has no boss check today
  (`./internal/game/engine.go:393`). If the map build refuses a retreat on
  the final, X5 is cut.
- **X8 rewards a fix after the twist bit you.** It is the learning-shaped
  twin of S8.
- **X9 is the long secret of the milestone thread.** The Turing note ends
  "exactly the size of a door" (text.md `milestone.doors.500`), so the bit
  sets when that beat is shown, not at the 500th door.

### Completion · `/usr/bin` · 12

Every row shows its progress in the SIZE column (R4).

| # | id | Name | Text | Condition |
|---|---|---|---|---|
| C1 | `every-bug` | Every Bug in the Pack | Defeat all six bosses. | \|R.Bosses\| = 6 |
| C2 | `cartographer` | Where You Want to Get To | Clear a room in every map node: 17. | R.Nodes ⊇ every node of `rules.MapNodes` that is not a boss, the skip or hidden |
| C3 | `regular-course` | The Regular Course | Hold all 11 capabilities. | M.Capabilities ⊇ values of `game.TopicCapabilities` |
| C4 | `apt-full` | apt list --installed | Install every package: 19. | M.Installed ⊇ `Rules.Packages` names |
| C5 | `own-invention` | It's My Own Invention | Clear a boot with each daemon running: 7. | R.Daemons ⊇ {strace, nice, fork, dev-random, kexec, cron, chrt} |
| C6 | `her-list` | Her List, in Ink | Retire 5 regressions. | M.Retired ≥ 5 |
| C7 | `ledger` | The Cat's Ledger Balances | Recover 5 core dumps. | R.Recovered ≥ 5 |
| C8 | `not-in-general` | Not in General | Rate 100 doors. | RatedDoors(M) ≥ 100 |
| C9 | `dictionary` | Humpty's Dictionary | Save 5 libs. | len(M.Libs) ≥ 5 |
| C10 | `every-topic` | Every Topic Knows You | Master every topic but mixed: 15. | ∀t ≠ mixed: M.TopicAttempts[t] ≥ Rules.MasteryAttempts |
| C11 | `hundred` | 100% | Reach 100%. | Completion(M) ≥ 100 (§(e)) |
| C12 | `hundred-twelve` | 112% | Reach 112%. | Completion(M) ≥ 112 |

**Counts.** 61 in all:

- 51 visible and 10 secret. The completion bits and the ring bits show
  their progress.
- About half are off the main path (skill, secret, the apt and daemon sets), as Charla recommends.
- `hello-world` sets in boot #1's first ten minutes.

## (e) Completion: 100% and 112%

Hollow Knight's split, katharsis's content. **100% is the divisions done**
(Bronze to Advanced). **The 12% beyond is rings -2 and -3 and the secret
ending.** That puts one point of 112% behind a `???`, which tells a curious
player there is something left to find, as Hollow Knight's 112% does.

| Part | Each | Count | Points |
|---|---|---|---|
| bosses defeated (R.Bosses) | 3 | 6 | 18 |
| map nodes with a cleared room (R.Nodes) | 1 | 17 | 17 |
| capabilities held | 2 | 11 | 22 |
| prestige 1–4 unlocked by a built-in boot (R.Unlocked) | 5 | 4 | 20 |
| a three-act clear at prestige 4 | 5 | 1 | 5 |
| firsts: a dump recovered, a regression retired, a lib saved, a topic mastered, a skip clear, a nightly clear | 3 | 6 | 18 |
| **100%** | | | **100** |
| prestige 5–8 unlocked | 2 | 4 | 8 |
| a three-act clear at prestige 8 | 2 | 1 | 2 |
| Dull Reality | 2 | 1 | 2 |
| **112%** | | | **112** |

- **No bytes and no purchases.** apt has its own bit (C4) and no share of
  the percentage, so completion is never a shopping list.
- **When it shows.** The percentage appears once the player has cleared a
  three-act boot. Before that, the header reads "23 of 61 set". This
  follows Silksong ("We don't reveal percentages … until you go beyond Act
  2") and Hollow Knight's World Sense.

## (f) The secret ending: Dull Reality

**Claim.** katharsis has two Carroll endings available, and it uses only
one.

- **The final clear is *Looking-Glass*'s ending.** "Which do you think it
  was?" The dream goes on: "Reboot."
- ***Wonderland* ends differently.** Alice grows at the trial, refuses the
  Queen's sentence, and the cards fly up. She wakes on the bank with her
  sister, who then closes her eyes and goes on dreaming it.
- **The bible already holds that sister.** She is ring -3, the Management
  Engine, "awake with the power off, eyes shut", and "She doesn't blink".

The waking ending gives that line a payoff: she never opens her eyes, so
after you wake, the machine is her dream.

### The route and the gate

```
LeaveMerge after act II's room
└─ enterMapTier(boss II) draws the exits of the act II node's Next
   ├─ heisenbug · race-condition · undefined-behavior            as today
   └─ trial                Hidden, NeedFull
      ├─ ♥ < MaxCycles  → Locked "a mile high"; the screen hides it
      │                    unless Meta has .two-miles-high: ⚔ ??? · locked
      └─ ♥ = MaxCycles  → open: ⚔ The Trial
ChooseRoute(trial)        → sets .stuff-and-nonsense (no door open yet)
└─ boss card (boss.enter.trial, boss.rule.trial)
   └─ door: cursed (no Cat, one failed submit ends the boot), one shot
      ├─ AC              → finish(): State.Ending = "wake", uncommitted banked,
      │                    no prestige unlock, not a full clear
      │                    → scene ending.wake → sets .dull-reality
      └─ WA · timeout · :q!  → crash, as any cursed one-shot door
```

**Why a full pool is the gate.** In chapter XII Alice grows during the
trial, and the King reads "Rule Forty-two. All persons more than a mile high
to leave the court." To arrive "a mile high", the pool has to reach its cap,
♥ 50/50.

- **The fountain can't do it.** It stops at the starting 30: "Never past the
  line".
- **fsck can**, and it costs 64 B of uncommitted bytes a time (`map.md`
  §(e)). At P0, from about 25 cycles, that is three buys.
- **Deeper rings cost more.** fsck heals 5 from P2 and the start drops to
  20 at P7, so up to six buys.
- **So it is a state the game nudges you away from**, as Silksong's curse
  is. Nobody banks bytes into a full pool before a boss for a 20-cycle
  retry *(inference)*.
- **Once inside, the pool is worth nothing.** The court is cursed: one
  failed submit and the boot ends, whatever you carry. That is the Queen's
  "Sentence first—verdict afterwards."

**The hints, each a game event (D53).** No lore tab; each step narrows the
search:

| Step | Where | Line | Status |
|---|---|---|---|
| 1 | every MLE verdict, and the MLE crash epitaph | "All persons more than a mile high to leave the court." | exists (`verdict.MLE`) |
| 2 | the first act II route | "The gardeners are painting the white roses red." In ch. VIII the Queen's procession is next. | exists (`story.first-act2`) |
| 3 | the strip, the first time ♥ reaches the cap | `setuid.two-miles-high`: "Nearly two miles high. Somewhere a King looks up a rule." | new |
| 4 | the boss II route, once step 3 has happened | a locked card: `⚔ ??? · locked · Rule Forty-two.` | new |
| 5 | the boss II route at ♥ 50/50 | the open card | new |
| ? | the `?` hint on the `.dull-reality` row | "Who Stole the Tarts?" (ch. XI's title) | new |

Step 4 is Star of Providence's seal door: once you have the clue, the gate
is visible before it opens.

### The cards

```
 route · boss II · after The ALU                          ♥ 50/50  ⏱ ~36:00
      Pick an exit. It sets the boss.
 ┌ 1 ───────────────────┐ ┌ 2 ───────────────────┐ ┌ ▸ 3 ─────────────────┐
 │ ⚔ Heisenbug          │ │ ⚔ Race Condition     │ │ ⚔ The Trial          │
 │ the Sheep's shop     │ │ the Caucus-race      │ │ the Court of Hearts  │
 │ :w says pass or fail │ │ clock 1.25× · pay ×4 │ │ cursed · one shot    │
 │ a fail −20 ♥, retry  │ │ a fail −20 ♥, retry  │ │ a fail ends the boot │
 │ → act III            │ │ → act III            │ │ ends the boot here   │
 │                      │ │                      │ │ no prestige unlock   │
 └──────────────────────┘ └──────────────────────┘ └──────────────────────┘

 ⚔ The Trial · the Court of Hearts                act II · ◆1450 · ⏱ 36:00
   "Sentence first—verdict afterwards." One WA, one head.
   cursed: no Cat, one failed submit ends the boot · one shot
```

The card states what the route costs: no act III and no prestige unlock.
Flavor never lies about mechanics (tone rule 5), and the player chooses
knowing.

### The scene: `ending.wake`

It plays once in full; later wakes play beats 4–6 only. It obeys D33's
reduced-motion setting and is 80x24-safe.

| Beat | Picture | Text (draft for the storyteller) |
|---|---|---|
| 1 · the court | the boss pane, the ♥ 50/50 bar lit | The King reads from his book: "Rule Forty-two." Everybody looks at you. / "Nearly two miles high," added the Queen. ♥ 50/50. |
| 2 · the verdict | the judge's AC line, then the Queen's | "Sentence first—verdict afterwards." The judge says AC. / Your AC says it for you: "Who cares for you?" |
| 3 · the cards | every box on screen (panes, HUD, cards) lifts off in half-blocks and scatters up, then down, as the DOOM title's fire does; reduced motion: one cut to black | At this the whole pack rose up into the air, and came flying down… |
| 4 · dull reality | **Mono**, the terminal's default colors, no theme; a riverbank in one line of plain text | You are lying on the bank, head in your sister's lap. / "Why, what a long sleep you've had!" |
| 5 · the man page | the prologue's man page, plain, now with an EXAMPLES section: the titles of this boot's cleared doors (real values, GR3) | The man page is still open. It has an EXAMPLES section now. |
| 6 · the loop | the sister's eyes stay shut; colors seep back into GRUB | "…but now run in to your tea; it's getting late." / Your sister closes her eyes. The White Rabbit hurried by. |

The man page in beat 5 answers the prologue's "What is the use of a man page
without examples or conversations?":

```
YOU(1)                     General Commands Manual                     YOU(1)

NAME
       you - PID 1

EXAMPLES
       Bovine Shuffle
       Cow Poetry
       Haybale Feast
       The Trial

SEE ALSO
       rabbit(6)
```

Its EXAMPLES are this boot's cleared doors, so they are the player's own
work.

**The end screen and the log.**

- **The cleared screen** heads "✓ Boot #57 · awake · Bronze", with no
  prestige line, and its `kept` block as usual.
- **git log** shows `awake (the Trial)`.
- **No "woke".** The word carries politics the game doesn't need.

**The cast.**

- **The court:** the White Rabbit is the herald (`boss.enter.trial`), the
  King reads Rule Forty-two, the Queen gives the sentence, and the jury
  writes on slates (the OLE line).
- **The Cat is absent.** The door is cursed, so there is no Cat and no
  grin.
- **The sister** goes on dreaming the machine.
- **You never speak.** Your AC speaks for you (bible: "You never speak").
- **No gender.** Carroll's "upon her" is elided. "Off with her head!" is
  not used.

**New quotes for the register.** All are checked verbatim against Gutenberg
`pg11.txt`, ch. XI–XII, on 2026-10-15; italics are dropped:

- A59: "Stuff and nonsense!"
- A60: "Nearly two miles high,"
- A61: "Who cares for you?"
- A62: "At this the whole pack rose up into the air, and came flying down"
- A63: "Why, what a long sleep you've had!"
- A64: "but now run in to your tea; it's getting late."
- A65: "the White Rabbit hurried by"
- the ch. XI title: "Who Stole the Tarts?"
- ch. I: "EAT ME"

The scene avoids A53 ("You're nothing but a pack of cards!") and A54 ("Wake
up, Alice dear!"). Each already sits in two hooks (rule 11). The
storyteller may move one of them here, since this scene wants them most.

### Balance

The Trial changes no other rule, and it is not a shortcut.

- **It never unlocks prestige.** It does not count as a full clear for D31's
  gate, the governor's `FullClears` or `Stats.FullClears`.
  - The governor reads it as not a crash.
- **It pays like any cursed boss door**, through `doorBytes`, so expected
  pay per door stays flat (GR5).
- **It costs to reach and pays less in total.** You spend 192–384 B on
  fsck, and you skip act III's room and the final's pay. Over a boot that
  is a net loss, so it is no farm *(inference; the simulator checks it
  below)*.
- **It moves no measured band.** It sits at tier 4, so G1 and the act I
  and II hazards are unchanged. A boot that takes it forgoes act III, so
  the band's full-clear measure excludes those boots.
- **It is deterministic.** The exit reads `State.Cycles` and the rules, and
  draws no random number. Route offers keep propensity 1.

**The simulator check before merge.**

- Policy: "fsck to the cap before Boss II when affordable, take the Trial
  when open".
- Against the default:
  - bytes per door-hour no higher
  - prestige unlocks per boot unchanged for non-takers
  - the Trial's clear rate on arrival at 35–45%
- Default aim: 0.55, Boss I's, to offset the curse. The final's 0.50 with
  a curse would land lower *(inference)*.

## (g) The screen

GRUB gets one entry, `find / -perm -4000`, after `top`. At P8 with a dump,
regressions and apt, GRUB holds 14 entries in a 15-row box
(`./internal/screens/meta/testdata/grub_p8_80x24.golden`). The new entry
fills it exactly. Its subtitle is `grub.subtitle.setuid`: "Nobody checks
your privileges. Somebody keeps a list."

**80x24.** Rows are `ls -l` lines:

- **MODE:** `s` when set, `x` when not, `-?????????` for a secret not yet
  found.
- **SIZE:** a counter's progress.
- **SET:** when the bit was set.
- **PATH:** the bit.

The line under the rule is the selected row's text.

```
find / -perm -4000 -ls · 23 of 61 set                           spoilers: none
 MODE        SIZE   SET           PATH
 /bin · progression                                                    12 of 23
 -rwsr-xr-x     -   Oct 14 21:07  /bin/hello-world
 -rwsr-xr-x     -   Oct 14 23:40  /bin/exit-0
 -rwsr-xr-x     -   Oct 15 01:12  /bin/two-twins
▸-rwxr-xr-x     -                 /bin/there-goes-bill
 -rwxr-xr-x  34/60                /bin/through-the-glass
 /sbin · skill                                                          2 of 16
 -rwsr-xr-x     -   Oct 15 02:30  /sbin/finds-none
 -rwxr-xr-x     -                 /sbin/speedrun-1
 -rwxr-xr-x     -                 /sbin/stopped-looking
 /usr/bin · completion                                                  3 of 12
 -rwxr-xr-x   2/6                 /usr/bin/every-bug
 -rwxr-xr-x  4/11                 /usr/bin/regular-course
 /home/you · secret                                                     1 of 10
 -rwsr-xr-x     -   Oct 15 02:31  /home/you/.two-miles-high
 -?????????     ?   ?             /home/you/.???
 -?????????     ?   ?             /home/you/.???
 ↓ more
────────────────────────────────────────────────────────────────────────────────
 There Goes Bill · defeat the OOM Killer. It waits behind Brute Force Bunker
 and The Emulator in act I. Not set.
j/k move · tab next dir · ? hint · q back
```

**140x40.**

- **Left:** the list.
- **Right:**
  - the selected bit's barrier and where it applies
  - the completion breakdown (once a three-act boot is cleared)
  - the last few bits set

```
find / -perm -4000 -ls · 23 of 61 set · 35% complete                                                          spoilers: none · notices: on
 MODE        SIZE   SET           PATH                                │ /bin/there-goes-bill
 /bin · progression                                         12 of 23  │
 -rwsr-xr-x     -   Oct 14 21:07  /bin/hello-world                    │ There Goes Bill
 -rwsr-xr-x     -   Oct 14 23:40  /bin/exit-0                         │   Defeat the OOM Killer.
 -rwsr-xr-x     -   Oct 15 01:12  /bin/two-twins                      │   act I boss · behind Brute Force Bunker or The Emulator
▸-rwxr-xr-x     -                 /bin/there-goes-bill                │   built-in presets · easing never counts against it
 -rwxr-xr-x     -                 /bin/shelf-stays-full               │
 -rwxr-xr-x     -                 /bin/caucus-race                    │   not set
 -rwxr-xr-x     -                 /bin/handsome-pig                   │
 -rwsr-xr-x     -   Oct 15 01:55  /bin/snicker-snack                  │ ─ completion · 35% ────────────────────────────────────
 -rwxr-xr-x  34/60                /bin/through-the-glass              │   bosses        ▓▓▓▓▓▓░░░░░░░░░░░░  2/6      6 of 18
 -rwxr-xr-x     -                 /bin/stamped                        │   map nodes     ▓▓▓▓▓▓▓▓▓░░░░░░░░░  9/17     9 of 17
 /sbin · skill                                               2 of 16  │   capabilities  ▓▓▓▓▓▓░░░░░░░░░░░░  4/11     8 of 22
 -rwsr-xr-x     -   Oct 15 02:30  /sbin/finds-none                    │   rings         ░░░░░░░░░░░░░░░░░░  0/5      0 of 25
 -rwsr-xr-x     -   Oct 15 03:02  /sbin/ditto-ditto                   │   firsts        ▓▓▓▓▓▓▓▓▓▓▓▓░░░░░░  4/6     12 of 18
 -rwxr-xr-x     -                 /sbin/speedrun-1                    │   beyond        ░░░░░░░░░░░░░░░░░░  0/6      0 of 12
 -rwxr-xr-x     -                 /sbin/speedrun-2                    │
 -rwxr-xr-x     -                 /sbin/all-o3                        │ ─ recent ──────────────────────────────────────────────
 -rwxr-xr-x     -                 /sbin/catless                       │   Oct 15 03:02  ditto-ditto      boot #41
 -rwxr-xr-x     -                 /sbin/stopped-looking               │   Oct 15 02:31  .two-miles-high  boot #41
 -rwxr-xr-x     -                 /sbin/speed-completion              │   Oct 15 02:30  finds-none       boot #40
 /usr/bin · completion                                       3 of 12  │   Oct 15 01:55  snicker-snack    boot #38
 -rwxr-xr-x   2/6                 /usr/bin/every-bug                  │
 -rwxr-xr-x   9/17                /usr/bin/cartographer               │
 -rwxr-xr-x  4/11                 /usr/bin/regular-course             │
 -rwxr-xr-x   6/19                /usr/bin/apt-full                   │
 -rwxr-xr-x  71/100               /usr/bin/not-in-general             │
 /home/you · secret                                          1 of 10  │
 -rwsr-xr-x     -   Oct 15 02:31  /home/you/.two-miles-high           │
 -?????????     ?   ?             /home/you/.???                      │
 -?????????     ?   ?             /home/you/.???                      │
 -?????????     ?   ?             /home/you/.???                      │
 -?????????     ?   ?             /home/you/.???                      │
 -?????????     ?   ?             /home/you/.???                      │
 ↓ more                                                               │
                                                                      │
──────────────────────────────────────────────────────────────────────┴────────────────────────────────────────────────────────────────────
 setuid bits open nothing. They record what you did, and when.
j/k move · tab next dir · ? hint for this row · s spoilers · n notices · q back
```

Both sketches skip rows inside each directory to show every kind; the real
list scrolls.

**How rows behave:**

- **Barriers are visible.** Every visible row states its condition before
  it is set (R1). Ring bits show the D31 gate's progress (34/60 rated
  doors), the same number GRUB's gate line shows.
- **Not set reads "not set".** No "missed", no "0 of 41 boots", and no
  countdown on the speed completions (GR8; the broken-streak finding,
  `retention.md` §(b)2).
  - Once a speed completion's bound is passed, its row reads "for a fresh
    store".
- **Backfilled bits** show their boot's date faint, with "from git log"
  in the detail pane.
- **Spoilers**, in settings.toml as `setuid_spoilers`:
  - `none` (the default): `-?????????` rows; `?` shows the selected row's
    hint for this visit only.
  - `hints`: every secret row shows its hint.
  - `all`: names and text too.
  - The setting never leaks a secret into GRUB or the strip.
- **ASCII mode** draws `+` for `┼` and `#` for `▓`, as the parody screens
  should (QA list: they still draw Unicode).

## (h) The unlock moment

| Where | When | What |
|---|---|---|
| dmesg strip | the next safe point after the condition holds: a door closed after AC, a merge window, a route screen. Never in `PhaseProblem`, never over an open door's timer (GR6) | `[ 3851.0] chmod u+s /bin/two-twins` and, faint, its `setuid.<id>` line; two bits at most, then "and 2 more · find / -perm -4000" |
| cleared screen | the boot ends cleared or awake | one line in the `kept` block: `setuid +2 · snicker-snack, finds-none`; at 80x24 it takes the blank row above `kept` (`cycles_cleared_80x24.golden` is full) |
| crash screen | never (R7, GR3) | nothing; the next GRUB status line ends `· setuid +1` |
| outside a boot | an upsolve that recovers a dump; a trial pass; an apt install | the existing flash, the trial's cleared screen, or the shop status line, plus `chmod u+s …` |
| sound | strip notices only | an optional short `setuid` sound (audio thread); never on a crash |

```
Boot #57 · Bronze · merge window                                  uptime 1:04:12
♥ 22/50 cycles  🦆1  412 B uncommitted                          daemons strace
[ 3851.0] door_5: AC in 24:10, +192 B · ⚔ Off-by-One defeated
[ 3851.0] chmod u+s /bin/two-twins /sbin/ditto-ditto
          Ditto, ditto: two bits where the gap was.
────────────────────────────────────────────────────────────────────────────────
```

**The `setuid.<id>` lines.** One per bit, at most 70 columns, in the bible's
tone rules. The storyteller writes them; here are drafts:

```text
hello-world        Carved over the door, and now in your inode: hello, world
two-twins          Ditto, ditto: two bits where the gap was.
snicker-snack      The Jabberwock, slain once. The Wizards note it in pencil.
two-miles-high     Nearly two miles high. Somewhere a King looks up a rule.
stuff-and-nonsense The White Rabbit blows a trumpet. The court is sitting.
dull-reality       The bank, the book, your sister. Her eyes stay shut.
speedrun-2         Faster than the Rabbit. He checks his watch twice. Late.
regular-course     Every certificate on the Gryphon's wall is yours now.
```

- **The off switch.** `setuid_notices = false` turns the strip and cleared
  lines off; the screen still records everything (GR6: "every one tunable
  or off").
- **No clash with milestone beats.** When a bit and a milestone beat come
  from the same event (C8 and `milestone.doors.100`), the beat shows and
  the bit's line waits for the next safe point.

**Telemetry.** These go to the ML2 events table, local only (GR12):

| Kind | Payload |
|---|---|
| `setuid_set` | `id`, `kind`, `backfill`, `phase` (where it was noticed) |
| `setuid_open` | `spoilers`, `set`, `of` |
| `setuid_hint` | `id` |

There is no new alarm. GR11's late-night and over-budget alarms already
catch chasing. The doc predicts that skill bits are attempted at GRUB, not
at 1 a.m. *(inference)*.

## (i) The engine and store contract

**Storage: on `Meta`, like `Seen`.**

- **Precedent:** D39 put one-time scenes on Meta with no schema change.
- **Atomic:** bits land in the same `Checkpoint` transaction as the boot
  that set them (`./internal/store/boot.go:32`), so a crash between "boot
  saved" and "bit saved" cannot happen.
- **Rejected: a store table.** It would mean a second write, and that
  crash window.

| Where | Change | JSON / saves |
|---|---|---|
| `game.Meta` | `Setuid []SetuidBit`: `{ID, At, Boot, Backfill}`, in order set | `omitempty`; like `Seen` (`./internal/game/types.go:103`) |
| `game.Meta` | `Record Record`, the fold the bits read: `Folded` (last boot folded), `Nodes`, `Bosses`, `Verdicts`, `Daemons`, `Unlocked`, `Endings`, `BootTime`, `Recovered`, `RecoveredFree`, `Nightly`, `Backfilled` | `omitzero`; small and bounded (at most 17 nodes, 6 bosses, 8 verdicts, 7 daemons, 8 levels, 2 endings) |
| `game.Attempt` | `Samples int`: `:w` runs on this door | `omitempty`; set by a new action `SamplesRun{}`, which only counts. The `samples_judged` event cannot stand in for it: events are best-effort (`./internal/store/event.go:69`) and a dropped one would set S10 falsely |
| `game.State` | `Ending string`: "" or "wake" | `omitempty` |
| `game.Upsolve` | raises `Record.Recovered`, and `RecoveredFree` for an independent one | `./internal/game/meta.go:25` already knows `recovered` |
| `rules.Biome` | `Hidden`, `NeedFull`, `Ending`, `Cursed` | zero values play as today |
| `rules.MapNodes` | `trial` at `TierBossII`, the act II nodes' `Next` include it | `./internal/rules/map.go:115` |
| engine | `trial`'s exit gates on `State.Cycles ≥ MaxCycles`; `finish()` sets `Ending` from the node, skips the prestige unlock, counts the end as not a crash and not a full clear | `./internal/game/engine.go:774`, `./internal/game/ease.go:385` |
| `NightlySeed` | moves from `screens/meta` to `game` (pure) so P23 can read it | `./internal/screens/meta/grub.go:286` |
| `internal/setuid` (new) | `Table []Def{ID, Kind, Name, Text, Hint, Holds, Progress, Live}`; `Fold`; `Check`; `Completion`; `Backfill` | pure; imports `game` and `rules` only |
| store | migration 4: `ALTER TABLE boots ADD COLUMN ending TEXT NOT NULL DEFAULT ''`; `Stats.FullClears` counts `outcome = 'cleared' AND ending = ''`; `FinishedBoots()` streams snapshots oldest first | additive; the outcome CHECK (`./internal/store/migrate.go:32`) stays, because a wake is `cleared` |
| store | event kinds `setuid_set`, `setuid_open`, `setuid_hint` | the events table |
| app | calls `Check` at safe points; settings `setuid_notices`, `setuid_spoilers` | `cmd/katharsis/settings.go` |

**`Meta.Record` vs TIME+.**

- If D55's TIME+ lands with a boot-time part on Meta, `Record.BootTime`
  is that part.
- Otherwise `Fold` sums `up(B)` over finished non-trial boots.
- The speed completions read only boot time (§(a)).

**When checks run.**

```
app.Update → game.Step → effects
└─ s.Current == nil                                    a safe point (GR6)
   ├─ the boot ended → setuid.Fold(&m, s, end)   once: Record.Folded = s.Boot
   ├─ new := setuid.Check(&m, s, now)                  appends to m.Setuid
   │  ├─ PhaseCrashed → no notice; GRUB says "setuid +n" (GR3)
   │  ├─ PhaseCleared → the kept-block line
   │  └─ otherwise    → strip lines
   └─ store.Checkpoint(m, s)                           bits land with the boot
outside a boot: upsolve · trial end · apt install
   → Check(&m, nil or the trial, now) → SaveMeta
launch: !m.Record.Backfilled → Backfill, then SaveMeta
```

**Backfill for existing saves.**

```
Backfill(m, boots)
├─ for B in store.FinishedBoots():                     boots.state, oldest first
│    Fold(&m, B, end(B)); Check(&m, B, end(B), backfill)
├─ Check(&m, nil, now, backfill)                       Meta-only bits
└─ m.Record.Backfilled = true
```

**What backfill must not guess** (GR3: never fabricate):

- **Timestamps:** `At` is the boot's end and `Backfill` is set.
- **S10 needs `Attempt.Samples`.** Boots that predate it fall back to
  `samples_judged` events where the events table has them, from migration
  2 on. Otherwise S10 is not set.
- **`Recovered` is rebuilt from `Meta.Solves`:** an upsolve of a crash's
  door problem with `Crash.Lost > 0`, after that crash and before the
  next.
- **S15/S16 are set only if completion holds now and `BootTime` is under
  the bound now.** That is the one case where the bound was certainly met
  in time.
- **Legacy linear boots** set the boss bits, `hello-world` and `exit-0`.
  Map-only bits need a map boot.

**Determinism and tests.**

- **The function.** `Check` reads only `Meta`, one `State` and `now`, and
  `now` only for timestamps.
  - It never revokes a bit, it is idempotent, and its order is the
    table's.
  - Property tests run on `property_test.go`'s random action sequences:
    - Check twice is the same as Check once.
    - The set never shrinks.
    - Backfill over a fixture store gives the same set as live play over
      the same boots.
- **Table tests per bit**, on `screens/boot/fixture`'s map boots
  (`fixture/cycles.go`, `fixture/map.go`).
- **Guardrail tests:**
  - No setuid notice in `PhaseProblem` (GR6).
  - No setuid line in any crash golden (GR3).
  - No "streak", "in a row", "today" or weekday in setuid strings (GR1,
    GR2).
  - `internal/game` does not import `internal/setuid`, and no door draw
    reads `Meta.Setuid` (GR7).
- **Story tests:** every `setuid.<id>` line is at most 70 columns, and
  every Carroll name and quote is in the register.

## (j) Rewards: no

**No rewards, cosmetic or otherwise. Not even a GRUB theme.** The reasons:

1. **Themes are apt's byte sink.** Seven of the 19 packages are banner
   themes (`./internal/rules/rules.go:924-940`). A free theme undercuts the
   bank and pays in kind for something other than a learning act (GR4).
2. **Rewards for doing the activity undermine wanting to.**
   Completion-contingent rewards lowered intrinsic motivation (d = −0.36);
   feedback raised it (Deci et al. 1999). A setuid bit is feedback; a
   reward would make it a payment.
3. **Neither Team Cherry game pays for achievements in game.** In both,
   achievements are platform records; Silksong's Twisted Child unlocks only
   a menu style ([W-SSE]) *(inference)*.

The screen is the trophy. The secret ending changes the game only in the
one boot that takes it.

## (k) The guardrails, line by line

| # | How this design meets it |
|---|---|
| GR1 | no bit counts consecutive anything; copy test bans "streak" and "in a row" |
| GR2 | the nightly bit takes any date; no bit, hint or line reads the weekday or date; nothing expires |
| GR3 | no unlock cue on a crash screen; backfill is marked and conservative; the percentage counts only what happened; the Trial card states its costs |
| GR4 | no bit pays bytes, items or themes; TIME+ never appears as hours to collect |
| GR5 | no randomness: bits are deterministic functions of play; the Trial pays through `doorBytes` |
| GR6 | notices only at safe points, one or two lines, never blocking, with an off switch |
| GR7 | the engine never reads bits; the import test enforces it |
| GR8 | unset rows read "not set"; no "missed"; a passed speed bound reads "for a fresh store" |
| GR9 | easing never disqualifies a bit |
| GR10 | no backlog: bits are a record, not a list of things due |
| GR11 | unchanged; the existing alarms watch for chasing |
| GR12 | local only: Meta, the events table, no platform or network |
| D53 | names and barriers only on the screen, no lore; hints are terse; story lines show at game events |

## (l) Decisions for the user

Each has a default. They are independent.

1. **The name.** Default: **setuid bits**, on a `find / -perm -4000`
   screen. Alternatives: CTF flags, or badges.
2. **Where the screen lives.** Default: **a GRUB entry after `top`**. It
   fills the P8 box exactly. Alternative: a `top` tab on `s`, next to `r`
   retention.
3. **Spoilers.** Default: **`none`**: `-?????????` rows, and `?` reveals
   one hint at a time. Alternatives: `hints` or `all` as the default.
4. **Door-time speedrun bounds.** Default: **1:30:00 and 1:00:00**, reset
   once from the first ten real map clears (about 1/2 and 1/3 of the
   median), then frozen.
5. **The speed completions' clock.** Default: **boot time** (TIME+ less
   upsolves and trials). Bounds come from the learning simulator at the
   25th percentile; 60 h and 150 h are placeholders. Alternative: raw
   TIME+, which charges for upsolving and RTFM.
6. **TIME+ hour bits** ("100 hours in doors"). Default: **none**. Hours
   are the wrong target in both directions (`retention.md` §(b)3), and GR4
   bars paying for them.
7. **Rewards.** Default: **none** (§(j)).
8. **The Trial and Dull Reality.** Default: **build it** as §(f): gate ♥ at
   the cap at the Boss II exits; a cursed one-shot door; no prestige unlock;
   not a full clear. Alternatives:
   - Gate on carrying the core dump's problem as the trial door (the stolen
     tarts). This needs a dump.
   - Make the Trial ring 3 only (Wonderland's ending, before the glass).
9. **Backfill.** Default: **yes**, marked "from git log", conservative
   where history is thin.
10. **Which boots count.** Default: **built-in presets only**; easing never
    disqualifies. Alternative: any preset. A custom preset can set
    `timer_mult` to 10.
11. **When the percentage shows.** Default: **after the first three-act
    clear**. Alternative: from the start.

## (m) Pitch (for the coordinator to show the user)

> Achievements are **setuid bits**: 61 of them, on a `find / -perm -4000`
> screen off GRUB. Earned rows show `-rwsr-xr-x` with the date.
> Secrets are `-?????????` dotfiles with a hint if you want one. They pay
> nothing, on purpose: research says rewards for an activity undermine
> wanting to do it, and feedback helps.
>
> Every visible bit names its wall, Hollow Knight style: "Defeat the OOM
> Killer", "Clear a three-act boot in under 1:00:00 of door time". Skill
> bits are runs you set out to do:
>
> - speedruns on door time
> - clears with no WA, no duck, or no bought time
> - every door -O3
> - P8 clean
> - one bit per boss for beating its twist: Heisenbug without ever running
>   `:w`, the Jabberwock under half its hidden clock
>
> Completion runs to 100% (the divisions) and 112% (rings -2 and -3, plus
> one `???`). Speed completions time 100% and 112% on boot time. Upsolves
> and RTFM trials never count against you.
>
> The secret is Wonderland's ending. Reach the act II bosses with a full
> cycle pool, two miles high, and a hidden exit opens: the Trial. It is
> one shot, cursed, judged by the Queen. Win it and the cards fly up off
> the screen. You wake on the riverbank in plain terminal colors, and the
> man page from the prologue has an EXAMPLES section now: the problems you
> just solved. Your sister closes her eyes and the White Rabbit hurries
> by. It ends the boot early, unlocks nothing and costs bytes, so it's a
> discovery, not a shortcut. Every hint is something the game already
> says: the MLE line quotes Rule Forty-two.
>
> Two corrections from the sources. Hollow Knight has no 112% speedrun
> achievement; its Speed Completion is 100% under 20 hours. Silksong's
> act II ending is the **Twisted Bud** given to **Greyroot**, then beating
> Grand Mother Silk while cursed. Eleven calls are yours, each with a
> default.

## (n) What remains uncertain, and sources

- **Thresholds.** The speedrun and speed-completion bounds rest on the
  model player, whose clock is not the player's (`map.md` §(m)).
  Decisions 4 and 5 replace them with real and simulated data.
- **Retreat on the final.** X5 assumes the map build keeps allowing a
  retreat on a boss door.
- **The Trial's aim.** 0.55 is a guess at offsetting the curse. The
  simulator sets it.
- **TIME+'s shape.** It is not on main or in the `timeplus` workspace yet.
  `Record.BootTime` stands in until D55 lands.
- **Achievements and learning.** No study used a learning game with
  achievements on a skill objective. Mekler et al. found no competence
  gain from points and badges, so this doc claims feel and curiosity only.
- **Silksong.** It is recent (2025) and patched since; the wiki and press
  agree on the endings, but the ending count varies (five, or six with a
  cut ending).
  - Whether a cursed kill wins out over a ready Soul Snare is the
    researcher's inference from the wiki.
- **Hollow Knight's timer** details (pause, inventory) are player reports.
- **Star of Providence** is still patched (August 2026), so its list may
  change.
  - Whether a Sudden Death win also sets Intense is unverified.

**Sources** (industry unless marked):

- [S-HK] https://steamcommunity.com/stats/367520/achievements
- [W-HK] https://hollowknight.wiki/w/Achievements_(Hollow_Knight)
- [W-HKC] https://hollowknight.wiki/w/Completion_(Hollow_Knight)
- [W-HKWS] https://hollowknight.wiki/w/World_Sense
- https://hollowknight.wiki/w/Steel_Soul_Mode_(Hollow_Knight)
- [S-SS] https://steamcommunity.com/stats/1030300/achievements
- [W-SS] https://hollowknight.wiki/w/Achievements_(Silksong)
- [W-SSE] https://hollowknight.wiki/w/Endings_(Silksong)
- [W-TB] https://hollowknight.wiki/w/Twisted_Bud ,
  https://hollowknight.wiki/w/Rite_of_Rebirth ,
  https://hollowknight.wiki/w/Witch_Crest ,
  https://hollowknight.wiki/w/Greyroot
- [PCG] https://www.pcgamer.com/games/action/silksong-endings/ (secondary)
- https://www.pcgamesn.com/hollow-knight-silksong/endings (secondary)
- [B] Bloomberg, Jason Schreier, 2025-11-28, via
  http://web.archive.org/web/20260221061025/https://www.bloomberg.com/news/newsletters/2025-11-28/the-makers-of-hollow-knight-silksong-aren-t-done-just-yet
  (secondary, quoting the developers)
- [S-SoP] https://steamcommunity.com/stats/603960/achievements
- [W-SoP] https://starofprovidence.wiki.gg/wiki/Achievements ,
  https://starofprovidence.wiki.gg/wiki/Lethality ,
  https://starofprovidence.wiki.gg/wiki/Forbidden ,
  https://starofprovidence.wiki.gg/wiki/Kleines
- [S-DC] https://steamcommunity.com/stats/588650/achievements
- Deci, Koestner & Ryan (1999), *Psychological Bulletin* 125(6) (PR):
  https://depts.washington.edu/techdocs/papers/deciExtrinsicRewardsAndIntrinsicMotivation99.pdf
- Hamari & Eranti (2011), DiGRA (PR):
  https://dl.digra.org/index.php/dl/article/view/545
- Hamari (2017), *Computers in Human Behavior* 71 (PR):
  https://research.utu.fi/converis/portal/detail/Publication/27336812
- Mekler et al. (2017) (PR):
  https://glassmanlab.seas.harvard.edu/annotated_works/mekler17.pdf
- Jakobsson (2011), *Game Studies* (PR):
  https://gamestudies.org/1101/articles/jakobsson
- Cruz, Hanus & Fox (2017), *Computers in Human Behavior* 71 (PR)
- Blair, "The cake is not a lie":
  https://gamedeveloper.com/design/the-cake-is-not-a-lie-how-to-design-effective-achievements
- ID@Xbox (2020): https://news.xbox.com/en-us/2020/01/31/how-community-influenced-hyperdot-achievement-design/
- Charla (2018): https://gamedeveloper.com/design/id-xbox-chief-shares-tips-on-what-makes-a-good-achievement
- Steamworks: https://partner.steamgames.com/doc/features/achievements
- Carroll, *Alice's Adventures in Wonderland*, ch. I, XI, XII:
  https://www.gutenberg.org/cache/epub/11/pg11.txt (public domain; checked
  2026-10-15)
