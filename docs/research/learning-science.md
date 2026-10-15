# Learning science for katharsis: how to pick doors

Neuro thread, 2026-10-14. Input for the ML thread that will design the
door-selection pipeline. Scope: `internal/game` (rating, calibration, doors,
timer, meta, engine), `internal/rules`, `internal/learn`, `internal/store`. No
code was changed. Line numbers are as of this date; other agents are editing
these files, so they may drift by a few lines.

**Evidence grades.** *Strong*: meta-analyses or several RCTs, in a domain close
to ours. *Moderate*: several studies, or one strong study, or strong evidence
from a distant domain. *Weak*: single studies, correlational designs, or
findings that failed to replicate. *Speculative*: theory, simulations, or my
own extrapolation.

**Domain tags** say where a finding comes from: `[verbal]` word and fact
memory, `[math]` school mathematics, `[cat]` category learning, `[motor]`,
`[percept]` perceptual learning, `[games]` video or educational games,
`[tutor]` intelligent-tutor logs, `[animal]`, `[sim]` simulation only, `[org]`
lab tasks from organizational psychology. **No study cited here used
competitive programming.** Every mapping to CP is an inference, graded as
such.

## (a) Summary

Two findings change the ML thread's job.

1. **Optimize learning per hour × hours played; the hours are what is at
   risk.** The player quits long-term CP practice and plays roguelikes for
   hours. In large experiments, easier versions keep people playing
   (Lomas 2013: engagement rose steadily all the way to 63% success, the
   easiest tested; Papoušek 2016: 89% vs 81% still playing at 95% vs 50%
   success, though returns later ran slightly the other way, 15.2% vs
   16.8%). More practice produced more learning (Jansen 2013). A
   loss-loss-loss streak churns about twice as often as loss-loss-win (EOMM:
   5.1% vs 2.6%). Learning per item was nearly flat from 50% to 80% success
   and worst at 95% (Papoušek). So engagement gets hard guardrails, and
   learning is tuned inside them.
2. **A per-topic "internal learn rate" cannot be measured for one player,
   and probably need not be.** Across 27 tutor datasets, learning per
   practice varies little between students while starting knowledge varies
   a lot (Koedinger 2023; contested by Lee 2026). In this game, a topic's
   slope needs 160–320 doors in that topic (roughly 500–1000 boots) to pin
   down; its level needs 20–80. Estimate levels per topic with uncertainty
   and forgetting; take the difficulty-to-learning curve from the
   literature instead of learning it online.

| # | Recommendation (ranked) | Learning | Coming back | Evidence | Where |
|---|---|---|---|---|---|
| 1 | **Guardrails.** A streak governor after two boots that crash early; buy back Userland's clear chance with lives or time when the target is below the pool floor (a 1000-rated player crashes in Userland 42% of the time, not 15%) | low | high | moderate | §1 P1.2–P1.3 |
| 2 | **Time-aware skill state.** A hierarchical Glicko-style filter per topic; uncertainty grows per day away; first doors after a break aimed conservatively | med | high | moderate | §2 P2.1, §3 R9 |
| 3 | **Retrieval as the learning engine.** Regressions only after an upsolve (retrieval at ≤ 50% success without feedback does nothing, g = 0.03); a failed regression costs a life, not the boot; gaps 3/10/30/90 days or adaptive; sibling problems after stage 1; reviews for hinted, overtime and WA-heavy clears too | high | med | strong (verbal, math); moderate for CP | §3 R1–R5 |
| 4 | **Practice choosing the technique.** Tags hidden by mastery; an off-topic door in topic biomes; trial integration doors from earlier topics | high | low | moderate–strong (math RCT d = 0.83; math meta g = 0.34) | §3 R6–R7 |
| 5 | **Feedback built on the attempt.** The player's code beside the editorial plus a one-line self-explanation; a third duck level (the key observation) | med | low | moderate | §5 H1–H2 |
| 6 | **Log now.** Offered door sets with propensities, saves, submits, quacks, sessions, per-topic snapshots | enabling | enabling | | §6 |
| 7 | **A learner model in `sim`.** True skill that grows and decays, plus a quit model, so policies are compared before the player sees them | enabling | enabling | | (c) |
| 8 | **Targets.** Keep 0.92 / 0.70 / 0.50 (0.79 per door over a boot, inside the 0.65–0.85 band); spread each room on purpose; stop prestige pushing the mid biome below about 0.6 | low–med | med | weak–moderate | §1 |
| 9 | **Rewards.** Give bytes a sink tied to learning acts; the crash screen shows what persisted; due reviews on GRUB; no streak penalties | low | med | moderate–weak | §4 |

**Do not build:** RL or bandits rewarded by measured learning gain
(underpowered by orders of magnitude, §2); deep knowledge tracing (simple
models match it); per-topic learning-rate parameters.

**Already right; keep:** blocked unit then blind integration in trials;
instruction first in trials and attempt first in boots (matches expertise
reversal); soft timer and visible failing input in trials; regressions from
a blank file, with a delayed first test, a reset on failure, and editorial
upsolves never counting as recall; the core dump, which makes loss aversion
pay for upsolving; equal expected payout per door, so easy doors are not
worth farming; the 50% boss; calibration from realized clear rates.

## (b) Findings by topic

### 1. Optimal challenge: what success probability, for what purpose

First, a correction to the framing. Prestige is named Bronze to Advanced, but
door difficulty does not come from the prestige name. It comes from the
player's rating, plus 25 points per prestige level (`doors.go:24-28`,
`rules.go:375`), plus calibration. USACO division bands enter only learn-mode
trials (`learn/topics.go:211`). So there are two separate axes:

```
curriculum  which topic          capabilities gate topic biomes (capability.go:10)
challenge   how far above you    biome TargetClear + prestige offset + calibration
```

There are no per-tier hard thresholds. The "recommender for each level and
each prestige" is one algorithm with a different target band and modifier
set per biome and prestige. Keep the two axes separate in the ML design too.

#### Findings

| Finding | Source | Domain | Grade |
|---|---|---|---|
| Performance during practice and durable learning dissociate. Conditions that slow acquisition (spacing, interleaving, testing, variation) can improve retention and transfer, but a difficulty is undesirable when the learner lacks the background to respond to it | Bjork 1994; Bjork & Bjork 2011; Soderstrom & Bjork 2015 | `[verbal]` `[motor]` | strong for the dissociation; "desirable" is often assigned after the fact |
| Zone of proximal development: learning happens in what the learner can do with help but not alone | Vygotsky 1978 | theory | no quantitative content; never cite it for a number |
| Challenge point: the best difficulty depends on skill level. No numeric optimum; a 2025 scoping review of 100 citing papers finds little applied testing | Guadagnoli & Lee 2004; Thomas et al. 2025 | `[motor]` theory | weak for CP |
| "85% rule": for a gradient-descent learner on binary classification with Gaussian noise, training accuracy ≈ 84% maximizes the learning rate; 82% for Laplacian noise, 75% for Cauchy. Shown in a perceptron, a small MNIST network and a model of monkey motion learning; no humans. The authors restrict it to binary tasks and call it a first step | Wilson et al. 2019 | `[sim]` | speculative for CP doors |
| Information about ability per item peaks at p = 0.5 | standard IRT (e.g. Barrada et al. 2009) | math | strong (a theorem) |
| Challenge–skill balance correlates "moderately" with flow across 28 studies, more weakly in work and education settings | Fong et al. 2015 | mixed | moderate, correlational |
| Flow's inverted U appears for a low-stakes game (Pac-Man, quadratic β −.54 and −.68) but not before a high-stakes exam, where flow stays high when demand is too low and drops when too high; moderated by achievement motive | Engeser & Rheinberg 2008 | `[games]`, students | moderate |
| Easier was always more engaging, up to the easiest condition tested (63% success): +34% trials, +18% time; no turning point in 69,642 further sessions. Easier versions taught more slowly | Lomas et al. 2013 | `[games]`, grades 4–8 | strong for engagement below 70%; silent above it |
| Moderate difficulty motivates most when players choose it; when difficulty is assigned, the easiest is most motivating. Novelty and suspense also matter | Lomas et al. 2017 | `[games]` | moderate |
| Randomized targets of 95/80/65/50% success, ~37,000 learners: still playing after 10 answers 89.2 / 87.0 / 84.0 / 81.2%; returning later 15.2 / 16.0 / 16.6 / 16.8%; learning lowest at 95%, little difference among the rest | Papoušek, Stanislav & Pelánek 2016 | `[tutor]` geography facts | strong for facts |
| Higher preset success → more problems attempted → larger gains; the effect runs through amount of practice | Jansen et al. 2013 (N = 207) | `[math]`, children | moderate |
| Letting children choose their success rate gave no benefit | Jansen et al. 2016 | `[math]` | moderate |
| Math Garden serves items at mean success .75 by design | Klinkenberg et al. 2011 | `[tutor]` | a design choice, not evidence |
| Learners pick items just beyond what they know ("region of proximal learning"); with more time they pick harder ones; learning is best when time goes to medium items | Metcalfe 2002; Metcalfe & Kornell 2003 | `[verbal]` | moderate |
| Dopamine: the cue response rises with reward probability; a slower ramp tracks uncertainty and peaks at p = 0.5 | Fiorillo, Tobler & Schultz 2003; ramp contested by Niv, Duff & Dayan 2005 | `[animal]` | moderate as physiology; speculative as game design |
| 7-day churn by last three results in a 1v1 game: LLL 5.1%, WWL 4.9%, WWW 3.7%, and 2.6–2.7% after LLW, DLW, LDW or DDD. Loss streaks churn most, and a pure win streak churns more than streaks ending in a comeback | Chen et al. 2017 (EOMM; 36.9M matches) | `[games]` | strong as description; the EOMM gain itself was only simulated |
| Difficulty adjustment in a match-3 game raised rounds played by 4.4–7.9% | Xue et al. 2017 | `[games]` | moderate |

#### Learning, measurement and motivation want different numbers

| Purpose | Best clear probability | Basis | Grade |
|---|---|---|---|
| Measuring the player | 0.5 | Fisher information | strong |
| Learning per door | about 0.65–0.85; avoid ≥ 0.95 | Wilson 2019 (models); Papoušek 2016, Lomas 2013 (humans) | weak–moderate |
| Staying in the session | higher is better, from 0.63 up to at least 0.95 | Lomas 2013; Papoušek 2016 | strong |
| Coming back another day | slightly harder is slightly better; streaks of either kind hurt; variety helps | Papoušek 2016; Chen 2017 | moderate |
| Suspense of one high-stakes moment | about 0.5 | Fiorillo 2003; Lomas 2017 | speculative |

#### What this means for katharsis

The current shape is defensible. Each biome serves a different purpose, and
the whole boot lands in the band:

