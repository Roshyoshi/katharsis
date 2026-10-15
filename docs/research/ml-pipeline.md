# ML pipeline for katharsis: which doors, which reviews, which tags

ML thread, 2026-10-14. Builds on `docs/research/learning-science.md` (the neuro
brief, cited as "neuro §n"). Scope: `internal/game` (rating, calibration, doors,
timer, meta, engine, trial), `internal/game/sim`, `internal/rules`,
`internal/store`, `internal/catalog`, `internal/content`. No code was changed.
Line numbers are as of this date; other agents are editing these files.

**Status.** Proposed design. Three forks were settled by the user on
2026-10-14 and are written in as decisions (§6.1):

- **(a)** Population priors may be fitted offline from the public Codeforces
  API. This is read-only data and is separate from platform integration (D7).
- **(b)** Prestige holds the mid biome's clear chance at 0.6 or above and gets
  harder through risk instead (fewer lives and ducks, more cursed and blind
  doors, tighter timers).
- **(c)** Tags are hidden by mastery on known topics. Topics still being
  learned keep their tags.

**Provenance labels.** `[code]` is read from the source at the cited line.
`[ran]` is computed here: the balance report
(`KATHARSIS_BALANCE=1 go test ./internal/game/sim -run TestBalanceReport -v`)
or a scratch Go calculation. `[cited]` comes from a primary source in §7.
`[assumed]` is a default chosen here. Every `[assumed]` number is a tunable
setting, locked per boot (D2).

## 0. Summary

**One measurement model replaces Elo plus the calibration offsets.** It is a
logistic IRT model with item noise per source and covariates for door
modifiers. It sits on a hierarchical, Glicko-style skill filter whose
uncertainty grows per day away. The engine's "internal learning rate" becomes
a fitted parameter (process noise), not the hand-set K = 40. The player's own
learning rate is one global number, shown as a trend with a band. Per-topic
learning rates cannot be identified for one player (neuro §2).

**Doors are chosen in probability space.** Each room offers three slots: easy,
target and stretch. Each slot is a band of predicted clear chance, and its
pick is uniform, so selection probabilities are exact and get logged. Floors
enforce decision (b). Mastery sets whether tags show, per decision (c).

**The policy is not learned online.** It is a fixed rule with a few dozen
tunables. They are chosen in a simulator with a learning player, under three
hypotheses about how difficulty drives learning. Real data refines only the
measurement model and checks the guardrails. One player cannot measure
learning differences between policies in useful time (§1.2).

```
offline, between boots (app; deterministic)
  store ─replay─► fit φ by prequential MAP ─► model_versions        M3
  cmd/kprior (developer, once) ─► prior_cf.json                     §2.9
NewBoot
  Rules.Model ← accepted φ (locked for the boot, D2)
  governor and Userland buy-back check                              M4
  layout ─► route offers: weighted draw, logged propensity          M5
rollRoom
  Skill propagated to now (variance grows with days away)           M2
  p̂ = predictive clear chance for every eligible problem            M1
  slots easy/target/stretch on p̂_base; floor on p̂_eff               M4
  tags hidden by mastery; interleave slot; due review by its p̂      M4, M6
  State.Offers += {items, p̂, propensity}                            §4
close (a door ends)
  Skill ← filter update (graded outcome, covariates)                M2
  review items created or advanced (FSRS-shaped)                    M6
```

**What the current engine gets wrong, measured `[ran]`** (balance report,
standard rules, model player of `sim.go:28-59`, union pool of 7,462 problems,
400 boots after 50 warm-up):

| Player, prestige | Userland crash | Target | Engine rating | Calibration "all" |
|---|---|---|---|---|
| 1000, P0 | 56.5% | 15% | 828 | +83 |
| 1600, P0 | 28.8% | 15% | 1594 | +66 |
| 1600, climbing to P4 | 43.2% | 15% | 1367 | +125 |

1. **Userland crashes 2–4× more often than designed.** For low ratings the
   cause is the content floor: CF problems start at 800, and 98% of the
   1000-player's Userland doors were off target. For a 1600 player the
   cause is risk modifiers. Plain Userland doors cleared 92.2% (on target),
   while all Userland doors cleared 84.2%. The model player picks by
   bytes × P, so it takes blind and cursed doors (`sim.go:384`).
2. **Elo is biased by stakes.** It rates a cursed or blind door, or one
   played on the last life under a shorter timer, as a plain door
   (`engine.go:375`). At P4 the estimate sank 233 points below the truth. The
   calibration offset then absorbs the bias (+125), together with the
   curve's shape and the timers, in one number whose parts cannot be told
   apart. The sim player is a stand-in (`sim.go:1-9`), so the size is
   indicative; the mechanism is in the code.
3. **Door choice is not logged as data.** Offered sets survive only as dmesg
   text with ratings (`doors.go:199`): no problem IDs, candidate counts or
   chosen index. Boots played before the logging slice (§4) cannot be used
   for off-policy evaluation.

**Build first:** logging (§4), then the learning simulator (§5.2), then the
model in shadow mode (§3, slice ML4). **Do not build** (§2.11): bandits or RL
rewarded by learning; the ZPDES route bandit the neuro brief proposed;
Thompson sampling inside bands; DKT or BKT; per-topic learning rates;
per-problem difficulty learning; Glicko-2 volatility; an engagement-optimizing
objective.

## 1. Problem statement

### 1.1 The pipeline owns door difficulty, door topic, routes, tags and reviews

| Decision | Today `[code]` | Pipeline |
|---|---|---|
| Door difficulty | target rating = `Aim(PlayerRating, TargetClear) + 25·prestige + calibration` (`doors.go:24-28`); uniform pick inside ±100/250/500 windows (`doors.go:101-140`) | three slots per room, each a band of predicted clear chance (M4) |
| Door topic | the biome's topic first, then any topic (`doors.go:90-95`) | same, plus an interleave slot from another known topic (M4) |
| Route topics | uniform over unlocked topics with pool, fresh first (`doors.go:295-333`) | weighted by staleness, due reviews and uncertainty; propensity logged (M5) |
| Tags shown | flat `BlindChance` 0.25 + ascension (`doors.go:248-251`, `rules.go:319`) | mastery rule, decision (c) (M4) |
| Reviews | a crash schedules +3 d, then +17 d, then retire (`engine.go:473`, `meta.go:59-95`, `rules.go:329`) | armed by an upsolve; also hinted, overtime and WA-heavy clears; FSRS-shaped gaps; sibling problems (M6) |
| Skill estimate | Elo K 40 (K 100 for 20 doors), topics follow the overall rating, hard switch at 5 topic doors (`rating.go:29-94`) | hierarchical filter with time-grown variance (M2) |
| Hint credit | hinted clear scores 0.7, overtime 0.5 (`engine.go:363-372`) | graded outcome with a fitted cut point (M1) |
| Prestige | +25 rating per level plus ascensions (`rules.go:377-383`) | ascension risk only; floors on clear chance, decision (b) (M4) |
| Guardrails | none | streak governor, Userland buy-back, floors, monitors (M4, §1.3) |
| Timers | `fitTimer` (`timer.go:45-86`) | keep; a censored lognormal model later, if it earns its place (M7) |

Not owned here: capability gating (`capability.go:10`), trial content, the
byte economy, and duck-ladder content (neuro H2).

**Hints need no model-driven policy.** The duck stays the player's decision.
The pipeline handles hints in three places. The measurement model credits a
hinted clear by a fitted amount (M1). The mastery rule decides whether level
1 (tags) has anything to reveal (M4). And quack times are logged (§4). Do not
build a proactive hint model (Maniktala 2020): it needs a population, and
forced hint delays did not help (Vanacore 2024, neuro §5).

### 1.2 The objective: guardrails first, then learning that the simulator ranks

The objective is lexicographic, as in neuro (c).

1. **Hold every guardrail** (table below). These are measurable on real data
   within weeks.
2. **Within them, maximize durable skill** on fresh doors: first attempts,
   not regressions, scored strictly (a hinted clear counts as a miss).

**The learning term cannot be measured on one player per policy.** Over 30
days (about 12 boots, 45 doors) the filtered general level has a steady SD of
about 60 points `[ran]`. Two estimates 45 doors apart correlate about 0.38,
so a 30-day change has an SD of about 67 (inference). Detecting a
policy's effect needs a difference several times larger than any plausible
effect. So policies are ranked in the simulator (§5.2). On real data the
learning trend is reported per quarter as a sanity check, and nothing is
tuned on it.

| # | Guardrail | Threshold | Design value |
|---|---|---|---|
| G1 | P(crash before the first merge window) | ≤ 0.20 | 0.154 |
| G2 | early-crash streak before the governor eases | ≤ 2 boots | |
| G3 | door success over a boot | 0.75–0.85 | 0.79 |
| G4 | P(full clear) at the top unlocked prestige | 0.15–0.30 | 0.21 |
| G5 | effective mid-biome clear chance, modifiers included | ≥ 0.60 at every prestige, decision (b) | 0.72 at P0 |
| G6 | calibration: \|z\| of realized vs predicted clears per biome over 100 doors | ≤ 3 | 0 |

