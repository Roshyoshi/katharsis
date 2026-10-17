# katharsis pick pools: daemons, upgrades, items

**Status (10-16): built** as `rules.PerkRules` (`internal/rules/perk.go`) and
`internal/game/perks.go`, the research's defaults unless noted here. A boot
whose rules have `perk.pool = 0` (every boot saved before) keeps the first
pools and their uniform draw.

| Where the build differs from the research below | Why |
|---|---|
| smt deals the room on screen again with one more door (from a merge window: the next room) | the pick comes after the room is dealt, so "the next room" landed a room late |
| perf (cards show your median solve time) is not built | UI-only; waits on a GR3-safe median per rating |
| killall is not shipped | Q4: the twist is the boss (D54) |
| watchdog, kdump spare one-shot doors (the final, the Court) | Q3 |
| Balance, P0, 400 boots: full clears 39.5% (player 1000), 42.2% (1600), against 46.8% / 44.2% before; Boss I hazard 15.7 → 21.6% | the old 3-of-3 offers handed the model kexec, its strongest net, nearly every boot; a pool of ten draws it far less. Making Overclock and ECC common did not help (34.0% / 40.5%), so they stay uncommon. Mean door time 2h59, inside the 2h45–3h15 target |

The research as written:


Scope: base pool of 10 for each pool, apt extras on top. This version follows the
user's direction of 10-16: bytes are a byproduct, like Dead Cells' cells, so the
power the player feels is in cycles, time, ducks, picks, doors and information;
add build-defining trades with a big upside and an honest catch; keep the
training-value rule. EV is given in cycles, minutes and picks first, bytes last.

---

## 0. Headline findings (read these first)

| # | Finding | Evidence | Consequence |
|---|---|---|---|
| F1 | **The upgrade pool goes dead by pick 4.** Overclock stops after 2 picks, and ECC (as the conversion) after 1. Without coreutils, every pick after that offers only Duck. | `cycles.go` offerable: Overclock `< OverclockMax(2)`, ECC `!Contains(Upgrades, ECC)`. Sim: ~5–6 picks a boot (§1). | The upgrade pool needs repeatable, capped picks more than it needs new one-off picks. |
| F2 | **Daemons are all chosen in the first three windows.** `MaxDaemons 3` with one pick a window means daemons are picked after Userland, act I and Boss I. Windows 4–5 offer nothing. | `engine.go` enterMerge: offers only while `len(Daemons) < MaxDaemons`. | The daemon decision happens early in the boot. The window after Boss I is the natural slot for a "boss relic" (StS). Windows 4–5 could offer a priced swap. |
| F3 | **Calibration averages away perk power across boots.** Every plain door teaches the aim (`calibration.go`). Cursed, blind and regression doors do not (`doors.go` dress: `plain := !Blind && !Cursed && ...`). | `calibration.go:calibrate`, `doors.go:399`. | A perk taken in every boot is absorbed over time and stops being felt. A pool of 10 drawn 3 at a time keeps the variance between boots, which players feel, and only the pool's mean is absorbed. A perk that makes doors cursed sidesteps calibration entirely. |
| F4 | **Boots die at bosses by timeout.** At P0, 93–94% of crashes are timeouts. 95% of them happen at Boss I, Boss II or the final, which are the only tiers with hazard above 3%. A Boss I or II timeout is fatal when cycles ≤ 20. | Sim run (§1). | The levers are boss clocks, banking more than 20 cycles before a boss, and the final. A perk that touches these is strong. A perk that only touches topic doors is weak. |
| F5 | **New time sources must enter `o3Timer`'s bought set.** Today it divides out only Overclock, nice and `at`. If any new time multiplier is missed, -O3 gets easier, and that restarts the loop that ran boots to 5 h (map.md §(i)4). | `timer.go:101` o3Timer. | This is a hard requirement for quiet, cpupower, selinux, prefetch, turbo, mesg and burst. |
| F6 | **The Court's full-pool gate compares against the boot's own MaxCycles.** | `mapgraph.go:186` `g.Full && s.Cycles < s.MaxCycles`. | Any perk that lowers MaxCycles (cpupower) or heals to full (selinux) opens the Court for free. Gate on `Rules.MaxCycles`, or exempt the Court. |
| F7 | **Prestige cuts change what some catches cost.** At P2, ducks −1 brings base ducks to 0. P2 also cuts heals by 5. P4 and P7 lower the start pool. P8 makes every boss one shot. | `map.go` MapAscensions. | Catches written as "no ducks" or "bosses one shot" cost nothing at high prestige. Each needs an offerable/idle rule (§4). |

---

## 1. Baseline: the meta in numbers

Source: `KATHARSIS_BALANCE=1 KATHARSIS_BALANCE_POOL=usaco+codeforces KATHARSIS_BALANCE_BOOTS=200 go test ./internal/game/sim/ -run TestBalanceReport/runs -v`.
The run was read-only: no -update, no file writes. Full output is in `tmp/balance-runs.txt`. Seed 7, boots 51–250, union pool of 7,146 problems, `rules.Standard()` (the map).