| Biome | Target | Information vs 50% | Role it plays | Verdict |
|---|---|---|---|---|
| Userland | 0.92 | 0.29 | momentum, warm-up, fluency | keep at 0.88–0.92; do not go higher (Papoušek's 95% arm learned least and returned least) |
| mid | 0.70 | 0.84 | where learning happens | keep, or move to 0.72–0.78 |
| boss | 0.50 | 1.00 | measurement and suspense | keep |
| whole boot | 0.79 per door | | | in the 0.65–0.85 band |

The problem is prestige. Before ascension modifiers, the mid biome drops to
0.67 / 0.64 / 0.60 / 0.57 at prestige 1–4 and the boss to 0.46 / 0.43 / 0.39 /
0.36. The modifiers (blind, mystery, cursed, a duck less, timer × 0.9, a life
less; `rules.go:377-380`) push them lower, and by design (D15) the
calibration does not undo it. At prestige 4 the learning biome sits below
every band with data behind it.

Proposals, each testable:

- **P1.1 Spread each room on purpose.** One door near 0.85, one at target,
  one stretch near target − 0.12. The ±100 window already spreads mid doors
  from about 0.57 to 0.80, but at random. A deliberate spread makes the
  choice meaningful (chosen moderate difficulty is the most motivating
  condition in Lomas 2017), and the pick becomes a measurement of how the
  player judges their own level (Metcalfe). *Test:* log the pick; check
  whether harder picks predict clears beyond what θ predicts.
- **P1.2 Fix the content floor** (table in §6). Below a rating of about
  1224, Userland cannot reach 0.92: a player whose true rating is 1000
  crashes in Userland 42% of the time, not 15%. When the target sits below
  the pool minimum,
  buy the probability back with stakes instead: +1 life, a longer timer, or
  a duck per room, until the floor stops binding. Or add content under 800.
  *Test:* Userland crash rate for players under 1200 falls to ≤ 0.2.
- **P1.3 Streak governor.** After two consecutive boots that crash before
  the first merge window, aim the next boot at 0.95 / 0.78 / 0.55, and
  release it once a merge window is reached. Long win streaks also churn
  (EOMM), so after three full clears in a row, offer the next prestige or a
  harder modifier. *Test:* return latency after streaks, governor on vs off
  in alternating weeks (within-player).
- **P1.4 Mid band 0.70–0.78**, with Thompson-sampled variation inside it
  rather than a point target.
- **P1.5 Let prestige add risk more than rating offset.** Keep the mid
  biome's effective clear probability, modifiers included, at 0.6 or above
  at every prestige. Prestige can still raise stakes (cursed, lives, bytes)
  and lower the boss. This is a values call for the user: harder prestige
  vs a learning-friendly mid biome.

### 2. Skill acquisition: what an "internal learn rate" can mean

#### Findings

| Finding | Source | Domain | Grade |
|---|---|---|---|
| Individual practice curves fit an exponential better than a power function in every unaveraged data set (40 data sets, 475 subjects); averaging people produces a power law | Heathcote, Brown & Mewhort 2000; proof in Murre & Chessa 2011 | speeded tasks (response time) | strong for speed; unknown for accuracy on hard problems |
| A delayed exponential (slow, then fast, then slow) fits most paradigms better still | Evans et al. 2018 | response time | moderate |
| The unit of learning is the knowledge component (KC); learning events range from memory and fluency to induction to sense-making | Koedinger, Corbett & Perfetti 2012 (KLI) | framework | theory |
| Additive factors model: `logit p = α_student + Σ_KC (β_j + γ_j · opportunities_j)` | Cen, Koedinger & Junker 2006 | `[tutor]` | strong as a workhorse |
| Performance factors analysis: separate gains for past successes and failures, `m = Σ_j (β_j + γ_j s_j + ρ_j f_j)`; beat knowledge tracing on all 4 datasets (e.g. A′ 0.711 vs 0.687) | Pavlik, Cen & Koedinger 2009 | `[tutor]` | moderate |
| Bayesian knowledge tracing: 4 parameters per skill (prior, learn, guess, slip); making the learning speed student-specific helped prediction more than making the prior student-specific | Corbett & Anderson 1995; Yudelson, Koedinger & Gordon 2013 | `[tutor]` | moderate |
| Across 27 datasets, students start near 65% correct (about 55% to 75%) and gain about 0.1 log-odds, or 2.5 points of accuracy, per practice opportunity. Learning rate varies little between students; starting knowledge varies a lot | Koedinger et al. 2023 | `[tutor]` | moderate: replicated on 15,000 MATHia students (Simpson et al. 2024), but the learning-rate spread grows 118% when the analysis includes practice differently (Lee et al. 2026) |
| Elo in learning systems: under adaptive item choice, proportion correct fails as an estimate while Elo matches joint maximum likelihood; about 10 answers give r = 0.8 for a new learner; uncertainty can be a function `U(n) = a/(1+bn)`; multivariate and learning variants exist | Pelánek 2016 | `[tutor]` | moderate |
| Glicko: between rating periods the variance grows by `ν²t`, so time away widens uncertainty; Elo is the special case with fixed K | Glickman 1999 | ratings | strong as method |
| Deep knowledge tracing reported AUC 0.86 vs 0.67 for BKT, but BKT with forgetting and ability terms, and plain hierarchical IRT, match it | Piech et al. 2015; Khajah, Lindsey & Mozer 2016; Wilson et al. 2016 | `[tutor]` | strong that simple models suffice |
| Far transfer rarely happens; effects shrink as study designs improve | Sala & Gobet 2017; taxonomy in Barnett & Ceci 2002 | mixed | strong (negative) |
| Accuracy and time can be scored together under a deadline: `S = (2x−1)(1−t)`, with `t` the response time as a fraction of the time limit. A fast right answer scores near +1, a fast wrong answer near −1, anything at the deadline 0 | Maris & van der Maas 2012 (formula as restated by Coomans et al. 2016); used in Math Garden (Klinkenberg et al. 2011) | `[tutor]` | moderate |

Units: one log-odds unit is 400/ln 10 = 173.7 points on the CF scale, so
0.1 log-odds per opportunity is about 17 points per practice of one narrow
KC. A katharsis topic such as `dp` spans dozens of KCs, so per-door movement
of a topic rating should be far smaller (an inference).

#### What "learn rate" can mean, and what one player's data can identify

Write the latent state as a level per topic that moves with practice and
with time:

```
after a door:      θ_k ← θ_k + γ · g(outcome, gap, hint, feedback) + noise
after Δt days off: θ_k ~ N(θ_k − decay_k(Δt), σ_k² + ν²·Δt)
```

That splits "learn rate" into four quantities. Only two are measurable for
one player in useful time:

| Quantity | Meaning | Identifiable for one player? | Use |
|---|---|---|---|
| `θ_k` | where you are in topic k | yes: ±92 points after 20 topic doors, ±46 after 80 | aim doors |
| `γ` | gain per door (the "learn rate") | globally, after hundreds of doors; per topic, no | prior, fit globally |
| `g(·)` | which difficulty and outcome teach most | no: needs comparisons between policies | fixed prior (§1) |
| `ν`, `decay_k` | forgetting with time away | yes, over months, from doors after breaks and from regressions | review scheduling, the aim after a break |

The numbers, assuming p(1−p) = 0.18 per door (p ≈ 0.76, near the boot
average of 0.79) and a linear trend in θ:

```
topic doors   SD of level   SD of slope (points per door)
    20            92            15.9
    40            65             5.6
    80            46             2.0
   160            32             0.70
   320            23             0.25
```

A topic sees about a third of a door per boot (`rating.go:47`), so 160 doors
in one topic is roughly 480 boots. If a plausible gain is about 0.5 to 1
point per door (my assumption: a few hundred rating points over a few
hundred problems), one topic's slope is not distinguishable from zero for
years, and two topics' slopes cannot be told apart at all. Comparing two
difficulty policies by how much each one teaches is harder still.

**Pushback on the framing.** A per-player, per-topic learn rate as a fitted
parameter is the wrong target. The population evidence says rates vary
little (Koedinger 2023, contested), and this player's data cannot measure
them. What the data can measure, and what door choice needs, is the level
per topic with an honest uncertainty, plus forgetting. Show the player
their learning as the trend of `θ_k` with its band, not as a parameter.

#### What this means for katharsis

What the engine already has right: `TopicFollow = 1` (`rules.go:335`) makes a
topic rating the overall rating plus what that topic taught, which is the
general-plus-specific structure above. The boosted cold-start K
(`rules.go:363-364`) is a crude version of a large initial uncertainty. Not
rating retreats (`engine.go:374`) is right for the skill estimate.

What is missing: uncertainty, time, transfer between related topics, and
any use of solve time. `TopicRatingMin` (`rules.go`) is a hard switch where
shrinkage would be smooth. K = 40 fixes how fast the estimate tracks; it is
the engine's tracking rate, not the player's learning rate.

- **P2.1 Replace Elo with a hierarchical Glicko-style filter per topic** (M2
  in part (c)). *Test:* one-step-ahead log loss and calibration on the
  replayed history vs the current Elo.
- **P2.2 Fit one shared gain `γ`, or none** (a random walk). Do not fit
  per-topic learning rates.
- **P2.3 Use time.** A fast AC says more than a last-minute one. Either a
  censored lognormal time model or a deadline score in the style of
  `(2x−1)(1−t)`. The existing `HintedScore` and `OvertimeScore` knobs are
  hand-set special cases of a graded score. *Test:* does adding time
  improve one-step-ahead prediction?
- **P2.4 Transfer through correlated topics.** Prior correlations from the
  USACO Guide order and shared tags (prefix sums, two pointers and binary
  search move together). Estimate them from population data if the user
  allows the Codeforces API pull (part (c), Constraints).
- **P2.5 Prestige is not a player state.** The learner model is the same at
  every prestige; prestige and its modifiers are covariates in the
  measurement model.

### 3. Memory and retention

#### Findings

| Finding | Source | Domain | Grade |
|---|---|---|---|
| Spaced beats massed: 47.3% vs 36.7% recall over 271 comparisons; 39.0% vs 17.0% at retention intervals of a month or more. The best gap grows with how long the memory must last | Cepeda et al. 2006 | `[verbal]` | strong |
| Best gap ≈ 20–40% of the retention interval at one week, 5–10% at one year (1 day for 7 days, 11 days for 35, 21 days for 70 and for 350). A gap too long costs far less than a gap too short | Cepeda et al. 2008 (N = 1,354) | `[verbal]` | strong |
| Expanding and equal gaps do about equally well long-term; what matters is not testing too soon the first time | Cepeda 2006 (62.0 vs 58.6%, n.s.); Karpicke & Roediger 2007; Kang et al. 2014 (day 84: .49 vs .46, n.s.) | `[verbal]` | moderate |
| Retrieval beats restudy, g = 0.50; with feedback 0.73, without 0.39. **Without feedback and with initial success ≤ 50%, g = 0.03** | Rowland 2014 | `[verbal]` | strong |
| Practice tests beat restudy and other comparisons | Adesope et al. 2017 | mixed, classroom | strong |
| Relearning to criterion across several sessions (successive relearning): recall 24 days after the exam above 60% vs below 20% | Rawson, Dunlosky & Sciartelli 2013 (numbers from a secondary source; primary paywalled) | `[verbal]`, course | moderate |
| Spacing math practice helped at 4 weeks, not at 1 week; extra massed problems (overlearning) did nothing | Rohrer & Taylor 2006 | `[math]` | moderate |
| Interleaved math practice: 60.7% vs 37.6% on a test 33 days later, d = 0.83, positive for every teacher (preregistered cluster RCT, 787 students) | Rohrer et al. 2020 | `[math]` | strong |
| Interleaving overall g = 0.42; math 0.34; words −0.39 (blocking better). The benefit grows when categories are similar to each other and shrinks when items inside a category vary a lot | Brunmair & Richter 2019 (238 effects) | mixed | strong |
| Interleave similar categories, block dissimilar ones: interleaving highlights differences, blocking highlights what a category shares | Carvalho & Goldstone 2014 | `[cat]` | moderate |
| Blocked practice hides the choice of strategy; interleaving trains choosing it | Rohrer 2012 (review; the author calls the evidence limited) | `[math]` | moderate |
| Learners believe blocking works better even right after their own test shows the opposite (78%) | Kornell & Bjork 2008 | `[cat]` | strong |
| Skill decay grows with time unused (d ≈ 0 right after training, −1.4 after a year); cognitive tasks decay more than physical, accuracy more than speed | Arthur et al. 1998 | skills | moderate |
| Half the accuracy gained in training is lost after about 6.5 months unused (13 months for speed) | Tatel & Ackerman 2025 (457 reports) | skills | strong |
| More than twice as many people found a hidden shortcut after sleep, but only if they had trained on the task first | Wagner et al. 2004 | problem solving | moderate |
| Incubation helps problem solving (d = 0.29), more after a longer preparation period; sleep helps on difficult problems only | Sio & Ormerod 2009; Sio, Monaghan & Ormerod 2013 | problem solving | moderate |
| Practice testing and distributed practice rate highest in utility; interleaving moderate | Dunlosky et al. 2013 | review | strong as a summary |
| Personalized review scheduling: +16.5% over massed and +10.0% over generic spacing, 28 days after the semester | Lindsey et al. 2014 (DASH) | `[verbal]` | moderate |
| Half-life regression: p = 2^(−Δ/h); beat a Leitner baseline on error (0.128 vs 0.235) but barely discriminates (AUC 0.54); +12% daily retention in one A/B test | Settles & Meeder 2016 | `[verbal]` | moderate |
| Programming-specific evidence is thin: one observational CS1 study links spaced, interleaved low-stakes quizzes to higher grades, confounded with another intervention | Moraes et al. 2023 | programming | weak |

Almost everything above is word, fact, category or school-math learning. CP
skill is closer to the math studies (choose a method, then execute it), but
nobody has tested spacing or interleaving on contest problems.

#### What the engine already gets right

```
crash → scheduleRegression(+3 d)            engine.go:473, meta.go:59
  ├─ re-solve from a blank file              app/intents.go:223 upsolveFiles;
  │                                          boots use a fresh boot-N dir (app/runner.go materialize)
  ├─ editorial upsolve never counts          meta.go:34   (reading ≠ recall)
  ├─ stage 1 → +17 d → retire                meta.go:77-95
  └─ crash again → back to stage 0           meta.go:63-69 (successive relearning)
```

A delayed first test, retrieval without the answer in view, and failure
resetting the schedule all match the evidence.

#### What is off, and proposals

- **R1. A test you will fail teaches nothing without feedback.** Rowland's
  moderator says retrieval at ≤ 50% success without feedback is worthless
  (g = 0.03). A regression is a problem the player failed. If they never
  upsolved it, the 3-day re-solve will probably fail too, and in a boot that
  failure is a crash. *Proposal:* offer a regression only after some upsolve
  of it (editorial included); until then, put the upsolve on GRUB (it already
  counts, `meta.go:20-37`). After a failed regression, open the editorial,
  as `:q!` does (`engine.go:501`).
- **R2. Make regressions low-stakes.** Failing a regression door ends the
  boot like any other door, and it is one of three doors, offered once per boot, in biome 1 room
  0 only (`doors.go:208`). Players will avoid it. Testing works without
  stakes. *Proposal:* a failed regression costs a life, not the boot; or
  regressions become "cron jobs" on GRUB that pay bytes. *Test:* log offered
  vs taken; the take rate is the avoidance measure.
- **R3. Lengthen the schedule.** 3 → 17 → retired covers about three
  weeks. Skill half-lives run to months (Tatel & Ackerman 2025), and for a
  one-year horizon the best gap is about 3–4 weeks (Cepeda 2008). Erring long
  is cheap. *Proposal:* 3, 10, 30, 90 days, or an adaptive half-life model
  (M3) with a target recall of 0.85. Expanding vs equal does not matter
  much; the first gap and the number of reviews do.
- **R4. Test the technique, not the memory of one problem.** Re-solving the
  same problem can be recall of that solution. The math studies used new
  problems of the same kind. *Proposal:* stage 1 is the same problem (did
  the fix stick?); later stages are a sibling problem with the same topic,
  overlapping tags, rating within ±100, never seen. *Test:* sibling clears vs
  the model's prediction; if they match same-problem clears, the technique
  transferred.
- **R5. Schedule more than crashes.** Only `crash` schedules a regression
  (`engine.go:473`). Hinted clears, overtime clears, clears with two or more
  failed submits, and retreats all mark weak learning, and none of them is
  ever reviewed. *Proposal:* one sibling review for each of these.
- **R6. Practice choosing the technique.** In a boot, Userland and the boss
  are mixed, but the mid biome is two rooms of one named topic with tags
  showing on three doors in four (`BlindChance` 0.25, `rules.go:317`):
  blocked practice with the strategy given away. Choosing the technique is
  the CP skill that matches what interleaving trains in math, and CP topics
  look alike on the surface (graph or DP or greedy?), which is the condition
  under which interleaving helps most (Brunmair & Richter). *Proposals:*
  (a) blind by mastery: the chance of hiding tags rises with the posterior
  confidence that the player is above the door, so new topics keep tags
  (respects D5) and known ones lose them; (b) one of the three mid-biome
  doors comes from another unlocked topic, tags hidden; (c) score blind
  clears higher in the model, since they include the identification step.
  *Test:* blind-door clear rate relative to tagged doors, by topic, over
  time.
- **R7. Trials: block, then really interleave.** The unit is blocked with
  tags and the integration doors are blind (`doors.go:254`); blocking first
  is supported (Carvalho & Goldstone; the Rohrer 2020 authors suggest some
  initial blocked practice). But both integration doors come from the same
  topic (`trial.go:127`), so the player still knows the technique.
  *Proposal:* one integration door from the module's topic and one blind
  door from a previously passed topic, without saying which is which; then
  seed the new topic, blind, into the next few boots' Userland doors (spaced
  and interleaved).