### 1.3 Every quantity has a definition and a query

Columns marked *new* arrive with migration 2 (§4.2). Until then, `p` and
`biome` can be read from `attempts.json` and `boots.state`.

| Quantity | Definition | Source |
|---|---|---|
| G1 early-crash rate | of the last 30 finished, non-trial boots: crashed with `State.Biome = 0` | `boots` (today) |
| G2 streak | count of the most recent consecutive boots in G1's numerator | `boots` (today) |
| G3 door success | cleared ÷ (opened − retreated) over the last 20 boots | `attempts` (today) |
| G4 full clears | `outcome = 'cleared'` share of the last 30 boots at `prestige = Meta.Prestige` | `boots` (today) |
| G5 mid clear | ex ante: every opened mid door has `p ≥ 0.60` (asserted); ex post: realized vs mean `p` over the last 50 mid doors | `attempts.p`, `attempts.biome` (*new*) |
| G6 calibration z | `Σ(y − p) / √Σ p(1−p)` per biome over the last 100 doors | `attempts.p` (*new*) |
| Learning trend | `μ_g` of the audit replay (§5.1) now minus 90 days ago, with its SD | `attempts`, `Meta.Solves` (today) |
| Hours per week | Σ `boots.uptime` + upsolve time per ISO week | `boots` (today), `events` (*new*) |
| Return latency | `started` of boot n+1 minus `ended` of boot n | `boots` (today) |
| Off-band rate | share of opened doors whose slot fell back past its band | `attempts.slot` (*new*) |
| Abandon rate | `outcome = 'abandoned'` share; reported, not a guardrail | `boots` (today) |

```sql
-- G1, today's schema. Trials have State.Trial set; abandoned boots excluded.
SELECT avg(outcome = 'crashed' AND json_extract(state, '$.Biome') = 0)
FROM (SELECT outcome, state FROM boots
      WHERE outcome IN ('crashed', 'cleared')
        AND json_extract(state, '$.Trial') IS NULL
      ORDER BY id DESC LIMIT 30);

-- G3, today's schema.
SELECT sum(a.outcome = 'cleared') * 1.0 / sum(a.outcome != 'retreated')
FROM attempts a
JOIN (SELECT id FROM boots
      WHERE outcome IN ('crashed', 'cleared')
        AND json_extract(state, '$.Trial') IS NULL
      ORDER BY id DESC LIMIT 20) b ON a.boot = b.id;

-- G6 for the mid biome, after migration 2.
SELECT sum((outcome = 'cleared') - p) / sqrt(sum(p * (1 - p)))
FROM (SELECT outcome, p FROM attempts
      WHERE biome = 1 AND p IS NOT NULL AND outcome != 'retreated'
      ORDER BY closed DESC LIMIT 100);
```

The `top` screen and the crash screen should show G1, G3 and G6 next to the
rating, so the player (who is also the designer) sees when the aim is off.

## 2. Models

### 2.0 Units and notation

- `s = 400 / ln 10 = 173.7` CF points per log-odds unit. `σ(x) = 1/(1+e^−x)`.
- `κ(v) = (1 + πv/8)^(−1/2)` flattens a logistic for a latent with variance
  `v` (in log-odds²): `E[σ(z)] ≈ σ(κ(v)·E[z])` (MacKay 1992). Glicko's
  `g(RD)` is the same function (Glickman 1999).
- Topics `k` are `catalog.Topics` without `mixed`: K = 15. Index 0 is the
  general level `θ_g`, with mean `μ_g` and SD `σ_g`. `δ_k` is topic k's
  deviation from it, with mean `μ_k` and SD `σ_k = √Σ_kk`.
- Door `j` has rating `r_j`, topic `k(j)`, source class `c(j)` and covariate
  vector `x_j`.

### M1. Measurement model: graded logistic IRT with item noise

**Chosen because** the logistic on the CF scale is what the content ratings
mean (Mirzayanov 2018), and simple Bayesian IRT matches deep models on tutor
data (Wilson et al. 2016; Khajah et al. 2016).

```
latent    z_j = (θ_g + δ_k(j) − r_j − o_c(j) − ε_j) / s + βᵀx_j
item      ε_j ~ N(0, σ_c(j)²)            noise of the rating source
grades    y = 0 fail · 1 assisted clear · 2 clean clear
          P(y ≥ 1) = σ(z)                 any clear
          P(y = 2) = σ(z − c)             clean clear, c > 0
```

`y = 1` is a clear with a quack or in overtime. `y = 0` is a timeout, out of
lives, `:q!`, or a cursed WA. A retreat is not rated, as today
(`engine.go:374`), but it is logged and checked for bias (§5.5, F7). This is
Samejima's (1969) graded response model with two categories above failure.
The fitted cut point `c` replaces `HintedScore` and `OvertimeScore`.

**Identification.** The discrimination is fixed at 1. A CF rating is defined
as the rating that solves the problem half the time, on the 400-point base
(Mirzayanov 2018). Flattening is attributed to item noise per source, which
is where the evidence puts it (neuro §6, pitfall 4). The CF offset is fixed:
`o_cf = 0` anchors the scale.

**Source classes** (from `Problem.RatingSource`, `content/usaco/rating.go:77-101`
and `content/codeforces/index.go:206`). Priors are `[assumed]`:

| Class | RatingSource values | Offset prior | Noise σ prior |
|---|---|---|---|
| `cf` | `cf` (6,761 judgeable) | 0, fixed | lognormal(ln 120, 0.4) |
| `usaco-base` | `division`, `division+position` (282) | N(0, 100²) | lognormal(ln 220, 0.4) |
| `usaco-refined` | `+guide-label` and/or `+solve-rate` (419) | N(0, 100²) | lognormal(ln 180, 0.4) |

**Covariates.** All are known when the door opens. Effects are in log-odds,
with Gaussian priors `[assumed]`.

| x | Value | Prior mean, SD | Basis for the prior |
|---|---|---|---|
| blind | tags hidden | −0.3, 0.3 | sim `BlindCost` 25 pts = −0.14 |
| mystery | title hidden too (on top of blind) | −0.15, 0.2 | |
| cursed | one WA crashes | −0.6, 0.4 | sim `WAProb` 0.2: ×0.8 at p 0.72 |
| timer | `ln(T_door / T_fit(gap))`; `T_fit` before all multipliers | +1.0, 0.5 | sim: ≈ +1.4 at p 0.72 |
| lives1 | one life left at open, or two under `nice` (a WA costs 2) | −0.3, 0.3 | |
| trial | trial door: module just read, soft timer, failing input shown | +0.5, 0.5 | |
| *duck* | a duck available, door not cursed | +0.2, 0.2 | extension |
| *seen* | problem attempted or its editorial read before | +0.5, 0.4 | extension; neuro §6 pitfall 9 |
| *strace* | failing input visible | +0.15, 0.2 | extension |
| *boss* | named-bug door | 0, 0.3 | extension |
| cut point c | clean vs assisted | ln c ~ N(0, 0.5²), so c ≈ 1 | |

The six core covariates ship first. An extension (italic) is added only when
the replay shows a better prequential log loss (§5.1), and otherwise stays at
0. An upper asymptote (a lapse rate) is the next extension to test, on
Userland doors.

**Predictive clear chance** (used for selection) integrates over both item
noise and the posterior of the player:

```
m = (μ_g + μ_k − r_j − o_c) / s + βᵀx_j
V = (Σ_gg + 2Σ_gk + Σ_kk + σ_c²) / s²
p̂_any   = σ(κ(V)·m)          p̂_clean = σ(κ(V)·(m − c))
```

Two versions are computed for each candidate:

- **p̂_base** uses reference stakes: no risk modifier (not cursed, no
  mystery, no prestige blindness), the preset's base lives and ducks, and a
  timer multiplier of 1. It includes blindness only when the mastery rule hid
  the tags. The aim uses it.
- **p̂_eff** uses the door's real stakes: lives and ducks at open, every timer
  multiplier, and every modifier. The floors and the logs use it.

So an Overclock the player bought raises p̂_eff and is never "compensated" by
a harder door. Prestige risk lowers p̂_eff and is compensated only below the
floor.

**Aim in closed form.** For a band edge `p`:
`r = μ_g + μ_k − o_c − s·(logit p / κ(V) − βᵀx_base)`. `V` depends only on
(topic, source class), not on the problem. So every slot is a rating interval
per (topic, source), cheap to compute and to explain ("dp doors 1590–1660").
With `Σ = 0`, `σ_c = 0` and `β = 0` this is today's `Aim` (`rating.go:21`),
which gives a regression test.

**Cost:** about 20 flops per candidate. A full pool scan (7.5k problems) per
room takes under 0.2 ms.

### M2. Skill filter: hierarchical Glicko with variance that grows per day

**State.** `x = (θ_g, δ_1 … δ_15)`, Gaussian with mean `μ` (16 floats) and
covariance `Σ` (16×16). This is the general-plus-specific structure that
`TopicFollow = 1` approximates today (neuro §2). It replaces the hard switch
at `TopicRatingMin`: a topic with few doors stays near the general level, by
shrinkage. `mixed` problems inform `θ_g` only.

