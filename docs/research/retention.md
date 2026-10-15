# Retention for katharsis: what brings a player back to a hard game

Psychology + neuroscience thread (D29), 2026-10-14. Input for the
implementation agents D29 funnels these findings into. No code was changed.
Line numbers are as of this date on the main working copy. The economy
workspace (`.agents/ws/a2-economy`, D25) is changing `engine.go`, `types.go`,
`rules.go` and the merge screen, so those will drift.

**Evidence grades**, as in `docs/research/learning-science.md`: *strong*:
meta-analyses or several RCTs in a nearby domain. *Moderate*: several
studies, one strong study, or strong evidence from a distant domain. *Weak*:
single studies, correlational designs, or failed replications.
*Speculative*: theory, simulation, or my extrapolation.

**Source labels.** *industry*: a developer talk, interview, postmortem or
company blog; not peer reviewed. *PR*: peer reviewed, used where a row mixes
the two. *abstract*: checked at abstract level only. *secondary*: numbers
taken from a secondary source.

**Cross-references.** *neuro* R1, M4, H2 and so on are proposals in
`docs/research/learning-science.md` (the neuro thread). *ML* M1–M8, G1–G6
and slices ML1–ML12 are in `docs/research/ml-pipeline.md`; neuro M4 and
ML M4 are different items. *UX* J1–J31 are in `docs/design/ux-journey.md`.
D numbers are decisions in `.agents/work/katharsis.md`. RT0–RT15 are this
doc's proposals and GR1–GR12 its guardrails.

**No study cited here used competitive programming,** and none tested a
learning roguelike. Every mapping to katharsis is inference.

**Coverage.** The source checks hit a shared web-search cap partway
through. Where this doc says "not found", the search was cut short. That does
not show the evidence is absent. The gaps are listed in §(c), "What remains
uncertain".

## (a) Summary

**First reframe.** The ask is "addictive like a roguelike". The player's own
goal is narrower and better: make CP stick without wasting time. The
evidence points the same way. Hours played predict almost nothing about
well-being. What goes wrong is play the player feels they *have to* do, not
a lot of play (§(b)3). Spaced practice beats massed practice, in games too
(§(b)3). So the target is **more days with a boot, each ending on something
to come back to**, not longer nights. "One more boot" is good when the boot
is short and the hour is early.

**Second reframe.** The biggest retention risk is not a missing hook. It is
the first biome. In the balance sim, Userland crashes 61% of boots for a
1000-rated model player and 26% for a 1600-rated one, against a 15% design.
Loss streaks churn about twice as often as streaks that end in a win. Every
hook below sits on top of that. Fix G1 first (RT0: D34 and ML slice ML7).

**Two corrections changed the proposals.**

- "Players like a game more after failing at it" is not what Juul found.
  That difference was not significant (p = .06). His significant result is
  about blame: players who blamed their own mistake rated the game higher
  than players who blamed the game. So a crash must read as "I made a
  mistake I can fix" (RT0, RT1), not as "failure is fun".
- "Players feel cheated when they find out the game adapts" has no
  peer-reviewed support. The one direct test found that *believing* a game
  adapts raised immersion. So RT8 announces easing for reasons that do have
  support, and plain copy is enough.

| # | Change | Mechanism | Evidence | Effect | Card |
|---|---|---|---|---|---|
| 0 | Hold Userland crash ≤ 20% before shipping hooks | loss streaks churn 2×; blaming the game lowers ratings | strong (descriptive); weak (blame) | L, protective | RT0 (D34, ML7) |
| 1 | End every boot with what persisted, one lever, one next step | task-level feedback; a fixable cause; the end weighs most in memory | strong (feedback, peak-end) | M | RT1 |
| 2 | Prove the spend-or-commit fork A2 is building is a real choice | autonomy at a risk choice | industry; moderate | L on loop feel | RT2 (A2) |
| 3 | Price `apt` on a goal gradient, options over power, about 40 boots deep | goal gradient; mastery stays the goal | moderate; industry | L feel, M return | RT3 (A2) |
| 4 | Core dump seen, protected, and paid more for an independent upsolve | people resume interrupted tasks; a plan closes the loop | moderate; strong (plans) | M on upsolves | RT4 |
| 5 | A fixed regression gets a beat; cron jobs pay; at most 3 shown | retrieval practice at low stakes (stakes settled by D35) | strong (learning) | M (mostly D35's) | RT5 |
| 6 | Story pacing: one beat per boot end, milestone beats in the gaps after boot 10 | curiosity near 50% confidence; Hades | moderate; industry | M (speculative) | RT6 (S9) |
| 7 | `short` and `patch` boots offered where "one more" is decided; boot #1 is short | spacing; a cheap next run | strong (spacing) | M on days/week | RT7 |
| 8 | Easing is announced in task terms and declinable; comeback boots after a break | belief in adaptation raised immersion; God Mode; pay the return after a miss | weak–moderate; industry | M after streaks | RT8 |
| 9 | `load average` instead of a streak | broken streaks demotivate; a missed day does not hurt habits | moderate | S | RT9 |
| 10 | Locked routes visible in the run | goals; Metroidvania gating | strong (goals); untested (gating) | S–M on RTFM | RT10 |
| 11 | First session closes one loop in about an hour | early competence; light onboarding | moderate | M on session 2 | RT11 |
| 12 | Session budget and bedtime at boot boundaries; no lockouts | spacing; caps do not work, nudges are small | moderate–weak | S–M | RT12 |
| 13 | Retention panel in `top` and a once-a-day "worth it?" | how play fits life tracks well-being; hours do not | weak–moderate | enabling | RT13 |
| 14 | Opt-in shell line when something is due | habits are cued by context | moderate | S–M | RT14 |
| 15 | Nightly with no expiry and no streak | no evidence for daily hooks | weak | S | RT15 |

**Effect** is the expected change on the card's metric for this player (L /
M / S / ?), an inference. **Evidence** is the grade of the best source for
the mechanism. One player cannot test these statistically (about 90 boots
per arm to see a 20-point change, §(c) intro). So the ranking rests on the
evidence, and the guardrails (§(d)) do the watching.

**Do not build** (details in §(c), "Considered and rejected"): streaks or
streak freezes, login rewards, random payouts or loot crates, story behind
bytes, hard session locks, fake near misses, and "adaptive AI" copy for
adaptation that did not happen.

**Pop psychology to drop from design talk:**

- "Dopamine hits". The only gameplay PET study (N = 8) shrank to
  non-significance after motion correction, by its own authors.
- "Variable ratio / Skinner box is why games hook people". Speculative in
  humans; rats on slot-machine schedules were "motivating but not addictive".
- "The Zeigarnik effect". It does not replicate: the recall ratio of
  interrupted to finished tasks is 0.99. People do *resume* interrupted
  tasks (Ovsiankina).
- "Failing makes players like a game more". A non-significant trend (Juul
  2009, p = .06).
- "Players feel cheated by difficulty adjustment". Unsupported; the one test
  ran the other way.
- Fogg's B = MAT and *Hooked*: never tested.
- Ego depletion: failed two multi-lab replications.

## (b) Findings by topic

### 1. Why hard games and roguelikes bring players back

The short answer, from five literatures that agree more than they differ:
**competence the player earned, deaths the player can blame on a fixable
mistake, and progress that survives the death.** Flow and reward
neuroscience add less than their reputation suggests.

#### 1a. Competence predicts staying; autonomy predicts enjoying

| Finding | Source | Grade |
|---|---|---|
| Lab, Super Mario 64 (N = 89): enjoyment tracked autonomy (β = .49) and competence (.34). *Actually choosing to keep playing* was predicted by competence only (β = .41; autonomy −.04). In an MMO survey (N = 730), the three needs explained 45% of enjoyment but only 10% of intended future play | Ryan, Rigby & Przybylski 2006 | moderate; all self-report, needs measured, not manipulated |
| Games appeal by satisfying competence, autonomy and relatedness | Przybylski, Rigby & Ryan 2010 | theory |
| Need satisfaction in play goes with *harmonious* passion: enjoyment and energy after play, and it did not predict hours. Low need satisfaction goes with *obsessive* passion: more play, tension after play, less enjoyment | Przybylski, Weinstein, Ryan & Rigby 2009 (abstract); Vallerand et al. 2003 (4 studies, N ≈ 949) | weak for games (one survey); moderate–strong for the passion model |
| Specific hard goals beat "do your best" (d = .42–.80), less so on complex tasks. Goals plus feedback beat goals alone. Self-efficacy after negative feedback decides whether the next goal goes up or down | Locke & Latham 2002 | strong |
| Performance accomplishments are the most dependable source of self-efficacy | Bandura 1977 | theory, experimental support |
| Practice explains 26% of performance variance in games, 4% in education (88 studies; the definition of deliberate practice is disputed) | Macnamara, Hambrick & Oswald 2014 | strong but disputed |

#### What this means for katharsis

- **Competence is the lever for coming back.** In the one study that
  measured a behaviour rather than an intention, only competence predicted
  the choice to keep playing. katharsis's competence signals are the clear,
  the rating and the topic trend. Today a boot ends without showing any of
  them (UX J1, J6). RT1 shows them.
- **The competence must be real.** The surveys cannot tell earned
  competence from bought competence. In a learning game, power bought with
  bytes would feel like progress, make clears easier and be read by the
  rating as skill. Hence RT3's options-over-power rule.