- **R8. Expect a preference for blocking.** The player will prefer topic
  biomes and will feel they learn more there (Kornell & Bjork). Route choice
  is good for autonomy; just do not read the preference as evidence of
  learning.
- **R9. Time away.** Nothing in the rating knows about time
  (`rating.go:51-80`). After a three-week break the engine aims at the old
  player. *Proposal:* uncertainty that grows per day away (M2), and the first
  doors after a break aimed at a conservative quantile (θ − 1 SD). *Test:*
  predicted vs observed clears on the first three doors after breaks of a
  week or more.
- **R10. Sleep on it.** A crash after a long attempt is a long preparation
  phase, which is when incubation helps most. *Proposal:* after a timer or
  lives crash, offer "upsolve now" (editorial available) or "sleep on it"
  (editorial locked until tomorrow; a bonus on the recovered core dump for a
  solve without it). The mapping is speculative; it is cheap to try.
  *Test:* within-player, rate of independent upsolves under each option.

### 4. Motivation and reward

#### Findings

| Finding | Source | Domain | Grade |
|---|---|---|---|
| Dopamine neurons signal reward prediction error | Schultz, Dayan & Montague 1997; Berridge 2007 argues dopamine mostly drives "wanting", not learning | `[animal]` | strong for the signal; its role is contested |
| A sustained dopamine ramp tracks reward uncertainty and peaks at p = 0.5 | Fiorillo, Tobler & Schultz 2003; contested by Niv, Duff & Dayan 2005 | `[animal]` | moderate |
| Curiosity before an answer improves recall of it: 70.6% vs 54.1% immediately, 45.9% vs 28.1% a day later; the spillover to unrelated material is small and only partly significant | Gruber, Gelman & Ranganath 2014 | `[verbal]` trivia | moderate |
| People choose activities by learning progress as well as success rate; those who followed progress mastered more (≥2 activities: 90.5% vs 70.6%) | Ten et al. 2021 (382 participants); theory in Oudeyer et al. 2016, Gottlieb & Oudeyer 2018 | lab task | moderate, correlational |
| In games, perceived competence predicted choosing to keep playing (β = .41); enjoyment in an MMO: autonomy β = .49, competence .24, relatedness .12 | Ryan, Rigby & Przybylski 2006 | `[games]` | moderate, correlational; difficulty never manipulated |
| Tangible rewards tied to doing, finishing or performing a task reduce later free-choice engagement (d = −0.40, −0.36, −0.28); positive verbal feedback raises it (+0.33) | Deci, Koestner & Ryan 1999 (128 studies); limited to expected tangible rewards on interesting tasks per Cameron, Banko & Pierce 2001 | mixed | moderate |
| Intrinsic motivation predicts quality of performance (ρ = .21–.45), incentives predict quantity | Cerasoli, Nicklin & Ford 2014 (N = 212,468) | school, work, sport | strong |
| Serious games beat conventional instruction on learning (d = 0.29) and retention (0.36) but are **not more motivating** (0.26, n.s.); gains are larger with several sessions and added instruction | Wouters et al. 2013 | `[games]` | strong |
| Gamification: cognitive g = .49, motivational .36, behavioral .25; only the cognitive effect survives in the rigorous studies | Sailer & Homner 2020 | `[games]` | moderate |
| Near misses under the player's own control are less pleasant than clear misses but raise the urge to keep playing, and recruit win-related striatum | Clark et al. 2009 | gambling | moderate |
| Loss aversion: mean λ = 1.96 across 607 estimates. Contested as a general law; lower with experience and domain knowledge | Brown et al. 2024; Gal & Rucker 2018; Mrkva et al. 2020 | economics | moderate |
| Pressure hurt high-working-memory people on the most demanding math problems; other studies find the reverse pattern in children; different kinds of pressure hurt different skills | Beilock & Carr 2005; Wang & Shah 2014; DeCaro et al. 2011 | `[math]` | weak–moderate |
| Mild acute stress the subject cannot control impairs prefrontal function; controllable stress did not | Arnsten 2009 (review) | `[animal]`, some human | moderate |
| Stress around learning can strengthen encoding but impairs retrieval and pushes behavior toward habit | Vogel & Schwabe 2016 (review) | human | moderate |
| On complex tasks a specific learning goal beat a specific performance goal | Seijts et al. 2004; resource account of Kanfer & Ackerman 1989 not supported by DeShon et al. 1996 | `[org]` | weak–moderate |
| How difficulty affects retention depends on what failure is blamed on (the player's skill vs the avatar's strength) | Allart et al. 2017 (Ubisoft data; abstract only) | `[games]` | weak |
| Growth-mindset interventions: weak overall; +0.10 GPA for lower achievers in a national trial; d = 0.05 after bias correction in a 2023 meta-analysis. Praise-for-intelligence effects did not replicate | Sisk et al. 2018; Yeager et al. 2019; Macnamara & Burgoyne 2023; Li & Bates 2019 | school | weak |
| Habits took 18 to 254 days to become automatic; missing a day did not matter | Lally et al. 2010 | health behaviors | moderate |
| Games are the art of failing on purpose ("the paradox of failure") | Juul 2013 | theory | theory |

