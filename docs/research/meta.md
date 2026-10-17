# The core meta of katharsis: cycles, stakes, recovery and the map

Meta research thread (D47), 2026-10-15. **Status: proposed.** It asks the
user for the decisions in §(g). Nothing here is merged. The prototype is a
set of rules flags in the simulator only, in the `meta-research` workspace
(§(i)). Baseline: D46 as another agent lands it (Userland 15 min in every
preset, `TimerFitLo` 0.8, `TimerHeadroom` 1.5, door time only in uptime,
`nice` paid in bytes). Line numbers are as of this date on that workspace.

**Vocabulary.** The pool is called **cycles** (user's naming, 10-15),
drawn as `♥ 24/30 cycles`. A failure **burns** cycles. 10 cycles is one of
today's lives, so every "+1 life" becomes "+10 cycles".

**Evidence grades**, as in `learning-science.md`: *strong*, *moderate*,
*weak*, *speculative*. **Source labels**: *industry* (developer talk,
interview, patch note, official wiki), *PR* (peer reviewed), *secondary*,
*player report* (forum posts, used only where nothing better exists).
*(inference)* marks my own reasoning. No study here used competitive
programming, and every mapping to katharsis is inference.

## (a) Summary

**Recommendation: the user's pool, renamed cycles, with three changes.**
Start at 30 cycles. A failed hidden-test submit burns 2, a timeout or a
give-up burns 10, and a boss failure burns 20. The three changes:

1. **A failed room is lost, and the boot moves on.** It pays nothing and
   gives no upgrade pick, but the next room opens. This is ICPC triage:
   an unsolved problem costs time, not the contest.
2. **The boss burns 20 and is drawn again.** A healthy arrival can afford
   one retry. That gives the merge window before the boss a real choice:
   fsck to afford a retry, or commit.
3. **Prestige takes the cushion away.** WAs burn 3, heals drop to 5, fails
   burn 15, and P4 starts at 20 cycles with a one-shot boss.

**What it fixes.** In the simulator, a boot ends on its first mistake 2%
of the time instead of 14%. Userland crashes fall from 5–15% to about 1%.
Three crashes in a row (EOMM's churn trigger) drop from 8.8 to 0.3 per
player-year. The meta also prices every failure in one currency, which is
the user's ask ("varying lives").

**What it costs.** Full clears at P0 rise from about 50% to 82%. P4 lands
at about 42%, above G4's 30% ceiling, though today's rules with D46
already sit at 27–36%. The mean boot gets 40–60 min longer in door time,
because fewer boots end early. Suspending on the map (§(c)) is what makes
that acceptable. No candidate moved durable skill measurably in the
learning simulator.

| Rank | Meta | One line | Verdict |
|---|---|---|---|
| **1** | **D. Cycles** | 30 cycles; WA 2, fail 10, boss 20; move on after a fail; HP-cut prestige | **default** |
| 2 | C. Lawnmower hearts | 3 hearts, lost only to fails; WAs cost bytes (CF); one watchdog per biome pays 32 B unused | most contest-faithful; too forgiving (P0 89–92%, P4 50–55%) |
| 3 | M4b. ICPC clock | 3 lives for fails; a WA moves the door's deadline 5 min earlier | the right idea for WAs; the most forgiving and the longest (P0 90–94%, P4 67–69%) |
| — | M1. Pool as written, rooms drawn again | 30 / 10 / 2; a failed room must be cleared again | rejected: about +1h15 per boot, P0 88–91%, P4 63–67% |
| — | Stamina / "load average" | a meter that fills with failures and drains with clears | rejected: regeneration removes scarcity, and RT9 already uses the name |
| — | M3. Today's lives + watchdog | 3 lives, WA −1, one free fail per biome | rejected: WAs still cost a third of the pool; the user's "harsh" stays |

## (b) The default meta: cycles

```
door open ─┬─ :w             samples, free
           ├─ :Submit  WA/TLE/RE on a hidden test   −2   (CE and sample failures free)
           ├─ timer runs out                         −10  boss −20 ─┐
           ├─ :q! give up                            −10  boss −20 ─┤
           │                                                         ├─ door closes as failed, rated,
           │                                                         │  back as a regression in 3 days;
           │                                                         │  no pay, no upgrade pick;
           │                                                         │  next room (boss: a new boss door)
           ├─ R retreat (swap the door, same room)   −5
           └─ AC                                     pay, upgrade pick (−O3: two)
crash = a burn that leaves 0 or less → kernel panic, core dump, editorial
```

| Event or item | Cycles (P0) | Today (lives) | Why |
|---|---|---|---|
| Start / max | 30 / 50 | 3 / 5 | the user's number; max is today's 5 lives |
| Failed hidden-test submit | −2 | −1 (a third of the pool) | the user's number. 2 of 30 is ICPC's 20 of 300 min (6.7%); CF's −50 on a 500 is 10% |
| CE, or a submit that fails a sample | free | CE free | all three contests charge nothing for these; `:w` already judges samples |
| Timeout | −10, the room is lost, the boot goes on | crash (Userland: −1 life, room drawn again, D44) | a timeout stops ending the boot; ICPC charges an unsolved problem only time |
| Give-up `:q!` | −10, same as a timeout; the editorial waits for the upsolve | crash, and the editorial opens | the user's decision (10-15) |
| Boss timeout or give-up | −20, then a new boss door | crash | a healthy arrival gets one retry (StS's rest site before every boss) |
| Retreat `R` | −5, same room, new doors | −1 life | half a fail: the room still has to be cleared |
| Cursed door | one failed submit crashes; pays 2× | same | the player opts in, and a plain door is always on offer (Dead Cells' curse) |
| ECC (upgrade pick) | +10 | +1 life | one life |
| fsck (merge window) | +10 for 64 B | +1 life | Dead Cells' fountain, StS's rest: heals are scheduled and bought |
| Heal on AC | none | none | scarce recovery is the tension (§(d)2) |
| `nice` daemon | +20% timer, doors pay ×0.8 | +20%, a WA costs 2 lives | D46 as settled |
| `chrt` daemon | −20% timer, ×1.5 pay | same | risk paid in timeouts, which now cost cycles |
| `kexec` daemon | one free retreat per biome | same | |
| D34 floor easing | +10 cycles | +1 life | keep the machinery; retarget the forecast (below) |
| Streak governor | +10 cycles and the EaseClear aims | +1 life | still binds at P3–P4 (G1 up to 15%) |
| Core dump | only a crash (0 cycles) dumps, with today's double-loss rule | same | the bloodstain is unchanged |
| Regression | every failed door returns in 3 and 17 days | crashed doors and D35 failures | a failure teaches even when it does not crash the boot |

**Prestige** stacks today's ascensions (blind, mystery, cursed, timer,
ducks) with one HP cut per level. Cutting healing is how Dead Cells, Slay
the Spire and Hades all raise difficulty (§(d)2).

| Level | Existing modifier | Added in cycles |
|---|---|---|
| P1 | blind +15%, timer ×0.95 | a failed submit burns 3 |
| P2 | −1 duck, mystery +5%, cursed +4% | ECC and fsck heal 5 |
| P3 | cursed +5%, timer ×0.9 | a fail burns 15 (boss 25) |
| P4 | −1 life, cursed +4%, timer ×0.9 | start at 20 cycles; the boss is one-shot (The Halting Problem) |

**D34's forecast needs retargeting.** It assumes a failed door ends the
boot (`ease.go` `forecast`). Under cycles it overstates the crash chance
by an order of magnitude, so it would hand out +10 cycles that no boot
needs. With easing off, the simulator's Userland crash rate at P0 is
0.5–1.3% for every rating (§(d)1). So the forecast should count the cycles
(P(burns ≥ start) in the first biome). The governor stays as it is: at P4,
G1 reaches 8–15%. This is engineering, not a decision.

### What the user sees

The pool shows as a count with one glyph, never a row of 30 hearts. Each
cost is on screen before it is paid, because uncontrollable costs hurt
most (Byron et al. 2010, §(d)3). The `:Submit` reply already names the
cost.

```
HUD     ⏱ 23:41  ♥ 24/30 cycles  🦆1  46B
:Submit WA 37/52 · #38 line 2 · −2 ♥, 22 cycles left
timeout kernel: door_4 timer expired, burned 10 cycles, 14 left · room lost
map     Userland ✓ ✗ → The Heap ▸ ○ ○ → ⚑ Off-by-One (−20 a try)
```

## (c) The map is the hub: suspend between doors, never inside one

**User's rule (10-15):** "it should save progress in map but like you cant
quit inside a level". Since D46 the clock runs only inside doors. So
stopping between doors costs nothing, and the map can be where a boot is
started, resumed or ended.

```
launch
 ├─ no boot in flight ─────────────> GRUB: new boot (preset · prestige · seed · nightly)
 └─ boot in flight
     ├─ suspended between doors ───> MAP (hub)
     │    enter  resume: the route, doors or merge window it stopped at
     │    n      new boot: "ends boot #42 as a crash: 46 B lost, the core dump on Karen destroyed"
     │    u      upsolve the core dump or a due regression (not a door on offer in this boot)
     │    q      quit to the shell
     └─ a door was open (kill -9, lost terminal, sleep)
          └──> DOOR reopens, same deadline; a pending submit is judged again (today's snapshot)
               deadline already passed ──> timeout: −10 cycles, room lost, then MAP

between doors (route · doors · upgrade · merge):  m → MAP · q or ctrl+c → saved, suspended, shell
inside a door:  ctrl+c → "no quitting inside a door: solve it · R retreat −5 · :q! give up −10 ·
                          closing the terminal does not stop the clock"
```

**Rules, each with its precedent:**

| Rule | Default | Precedent |
|---|---|---|
| Where a boot can be suspended | between doors only, on any screen outside a door | Returnal's Suspend Cycle (2.0): one suspend point, blocked in boss fights (secondary: patch-note coverage); Hades: a safe quit only after an encounter or on entering a chamber; FTL greys out Save+Quit in combat (industry; player report for FTL) |
| What resuming does | opens where it stopped, saved after every step as today | Balatro's Play screen defaults to Continue; StS reloads at room start (industry; player report for StS) |
| How many boots in flight | **one**, shared by standard, short, patch and the nightly | StS2 keeps one run per profile (a mod exists to get around it); Balatro keeps one per profile (industry) |
| Starting a new boot while one is suspended | counts as a crash: uncommitted bytes lost, the old core dump destroyed, logged as abandoned | LocalThunk (Balatro): "Starting a new run is the same as losing the current one!"; Hades' Give Up is worse than dying (industry; player report for Hades) |
| ctrl+c in a door | one refusal line; no y/n quit (D31's two-step quit goes away) | user's rule |
| Involuntary exit in a door | the deadline stays absolute (D31); the door reopens; a submit in flight is judged again | a contest clock keeps running for a team whose machine fails (inference); FTL players kill the process to save-scum (player report) |
| A process kill and the core dump | never makes or dodges a dump; only an in-game crash does | Dead Cells: copying saves "is essentially removing the permadeath aspect" (Filby, industry) |

**Why one slot.** Suspending at doors already splits a 3 h boot into
sessions of under an hour. A second slot fixes nothing, and it lets a bad
boot be parked instead of lost, which erodes the stakes (inference). The
nightly needs no slot of its own, because nightlies never expire (GR2,
RT15): a suspended boot only delays one. One `Meta` also backs every boot.
`store.Checkpoint` abandons any older running boot today (`store/boot.go`
`Checkpoint`), so one slot needs no store change.

**Why the deadline stays absolute.** If the clock paused while the game
was closed, a kill -9 would buy untimed thinking on a statement already
read: the in-door suspend the user forbids. If the door were forfeited
instead, as Hades does on a crash, a terminal glitch would cost a 50-min
door. Under cycles the worst case of a lost terminal is now bounded: an
expired door on resume is a timeout, so it burns 10 cycles and the boot
goes on. The resume note (`app/debrief.go` `resumeNote`) stays.

**How the hub interacts with the rest:**

- **Core dump.** A suspend leaves it alone. It is at risk only once a new
  boot starts (`State.AtStart.Dump`), and the map shows it.
- **Upsolves while suspended.** Allowed for the dump and for due
  regressions. Refused for a problem on offer in the suspended boot: a
  door drawn but not opened would otherwise be pre-solved. (10-16, user:
  "this shouldnt be in a active run map": the map has no upsolve; GRUB's
  `upsolve…` lists the dump, the regressions due and the doors boots lost,
  between boots.)
- **Nightly.** A seed choice at GRUB, in the one slot. Its doors are fixed
  by the seed, so a suspended nightly resumes the same doors.
- **Prestige and rules.** Already snapshotted per boot (`State.Rules`). A
  suspended P2 boot stays P2, and a prestige unlock only happens at a
  clear.
- **Trials (RTFM).** Give them their own slot. A trial never touches the
  dump or the governor (`engine.go` `crash`, `ease.go` `countEnd`), so a
  parked trial cannot dodge a boot's stakes. I resolved this one without
  asking. Say so if a trial should end a suspended boot instead.
- **Session budget (RT12).** The map counts as a boot boundary for nudges
  (GR6). Suspend is the natural place for the bedtime line.
- **Comeback (RT8).** It eases only new boots. A boot resumed after days
  away plays as it was drawn, with no date pressure (GR2).

## (d) Evidence

### 1. Simulator: every forgiving meta trades one-strike endings for clears and length

Union pool (USACO + Codeforces, 7,146 judgeable). Model player
`DefaultPlayer` with `GiveUpAt` 0.75 (it gives up a door it won't clear at
¾ of the timer, when a give-up costs no more than a timeout). Each cell
covers P0 boots 51–350 over seeds 7 and 8, 600 boots in all; rows marked †
cover boots 51–250, 400 in all. Measures:

- *one-strike*: the share of crashed boots whose only mistake (a charged
  failed submit, timeout or give-up) was the one that crashed them;
- *crash door*: the median number of doors opened at a crash;
- *door time*: D46's uptime, in-door time only;
- *clear med*: the median full clear's door time.

| Meta (P0) | G1 % 900 / 1000 / 1600 | Full % 1000 / 1600 | One-strike % of crashes | Crash door | Door time mean | Clear med |
|---|---|---|---|---|---|---|
| M− pre-D46 † | 11.5 / 7.8 / 6.0 | 51.5 / 53.8 | 17–27 | 4–5 | 2h08–2h30 | 1h50–2h07 |
| **M0 today + D46** | 14.5 / 6.8 / 6.3 | 48.7 / 49.3 | 19–27 | 4–5 | 2h17–2h29 | 1h58–2h18 |
| M1 pool as written † | 4.0 / 1.2 / 0.8 | 90.2 / 89.8 | 9–15 | 7–8 | 3h30–3h46 | 2h44–3h07 |
| M1c pool, move on † | 1.0 / 1.0 / 0.8 | 89.2 / 90.2 | 13–19 | 8 | 3h07–3h37 | 2h21–2h50 |
| **D cycles (default)** | 1.7 / 0.8 / 0.8 | **82.3 / 83.0** | **4–12** | 7 | 3h04–3h25 | 2h20–2h43 |
| E = D, easing off | 1.3 / 0.5 / 0.8 | 83.2 / 83.0 | 6–10 | 7 | 2h59–3h14 | 2h14–2h32 |
| F = D, boss one-shot | 1.7 / 1.0 / 1.2 | 72.7 / 71.2 | 12–20 | 6 | 2h53–3h03 | 2h12–2h26 |
| B4 = D at 20 cycles † | 2.2 / 1.2 / 2.2 | 78.2 / 81.0 | 7–14 | 7 | 2h50–3h12 | 2h13–2h36 |
| C lawnmower hearts | 1.5 / 0.5 / 1.0 | 88.7 / 91.5 | 11–20 | 8 | 3h18–3h33 | 2h30–2h46 |
| M3 lives + watchdog | 17.8 / 7.3 / 3.7 | 69.2 / 74.8 | 1–9 | 5–6 | 2h59–3h06 | 2h18–2h53 |
| M4b ICPC clock | 1.7 / 0.3 / 1.0 | 91.7 / 90.8 | 10–18 | 5–8 | 3h12–3h22 | 2h30–2h46 |

As a share of **all** boots, a boot that ends on its first mistake is
about 14% under M0 (51% crash × 27%), 2% under D, and 5% under F.

**Prestige, each level played on its own** (full clear %, players 1000 /
1600, boots 51–350, seed 7):

| Meta | P0 | P1 | P2 | P3 | P4 | P4 G1 % | P4 door time |
|---|---|---|---|---|---|---|---|
| M0 today + D46 | 52 / 51 | 48 / 48 | 39 / 52 | 39 / 47 | **27 / 36** | 28 / 13 | 1h26 / 1h38 |
| B1 = D without the HP cuts | 83 / 82 | 79 / 82 | 78 / 76 | 68 / 73 | 66 / 61 | 5 / 5 | 2h13 / 2h35 |
| **D cycles** | 83 / 82 | 79 / 81 | 74 / 74 | 65 / 70 | **43 / 42** | 13 / 8 | 2h00 / 2h05 |
| D′ = D, P4 fails burn 20 | 83 / 82 | 79 / 81 | 74 / 74 | 65 / 70 | 47 / 38 | 11 / 10 | 1h50 / 2h05 |
| F boss one-shot | 76 / 71 | 62 / 68 | 69 / 61 | 55 / 63 | 43 / 42 | 13 / 8 | 2h00 / 2h05 |
| C lawnmower hearts | 89 / 92 | 87 / 86 | 76 / 78 | 66 / 75 | 55 / 50 | 4 / 5 | 1h55 / 2h12 |
| M4b ICPC clock | 92 / 89 | 89 / 88 | 87 / 85 | 78 / 80 | 67 / 69 | 6 / 5 | 2h27 / 2h39 |

**Climbing from a new player** (600 boots, seed 7). P4 unlocks at boot
24 (1000) and 19 (1600) under M0, and at boot 16 under D. Once at P4, the boots there
fully clear 31–38% under M0 and 38–48% under D. In every meta the ladder
ends within the first 25 boots: the 60-rated-door gate (D31) plus four
clears.

**Learning player** (`DefaultLearning`: γ 3 flat, sessions and churn;
ratings 900/1100/1300; 30 players a year each):

| Meta | Durable gain | Hours / year | Boots | Durable gain per 100 h | Quit % | LLL runs / year |
|---|---|---|---|---|---|---|
| M0 today + D46 | +29 | 213 | 82 | 13.6 | 67 | 8.8 |
| **D cycles** | +31 | 291 | 86 | 10.7 | 73 | 0.3 |
| F boss one-shot | +31 | 277 | 85 | 11.2 | 70 | 1.3 |
| C lawnmower hearts | +33 | 325 | 88 | 10.2 | 70 | 0.1 |
| M4b ICPC clock | +32 | 310 | 85 | 10.3 | 73 | 0.1 |

**Reading the evidence:**

- **Survivable timeouts are the whole effect.** Every meta in which a
  timeout no longer ends the boot lands at 70–94% full clears at P0. How
  the pool is denominated barely matters next to that.
- **Drawing the failed room again is what makes boots long.** Moving on
  (M1c) and the boss burn (D) take back some of the length. A one-shot
  boss (F) takes back the most, but it removes the fsck-before-the-boss
  choice.
- **The HP cuts to prestige do work.** P4 falls from 61–66% (B1) to 42–43%
  (D). Burning more on fails at P4 (D′) changes nothing: at 20 cycles, a
  15-cycle fail already leaves one more.
- **Easing is redundant at P0 under cycles** (E vs D). It still matters at
  P3–P4.
- **The learning simulator cannot rank these.** It credits learning per
  door, and the metas change hours per boot more than doors. Durable gain
  differs by 2–4 points over 30 players, which is noise. Hours rise
  35–50% and gain per hour falls about 20%. Quit rates are all high
  (67–73%), and the churn model sees only crash streaks, which cycles all
  but remove.

### 2. Games: price failures differently, schedule the heals, make every net visible

| Game | Cheap failure | Expensive failure | Recovery | Safety net, and how visible | Source |
|---|---|---|---|---|---|
| Dead Cells | chip damage, 80% of it winnable back by hitting ("Recovery") | curse: one hit kills until 10 kills | flask heals 60%, refilled at biome passages; BC1–4 remove the fountains in steps | the curse is a skull with a counter over the head; opt-in for loot | deadcells.wiki.gg (Health Flask, Boss Stem Cells, Curse, Mechanics), industry |
| PvZ1 | a zombie reaches a lane | a second one in the same lane ends the level | none in a level; the mowers reset each level | the lawnmower: one per lane, single use, on screen; **each unused one pays $50** from 2-1 on | plantsvszombies.wiki.gg (Lawn Mower), industry |
| Hades | damage | death ends the run | fountains; Centaur Hearts +25 max HP | Death Defiance, 1–3 per run, revives at 50%, shown as icons; God Mode +2% resistance per death, max 80% | hades.wiki.fextralife.com (Mirror of Night); caniplaythat.com 2021 (God Mode), industry |
| Slay the Spire | HP lost per fight, carried all run | death | rest site: heal 30% *or* upgrade a card; the room before each boss is always a rest site; A5 cuts the post-boss heal to 75% of missing HP, A6 starts 10% hurt | none; HP is the only buffer | slaythespire.wiki.gg (Rest Site, Ascension), industry |
| Spelunky | 1 heart per hit, of 4 | spikes and boulders kill outright | a damsel's kiss, +1 heart | the Ankh in Spelunky 2: one revive | spelunky.fandom.com (Health), industry |
| Balatro | a discard (3, a separate budget) | missing a blind's score ends the run | none | Mr. Bones: catches only a near miss (≥ 25% of the score), then self-destructs; **each unused hand pays $1** | balatrowiki.org (Hands, Discards, Mr. Bones), industry |
| Into the Breach | a damaged mech | a building hit costs grid power (start 5, max 7) | reputation buys grid | Reset Turn, once per battle; Resist, a small chance to shrug off a hit | Wikipedia; GameSpot review; Cliqist 2018 (Justin Ma), industry |
| Celeste | death resets the screen | none | instant | Assist Mode (speed 50–100%, invincibility) with text that names it without judging | Vice 2018 (Thorson), industry |

What the designers said, and what it means here:

- **Price failures differently, and say the price.** Every game above has
  a cheap channel and an expensive one: Balatro's discards against its
  hands, Into the Breach's free move undo against its single Reset Turn,
  Dead Cells' winnable-back chip damage against the curse. In katharsis,
  a WA is information, so it is the discard and costs 2. A timeout is the
  heart and costs 10 (inference).
- **Tension comes from scarce recovery, not from a small pool.** Dead
  Cells (BC1–4), StS (A5, A6, A14) and Hades ("Lasting Consequences",
  −25% healing per rank) all raise difficulty by cutting healing. Hence no
  heal on AC, and P2 halves the heals (inference from industry sources).
- **Keep the safety net rare and visible.** Ma on Resist: it made a run's
  last moments "more palatable", but "we also didn't want the player to
  rely on this mechanic" (Cliqist 2018, industry). Mr. Bones catches only
  a near miss. Under cycles the pool itself is the net, so D adds no
  watchdog on top.
- **Avoid whimper endings.** Ma: without Resist, a doomed player would
  "abandon the timeline… a whimper rather than a bang". A big pool drained
  by many small charges makes the whimper likely (inference). D answers
  it with large fail costs against a 30 pool (three bangs, not thirty
  cuts) and with fsck as the way back.
- **Many small penalties read as friction.** Yu (Spelunky book): "the real
  culprits were all of the aggregate smaller annoyances". That is the case
  against making WAs expensive (industry).
- **Don't let the player blow through it.** Kasavin: "If you could just
  blow through it, what's interesting about the game goes away" (Inverse
  2021, industry). That is the case against M1 and M4b's 90% clears, and
  for prestige that bites.

**What exactly makes PvZ1's meta great:**

| PvZ1 | Source | katharsis today | Under this proposal |
|---|---|---|---|
| A new toy at the end of most levels; peripheral mechanics introduced "very slowly" | Fan, GDC 2012 (gamedeveloper.com); wiki | a package every 2–3 boots (5–8 h); capabilities from RTFM | **gap, not closed here**: the unlock rhythm is per boot, not per level (inference) |
| The zombie lineup is shown, then you pick a seed loadout | wiki (Seed slot); Fan | door cards show title, tags, timer and reward; libs are linked before the first door | already there |
| A visible one-shot net per lane that pays $50 if unused | wiki (Lawn Mower) | none; lives are pooled | the pool is the net; the watchdog is candidate C |
| Money and the shop held back until about level 10; one purchase on the first visit | Fan; talk notes (secondary) | apt is open from boot 1 | fine; apt prices already pace it (RT3) |
| Losing costs nothing permanent, and a level is minutes long | wiki, inference | a crash costs 1–2 h of a boot | crashes become rarer (51% → 17% of boots at P0), and suspend makes stopping free |
| Hints only for players "doing the wrong thing", so others "feel smart" | Fan | ducks on demand; tip.* lines once | already close |

The user's "pvz 1 has a great meta" most likely means this loop: cheap
losses, a visible net, and a constant trickle of new tools (inference).
Cycles and the map handle the first two. The third is a progression
question for the apt and curriculum owners (§(f)).

### 3. Contest scoring: solves dominate, WAs are a small tax, unsolved problems cost time

| Contest | Ranking | A wrong submission | Free | WAs on a problem never solved |
|---|---|---|---|---|
| ICPC World Finals | solved count, then penalty minutes | +20 min, only if the problem is solved later | compile error | free |
| Codeforces Div 1/2 | points | −50, and points decay with time; floor at 30% of the problem's value | failing (pre)test 1; CE | free |
| CF Educational, Div 3 | ICPC-style | +10 min | | free |
| AtCoder ABC/ARC/AGC | score, then time | +5 min | CE | free |
| IOI | sum of subtask maxima | none; at most 50 submissions a task | | n/a |

Sources: ICPC WF Judging Notes (image.icpc.global/wf2026/doc/Judging-Notes.pdf);
codeforces.com blog entries 133094, 456 and 76526 (search snippets: the
site returned 403); atcoder.jp contest rules pages (abc133, arc185, agc070);
ioi2020.sg/rules. All industry.

**WF 2025, recomputed from the official scoreboard** (139 teams; the
recomputation matched every published penalty):

- Teams accepted 61% of solves on the first try and made 0.66 WAs per
  solve. The top 12 still made 0.52.
- WAs were **10.4%** of all penalty minutes; time to AC was the other
  90%.
- 41% of rejected runs were on problems never solved, so they cost
  nothing on the scoreboard. Unsolved problems drew 3.4 rejections on
  average and up to 16, which looks like a sunk-cost trap.

So ICPC trains "solve more, don't burn time" more than "never WA".
Mapped onto cycles: a WA should be a small, predictable tax (2 of 30 is
ICPC's 6.7% share). A timeout should cost the problem's worth of the run.
The boot should go on after an unsolved room, as a contest does
(inference).

### 4. Learning science: costs should be small, known in advance, and paid for informative errors

| Finding | Source | Grade | Consequence |
|---|---|---|---|
| Error-management training: transfer d = 0.56, novel tasks d = 0.80, but **only with clear feedback** (d = 0.56; unclear feedback, n.s.) | Keith & Frese 2008, *J Appl Psych* 93:59 (24 studies) | moderate | a WA must teach: the failing test number, and its input under strace or after the door |
| Stakes near a month's spending cut performance to 19.5% of the maximum, against 35–37% at lower stakes; 10× more money at moderate stakes added nothing | Ariely, Gneezy, Loewenstein & Mazar 2009, *REStud* 76:451 | low–moderate | no single failure should threaten a large share of what the player holds |
| Outcome pressure hurts working-memory-heavy, rule-based tasks; high-WM people choke most on math | Beilock & Carr 2005; DeCaro et al. 2011 | moderate | CP is WM-heavy; fear of the boot ending sits on top of the timer |
| Challenge beats threat appraisal, with small effects (r ≈ .08–.18) | Hase et al. 2025 (62 studies) | moderate on direction | show the cost as a resource to spend, not a sword |
| Mild evaluation helps creativity, heavy evaluation hurts it, and uncontrollability hurts linearly | Byron, Khazanchi & Nazarian 2010, *JAP* 95:201 | moderate | every cost shown before it is paid |
| Loss framing raises effort by at most about 0.1 SD; field effect 0.02 SD; it helped only the ~20% who prefer it | Ferraro & Tracy 2022, *Exp Econ* 25:1441 (meta-analysis) | moderate | a pool you lose from does not buy effort by itself; it is a stakes device, not a motivator |
| A penalty for wrong answers makes people skip, risk-averse people most; skippers scored lower at equal knowledge | Baldiga 2014, *Mgmt Sci* 60:434; Coffman & Klinowski 2020, *PNAS* 117:8794 | moderate–strong | an expensive WA would train never submitting; keep it a small tax |
| Accuracy rises toward its ceiling with time; deadlines that are too short encourage guessing | Heitz 2014, *Front Neurosci* 8:150 | strong | tight timers breed submit-spam; D46's longer timers help the WA tax work |
| Time pressure raises output and lowers quality in software work, more so on algorithmic tasks | Kuutila et al. 2020, *IST* 121 (102 studies) | moderate | same |
| A difficulty is desirable only if the learner can respond to it | Bjork & Bjork 2011 | strong | prestige's cuts come after the player has the tools |
| 85% rule: optimal training accuracy for a gradient learner | Wilson et al. 2019 | speculative for CP; no human replication found | door targets stay as they are; this meta changes run survival, not door success |

**What failure costs keep effort high without choking** (inference from
the rows above):

- No single failure should take more than about a third of the pool. In
  D a fail is 10 of 30 and a WA is 2.
- Every cost should be known before it is paid, in the same unit.
- The informative error (a WA) should cost the least.
- The uninformative one (sitting out the clock) should cost the most.
- Costs must rise only once the player can respond, which is prestige.

## (e) The candidates, mapped to every mechanic

| Mechanic | D cycles (default) | C lawnmower hearts | M4b ICPC clock | M0 today + D46 |
|---|---|---|---|---|
| Pool | 30 / 50 cycles | 3 / 5 hearts | 3 / 5 lives | 3 / 5 lives |
| Failed submit | −2 | 10% off the door's pay, floor 30%; no heart | the deadline moves 5 min earlier | −1 |
| Timeout, give-up | −10, room lost, boot goes on | first per biome: the watchdog; then −1 heart, room lost | −1 life, room lost | crash (Userland: −1, D44) |
| Boss failure | −20, new boss door | −2 hearts, new boss door | −1 life, new boss door | crash |
| Retreat | −5 | −1 heart | −1 life | −1 life |
| Unused safety net | — | watchdog pays 32 B at biome end | — | — |
| nice (D46) | ×0.8 pay | ×0.8 pay | ×0.8 pay | ×0.8 pay |
| chrt | −20% timer, ×1.5 pay | same | same; penalty minutes bite harder | same |
| fsck, ECC | +10 | +1 heart | +1 life | +1 life |
| D34, D44 | +10; D44 is subsumed (every timeout is survivable) | +1; subsumed | +1; subsumed | +1; D44 Userland only |
| Governor | +10 and the EaseClear aims | +1 and aims | +1 and aims | +1 and aims |
| Prestige | the existing ladder + WA 3, heal 5, fail 15, P4 20 cycles and a one-shot boss | the existing ladder; P2 drops the watchdogs; P4 one-shot boss | the existing ladder | the existing ladder |
| Core dump | crash at 0 only | crash only | crash only | every crash |
| Cursed door | one failed submit crashes | same | same | same |

Each candidate's strongest and weakest point:

- **C** is closest to the contest evidence: WAs cost reward, not
  survival. It is also the most PvZ-like. But it stacks two nets (the
  hearts and a watchdog) and lands at 89–92% at P0. It would need the
  watchdog cut to one per boot and an HP-style ladder to reach D's
  numbers. That leaves three currencies (hearts, watchdog, bytes) where D
  has one.
- **M4b** charges a WA in the currency ICPC actually charges, time. That
  makes a WA's cost rise as the deadline nears, which is what makes ICPC
  WAs hurt late in a contest. The simulator cannot show its benefit: the
  model's WA rate is fixed and does not react to the price. Its numbers
  are the most forgiving, and it adds a moving deadline to explain. Worth
  keeping as an option for the WA price only (§(g) 3).

## (f) What remains uncertain

- **The model player does not react to prices.** Its WA rate, its give-up
  point (¾ of the timer, only on doors it will fail, which it knows in
  advance) and its spending are fixed. Any claim that a price builds a
  habit (testing before submitting, moving on) rests on §(d)3–4, not on
  the simulator.
- **The absolute times are the model's.** Its median solve is 30 min at
  gap 0 and sigma 0.5. Boot #1's two clears took 6:04 and 6:42. Read
  door-time differences between metas, not the levels.
- **G4 may be the wrong band now.** Its ceiling of 0.30 at the top
  prestige comes from the design's 20% full-clear aim. M0 itself sits at
  27–36% at P4 with D46. D's 42% needs either a deeper ladder or a new
  band. The user must choose (§(g) 5).
- **The prestige ladder is short in every meta.** P4 arrives by boot
  15–24. The unlock rhythm PvZ is loved for (a new tool every few minutes)
  is not in this meta. It belongs to apt, RTFM and capabilities, and it
  needs its own thread.
- **Noise.** One cell is 300–600 boots, so full-clear rates carry about
  ±2–3 points of standard error. The learning comparison (30 players) is
  noise on durable skill.
- **M3's 17.8% G1 at rating 900** differs from its 7.2% at 400 boots. I
  did not chase it, since M3 is rejected on other grounds.
- **Codeforces and ICPC rule text** came partly from search snippets
  (codeforces.com returned 403; icpc.global renders only in a browser).
  The Judging Notes PDF and the AtCoder pages were read directly.
- **The GDC talks themselves** (Dead Cells 2019, Slay the Spire 2019, Into
  the Breach 2019) could not be opened. Their HP content is not found.
  The quotes above come from interviews.

## (g) Decisions for the user

Each fork has a recommended default. The forks are independent unless
noted.

1. **Boss: one retry at −20, or one-shot.**
   - Default: retry.
   - One-shot (F) lowers P0 full clears from 82% to 72% and shortens the
     mean boot by 10–20 min.
   - It also makes fsck before the boss pointless, and it raises
     one-strike crashes from 4–12% to 12–20% of crashes.
2. **After a failed room: move on, or draw the room again.**
   - Default: move on.
   - Drawing again (today's D44, M1) adds about 20–40 min per boot and
     raises P0 to 88–91%.
3. **Price of a failed submit.**
   - Default: 2 cycles (your number; ICPC's share).
   - Codeforces-style: 10% off the door's pay, floor 30%, no cycles.
   - ICPC-style: the door's deadline moves 5 min earlier.
   - The contest research favours a reward or time price. The simulator
     cannot separate them.
4. **Give-up: a flat −10, or cheaper when early.**
   - Default: flat, your decision. A give-up then pays off only in time
     saved.
   - Scaled: −10 × (share of the timer used), floor −3. That makes triage
     strictly cheaper, which is the ICPC habit, but it is a second number
     to read.
5. **Top prestige: accept about 42% full clears at P4, or deepen the
   ladder.**
   - Default: deepen it. Add P5–P8 that only cut cycles (heals 0, fail
     20, WA 4, start 15), Hades Heat style, so the ladder lasts past boot
     25.
   - The alternative is to widen G4 to 0.15–0.45.
6. **Starting a new boot while one is suspended.**
   - Default: it counts as a crash (uncommitted bytes lost, the old dump
     destroyed).
   - Today an abandon is free (`app/boot.go` `abandon` only marks it).
     That would let a doomed boot be dropped to save the dump.

## (h) Pitch (for the coordinator to show the user)

> Your 30-pool, named **cycles** (`♥ 24/30 cycles`). A failed submit burns
> 2, a timeout or a give-up burns 10, and the boot goes on: the room is
> lost and you move to the next, like skipping a problem at ICPC. The
> boss burns 20, so you get a second boss problem only if you arrive
> healthy, which makes the fsck before it worth buying. Prestige takes the
> cushion away: WA 3, heals halved, fails 15, and P4 starts at 20 cycles
> with a one-shot boss.
>
> In the simulator, a boot ends on its first mistake 2% of the time
> instead of 14%. You almost never crash in Userland. Three crashes in a
> row go from 9 a year to almost none. The price: about 80% of P0 boots
> clear and boots run 40–60 min longer. That is why the map becomes the
> hub. You can stop between any two doors and resume later. Inside a door
> there is no quit, only solve, retreat (−5) or give up (−10), and the
> clock keeps running if the terminal dies.
>
> Six calls are yours (boss retry, move on, WA price, early give-up, a
> deeper ladder, abandon = crash). Each has a default.

## (i) Implementation notes

### The rename to cycles touches these user-visible strings

| Where | Today | Becomes |
|---|---|---|
| HUD status (`screens/boot/hudbar.go:49`), vim border (`app/app.go:498`, `:517`), problem line (`app/problem.go:395`) | a row of hearts, `units.Hearts` (`units/units.go:95`), `ui.BorderHearts` | `♥ 24/30 cycles`, a count (30 hearts do not fit) |
| `:Submit` reply and confirms (`app/confirm.go:49`, `:51`, `:68`, `:81–90`; `hud/hud.go:45`) | "a fail costs ♥", "-1 ♥, 2 left", "retreat: -1 ♥" | "−2 ♥, 22 cycles left", "retreat: −5 ♥" |
| dmesg strip (`screens/boot/dmesg.go:60`, `:61`, `:74`, `:407`, `:412–414`) and tips `tip.door`, `tip.fail` (`:601`, `:607`) | "lives: ♥♥♡", "a failed submit costs ♥" | "cycles: 24/30", "a failed submit burns 2 cycles" |
| Engine dmesg (`game/engine.go:315`, `:593`, `:666`; `game/ease.go:108`, `:466`; `game/spend.go:52`, `:75`) | "out of lives", "+1 lives", "lives are full" | "out of cycles", "+10 cycles", "cycles are full" |
| Notices (`screens/boot/notice.go:25`, `:67`), map (`bootmap.go:298`, `:302`), merge (`merge.go:362`), crash (`crash.go:517`), cleared (`cleared.go:157`) | "a timeout costs ♥, not the boot", "✗ lives", "lives full", "Out of lives", "lives left" | "a timeout burns 10 cycles", "out of cycles", "cycles left" |
| History filters (`screens/meta/log.go:43`, `meta.go:153`), kernel text (`flavor/kernel.go:29`) | "out of lives" | "out of cycles". The stored outcome `out-of-lives` stays, so no migration |
| Item and daemon texts (`flavor/names.go:57`, `:75`, `:82`, `:110`) | ECC "+1 life.", nice "…lives.", kexec "…costs a life", fsck "+1 life now." | "+10 cycles.", D46's byte text, "…burns 5 cycles", "+10 cycles now." |
| Help (`app/pane.go:91`) | "retreat for a life (asks)" | "retreat for 5 cycles (asks)" |
| Story (`flavor/story.go:414–423` crash.epitaph.out-of-lives*, `:501` daemon.nice "The lives fall out."; `docs/story/text.md`, 6 mentions; `story-bible.md`, 1) | lives | a storyteller pass; keys can stay |
| DESIGN.md "Vocabulary" (`lives`) and rules keys `lives`, `max_lives` | lives | the vocabulary row changes; the TOML keys can stay as names for cycles |

### The prototype (evidence, not a merge)

Flags in `internal/rules/rules.go` (all off by default, so every existing
test passes):

- `HPPerLife`: the life unit;
- `SubmitHP`, `TimeoutHP`, `GiveUpHP`, `RetreatHP`, `Biome.FailHP`: the
  costs;
- `BossFatal`, `FailAdvance`, `ClearHeal`;
- `Watchdogs` and `WatchdogBytes`: the lawnmower;
- `PenaltyTime`: ICPC's clock;
- `SubmitBytesFrac` and `SubmitBytesFloor`: the CF price;
- `Ascension.SubmitHP`, `FailHP`, `HealHP`, `Watchdogs`, `BossFatal`.

The engine side is `failCost` and `saveFail` in `internal/game/ease.go`,
plus small edits in `engine.go` and `spend.go`. The model's `GiveUpAt` is
in `internal/game/sim/sim.go`, and the report is
`internal/game/sim/meta_test.go`:

```
KATHARSIS_META=1 KATHARSIS_META_PARTS=guard,levels,prestige,learn \
KATHARSIS_META_BOOTS=300 [KATHARSIS_META_ONLY="M0,D HP"] \
go test ./internal/game/sim/ -run TestMetaReport -v -timeout 60m
```

Building D for real needs more than the flags. The HUD and strings
change (above), D34's forecast must count cycles, and the map hub of §(c)
must be built. Ascension needs the cycles fields, and the engine needs a
test of each cost path. The `failCost` and `saveFail` pair is a fair
model for the engine half.

## (j) References

Bibliographic details come from the research agents' reads of the sources and were not all checked against Crossref.

**Peer-reviewed and academic**

- Ariely, Gneezy, Loewenstein & Mazar (2009). Large stakes and big mistakes. *Review of Economic Studies* 76(2):451–469.
- Baldiga (2014). Gender differences in willingness to guess. *Management Science* 60(2):434–448.
- Beilock & Carr (2005). When high-powered people fail: working memory and "choking under pressure" in math. *Psychological Science* 16(2):101–105.
- Bjork & Bjork (2011). Making things hard on yourself, but in a good way. In *Psychology and the Real World*.
- Byron, Khazanchi & Nazarian (2010). The relationship between stressors and creativity: a meta-analysis. *Journal of Applied Psychology* 95(1):201–212.
- Coffman & Klinowski (2020). The impact of penalties for wrong answers on the gender gap in test scores. *PNAS* 117:8794.
- DeCaro, Thomas, Albert & Beilock (2011). Choking under pressure: multiple routes to skill failure. *JEP: General* 140(3):390–406.
- Ferraro & Tracy (2022). A reassessment of the potential for loss-framed incentive contracts to increase productivity. *Experimental Economics* 25:1441.
- Hase et al. (2025). Meta-analysis of challenge and threat states and performance (62 studies, N = 7,418). *EXCLI Journal*. Title not checked.
- Heitz (2014). The speed-accuracy tradeoff: history, physiology, methodology, and behavior. *Frontiers in Neuroscience* 8:150.
- Keith & Frese (2008). Effectiveness of error management training: a meta-analysis. *Journal of Applied Psychology* 93(1):59–69.
- Kuutila, Mäntylä, Farooq & Claes (2020). Time pressure in software engineering: a systematic review. *Information and Software Technology* 121.
- Wilson, Shenhav, Straccia & Cohen (2019). The eighty five percent rule for optimal learning. *Nature Communications* 10:4646.

**Industry and contest rules**

- ICPC World Finals Judging Notes, updated 2025-03-20: https://image.icpc.global/wf2026/doc/Judging-Notes.pdf ; WF 2025 scoreboard: https://worldfinals.icpc.global/scoreboard/2025/finals/
- Codeforces rules: https://codeforces.com/blog/entry/133094 , /456 , /76526 (snippets)
- AtCoder rules: https://atcoder.jp/contests/abc133/rules
- IOI 2020 rules: https://ioi2020.sg/rules
- Dead Cells: https://deadcells.wiki.gg/wiki/Health_Flask , /Boss_Stem_Cells , /Curse , /Mechanics ; Filby in https://www.vice.com/en/article/why-save-and-quit-might-not-be-an-easy-change-for-returnal/
- Plants vs. Zombies: https://plantsvszombies.wiki.gg/wiki/Lawn_Mower , /Seed_slot , /Adventure_Mode ; George Fan, GDC 2012: https://www.gamedeveloper.com/design/gdc-2012-10-tutorial-tips-from-i-plants-vs-zombies-i-creator-george-fan
- Hades: https://hades.wiki.fextralife.com/Mirror+of+Night ; Kasavin: https://inverse.com/gaming/hades-god-mode-interview , https://inlander.com/culture/hades-writer-greg-kasavin-on-how-he-made-video-game-deaths-drive-a-feel-good-story-22725237 ; safe quit: https://steamcommunity.com/app/1145360/discussions/0/2632850242355922599/ (developer reply)
- Slay the Spire: https://slaythespire.wiki.gg/wiki/Rest_Site , /Ascension ; Neowsletter 15: https://megacrit.com/news/2025-10-16-neowsletter-issue-15/
- Balatro: https://balatrowiki.org/w/Mr._Bones , /Hands , /Gameplay_loop ; LocalThunk: https://steamcommunity.com/app/2379780/discussions/0/4346606879508964186
- Into the Breach: Justin Ma, https://cliqist.com/2018/03/06/into-the-breach-building-a-better-mech-with-subset-games-justin-ma/
- Spelunky: Derek Yu quoted at https://www.robinrendle.com/notes/spelunky
- Celeste: https://www.vice.com/en_us/article/d3w887/celeste-difficulty-assist-mode
- Returnal Suspend Cycle (patch 2.0): https://mp1st.com/news/returnal-update-2-000-001-patch-notes-adds-suspend-cycle-more-this-october-26
- Monster Train 2 undo FAQ: https://steamcommunity.com/app/2742830/discussions/0/591763911730026417/