**Prior** `[assumed]` until §2.9 lands:
- `μ_g = 1000`, `σ_g = 350` (Glicko's unrated RD).
- `δ_k ~ N(0, τ²·R)` with `τ = 150`.
- `R` has correlation 0.4 within the clusters below and 0 across them:

```
{prefix-sums, two-pointers, binary-search, sorting-greedy}
{graphs, trees, shortest-paths, dsu}
{complete-search, simulation}
{math, geometry}
{dp} {strings} {data-structures}
```

**Between events** (`now` comes from the engine; never `time.Now`):

```
days = (now − Skill.At) / 24h
Σ_gg += q_g·days              Σ_kk += q_δ·days    (every topic)
per rated door, before its update:  Σ_gg += q_gd    Σ_kk += q_δd
cap: if Σ_ii > cap_i², scale row and column i by cap_i/√Σ_ii    (keeps Σ PSD)
optional drift per rated door: μ_g += γ_g, μ_k += γ_δ           (default 0)
```

Defaults `[assumed]`, refit by M3: `q_g = 100` pts²/day (±100 over 100 days),
`q_δ = 50`, `q_gd = 50` pts²/door, `q_δd = 25`, `cap_g = 350`, `cap_δ = 225`.

**Update for one closed door** on topic `k`, with `h = e_0 + e_k` (`mixed`:
`h = e_0`). This is one Newton step at the prior mean. In one dimension, with
no process noise and one door per rating period, it is exactly Glicko-1
(Glickman 1999). Szczecinski & Tihon (2023) derive the same family as a
simplified Kalman filter.

```go
kj := kappa(sigmaC*sigmaC / (S*S))       // item noise only (Glicko's g)
eta := (dot(h, mu) - r - oC)/S + dot(beta, x)
a, b := sigmoid(kj*eta), sigmoid(kj*(eta-c))
var g, H float64                          // d/dη and −d²/dη² of log P(y)
switch y {
case Fail:     g, H = -a, a*(1-a)
case Assisted: g, H = 1-a-b, a*(1-a)+b*(1-b)
case Clean:    g, H = 1-b, b*(1-b)
}
g, H = kj*g, kj*kj*H
v := mulVec(Sigma, h)                     // Σh: column g plus column k
w := dot(h, v)                            // hᵀΣh
den := 1 + H*w/(S*S)
mu  = add(mu, scale(v, g/S/den))
Sigma = sub(Sigma, scale(outer(v, v), H/(S*S)/den)); symmetrize(Sigma)
```

**What "learning rate" means here, in numbers** `[ran]`, with p(1−p) = 0.18
per door:

| Filter | Process noise per door | Steady SD | Time constant | Elo-equivalent K |
|---|---|---|---|---|
| Elo K 40 (today) | 300 pts² (17 pts) | 83 | 24 doors | 40 |
| q = 150 | 150 | 70 | 34 | 28 |
| q = 50 | 50 | 54 | 58 | 17 |
| q = 25 | 25 | 45 | 82 | 12 |

Read as a Kalman filter, Elo K 40 assumes the player's skill jumps about ±17
points per door, far above the ~1–5 points a door plausibly teaches (neuro
§2). That buys fast tracking with noisy aim. M3 fits the trade-off instead of
hand-setting it. The day term makes time away widen uncertainty, which Elo
cannot do.

**Breaks are handled by the predictive κ, not a special rule.** For targets
above 0.5, a wider posterior moves the aim down by itself `[ran]`:

| Posterior + item SD | κ | Aim below μ for 0.92 | for 0.72 | for 0.50 |
|---|---|---|---|---|
| 0 | 1.00 | 424 | 164 | 0 |
| 100 | 0.94 | 451 | 174 | 0 |
| 150 | 0.88 | 482 | 187 | 0 |
| 250 | 0.74 | 571 | 221 | 0 |

This replaces neuro R9's "aim at θ − 1 SD", which moves the aim 2–7× further
than the predictive does. An optional mean decay,
`μ ← μ − λ·ln(1 + days/30)` for gaps over 7 days, has prior `λ = 0`. It is
fitted only if the residuals of the first doors after breaks demand it
(§5.5, F4).

**Hierarchy payoff, per topic** `[ran]`: posterior SD of `δ_k` after `n`
topic doors, with `τ = 150` vs a flat prior:

```
n    5    10    20    40    80
    116    98    78    59    44      hierarchical
    183   129    92    65    46      flat (neuro §2)
```

The gain is real but modest, and it is largest early, where it matters. If
the replay shows `δ ≡ 0` predicts as well (F3), drop topic ratings for aiming.

**Display.** `Meta.Rating := μ_g` and `Meta.TopicRating[k] := μ_g + μ_k`, so
`top`, `rating_history` and `PlayerRating` (`rating.go:29`) keep working. Add
the ±1 SD band from `skill_history` (§4). The player sees their learning as a
trend with a band (neuro §2), not as a parameter.

**Migration of existing saves.** Replay every stored attempt from the prior
with M2. The store keeps them all, so replay is exact. Do not convert
`Meta.Rating`.

**Cost.** A rank-1 update is about 300 flops. State: 272 float64 (2.2 KB), or
about 5 KB of JSON in `Meta`.

### M3. Hyperparameters are fitted by prequential MAP replay, locked per boot

`φ` holds β (6–10 values), source offsets and noises (5), `c`, the four `q`s,
`τ`, `γ_g` and `γ_δ`: about 22 parameters.

```
J(φ) = − Σ_t log P(y_t | y_<t, φ)  − log prior(φ)
       └── the filter replayed over every rated attempt in `closed` order
```

The prequential log likelihood is the filter's own one-step-ahead predictive
score (Dawid 1984), so the fit and the evaluation use the same quantity.

- **Optimizer.** Nelder–Mead on unconstrained transforms (log for SDs and
  `q`s), from a fixed initial simplex, 4,000 evaluations at most. Use
  `gonum/optimize` if a new dependency is acceptable. Otherwise write about
  100 lines, which avoids a flake `vendorHash` refresh.
- **When.** Never before 150 rated doors (about 40 boots); until then priors
  only. Afterwards, at a boot's end once 20 boots have passed since the last
  fit, and on demand with `katharsis model fit`.
- **Acceptance.** Fit on the first 80% of doors in time order. Accept the new
  φ only if its prequential log loss on the last 20% beats the current φ's,
  and no parameter sits at a bound. Record the result in `model_versions`
  (§4.2) either way.
- **Locking (D2).** NewBoot copies the accepted φ into `State.Rules.Model`.
  A preset can pin it (`[model] pin = true`) or override any value.
- **Determinism.** The fit is a pure function of (attempts, prior, settings).
  Sort inputs by (closed, boot, idx). Never iterate a map unsorted.
- **Cost.** One replay of 1,000 doors is about 0.3 ms, so a fit takes 1–2 s.

### M4. Door choice: three slots per room, banded on predicted clear chance

**Bands** (`[assumed]`, tunable; centers are on p̂_base, floors on p̂_eff):

| Biome | Easy | Target | Stretch | Half-width | Floor (p̂_eff) |
|---|---|---|---|---|---|
| Userland | 0.95 | 0.92 | 0.89 | 0.02 | 0.85 |
| topic biome (mid) | 0.82 | 0.72 | 0.63 | 0.05 | 0.60 |
| boss (one door) | | 0.50 | | 0.04 | 0.40 |

With the governor on, every center moves up: Userland +0.03 (capped at
0.97), mid +0.06, boss +0.05. That is neuro P1.3's 0.95 / 0.78 / 0.55.

- **Userland's easy slot rolls no risk modifiers**, so a safe door is always
  on offer and risk stays the player's choice. In the balance report, plain
  Userland doors cleared on target and the risky ones caused the excess
  crashes (§0).
- The spread follows neuro P1.1. Chosen moderate difficulty motivates most
  (Lomas 2017), and the pick shows how the player judges their own level
  (Metcalfe 2002).
- The mid center 0.72 sits in neuro's 0.70–0.78. It is dithered per room by
  a logged U(−0.03, +0.03): the variation of neuro P1.4, with exact
  propensities (§5.3).
- **Each band must span at least 100 rating points.** CF ratings come in
  100-point steps (`[ran]`: histogram of the 6,761 judgeable), and a ±0.05 band
  at p 0.72 spans only about ±49 points. The selector widens a band to 100
  points before searching.

**Prestige adds risk and no rating offset (decision b).** `PrestigeOffset`
goes to 0. Bands and floors are the same at every prestige. A higher
prestige plays with the ascensions (`rules.go:378-383`: blind, mystery,
cursed, one duck less, timer ×0.9, one life less). They lower p̂_eff and are
compensated only where they would cross the floor. The floor also covers
fewer lives and tighter timers, because both are covariates of p̂_eff.

Implied bounds `[ran]` (2 Userland rooms, 2 mid, 1 boss; door success counts
doors until the first failure):

| Scenario (U / mid / boss per door) | P(full clear) | P(crash in Userland) | Door success |
|---|---|---|---|
| today's design aim, 0.92 / 0.70 / 0.50 | 0.207 | 0.154 | 0.790 |
| today at P4 before modifiers, 0.92 / 0.57 / 0.36 (neuro §1) | 0.099 | 0.154 | 0.744 |
| proposed P0, 0.92 / 0.72 / 0.50 | 0.219 | 0.154 | 0.795 |
| proposed at the floors' edge, 0.90 / 0.65 / 0.45 | 0.154 | 0.190 | 0.764 |
| worst case at the floors, 0.85 / 0.60 / 0.40 | 0.104 | 0.278 | 0.726 |
| governor, 0.95 / 0.78 / 0.55 | 0.302 | 0.098 | 0.830 |

So G4 (≥ 0.15 at the top prestige) and G1 bound how much risk the ascensions
may add. Tune the ascensions until the learning simulator reports G1 and G4
inside their bands at P4. The rating offset is not the knob.

**Tags hidden by mastery (decision c).** For each topic `k`:

```
n_k    = clean clears on topic k                 (Skill.NClean[k])
weak_k = Φ((μ_k + 100) / σ_k) < 0.8              P(δ_k > −100) < 0.8: not yet shown
                                                 within 100 of the general level
m_k    = Unlocked(k) && !weak_k ? clamp((n_k − 4) / 8, 0, 1) : 0
P(blind | k)   = 0.10 + m_k·(0.50 + asc.BlindChance)        cap 0.95
P(mystery | k) = m_k·(MysteryChance + asc.MysteryChance)
```

- One draw `u` per slot decides both kinds of blindness. If
  `u < 0.10 + 0.50·m_k`, the tags were hidden for mastery. Otherwise, if
  `u < 0.10 + m_k·(0.50 + asc)`, they were hidden for prestige.
- Mastery blindness is in p̂_base, so the aim compensates it: it is a
  learning condition. Prestige blindness is only in p̂_eff: it is risk.
- A topic still being learned (`m_k = 0`) keeps its tags except for the 0.10
  base, which is mild interleaving practice. Prestige never removes tags
  from it, and mystery never applies to it.
- A problem whose topic is `mixed` uses `m = clamp((N_clean − 20) / 40, 0, 1)`
  over all topics.
- `BlindAll` (the player's run modifier) and trial integration doors
  (`doors.go:252-255`) are unchanged. Decision (c) changes D5 for known
  topics only.

**Interleave slot** (neuro R6b; open decision (h), default on). In a topic
biome, with probability 0.33 per room, the target slot draws from the other
known topics (`m ≥ 0.5`), with tags always hidden. That blindness counts as
mastery blindness, so it is in p̂_base. It needs at least two known topics.

**The selection algorithm** keeps the engine's rule that one roll never
shifts the next (`doors.go:227-248`). It takes exactly 15 draws per room,
whatever the candidate counts.

```
rollRoom(fresh):
  u[0..14] ← State.RNG                                   fixed count
  sk ← Propagate(copy(Meta.Skill), now)                  never mutates Meta here
  slots ← Bands(biome kind, governor); dither target center by u[0]
  inter ← topic biome && u[1] < InterleaveChance && ≥ 2 known other topics
  for slot i (easy, target, stretch; boss: target only):
    mystery_i ← u[2+3i]     cursed_i ← u[3+3i]           (cursed: not boss, not review,
                                                          not Userland's easy slot)
    blind_ij  ← u[4+3i] against P(blind | k(j))          per candidate, same draw
    E_i ← eligible problems (not excluded, not in earlier slots,
           topic = biome topic | other known topics if inter | any in Userland/boss)
           with p̂_base ∈ band_i and p̂_eff ≥ floor, sorted by ID
    widen band_i by 0.03 up to 3 times while E_i is empty; then fall back
           as pick() does today (nearest p̂_base in topic, then any topic)
    j ← E_i[⌊u[11+i]·|E_i|⌋]   propensity_i = 1/|E_i|      (u[11..13]; boss uses u[11])
  if fresh: place one due review (M6) in the slot nearest its p̂_base,
           only if its p̂_eff ≥ floor              propensity 1
  Userland target slot fell back because the pool bottoms out → buy-back (below)
  display order: one of the n! permutations from u[14]; position logged
  State.Offers += Offer{...}                      §4.1
```

The set's propensity is `P(inter) × Π_i 1/|E_i|`. Each factor is conditional
on the slots before it, so the product is exact even when a widened band
overlaps an earlier one.

**Userland buy-back** (neuro P1.2). Two kinds of rule apply:

- **Door level.** Suppose no candidate reaches the target band because the
  pool bottoms out, which happens when the highest p̂_base is below the band.
  Then give the easiest candidate the smallest timer multiplier in {1.25,
  1.5, 2} that lifts p̂_eff to `0.92 − 0.02`, and log `reason: buyback`.
- **Boot level** (open decision (d)). At NewBoot, forecast
  P(crash in Userland) from the best p̂_eff the pool allows. If it exceeds G1
  (0.20), grant +1 life for the boot.

The timer cannot close the gap alone. For a 1000 player on an 800 door, at
the prior β_timer = 1.0 and a posterior plus item SD of 150 `[ran]`:

```
timer                           ×1     ×1.25   ×1.5    ×2
p̂_eff                          0.73   0.78    0.81    0.85
P(crash in 2 Userland doors)    0.46   0.40    0.35    0.28
```

With the uncertainty included, 0.92 on an 800 door needs μ ≥ 1282 (SD 150) or
1334 (SD 212) `[ran]`, not neuro's 1224. No local source goes below 800: CF
starts at 800, USACO at 834, and AtCoder's test data was withdrawn (DESIGN.md
"Platform facts"). So low-rated players need the life, or decision (e)'s rule
applied to Userland.

**Streak governor** (neuro P1.3). `Meta.Governor.EarlyCrashes` counts
consecutive boots that crashed in biome 0. When it reaches 2, NewBoot uses the
governor bands, and logs `reason: governor`. Reaching a merge window resets
the count. After three full clears in a row, the GRUB screen suggests the
next prestige. That is a UI hint, not a rule.

**Why each door was offered.** Each item carries a reason the door screen can
show, for example:

- `target: dp 1640, 72%`
- `review: usaco:1598, stage 2, last seen 19 d ago`
- `interleave: graphs, tags hidden (known topic)`
- `buyback: timer ×1.5, no easier door`

**Bytes** stay priced by `0.5 / p̂_base` (`doors.go:265-289`), so every door's
expected payout is equal and easy doors are not worth farming.

### M5. Route offers: a weighted draw, not a bandit

```
score_k = 1.0·[a review is due on k]
        + 1.0·min(days since k's last door, 30)/30         spacing
        + 0.5·clamp((σ_k − mean σ)/mean σ, −1, 1)          uncertainty
        − 2.0·[k offered earlier this boot]
w_k = exp(score_k)        over unlocked topics with pool (RouteMinPool, as today)
draw 2 without replacement, ∝ w (Plackett–Luce): 2 draws from State.RNG
P({i, j}) = w_i/W · w_j/(W − w_i) + w_j/W · w_i/(W − w_j)     logged
```

Weights are `[assumed]`. Staleness spreads practice of a topic over time
(Cepeda 2008). The player still chooses between the two, for autonomy (neuro
§4).

**Why not ZPDES** (Clement et al. 2015, proposed in neuro (c) M4). ZPDES
rewards an activity by its learning progress: success over the last d/2
attempts minus success over the d/2 before. Here that signal is gone by
construction, because the aim holds success near the target. Measured
through the lag instead, a gain of 3 points per door moves success by at
most 0.028 over a window of 8 doors (no tracking at all). The noise SD of that difference is 0.22 at d = 16
`[ran]`. A window of 16 topic doors is about 48 boots. The ZPDES field study
had 400 children. One player gets noise.

### M6. Reviews: FSRS-shaped stability, armed by an upsolve

A review item is a problem the player failed or barely passed. Its
re-solve, or a sibling problem, comes back when predicted recall falls to
`R* = 0.85`.

```
R(t, S) = (1 + (19/81)·t/S)^(−0.5)           FSRS-4.5 forgetting curve [cited]
I(S)    = (81/19)·S·(R*^(−2) − 1) = 1.637·S  days until R = R*
success: S ← S·(1 + e^{w8}·S^{−w9}·(e^{w10(1−R_t)} − 1))   R_t = R at the review
hinted:  the increment × 0.3                 (FSRS's "hard" factor, w15)
fail:    S ← S0, Stage ← 0, unarmed until the next upsolve (R1)
retire:  when I(S) > 180 days
```

This is the FSRS-4.5 form (open-spaced-repetition wiki, "The Algorithm"; Ye
et al. 2022) with the difficulty term dropped. Defaults `[assumed]` are
`S0 = 1.83 d`, `w8 = 2.69`, `w9 = 0.137`, `w10 = 1.05`. They give gaps of
3.0, 9.9, 29.4, 79.3 and 196 days `[ran]`, which matches neuro R3's
3/10/30/90 and grows when a review succeeds late.

**Triggers:**

- A crash creates an unarmed item.
- Any upsolve of that problem, editorial included, arms it: `S = S0`, due in
  3 days (neuro R1).
- A hinted clear, an overtime clear, or a clear with 2+ failed submits creates
  an armed item (neuro R5).
- The first review re-solves the same problem. Later reviews use a
  **sibling** (neuro R4): same topic, tag Jaccard ≥ 0.5 when tags exist, rating within ±100,
  never attempted. One RNG draw picks it, and the same problem is the
  fallback.
- A non-editorial upsolve after the due date counts as the review, as today
  (`meta.go:15-37`).

**Placement.** The predicted clear of a review door is
`p̂_review = R + (1 − R)·p̂_fresh`: the player either recalls the solution (or
the technique, for a sibling) or solves it fresh. This mixture is
`[assumed]`. The review goes into the room's slot whose band is nearest
`p̂_review`, and only if `p̂_eff ≥ floor`. Keep one review per boot and the
current room (biome 1, room 0, `doors.go:206-225`). When several are due,
offer the one with the lowest `R(now, S)` first.

**Fit.** Fit `S0` and `w8` by maximum likelihood of review outcomes under the
mixture, after 40 outcomes. Before that, use the defaults. FSRS parameters
come from flashcards (neuro M3), so only the shape transfers.

**Keep:** the blank file (`app/intents.go:223`), the rule that an editorial
upsolve never advances a review (`meta.go:34`), and restart on failure.
**Migrate:** convert each `Meta.Regressions` entry to an armed item, with `S`
set so that `I(S)` matches its remaining days.

### M7. Timers: keep `fitTimer` until a time model earns its place

`fitTimer` (`timer.go:45-86`) works and the neuro brief says to leave timers
mostly alone. The principled replacement is a censored lognormal,
`ln T_AC ~ N(μ_T + λ·(r_j − μ_g − μ_k)/s, σ_T²)` (van der Linden 2007):

- a clear gives an exact time;
- a timeout is censored at the timer;
- `:q!` and out of lives are censored at the time they ended. That treats
  competing risks as independent, which is a known bias.

It would remove the `TimerHeadroom` fixed-point hack. Build it only if
adding time to M1 improves prequential log loss (neuro P2.3), or if realized
timeouts among eventual solvers drift from 15%.

### M8. Engagement: a monitor, never a controller

Fit a logistic hazard of "no boot within 48 h of this boot's end" on:

- crashed in Userland;
- an early-crash streak of 2 or more;
- a crash streak of 3 or more;
- a full-clear streak of 3 or more;
- ln(uptime);
- hour-of-day bucket.

Use ridge priors centered on the EOMM ratios (Chen 2017: LLL vs LLW is 5.1%
vs 2.6%, +0.70 log-odds). With about one observation per boot it is
estimable after about 150 boots. Use it to check the governor's threshold of
2. Do not feed it into door choice.

### 2.9 Population prior from the Codeforces API (decision a)

A developer-run tool, `cmd/kprior`, builds the prior offline. The game never
calls Codeforces, so D7 is untouched. The tool outputs
`internal/game/model/prior_cf.json` (about 3 KB, checked in, with
provenance).

**Data** (checked 2026-10-14 `[ran]`):

- `contest.standings?contestId=N` is only served to anonymous GETs with no
  other parameters, so each contest comes as one full download. Round 911
  (Div. 2) returned 9,581 rows in 6.6 MB.
- `contest.ratingChanges?contestId=N` gives each contestant's `oldRating`
  (1.6 MB for that round).
- `problemset.problems` gives ratings and tags.
- The rate is one call per 2 s (DESIGN.md).
- About 300 rated Div. 2/3/4 and Educational rounds from 2019 to 2025 make
  600 calls, about 20 minutes, and about 2.4 GB of transient download. Reduce
  each contest at once to rows of (user hash, contest, problem, solved,
  rejected count, time). Keep no handles, and delete the raw files.

**Estimators** (Go, method of moments; no Python):

1. Map each problem to a topic with the game's own CF mapping
   (`content/codeforces/topics.go`), so a topic means the same in the prior
   and the game.
2. Anchor each contestant's general level at `oldRating`. Rows with
   `oldRating` in 800–2400 and contestants with 15 or more sampled contests
   are kept.
3. Compute per (user, topic) the information-weighted mean residual of
   `solved − σ((oldRating − r_j)/s)`.
4. Across users, `Cov(δ)` is the covariance of those means minus the mean
   binomial noise on the diagonal.
5. Shrink toward the block structure above (Ledoit & Wolf 2004), project to
   PSD, and write `τ_k` and `R`.
6. Write `σ_cf` as the spread of per-problem residuals beyond binomial noise.
   It is in-sample (CF ratings are fitted on the same rounds), so it is a
   lower bound; inflate it ×1.5 for doors.
7. Optionally, from about 2,000 `user.rating` histories (skip each user's
   first 6 contests, which are placement): regress squared rating changes on
   the gap for `q_g`. Use it as a weak prior with ×2 variance. Do not take γ
   from population data, because how much people practice is confounded
   with motivation.

**Caveats.** Population covariance between people is used as a within-person
prior, which is an ergodicity assumption. Contest solves are not door clears:
the prior informs `R`, `τ` and `σ_cf` only, never β or the timers.

### 2.10 Where state lives, and the determinism rules

```
rules.Rules                        snapshot in State.Rules; locked per boot (D2)
├─ Aim     "elo" | "model"         switch for slice ML6
├─ Model   ModelParams             φ, version hash, pin
└─ Policy  PolicyParams            bands, floors, dither, mastery, interleave,
                                   route weights, review params, governor
game.Meta                          persistent; the engine mutates it at close
├─ Skill    μ, Σ, At, N[], NClean[], LastDoor[topic]       M2
├─ Shadow   *Skill                                         slice ML4 only
├─ Reviews  []Review                                       M6 (replaces Regressions)
└─ Governor {EarlyCrashes, FullClears}
game.State                         one boot
└─ Offers   []Offer                                        §4.1
store                              outside the engine
└─ model_versions, offers, events, skill_history           §4.2
```

1. **Pure functions.** `Predict`, `Update`, `Propagate` and `Replay` in a new
   `internal/game/model` package take `now` as an argument and do no I/O.
2. **A fixed number of RNG draws** per room (15) and per route offer (2),
   always from `State.RNG`.
3. **No unsorted map iteration** where order affects a result. Topics are
   indexed by `catalog.Topics` order.
4. **Floating-point fusion.** The Go spec lets an implementation fuse
   `x*y + z` into an FMA, and the result can differ by architecture. Within
   one binary on one machine the result is stable, which is what resuming a
   boot needs. Across machines or Go versions, replays may differ in the last
   bits. Compare p̂ to band edges after rounding to 1e-9, and use
   `float64(x*y)` where an exact cross-machine replay is wanted.
5. **JSON round-trip.** Every new struct uses exported fields with no
   interfaces, and is covered by `TestJSONRoundTrip` (`property_test.go:517`).

**Cost summary:**

| Component | State | Per event | Offline |
|---|---|---|---|
| M1 predict | none | ~20 flops per candidate, < 0.2 ms per room | |
| M2 filter | 2.2 KB (5 KB JSON) | ~300 flops per door | |
| M3 fit | ~25 floats per version | | 1–2 s per fit |
| M4 selection | ~1 KB of Offer per room | 15 draws | |
| M6 reviews | ~100 B per item | O(items) | ms |
| §2.9 prior | 3 KB JSON | | ~20 min of API calls, once |
| learning sim | | ~3 ms per boot `[ran]` | policy grid ≈ 2M boots ≈ 10 min on 10 cores |

### 2.11 Do not build

| Do not build | Why | Instead |
|---|---|---|
| RL, or bandits rewarded by learning gain | One topic's slope needs 160+ topic doors (neuro §2); comparing policies needs far more | a fixed policy tuned in the learning sim |
| ZPDES route bandit | the aim removes its reward signal; SNR < 0.2 (M5) | the weighted route draw |
| Thompson sampling inside bands | its propensity is an integral over θ, not closed form; the slots already spread doors | slots plus a logged uniform dither |
| DKT, BKT | simple models match DKT (Khajah 2016); BKT's binary mastery does not fit a continuous-difficulty pool | M1 + M2 |
| Per-topic learning rates or decay half-lives | not identifiable for years (neuro §2) | global γ and q, fitted |
| Per-problem difficulty learning | each problem is seen about once | per-source offset and noise |
| Glicko-2 volatility | an extra latent the data cannot identify | fixed day and door noise, fitted |
| A 3PL guessing parameter | nobody gets AC on CP by guessing | a lower asymptote of 0 |
| Competing-risks survival, a joint speed-accuracy model | unproven value; heavy | logistic + covariates; M7 if it earns it |
| An engagement-optimizing objective (EOMM-style) | about one observation per boot | hard guardrails |
| Hyperparameters learned inside a boot | breaks D2 | M3 between boots |

## 3. What to replace, what to keep, and the migration slices

| Today `[code]` | Fate | Reason |
|---|---|---|
| Elo `rate`, `ratingK`, cold K (`rating.go:41-106`) | replace with M2 | no uncertainty or time; biased by stakes (−233 at P4 `[ran]`) |
| `TopicFollow`, `TopicRatingMin` (`rules.go:336-337`) | replace: the hierarchy does both, smoothly | |
| Calibration offsets (`calibration.go:37-105`) | replace with M1's σ, o and β plus the G6 monitor; keep the code for `Aim = "elo"` | one number absorbs bias, shape and timers |
| `PrestigeOffset` 25 (`rules.go:377`) | set to 0, decision (b); D15 becomes moot | prestige is risk; the floors hold G5 |
| `TargetClear` per biome (`rules.go:315-317`) | keep as the target-slot center; add slots and floors | |
| `RatingWindows` (`rules.go:335`) | replace with p̂ bands; keep as the fallback | |
| `RegressionDays` 3/17 (`rules.go:329`) | replace with M6 | gaps too short; crash-only; no arming |
| `HintedScore`, `OvertimeScore` (`rules.go:343`, `:374`) | replace with the cut point `c` | |
| Flat `BlindChance` (`rules.go:319`) | replace with the mastery rule, decision (c) | |
| `fitTimer`, `doorBytes` at `0.5/p̂_base`, fixed draws per door, retreats unrated, blank-file reviews, unaimed trials, capability gates | keep | already match the evidence (neuro (a)) |
| `sim.Player` | keep as the no-learning variant | |

Each slice that changes play sits behind a rules field, so a preset can turn
it off. Rollback is flipping the field back.

| Slice | Change | Verify (observable) | Done when |
|---|---|---|---|
| **ML1** log in the engine | `State.Offers`, `Attempt.Events`, `Attempt.{Biome, Offer, LivesAtOpen, DucksAtOpen}`; today's `pick` logs its propensities, inclusion = needed ÷ \|tier set\| (`doors.go:115-135`; approximate where a union lists an ID twice, `doors.go:131`) | golden test: same seed gives the same doors as before; Monte Carlo over 10⁴ seeds: inclusion frequencies match logged propensities (χ², p > 0.01); `TestDeterminism`, `TestJSONRoundTrip` | merged with no behavior change |
| **ML2** store and app events | migration 2 (§4.2); `Store.Event`; app emits the kinds in §4.3 | store test: rows after a scripted door session; migration from v1 keeps old rows | rows appear in a real boot |
| **ML3** learning sim | §5.2 player; item noise drawn per problem | with learning and item noise off, it reproduces today's report exactly (same seeds); EOMM churn ratio ≈ 2; ideal player calibrated | report runs as a balance-style test |
| **ML4** model in shadow | `internal/game/model`; `Door.PShadow`, `Meta.Shadow`, updated at close; aim unchanged | 1-D, no-noise filter equals Glicko-1 to 1e-9; replay equals incremental; sim ideal player: \|μ − θ\| < 2 SD, ECE < 0.02; real store: prequential log loss vs `Door.SolveP` | the report exists; no decision yet |
| **ML5** fit tool | M3, `model_versions`, `katharsis model fit` | recovers the sim's generating σ_c and β within 2 SE; byte-identical output on rerun | accepted versions appear |
| **ML6** aim switch | `Aim = "model"`; slots and floors; `CalibRate = 0`; `PrestigeOffset = 0`; deduplicate union IDs before sampling | sim: each slot's realized clear within 3 SE of its mean p̂; G5 holds at P0–P4; balance report against G1–G4 | ML4's real-data log loss is no worse than Elo's (F1) |
| **ML7** guardrails | governor; Userland buy-back; boot-level life (decision d) | sim at 900, 1000, 1100: G1 ≤ 0.20 or a logged reason | |
| **ML8** mastery tags, interleave | decision (c); open decision (h) | unit tests of `m_k`; sim: blind share by topic state; real: blind-vs-tagged residuals by topic over time (neuro R6 test) | |
| **ML9** reviews | M6; R1, R4, R5; decision (e) | interval unit tests (3.0, 9.9, 29.4, 79.3); sim retention vs 3/17; real: take rate, outcome calibration | |
| **ML10** route weights | M5 | propensity Monte Carlo; sim: gaps between a topic's doors | |
| **ML11** CF prior | §2.9 | held-out CF users: prediction beats the independent-topic prior; R is PSD; rerun is identical | replaces the hand-set R and τ |
| **ML12** gated extras | M7, M8, extension covariates | each must beat the baseline on prequential log loss (§5.1) | |

## 4. Logging to add now

Log first: data from boots before ML1 cannot be recovered. Everything the
engine sees goes into `State` or `Attempt` JSON, which the store already
keeps whole. The engine stays pure. App-only facts go to a new `events`
table.

### 4.1 Engine types (additive changes to the `types.go` contract)

```go
type AttemptEvent struct {
	At        time.Time
	Kind      string // "submit", "quack", "overtime"
	Verdict   string `json:",omitempty"`
	Passed    int    `json:",omitempty"`
	Total     int    `json:",omitempty"`
	FirstFail int    `json:",omitempty"` // 1-based first failing test; 0 unknown
	Level     int    `json:",omitempty"` // quack level
}

// Attempt gains:
//   Biome       int            index into State.Biomes (the S8d plan)
//   Offer       int            index into State.Offers it was chosen from
//   LivesAtOpen, DucksAtOpen int
//   Events      []AttemptEvent

// Door gains:
//   P, PBase    float64        p̂_eff and p̂_base at offer time (SolveP keeps
//                              the old meaning until ML6, then = PBase)
//   PShadow     float64        ML4 only
//   Slot        string         easy | target | stretch | review | interleave | boss | fallback
//   Propensity  float64        P(this problem in this slot | room state)
//   BlindWhy    string         "" | mastery | prestige | run | trial
//   TimerBuy    float64        buy-back multiplier, 0 when none

type OfferItem struct {
	Problem    catalog.ID
	Source     catalog.Source
	RatingSrc  string // source class (M1)
	Topic      string
	Rating     int
	Slot       string
	Band       [2]float64 // p̂_base band the slot searched
	P, PBase   float64
	Propensity float64
	Eligible   int // |E| of the slot
	Tier       int // 0 in band; 1..3 widened; 4+ fallback
	Blind, Mystery, Cursed, Review bool
	BlindWhy   string
	Timer      time.Duration
	Bytes      int64
	Position   int    // display index
	Reason     string // what the door screen can show
}

type Offer struct {
	Seq        int
	Kind       string // "doors" | "route"
	Biome      int
	Room       int // -1 for a route offer
	At         time.Time
	Policy     string     // e.g. "elo-window/1" or "slots/1 model=3f2a"
	Reason     string     // fresh | reroll | governor | buyback
	RNG        uint64     // State.RNG before the roll, for audits
	Target     float64    // dithered target center; 0 for routes
	Floor      float64
	Mu, SD     float64    // μ and σ of the biome topic at roll time
	Items      []OfferItem // route items use Topic, P (= weight share), Propensity
	SetProp    float64     // propensity of the whole set
	Chosen     int         // −1 until chosen
	ChosenAt   time.Time
}
// State gains: Offers []Offer
```

### 4.2 Migration 2 (`internal/store/migrate.go`; append-only, as its rule says)

```sql
-- 2: ML logging.
CREATE TABLE offers (
	boot      INTEGER NOT NULL REFERENCES boots (id),
	seq       INTEGER NOT NULL,        -- index into State.Offers
	kind      TEXT    NOT NULL CHECK (kind IN ('doors', 'route')),
	biome     INTEGER NOT NULL,
	room      INTEGER NOT NULL,
	at        INTEGER NOT NULL,
	policy    TEXT    NOT NULL,
	reason    TEXT    NOT NULL,
	set_prop  REAL    NOT NULL,
	chosen    INTEGER,                 -- NULL until chosen
	chosen_at INTEGER,
	json      TEXT    NOT NULL,        -- game.Offer
	PRIMARY KEY (boot, seq)
);

CREATE TABLE events (
	id      INTEGER PRIMARY KEY,
	at      INTEGER NOT NULL,
	session INTEGER,                   -- id of this run's app_start event
	boot    INTEGER,                   -- NULL outside a boot
	attempt INTEGER,                   -- State.History index, NULL if none
	problem TEXT    NOT NULL DEFAULT '',
	kind    TEXT    NOT NULL,
	json    TEXT    NOT NULL DEFAULT '{}'
);
CREATE INDEX events_kind_at ON events (kind, at);
CREATE INDEX events_boot ON events (boot, attempt);

CREATE TABLE skill_history (
	id      INTEGER PRIMARY KEY,
	at      INTEGER NOT NULL,
	boot    INTEGER,
	attempt INTEGER,
	model   TEXT    NOT NULL,          -- version; 'shadow:<v>' during ML4
	topic   TEXT    NOT NULL,          -- 'general' or a catalog topic
	mu      REAL    NOT NULL,          -- μ_g, or μ_g + μ_k
	sd      REAL    NOT NULL,
	n       INTEGER NOT NULL           -- rated doors behind it
);
CREATE INDEX skill_history_topic_at ON skill_history (topic, at);

CREATE TABLE model_versions (
	id         TEXT    PRIMARY KEY,    -- hash of the params JSON
	created    INTEGER NOT NULL,
	params     TEXT    NOT NULL,       -- rules.ModelParams
	n_doors    INTEGER NOT NULL,
	heldout_ll REAL,                   -- new φ on the last 20%
	current_ll REAL,                   -- current φ on the same doors
	accepted   INTEGER NOT NULL
);

ALTER TABLE boots    ADD COLUMN rules_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE boots    ADD COLUMN model      TEXT NOT NULL DEFAULT '';
ALTER TABLE attempts ADD COLUMN biome      INTEGER;
ALTER TABLE attempts ADD COLUMN p          REAL;
ALTER TABLE attempts ADD COLUMN p_base     REAL;
ALTER TABLE attempts ADD COLUMN slot       TEXT NOT NULL DEFAULT '';
ALTER TABLE attempts ADD COLUMN propensity REAL;
ALTER TABLE attempts ADD COLUMN offer      INTEGER;
```

**Write paths:**

- `Checkpoint` inserts `State.Offers[next:]` the way `putAttempts` does
  (`store/boot.go:166-192`).
- It runs `UPDATE offers SET chosen = ?, chosen_at = ? WHERE boot = ? AND
  seq = ? AND chosen IS NULL` for the latest offer only, so a Tick a second
  rewrites nothing.
- It appends `skill_history` rows when `Meta.Skill` changed, comparing with
  the last row per topic, as `putRating` does (`store/store.go:198-212`).
- Old rows keep NULL in the new columns. Backfill `p` from
  `json_extract(json, '$.Door.SolveP')` and mark them `slot = 'legacy'`.

**Volume.** About 1 KB of offer JSON per room and about 10 rooms per boot,
so about 10 KB per boot and 10 MB per 1,000 boots.

### 4.3 App events (kinds and payloads)

| Kind | When | Payload |
|---|---|---|
| `app_start` / `app_quit` | process start and clean exit; an unclean exit ends at the session's last event | version, rules preset names, tmux or not |
| `samples_judged` | every `:w` judge | compile ok, samples passed/total, wall ms |
| `editorial_open` / `editorial_close` | RTFM editorial shown and left (`app/intents.go:122-160`) | problem; during a boot or not |
| `upsolve_open` / `upsolve_judged` / `upsolve_close` | upsolve door (`app/intents.go:185-262`) | verdict, passed/total, editorial seen, elapsed |
| `screen` | door, route and merge screens shown | screen name (dwell = next event − at) |
| `self_report` (optional) | at most one prompt per boot, after a crash or clear | easy / right / hard |

## 5. Evaluation without a population

### 5.1 Prequential prediction on real data is the one real-data test with power

- Replay the store in `closed` order. For each rated attempt, record the
  model's p̂ before the update.
- Compare models by mean log loss, Brier score, a reliability table
  (deciles of p̂), and AUC.
- Use a paired Diebold–Mariano test on the per-door loss differences, with a
  block bootstrap by boot, because doors within a boot share state.
- Baselines: `Door.SolveP` as logged (today's Elo plus calibration), and M2
  with `δ ≡ 0`.

**Power.** With a per-door SD of the loss difference of about 0.15
`[assumed; measure it]`, detecting an improvement Δ at 80% power needs about
`(2.8·0.15/Δ)²` doors. That is 71 doors for Δ = 0.05, and 441 for Δ = 0.02.
Only large improvements show within months.

**Audit replay for the learning trend.** Run the same filter and φ on audit
rows only: the first attempt at a problem (no earlier `attempts` row or
`Meta.Solves` entry), not a regression, not a trial. Score `y* = 1` only for
a clean clear, so a hinted clear counts as a miss and is not dropped.
Dropping hinted attempts would select on the outcome. The 1-of-3 pick
inflates the level by an unknown amount. The bias is roughly constant, so it
cancels in a trend more than in a level.

### 5.2 The learning simulator specifies what the policy is tuned on

Today's `sim.Player` has a fixed rating (`sim.go:28-43`), so the simulator
can balance a preset but cannot rank policies by learning (neuro (c)). Add a
learning player. Every number below is `[assumed]` and tunable. The decision
rule at the end of this section only accepts results that hold across these
assumptions.

```
true skill    θ*_k(t) = base_k + slow_k(t) + fast_k(t)          per topic
              θ*_g gets ρ = 0.3 of every topic gain (transfer)
items         b*_j = r_j + o*_c + ε_j,  ε_j ~ N(0, σ*_c²), fixed per problem
              (seeded from a hash of the problem ID; today's sim has none)
door play     as today (capable ~ logistic, lognormal time, WAs, ducks),
              using θ*_k(j) and b*_j; same code path when learning is off
gain          g = γ·G(p*)·F(outcome),  p* = true capable chance
              slow += ζ(gap_k)·g,  fast += (1 − ζ(gap_k))·g
              ζ(gap) = 0.3 + 0.7·(1 − e^(−gap/3 d))   spaced practice consolidates
decay/day     fast ·= 2^(−1/21),  slow ·= 2^(−1/365)
              (calibrate ζ so massed practice keeps ~50% of a gain after
               6.5 months: Tatel & Ackerman 2025)
G variants    flat: 1
              inverted U: exp(−(p − 0.75)²/(2·0.15²)), normalized to mean 1
              harder better: ∝ 1.6 − p
γ variants    1, 3, 6 points per door
F             clean clear 1 · hinted 0.6 · fail then own upsolve 0.8 ·
              fail then editorial upsolve 0.5 · fail, no upsolve 0.1
              (Rowland 2014: no feedback, no gain) · blind clean ×1.2 (variant ×1)
re-solve      true stability S*_j; P(clear) = R*(t) + (1 − R*)·p*_fresh,
              with true S0* = 3 d, w8* = 2.3 (unlike M6's defaults on purpose)
choice        softmax over doors of ln(bytes) + λ·ln(p_believed),
              p_believed = σ(logit p* + N(0, 0.5²)),  λ ∈ {1, 2, 4};
              today's bytes×P greedy player is one variant
sessions      gaps: 0.7 Exp(1.2 d), 0.25 Exp(4 d), 0.05 Exp(21 d);
              another boot the same day: 0.3 after a clear, 0.15 after a crash
churn         at a boot's end: h = σ(α0 + α1·earlyCrash + α2·[earlyStreak ≥ 2]
              + α3·[crashStreak ≥ 3] + α4·[clearStreak ≥ 3] + α5·ln uptime),
              α0 ≈ logit 0.026, α3 ≈ +0.70 (EOMM); on churn, 0.7 a break of
              Exp(30 d), 0.3 quit for good
```

- **Run.** 200 simulated players per cell, with starting levels 900, 1000,
  1300 and 1600. Simulate 365 days or until the player quits. Use common
  random numbers: the same seeds per player across policies.
- **Policies.** Today's engine, ML6, and ML6 with one component removed at a
  time (slots, mastery tags, interleave, review schedule, route weights).
  Also mid centers 0.65, 0.72 and 0.80, and ascension sets for G4.
- **Metrics per policy.**
  - Durable skill: mean `θ*_k` gain over unlocked topics after 30 more days
    without practice.
  - Hours played, and P(quit).
  - G1–G6, and the number of violations.
  - The estimator's RMSE of `μ − θ*`, and the ECE of p̂.
- **Decision rule.** Adopt a component only if its gain in durable skill is
  positive, with a 90% interval above 0, under all three G variants, and no
  guardrail gets worse. If the ranking flips across G variants, the evidence
  cannot choose. Then keep the simpler policy and say so.
- **Cost.** ≈ 3 ms per boot `[ran]`. 200 players × ~150 boots × 9 (G, γ)
  cells × 8 policies is about 2M boots, about 10 minutes on 10 cores. Gate it
  behind an env var, like `TestBalanceReport`.

### 5.3 Off-policy evaluation from logged propensities covers door-level outcomes only

Logged data per room `i`: context `x_i` (skill, biome, stakes), the offered
set `S_i` with `π0(S_i | x_i) = SetProp`, the choice, and the outcome `y_i`
(clear, crash in room, time, choice share). For a candidate policy `π1`
(say, other band centers):

```
weights      w_i = π1(S_i | x_i) / π0(S_i | x_i)
IPS          V̂ = (1/n) Σ w_i y_i                     (Li et al. 2011 replay is the uniform case)
SNIPS        V̂ = Σ w_i y_i / Σ w_i                   (Swaminathan & Joachims 2015)
DR           V̂ = (1/n) Σ [q̂(x_i, π1) + w_i (y_i − q̂(x_i, S_i))]
             q̂ from M1 plus a fitted choice model    (Dudík et al. 2011)
ESS          (Σ w)² / Σ w²;  no estimate is reported when ESS < 30
```

- **Support.** With exact set matching most weights are 0. So estimate at the
  slot level (outcome of the chosen door vs that slot's item propensities),
  and use the per-room dither of the target center for overlap between
  nearby bands. This ignores how set composition shifts the choice, which is
  an approximation.
- **What OPE can answer:** clear rates, crashes per room, time on door, the
  choice share by slot, and possibly 48-hour return when policies differ per
  boot.
- **What it cannot answer:** learning. Sequential importance weights over
  hundreds of rooms have unbounded variance (Precup et al. 2000). That
  question belongs to §5.2.

### 5.4 Metrics, in one place

- **Real data, weekly:** G1–G6 (§1.3), off-band rate, buy-back rate,
  governor activations, review take rate and review outcome calibration.
- **Real data, per refit:** prequential log loss and reliability vs the last
  version.
- **Real data, quarterly:** the audit learning trend per topic, with its
  band.
- **Simulator:** §5.2's metrics, per policy and per variant.

### 5.5 What would falsify the approach

| # | Observation | Conclusion | Action |
|---|---|---|---|
| F1 | after 200 rated doors, M2's prequential log loss is not better than `Door.SolveP` (the block-bootstrap CI includes 0 or favors Elo) | uncertainty and time do not pay for this player | keep Elo; still adopt slots and floors on the Elo p |
| F2 | \|G6 z\| > 3 persistently in one biome after a refit | M1 is misspecified there (lapse, timer, choice) | add the extension covariate or lapse; refit |
| F3 | M2 with `δ ≡ 0` predicts as well as with topics | topic deviations are noise at this data size | aim with `θ_g` only; topics stay for curriculum and display |
| F4 | residuals of the first 3 doors after 7+ day breaks are no wider than the rest; or they are biased one way | `q_g` per day ≈ 0; or a mean decay is needed | set `q_g` small; or fit λ |
| F5 | G1 > 0.20 at ratings under 1300 with buy-back on | buy-back is too weak | decision (d)'s life rule or (e) applied to Userland |
| F6 | the sim's policy ranking flips across G variants | the literature cannot decide; neither can the sim | ship the simpler policy |
| F7 | chosen doors clear more than p̂ (> 0.05 over 100 doors) while boss doors (no choice) do not | self-selection bias in the 1-of-3 pick | report it; correct the level for display; do not trust the 1-of-3 doors' level |
| F8 | sibling reviews clear like same-problem reviews | the technique transferred (neuro R4, question 2) | keep siblings, and consider them from stage 1 |
| F9 | mastery-blind doors stay below their p̂ by a growing margin in a topic | the topic was not known; the mastery rule fires too early | raise N0 and N1 |

## 6. Decisions

### 6.1 Settled on 2026-10-14

- **(a) Population priors from the Codeforces API: yes, offline.** §2.9 is
  slice ML11. The hand-set `R` and `τ` serve until it lands. The game binary
  makes no network call to Codeforces, so D7 stands.
- **(b) Prestige holds the mid biome at 0.6 or above and adds risk.** M4:
  `PrestigeOffset = 0`, the same bands at every prestige, floors on p̂_eff,
  and ascensions as the only difficulty knob. D15 (calibration minus the
  prestige offset) is superseded once ML6 lands. The workfile owner should
  record that.
- **(c) Hide tags by mastery.** M4's `m_k` rule. Topics still being learned
  keep their tags, and prestige blindness and mystery apply only to known
  topics. This changes D5 for known topics.

### 6.2 Open, each with a recommended default

| # | Question | Recommendation | Why |
|---|---|---|---|
| (d) | When the content floor makes Userland unreachable (rating < ~1300), what buys the clear chance back? | +1 life for the boot when the forecast P(crash in Userland) > 0.20, plus the door-level timer buy-back | even a doubled timer leaves a 1000 player at 28% Userland crashes (M4); no local content below 800 exists |
| (e) | Does a failed review door cost a life instead of the boot (neuro R2)? | yes | testing works without stakes; otherwise players avoid reviews and the review take rate collapses |
| (f) | Should φ refit automatically between boots? | yes, with the held-out acceptance test, `model_versions`, and `[model] pin` | keeps calibration current without breaking D2 |
| (g) | Should doors show p̂ to the player? | no on doors; yes after the door (crash and clear screens, `top`) | shown on doors, the pick stops measuring self-assessment and invites gaming; shown after, it is calibration feedback |
| (h) | One interleave door per topic-biome room, a third of the time (neuro R6b)? | yes, in ML8, kept only if the sim's decision rule passes | practice choosing the technique is the interleaving effect (Rohrer 2020, d = 0.83) |
| (i) | Optional self-reported CF rating as the cold-start prior (σ 150) on GRUB? | yes, optional | skips the boosted cold start for experienced players; a typed number is not platform integration |

## 7. References

Methods relied on here. The learning-science sources are in neuro (d).

- Chen et al. (2017). EOMM: an engagement optimized matchmaking framework. *WWW 2017*. https://doi.org/10.1145/3038912.3052559
- Clement, Roy, Oudeyer & Lopes (2015). Multi-armed bandits for intelligent tutoring systems. *JEDM* 7(2). https://arxiv.org/abs/1310.3174 (reward definition and study size checked in the HTML version)
- Dawid (1984). Present position and potential developments: some personal views. Statistical theory: the prequential approach. *JRSS A* 147(2):278–292. https://doi.org/10.2307/2981683
- Diebold & Mariano (1995). Comparing predictive accuracy. *JBES* 13(3):253–263. https://doi.org/10.1080/07350015.1995.10524599
- Dudík, Langford & Li (2011). Doubly robust policy evaluation and learning. *ICML 2011*. https://arxiv.org/abs/1103.4601
- Glickman (1999). Parameter estimation in large dynamic paired comparison experiments. *Applied Statistics* 48(3):377–394. https://www.glicko.net/research/glicko.pdf ; formulas in the Glicko system description, https://www.glicko.net/glicko/glicko.pdf (checked via its Wikipedia transcription)
- The Go Programming Language Specification, "Arithmetic operators" (fused floating-point operations). https://go.dev/ref/spec (checked)
- Khajah, Lindsey & Mozer (2016). How deep is knowledge tracing? *EDM 2016*. https://arxiv.org/abs/1604.02416
- Ledoit & Wolf (2004). A well-conditioned estimator for large-dimensional covariance matrices. *J. Multivariate Analysis* 88(2):365–411. https://doi.org/10.1016/S0047-259X(03)00096-4
- Li, Chu, Langford & Wang (2011). Unbiased offline evaluation of contextual-bandit-based news article recommendation algorithms. *WSDM 2011*. https://arxiv.org/abs/1003.5956
- Lomas et al. (2017). Is difficulty overrated? *CHI 2017*. https://doi.org/10.1145/3025453.3025638
- MacKay (1992). The evidence framework applied to classification networks. *Neural Computation* 4(5):720–736. https://doi.org/10.1162/neco.1992.4.5.720
- Mirzayanov (2018). Codeforces: problem difficulties. https://codeforces.com/blog/entry/62865
- Open Spaced Repetition. The Algorithm (FSRS-4.5, FSRS-5, FSRS-6 formulas). https://github.com/open-spaced-repetition/awesome-fsrs/wiki/The-Algorithm (checked)
- Pelánek (2016). Applications of the Elo rating system in adaptive educational systems. *Computers & Education*. https://doi.org/10.1016/j.compedu.2016.03.017
- Precup, Sutton & Singh (2000). Eligibility traces for off-policy policy evaluation. *ICML 2000*:759–766. https://mlanthology.org/icml/2000/precup2000icml-eligibility/
- Rohrer, Dedrick, Hartwig & Cheung (2020). A randomized controlled trial of interleaved mathematics practice. *J. Educational Psychology* 112(1):40–52. https://doi.org/10.1037/edu0000367
- Samejima (1969). Estimation of latent ability using a response pattern of graded scores. *Psychometrika Monograph Supplement* 17. https://doi.org/10.1007/BF03372160
- Settles & Meeder (2016). A trainable spaced repetition model for language learning. *ACL 2016*. https://aclanthology.org/P16-1174/
- Swaminathan & Joachims (2015). The self-normalized estimator for counterfactual learning. *NeurIPS 28*. https://papers.nips.cc/paper/2015/hash/39027dfad5138c9ca0c474d71db915c3-Abstract.html
- Szczecinski & Tihon (2023). Simplified Kalman filter for online rating: one-fits-all approach. *J. Quantitative Analysis in Sports* 19(4):295–315. https://doi.org/10.1515/jqas-2021-0061 ; https://arxiv.org/abs/2104.14012
- Tatel & Ackerman (2025). Skill decay meta-analysis. *Psychological Bulletin* 151(6):696–736. https://doi.org/10.1037/bul0000481 (abstract)
- van der Linden (2007). A hierarchical framework for modeling speed and accuracy on test items. *Psychometrika* 72(3):287–308. https://doi.org/10.1007/s11336-006-1478-z
- Wilson, Karklin, Han & Ekanadham (2016). Back to the basics: Bayesian extensions of IRT outperform neural networks for proficiency estimation. *EDM 2016*. https://arxiv.org/abs/1604.02336
- Ye, Su & Cao (2022). A stochastic shortest path algorithm for optimizing spaced repetition scheduling. *KDD '22*. https://doi.org/10.1145/3534678.3539081
- Codeforces API, checked 2026-10-14 with four public, anonymous calls: `contest.standings?contestId=1900&from=1&count=1` (refused: non-gym standings are served only to parameter-free anonymous GETs), `contest.standings?contestId=1900` (9,581 rows, 6.6 MB), `contest.ratingChanges?contestId=1900` (1.6 MB), `user.rating`. https://codeforces.com/apiHelp (the HTML page returns 403 to scripts, DESIGN.md)