| Measure | 1000 P0 | 1600 P0 | 1000 P8 | 1600 P8 | Source |
|---|---|---|---|---|---|
| Full clears | 48.0% | 47.5% | 8.0% | 5.5% | sim |
| Hazard: Userland / act I / Boss I / act II / Boss II / act III / final | 3.0 / 1.0 / 13.0 / 0 / 18.6 / 0 / 29.4% | 0.5 / 0 / 11.1 / 0 / 19.2 / 1.4 / 32.6% | 12 / 12 / 36 / 19 / 53 / 18 / 48% | 7 / 10 / 34 / 16 / 62 / 17 / 62% | sim |
| Crash causes (timeout / cycles / :q!) | 97 / 6 / 1 | 98 / 4 / 3 | 148 / 27 / 9 | 161 / 22 / 6 | sim |
| Door clear rate, Userland → final | 80 / 75 / 60 / 75 / 43 / 73 / 47% | 93 / 77 / 57 / 70 / 47 / 70 / 45% | | | sim |
| Mean fitted timer, Userland → final | 14 / 26 / 50 / 23 / 62 / 29 / 71 min | 11 / 35 / 56 / 35 / 61 / 27 / 67 min | 11 / 11 / 22 / 10 / 38 / 12 / 43 | | sim |
| Clears a boot (≈ picks + 1) | 5.6 | 6.1 | | | derived: Σ attempts × clear rate / 200 |
| Upgrade picks a boot | ~5–6 (every clear except the boot's last, gated scrolls and -O3) | same | | | derived |
| Door time (uptime), mean | 2h33 | 2h48 | 56 m | 53 m | sim |
| Bytes earned / committed / spent a boot | 766 / 610 / 155 | 809 / 645 / 164 | 526 / 430 / 95 | 552 / 455 / 95 | sim |
| Bytes per cleared door | ~137 | ~133 | | | derived |
| fsck bought a boot | 1.2–1.4 | | | | map.md §(i)4 |

**Exchange rates.** These are derived from the rules defaults and are inference, not measurements. They put every EV below on one scale.

| Unit | ≈ | Why |
|---|---|---|
| 1 cycle | 6.4 B | fsck: +10 for 64 B |
| 1 cycle | 2.5 min | ECC: a failed submit costs 2 cycles or 5:00 |
| 1 duck | ~7.5 cycles / 48 B | dup's price |
| 1 shuf / 1 retreat | 5 cycles / 32 B | RetreatCycles 5, ShufPrice 32 |
| 1 upgrade pick | ~5–7 cycles | the market for a pick is Duck (≈7.5) or Overclock (+15% for 5 cycles) |
| +10% on every later door | ~25 min a boot | ~4h20 of clock a full boot (timers above) |
| a boss retry | needs > 20 cycles at the timeout | FailCycles 20 and `FailCost ok = Cycles > cost` |

What a pick has to beat: Duck (+1 duck a window, ≈ 48 B ≈ 7.5 cycles in each later window) and fsck (10 cycles for 64 B).

---

## 2. Design principles from the research

| # | Principle | Precedent (source) | What it implies here |
|---|---|---|---|
| P1 | **The pool must be much larger than the offer.** 3 slots drawn from a large pool make every run's set different. | Dead Cells: 53 mutations, 3 slots, one added per cleared area ([wiki](https://deadcells.wiki.gg/wiki/Mutation)). StS: choose 1 of 3. | 10 base with 3 offered gives 120 distinct offers. Today's 3-of-3 is no draw at all. |
| P2 | **Rarity tiers, with the dramatic picks rare.** | StS relics 50/33/17% common/uncommon/rare outside chests ([wiki](https://slaythespire.wiki.gg/wiki/Relics)). Balatro rolls rarity first, then the joker ([Steam](https://steamcommunity.com/app/2379780/discussions/0/4293690852338935237/)); my recollection of 70/25/5 weights is unverified. | Weights 6/3/1 (60/30/10%). Rares are the build-definers. With 3 slots, 27% of offers show a rare and ~61% of boots see one across 3 daemon windows. |
| P3 | **Boss relics: a big upside with a drawback, offered after a boss.** Many trades share one upside and differ in what they cost. | StS's +1-energy family: Sozu (no potions), Ectoplasm (no gold), Velvet Choker (6 cards a turn), Coffee Dripper (no rest) ([wiki](https://slaythespire.wiki.gg/wiki/Velvet_Choker), [Steam](https://steamcommunity.com/app/646570/discussions/0/1640913421075645135)). Boss relics come only from bosses, as 1 of 3 ([wiki](https://slaythespire.wiki.gg/wiki/Relics)). | A **time family**: one upside (clock) bought with a different currency each. nice pays bytes, quiet pays ducks, cpupower pays cycles and boss retries, selinux pays the curse. The window after Boss I guarantees one rare. |
| P4 | **Curse first, blessing after; or a curse lifted by play.** | Dead Cells: cursed chest = +10 curse, one hit kills until 10 kills; Alienation heals on clearing a curse ([wiki](https://deadcells.wiki.gg/wiki/Curse)). Hades Chaos: a curse for ~3–5 encounters, then a permanent blessing, after paying health at the gate ([guide](https://www.exitlag.com/blog/chaos-hades-2/)). | Cursed doors are already the game's chest. The cursed trades (selinux, -Ofast, sudo) use them as a currency, and the cost is announced. |
| P5 | **A capacity cost is a balance knob.** Overcharming takes damage ×2. | Hollow Knight: 11 notches; overcharmed, all damage taken is doubled ([wiki](https://hollowknight.wiki/w/Charms)). | Daemon slots are the notches: 3 is the cap. An optional 4th slot "overcharmed" could double fail prices (§6). cpupower is the glass-cannon form. |
| P6 | **Stacking curves.** Linear is fine for flat stats; percentages need hyperbolic or a cap. Exponential runs away. | RoR2: linear 1+ax, hyperbolic 1−1/(1+ax), exponential aˣ ([wiki](https://riskofrain2.wiki.gg/wiki/Item_Stacking)). katharsis already learned this: compounding Overclock ran boots to 5 h (map.md §(i)4). | Every repeatable upgrade gets a hold cap (snapshot ≤ 3, prefetch ≤ 2, turbo ≤ 1) or a ceiling (heatsink to 30). Time multipliers from daemons share one cap (×1.75). |
| P7 | **Every pick has a place; avoid warping; nerf the boring under-costed common.** | StS GDC 2019, metrics-driven design: "every card should have a place", "avoid anything too warping", and a slightly under-costed boring common gets nerfed because it is taken regardless ([summary](https://mechanicsofmagic.com/2022/05/22/critical-play-is-this-game-balanced-10/), [GDC](https://www.gdcvault.com/browse/gdc-19/play/1025731)). | The sim checks pick rates. Commons that touch boss timeouts (watchdog, nohup) are the dominance risk, so they are priced or scoped. |
| P8 | **A skip must pay something, or the offer is forced.** | StS lets you skip a card reward, and Singing Bowl pays +2 max HP for it (inference: standard StS knowledge, not fetched). Necrodancer lets players remove unlocks from the pool ([forum](https://steamcommunity.com/app/1691190/discussions/0/4625854789815698233)). | Upgrade picks get a skip: +2 cycles, up to 30. It is never dead, and it never beats a real pick. |
| P9 | **Information perks are low-power but high-agency, and need a twist that makes them matter.** | (inference) strace already sells "a WA becomes a diff to match" (`engine.go` ShowsFailingInput). | Heisenbug (pass/fail-only samples) and UB (no sanitizers) are where info perks matter. That is their synergy cluster. |
| P10 | **Synergy clusters ("builds").** | Hades duo boons require boons from two gods ([guide](https://primagames.com/tips/hades-what-are-duo-boons-how-get-duo-boons)). Balatro's economy and scaling jokers. | Four clusters (§3.4): Clean code (picks), Time family, Enforcing (curse), Recon (info). A perk belongs to one or two of them. |

Other constraints the game already imposes:

- D50: every source of time has a price.
- D49: no heal on AC, trial by fire.
- GR1: no streaks. Retention GR1 covers streak counters, freezes and daily-login rewards; achievements.md extends it to "no bit counts consecutive anything" and bans "in a row" in copy. An in-boot combo that resets on a fail is not literally GR1, but it breaks the spirit as the project reads it, so it is out.
- GR4: bytes pay only for learning acts.
- GR5: equal expected bytes per door.
- Training value: no skipping problems wholesale, no free solves.

---

## 3. The roster

Hooks: **exists** means the function and branch are there and only need a new case. **new** means new state or a new branch. Paths are `internal/game/` unless noted. Every number is a rules setting (D2) named in `code` (suggested names).

### 3.1 Daemons (passive, 3 slots, 3 offered at each of the first windows)

| Name | Tier | Card line | Numbers (rules) | Hook | EV (cycles / min / picks; bytes last) | Role |
|---|---|---|---|---|---|---|
| nice -20 *(keep)* | C | +20% clock; every door pays 0.8x. | `NiceTimerFrac .2`, `NiceBytesFrac .2` | exists | +~50 min a boot for −~150 B | time (paid in bytes) |
| fork *(keep)* | C | 4 doors per room after an AC. | `ForkDoors 4` | exists | +1 door of choice in each of ~4 topic rooms. Idle with tmux. | doors |
| kexec *(keep)* | C | One free swap per act. | `KexecPerAct` | exists | ≤ 3 × 5 = 15 cycles. The sim's strongest net (map.md §(i)3). | safety / doors |
| **ccache** | U | A clean AC (no failed submit on that door) after Userland owes one more upgrade pick. | `CcachePicks 1`, `CcacheFrom TierActI` | engine.go `clear()` (`owe(picks)` exists; read `a.Failed`) | ~55% of 4–5 post-Userland clears (inference) = **+2–3 picks ≈ 12–18 cycles** | picks; trains ICPC penalty discipline |
| **ksmd** | C | Each AC on Boss I or II frees 10 cycles. | `KsmdCycles 10` | engine.go `clear()` (new branch on `d.Boss != ""`) | P(Boss I) 0.6 × 10 + P(Boss II) 0.45 × 10 ≈ **+10 cycles**, arriving just before the next act, which is what makes the next boss retry affordable | safety |
| **burst** (cgroup `cpu.max.burst`: unused quota carries over) | C | A clear carries 25% of its unused clock onto the next door, 5:00 at most. | `BurstFrac .25`, `BurstMax 5m`. Carried time is flat minutes added after every multiplier, as `at` is. It is spent on that door and never carries again. | engine.go `clear()`: `unused = Deadline − now`, then `s.Carry = min(BurstFrac·unused, BurstMax)`. `chooseDoor()` adds it as the `AtTime` path does. `o3Timer()` subtracts it (F5). | Act doors solved in ~60% of their clock leave 8–12 min unused, so they carry 2–3 min. A Userland door carries ~1:15. ≈ **+2–3 min on each of ~5 next doors ≈ +12 min a boot ≈ 5 cycles**. The time lands on boss doors when the topic door before them went fast. | tempo; trains speed |
| **watchdog** | U | Once an act, a door's timeout becomes 3:00 of overtime for 5 cycles. Not the final. | `WatchdogOvertime 3m`, `WatchdogCycles 5`, per act (as kexec) | engine.go `expired()`, new branch before `fail()`. The overtime machinery exists (`Attempt.Overtime`, `OvertimeScore .5`, `OvertimeBytesMult .5`). | Saves a 10–20 cycle burn, or a boss-I/II crash, about 15% of the time it fires (inference: AC within 3:00 of the deadline). ≈ **+3–5 cycles, plus a few points of boss hazard**. | safety (priced, so D50 holds) |
| **quiet** | U | No ducks this boot; clocks +20% for each duck a window would refill (at most +40%). | `QuietPerDuck .2`, `QuietMax .4`. Not offerable when the refill is 0. Duck and dup leave the offers. | engine.go `enterMerge()` refill, `timerMult()`, `o3Timer()` (F5) | At P0 (1 refill duck): **+20% clock ≈ +50 min** for ~3–5 duck uses (≈ 22–37 cycles of value, but each duck spent costs a 0.7 rating score) | time (paid in ducks); trains independent solves |
| **cpupower** | R | Every clock ×1.4; cycles cap at 20, so every boss is one shot. | `CpupowerTimer 1.4`, `CpupowerMaxCycles 20` | `State.MaxCycles` exists. Cut cycles to the cap at pick. `timerMult` and `o3Timer`. The Court gate needs the fix (F6). | +40% clock (≈ +100 min, worth ~2.7 Overclocks) against losing the Boss I/II retries (~13–19% hazard each becomes one shot: roughly +5–10 points of crash). fsck is useless above 20. | glass cannon |
| **selinux** | R | Enforcing: cycles fill to 50 now; every topic door is cursed for the rest of the boot, with a ×1.5 clock. | `SelinuxTimer 1.5`; heal to `MaxCycles` once; bosses and regressions exempt | `pickDaemon()` heal (new); doors.go `dress()` sets `Cursed` on non-boss doors; `timerMult` per door; `o3Timer` | +20–28 cycles now (≈ one boss retry banked), +50% on 1–3 topic doors. Cost: a charged WA on any of those doors ends the boot. Cursed doors pay 2x (byproduct). Cursed doors do not teach calibration (F3). | build-definer (user's own example); trains "submit when sure" |

**Apt extras** (existing ones kept; reworks are proposed, not assumed):

| Name | Tier | Card line | Numbers | Hook | EV | Package |
|---|---|---|---|---|---|---|
| strace *(keep)* | U | A failed submit shows the failing test's input. | none | exists | info. Strongest on Heisenbug and UB. | strace |
| chrt -f 99 *(keep)* | U | −20% clock; every door pays 1.5x. | exists | exists | **the one byte-payout risk/reward daemon** | util-linux |
| /dev/random *(rework)* | C | Mystery doors owe one more upgrade pick. (today: pay 2x) | `DevRandomPicks 1` | `clear()` on `d.Mystery` | ~0.3 picks a boot at mystery 8% | rng-tools |
| cron *(rework)* | C | A regression door's AC frees 5 cycles. (today: pays 2x) | `CronCycles 5` | `clear()` on `d.Regression` | ~0.5 regression doors a boot (inference) ≈ 2.5 cycles. Rewards revisiting failures. | cron |
| **kdump** | R | The first crash this boot, the final aside, reboots into a capture kernel: the panic is written (uncommitted bytes dumped, regression scheduled), and you go on at 10 cycles. | `KdumpCycles 10` | engine.go `crash()` intercept (new); not for a trial, the Court or the final | Erases one Boss I/II crash: ≈ **+10–15 points of full clears** if taken (inference from the 13/19% hazards). The cost is real: the bytes go to the dump. | kdump-tools (new, late-ladder price) |
| **tuned** | C | -O3 pays for an AC under 2/3 of the clock, not 1/2. | `TunedO3Frac .67` | `clear()` reads `r.O3Frac`; new override | +~1 pick a boot (inference). Moved out of base because it stacks with burst on the same fast clear. | tuned (new) |
| **perf** | C | Door cards show your own median solve time at each door's rating, beside its clock. | none | UI over the store's medians (GR3: every ETA is the player's own) | info for door choice; low power | linux-tools (new) |

**burst: the knobs and the two interactions to watch**

| Interaction | What happens | Rule |
|---|---|---|
| -O3 (AC under half the clock → a free pick) | A clear fast enough for -O3 also leaves the most to carry, so speed pays twice. Worse, carried time would lengthen the next door's clock, which would make the next -O3 easier: the compounding loop (map.md §(i)4). | Carried time counts as bought. `o3Timer` measures against the door's timer before the carry (as `a.At` is today). The carry does not recurse: time carried onto a door never carries again. With both rules, -O3 is unchanged and burst only adds minutes. |
| Race Condition (clock ×0.8, pays ×4) | Carry **onto** Race: flat minutes added after the ×0.8, so the race's fast clock does not shrink them. That softens the twist by up to 5:00 of its ~50 min. Carry **from** Race: measured on its shortened clock, so less is left over. | Accept both. The flat add is the honest reading: the card says "+N:NN on the next door", and the boss card shows the clock with the carry in it. If Race's clear rate rises by more than 3 points in the sim, cut the carry onto a twist boss to half. |
| Halting Problem (hidden clock) | The carry is added to a clock the player cannot see. | The HUD's strip line says "+3:12 carried" when the door opens, as `at` logs today. The clock stays hidden. |
| Upgrade instead of daemon | The same rule as a stacking pick: +15% carry a pick, two at most (30%, cap 5:00). It fits the dead-pool fix (F1). | Default is a daemon, so it is a boot-long identity. Q10 asks. |

Daemon pool after the change: 10 base + 7 apt. Byte-payout daemons: chrt only (dev-random and cron if they are not reworked).

### 3.2 Upgrades (one pick after each clear, 3 offered, ~5–6 picks a boot)

| Name | Tier | Card line | Numbers | Repeat rule | Hook | EV | Role |
|---|---|---|---|---|---|---|---|
| Overclock *(keep)* | U | +15% clock on later doors; 5 cycles a pick, 2 at most. | exists | cap 2 | exists | +~37 min a pick | time |
| ECC *(keep)* | U | A failed submit costs 5:00 of the clock, not 2 cycles. | exists | once | exists | ~4–6 WAs a boot (inference) ≈ 8–12 cycles turned into 20–30 min | safety (cycles → time) |
| Rubber Duck *(keep)* | C | +1 duck at every window. | exists | repeatable | exists | ≈ 7.5 cycles each later window | ducks |
| **heatsink** | C | +4 cycles, up to 30. | `HeatsinkCycles 4`, up to `StartCycles` | offered while cycles < StartCycles | `pickUpgrade()` + `offerable()` | 4 cycles. The ceiling keeps it from beating fsck. | safety; the pool's floor pick |
| **snapshot** | C | Your next failed submit is free; hold up to 3. Not on a cursed door. | `SnapshotHold 3` | offered while held < 3 | cycles.go `SubmitCost()` (new `State.Snapshots`) | 2 cycles each (4 on Off-by-One's double) | safety |
| **prefetch** | C | +5:00 on the next door's clock; hold up to 2. | `PrefetchTime 5m`, `PrefetchHold 2` | held < 2 | `chooseDoor()`; reuse `AtTime` (exists) with a separate counter so `o3Timer` subtracts it (F5) | 5 min ≈ `at` at 40 B ≈ 2 cycles by ECC's rate | time (one-shot) |
| **turbo** | U | +20% on the next boss door's clock. | `TurboFrac .2`, hold 1 | held < 1 | `chooseDoor()` on `d.Boss` | +10–14 min on the doors that kill boots (F4). Best before the final. | boss tempo |
| **smt** | C | The next room offers one more door. | `SmtDoors 1` | one-shot | rollRoom room size (as fork, `roomSize`) | fork for one room. Idle before a boss room (one door). | doors |
| **microcode** | U | From now on, upgrade picks offer 4. | `MicrocodeOffers 1`, once | once | `offerUpgrades()` adds to the count (exists for packages) | each later pick gets a better choice. Worth ~1 pick when taken early. | picks / quality |
| **-Ofast** (`ofast`) | R | The next door is cursed; clear it for 2 more picks. | `OfastPicks 2` | one-shot | `chooseDoor()` sets `Cursed` (new); `clear()` owes picks | +2 picks (≈ 10–14 cycles) at the risk of the boot. On a boss it is the most dramatic pick in the game. | risk/reward picks (user's "one-shot doors for picks") |
| *(skip)* | – | Skip: +2 cycles, up to 30. | `SkipCycles 2` | always | `pickUpgrade` accepts "skip" (new Action) | 2 cycles. The pick is never dead. | – |

**Apt extras:**

| Name | Tier | Card line | Hook | Note | Package |
|---|---|---|---|---|---|
| shuf *(keep)* | C | +1 shuf. | exists | | coreutils |
| **-fsanitize** | R | The next boss door's samples run with sanitizers and full output, whatever its rule. | judge sample build flags (UI lane: SampleWarnings) | Cancels the UB and Heisenbug sample twists for one door. A taste question (§6). | clang-tools (new) |

Upgrade pool: 10 base + skip + 2 apt. Every pick has a repeat rule, so the pool never collapses to Duck (F1). Byte-payout upgrades: none.

### 3.3 Items (bought at merge windows with uncommitted bytes)

Items are the byte sink, the Collector's shop. Every item turns bytes into cycles, time, ducks, picks, doors or information. None pays bytes.

| Name | Card line | Price (B) | Numbers | Hook | EV in the boot's terms | Role |
|---|---|---|---|---|---|---|
| fsck *(keep)* | +10 cycles now. | 64 | exists | exists | 10 cycles | heal (always stocked) |
| dup *(keep)* | +1 duck. | 48 | exists | exists | 1 duck | ducks |
| shuf *(keep)* | Re-roll one room's doors. | 32 | exists | exists | a retreat before reading (5 cycles) | doors |
| **nohup** | The next door's timeout or :q! costs no cycles; not a one-shot door. | 64 | `NohupPrice` | cycles.go `FailCost()`; `State.Nohup` | At a Boss I/II door: P(timeout) ~0.45 × 20 = **9 cycles, plus the retry**. At a topic door: ~3. Only worth buying before a boss, which is the intended timing decision. | boss insurance |
| **popd** | Your next retreat is free. | 32 | `PopdPrice` | cycles.go `RetreatCost()`; `State.FreeRetreats` | 5 cycles, still before the 2:30 lock | doors / safety |
| **diff** | Your next failed submit shows its failing test, as strace does. | 40 | `DiffPrice` | engine.go `ShowsFailingInput()`; `State.Diffs` | info, once. Strong on Heisenbug and UB. | information |
| **whatis** | The next room's doors show their titles and tags; blind and mystery doors pay as plain. | 24 | `WhatisPrice` | doors.go `plainDoors()` path (exists for Plain) | info for choosing a door; trades away the blind bonus | information |
| **sudo** | The next cursed door's failed submit costs 10 cycles instead of the boot. Not the Court. | 96 | `SudoPrice`, `SudoCycles 10` | engine.go `submit()` cursed branch | One curse survived. The selinux / -Ofast build buys these. | curse insurance |
| **make** | One upgrade pick, now. | 72 | `MakePrice` | `owe(1)` (exists) | 1 pick ≈ 5–7 cycles | picks |
| **mesg n** | Each duck you hold becomes +4:00 on the next door. | 16 | `MesgPerDuck 4m` | `spend()`; `AtTime` path + `o3Timer` (F5) | 1–2 ducks → +4–8 min. A one-window quiet. | ducks → time |

**Apt extras:**

| Name | Card line | Price | Note | Package |
|---|---|---|---|---|
| at *(keep)* | +5:00 on the next door. | 40 | exists | at |
| **killall** | The next boss door has no twist. | 160 | Removes the boss's identity for a door. Recommendation: do not ship (§6). | psmisc (new) |

Item pool: 10 base + 2 apt.

### 3.4 Builds (the synergy clusters)

```
Clean code (picks)   ccache + microcode + make + -Ofast (+ tuned)      → many picks, each one better
Speed (tempo)        burst + prefetch + turbo (+ tuned)                → fast topic doors bank time for the boss
Time family          nice | quiet | cpupower | selinux + prefetch/turbo  → clock, bought with a different currency each
Enforcing (curse)    selinux + quiet + sudo + -Ofast + kexec/popd       → no ducks anyway; insure the curse; swap out of bad reads
Recon (info)         strace + diff + whatis + perf (+ -fsanitize)      → Heisenbug/UB answer
Boss bank (safety)   ksmd + watchdog + nohup + turbo + heatsink + kdump → survive Boss I/II
```

---

## 4. Balance notes

### 4.1 Dominant and dead risks

| Perk | Risk | Mitigation (already in the numbers above) |
|---|---|---|
| watchdog, nohup, kdump | Each attacks the crash cause directly (F4), and the model takes nets first (map.md §(i)3: once-a-biome kexec was worth 8–14 points). | watchdog once an act and 5 cycles; nohup next door only, 64 B; kdump rare, apt, costs the bytes, never the final. Sim pick rates; nerf any above 2x the pool median. |
| ccache | +2–3 picks with no catch could be the "under-costed common" (P7). | Uncommon; Userland's easy clean ACs excluded (`CcacheFrom`). |
| quiet | At P2+ base ducks are 0 (F7), so the catch disappears. | Its clock scales with the ducks given up; not offered when the refill is 0. |
| cpupower | At P8 every boss is one shot already, so its catch is free there (F7). At P7 the start pool is 20 = its cap. | Not offerable where `OneShot` covers every boss; or at P7+ the clock drops to ×1.25. Taste question (§6). |
| heatsink | P2's `HealCycles −5` would make it 1 cycle if it follows fsck. | Exempt it from the cut (it is capped at 30 anyway), or take half. |
| Overclock | With turbo and prefetch in the pool, Overclock's 5-cycle price may become the dead pick. | Watch its pick rate. If < 10%, drop the price to 3. |
| smt / fork | Idle before a boss room and with tmux (`idle()` exists for fork). | `offerable()`: not when the next room is a boss's. |
| Daemon timer multipliers | nice ×1.2 · quiet ×1.4 · cpupower ×1.4 · selinux ×1.5 = ×3.5 on a topic door. | One cap on the product of daemon clock multipliers, `DaemonTimerMax 1.75`. Upgrades and items stay additive under their own caps. |

### 4.2 Combos

| Combo | What it does | Verdict |
|---|---|---|
| selinux + quiet | Cursed doors have no ducks, so quiet costs only its boss-door ducks: ×1.5 · ×1.2 (capped ×1.75) on topic doors. | Intended build. The cap holds it. |
| selinux + cpupower | Heal to the cap (20), every boss one shot, cursed topic doors, ×1.75. Glass cannon squared, and it opens the Court (F6). | Allowed, once F6 is fixed. |
| selinux + snapshot | Would make the curse free. | snapshot does not apply to cursed doors; sudo is the priced way. |
| kdump + kexec | Flavor pair: kdump uses kexec. | No mechanical stacking. Fine. |
| ccache + tuned + microcode + make | A pick engine. | Bounded, because every repeatable upgrade has a hold cap. Watch picks a boot: target ≤ 9. |
| -Ofast on a boss door | Cursed boss: a charged WA ends the boot. A boss timeout still has its price. | Allowed and announced. It is the "legendary" moment. |
| watchdog + Race Condition | 3:00 of overtime on a ×0.8 clock. | Fine. |
| nohup + Boss I/II | Makes the retry free once. | The intended use. |
| ECC + snapshot | snapshot is checked first (free), then ECC's clock. | Order: snapshot, then ECC, then cycles. |

### 4.2b Interactions with the main thread's reward doors (flags only; the doors are out of scope here)

The doors: a speedrun door (clear the act fast) and a no-hit door (clear the act without losing a cycle) after each boss.

| Perk | Speedrun door | No-hit door | Flag |
|---|---|---|---|
| burst, tuned, prefetch, turbo, Overclock, nice, quiet, cpupower, selinux, mesg, `at` | Bought time makes "fast" easier if fast is judged against the clock. It does not if fast is judged in absolute minutes. | | Define speed in absolute door time, or against the base clock (as `o3Timer` does), never against the bought clock. burst is the sharpest case: it is built to reward the speed the door also rewards. |
| ECC | | It turns every WA into clock, so a no-hit act becomes "no timeout and no retreat". | ECC makes no-hit nearly free for careful players. Either count an ECC conversion as a hit, or accept it as ECC's identity. |
| snapshot, sudo | | A free WA is not a lost cycle. | The same question as ECC: is a hit a cycle lost or a failed submit? Recommended: **a hit is a charged failed submit, a timeout, a :q! or a paid retreat, whatever paid for it**. That keeps the door about play, not about the perks that absorb it. |
| kexec, popd | | A free swap is not a lost cycle. | Under the definition above, a free swap is not a hit, because swapping a door before the 2:30 lock is a choice, not a failure. |
| watchdog, nohup | | They pay for a timeout (5 cycles, or nothing). | Under the definition above it is still a hit. |
| Overclock (5 cycles a pick), cpupower (cut to 20), watchdog (5) | | These are chosen spending, not damage. | A no-hit door must count only failures, never prices paid at a pick. Otherwise Overclock becomes dead in no-hit runs. |
| heatsink, ksmd, cron (rework), selinux heal | | Heals do not undo a hit. | Count hits, not net cycles. |
| -Ofast, selinux | A cursed door is a no-hit door by construction: any hit ends the boot. | | Synergy, fine. |

### 4.3 Boss-twist interactions

| Twist | Perks it amplifies | Perks it blunts | Rule |
|---|---|---|---|
| Off-by-One (one-test WA ×2) | snapshot (saves 4), ECC (10:00) | | snapshot consumes 1 for the doubled cost |
| OOM Killer (memory ×0.5) | diff/strace (MLE input) | | none |
| Heisenbug (pass/fail-only samples) | diff, strace, -fsanitize | | The Recon build's target |
| Race Condition (clock ×0.8, pays ×4) | turbo, prefetch, watchdog, burst (flat carry, up to 5:00, not shrunk by ×0.8) | cpupower helps the clock, but the boss becomes one shot | none |
| Undefined Behavior (no sanitizers on samples) | diff, strace, -fsanitize | | -fsanitize cancels it, so it is a taste question |
| Halting Problem (hidden clock, one shot) | turbo (it still adds time) | watchdog (excluded), nohup (n/a: no cycles price), kdump (excluded) | the one-shot final stays one shot |
| The Court (cursed, one shot, needs a full pool) | | selinux and cpupower open the gate (F6); sudo | Court exempt from sudo, snapshot, kdump, watchdog; gate on `Rules.MaxCycles` |
| ECC short-clock fallback | | | snapshot is checked before ECC |

### 4.4 Training value

| Perk | Verdict |
|---|---|
| ccache, selinux, -Ofast, sudo | Positive: test locally, submit when sure. This is ICPC penalty discipline. |
| tuned | Positive: rewards speed. |
| quiet, mesg | Positive: fewer hints and more independent solves (score 1.0 vs 0.7). |
| watchdog | Neutral: an overtime AC scores 0.5 and pays 0.5. No free solve. |
| kdump | Neutral: the failure is still recorded (dump, regression, rating). |
| diff, strace | Mildly negative: a WA becomes a diff to match. Kept to one shot or a daemon slot. |
| killall, -fsanitize | Negative or neutral: they remove a boss's lesson. This is why they are apt-locked and flagged. |

No perk skips a problem wholesale or grants an AC. Swaps stay behind the 2:30 retreat lock.

### 4.5 What to verify in the sim after implementing

1. **The sim player needs policies.** `sim.go:940` daemon priority, `upgrade()` at 909 and `buy()` at 843 know only today's perks. Add priorities, and a per-perk forced-pick run (each perk forced in every boot) to read its marginal effect.
2. **Per-perk marginal (forced-pick).** Full clears and hazard I/II/III at P0, P4 and P8, for 1000/1300/1600. Target: each perk within ±6 points of the pool mean. No perk above +10.
3. **The band holds with the new pool on random picks.** P0 full clears 40–55%, hazards ~15/25/35%, P4 ≤ 30%, P8 ~10% (D49).
4. **Boot length.** The mean door time stays in 2h45–3h15 (map.md §(a)), which catches F5 and the multiplier cap.
5. **Picks a boot** ≤ 9 under the Clean-code build.
5b. **burst:** the -O3 rate with and without it must be equal (it is excluded from `o3Timer`). Race Condition's clear rate may rise by at most 3 points. Mean carry is ~2–3 min a door.
6. **The Court's open rate** with selinux or cpupower, after F6.
7. **The sim cannot see some effects.** Information perks, careful submitting under a curse, and duck value are not modelled; the WA model is `sim.go fail()`. selinux and quiet need real boots. Flag them as alpha-feedback perks.

---

## 5. Shop and offer rules

| Rule | Recommendation | Why |
|---|---|---|
| Daemon offers | 3 drawn by rarity weight 6/3/1 from the unlocked, not-running, not-idle pool | P1, P2 |
| Boss relic slot | The window after Boss I always has one rare among its 3 (if the player still has a free slot) | P3, F2 |
| Full slots | Windows after act II and Boss II offer a **swap**: replace one running daemon for 64 B. Its cost could double with each swap, as Dead Cells' mutation reset does. | F2. Dead Cells' reset is 1000 gold, doubling to 8000 ([wiki](https://deadcells.wiki.gg/wiki/Mutation)). |
| Upgrade offers | 3 by weight 6/3/1 (4 with microcode or aptitude), plus a skip (+2 cycles, to 30) | P8, F1 |
| Upgrade offerable rules | Hold caps and ceilings as in §3.2; smt not before a boss room | P6 |
| Shop stock | **5 a window: fsck always, plus 4 drawn uniformly** from the other unlocked items, with no repeats within a window; restocked each window | fsck is the real heal (no fsck: −2 to −9 points, map.md §(i)); each other item shows ~2.2 times a boot (6 windows × 4/9) |
| Before a boss | The window before Boss I/II stocks nohup or turbo-adjacent items with weight ×2 | the timing decision (F4) |
| Prices | Flat, set in rules; no inflation within a boot | bytes are a byproduct; keep the arithmetic readable |
| Logging | Every offer (daemon, upgrade, shop) goes into the ML1 offer log with its propensity, as doors and routes do | offers.go: the policy log is a standing contract |
| Trials | Unchanged: no daemons, no shop | engine.go enterMerge, spend.go Price |

---

## 6. Open questions for the user (taste)

| # | Question | Recommended default |
|---|---|---|
| Q1 | **selinux scope:** every topic door for the rest of the boot (heal once), or a Chaos shape (the next 3 doors cursed, then a full heal)? | Rest of the boot, heal once. It is your example, and how early you pick it sets the risk (1–3 cursed doors). Sim-check against Q1's alternative. |
| Q2 | **Rework dev-random and cron away from bytes** (to a pick and +5 cycles)? | Yes. It leaves chrt as the one byte-payout daemon, per "bytes are a byproduct". |
| Q3 | **watchdog on the final?** | No: the final stays one shot (the act-III lever, map.md §(i)2). |
| Q4 | **killall and -fsanitize** remove or soften a boss twist. Ship them? | Ship -fsanitize as a rare apt upgrade, since sanitizers are good practice. Do not ship killall: the twist is the boss (D54). |
| Q5 | **cpupower at P7–P8**, where its catch is already in force? | Not offered at P8; at P7 its clock drops to ×1.25. |
| Q6 | **A 4th daemon slot "overcharmed"** (every failure price ×2), Hollow Knight style? | Not now. Revisit after the 10-pool ships and pick rates are in. |
| Q7 | **Daemon swaps** in windows 4–5 for 64 B? | Yes. Otherwise half the windows have no daemon decision. |
| Q8 | **Rarity visible on the card** (a color or a glyph)? | Yes, as a dim tag. It is honest about what's rare (GR3 spirit). |
| Q10 | **burst as a daemon (boot identity) or a stacking upgrade** (+15% carry a pick, two at most)? | A daemon, at 25% and 5:00. The upgrade form is the fallback if the upgrade pool still runs thin in the sim. |
| Q11 | **What counts as a "hit"** for the no-hit reward door (main thread)? | A charged failed submit, a timeout, a :q! or a paid retreat, whatever paid for it. Prices paid at a pick are not hits (§4.2b). |
| Q9 | **Name collision:** `watchdog` is the judge's memory/CPU killer in comments (`internal/judge/run.go:124`). | Keep it; the player never sees the judge's. The alternative is `softlockup`. |