- **Autonomy is already rich:** routes, doors, presets, prestige, upgrade
  and daemon picks. More menus add little. Add choice where risk is decided
  (RT2's merge-window fork).
- **The crash screen's job is self-efficacy.** Locke & Latham: after negative
  feedback, efficacy decides whether the next goal rises or falls. So the
  screen shows what was achieved (`kept`) and a reachable next goal
  (`next`) (RT1).
- **SDT explains enjoyment better than return** (R² = .10 for intended
  future play). It cannot carry the retention case alone. The session-level
  findings in §(b)2 carry the rest.

#### 1b. Failure helps when it is blamed on a fixable mistake and followed by task feedback

| Finding | Source | Grade |
|---|---|---|
| "Fiero: personal triumph over adversity… it does require effort." From a field study of 30 adults playing their own games, with over 2,000 coded observations | Lazzaro 2004 | industry white paper; no peer-reviewed validation found |
| Dark Souls III players (N = 95, self-selected after release): achievement and learning moments drove positive experience, and difficulty and avatar death enabled them | Petralito et al. 2017 (PR, abstract) | weak (survivor bias) |
| Players value negative emotion in games (121 accounts); emotional challenge was appreciated more than conventional challenge (N = 171) | Bopp, Mekler & Opwis 2016; Bopp, Opwis & Mekler 2018 | weak |
| In internet chess, stronger opponents and close games were more enjoyable | Abuhamdeh & Csikszentmihalyi 2012 (abstract) | weak–moderate |
| Some negative events in Super Monkey Ball 2 produced positively valenced arousal (N = 36) | Ravaja et al. 2006 (abstract) | weak |
| **Juul's experiment**, often cited as "failing makes players like a game more": players who failed and then completed a Pac-Man-like prototype rated it highest, but across the three outcome groups **p = .06, not significant** (85 self-selected blog readers plus 9 testers). The **significant** result (p < .016) is about blame: players who blamed themselves ("I made a mistake") rated the game higher than those who blamed the game ("too hard") | Juul 2009 | weak |
| Retention depended on what players blamed failure on, the player's skill or the avatar's strength (Ubisoft data) | Allart et al. 2017 (abstract; `learning-science.md` §4) | weak |
| Causes of success and failure differ in locus, stability and controllability. Stability sets the expectation of future success; all three shape pride, shame and anger | Weiner 1985 | theory |
| Feedback raises performance on average (d = .41), but over a third of interventions *lowered* it. Feedback aimed at the self hurt: praise d = .09 vs .34 without; discouraging feedback −.14; high threat to self-esteem .08 vs .47 low. Feedback aimed at the task helped: improvement over previous trials .55 vs .28; the correct solution .43 vs .25 | Kluger & DeNisi 1996 (607 effect sizes) | strong (mean); moderate (moderators, compared across studies) |
| Feedback works at the task, process and self-regulation levels and answers "where am I going, how am I going, where to next"; praise (self level) 0.14. The pooled 0.79 is superseded by 0.48 | Hattie & Timperley 2007; Wisniewski, Zierer & Hattie 2020 | moderate |
| Induced pride increased persistence on an effortful, unpleasant task, separately from self-efficacy and from general positive mood | Williams & DeSteno 2008 (abstract) | weak–moderate |
| Pride attributed to effort (authentic) and to ability (hubristic) are distinct, with different correlates | Tracy & Robins 2007 (7 studies) | moderate |
| Passivity is the default response to prolonged aversive events you cannot control; detecting control (medial prefrontal cortex) suppresses it | Maier & Seligman 2016 | moderate in rodents; speculative for humans and games |
| People avoid cognitive demand: 36 of 43 favoured the low-demand option | Kool et al. 2010 | moderate–strong |
| Effort is costly but can also add value; people sometimes choose options because they are hard | Inzlicht, Shenhav & Olivola 2018 | theory |
| Rewarding high effort gives effort secondary reward value; rewarding cognitive effort raised later preference for effortful tasks | Eisenberger 1992; Clay et al. 2022 | moderate; transfer across domains contested |
| "When X, I will Y" plans: d = .65 over 94 independent tests | Gollwitzer & Sheeran 2006 (abstract) | strong; a 2006 pooled d before modern bias correction, so likely an overestimate (inference) |

#### What this means for katharsis

- **The bar for a crash is Juul's significant result, not his headline.**
  A crash should read as "I made a mistake I can fix", never as "the game
  is too hard". Two things break that today. First, deaths that come from
  the interface: a failing-sample `:Submit` spends a life (UX J3), `R`
  spends one without asking (UX J4), and a door can take 34–75 s to open
  (UX J9). Second, a first biome that kills 26–61% of the time (RT0). Both
  invite blaming the game.
- **Point the blame at the task, not the self.** In Juul, "self-blame" means
  "I made a mistake". Silverman & Barasch (§(b)2) found that blaming yourself
  for a *broken streak* made it worse. The two fit Weiner (inference): a
  specific, controllable, unstable cause ("TLE on the largest test") keeps
  the expectation of success; a global, stable one ("I'm bad at this", "I
  broke my streak") lowers it. Kluger & DeNisi give the same rule from the
  feedback side. So RT1's lever line names a test, a time, a tool or a
  topic, never the player.
- **Show improvement, not praise.** Feedback on improvement over previous
  trials was one of the strongest moderators (d = .55); praise was near
  zero. RT1's personal-best and first-time lines are improvement feedback.
  "Great job!" is self-level and does nothing.
- **Pride is the boss kill's job.** Earned pride sustains effort (weak–
  moderate). Name what earned it (time, attempts, the test that finally
  passed). Never inflate it (GR3).
- **Pay the effort the player could have skipped.** Effort is avoided by
  default (Kool) and gains value when it pays (Eisenberger). That is RT4's
  1.5× for an upsolve done without the editorial.
- **Write the plan for the player.** Implementation intentions are the
  strongest evidence in this section. RT1's `next` line and RT4's
  "upsolve Karen and Morning" are plans the game writes.
- **Helplessness is a risk only when outcomes stop tracking actions**
  (Maier & Seligman; speculative for games). A crash streak against an aim
  the player cannot reach is exactly that case. RT0 again.

#### 1c. What the designers of hard games say (industry, not peer-reviewed)

Every row is a developer statement, not a test. The peer-reviewed rows
are marked PR.

| Principle | Who says it | Grade |
|---|---|---|
| **Forgive the interface, keep the challenge in decisions.** Coyote time, jump buffering, corner correction (Celeste). Ledges forgive pixels, attacks auto-aim: "the challenge is not here" (Dead Cells). Telegraphed enemy intent, ~10 versions, because random movesets made blocking guesswork (Slay the Spire). "Give the player all of the tools that they need" (Spelunky) | Thorson 2020; Bénard, GDC 2019; Giovannetti, GDC 2019; Yu 2011 | industry |
| **Death has to mean something, and the loss creates the payoff.** "If the Souls could be recovered anytime, there would be no suspense or sense of accomplishment… 'I did it! I made it to my bloodstain!'" "How do I make death enjoyable?" | Miyazaki 2009, 2022 | industry |
| Dark Souls III players (survey, N = 95): achievement and learning drove positive experiences, and these "were enabled by negative events such as difficulties and avatar death" | Petralito et al. 2017, CHI (PR) | weak–moderate |
| Consequential death raised perceived investment and social stakes (DayZ); self-imposed permadeath turns play "from experimentation to vulnerability" | Carter, Gibbs & Wadley 2013 (PR); Keogh 2013 (PR, autoethnography) | weak |
| **Every run counts.** "No sense of a 'game over' moment"; "many channels of permanent progression"; characters react whether it is the first or the 20th time. The Mirror makes you "meaningfully more powerful". "Engrossing but… not interested in making it addictive"; no daily login bonuses | Kasavin, Tom's Guide 2021 | industry |
| Hades: "more than 22,000 lines of dialogue" from fewer than 20 people; Early Access "a huge boon for the narrative" | Kasavin & Korb, GDC 2021 | industry |
| **Progression as consolation, mastery as the goal.** The metagame should "offer a meaningful sensation of progression… even when the run ends earlier than you would have liked"; "the challenge is not in acquiring more abilities, but in mastering those you already have"; "a short leash on the RNG" | Bénard 2018; Filby 2017 | industry |
| **Ease off without breaking the loop.** God Mode: 20% damage resistance, +2% per death, cap 80%, to "take the sting of failure"; a normal easy mode was rejected because dying and looping "is a really important part of the experience" | Kasavin, Inverse 2021 | industry |
| Assist Mode text: "its difficulty is essential to the experience… If Celeste is inaccessible to you due to its difficulty, we hope that Assist Mode will allow you to still enjoy it"; first named "Cheat Mode", dropped as judgmental. Chapter 3 postcard: "Be proud of your Death Count! The more you die, the more you're learning. Keep going!" | Thorson; Vice 2018; celeste.ink (secondary) | industry |
| **Difficulty as a ladder, not a knob.** Ascension "a super cool way to add difficulty and give players a sort of checklist"; "Difficulty should ebb and flow"; a single easy-to-hard knob is "futile" | LocalThunk 2025; Yu 2016 | industry |
| **Every trade-off is a choice.** Rogue Legacy 2: traits were novelties in the first game; "Make everything have a trade-off!"; a gold bonus on hard traits "to encourage players to always test themselves" | Lee 2022 | industry |
| **Instrument, but distrust raw win rates and the loudest voices.** Slay the Spire sliced death and win data by Ascension and found late-pick cards looked inflated; Hades tracked first-clear weapons and where players got stuck, but "the most vocal requests might not even be the thing we need to look at" | Giovannetti 2019; Rao, PC Gamer 2020 | industry |
| Daily runs (Spelunky's one shared attempt, Slay the Spire's modifier dailies) are framed as same-seed competition; no developer retention data found | Rose 2013 (press); Giovannetti 2019 | no evidence |
| Run lengths: no developer-stated averages found for Hades, Dead Cells, Slay the Spire, Spelunky or Balatro; only anecdotes (20–60 min Dead Cells runs; a 30-min Hades win) | | no evidence |

#### What this means for katharsis

- **katharsis already follows principle 1 except at three seams:** a
  failing-sample `:Submit` spends a life (UX J3), `R` spends one without
  asking (UX J4), and a 34–75 s fetch sits between choosing and playing
  (UX J9). These are interface punishments, the kind Celeste and Dead Cells
  remove, and they invite blaming the game, which goes with lower ratings
  (Juul 2009, §(b)1b). Fix them before adding difficulty anywhere.
- **The core dump is the bloodstain**, nearly word for word: recovering it
  is the suspense. Keep the double-loss rule; make it visible (RT4).
- **"Every run counts" is the bar.** Today a crash carries forward a
  regression and maybe a dump, but nothing on screen says so, and a clear
  can carry forward nothing (UX J1). RT1 shows it; RT3 and RT6 add
  channels (unlocks, story).
- **Mastery is the goal, meta-progression the consolation** (Bénard). For
  a learning game this is not taste, it is the measurement model: power
  bought with bytes would be counted as skill. Hence RT3's
  options-over-power rule.
- **God Mode is the template for the governor** (RT8): visible, opt-out,
  small per-failure steps, never a different game. Celeste's renaming of
  "Cheat Mode" sets the copy rule: name the help without judging the
  player.
- **Celeste's postcard line is the right register** for crash copy: death
  as learning, not as a verdict. Its claim ("the more you die, the more
  you're learning") is true here only when the death is followed by
  feedback (neuro R1: retrieval without feedback at low success does
  nothing). So RT1 pairs every death with a lever.

#### 1d. Flow, and difficulty adjustment the player knows about

The numbers for the challenge band are in `learning-science.md` §1 (0.65–0.85
per door for learning; easier is stickier within a session). This section
adds what happens when the band breaks and what happens when the game says
it is adjusting.

| Finding | Source | Grade |
|---|---|---|
| Challenge–skill balance correlates moderately with flow; flow's inverted U holds for a low-stakes game but not before a high-stakes exam; easier was more engaging down to 63% success | Fong et al. 2015; Engeser & Rheinberg 2008; Lomas et al. 2013 (all in `learning-science.md` §1) | moderate |
| Tetris at too-easy, adaptive and too-hard settings: the adaptive one gave the most involvement and enjoyment and the shortest perceived time | Keller & Bless 2008 (N = 72 and design from a secondary source) | moderate–weak |
| GameFlow lists eight elements (concentration, challenge, skills, control, clear goals, feedback, immersion, social interaction). Validation: expert reviews of two strategy games | Sweetser & Wyeth 2005 | weak as validation |
| System-driven adjustment fails because "performance does not mirror flow" (a DDA would read GTA suicide stunts as low skill). Put the choice inside play: flOw lets you dive or rise, with "a minimal death penalty" | Chen 2007 (CACM viewpoint; MFA thesis) | industry / theory; informal evidence |
| Hamlet DDA. The abstract states "players feel cheated by adjustment" as conventional wisdom in order to challenge it; results are preliminary | Hunicke 2005 | speculative–weak |
| **Telling players a game had adaptive AI, when it had none, raised immersion**: within subjects N = 21, F(1,20) = 7.88, p = .011, ηp² = .28; between subjects N = 40, 124.2 vs 116.0, p = .019, ηp² = .14 | Denisova & Cairns 2015 (full text) | weak–moderate (two small studies, one game) |
| Difficulty set by DDA can leave players highly overconfident about their performance (in-game bets, three games) | Constant & Levieux 2019 (abstract) | weak |
| Difficulty adjustment in a match-3 game raised rounds played by 4.4–7.9% | Xue et al. 2017 (`learning-science.md` §1) | moderate |
| Seven-day churn after three losses in a row 5.1%, after loss-loss-win 2.6% | Chen et al. 2017, EOMM (`learning-science.md` §1) | strong as description |
| A study showing that *knowing* about DDA lowers enjoyment | not found (search cut short) | none |

#### What this means for katharsis

- **The flow numbers are already set** (`learning-science.md` P1.1–P1.5,
  ML M4). Retention adds the losing side: the streak that breaks flow is
  what churns (EOMM), so the governor and the Userland buy-back are
  retention mechanics first (RT0, RT8).
- **Announcing adaptation is not a cost, as far as the evidence goes.** The
  common worry has no peer-reviewed support: Hunicke raised it to challenge
  it, and the one direct test found the *belief* that a game adapts raised
  immersion. So RT8 announces easing because (1) it costs nothing known,
  (2) the designer-player will know anyway, and (3) hidden adjustment can
  leave players overconfident about their skill (weak), which here would
  corrupt the self-assessment the game exists to train.
- **The placebo is not a license.** Denisova & Cairns produced the effect
  by claiming adaptation that did not exist. katharsis's aim really does
  adapt (calibration today, the ML aim later), so saying so is true.
  Claiming more than happens breaks GR3.
- **Prefer difficulty the player chooses.** Chen's argument, and Lomas 2017
  (`learning-science.md` §1): moderate difficulty motivates most when
  chosen. Prestige, presets and `patch` (RT7) are that ladder. The governor
  is a floor, not a knob.

#### 1e. Reward, dopamine, near misses: what the neuroscience supports

| Finding | Source | Grade |
|---|---|---|
| Midbrain dopamine neurons signal reward prediction error (RPE): firing to an unpredicted reward, moving to the cue once learned, dipping when an expected reward is omitted; a timed dopamine pulse causes learning (rats); human striatal fMRI shows RPE signals | Schultz, Dayan & Montague 1997; Steinberg et al. 2013; Garrison et al. 2013 (meta) | strong; challenged by value-of-work (Hamid et al. 2016) and causal-association (Jeong et al. 2022) accounts |
| The cue response scales with reward probability. A slower ramp peaking at p = 0.5 ("uncertainty") may be an averaging artifact | Fiorillo, Tobler & Schultz 2003; Niv, Duff & Dayan 2005 | scaling strong; the 0.5 ramp weak/contested, monkeys only |
| Dopamine drives *wanting* (incentive salience), not *liking*: 99% dopamine depletion left rats' liking of sweet intact. In humans this is contested: levodopa raised musical pleasure (N = 27, double-blind) | Berridge & Robinson 1998, 2016; Pool et al. 2016; Ferreri et al. 2019 | strong in animals; weak/contested in humans |
| "Video games release dopamine" rests on one PET study (N = 8, −12% raclopride binding); the same group's reanalysis with motion correction found −2.8% to −5.1%, no region significant: "head movement may have biased our published results" | Koepp et al. 1998; Egerton et al. 2009 | weak; do not cite as fact |
| An unpredictable 25% reward schedule changed striatal dopamine more than a predictable one (mixed direction by region; N ≈ 9) | Zald et al. 2004 | weak |
| Partial reinforcement raises persistence in extinction in lab humans; human evidence on variable- vs fixed-ratio is "sparse"; rats on random-ratio schedules were "motivating but not addictive" | Humphreys 1939; James, O'Malley & Tunney 2016; Laskowski et al. 2019 | moderate (lab persistence); "variable ratio is why games are addictive" is speculative |
| Near misses raise rated desire to play, **only when the player chose** (control × outcome F(2,78) = 6.50); computer-chosen near misses *lowered* it; near misses felt worse than full misses. Ratings only; no persistence measured | Clark et al. 2009, *Neuron* (N = 40 + 15 fMRI) | moderate for ratings; weak for the control moderation |
| Near-miss effects on actual persistence are inconsistent: a systematic review of 51 experiments says they "appear" to prolong play; a 2020 study in pigeons and 178 humans found none; a 2024 preregistered replication found higher motivation and bets but reversed the valence | Barton et al. 2017; Pisklak, Yong & Spetch 2020; Palmer, Ferrari & Clark 2024 | moderate (arousal, motivation); weak/mixed (persistence) |
| In Candy Crush, losses within three moves of passing raised urge (p = .05) and frustration vs plain losses; no play measured | Larche, Musielak & Dixon 2017 (N = 56) | weak |
| Losses disguised as wins (paying back less than the bet, with win sounds) produce arousal like real wins and make players overestimate winnings | Dixon et al. 2010; Barton et al. 2017 | moderate |
| Loot-box spending correlates with problem gambling, r ≈ .26 across 15 studies; causation unresolved | Zendle & Cairns 2018, 2019; Garea et al. 2021; Spicer et al. 2022 | strong association; weak causal |
| Curiosity is an inverted U of confidence (peaks near 50% confident); it predicts paying time or tokens to see answers and better memory for them (d = 1.11, small N); the caudate tracks it | Kang et al. 2009; theory Loewenstein 1994 | moderate |
| High curiosity improves memory for the answer (70.6% vs 54.1%); spillover to incidental material is small and failed once | Gruber, Gelman & Ranganath 2014; Keller et al. 2024 | moderate (target); weak (spillover) |
| Surprise (unsigned prediction error) strengthens memory for the surprising event; whether positive surprise is special is open | Rouhani, Norman & Niv 2018; Jang et al. 2019 | moderate |
| Uncertain points rewards ("Mr Uncertain") were preferred by children in a learning game | Howard-Jones & Demetriou 2009; Ozcelik et al. 2013 | weak |

#### What this means for katharsis

- **Drop "dopamine hits" from the design vocabulary.** The defensible
  claims are narrower: dopamine signals prediction error and drives
  *wanting*; the evidence that games trigger large dopamine release is one
  N = 8 study that its own authors walked back. Design for informative
  surprises, not for "hits". (Pop-psychology flag.)
- **Variety goes in what is offered, not in whether effort pays.** The
  lab evidence for variable schedules is real but narrow, and the
  manipulative versions (slots, loot boxes) attach randomness to payout.
  katharsis already randomizes the *draw* (doors, routes, daemons,
  upgrades) and equalizes expected bytes per door (`doorBytes`); keep both
  (GR5). The thing to add is *more kinds* of draw (RT3's packages).
- **Near misses here are true, and that is the ethical difference.** In
  Clark's task the near miss motivated only under an *illusion* of control.
  In katharsis control is real: 51/52 tests passed is evidence about the
  solution. Show it plainly (HP bar, first failing test, RT1's lever) and
  never manufacture one. Expect it to raise motivation more than
  persistence, and to feel bad: pair it with the next step.
- **Never put win cues on a loss.** No AC sound, green or fanfare on a
  crash screen, even when bytes are recovered later (GR3).
- **Curiosity is a real lever for the story and the lever line.** It peaks
  when the player is about half sure of the answer: a story beat should end
  on a question the player can almost answer (RT6), and a crash lever that
  names the failing test ("n = 1?") invites the upsolve.
- **The 50% boss is still right, for measurement and suspense, not for
  "dopamine at 0.5".** Information peaks at p = 0.5 (a theorem; neuro §1);
  the dopamine-ramp argument for it is contested.

### 2. Session hooks that respect the player

#### Findings

| Finding | Source | Grade |
|---|---|---|
| **Zeigarnik (better memory for unfinished tasks) does not replicate on average**: recall ratio interrupted ÷ completed = 0.99 over 37 publications. **Ovsiankina (people resume interrupted tasks) holds**: 67% resumed vs 50% chance (21 publications; measures vary). A money incentive *lowered* resumption in one study | Ghibellini & Meier 2025 (meta-analysis, sample-weighted means); McGraw & Fiala 1982 | Zeigarnik weak; Ovsiankina moderate |
| Writing a specific plan removed the intrusive effects of an unfinished goal | Masicampo & Baumeister 2011 (6 lab studies; no replication found) | weak |
| Goal gradient: coffee-card customers bought ~20% faster near the reward; a 12-stamp card with 2 free stamps finished in 12.7 vs 15.6 days; fast accelerators were retained better | Kivetz, Urminsky & Zheng 2006 (field) | moderate |
| Endowed progress: a 10-stamp car-wash card with 2 stamps given completed 34% vs 19% for a blank 8-stamp card; the effect rests on perceived completion, not on not wasting the gift | Nunes & Drèze 2006; conceptual replication in Kivetz 2006 | moderate |
| **Broken streaks demotivate.** Field: a just-broken streak b = −1.01 for continuing a step challenge (N = 980, observational). Lab (preregistered, N 400–600 each): the same behaviour logged as a broken streak lowered the next choice (OR 0.53 for showing the break); stronger when people blamed themselves; a repairable streak recovered most of it (85% vs 69%). Outcomes are within-session, not days | Silverman & Barasch 2023, *JCR* | moderate |
| Streak apps motivated but bred dependency; a life disruption led people to drop both app and habit (16 interviews) | Renfree et al. 2016 | weak |
| Duolingo: Streak Wager +14% day-7 retention; two Streak Freezes +0.38% DAU; 7-day streakers 2.4× likelier to return (correlational, selection) | Duolingo blog 2017, 2021, 2022 | industry, not peer-reviewed |
| Notification timing bandit: +0.5% DAU, +2% new-user retention; new templates had a novelty lift that faded (15-day half-life) | Yancey & Settles 2020, KDD (peer-reviewed, industry-authored) | moderate |
| Gym megastudy (61,293 members, 54 arms): the best arm paid $0.09 of points for **returning after a missed workout**: +27% visits vs placebo, +16% vs the shared base arm; the authors call top estimates inflated; most effects vanished after the 4 weeks | Milkman et al. 2021, *Nature*; critique Collins 2022 | moderate |
| Fresh starts: goal behaviour jumps at temporal landmarks (gym visits +33% at a new week, +47% at a new semester) | Dai, Milkman & Riis 2014 | moderate (archival) |
| Habits take 18–254 days to automate (254 extrapolated); missing one opportunity did not matter | Lally et al. 2010; review Singh et al. 2024 | weak–moderate |
| Habits are context–response links; the cue alone triggers the behaviour; change exposure to cues to change habits | Wood & Neal 2007; Wood & Rünger 2016 | strong as synthesis |
| Peak-end: remembered experience tracks the peak and the end (r = 0.58 over 174 effects; duration ≈ 0), though the plain average predicts about as well. A randomized gentler ending of a colonoscopy raised return for the next one (OR 1.41) | Alaybek et al. 2022; Kahneman et al. 1993; Redelmeier, Katz & Kahneman 2003 | strong |
| Fogg's B = MAT and *Hooked* | Fogg 2009 (conference, never tested); Eyal 2014 (trade book) | speculative, pop-psychology |
| Daily challenges, login rewards, daily quests and retention | none verified; "playing by appointment" is a dark pattern when progress depends on it (Zagal 2013) | no evidence |
| Prolonged attention work (3 h, no rest) degrades goal-directed attention | Boksem, Meijman & Lorist 2005 | weak–moderate |
| Ego depletion: d = 0.04 (23 labs) and d = 0.06 (36 sites) | Hagger et al. 2016; Vohs et al. 2021 | failed replication; do not cite |
| Total playtime per player fits a Weibull: interest decays, early play predicts quitting | Bauckhage et al. 2012 (250k players) | weak–moderate |
| Narrative transportation shifts story-consistent beliefs; no peer-reviewed study shows cliffhangers drive return | Green & Brock 2000; van Laer et al. 2014 (meta) | moderate for transportation; none for cliffhangers |

#### What this means for katharsis

- **Cliffhangers: build on resumption, not memory.** The pop claim
  ("unfinished tasks nag at you", Zeigarnik) is not supported; the
  resumption tendency is. So the hook is a *resumable object* (a dump, a
  due cron job, a beat waiting) that GRUB puts first, not an itch.
  Pairing it with a stated next step (RT1's `next`) may also let the
  player stop without rumination (weak evidence).
- **Goal gradient is the strongest lever on "one more boot"** that is not
  manipulative when the progress is real. RT3 prices packages so the next
  one is one to three boots away and shows the bar where boots end.
- **Endowed progress, done honestly:** make the first goal cheap (the first
  package costs at most one boot), rather than pre-filling a bar the
  designer-player can see is fake.
- **No streaks.** The best evidence says the *display* of a broken streak
  lowers engagement, especially with self-blame. Duolingo's gains are
  small and selection-confounded. `load average` (RT9) shows consistency
  with no break event to display.
- **Reward the return, not the run of days.** The single best arm of the
  largest habit megastudy paid for coming back after a miss; the fresh-
  start data say returns cluster at landmarks (new week). RT8's comeback
  boot does both: an easier first door and what is due, framed as a fresh
  start, with no mention of the gap. (The effect may be inflated, and it
  faded when the reward stopped; so keep it permanent and small.)
- **Cues beat reminders.** A context cue (a new shell, RT14) fits the habit
  literature; push-style reminders show small, fading effects.
- **The end of a boot is a measured lever.** Peak-end is strong, and the
  colonoscopy RCT shows a gentler ending raises return. RT1 makes the end
  constructive without faking it.
- **Daily seeds have no evidence for or against.** Keep the nightly as a
  cue, with no penalty and no expiry (RT15).

### 3. Compelling vs. compulsive

#### Findings

| Finding | Source | Grade |
|---|---|---|
| Gaming disorder (ICD-11 6C51) needs impaired control, rising priority over other activities, and continuing despite harm, with significant impairment, normally for 12 months | WHO ICD-11 | definition |
| Over two thirds of gamers report no IGD symptoms; 0.3–1.0% of the general population might qualify for an acute diagnosis (4 surveys, N = 18,932, preregistered) | Przybylski, Weinstein & Murayama 2017 | moderate |
| Hours played barely relate to well-being: +1 h/day of play changed affect by [−0.09, 0.16] across 38,935 players of 7 games over 6 weeks. Intrinsic motivation predicted higher life satisfaction (0.18), extrinsic lower (−0.11) | Vuorre et al. 2022; Johannes, Vuorre & Przybylski 2021 (telemetry, weak alone) | moderate |
| Preregistered, N = 703: hours did not predict well-being; how gaming fits the player's life did (b ≈ 0.24, about 10× the hours estimate; scale unvalidated). A 12-week registered report on 414 Xbox players found no meaningful within-person hours–well-being link | Ballou et al. 2025; Ballou et al. 2024 | weak–moderate |
| Harmonious vs. obsessive passion for play: wanting to play vs. feeling you have to; obsessive engagement goes with need frustration elsewhere | Przybylski et al. 2009; Vallerand et al. 2003 | moderate (correlational) |
| Problematic play tracks need frustration in daily life, and games that satisfy needs life does not (N = 1,029; cross-sectional) | Mills et al. 2018; Allen & Anderson 2018 | weak–moderate together |
| Dark patterns in games: grinding and playing by appointment (temporal), pay to skip (monetary), social obligation. Appointment content is not dark when optional for progression. The taxonomy is contested as lacking empirical grounding | Zagal, Björk & Lewis 2013; critique Deterding, Stenros & Montola 2020 | speculative (taxonomy) |
| FoMO is driven by low need satisfaction (β = −.25); in games it links to disorder through impulsivity and time, and to risky loot-box buying | Przybylski et al. 2013; Li et al. 2020; Hartanto & Kasturiratna 2024 | weak for games |
| Enforced playtime caps: after China's 2019 and 2021 under-18 limits, heavy play *rose* slightly (OR 1.14; 7 billion hours; preregistered; no ages in the data) | Zendle et al. 2023, *Nat Hum Behav* | moderate |
| In a game, a forced shutdown shortened sessions but upset players; a "fatigue" mechanic lengthened sessions, the opposite of its aim | Davies & Blake 2016 | weak |
| Gambling pop-ups move about 1% of sessions to stop; self-appraisal wording roughly doubles that (OR 2.13); opt-in personalized feedback reduced play for ~60% of users vs 50% by chance (self-selected) | Auer, Malischnig & Griffiths 2014; Auer & Griffiths 2015a, 2015b; Monaghan & Blaszczynski 2010 | weak |
| Spacing helps skill learning *in games*: 854,064 Axon players (observational), controlled CHI PLAY studies, breaks and checkpoints helped progress with no transfer cost | Stafford & Dewar 2014; Johanson et al. 2019; Piller et al. 2020; Johanson et al. 2023 | moderate |
| Deliberate practice explains 26% of performance variance in games, 4% in education (88 studies; definition disputed) | Macnamara, Hambrick & Oswald 2014 | strong but disputed |

#### What this means for katharsis

- **Hours are the wrong target in both directions.** Long play is not harm
  and not success. Track the feeling (RT13's worth-it key) and the days
  (RT9), and keep the objective on durable skill (GR7).
- **The risk here is "have to", not "want to".** The mechanics most likely
  to create "have to" are the ones that punish absence: streaks, decaying
  dumps, time-limited nightlies, guilt copy. GR1, GR2, GR8 remove them.
- **Breaks serve the goal.** This is the strongest evidence-backed argument
  in this doc: spacing helps learning in general and in games
  specifically. So the session budget (RT12) is framed as training
  ("tomorrow's boot learns more than tonight's third one"), not as a nanny.
- **Expect small effects from reminders, and none from locks.** RT12 is a
  one-line nudge at a stopping point with an easy shorter option, not a
  shutdown. Shutdowns upset players and caps did not reduce heavy play.
- **This player is low risk by the clinical bar, and the ask is
  different**: the failure mode to avoid is not disorder but regret
  (sessions that felt wasted). That is what worth-it measures.

## (c) Proposals for katharsis

Each proposal is sized for one implementation agent in one jj workspace
(D30). Format: claim → problem → solution → proof, then the build card.

**How to read the effect grades.** *Expected effect* is my estimate of the
change on the named metric for this player: **L** large, **M** medium,
**S** small, **?** unknown. It is an inference. *Evidence* is the grade of
the best source behind the mechanism (strong / moderate / weak /
speculative), as in §(b).

**What telemetry can and cannot prove here.** One player plays a few boots
a week. To see a return rate move from 0.5 to 0.7 at 80% power needs about
90 boots per arm (two-proportion test: 7.84 × (0.25 + 0.21) / 0.2² ≈ 90).
At four boots a week that is 22 weeks per arm. So the queries below are
**monitors**: they catch a guardrail breaking or a change going badly
wrong. They will not prove a medium effect. Rank by the evidence, ship,
and watch the monitors. The ML thread reached the same conclusion for
learning (`docs/research/ml-pipeline.md` §1.2).

Queries run on today's schema (`internal/store/migrate.go`) unless marked
*events*, which need the `events` table from ML migration 2
(`docs/research/ml-pipeline.md` §4.2). New event kinds this doc adds are
listed in RT13.

**Owners already in flight.** Several cards sit on work another thread owns.
Each card names who to coordinate with:

```
RT0 G1 aid          D34 + ML slice ML7 (governor, buy-back, +1 life)
RT2, RT3 economy    .agents/ws/a2-economy (D25): spend.go, apt.go, Packages
RT5 review stakes   D35 = ML decision (e), slice ML9
RT6 story           docs/story/story-bible.md (text, beat table); S9 / D39 (wiring)
RT13 monitor        ML M8 (48 h return hazard model)
debrief p̂           D36 = ML decision (g): predicted clear shown after the door
```

### RT0 · Precondition: hold G1 before adding any hook

**Claim.** Hooks amplify the loop they sit on. Today the loop's first
biome kills too often, and every hook below lands on a losing streak.

**Problem.** The balance report run for this doc (2026-10-14, synthetic
pool, standard rules, P0) gives P(crash in Userland) = 61.2% for a
1000-rated model player and 26.0% for a 1600-rated one, against the design's
15%. The ML thread measured 56.5% and 28.8% on the real union pool
(`ml-pipeline.md` §0). Loss streaks churn about twice as often as streaks
that end in a win (EOMM: LLL 5.1% vs LLW 2.6% seven-day churn). A crash at
an aim the player cannot reach is also the case where players blame the
game, which went with lower ratings in the one experiment on it (Juul 2009,
§(b)1b).

**Solution.** No new build here. D34 (+1 life when the forecast Userland
crash chance exceeds 0.20) and ML M4's easy slot without modifiers,
Userland buy-back and streak governor own it (slice ML7). RT8 adds only the
presentation.

**Proof.** G1 query (`ml-pipeline.md` §1.3) ≤ 0.20 on the last 30 boots
before RT2–RT7 ship.

### RT1 · The end of a boot says what persisted, one lever, and one next step

**Claim.** A crash should end on "here is a mistake you can fix, and how",
not on a verdict.

**Problem.** The crash screen ends with the quote and the dump line; the
clear screen with uptime and bytes. Neither shows the rating change, the
unlock progress, the regression date or the dump's fate (UX J1, J6, J7). A
death the player cannot act on invites "the game is too hard", the
attribution that went with lower ratings (Juul 2009), and at worst
helplessness (§(b)1b).

**Solution.** Three short blocks under the quote, in this order (the end
of an episode weighs most in how it is remembered: peak-end):

```
before                                     after
core dumped: 37 B on "Coder" ·             core dumped: 37 B on "Coder"
[u] upsolve [r] reboot [l] git log
                                           kept     rating 1108 → 1041 ▼67 (±90)
                                                    bank 607 B · apt strace 88 B to go
                                                    ↺ "Coder" comes back Oct 17
                                           lever    passed 51/52 · TLE on the largest
                                                    test → the idea worked; the
                                                    complexity did not · 62%
                                                    predicted (D36)
                                           next   ▸ u upsolve "Coder" untimed: +37 B
                                                    e editorial · r reboot ≈1h41 ·
                                                    s short ≈1h · g GRUB
```

- **kept**: what survived the crash. Never celebratory on a crash: no AC
  sound, no green (no "loss disguised as a win").
- **lever**: one line about the task, never about the player. It names a
  test, a time, a tool or a topic. Today's data supports: verdict, best
  pass k/N, failed submits, cursed, `:q!`, overtime. RT1b adds time to
  first save and to first all-samples pass once ML's `AttemptEvent`
  lands.
- **next**: one highlighted key. Priority: live dump → `u`; a topic with 2+
  crashes in 14 days and no capability → RTFM entry; otherwise `r`.
- The predicted clear chance appears here, after the door, as D36 decided:
  calibration feedback, never shown on the door.
- Easing this boot had (governor, buy-back, D34's life; RT8) is listed in
  `kept`, so a clear on an eased boot is not read as more skill than it
  shows.
- Clear screen: same blocks, plus up to two *first-time* or *personal best*
  lines (`first Heisenbug kill`, `fastest Userland 06:12`). Feedback on
  improvement over earlier attempts is among the strongest feedback
  moderators (d = .55, Kluger & DeNisi 1996), and positive verbal feedback
  raised free-choice motivation where expected tangible rewards lowered it
  (+0.33, Deci et al. 1999).

| | |
|---|---|
| Where | `internal/screens/boot/crash.go` (`elements`), `cleared.go` (`View`); a pure `game.Debrief(State, Meta) Debrief` so screens only render; additive `State.AtStart{Rating, TopicRating, Bank}` set in `NewBoot` (`./internal/game/engine.go:55`); lever copy in `internal/flavor` |
| Accept | goldens at 80x24 and 140x40 for timer, lives, cursed, `:q!`, dump overwritten, no editorial, clear; every lever string names a test, time, tool or topic (table test over causes); the highlighted key performs its action |
| Telemetry | `upsolved48h` and `replay10m` by cause (queries below); *events*: `boot_end_action{key, dwell_ms}` |
| Effect | **M** on upsolve within 48 h and on replay after a crash. Evidence: strong that task-level feedback helps and self-level feedback hurts (Kluger & DeNisi 1996); strong that a written plan helps follow-through (Gollwitzer & Sheeran 2006); weak that blaming a fixable mistake rather than the game raises ratings and retention (Juul 2009; Allart et al. 2017); theory for why (Weiner 1985) |
| Overlaps | UX J1, J6, J7, J12 (this is their content spec); neuro M4 |

```sql
-- replay within 10 minutes, by outcome and crash biome (today's schema)
WITH b AS (
  SELECT id, outcome, json_extract(state, '$.Biome') AS biome, ended,
         lead(started) OVER (ORDER BY id) AS next_start
  FROM boots)                    -- lead over every boot, then filter
SELECT outcome, biome, avg(next_start - ended < 600e9) AS replay10m, count(*)
FROM b WHERE outcome IN ('crashed', 'cleared') GROUP BY outcome, biome;

-- upsolve of the crash door within 48 h (Meta.Solves, today's schema)
SELECT avg(EXISTS (
  SELECT 1 FROM meta, json_each(meta.json, '$.Solves') s
  WHERE json_extract(s.value, '$.Upsolve')
    AND json_extract(s.value, '$.Problem') = b.last_door
    AND unixepoch(json_extract(s.value, '$.At')) * 1e9
        BETWEEN b.ended AND b.ended + 48 * 3.6e12)) AS upsolved48h
FROM boots b WHERE b.outcome = 'crashed';
```

Both queries ran against a copy of the UX tester's store
(`/tmp/ux-r/data/katharsis/katharsis.db`, 6 boots).

### RT2 · Spend now or commit: prove the fork A2 is building is a real choice (D25, boot half)

**Claim.** Commit should be a bet, not a chore.

**Problem.** On main, commit has no downside and the bank buys nothing
(`./internal/game/engine.go:590-608`; UX J2; neuro M2). Every merge window
has one right answer. The economy workspace is building the fork
(`.agents/ws/a2-economy`, D25, not merged): uncommitted bytes buy merge-
window items, and the sim's model player can spend.

```
a2-economy, as of 2026-10-14
  internal/game/spend.go     Spend{Item}: fsck +1 ♥ 64 B · dup +1 🦆 48 B ·
                             shuf (re-roll a room) 32 B; from Uncommitted only
  internal/game/apt.go       Install(m, pkg) with the bank, at GRUB (RT3)
  internal/game/sim/sim.go   Player.Spend: the model buys before it commits
```

Nothing yet tests whether the fork is real. A fork where one fixed policy
always wins is a chore with more keys.

**Solution.** No new mechanic. This card is the acceptance test and the
telemetry for A2's fork, run once A2 merges:

- A sim comparison of three policies on the standard preset at 1000 and
  1600: always commit; always spend when affordable; A2's adaptive model.
- The merge window says in one line what each choice protects:
  `c commit: safe forever · $ spend: this boot only`.
- Lives, timer and ducks are already covariates in ML M1 (`p̂_eff`), so
  spending does not corrupt the skill estimate.

| | |
|---|---|
| Where | `internal/game/sim` (policy variants, `balance_test.go` report lines), `internal/screens/boot/merge.go` (the one-line copy); prices stay rules settings, locked per boot (D2). **Coordinate with `.agents/ws/a2-economy`, which owns D25** |
| Accept | **neither fixed policy beats the adaptive one on both bank per hour and full-clear rate**, at both ratings. If one does, the fork is fake: retune `FsckPrice`, `DupPrice`, `ShufPrice` |
| Telemetry | share of merge windows with a spend; bytes spent vs committed per boot (`State.Spent` in `boots.state`); clear rate when spent vs not (descriptive) |
| Effect | **L** on "commit is a decision" (loop feel, D1); **?** on return. Evidence: Dead Cells' in-run shops vs the Collector's permanent unlocks (game observation, untested); choice at a risk point as autonomy (SDT, moderate; the mapping is inference) |

### RT3 · `apt` at GRUB: deepen the catalog on a goal gradient, options over power (D25, meta half)

**Claim.** The reason for one more boot is a visible unlock that one more
boot can buy, and what it buys must widen play, not replace skill.

**Problem.** On main the bank is a score (UX J2). A2 adds the sink
(`.agents/ws/a2-economy/internal/rules/rules.go`, `Packages`; `apt.go`,
`Install`). Its catalog as of today:

```
package      price   grants                         kind
cowsay       128 B   GRUB theme                     cosmetic
tux          128 B   GRUB theme                     cosmetic
rng-tools    256 B   daemon dev-random joins pool   option
rubber-duck  384 B   +1 duck at boot start          capped power
strace       512 B   daemon strace joins pool       option
procps       768 B   +1 daemon offered per window   option (needs rng-tools, strace)
binutils    1024 B   +1 lib link slot               capped power
total       3200 B
```

The start is right: a theme is affordable after boot 1, and the first
packages come one to three boots apart. Then the ladder stretches and ends.
The sim's P0 players commit 133–192 B per boot (balance report,
2026-10-14). Bought cheapest first at 133 B per boot, strace is about 4
boots past rubber-duck, procps about 6 and binutils about 8, and the whole
catalog is 24 boots (17 at 192 B). After that the bank is a score again,
before even the first new ring opens (boot 29 for the 1000-rated sim
player). Separately, power bought with bytes would
make clears come from purchases rather than skill. Of the two power
packages, rubber-duck is safe: ducks are an ML M1 covariate of `p̂_eff`.
binutils is not covered: M1's `p̂_eff` uses lives, ducks, timers and
modifiers, not libs (`ml-pipeline.md` M1). Either add linked libs as a
covariate or keep binutils the only package of its kind.

**Solution.** Extend A2's catalog and add the goal bar. Packages are mostly
**options**: things that join a pool or open a choice.

| Group | Examples to add | Power? |
|---|---|---|
| daemons | `perf` (per-sample time breakdown), `valgrind` (peak memory per sample), `tee` (side-by-side sample diff), `cron` (a due regression pays 2×) | sidegrades |
| upgrade variants | `duck+` (a third quack: the editorial's key observation, neuro H2), `overclock+`, `ecc+`; fixes UX J18 (the same three cards every time) | sidegrades |
| loadout | `init=`: start the boot with one owned daemon | capped power |
| cosmetic | banner art, quote packs, kernel version string, hostname | none |

- **Price ladder rule** (a sim test; prices stay rules settings): bought
  cheapest first, every package costs at most three boots of the
  1000-rated sim player's median commit (about 400 B at 133 B per boot),
  and the catalog lasts at least 40 boots (about 5,300 B at that rate, so
  14 or more packages; A2 has 7). That keeps the next package one to three
  boots away, where the goal-gradient effect is: purchase intervals
  shortened by about 20% approaching the reward (Kivetz et al. 2006). A
  capstone above the bound can come after boot 40.
- **The first package costs at most one boot.** This is the honest form of
  the endowed-progress effect: real progress, not a pre-filled bar. A2's
  128 B themes already meet it.
- **A goal bar** for the cheapest unowned package, or one the player pins,
  on GRUB, the merge window, and the crash/clear `kept` block:
  `apt strace ▓▓▓▓▓▓░░ 312/512 B · ≈2 boots`. The ETA is the player's own
  median committed per boot over the last 10 boots: honest, not
  motivational.
- **Recovered core dumps show their effect**: `u upsolve "Coder": +183 B
  → installs strace`. The learning act and the unlock become one key.
- **Not for sale:** story beats (RT6), streak protection, anything that
  changes a door's measured difficulty without entering `p̂_eff`.

| | |
|---|---|
| Where | after A2 merges: `internal/rules/rules.go` (`Standard().Packages`, ladder check in `Validate`), new daemons and upgrade variants in `internal/game/engine.go` (`enterMerge`, `offerUpgrades`), `internal/flavor/names.go` (names, one-line descriptions), `internal/screens/meta/grub.go` (goal bar on the status line, `./internal/screens/meta/grub.go:100` today), `internal/screens/boot/merge.go`, `crash.go`, `cleared.go` (goal bar in `kept`) |
| Accept | sim from an empty `Meta`: a package affordable after boot 1 in ≥ 80% of runs for a 1000-rated player; bought cheapest first, no package before boot 40 costs more than three of that player's median boots; the catalog lasts ≥ 40 boots at the P0 commit rate for both 1000 and 1600; the goal bar fits at 80x24 |
| Telemetry | boots between installs (needs an `apt_install` event: A2's `Meta.Installed` is a name list with no timestamps); commit share of earned bytes (`boots.committed` vs `State.Earned()`); boots per week before vs after ship (monitor only) |
| Effect | **L** on loop feel; **M** on return (inference from the role of Dead Cells' Collector and Hades' Mirror). Evidence: goal gradient moderate (Kivetz et al. 2006); endowed progress moderate (Nunes & Drèze 2006, conceptually replicated in Kivetz 2006); options over power is industry consensus, not tested |

### RT4 · The core dump is a cliffhanger that comes with its plan

**Claim.** The dump is katharsis's best hook: loss aversion pointed at the
learning act (neuro §4). It needs to be seen, protected, and paid.

**Problem.** It is overwritten silently (UX J7,
`./internal/game/engine.go:467`); `r` on the crash screen risks it with no
warning; GRUB lists it below the presets (`./internal/screens/meta/grub.go:118-123`);
an editorial upsolve recovers the same as an independent one
(`./internal/game/meta.go:20-37`).

**Solution.**

```
crash screen, live dump:   r → "reboot leaves 183 B at risk · r again · u upsolve first"
during the next boot:      status line carries ☠ 183 B dump
crash over a live dump:    headline "core dump lost: 183 B (Karen and Morning)"
GRUB:                      the dump entry sits directly under the default boot entry
recovery:                  1.0× after the editorial was opened, 1.5× if it never was
```

- 1.5× for an independent upsolve rewards the effortful act (learned
  industriousness, moderate) and gives "read, close, write" (neuro H4) a
  reason. The app already knows whether the editorial was seen
  (`./internal/app/intents.go:205`, `sawEditorial`), and `Upsolve` already
  takes the `SolveKind` (`./internal/game/meta.go:20`).
- The dump never expires with time. Only a crash destroys it. No calendar
  pressure (GR2).
- Why it is a *healthy* cliffhanger: the unfinished task comes with a
  concrete plan ("upsolve Karen and Morning"). Plans of that form raise
  follow-through (Gollwitzer & Sheeran 2006, strong), and a plan may also
  let an open goal stop intruding (Masicampo & Baumeister 2011, weak). The
  player can stop tonight; the game holds the thread.
- With A2's spend, bytes spent at a merge window leave the dump's reach.
  That is fine: spending is the other honest way to protect bytes.

| | |
|---|---|
| Where | `internal/game/meta.go` (`Upsolve`: recovery multiplier by kind, from rules), `internal/rules/rules.go` (`DumpIndependentMult`), `internal/screens/boot/crash.go` (double `r`), `hudbar.go`/header (☠ line), `internal/screens/meta/grub.go` (entry order) |
| Accept | unit tests: independent 1.5×, editorial 1.0×, exact bank arithmetic; `r r` needed with a live dump, one `r` without; goldens for the overwrite headline |
| Telemetry | `upsolved48h` (RT1); independent share of recoveries (`Meta.Solves[].Kind`); dumps lost vs recovered bytes (the sim reports both; real: `logf` lines in `boots.state` until *events*) |
| Effect | **M** on upsolve rate. Evidence: loss aversion moderate (λ ≈ 2, contested); resumption of interrupted tasks moderate (see §(b)2); plan-making strong in general, weak for the unfinished-goal version |

### RT5 · Regressions: the bug comes back, the stakes are low, the fix gets a beat

**Claim.** A regression should feel like an old enemy returning that you
now know how to beat, not like a trap.

**Problem.** On main, a failed regression door ends the boot like any door,
so the player will skip it (neuro R2). Clearing one produces no moment (UX
J5).

**Solution.** The stakes are settled: D35 makes a failed regression door
cost a life, not the boot (ML decision (e), built in slice ML9). This card
owns the rest.

- **Beat on clear:** `↺ fixed: "Coder" (stage 2) · next check Nov 13 · +64 B`
  on the dmesg strip (UX J5) and in git log.
- **Outside boots:** GRUB's `regressions due` becomes `cron jobs due: 2`;
  a non-editorial re-solve after the due date already counts
  (`./internal/game/meta.go:20-37`); pay it the door's bytes.
- **No backlog dread:** show at most 3 due items; the rest wait. Overdue
  items reschedule; there is no debt counter (GR10).

ML M6 owns *when* reviews come back and ML9 owns the stakes; this card owns
the feel and the pay.

| | |
|---|---|
| Where | `meta.go` (`advanceRegression` returns the next due date for the beat; cron-job pay in `Upsolve`), `doors.go` (`regressionDoor`, `./internal/game/doors.go:206`, for the stage shown on the card), `internal/screens/boot` (beat line on the dmesg strip, card), `internal/screens/meta/grub.go` (`cron jobs due`, cap of 3), `log.go` (beat in git log) |
| Accept | a cleared regression door emits one beat with stage and next date; a cron-job re-solve pays bytes once per stage, never for an editorial solve; GRUB golden with 10 due shows 3 |
| Telemetry | take rate of offered regression doors (*offers* table, ML §4.2); regression clear rate by stage; cron jobs done per week |
| Effect | **M** on take rate, most of it from D35's stakes; the beat and the pay add **S**. Evidence: strong that retrieval practice helps retention (Rowland 2014, verbal); moderate that low stakes raise approach (inference from avoidance under threat) |

### RT6 · Story pacing: one beat per boot end, and beats in the long gaps

**Claim.** The story pulls hardest when dying and learning advance it, when
each piece leaves a small question open, and when it keeps arriving after
the first evening.

**Problem.** The story bible already gets the triggers right
(`docs/story/story-bible.md`, "How the story unfolds"). Beats fire on
deaths and learning acts (first crash, dump recovered, dump lost, first
RTFM, first regression), never on dates. Hades does the same: characters
react whether it is "the first time or the 20th time". Three gaps remain:

```
gap                         evidence
front-loaded by design      "explain the plot after ten boots": most one-time beats fire by boot ~10
long dry spells after that  new story then comes from bosses and prestige reveals only;
                            sim rings open at boots 29, 45, 48, 92 (1000-rated) and 15, 18, 21, 28 (1600)
no pacing, no reread        the bible sets no per-boot limit and no way to reread a beat
```

For the 1000-rated player that leaves stretches of 16–44 boots with no new
ring (boots 10–29, 29–45, 48–92). The bosses fill some of it.

**Solution.** A pacing layer over the bible's beat table. The storyteller
writes the text; this card adds rules and triggers.

- **At most one new beat per boot end.** Extra beats queue for the next
  boot end. The beat shows as one teaser line in RT1's `kept` block with a
  key to read it. A beat waiting to be found gives the next boot a reason
  without costing tonight's sleep.
- **Milestone beats in the gaps**, keyed to cumulative learning acts, never
  to time: the 5th and 25th upsolve, the first regression retired, each
  capability earned, the first topic mastered (tags hidden, D24), and
  progress toward the next ring's 60-rated-door gate (D31). The storyteller
  picks which bible threads they carry.
- **Each one-time beat ends on a small question** that a later beat
  answers. Curiosity peaks when the answer feels close (Kang et al. 2009;
  Loewenstein 1994).
- **A reread log** in GRUB (`/proc/story`): nothing is missable.
- **Never** keyed to dates, streaks, logins or bytes (GR1, GR2, RT3).

| | |
|---|---|
| Where | `internal/flavor` (the bible's hook table; add a pure `NextBeat(State, Meta) (hook, bool)`), the bible's "seen" set on `Meta` (D39 proposes recording one-time scenes on `Meta`), `internal/screens/boot/crash.go` and `cleared.go` (teaser + key), a reader page under `internal/screens/meta` (reuse `screens/reader`). **Coordinate with the S9 wiring agent (D39)**: this card is pacing and triggers, not text or animation |
| Accept | selection is pure and deterministic; at most one new beat per boot end; a table test proves every trigger reachable; sim from an empty `Meta` at 1000 and 1600: no run of more than 8 boots (a setting) without a new beat through boot 100; grep test: no trigger references `time.Now`, a weekday or a date |
| Telemetry | *events*: `story_beat{id}`, `story_read{id, dwell_ms}`; return latency after boots that ended with a beat vs without (descriptive, confounded by outcome) |
| Effect | **M** on return (speculative). Evidence: curiosity improves memory for the answer and drives paying to see it (moderate, Gruber et al. 2014; Kang et al. 2009); story delivered through death is industry (Hades), not tested |

### RT7 · Right-size "one more boot"

**Claim.** "One more run" works when the next run is cheap. A standard
boot is not cheap.

**Problem.** In the balance report a standard boot averages 1h07m (1000
player) to 1h41m (1600 player); a clear takes 2h19m–2h20m. "One more" at 23:00
is a two-hour commitment, and it trades against spacing: distributed
practice beats massed practice (Cepeda et al. 2006, strong for verbal
recall). D13 (≈ 2 h full clears) is still *proposed*; the user asked for
2–3 h runs.

**Solution.** Keep standard. Add two built-in presets and offer them where
the decision is made:

```
preset    shape                                            aim (sim)
standard  2 Userland · 2 mid · boss                        ≈1–2.3 h (today)
short     2 Userland · 1 mid · boss, shorter timers        ≈1 h (presets/short.toml)
patch     1 Userland · 1 mid · mini-boss at 0.6            ≈25–40 min (new)
```

- Crash/clear footers: `r reboot ≈1h41 · s short ≈1h · p patch ≈30m`,
  each ETA the player's own median uptime for that preset (fallback: sim
  value).
- Boot #1 uses `short`, so the first session closes one full loop (RT11).
- **Decision for the user:** standard's length. Recommended default: keep
  it (2–3 h was the user's ask) and make short and patch first-class. The
  alternative is to shorten standard itself.

| | |
|---|---|
| Where | `internal/rules` (built-in `Short()`, `Patch()`; registry the app reads), `internal/app/app.go:115-120` (built-ins next to standard), `internal/screens/meta/grub.go`, `internal/screens/boot/crash.go`, `cleared.go` (footers), store query for median uptime per preset |
| Accept | balance report: patch mean uptime 25–40 min with door success in 0.75–0.85 (G3); GRUB lists the three; footers show ETAs |
| Telemetry | boots per session, sessions per week, uptime by preset (session query below) |
| Effect | **M** on sessions per week (inference). Evidence: strong for spacing over massing (learning); industry for short runs |

```sql
-- sessions: a gap over 30 min starts a new one (today's schema)
WITH b AS (
  SELECT id, started, ended,
         coalesce(started - lag(ended) OVER (ORDER BY id) > 1800e9, 1) AS new
  FROM boots WHERE ended IS NOT NULL),
s AS (SELECT *, sum(new) OVER (ORDER BY id) AS session FROM b)
SELECT session, count(*) AS boots,
       round((max(ended) - min(started)) / 3.6e12, 2) AS hours,
       strftime('%H:%M', min(started) / 1e9, 'unixepoch', 'localtime') AS start
FROM s GROUP BY session;
```

### RT8 · Easing is announced in task terms and declinable; comeback boots after a break

**Claim.** When the game eases off, it should say what changed and why, and
let the player decline, as Hades' God Mode does.

**Problem.** ML M4's streak governor and buy-back, and D34's +1 life,
change the aim without a word. The usual reason to hide adaptation, that
players feel cheated when they find out, has no peer-reviewed support; the
one direct test found that believing a game adapts *raised* immersion
(Denisova & Cairns 2015, §(b)1d). Hiding it has costs here: hidden
adjustment can leave players overconfident about their skill (Constant &
Levieux 2019, weak), and the clear is the competence signal katharsis
trains. This player is also the designer and will know anyway. Nothing
marks a return after a break either, when the aim is most likely wrong
(neuro R9).

**Solution.** Say it once, in task terms, with a decline key.

```
first session (once):   dmesg  doors are aimed at your rating; the aim adapts as you play
after 2 early crashes:  Boot #43 · Userland aim retuned · +1 ♥ (last 2 boots ended there) · x decline
after ≥ 3 days away:    GRUB  welcome back · 2 cron jobs due · ☠ 183 B dump · first doors aimed conservatively
```

- **Copy rule:** "retuned" and "aimed", never "easy mode", "assist" or
  "mercy". Celeste dropped "Cheat Mode" as judgmental; Kasavin framed God
  Mode as "a little bit tougher". The reason names the boots, not the
  player (§(b)1b).
- `x` at the first room declines the easing for this boot (logged).
- The comeback boot uses ML's conservative aim (θ − 1 SD) and says so. It
  never frames the time away as guilt (GR8).
- RT1's `kept` block lists any easing the boot had, with numbers. Before a
  door, the line names the change but no clear chance (D36).
- Announce only what happened. Denisova & Cairns got their effect by
  claiming adaptation that did not exist; doing that here breaks GR3.

| | |
|---|---|
| Where | `internal/game/engine.go` (`NewBoot` records reasons in `State.Ease []string`), `internal/rules/rules.go` (`ComebackDays` 3), `internal/screens/boot` header/dmesg strip, `internal/screens/meta/grub.go`, `internal/flavor` (the copy). Pairs with ML7, which builds the easing itself |
| Accept | engine test: 2 early crashes → reasons set; decline clears them and is logged; ≥ 3 days since last boot → comeback line and reason; table test: every reason string names boots, doors or days, and none contains "easy", "assist" or "mercy" |
| Telemetry | G2 streak length; return latency after early-crash streaks; decline rate; clear rate of the first 3 doors after a break, predicted vs observed |
| Effect | **M** on return after streaks. Evidence: strong as description that loss streaks churn (EOMM); moderate for paying a return after a miss (§(b)2); weak–moderate that announcing adaptation does no harm (Denisova & Cairns 2015); industry for the framing (God Mode, Assist Mode) |

### RT9 · `load average`, not streaks

**Claim.** Show consistency without a cliff.

**Problem.** A streak counter turns one missed day into a visible loss,
and broken streaks reduce later engagement (§(b)2). Missing a day does not
hurt habit formation (Lally et al. 2010). The game has no consistency
signal at all today.

**Solution.** The Linux `load average` idiom on GRUB's status line: the
share of days with practice over the last 7, 30 and 90 days.

```
before   bank 607 B · rating 1108 · next boot #43
after    bank 607 B · rating 1108 · next boot #43 · load average: 0.71, 0.53, 0.40
```

- A practice day is a day with a boot or an upsolve of ≥ 15 min (setting).
- A missed day lowers the 7-day figure by 1/7. Nothing ever resets to zero.
- `top` adds a 13-week strip of practice days, with no streak count.
- The word "streak" appears nowhere (grep test).

| | |
|---|---|
| Where | `internal/store/history.go` (`PracticeDays(since)`), `internal/screens/meta/grub.go:100` (status line), `top.go` |
| Accept | fixture store: values match a hand count; no string contains "streak" |
| Telemetry | the metric itself: days active per week (query below) |
| Effect | **S** (a reframe of existing data). Evidence: moderate that broken streaks demotivate (Silverman & Barasch 2023); weak–moderate that a missed day does not hurt habits (Lally et al. 2010) |

```sql
-- days active and hours per week (%W: Monday start; today's schema; boots only)
SELECT strftime('%Y-%W', started / 1e9, 'unixepoch', 'localtime') AS wk,
       count(DISTINCT date(started / 1e9, 'unixepoch', 'localtime')) AS days,
       round(sum(uptime) / 3.6e12, 2) AS hours
FROM boots GROUP BY wk;
```

### RT10 · Locked routes are visible inside the run

**Claim.** Dead Cells shows rune-locked paths before you own the rune (an
observation of the game; no developer source found). Seeing where you
cannot go yet is a goal.

**Problem.** Capabilities gate topic biomes (`./internal/game/capability.go:10`),
but a route choice shows only what is open. The player never sees what
RTFM would add.

**Solution.** At a route choice, a third card that cannot be picked:

```
╭─ 🔒 ───────────────────────────────╮
│ The Accumulator   prefix sums       │
│ RTFM Prefix Sums → CAP_PREFIX       │
│ b bookmark: GRUB opens it next      │
╰─────────────────────────────────────╯
```

- Drawn from gated topics with pool, by the same staleness weights as ML
  M5. It must use a fixed number of RNG draws so the open offers do not
  shift (`./internal/game/doors.go:295`).
- `b` queues the RTFM module as GRUB's second entry after the boot.

| | |
|---|---|
| Where | `internal/game/doors.go` (`routeOffers` returns a locked candidate), `types.go` (`BiomeState.Locked`), `internal/screens/boot/route.go`, `internal/screens/meta/grub.go` |
| Accept | the locked card never changes the open offers for a fixed seed (determinism test); it appears only when RTFM is wired and a gated topic has pool |
| Telemetry | trials started within 7 days of seeing a locked card (`TrialsPassed`, *events* `route_offered`) |
| Effect | **S–M** on RTFM uptake. Evidence: game observation, untested (Metroidvania gating); goal setting strong in general (Locke & Latham 2002) |

### RT11 · The first session closes one whole loop

**Claim.** The first session must reach fiero, a death that teaches, an
unlock, and a reason to come back, in about an hour.

**Problem.** A standard first boot can run two hours, and the UX tester's
first boss kill unlocked nothing (UX J1). With one player, there is exactly
one first session.

**Solution.** (UX's onboarding plan plus three rules.)

```
t≈0:05   first door: floor rating, tags shown, has an editorial       (UX)
t≈3:00   first AC beat · upgrades explained in one dmesg line          (UX, J5)
t≈8:00   first merge window: "commit or spend" in one line            (RT2)
first crash   lever + kept + next (RT1) · story.first-crash (story bible)
end of boot 1 first apt package affordable (RT3: ≤ 1 boot)
session end   "next: upsolve X · nightly 10-15 · cron jobs: 0"
```

- Boot #1 runs `short` (RT7).
- Tips are one-time `dmesg` lines, never modal. Tutorials paid off only in
  the most complex of three games (Foldit: up to +29% play time); in the
  two simpler ones they did nothing, and on-demand help hurt in one
  (Andersen et al. 2012). This player brings 400 h of Dead Cells, so the
  roguelike layer gets one line per idea and the words go to the CP layer.

| | |
|---|---|
| Where | `internal/game/engine.go` (`NewBoot` when `Meta.Boots == 0`), `doors.go` (first-door constraints), `internal/rules`, `internal/flavor` (tip lines), `Meta.Seen []string`, screens per UX |
| Accept | sim from an empty `Meta`: boot 1 mean uptime ≤ 70 min; first door P(clear) ≥ 0.95 by the engine's own estimate; a package affordable after boot 1 in ≥ 80% of runs |
| Telemetry | time to first AC; boots in session 1; return within 48 h (n = 1: a check, not a statistic) |
| Effect | **M** on the second session. Evidence: moderate (tutorials by game complexity; competence predicting continued play); industry |

### RT12 · A session budget and a bedtime, at boot boundaries only

**Claim.** The player wants CP to stick, not to lose nights. The game
should make stopping easy at good stopping points.

**Problem.** Nothing in the game knows how long tonight has been. Late,
massed sessions learn less per hour (spacing) and cost the sleep that
consolidates problem-solving gains (Wagner et al. 2004, moderate).

**Solution.** Two settings: `session_budget` (default 3 h of uptime per
day; a decision for the user) and `bedtime` (unset by default).

```
crash/clear footer, past budget:
  today 3:12 of 3:00 · dump saved · nightly 10-15 tomorrow · r reboot ≈1h41 · p patch ≈30m · q poweroff
GRUB, past bedtime:
  01:12 · past bedtime · patch is ≈30m           (cursor on patch, not standard)
```

- Shown only at boot boundaries and on GRUB, never inside a door; one line;
  never blocks; `r` still works. A hard cap would override autonomy, and
  enforced playtime caps have not shown effects (§(b)3).

| | |
|---|---|
| Where | user config (`internal/rules` or a small `config` file beside presets), `internal/app` (today's uptime from the store), crash/clear footers, `grub.go` default-entry choice |
| Accept | never rendered in `PhaseProblem`; one line at 80 columns; disable with `session_budget = 0`; *events* `nudge_shown`, `nudge_action` |
| Telemetry | stopped within 10 min after a nudge; sessions over budget per week; boots ending 00:00–05:00 (query below) |
| Effect | **S–M** on overlong sessions. Evidence: moderate–weak (gambling pop-ups and personalized feedback, §(b)3) |

```sql
-- share of boots ending between midnight and 05:00 local, per week (%W)
SELECT strftime('%Y-%W', ended / 1e9, 'unixepoch', 'localtime') AS wk,
       avg(CAST(strftime('%H', ended / 1e9, 'unixepoch', 'localtime') AS INT) < 5) AS late
FROM boots WHERE ended IS NOT NULL GROUP BY wk;
```

### RT13 · A retention monitor in `top`, and one "worth it?" key

**Claim.** The game can watch for compulsive drift the way it watches its
own calibration.

**Solution.** A `retention` panel in `top`: days active per week (8
weeks), sessions per week, median session length, median return latency,
replay-after-crash rate, upsolve-within-48h, late-night share, worth-it
ratio. At `poweroff`, at most once a day: `worth it? y / n · enter skips`.

ML M8 already plans a hazard model of "no boot within 48 h" as a monitor
that never steers doors. This panel is its display plus the one signal M8
lacks: whether the play felt worth it. Hours cannot stand in for that
(§(b)3).

New *events* kinds this doc needs (beyond ML §4.3): `boot_end_action`,
`shop_spend`, `apt_install`, `story_beat`, `story_read`, `nudge_shown`,
`nudge_action`, `governor_declined`, `worth_it`.

| | |
|---|---|
| Where | `internal/store/history.go` (queries), `internal/screens/meta/top.go`, `internal/app/app.go` (`quit`, the prompt), `events` table from ML migration 2 (slice ML2); coordinate with ML M8 |
| Accept | every metric matches a fixture; the prompt shows at most once per calendar day and `enter` skips |
| Telemetry | it is the telemetry; GR11 lists the alarm thresholds |
| Effect | enabling. The worth-it ratio is the alarm for "playing but not wanting to" |

### RT14 · An opt-in shell line when something is due

**Claim.** A terminal player's daily cue is a new shell.

**Solution.** `katharsis motd` prints one line, or nothing:

```
katharsis: 2 cron jobs due · ☠ 183 B dump on "Coder" · nightly 10-15
```

- Prints only when a review is due, a dump is live, or a nightly is new.
- Read-only, under 50 ms on a 1,000-boot store; the README shows the
  `~/.zshrc` line; off unless the player adds it.

| | |
|---|---|
| Where | `cmd/katharsis` (subcommand), `internal/store` (read-only open) |
| Accept | no output when nothing is due; never writes; timing test |
| Telemetry | return latency 4 weeks before vs after enabling (weak) |
| Effect | **S–M**. Evidence: moderate that stable context cues drive habits (Wood & Neal 2007); notification effects are from Duolingo, partly peer-reviewed (§(b)2) |

### RT15 · The nightly build has no fear of missing out

**Claim.** A daily seed is a fine cue; a daily seed you can lose is a
chore.

**Solution.** Keep the nightly (`./internal/screens/meta/grub.go:105-108`).
The first attempt on a seed is marked `first` in git log; later attempts
are practice. Past nightlies stay playable from git log. A personal best
per seed shape (biome reached, uptime) is shown on the entry, no streak.

| | |
|---|---|
| Where | `internal/screens/meta/grub.go`, `log.go`, `internal/store/history.go` |
| Accept | any past date's seed is playable; no copy says "today only" |
| Telemetry | nightly share of boots; days with a nightly |
| Effect | **S**. Evidence: weak (industry only) |

### Considered and rejected

| Idea | Why not |
|---|---|
| Streak counter, streak freeze, streak repair | loss-framed; broken streaks reduce engagement; turns practice into a debt (GR1) |
| Daily login rewards | "playing by appointment", a temporal dark pattern (Zagal et al. 2013) |
| Random byte payouts, loot crates bought with the bank | variable-ratio payout on effort; loot boxes correlate with problem gambling (GR5) |
| Story behind a byte paywall | makes lore a grind (RT6) |
| Hard session lockouts | no evidence they reduce heavy play; overrides autonomy (GR6) |
| "Rally": a quick AC after a WA refunds the life | rewards fast resubmits over thought; revisit after RT1b data |
| One attempt per nightly (Spelunky) | scarcity is fine for a game, but here it would pressure a learning session; first attempt is marked instead |
| Hide the governor and buy-back | no evidence that hiding helps; belief in adaptation raised immersion; hidden adjustment risks overconfidence (RT8) |
| "Adaptive AI" copy beyond what the aim does (the placebo) | raised immersion in the lab, but it is a lie, and the player is the designer (GR3) |
| Tune any payout to a 50% rate "because dopamine peaks there" | rests on one contested monkey ramp (§(b)1e); 0.5 is right for the boss on measurement grounds only |

### What remains uncertain

**Evidence that was not found** (the search was cut short; absence is not
shown):

- No peer-reviewed retention study of daily challenges, login rewards or
  daily quests, and no developer-published retention data for any daily
  run. RT15 rests on the absence of harm, not on a gain.
- No study of cliffhangers driving return. Ovsiankina resumption (moderate,
  with varied measures) is the closest mechanism for RT4 and RT6.
- No direct study of loss aversion behind streak loss or lost progress.
  "The dump points loss aversion at the learning act" is inference.
- Clark's finding that near misses motivate only under personal control is
  a single study; no replication found.
- Fiero has no peer-reviewed validation.
- No replication found for Masicampo & Baumeister 2011 or for Nunes &
  Drèze 2006 (beyond Kivetz's conceptual one).
- Knowing that a game adapts: one study (two small samples, one game, a
  placebo design), none in a learning game. RT8's copy is safe on that
  evidence, not proven good.
- No developer-stated run lengths for Hades, Dead Cells, Slay the Spire,
  Spelunky or Balatro. RT7's ETAs come from the sim and the player's own
  medians.
- Details taken from secondary sources: Kivetz's purchase numbers (the PDF
  was a scan), Keller & Bless's Tetris design, Nunes & Drèze's completion
  times, Lally's 66-day median, the Celeste postcard text.

**Transfer.** No study here used competitive programming or a learning
roguelike. The largest jump (inference): in the entertainment games behind
§(b)1, a death costs nothing outside the game. Here a crash costs an hour
of real study. That should make RT0 and RT1 matter more than the game
literature suggests, and hooks that lengthen sessions matter less.

**Power.** One player cannot confirm a medium effect on any card
(§(c) intro). The monitors catch breakage. They cannot rank the cards.

**Decisions for the user**, each with a recommended default:

| # | Question | Default |
|---|---|---|
| D13 / RT7 | Keep standard at about 1–2.3 h, or shorten it? | keep it (the 2–3 h ask); make `short` and `patch` first-class |
| RT12 | Session budget | 3 h of uptime per day, a nudge only; bedtime unset |
| RT4 | Pay 1.5× for an upsolve done without the editorial? | yes, as a rules setting |
| RT3 | Catalog depth | at least 40 boots of packages at the P0 commit rate |

## (d) Guardrails

The line between compelling and compulsive, per §(b)3: **wanting to play
vs. feeling you have to**. Harmonious engagement goes with need
satisfaction and better outcomes; obsessive engagement goes with need
frustration elsewhere and worse ones. Hours alone predict little. So the
guardrails protect autonomy and honesty, and they watch the *feeling*
(worth it?) more than the hours.

| # | Guardrail | Enforced by | Test |
|---|---|---|---|
| GR1 | No streak counters, streak freezes, streak repair, or daily-login rewards | design rule | grep: no "streak" in UI strings |
| GR2 | No calendar pressure on learning content: nightlies stay playable later; dumps and regressions never expire by date; story beats never keyed to dates | RT4, RT6, RT15 | engine/flavor tests: no `time.Now` or weekday in triggers |
| GR3 | Never fabricate or inflate: no invented near misses; no win cues on a crash; the predicted clear chance shown after a door (D36) includes curses and the timer (UX J15); adaptation is announced only when it happened (no placebo "adaptive AI" copy); every ETA is the player's own median | RT1, RT3, RT8 | golden tests; ETA computed from the store; each easing line traces to a `State.Ease` reason |
| GR4 | Bytes pay for learning acts only: doors, upsolves, regressions. Never for logging in, idling or hours played | RT2–RT5 | unit tests on every `Bank +=` site |
| GR5 | Randomness lives in *what is offered* (doors, routes, daemons, upgrades), never in *whether effort pays*: equal expected bytes per door stays (`doorBytes`) | economy | sim: mean bytes per door flat across difficulty |
| GR6 | Nudges only at safe points (boot boundaries, GRUB, the doors screen), one line, never blocking, every one tunable or off; no hard lockouts | RT12 | screen tests: no nudge in `PhaseProblem` |
| GR7 | Engagement metrics never steer doors and are never the objective; the objective stays durable skill per week (`ml-pipeline.md` §1.2, M8) | ML | code review |
| GR8 | No guilt or social-pressure copy ("the rabbit misses you"); a return gets what is due and an easier first door | RT8, flavor | copy review |
| GR9 | Every easing is announced, in task terms, and declinable: governor, buy-back, D34's +1 life, comeback aim | RT8, ML7 | engine tests; copy table test (no "easy", "assist", "mercy") |
| GR10 | Review backlog is capped on screen (≤ 3); overdue items reschedule; no debt counter | RT5 | GRUB golden with 10 due |
| GR11 | Alarms in `top`, shown once each, never blocking: worth-it "no" ≥ 1/3 over 14 days; ≥ 2 boots ending after 00:00 in a week; 3+ sessions over budget in a week; hours per week rising while worth-it falls | RT13 | fixture stores trip each alarm |
| GR12 | Telemetry stays local and visible: the SQLite store and `top`; nothing leaves the machine | store | no network code in store or app telemetry paths |

What this player asked for decides the defaults: make CP stick, not waste
time; the loop should feel like a roguelike (D1). That means more *days*
with practice rather than longer nights, and a game that is easy to stop
because it holds the thread (dump, cron jobs, story) for tomorrow.

## (e) References

Grades are given where each source is used above. "Abstract" marks sources
checked at abstract level only; "secondary" marks numbers taken from a
secondary source; "industry" marks sources that are not peer reviewed.
Bibliographic details were checked against Crossref, Europe PMC or the
publisher.

**Peer-reviewed and academic sources**

- Abuhamdeh & Csikszentmihalyi (2012). The importance of challenge for the enjoyment of intrinsically motivated, goal-directed activities. *Personality and Social Psychology Bulletin* 38(3):317–330. https://doi.org/10.1177/0146167211427147 (abstract)
- Alaybek, Dalal, Fyffe, Aitken, Zhou, Qu, Roman & Baines (2022). All's well that ends (and peaks) well? A meta-analysis of the peak-end rule and duration neglect. *Organizational Behavior and Human Decision Processes* 170:104149. https://doi.org/10.1016/j.obhdp.2022.104149
- Allart, Levieux, Pierfitte, Guilloux & Natkin (2017). Difficulty influence on motivation over time in video games using survival analysis. *FDG '17*. https://doi.org/10.1145/3102071.3102085 (abstract)
- Allen & Anderson (2018). Satisfaction and frustration of basic psychological needs in the real world and in video games predict internet gaming disorder scores and well-being. *Computers in Human Behavior* 84:220–229. https://doi.org/10.1016/j.chb.2018.02.034 (secondary abstract; N not verified)
- Andersen, O'Rourke, Liu, Snider, Lowdermilk, Truong, Cooper & Popović (2012). The impact of tutorials on games of varying complexity. *CHI 2012*:59–68. https://doi.org/10.1145/2207676.2207687
- Auer, Malischnig & Griffiths (2014). Is "pop-up" messaging in online slot machine gambling effective as a responsible gambling strategy? *Journal of Gambling Issues* 29. https://doi.org/10.4309/jgi.2014.29.3 (authors have industry ties)
- Auer & Griffiths (2015a). Testing normative and self-appraisal feedback in an online slot-machine pop-up in a real-world setting. *Frontiers in Psychology* 6:339. https://doi.org/10.3389/fpsyg.2015.00339
- Auer & Griffiths (2015b). The use of personalized behavioral feedback for online gamblers: an empirical study. *Frontiers in Psychology* 6:1406. https://doi.org/10.3389/fpsyg.2015.01406
- Ballou, Sewall, Ratcliffe, Zendle, Tokarchuk & Deterding (2024). Registered report evidence suggests no relationship between objectively tracked video game playtime and well-being over 3 months. *Technology, Mind, and Behavior* 5(1):28–42. https://doi.org/10.1037/tmb0000124 (abstract)
- Ballou, Vuorre, Hakman, Magnusson & Przybylski (2025). Perceived value of video games, but not hours played, predicts mental well-being in casual adult Nintendo players. *Royal Society Open Science* 12(3):241174. https://doi.org/10.1098/rsos.241174
- Bandura (1977). Self-efficacy: toward a unifying theory of behavioral change. *Psychological Review* 84(2):191–215. https://doi.org/10.1037/0033-295X.84.2.191
- Barton, Yazdani, Ayer, Kalvapalle, Brown, Stapleton, Brown & Harrigan (2017). The effect of losses disguised as wins and near misses in electronic gaming machines: a systematic review. *Journal of Gambling Studies* 33(4):1241–1260. https://doi.org/10.1007/s10899-017-9688-0 (abstract)
- Bauckhage, Kersting, Sifa, Thurau, Drachen & Canossa (2012). How players lose interest in playing a game: an empirical study based on distributions of total playing times. *IEEE CIG 2012*:139–146. https://doi.org/10.1109/CIG.2012.6374148
- Berridge & Robinson (1998). What is the role of dopamine in reward: hedonic impact, reward learning, or incentive salience? *Brain Research Reviews* 28(3):309–369. https://doi.org/10.1016/s0165-0173(98)00019-8 (abstract)
- Berridge & Robinson (2016). Liking, wanting, and the incentive-sensitization theory of addiction. *American Psychologist* 71(8):670–679. https://doi.org/10.1037/amp0000059
- Boksem, Meijman & Lorist (2005). Effects of mental fatigue on attention: an ERP study. *Cognitive Brain Research* 25(1):107–116. https://doi.org/10.1016/j.cogbrainres.2005.04.011 (abstract)
- Bopp, Mekler & Opwis (2016). Negative emotion, positive experience? Emotionally moving moments in digital games. *CHI 2016*:2996–3006. https://doi.org/10.1145/2858036.2858227
- Bopp, Opwis & Mekler (2018). "An odd kind of pleasure": differentiating emotional challenge in digital games. *CHI 2018*. https://doi.org/10.1145/3173574.3173615 (abstract)
- Brown, Imai, Vieider & Camerer (2024). Meta-analysis of empirical estimates of loss aversion. *Journal of Economic Literature* 62(2):485–516. https://doi.org/10.1257/jel.20221698 (abstract)
- Carter, Gibbs & Wadley (2013). Death and dying in DayZ. *IE '13*. https://doi.org/10.1145/2513002.2513013
- Cepeda, Pashler, Vul, Wixted & Rohrer (2006). Distributed practice in verbal recall tasks: a review and quantitative synthesis. *Psychological Bulletin* 132(3):354–380. https://pubmed.ncbi.nlm.nih.gov/16719566/
- Chen, J. (2007). Flow in games (and everything else). *Communications of the ACM* 50(4):31–34. https://doi.org/10.1145/1232743.1232769 (viewpoint); thesis: http://www.jenovachen.com/flowingames/Flow_in_games_final.pdf
- Chen, Xue, Kolen, Aghdaie, Zaman, Sun & Seif El-Nasr (2017). EOMM: an engagement optimized matchmaking framework. *WWW 2017*:1143–1150. https://doi.org/10.1145/3038912.3052559
- Clark, Lawrence, Astley-Jones & Gray (2009). Gambling near-misses enhance motivation to gamble and recruit win-related brain circuitry. *Neuron* 61(3):481–490. https://doi.org/10.1016/j.neuron.2008.12.031
- Clay, Mlynski, Korb, Goschke & Job (2022). Rewarding cognitive effort increases the intrinsic value of mental labor. *PNAS* 119(5). https://doi.org/10.1073/pnas.2111785119 (via the motivation batch; abstract)
- Constant & Levieux (2019). Dynamic difficulty adjustment impact on players' confidence. *CHI 2019*. https://doi.org/10.1145/3290605.3300693 (abstract)
- Dai, Milkman & Riis (2014). The fresh start effect: temporal landmarks motivate aspirational behavior. *Management Science* 60(10):2563–2582. https://doi.org/10.1287/mnsc.2014.1901
- Deci, Koestner & Ryan (1999). A meta-analytic review of experiments examining the effects of extrinsic rewards on intrinsic motivation. *Psychological Bulletin* 125(6):627–668. https://doi.org/10.1037/0033-2909.125.6.627 (abstract)
- Denisova & Cairns (2015). The placebo effect in digital games: phantom perception of adaptive artificial intelligence. *CHI PLAY 2015*:23–33. https://doi.org/10.1145/2793107.2793109
- Deterding, Stenros & Montola (2020). Against "dark game design patterns". *DiGRA 2020* (extended abstract; not retrieved through Crossref)
- Dixon, Harrigan, Sandhu, Collins & Fugelsang (2010). Losses disguised as wins in modern multi-line video slot machines. *Addiction* 105(10):1819–1824. https://doi.org/10.1111/j.1360-0443.2010.03050.x
- Egerton, Mehta, Montgomery, Lappin, Howes, Reeves, Cunningham & Grasby (2009). The dopaminergic basis of human behaviors: a review of molecular imaging studies. *Neuroscience & Biobehavioral Reviews* 33:1109–1132. https://doi.org/10.1016/j.neubiorev.2009.05.005 ; full text https://pmc.ncbi.nlm.nih.gov/articles/PMC3797507/
- Eisenberger (1992). Learned industriousness. *Psychological Review* 99(2):248–267. https://doi.org/10.1037/0033-295X.99.2.248
- Engeser & Rheinberg (2008). Flow, performance and moderators of challenge-skill balance. *Motivation and Emotion*. https://doi.org/10.1007/s11031-008-9102-4
- Ferreri, Mas-Herrero, Zatorre, Ripollés, Gomez-Andres, Alicart, Olivé, Marco-Pallarés, Antonijoan, Valle, Riba & Rodriguez-Fornells (2019). Dopamine modulates the reward experiences elicited by music. *PNAS* 116:3793. https://doi.org/10.1073/pnas.1811878116
- Fiorillo, Tobler & Schultz (2003). Discrete coding of reward probability and uncertainty by dopamine neurons. *Science* 299(5614):1898–1902. https://doi.org/10.1126/science.1077349
- Fogg (2009). A behavior model for persuasive design. *Persuasive '09*. https://doi.org/10.1145/1541948.1541999 (conceptual; never tested)
- Fong, Zaleski & Leach (2015). The challenge–skill balance and antecedents of flow: a meta-analytic investigation. *Journal of Positive Psychology* 10(5):425–446. https://doi.org/10.1080/17439760.2014.967799 (abstract)
- Gal & Rucker (2018). The loss of loss aversion: will it loom larger than its gain? *Journal of Consumer Psychology* 28(3):497–516. https://doi.org/10.1002/jcpy.1047 (abstract)
- Garea, Drummond, Sauer, Hall & Williams (2021). Meta-analysis of the relationship between problem gambling, excessive gaming and loot box spending. *International Gambling Studies* 21(3):460–479. https://doi.org/10.1080/14459795.2021.1914705
- Garrison, Erdeniz & Done (2013). Prediction error in reinforcement learning: a meta-analysis of neuroimaging studies. *Neuroscience & Biobehavioral Reviews* 37:1297–1310. https://doi.org/10.1016/j.neubiorev.2013.03.023
- Ghibellini & Meier (2025). Interruption, recall and resumption: a meta-analysis of the Zeigarnik and Ovsiankina effects. *Humanities and Social Sciences Communications* 12:962. https://doi.org/10.1057/s41599-025-05000-w
- Gollwitzer & Sheeran (2006). Implementation intentions and goal achievement: a meta-analysis of effects and processes. *Advances in Experimental Social Psychology* 38:69–119. https://doi.org/10.1016/S0065-2601(06)38002-1 (abstract)
- Green & Brock (2000). The role of transportation in the persuasiveness of public narratives. *Journal of Personality and Social Psychology* 79(5):701–721. https://doi.org/10.1037/0022-3514.79.5.701
- Gruber, Gelman & Ranganath (2014). States of curiosity modulate hippocampus-dependent learning via the dopaminergic circuit. *Neuron* 84(2):486–496. https://doi.org/10.1016/j.neuron.2014.08.060
- Hagger, Chatzisarantis, Alberts et al. (2016). A multilab preregistered replication of the ego-depletion effect. *Perspectives on Psychological Science* 11(4):546–573. https://doi.org/10.1177/1745691616652873
- Hamid, Pettibone, Mabrouk, Hetrick, Schmidt, Vander Weele, Kennedy, Aragona & Berke (2016; online 2015). Mesolimbic dopamine signals the value of work. *Nature Neuroscience* 19(1):117–126. https://doi.org/10.1038/nn.4173
- Hartanto & Kasturiratna (2024). Longitudinal bidirectional relation between fear of missing out and risky loot box consumption. *Computers in Human Behavior Reports* 16:100535. https://doi.org/10.1016/j.chbr.2024.100535
- Hattie & Timperley (2007). The power of feedback. *Review of Educational Research* 77(1):81–112. https://doi.org/10.3102/003465430298487
- Howard-Jones & Demetriou (2009). Uncertainty and engagement with learning games. *Instructional Science* 37(6):519–536. https://doi.org/10.1007/s11251-008-9073-6 (findings via the authors' 2011 summary)
- Humphreys (1939). The effect of random alternation of reinforcement on the acquisition and extinction of conditioned eyelid reactions. *Journal of Experimental Psychology* 25:141–158. https://doi.org/10.1037/h0058138 (secondary)
- Hunicke (2005). The case for dynamic difficulty adjustment in games. *ACE '05*:429–433. https://doi.org/10.1145/1178477.1178573 (abstract)
- Inzlicht, Shenhav & Olivola (2018). The effort paradox: effort is both costly and valued. *Trends in Cognitive Sciences* 22(4):337–349. https://doi.org/10.1016/j.tics.2018.01.007
- James, O'Malley & Tunney (2016). Why are some games more addictive than others: the effects of timing and payoff on perseverance in a slot machine game. *Frontiers in Psychology* 7:46. https://doi.org/10.3389/fpsyg.2016.00046
- Jang, Nassar, Dillon & Frank (2019). Positive reward prediction errors during decision-making strengthen memory encoding. *Nature Human Behaviour* 3(7):719–732. https://doi.org/10.1038/s41562-019-0597-3 (abstract)
- Jeong, Taylor, Floeder, Lohmann, Mihalas, Wu, Zhou, Burke & Namboodiri (2022). Mesolimbic dopamine release conveys causal associations. *Science* 378:eabq6740. https://doi.org/10.1126/science.abq6740
- Johannes, Vuorre & Przybylski (2021). Video game play is positively correlated with well-being. *Royal Society Open Science* 8(2):202049. https://doi.org/10.1098/rsos.202049
- Johanson, Gutwin, Bowey & Mandryk (2019). Press pause when you play: comparing spaced practice intervals for skill development in games. *CHI PLAY 2019*:169–184. https://doi.org/10.1145/3311350.3347195
- Johanson, Piller, Gutwin & Mandryk (2023). If at first you don't succeed: helping players make progress in games with breaks and checkpoints. *PACM HCI* 7(CHI PLAY):342–368. https://doi.org/10.1145/3611033
- Juul (2009). Fear of failing? The many meanings of difficulty in video games. In Perron & Wolf (eds.), *The Video Game Theory Reader 2*, Routledge, 237–252. https://www.jesperjuul.net/text/fearoffailing/
- Kahneman, Fredrickson, Schreiber & Redelmeier (1993). When more pain is preferred to less: adding a better end. *Psychological Science* 4(6):401–405. https://doi.org/10.1111/j.1467-9280.1993.tb00589.x
- Kang, Hsu, Krajbich, Loewenstein, McClure, Wang & Camerer (2009). The wick in the candle of learning: epistemic curiosity activates reward circuitry and enhances memory. *Psychological Science* 20(8):963–973. https://doi.org/10.1111/j.1467-9280.2009.02402.x
- Keller, J. & Bless (2008; online 2007). Flow and regulatory compatibility: an experimental approach to the flow model of intrinsic motivation. *Personality and Social Psychology Bulletin* 34(2):196–209. https://doi.org/10.1177/0146167207310026 (Tetris details secondary)
- Keller, Salvi, Leiker, Gruber & Dunsmoor (2024). States of epistemic curiosity interfere with memory for incidental scholastic facts. *npj Science of Learning* 9. https://doi.org/10.1038/s41539-024-00234-w
- Keogh (2013). When game over means game over: using permanent death to craft living stories in Minecraft. *IE '13*. https://doi.org/10.1145/2513002.2513572
- Kivetz, Urminsky & Zheng (2006). The goal-gradient hypothesis resurrected: purchase acceleration, illusionary goal progress, and customer retention. *Journal of Marketing Research* 43(1):39–58. https://doi.org/10.1509/jmkr.43.1.39 (abstract; numbers secondary, via Chicago Booth Review: https://www.chicagobooth.edu/review/going-goal)
- Kluger & DeNisi (1996). The effects of feedback interventions on performance: a historical review, a meta-analysis, and a preliminary feedback intervention theory. *Psychological Bulletin* 119(2):254–284. https://doi.org/10.1037/0033-2909.119.2.254
- Koepp, Gunn, Lawrence, Cunningham, Dagher, Jones, Brooks, Bench & Grasby (1998). Evidence for striatal dopamine release during a video game. *Nature* 393:266–268. https://doi.org/10.1038/30498
- Kool, McGuire, Rosen & Botvinick (2010). Decision making and the avoidance of cognitive demand. *JEP: General* 139(4):665–682. https://doi.org/10.1037/a0020198
- Lally, van Jaarsveld, Potts & Wardle (2010; online 2009). How are habits formed: modelling habit formation in the real world. *European Journal of Social Psychology* 40(6):998–1009. https://doi.org/10.1002/ejsp.674
- Larche, Musielak & Dixon (2017; online 2016). The Candy Crush sweet tooth: how "near-misses" in Candy Crush increase frustration, and the urge to continue gameplay. *Journal of Gambling Studies* 33(2):599–615. https://doi.org/10.1007/s10899-016-9633-7
- Laskowski, Dorchak, Ward, Christensen & Euston (2019). Can slot-machine reward schedules induce gambling addiction in rats? *Journal of Gambling Studies* 35:887–914. https://doi.org/10.1007/s10899-019-09852-1
- Lazzaro (2004). Why we play games: four keys to more emotion without story. XEODesign white paper, GDC 2004. https://www.xeodesign.com/xeodesign_whyweplaygames.pdf (industry)
- Li, Griffiths, Niu & Mei (2020). Fear of missing out (FoMO) and gaming disorder among Chinese university students: impulsivity and game time as mediators. *Issues in Mental Health Nursing* 41(12):1104–1113. https://doi.org/10.1080/01612840.2020.1774018
- Locke & Latham (2002). Building a practically useful theory of goal setting and task motivation: a 35-year odyssey. *American Psychologist* 57(9):705–717. https://doi.org/10.1037/0003-066X.57.9.705
- Loewenstein (1994). The psychology of curiosity: a review and reinterpretation. *Psychological Bulletin* 116(1):75–98. https://doi.org/10.1037/0033-2909.116.1.75
- Lomas, Patel, Forlizzi & Koedinger (2013). Optimizing challenge in an educational game using large-scale design experiments. *CHI 2013*:89–98. https://doi.org/10.1145/2470654.2470668
- Lomas, Koedinger, Patel, Shodhan, Poonwala & Forlizzi (2017). Is difficulty overrated? The effects of choice, novelty and suspense on intrinsic motivation in educational games. *CHI 2017*:1028–1039. https://doi.org/10.1145/3025453.3025638 (abstract)
- Macnamara, Hambrick & Oswald (2014). Deliberate practice and performance in music, games, sports, education, and professions: a meta-analysis. *Psychological Science* 25(8):1608–1618. https://doi.org/10.1177/0956797614535810
- Maier & Seligman (2016). Learned helplessness at fifty: insights from neuroscience. *Psychological Review* 123(4):349–367. https://doi.org/10.1037/rev0000033
- Masicampo & Baumeister (2011). Consider it done! Plan making can eliminate the cognitive effects of unfulfilled goals. *Journal of Personality and Social Psychology* 101(4):667–683. https://doi.org/10.1037/a0024192 (abstract)
- McGraw & Fiala (1982). Undermining the Zeigarnik effect: another hidden cost of reward. *Journal of Personality* 50(1):58–66. https://doi.org/10.1111/j.1467-6494.1982.tb00745.x (finding as reported by Ghibellini & Meier 2025)
- Milkman, Gromet, Ho, Kay, Lee, Pandiloski, Park, Rai, Bazerman, Beshears et al. (2021). Megastudies improve the impact of applied behavioural science. *Nature* 600:478–483. https://doi.org/10.1038/s41586-021-04128-4
- Mills, Milyavskaya, Heath & Derevensky (2018; online 2017). Gaming motivation and problematic video gaming: the role of needs frustration. *European Journal of Social Psychology* 48(4):551–559. https://doi.org/10.1002/ejsp.2343 (abstract)
- Monaghan & Blaszczynski (2010). Impact of mode of display and message content of responsible gambling signs for electronic gaming machines on regular gamblers. *Journal of Gambling Studies* 26(1):67–88. https://doi.org/10.1007/s10899-009-9150-z
- Niv, Duff & Dayan (2005). Dopamine, uncertainty and TD learning. *Behavioral and Brain Functions* 1:6. https://doi.org/10.1186/1744-9081-1-6
- Nunes & Drèze (2006). The endowed progress effect: how artificial advancement increases effort. *Journal of Consumer Research* 32(4):504–512. https://doi.org/10.1086/500480 (abstract; numbers secondary)
- Ovsiankina (1928). Die Wiederaufnahme unterbrochener Handlungen. *Psychologische Forschung* 11:302–379. https://doi.org/10.1007/BF00410261
- Ozcelik, Cagiltay & Ozcelik (2013). The effect of uncertainty on learning in game-like environments. *Computers & Education* 67:12–20. https://doi.org/10.1016/j.compedu.2013.02.009 (abstract)
- Palmer, Ferrari & Clark (2024). The near-miss effect in online slot machine gambling: a series of conceptual replications. *Psychology of Addictive Behaviors* 38(6):716–727. https://doi.org/10.1037/adb0000999 (abstract)
- Petralito, Brühlmann, Iten, Mekler & Opwis (2017). A good reason to die: how avatar death and high challenges enable positive experiences. *CHI 2017*:5087–5097. https://doi.org/10.1145/3025453.3026047 (abstract)
- Piller, Johanson, Phillips, Gutwin & Mandryk (2020). Is a change as good as a rest? Comparing break types for spaced practice in a platformer game. *CHI PLAY 2020*. https://doi.org/10.1145/3410404.3414225
- Pisklak, Yong & Spetch (2020; online 2019). The near-miss effect in slot machines: a review and experimental analysis over half a century later. *Journal of Gambling Studies* 36(2):611–632. https://doi.org/10.1007/s10899-019-09891-8
- Pool, Sennwald, Delplanque, Brosch & Sander (2016). Measuring wanting and liking from animals to humans: a systematic review. *Neuroscience & Biobehavioral Reviews* 63:124–142. https://doi.org/10.1016/j.neubiorev.2016.01.006 (abstract)
- Przybylski, Murayama, DeHaan & Gladwell (2013). Motivational, emotional, and behavioral correlates of fear of missing out. *Computers in Human Behavior* 29(4):1841–1848. https://doi.org/10.1016/j.chb.2013.02.014
- Przybylski, Rigby & Ryan (2010). A motivational model of video game engagement. *Review of General Psychology* 14(2):154–166. https://doi.org/10.1037/a0019440 (abstract)
- Przybylski, Weinstein & Murayama (2017). Internet gaming disorder: investigating the clinical relevance of a new phenomenon. *American Journal of Psychiatry* 174(3):230–236. https://doi.org/10.1176/appi.ajp.2016.16020224 (abstract)
- Przybylski, Weinstein, Ryan & Rigby (2009). Having to versus wanting to play: background and consequences of harmonious versus obsessive engagement in video games. *CyberPsychology & Behavior* 12(5):485–492. https://doi.org/10.1089/cpb.2009.0083 (abstract)
- Ravaja, Saari, Salminen, Laarni & Kallinen (2006). Phasic emotional reactions to video game events: a psychophysiological investigation. *Media Psychology* 8(4):343–367. https://doi.org/10.1207/s1532785xmep0804_2 (abstract)
- Redelmeier, Katz & Kahneman (2003). Memories of colonoscopy: a randomized trial. *Pain* 104:187–194. https://doi.org/10.1016/S0304-3959(03)00003-4
- Renfree, Harrison, Marshall, Stawarz & Cox (2016). Don't kick the habit: the role of dependency in habit formation apps. *CHI 2016 Extended Abstracts*:2932–2939. https://doi.org/10.1145/2851581.2892495
- Rouhani, Norman & Niv (2018). Dissociable effects of surprising rewards on learning and memory. *JEP: LMC* 44(9):1430–1443. https://doi.org/10.1037/xlm0000518 (abstract)
- Rowland (2014). The effect of testing versus restudy on retention: a meta-analytic review of the testing effect. *Psychological Bulletin* 140(6):1432–1463. https://doi.org/10.1037/a0037559
- Ryan, Rigby & Przybylski (2006). The motivational pull of video games: a self-determination theory approach. *Motivation and Emotion* 30(4):344–360. https://doi.org/10.1007/s11031-006-9051-8
- Sailer & Homner (2020; online 2019). The gamification of learning: a meta-analysis. *Educational Psychology Review* 32:77–112. https://doi.org/10.1007/s10648-019-09498-w
- Schultz, Dayan & Montague (1997). A neural substrate of prediction and reward. *Science* 275(5306):1593–1599. https://doi.org/10.1126/science.275.5306.1593 (abstract)
- Silverman & Barasch (2023; online 2022). On or off track: how (broken) streaks affect consumer decisions. *Journal of Consumer Research* 49(6):1095–1117. https://doi.org/10.1093/jcr/ucac029 (accepted manuscript)
- Singh, Murphy, Maher & Smith (2024). Time to form a habit: a systematic review and meta-analysis of health behaviour habit formation and its determinants. *Healthcare* 12:2488. https://doi.org/10.3390/healthcare12232488
- Spicer, Nicklin, Uther, Lloyd, Lloyd & Close (2022). Loot boxes, problem gambling and problem video gaming: a systematic review and meta-synthesis. *New Media & Society* 24(4):1001–1022. https://doi.org/10.1177/14614448211027175
- Stafford & Dewar (2014). Tracing the trajectory of skill learning with a very large sample of online game players. *Psychological Science* 25(2):511–518. https://doi.org/10.1177/0956797613511466
- Steinberg, Keiflin, Boivin, Witten, Deisseroth & Janak (2013). A causal link between prediction errors, dopamine neurons and learning. *Nature Neuroscience* 16:966–973. https://doi.org/10.1038/nn.3413
- Sweetser & Wyeth (2005). GameFlow: a model for evaluating player enjoyment in games. *Computers in Entertainment* 3(3). https://doi.org/10.1145/1077246.1077253
- Tracy & Robins (2007). The psychological structure of pride: a tale of two facets. *Journal of Personality and Social Psychology* 92(3):506–525. https://doi.org/10.1037/0022-3514.92.3.506 (abstract)
- Vallerand, Blanchard, Mageau, Koestner, Ratelle, Léonard, Gagné & Marsolais (2003). Les passions de l'âme: on obsessive and harmonious passion. *Journal of Personality and Social Psychology* 85(4):756–767. https://doi.org/10.1037/0022-3514.85.4.756
- van Laer, de Ruyter, Visconti & Wetzels (2014). The extended transportation-imagery model: a meta-analysis of the antecedents and consequences of consumers' narrative transportation. *Journal of Consumer Research* 40(5):797–817. https://doi.org/10.1086/673383 (abstract)
- Vohs, Schmeichel, Lohmann et al. (2021). A multisite preregistered paradigmatic test of the ego-depletion effect. *Psychological Science* 32:1566–1581. https://doi.org/10.1177/0956797621989733
- Vuorre, Johannes, Magnusson & Przybylski (2022). Time spent playing video games is unlikely to impact well-being. *Royal Society Open Science* 9(7):220411. https://doi.org/10.1098/rsos.220411
- Wagner, Gais, Haider, Verleger & Born (2004). Sleep inspires insight. *Nature* 427:352–355. https://doi.org/10.1038/nature02223 (abstract)
- Weiner (1985). An attributional theory of achievement motivation and emotion. *Psychological Review* 92(4):548–573. https://doi.org/10.1037/0033-295X.92.4.548
- Williams & DeSteno (2008). Pride and perseverance: the motivational role of pride. *Journal of Personality and Social Psychology* 94(6):1007–1017. https://doi.org/10.1037/0022-3514.94.6.1007 (abstract)
- Wisniewski, Zierer & Hattie (2020). The power of feedback revisited: a meta-analysis of educational feedback research. *Frontiers in Psychology* 10:3087. https://doi.org/10.3389/fpsyg.2019.03087
- Wood & Neal (2007). A new look at habits and the habit-goal interface. *Psychological Review* 114(4):843–863. https://doi.org/10.1037/0033-295X.114.4.843
- Wood & Rünger (2016). Psychology of habit. *Annual Review of Psychology* 67:289–314. https://doi.org/10.1146/annurev-psych-122414-033417
- World Health Organization. ICD-11 6C51 Gaming disorder; WHO Q&A. https://www.who.int/standards/classifications/frequently-asked-questions/gaming-disorder
- Xue, Wu, Kolen, Aghdaie & Zaman (2017). Dynamic difficulty adjustment for maximized engagement in digital games. *WWW 2017 Companion*:465–471. https://doi.org/10.1145/3041021.3054170
- Yancey & Settles (2020). A sleeping, recovering bandit algorithm for optimizing recurring notifications. *KDD 2020*:3008–3016. https://doi.org/10.1145/3394486.3403351 (peer reviewed; industry authors and data)
- Zagal, Björk & Lewis (2013). Dark patterns in the design of games. *FDG 2013*:39–46. http://www.fdg2013.org/program/papers/paper06_zagal_etal.pdf
- Zald, Boileau, El-Dearedy, Gunn, McGlone, Dichter & Dagher (2004). Dopamine transmission in the human striatum during monetary reward tasks. *Journal of Neuroscience* 24(17):4105–4112. https://doi.org/10.1523/JNEUROSCI.4643-03.2004 (abstract; N from a press release)
- Zeigarnik (1927). Das Behalten erledigter und unerledigter Handlungen. *Psychologische Forschung* 9:1–85. https://doi.org/10.1007/BF02409755
- Zendle & Cairns (2018). Video game loot boxes are linked to problem gambling: results of a large-scale survey. *PLOS ONE* 13:e0206767. https://doi.org/10.1371/journal.pone.0206767
- Zendle & Cairns (2019). Loot boxes are again linked to problem gambling: results of a replication study. *PLOS ONE* 14:e0213194. https://doi.org/10.1371/journal.pone.0213194
- Zendle, Flick, Gordon-Petrovskaya, Ballou, Xiao & Drachen (2023). No evidence that Chinese playtime mandates reduced heavy gaming in one segment of the video games industry. *Nature Human Behaviour* 7(10):1753–1766. https://doi.org/10.1038/s41562-023-01669-8

**Industry and press sources (not peer reviewed)**, listed by the person
quoted

- Bénard (2018-01-25). PlayStation Blog post on Dead Cells. https://blog.playstation.com/archive/2018/01/25/gorgeous-rogue-lite-action-platformer-dead-cells-is-battling-onto-ps4-this-year
- Bénard (2019). "'Dead Cells': What the F*n!?", GDC 2019. Listing: https://gdconf.com/article/get-game-design-tips-and-tricks-from-dead-cells-co-creator-at-gdc-2019/ ; write-up (Game Watch, 2019-03-23): https://game.watch.impress.co.jp/docs/news/1176126.html
- Collins (2022). Megastudy scepticism (blog). https://blog.jcx.au/posts/megastudy-scepticism
- Duolingo blog (2017, Loh), on streaks, the Streak Wager and the Weekend Amulet: https://blog.duolingo.com/how-streaks-keep-duolingo-learners-committed-to-their-language-goals
- Duolingo blog (2021, van Ogtrop), on habit and reminder opt-in copy: https://blog.duolingo.com/putting-in-work-the-habit-of-language-learning
- Duolingo blog (2022, Mansur), on Streak Freezes and the streak animation: https://blog.duolingo.com/how-duolingo-streak-builds-habit
- Eyal (2014). *Hooked: How to Build Habit-Forming Products.* Portfolio. (trade book)
- Filby (2017-06-08). Interview with the developers of Dead Cells, 80.lv. https://80.lv/articles/interview-with-the-developers-of-dead-cells
- Giovannetti (2019). "Slay the Spire: Metrics Driven Design and Balance", GDC 2019. https://www.gdcvault.com/play/1025731 (worked from a machine-generated summary, secondary: https://podwise.ai/episodes/1459444)
- Kasavin (2021), interviewed by Honorof, Tom's Guide, 2021-09-05. https://www.tomsguide.com/features/hades-exclusive-interview-supergiant
- Kasavin (2021), interviewed by Franzese on God Mode, Inverse, 2021-08-11. https://inverse.com/gaming/hades-god-mode-interview
- Kasavin & Korb (2021). "Breathing Life into Greek Myth: The Dialogue of 'Hades'", GDC 2021. https://gdcvault.com/play/1026975/Breathing-Life-into-Greek-Myth (listing only)
- Lee (2022-11-14). Indie World interview with Rogue Legacy 2 designer Teddy Lee, Nintendo. https://nintendo.com/whatsnew/indie-world-interview-rogue-legacy-2-game-designer-teddy-lee-talks-about-the-newly-available-heir
- LocalThunk (2025-03-06). The Balatro timeline. https://localthunk.com/blog/balatro-timeline-3aarh
- Miyazaki (2009), interviewed by Kollar, Game Informer, 2009-11-05. https://www.gameinformer.com/games/demons_souls/b/ps3/archive/2009/11/05/feature-demon-s-souls-director-discusses-difficulty-sequels-and-more.aspx
- Miyazaki (2022), interviewed by Parkin, "Hidetaka Miyazaki sees death as a feature, not a bug", The New Yorker, 2022-02-25. https://www.newyorker.com/culture/persons-of-interest/hidetaka-miyazaki-sees-death-as-a-feature-not-a-bug
- Rao (2020), quoted by Hetfeld, PC Gamer, 2020-10-05. https://www.pcgamer.com/a-journey-through-early-access-helped-make-hades-a-masterpiece/
- Rose (2013-08-29). The understated genius of the Spelunky Daily Challenge, Game Developer. https://www.gamedeveloper.com/design/the-understated-genius-of-the-i-spelunky-i-daily-challenge
- Thorson (2020). Celeste & forgiveness. https://maddymakesgames.com/articles/celeste_and_forgiveness/ ; Chapter 3 postcard text via the fan wiki (secondary): https://celeste.ink/wiki/Postcards
- Vice (2018-02-07, Klepek). Celeste's Assist Mode text and its "Cheat Mode" origin. https://www.vice.com/en/article/celeste-difficulty-assist-mode
- Yu (2011, 2016). "The Full Spelunky" and *Spelunky* (Boss Fight Books #11), quoted via Wikiquote (secondary): https://en.wikiquote.org/wiki/Derek_Yu

**Game text**

- Carroll (1865). *Alice's Adventures in Wonderland*, ch. IX (the Gryphon: "That's the reason they're called lessons… because they lessen from day to day"). Project Gutenberg: https://www.gutenberg.org/files/11/11-0.txt