#### What the engine already gets right

- **Autonomy**, the strongest predictor of enjoyment above: route choice,
  door choice, prestige, presets, upgrade and daemon picks.
- **The core dump points loss aversion at the learning act.** Bytes lost in
  a crash come back only by upsolving the problem that caused it
  (`meta.go:20-37`). An engagement mechanic that pays for exactly the
  behavior that teaches is rare; keep it.
- **No incentive to farm easy doors.** `doorBytes` scales pay by
  0.5/SolveP so every door's expected payout is the same (`doors.go`
  `doorBytes`). Rewards do not pull the player toward the 95% zone.
- **Failure is attributed to a specific, fixable cause.** The crash screen
  names the cause and matches a quote to it. Allart's finding (weak) suggests
  that matters for persistence.
- **Learning goals where learning happens.** Trials run on a soft timer with
  failing input shown (`rules.go:407-408`); boots are the performance setting.

#### Risks and proposals

- **M1. Failure streaks drive quitting** (EOMM). The streak governor is P1.3.
- **M2. Bytes buy nothing.** `Meta.Bank` is only ever added to
  (`engine.go` `bank`, `meta.go:31`); nothing spends it. Commit is free, so a
  crash only costs the current biome's bytes. Loss aversion over a number
  with no use is weak, and the meta-progression that keeps a roguelite
  coming back has no currency. *Proposal:* give bytes a sink tied to
  learning acts (unlocking trials, libs, an extra duck, a door re-roll), so
  the currency points toward learning. *Test:* commit and upsolve rates
  before and after.
- **M3. Keep bytes informational.** Performance-contingent tangible rewards
  carry a small undermining risk (d ≈ −0.3). Show what a reward means ("dp
  +12, regression due Oct 17") next to the number, so it reads as feedback.
  Speculative but cheap.
- **M4. The crash screen should show what persisted.** A crash costs one to
  two hours. Show what survived it: topic change, the failing test now
  visible (`State.ShowsFailingInput` is already true after a crash), the
  regression date, the upsolve path. That is competence feedback; skip
  growth-mindset copy (weak evidence).
- **M5. The timer is imposed pressure on a working-memory-heavy task.** It
  is fitted to the player's own 85th-percentile time with headroom
  (`timer.go:45-58`, `rules.go` `TimerQuantile` 0.85, `TimerHeadroom` 1.25),
  and Overclock and `nice` let the player buy time, which keeps it partly
  controllable. Keep that. For the first few doors of a topic after its
  trial, give a longer timer: that is when the task is newest and pressure
  hurts most (inference).
- **M6. Near misses are real progress signals here.** In gambling a near
  miss is an illusion; here 51/52 tests passed is evidence. The HP bar is
  fine; lives already bound submit-spamming. After a crash, show which test
  failed first.
- **M7. Reasons to come back that are themselves learning.** Show due
  reviews on GRUB ("2 cron jobs due"); keep the nightly build as a daily
  cue. Do not add streak-loss penalties: missing a day does not hurt habit
  formation (Lally), and a streak penalty turns loss aversion into guilt
  about the game (speculative).
- **M8. Show learning progress.** Per-topic θ trend with its band in `top`.
  People steer toward activities where they see progress (Ten 2021), so this
  may pull route choices toward learnable topics (speculative).

### 5. Hints, worked examples, and failing first

#### Findings

| Finding | Source | Domain | Grade |
|---|---|---|---|
| Studying worked examples beat solving the same problems for novices: faster, fewer errors, but only on problems of the same structure | Sweller & Cooper 1985 | `[math]` | strong for novices; no transfer |
| Aids that help novices lose their effect or backfire as expertise grows (expertise reversal) | Kalyuga et al. 2003 | review | moderate–strong |
| Subgoal-labeled worked examples improved novices' performance on new app-building tasks | Margulieux, Guzdial & Catrambone 2012 | programming | moderate |
| Problem solving before instruction beats the reverse, g = 0.36 [0.20, 0.51]; 0.37–0.58 when the design follows productive-failure principles; younger learners and domain-general skills favor instruction first. Secondary sources say the benefit is for conceptual knowledge and transfer, not procedure | Sinha & Kapur 2021 (53 studies); Kapur 2008, 2014 | `[math]` science | moderate |
| When element interactivity is high, instruction first won on similar and transfer problems | Ashman, Kalyuga & Sweller 2020 (Year 5, N = 64 and 71) | science, children | moderate |
| Failing first helps only with contrasting cases or instruction built on the learner's own attempt | Loibl, Roll & Rummel 2017 | review | moderate |
| A failed attempt followed by the answer beats studying the answer, but only with immediate feedback | Kornell, Hays & Bjork 2009; Hays, Kornell & Bjork 2013; Grimaldi & Karpicke 2012 | `[verbal]` | moderate |
| Errors help when corrective feedback addresses the reasoning; confident errors are corrected most (hypercorrection) but can return | Metcalfe 2017; Butterfield & Metcalfe 2001; Butler, Fazio & Marsh 2011 | `[verbal]` | moderate |
| Prompting learners to explain (self-explanation): g = 0.55; 0.48 while solving problems, 0.36 while studying worked examples | Bisra et al. 2018 (64 reports) | mixed | strong |
| How much help to give vs withhold is an open problem ("assistance dilemma") | Koedinger & Aleven 2007 | `[tutor]` | theory |
| Learners use on-demand help poorly; feedback on help-seeking improved help-seeking but not learning | Aleven et al. 2003; Aleven et al. 2016 | `[tutor]` | moderate |
| Gaming the system (abusing hints) was the off-task behavior most tied to lower learning | Baker et al. 2004 | `[tutor]` | moderate |
| Proactive hints triggered by a model of when help is needed reduced help avoidance and improved post-tests; delaying hints for gamers did not close their gap | Maniktala et al. 2020; Vanacore et al. 2024 | `[tutor]` logic, math | weak–moderate |

#### Mapping the mechanics

```
door open ──(stuck)──> :Quack 1  tags        (skipped if tags are shown, engine.go quack)
                       :Quack 2  technique
          ──(give up)─> :q!  crash → editorial opens now (engine.go:501)
                                   → upsolve: editorial allowed, counts as Editorial,
                                     never advances a regression (meta.go:34)
          ──(timer/lives)─> crash → failing test shown, upsolve from GRUB
```

What already fits the evidence:

- **The split between trials and boots matches expertise reversal.** A
  trial puts instruction first (read the module, then the unit), which suits
  a novice facing high element interactivity (Ashman 2020). A boot puts the
  attempt first, which suits topics already learned (Sinha & Kapur). Keep
  both.
- **Rationed hints** (one duck, refilled at merge windows) limit hint abuse.
  A hinted clear still pays full bytes but earns 0.7 of the rating credit,
  so the estimate is not inflated by help.
- **Give-up opens the editorial at once.** A failed attempt followed by
  immediate feedback is the best-supported form of failing first.
- **Ducks fade with prestige.** Ascension 2 removes the base duck
  (`rules.go:378`), which matches expertise reversal by accident.

Gaps and proposals:

- **H1. Build the editorial on the attempt.** Failing first works with
  instruction that builds on the learner's own attempt (Loibl). Today the
  editorial is generic. *Proposal:* after `:q!` or a crash, show the
  player's code beside the editorial and ask one line: "the idea I missed
  was ___". That is a self-explanation prompt (g ≈ 0.55), and the stored
  line is a free label for the ML thread (a taxonomy of misses). *Test:*
  later sibling-door clears with vs without the prompt, alternating by week.
- **H2. Add a third duck level: the key observation.** The ladder stops at
  the technique, but CP learners usually stall on the reduction or the key
  observation. A third level, the editorial's first idea, is where help pays
  off for high element interactivity. It could cost two ducks. Content
  allowing: USACO has analyses for all problems, Codeforces for about 60%.
  *Test:* clear rate after each hint level, and later sibling clears.
- **H3. No forced hint delays.** The evidence does not support them
  (Vanacore 2024). Do log the time of each quack: a quack in the first
  minute vs after twenty minutes means different things.
- **H4. "Read, close, write" upsolves.** An upsolve written with the
  editorial open is close to copying. Offer an upsolve mode that closes the
  editorial before the editor opens, so the solve comes from memory
  (retrieval with feedback just before it, Rowland 2014). Speculative.
- **H5. Fade scaffolds by mastery.** Blind-by-mastery (R6) applies the same
  principle to tags.



### 6. Measurement

#### Information per door

A binary outcome at clear probability `p` carries Fisher information
`p(1−p)·(ln10/400)²` about the rating (standard IRT; maximal at `p = 0.5`).
Computed for the standard rules (`rules.go:313-315`), assuming the aim is
right:

| Biome | Target p | Information vs a 50% door |
|---|---|---|
| Userland | 0.92 | 0.29 |
| mid | 0.70 | 0.84 |
| boss | 0.50 | 1.00 |

```
per boot (design aim):   3.77 doors opened, 2.98 cleared, door success 0.79,
                         P(full clear) 0.207, P(crash in Userland) 0.154
rating SD from outcomes alone (stationary player):
  1 boot 235 · 10 boots 74 · 30 boots 43 · 100 boots 23   (points, overall)
```

Per topic it is worse by roughly the square root of the topic's share of
doors. Userland doors buy fun, not measurement.

#### Which signals carry what

| Signal | Carries | Confounded by | Logged now |
|---|---|---|---|
| Clear vs crash (`attempts.outcome`) | level on the topic at that gap | door self-selection, timer, lives, modifiers, retreats | yes |
| Time to AC (`Solve.Elapsed`) | fluency | right-censored at the timer; reading and idle time; the −O3 bonus at half the timer rewards speed (`engine.go` clear) | clears only; deadline in attempt JSON |
| Failed submits (`attempts.failed`) | carelessness or a wrong idea (ambiguous) | lives; cursed doors (one WA crashes, so the player over-tests); `nice` (2 lives per WA); CE is free | count only |
| Best pass ratio (`best_pass/total`) | near miss vs wrong algorithm | test order; TLE on big tests is not WA on small ones | max only |
| Verdict mix | error type (TLE after many passes ≈ right idea, wrong complexity) | | last verdict only |
| Samples pass, then full WA | edge cases, misread constraints | sample strength varies by source | **no** |
| Time to first all-samples pass | time to a working idea | | **no** |
| Ducks (`quacks`, `Hint`) | needed the technique | ducks are scarce, so no quack ≠ no need | count and max level; no times |
| Give-up | the player's own stop decision | fatigue, time of day | outcome and close time |
| Retreat | read the statement, judged it too hard | free with kexec; the rating ignores it (`engine.go:374`) | outcome and close time |
| Door choice (1 of 3) | self-assessed competence per topic | | ratings and modifiers as dmesg text (`doors.go:199`); no problem IDs, SolveP or chosen index |
| Route choice | topic preference and avoidance | | dmesg text |
| Upsolve | persistence; learning from the editorial | | successful upsolves only (`meta.go:20`); no attempts, time, editorial dwell |
| Regression re-solve | retention | memory of that specific problem | derivable (`attempts.regression`, Solves) |
| Return latency | engagement | life | `boots.started`; app launches not logged |
| Per-topic rating over time | the estimate's history | | **no**; only overall `rating_history` |

#### Pitfalls specific to this engine

1. **The speed–accuracy tradeoff is shaped by the rules.** Timers, lives,
   cursed doors, `nice` and the −O3 bonus each move the player along it
   (Heitz 2014 reviews the tradeoff; `[percept]`, strong as a phenomenon). So
   WA counts and solve times mean different things under different modifiers;
   put the modifiers in the model as covariates, never pool them raw.
2. **Censoring.** Solve time is seen only for clears. A timed-out door is
   censored at the deadline; out of lives, give-up and retreat are competing
   events. `fitTimer` already corrects for the censoring with a headroom
   factor (`timer.go:54-58`); a survival model uses all of it instead.
3. **Selection.** The player picks 1 of 3 doors from title and tags (D5),
   retreats, picks routes and can skip a regression door. Outcomes are
   missing not at random. The calibration code already handles one case
   (counting retreats as failures, `calibration.go:57-61`). Log the offered
   set so the model can condition on it.
4. **Item difficulty is noisy, and noise flattens the curve.** USACO ratings
   are division plus position in 150-point steps, adjusted ±150 by guide
   labels (`content/usaco/rating.go:26-50`). If true difficulty scatters
   around the estimate, the observed clear curve is the logistic convolved
   with that scatter: flatter. That is a sufficient explanation for "real
   solve curves are flatter than the logistic" (`game/types.go:105`). Model
   noise per source; do not read the flatness as psychology.
5. **Ratings from different sources measure different things.** A
   Codeforces difficulty is the rating of a participant who "on a typical
   round" solves the problem half the time (Mirzayanov 2018): a two-hour
   contest with several problems, not a 15–50 minute door with lives. USACO
   ratings here are estimated from division and position. AtCoder Problems
   uses a different logistic slope (base 6 per 400 points instead of base 10;
   its estimator fixes discrimination at ln 6/400), so AtCoder difficulties
   are not on the Codeforces scale without rescaling. Fit an offset (and
   possibly a slope) per source rather than letting one calibration offset
   absorb everything.
6. **The content floor.** The pool bottoms out at 800 (Codeforces) and 850
   (USACO Bronze P1 Very Easy). For a Userland target of 0.92 the aim is the
   rating minus 424, so the floor binds below a rating of about 1224:

   | Player rating | Userland aim | P(clear) at an 800 door | P(crash in Userland) |
   |---|---|---|---|
   | 800 | 376 | 0.50 | 0.75 |
   | 900 | 476 | 0.64 | 0.59 |
   | 1000 (cold start) | 576 | 0.76 | 0.42 |
   | 1100 | 676 | 0.85 | 0.28 |
   | 1200 | 776 | 0.91 | 0.17 |

   Under the floor the calibration cannot help: a door more than 100 points
   from its target gets no `Tier` (`doors.go:256-258`) and teaches nothing.
   This lands on exactly the players most likely to quit.
7. **Non-stationarity.** The player is supposed to improve. Elo with fixed K
   is a tracking filter that lags a learner and has no notion of time away.
8. **Hints contaminate outcomes.** A hinted clear scores 0.7
   (`rules.go:341`), a number chosen by hand. Model the hint as a covariate
   whose effect is fitted.
9. **Item exposure.** Hinted and editorial-solved problems can come back as
   ordinary doors (`doors.go:52-60`), and regressions are re-solves.
   Familiarity inflates clears; flag exposure count as a feature.
10. **Fatigue and time of day.** Boots run one to two hours. Log the door's
    position in the boot and the hour.
11. **The player designs the game.** They can see every knob and can play
    around the model. Log the rules hash and model version per boot.

#### What to log now

One append-only table is enough: `events(boot, attempt, at, kind, json)`.

| Kind | Payload |
|---|---|
| `door_offered` | room, problem, rating, topic, SolveP, modifiers, Tier, window tier, candidate count in that window (for propensities) |
| `door_chosen` | index; time spent on the door screen |
| `route_offered`, `route_chosen` | topics, index |
| `save_judged` | which samples pass; compile ok |
| `submit` | verdict, passed, total, index of the first failing test |
| `quack` | level |
| `retreat`, `give_up`, `timer_expired` | |
| `editorial_open`, `editorial_close` | for dwell time |
| `upsolve_open`, `upsolve_judged`, `upsolve_close` | verdict; editorial read or not |
| `app_start`, `app_quit` | session boundaries |
| `boot_snapshot` | TopicRating, TopicAttempts, Calibration, model version, rules hash |
| `self_report` (optional) | one key after a crash or clear: easy / right / hard |

The self-report is optional and should stay at one prompt per boot; it is
cheap ground truth for the engagement model, and comparing it with outcomes
measures how well the player judges their own level.

## (c) Brief for the ML thread

### Decision points the pipeline controls

```
rollRoom (doors.go:160)
 ├─ candidates/pick (doors.go:70,101)   which 3 problems: topic × rating window
 ├─ makeDoor (doors.go:235)             modifiers, SolveP, timer, bytes
 └─ regressionDoor (doors.go:206)       which due review, where it appears
routeOffers (doors.go:295)              which 2 topics a choice biome offers
planTrial/trialDraw (trial.go:74,127)   unit top-ups and integration doors
fitTimer (timer.go:59)                  timers (leave mostly alone)
rate/calibrate (rating.go:51, calibration.go:68)   the state the above read
```

### Models the evidence supports

**M1. Measurement model, one row per closed attempt.** Logistic IRT with
context terms, on the CF scale:

```
logit P(clear) = a·(θ_topic(t) − b_j)/173.7 + β_hint[h] − β_blind·blind
                 + β_time·log(T_door / T_typical(gap)) − β_src[source] + ...
b_j = rating_j + ε_j,   ε_j ~ N(0, σ_src²)      (σ_usaco > σ_cf)
```

`h` is the highest duck level used; `T_typical(gap)` is the player's typical
solve time at that gap (what `fitTimer` already estimates).

`a` (discrimination) is fitted globally; it replaces the shape correction the
calibration does today. The item noise term is the honest version of
"real solve curves are flatter than the logistic" (`game/types.go:105`).
Optional joint time model in the style of van der Linden (2007):
`log T_AC ~ N(μ0 − ρ·(θ − b_j), τ²)`, right-censored at the deadline, so
timed-out doors still inform θ. A competing-risks survival model over {AC,
out of lives, timer, give up, retreat} is the full version; start with the
logistic plus a censored lognormal.

**M2. State model.** `θ_k(t) = θ_g(t) + δ_k(t)` (general plus topic, which is
what `TopicFollow = 1` approximates today, `rules.go:335`). Between doors:

- variance grows with elapsed days (Glicko's rating deviation; Glickman
  1999), so a player back from a break is uncertain, not assumed unchanged;
- drift per practice opportunity `γ` shared by all topics (a prior, fitted
  globally; see §2 for why not per topic);
- prior correlation between topics from the USACO Guide order (transfer is
  mostly near, between topics that share components);
- optional decay of the mean with time since the topic was last practiced
  (half-life per topic, prior from skill-decay data; §3).

Inference: an online assumed-density filter (Glicko-2 or TrueSkill-style
moment matching, or an extended Kalman filter on the logistic). It is closed
form, deterministic, cheap, and fits in Go. Offline, refit the
hyperparameters (`a`, the `β`s, `σ_src`, process noise, `γ`, half-lives) by
maximum marginal likelihood, replaying the filter over the whole store. The
store keeps every attempt, so replay is exact.

**M3. Review scheduler**, per problem (regressions) and per topic. Half-life
regression (Settles & Meeder 2016) or FSRS's difficulty/stability/
retrievability model (Ye et al. 2022): predicted recall falls with time over
a stability that grows with each successful review; schedule when predicted
recall drops below a target (0.8–0.9). Both were fitted on flashcard facts
`[verbal]`; use their shape, not their parameters, and fit one or two
parameters (initial stability, growth per success) on this player's re-solve
outcomes.

**M4. Door and route policy: constrained choice, not RL on learning gain.**

```
for each room:
  band   = prior clear-probability band for the biome (§1 table)
  cands  = problems whose P(clear | posterior θ sample) falls in band
  score  = w_L·G(p) + w_R·review_due(topic) + w_I·info(topic) + w_N·novelty
  doors  = 3 draws by score, spread across the band (one easier, one at
           target, one stretch), sampled with State.RNG
route offers: learning-progress bandit (ZPDES, Clement et al. 2015) over
  unlocked topics, plus any topic due for review; the player picks
guardrails (override everything): streak governor, return-from-break aim
```

`G(p)`, the learning value of a door cleared with probability `p`, is a fixed
prior curve: an inverted U peaking around 0.7–0.8 when feedback follows a
failure. That shape is speculative, anchored only on Papoušek 2016 (facts)
and Wilson 2019 (simulation). It is not learned online: §2 shows it cannot be measured from one
player in any useful time. Thompson sampling from the posterior over θ gives
door variety and keeps estimates from collapsing.

**M5. Engagement model.** Probability of starting another boot within 48 h,
as a function of the last boots (crash biome, cause, streak, bytes lost,
uptime, hour of day). A discrete-time hazard model. About one observation
per boot, so it sets guardrail thresholds; it does not steer doors directly.

**Do not build:** deep knowledge tracing (too little data; simple models
match it on tutor data, Khajah et al. 2016); free per-topic learning-rate
parameters; bandits or RL whose reward is measured learning gain.

### The objective

What the product needs is durable skill gained per week, which is
learning per hour × hours played per week. For this player the second factor
is the one at risk (they quit long-term practice, they play roguelikes for
hours), so engagement is a constraint that protects the larger term, not a
competing goal.

```
maximize   E[Δθ̄ over 30 days]   measured on first attempts at fresh, unhinted,
                                 non-regression doors
subject to P(crash before the first merge window) ≤ 0.20   (design: 0.15)
           crash streak before the governor eases ≤ 2 boots
           door success over a boot in 0.75–0.85            (design: 0.79)
           P(full clear) in 0.15–0.30 at the top prestige   (design: 0.21)
```

Lexicographic: guardrails first, then the learning terms with weights fixed
from priors. A learned weighted sum of learning and engagement is not on the
table, because neither term can be estimated per policy from one player in
months (§2).

### Features and labels the store already has

| Where (`internal/store/migrate.go:16-71`) | Fields | Use |
|---|---|---|
| `attempts` columns | boot, idx, problem, topic, rating, blind, mystery, cursed, regression, boss, opened, closed, outcome, submits, failed, quacks, best_pass, total | the measurement rows |
| `attempts.json` (`game.Attempt`) | Door.SolveP, Timer, Bytes, Tier, Deadline, Hint, O3, Overtime, LastVerdict | engine's prediction (baseline), censoring point, hint level |
| `boots` columns | seed, prestige, rules, started, ended, outcome, cause, uptime, committed | session and engagement rows |
| `boots.state` (`game.State`) | Rules snapshot, Daemons, Upgrades, Log (dmesg) | context; dmesg holds offered door ratings and route offers as text (`doors.go:199`, `engine.go` route logs) |
| `meta.json` (`game.Meta`) | Rating, TopicRating, TopicAttempts, Calibration, Solves (kind, elapsed, gap, upsolve, trial, at), Regressions, CoreDump, TrialsPassed | current state; Solves give upsolve and re-solve labels |
| `rating_history` | overall rating over time | the only time series of the estimate |

Labels: clear (binary); time to AC (censored at the deadline); hint use;
give-up; regression re-solve success by stage (retention); return latency
(engagement). Gaps are listed in §6.

### Constraints

- **One player, small data.** At the design aim a boot opens about 3.8 doors
  and clears 3.0 (§6); a topic sees about a third of a door per
  boot (`rating.go:47`). At one boot every few days that is a few hundred
  doors a year, spread over 15 topics.
- **Cold start.** Everyone starts at 1000 (`rules.go:331`) with K 100 for 20
  attempts (`rules.go:363-364`); calibration waits for 60 rated attempts
  (`rules.go:368`). A population prior would help. One option: the public
  Codeforces API (`user.status`) gives rating, tags, verdict and time for
  every submission of a sample of users, which is enough to fit population
  learning and forgetting curves by tag offline. That is modeling data, not
  platform integration, but it touches D7; the user's call.
- **Determinism.** The engine is pure and seeded (`game/types.go:1-10`). A
  model update must be a pure function of (Meta, closed attempt), and
  exploration must draw from `State.RNG` with a fixed number of draws per
  door, as `makeDoor` already does so one door's roll never shifts the next
  (`doors.go:227-248`). Fitted parameters are settings, so they are locked per
  boot (D2): snapshot them into the boot (in Rules, or a versioned
  `Meta.Model`).
- **No Python** (user preference). The online filter and the offline fit are
  small (tens to hundreds of parameters); Go plus SQL is enough.
- **Explainable.** The player is also the designer. Each door should be able
  to say why it was offered ("review: dp, last seen 19 days ago").

### Validation

1. **Learner simulator.** `sim.Player` has a fixed true rating today
   (`game/sim/sim.go:28-29`), so the sim can balance a preset but cannot rank
   policies by learning. Add per-topic true skill that grows per opportunity,
   decays with time off, shifts with hints, and a quit model (the chance of
   not starting the next boot rises with crash streaks). Policy comparisons
   belong here, where thousands of boots are cheap.
2. **One-step-ahead prediction on replayed real data.** Log loss,
   calibration, AUC of P(clear), new model vs the current Elo. This is the
   one real-data test with power.
3. **Off-policy evaluation** of door policies needs logged propensities (the
   replay method of Li et al. 2011). `pick` samples uniformly within a
   rating window (`doors.go:101-140`), so propensities are computable if the
   candidate count per window is logged.

### Open scientific questions

No study answers these for CP. The simulator can explore them first; the
game's own logs can answer some of them slowly.

1. Does door difficulty change how much a player learns, beyond changing
   how long they play? For facts, Papoušek found little difference between
   50% and 80%.
2. Is re-solving the same problem days later a test of the technique, or
   of memory for that one solution? (Same-problem vs sibling re-solves, R4.)
3. Which CP topics transfer to which? Population data could estimate the
   correlation structure.
4. Does the per-door timer reduce what is learned from a door, or only the
   clear rate?
5. Does interleaving improve technique identification in CP as it does in
   math? (Blind vs tagged clear trends, R6.)
6. How fast do CP skills decay with disuse? The half-life of θ per topic.
7. What review gaps suit problem-solving methods, as opposed to facts?
8. Does sleeping on a failed problem raise independent upsolves? (R10.)
9. How much does choosing 1 of 3 doors bias the skill estimate, and can the
   choice itself measure self-assessment?
10. Do near-miss HP bars produce productive persistence, or just more
    submits?

## (d) References

Grades are given where each source is used above. "Abstract" marks sources
checked at abstract level only; "secondary" marks numbers taken from a
secondary source.

- Adesope, Trevisan & Sundararajan (2017). Rethinking the use of tests: a meta-analysis of practice testing. *Review of Educational Research* 87(3):659–701. https://doi.org/10.3102/0034654316689306 (abstract)
- Aleven, Stahl, Schworm, Fischer & Wallace (2003). Help seeking and help design in interactive learning environments. *Review of Educational Research* 73(3):277–320. https://doi.org/10.3102/00346543073003277 (abstract)
- Aleven, Roll, McLaren & Koedinger (2016). Help helps, but only so much: research on help seeking with intelligent tutoring systems. *IJAIED*. https://doi.org/10.1007/s40593-015-0089-1 (abstract)
- Allart, Levieux, Pierfitte, Guilloux & Natkin (2017). Difficulty influence on motivation over time in video games using survival analysis. *FDG '17*. https://doi.org/10.1145/3102071.3102085 (abstract)
- Arnsten (2009). Stress signalling pathways that impair prefrontal cortex structure and function. *Nature Reviews Neuroscience* 10(6):410–422. https://doi.org/10.1038/nrn2648
- Arthur, Bennett, Stanush & McNelly (1998). Factors that influence skill decay and retention: a quantitative review and analysis. *Human Performance* 11(1):57–101. https://www.gwern.net/doc/psychology/spaced-repetition/1998-arthur.pdf
- Ashman, Kalyuga & Sweller (2020). Problem-solving or explicit instruction: which should go first when element interactivity is high? *Educational Psychology Review* 32(1):229–247. https://doi.org/10.1007/s10648-019-09500-5 (abstract)
- Baker, Corbett, Koedinger & Wagner (2004). Off-task behavior in the cognitive tutor classroom: when students "game the system". *CHI 2004*:383–390. https://doi.org/10.1145/985692.985741 (abstract)
- Barnett & Ceci (2002). When and where do we apply what we learn? A taxonomy for far transfer. *Psychological Bulletin* 128(4):612–637. https://doi.org/10.1037/0033-2909.128.4.612 (secondary)
- Barrada, Olea, Ponsoda & Abad (2009). Item selection rules in computerized adaptive testing. *Methodology* 5(1):7–17. https://doi.org/10.1027/1614-2241.5.1.7 (for Fisher information peaking at p = 0.5)
- Beilock & Carr (2005). When high-powered people fail: working memory and "choking under pressure" in math. *Psychological Science* 16(2):101–105. https://doi.org/10.1111/j.0956-7976.2005.00789.x (abstract)
- Berridge (2007). The debate over dopamine's role in reward: the case for incentive salience. *Psychopharmacology* 191:391–431. https://doi.org/10.1007/s00213-006-0578-x (abstract)
- Bisra, Liu, Nesbit, Salimi & Winne (2018). Inducing self-explanation: a meta-analysis. *Educational Psychology Review* 30:703–725. https://summit.sfu.ca/item/34322 (subgroup values secondary)
- Bjork (1994). Memory and metamemory considerations in the training of human beings. In Metcalfe & Shimamura (Eds.), *Metacognition*, 185–205. MIT Press. https://www.gwern.net/doc/psychology/spaced-repetition/1994-bjork.pdf
- Bjork & Bjork (2011). Making things hard on yourself, but in a good way: creating desirable difficulties to enhance learning. In *Psychology and the Real World*, 56–64. Worth. https://bjorklab.psych.ucla.edu/wp-content/uploads/sites/13/2016/04/EBjork_RBjork_2011.pdf
- Brown, Imai, Vieider & Camerer (2024). Meta-analysis of empirical estimates of loss aversion. *Journal of Economic Literature* 62(2):485–516. https://doi.org/10.1257/jel.20221698 (abstract)
- Brunmair & Richter (2019). Similarity matters: a meta-analysis of interleaved learning and its moderators. *Psychological Bulletin* 145(11):1029–1052. https://doi.org/10.1037/bul0000209
- Butler, Fazio & Marsh (2011). The hypercorrection effect persists over a week, but high-confidence errors return. *Psychonomic Bulletin & Review*. https://doi.org/10.3758/s13423-011-0173-y (abstract)
- Butterfield & Metcalfe (2001). Errors committed with high confidence are hypercorrected. *JEP: LMC* 27(6):1491–1494. https://doi.org/10.1037/0278-7393.27.6.1491 (abstract)
- Cameron, Banko & Pierce (2001). Pervasive negative effects of rewards on intrinsic motivation: the myth continues. *The Behavior Analyst* 24:1–44. https://doi.org/10.1007/BF03392017 (abstract)
- Carvalho & Goldstone (2014). Putting category learning in order: category structure and temporal arrangement affect the benefit of interleaved over blocked study. *Memory & Cognition* 42(3):481–495. https://doi.org/10.3758/s13421-013-0371-0 (abstract)
- Cen, Koedinger & Junker (2006). Learning factors analysis: a general method for cognitive model evaluation and improvement. *ITS 2006*, LNCS 4053:164–175. https://doi.org/10.1007/11774303_17 (equation from the 2005 precursor)
- Cepeda, Pashler, Vul, Wixted & Rohrer (2006). Distributed practice in verbal recall tasks: a review and quantitative synthesis. *Psychological Bulletin* 132(3):354–380. https://pubmed.ncbi.nlm.nih.gov/16719566/
- Cepeda, Vul, Rohrer, Wixted & Pashler (2008). Spacing effects in learning: a temporal ridgeline of optimal retention. *Psychological Science* 19(11):1095–1102. https://doi.org/10.1111/j.1467-9280.2008.02209.x
- Cerasoli, Nicklin & Ford (2014). Intrinsic motivation and extrinsic incentives jointly predict performance: a 40-year meta-analysis. *Psychological Bulletin* 140(4):980–1008. https://doi.org/10.1037/a0035661 (abstract)
- Chen, Xue, Kolen, Aghdaie, Zaman, Sun & Seif El-Nasr (2017). EOMM: an engagement optimized matchmaking framework. *WWW 2017*:1143–1150. https://doi.org/10.1145/3038912.3052559
- Clark, Lawrence, Astley-Jones & Gray (2009). Gambling near-misses enhance motivation to gamble and recruit win-related brain circuitry. *Neuron* 61(3):481–490. https://doi.org/10.1016/j.neuron.2008.12.031
- Clement, Roy, Oudeyer & Lopes (2015). Multi-armed bandits for intelligent tutoring systems. *Journal of Educational Data Mining* 7(2):20–48. https://arxiv.org/abs/1310.3174
- Coomans, Hofman, Brinkhuis, van der Maas & Maris (2016). Distinguishing fast and slow processes in accuracy–response time data. *PLoS ONE*. https://pmc.ncbi.nlm.nih.gov/articles/PMC4864306/ (source of the restated scoring rule)
- Corbett & Anderson (1995; often cited as 1994). Knowledge tracing: modeling the acquisition of procedural knowledge. *User Modeling and User-Adapted Interaction* 4(4):253–278. https://doi.org/10.1007/BF01099821 (model as restated by later papers)
- DeCaro, Thomas, Albert & Beilock (2011). Choking under pressure: multiple routes to skill failure. *JEP: General* 140(3):390–406. https://doi.org/10.1037/a0023466 (abstract)
- DeShon, Brown & Greenis (1996). Does self-regulation require cognitive resources? Evaluation of resource allocation models of goal setting. *Journal of Applied Psychology* 81(5):595–608. https://iro.uiowa.edu/esploro/outputs/journalArticle/Does-Self-Regulation-Require-Cognitive-Resources-Evaluation/9984371101602771 (abstract)
- Deci, Koestner & Ryan (1999). A meta-analytic review of experiments examining the effects of extrinsic rewards on intrinsic motivation. *Psychological Bulletin* 125(6):627–668. https://doi.org/10.1037/0033-2909.125.6.627 (abstract)
- Dunlosky, Rawson, Marsh, Nathan & Willingham (2013). Improving students' learning with effective learning techniques. *Psychological Science in the Public Interest* 14(1):4–58. https://doi.org/10.1177/1529100612453266 (abstract)
- Engeser & Rheinberg (2008). Flow, performance and moderators of challenge-skill balance. *Motivation and Emotion*. https://doi.org/10.1007/s11031-008-9102-4
- Evans, Brown, Mewhort & Heathcote (2018). Refining the law of practice. *Psychological Review* 125(4):592–605. https://doi.org/10.1037/rev0000105 (abstract)
- Fiorillo, Tobler & Schultz (2003). Discrete coding of reward probability and uncertainty by dopamine neurons. *Science* 299:1898–1902. https://doi.org/10.1126/science.1077349 (abstract)
- Fong, Zaleski & Leach (2015). The challenge–skill balance and antecedents of flow: a meta-analytic investigation. *Journal of Positive Psychology* 10(5):425–446. https://doi.org/10.1080/17439760.2014.967799 (abstract; no r reported there)
- Gal & Rucker (2018). The loss of loss aversion: will it loom larger than its gain? *Journal of Consumer Psychology* 28(3):497–516. https://doi.org/10.1002/jcpy.1047 (abstract)
- Glickman (1999). Parameter estimation in large dynamic paired comparison experiments. *Applied Statistics* 48(3):377–394. https://www.glicko.net/research/glicko.pdf
- Gottlieb & Oudeyer (2018). Towards a neuroscience of active sampling and curiosity. *Nature Reviews Neuroscience* 19(12):758–770. https://doi.org/10.1038/s41583-018-0078-0 (abstract)
- Grimaldi & Karpicke (2012). When and why do retrieval attempts enhance subsequent encoding? *Memory & Cognition*. https://doi.org/10.3758/s13421-011-0174-0 (abstract)
- Gruber, Gelman & Ranganath (2014). States of curiosity modulate hippocampus-dependent learning via the dopaminergic circuit. *Neuron* 84(2):486–496. https://doi.org/10.1016/j.neuron.2014.08.060
- Guadagnoli & Lee (2004). Challenge point: a framework for conceptualizing the effects of various practice conditions in motor learning. *Journal of Motor Behavior* 36(2):212–224. https://doi.org/10.3200/JMBR.36.2.212-224 (abstract)
- Hays, Kornell & Bjork (2013). When and why a failed test potentiates the effectiveness of subsequent study. *JEP: LMC*. https://doi.org/10.1037/a0028468 (abstract)
- Heathcote, Brown & Mewhort (2000). The power law repealed: the case for an exponential law of practice. *Psychonomic Bulletin & Review* 7(2):185–207. https://doi.org/10.3758/BF03212979 (abstract)
- Heitz (2014). The speed-accuracy tradeoff: history, physiology, methodology, and behavior. *Frontiers in Neuroscience* 8:150. https://doi.org/10.3389/fnins.2014.00150
- Herbrich, Minka & Graepel (2007). TrueSkill: a Bayesian skill rating system. *NIPS 19*:569–576. https://papers.nips.cc/paper_files/paper/2006/file/f44ee263952e65b3610b8ba51229d1f9-Paper.pdf
- Jansen, Louwerse, Straatemeier, Van der Ven, Klinkenberg & Van der Maas (2013). The influence of experiencing success in math on math anxiety, perceived math competence, and math performance. *Learning and Individual Differences* 24:190–197. https://doi.org/10.1016/j.lindif.2012.12.014 (abstract; exact success levels not read)
- Jansen et al. (2016). [Children choosing their own success rate in Math Garden; title not checked.] *Learning and Individual Differences* 51:1–10. https://doi.org/10.1016/j.lindif.2016.08.027 (abstract)
- Juul (2013). *The Art of Failure: An Essay on the Pain of Playing Video Games.* MIT Press. https://jesperjuul.net/artoffailure/
- Kalyuga, Ayres, Chandler & Sweller (2003). The expertise reversal effect. *Educational Psychologist* 38(1):23–31. https://doi.org/10.1207/s15326985ep3801_4 (abstract)
- Kanfer & Ackerman (1989). Motivation and cognitive abilities: an integrative/aptitude-treatment interaction approach to skill acquisition. *Journal of Applied Psychology* 74(4):657–690. (bibliographic record only)
- Kang, Lindsey, Mozer & Pashler (2014). Retrieval practice over the long term: should spacing be expanding or equal-interval? *Psychonomic Bulletin & Review* 21(6):1544–1550. https://doi.org/10.3758/s13423-014-0636-z
- Kapur (2008). Productive failure. *Cognition and Instruction* 26(3):379–424. https://doi.org/10.1080/07370000802212669 (abstract)
- Kapur (2014). Productive failure in learning math. *Cognitive Science* 38(5):1008–1022. https://doi.org/10.1111/cogs.12107 (abstract)
- Karpicke & Roediger (2007). Expanding retrieval practice promotes short-term retention, but equally spaced retrieval enhances long-term retention. *JEP: LMC* 33(4):704–719. https://doi.org/10.1037/0278-7393.33.4.704
- Khajah, Lindsey & Mozer (2016). How deep is knowledge tracing? *EDM 2016*. https://arxiv.org/abs/1604.02416
- Kidd, Piantadosi & Aslin (2012). The Goldilocks effect: human infants allocate attention to visual sequences that are neither too simple nor too complex. *PLoS ONE* 7(5):e36399. https://doi.org/10.1371/journal.pone.0036399
- Klinkenberg, Straatemeier & van der Maas (2011). Computer adaptive practice of maths ability using a new item response model for on the fly ability and difficulty estimation. *Computers & Education* 57(2):1813–1824. https://doi.org/10.1016/j.compedu.2011.02.003 (abstract)
- Koedinger & Aleven (2007). Exploring the assistance dilemma in experiments with cognitive tutors. *Educational Psychology Review* 19(3):239–264. https://doi.org/10.1007/s10648-007-9049-0 (abstract)
- Koedinger, Corbett & Perfetti (2012). The Knowledge-Learning-Instruction framework: bridging the science-practice chasm to enhance robust student learning. *Cognitive Science* 36(5):757–798. https://doi.org/10.1111/j.1551-6709.2012.01245.x (abstract)
- Koedinger, Carvalho, Liu & McLaughlin (2023). An astonishing regularity in student learning rate. *PNAS* 120(13):e2221311120. https://doi.org/10.1073/pnas.2221311120 (numbers via the author's talk abstract and the 2024 replication)
- Kornell & Bjork (2008). Learning concepts and categories: is spacing the "enemy of induction"? *Psychological Science* 19(6):585–592. https://doi.org/10.1111/j.1467-9280.2008.02127.x
- Kornell, Hays & Bjork (2009). Unsuccessful retrieval attempts enhance subsequent learning. *JEP: LMC* 35(4):989–998. https://doi.org/10.1037/a0015729 (abstract)
- Lally, van Jaarsveld, Potts & Wardle (2010). How are habits formed: modelling habit formation in the real world. *European Journal of Social Psychology* 40(6):998–1009. https://doi.org/10.1002/ejsp.674 (abstract; median secondary)
- Lee, Lichand, Barnard, Klotz, Thille, Kim & Domingue (2026). Revisiting the regularity of student learning rate: sensitivity to which observations are included. *L@S '26*. https://arxiv.org/abs/2605.01690 (abstract)
- Li & Bates (2019). You can't change your basic ability, but you work at things, and that's how we get hard things done: testing the role of growth mindset on response to setbacks, educational attainment, and cognitive ability. *JEP: General* 148(9):1640–1655. https://doi.org/10.1037/xge0000669 (abstract)
- Li, Chu, Langford & Wang (2011). Unbiased offline evaluation of contextual-bandit-based news article recommendation algorithms. *WSDM 2011*. https://arxiv.org/abs/1003.5956
- Lindsey, Shroyer, Pashler & Mozer (2014). Improving students' long-term knowledge retention through personalized review. *Psychological Science* 25(3):639–647. https://doi.org/10.1177/0956797613504302
- Loibl, Roll & Rummel (2017). Towards a theory of when and how problem solving followed by instruction supports learning. *Educational Psychology Review* 29(4):693–715. https://doi.org/10.1007/s10648-016-9379-x (abstract)
- Lomas, Patel, Forlizzi & Koedinger (2013). Optimizing challenge in an educational game using large-scale design experiments. *CHI 2013*:89–98. https://doi.org/10.1145/2470654.2470668
- Lomas, Koedinger, Patel, Shodhan, Poonwala & Forlizzi (2017). Is difficulty overrated? The effects of choice, novelty and suspense on intrinsic motivation in educational games. *CHI 2017*:1028–1039. https://doi.org/10.1145/3025453.3025638 (abstract)
- Macnamara & Burgoyne (2023). Do growth mindset interventions impact students' academic achievement? *Psychological Bulletin*. https://doi.org/10.1037/bul0000352 (abstract)
- Maniktala, Cody, Barnes & Chi (2020). Extending the Hint Factory for the assistance dilemma: a novel data-driven HelpNeed predictor for proactive problem-solving help. *Journal of Educational Data Mining* 12(4):24–65. https://arxiv.org/abs/2009.13371 (abstract)
- Margulieux, Guzdial & Catrambone (2012). Subgoal-labeled instructional material improves performance and transfer in learning to develop mobile applications. *ICER 2012*. https://doi.org/10.1145/2361276.2361291 (abstract)
- Maris & van der Maas (2012). Speed-accuracy response models: scoring rules based on response time and accuracy. *Psychometrika* 77(4):615–633. https://doi.org/10.1007/s11336-012-9288-y (abstract)
- Metcalfe (2002). Is study time allocated selectively to a region of proximal learning? *JEP: General* 131(3):349–363. https://doi.org/10.1037/0096-3445.131.3.349 (abstract)
- Metcalfe & Kornell (2003). The dynamics of learning and allocation of study time to a region of proximal learning. *JEP: General* 132(4):530–542. https://doi.org/10.1037/0096-3445.132.4.530 (abstract)
- Metcalfe (2017). Learning from errors. *Annual Review of Psychology* 68:465–489. https://doi.org/10.1146/annurev-psych-010416-044022 (abstract)
- Mirzayanov (2018). Codeforces: problem difficulties. https://codeforces.com/blog/entry/62865
- AtCoder Problems difficulty estimator (kenkoooo). https://github.com/kenkoooo/AtCoderProblems/blob/main/estimator/main.py
- Moraes et al. (2023). [Spaced, interleaved low-stakes retrieval practice in CS1; title not checked.] ICETC 2023. https://doi.org/10.1145/3629296.3629362 (secondary description)
- Mrkva, Johnson, Gächter & Herrmann (2020). Moderating loss aversion: loss aversion has moderators, but reports of its death are greatly exaggerated. *Journal of Consumer Psychology*. https://doi.org/10.1002/jcpy.1156 (abstract)
- Murre & Chessa (2011). Power laws from individual differences in learning and forgetting. *Psychonomic Bulletin & Review* 18(3):592–597. https://doi.org/10.3758/s13423-011-0076-y (abstract)
- Niv, Duff & Dayan (2005). Dopamine, uncertainty and TD learning. *Behavioral and Brain Functions* 1:6. https://doi.org/10.1186/1744-9081-1-6 (abstract)
- Oudeyer, Gottlieb & Lopes (2016). Intrinsic motivation, curiosity, and learning: theory and applications in educational technologies. *Progress in Brain Research* 229:257–284. https://doi.org/10.1016/bs.pbr.2016.05.005 (abstract)
- Papoušek, Stanislav & Pelánek (2016). Impact of question difficulty on engagement and learning. *ITS 2016*, LNCS:267–272. https://doi.org/10.1007/978-3-319-39583-8_28
- Pavlik, Cen & Koedinger (2009). Performance factors analysis: a new alternative to knowledge tracing. *AIED 2009*:531–538. https://doi.org/10.3233/978-1-60750-028-5-531
- Pelánek (2016). Applications of the Elo rating system in adaptive educational systems. *Computers & Education*. https://doi.org/10.1016/j.compedu.2016.03.017
- Pelánek & Řihák (2017). Experimental analysis of mastery learning criteria. *UMAP 2017*. https://doi.org/10.1145/3079628.3079667
- Piech et al. (2015). Deep knowledge tracing. *NIPS 28*. https://papers.neurips.cc/paper/2015/hash/bac9162b47c56fc8a4d2a519803d51b3-Abstract.html (abstract)
- Rawson, Dunlosky & Sciartelli (2013). The power of successive relearning: improving performance on course exams and long-term retention. *Educational Psychology Review* 25(4):523–548. https://doi.org/10.1007/s10648-013-9240-4 (numbers secondary)
- Renkl (2014). Toward an instructionally oriented theory of example-based learning. *Cognitive Science* 38(1):1–37. https://doi.org/10.1111/cogs.12086 (abstract)
- Rohrer (2012). Interleaving helps students distinguish among similar concepts. *Educational Psychology Review* 24(3):355–367. https://doi.org/10.1007/s10648-012-9201-3 (abstract)
- Rohrer & Taylor (2006). The effects of overlearning and distributed practise on the retention of mathematics knowledge. *Applied Cognitive Psychology* 20(9):1209–1224. https://doi.org/10.1002/acp.1266
- Rohrer & Taylor (2007). The shuffling of mathematics problems improves learning. *Instructional Science* 35:481–498. https://doi.org/10.1007/s11251-007-9015-8
- Rohrer, Dedrick, Hartwig & Cheung (2020). A randomized controlled trial of interleaved mathematics practice. *Journal of Educational Psychology* 112(1):40–52. https://doi.org/10.1037/edu0000367
- Rowland (2014). The effect of testing versus restudy on retention: a meta-analytic review of the testing effect. *Psychological Bulletin* 140(6):1432–1463. https://doi.org/10.1037/a0037559
- Ryan, Rigby & Przybylski (2006). The motivational pull of video games: a self-determination theory approach. *Motivation and Emotion* 30:347–363. https://doi.org/10.1007/s11031-006-9051-8
- Sailer & Homner (2020). The gamification of learning: a meta-analysis. *Educational Psychology Review* 32(1):77–112. https://doi.org/10.1007/s10648-019-09498-w (abstract)
- Sala & Gobet (2017). Does far transfer exist? Negative evidence from chess, music, and working memory training. *Current Directions in Psychological Science* 26(6):515–520. https://doi.org/10.1177/0963721417712760 (abstract)
- Schultz, Dayan & Montague (1997). A neural substrate of prediction and reward. *Science* 275:1593–1599. https://doi.org/10.1126/science.275.5306.1593 (abstract)
- Seijts, Latham, Tasa & Latham (2004). Goal setting and goal orientation: an integration of two different yet related literatures. *Academy of Management Journal* 47(2). https://leeds-faculty.colorado.edu/dahe7472/Sejits%202004.pdf (abstract via search only)
- Settles & Meeder (2016). A trainable spaced repetition model for language learning. *ACL 2016*:1848–1858. https://aclanthology.org/P16-1174/
- Simpson, Norberg & Fancsali (2024). Replicating an "astonishing regularity in student learning rate". *EDM 2024*. https://educationaldatamining.org/EDM2024/proceedings/2024.EDM-short-papers.40/index.html
- Sinha & Kapur (2021). When problem solving followed by instruction works: evidence for productive failure. *Review of Educational Research* 91(5):761–798. https://doi.org/10.3102/00346543211019105 (abstract; conceptual vs procedural split secondary)
- Sio & Ormerod (2009). Does incubation enhance problem solving? A meta-analytic review. *Psychological Bulletin* 135(1):94–120. https://doi.org/10.1037/a0014212
- Sio, Monaghan & Ormerod (2013). Sleep on it, but only if it is difficult: effects of sleep on problem solving. *Memory & Cognition* 41(2):159–166. https://doi.org/10.3758/s13421-012-0256-7 (abstract)
- Sisk, Burgoyne, Sun, Butler & Macnamara (2018). To what extent and under which circumstances are growth mind-sets important to academic achievement? *Psychological Science* 29(4):549–571. https://doi.org/10.1177/0956797617739704 (abstract)
- Soderstrom & Bjork (2015). Learning versus performance: an integrative review. *Perspectives on Psychological Science* 10(2):176–199. https://doi.org/10.1177/1745691615569000 (abstract)
- Sweller & Cooper (1985). The use of worked examples as a substitute for problem solving in learning algebra. *Cognition and Instruction* 2(1):59–89. https://doi.org/10.1207/s1532690xci0201_3 (abstract)
- Tatel & Ackerman (2025). [Meta-analysis of skill decay, 457 reports; title not checked.] *Psychological Bulletin* 151(6):696–736. https://doi.org/10.1037/bul0000481 (abstract)
- Ten, Kaushik, Oudeyer & Gottlieb (2021). Humans monitor learning progress in curiosity-driven exploration. *Nature Communications* 12:5972. https://doi.org/10.1038/s41467-021-26196-w
- Thomas, Paul, Rasenyalo, Jones & Hendricks (2025). [Scoping review of the challenge point framework; title not checked.] *Journal of Motor Behavior*. https://doi.org/10.1080/00222895.2025.2508283 (secondary description)
- van der Linden (2007). A hierarchical framework for modeling speed and accuracy on test items. *Psychometrika* 72(3):287–308. https://doi.org/10.1007/s11336-006-1478-z
- Vanacore et al. (2024). [Randomized trial delaying hints and feedback for students who game the system; title not checked.] *LAK '24*:462–472. https://eric.ed.gov/?id=ED662901 (abstract)
- Vogel & Schwabe (2016). Learning and memory under stress: implications for the classroom. *npj Science of Learning* 1:16011. https://doi.org/10.1038/npjscilearn.2016.11 (abstract)
- Vygotsky (1978). *Mind in Society: The Development of Higher Psychological Processes.* Harvard University Press.
- Wagner, Gais, Haider, Verleger & Born (2004). Sleep inspires insight. *Nature* 427:352–355. https://doi.org/10.1038/nature02223 (abstract)
- Wang & Shah (2014). [Pressure, working memory and arithmetic in grade 3–4 children, N = 53; title not checked.] *British Journal of Educational Psychology*. https://doi.org/10.1111/bjep.12027 (abstract)
- Wilson, Shenhav, Straccia & Cohen (2019). The Eighty Five Percent Rule for optimal learning. *Nature Communications* 10:4646. https://doi.org/10.1038/s41467-019-12552-4 ; full text https://pmc.ncbi.nlm.nih.gov/articles/PMC6831579
- Wilson, Karklin, Han & Ekanadham (2016). Back to the basics: Bayesian extensions of IRT outperform neural networks for proficiency estimation. *EDM 2016*. https://arxiv.org/abs/1604.02336
- Wouters, van Nimwegen, van Oostendorp & van der Spek (2013). A meta-analysis of the cognitive and motivational effects of serious games. *Journal of Educational Psychology* 105(2):249–265. https://doi.org/10.1037/a0031311 (abstract)
- Xue, Wu, Kolen, Aghdaie & Zaman (2017). Dynamic difficulty adjustment for maximized engagement in digital games. *WWW 2017 Companion*:465–471. https://doi.org/10.1145/3041021.3054170
- Ye, Su & Cao (2022). A stochastic shortest path algorithm for optimizing spaced repetition scheduling. *KDD '22*:4381–4390. https://doi.org/10.1145/3534678.3539081 (abstract)
- Yeager et al. (2019). A national experiment reveals where a growth mindset improves achievement. *Nature* 573:364–369. https://doi.org/10.1038/s41586-019-1466-y
- Yudelson, Koedinger & Gordon (2013). Individualized Bayesian knowledge tracing models. *AIED 2013*, LNCS 7926:171–180. https://doi.org/10.1007/978-3-642-39112-5_18 (abstract)
