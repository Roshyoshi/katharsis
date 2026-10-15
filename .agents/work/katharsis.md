# katharsis: a Dead Cells-style CP roguelike in the terminal, done asap
Out of scope (for now): CF/AtCoder platform integration (W6), non-macOS polish.

## Decisions
D1  settled 10-14  Objective is loop feel. "whatever improves the game experience such
                   that its loop is as fun or approaching that of a roguelike"
D2  settled 10-14  Every limit is a tunable setting, locked per boot (DESIGN.md "North star").
D3  settled 10-14  Vocabulary: "rename BC to prestige, cells to bytes ... I like cursed
                   doors ... lives is fine"; rest themed (DESIGN.md "Vocabulary").
D4  settled 10-14  Crash screen quote: "a cheeky esoteric CS quote from some 10X dev or
                   cracked individual" (internal/flavor/quotes.go, 92 sourced).
D5  settled 10-14  Door choice: "title + tags is good and we can have a blind mode".
D6  settled 10-14  Sources picked by difficulty, local judging: "it should be any source,
                   we can go by difficulty IG if we're targetting local problems".
D7  settled 10-14  Finish the game before platforms: "before adding CF and AT coder
                   finishin the entire game might be a better use of time".
D8  settled 10-14  Stack Go + Bubble Tea: "python is kinda slow" → "Alright lets do go".
D9  superseded by D17
D10 settled 10-14  Parallel agents ok: "you can use as many subagents as you want",
                   "i want this game done asap".
D11 settled 10-14  jj commit per finished wave (local only); this workfile.
D12 settled 10-14  CSES stays in W5 ("Later, keep W5"); short learn units are topped up
                   from the catalog by topic and band.
D13 proposed       Run length: full clear ≈ 2 h, typical crash ≈ 1 h 15 m (TimerFitHi
                   1.5; user asked for "2-3 hours per run" and "30 min per problem
                   or something"; not confirmed).
D14 settled 10-14  Keep quote 81 (Torvalds "real men ... ftp"): "Keep it".
D15 proposed       (superseded by the ML aim once it lands) Balance: calibrate the clear-rate aim from outcomes with the prestige
                   offset subtracted, so prestige stays harder (engine finding 1).
D16 settled 10-14  Pull the Codeforces open-r1 dataset (local, no login) forward to fix
                   pool exhaustion and the 800 floor: "Yes, pull it forward".
D17 settled 10-14  Name is katharsis: "lets change the name to catharsis" then
                   "katharsis is better" (rename pending, see Now).
D18 settled 10-14  QA extensively: "hmmm yes we should QA extensively" (parallel QA
                   agents: content, judge, chaos, core loop, learn mode).
D19 settled 10-14  Code QA: "we might wanna do some code QA as well", but practical:
                   jv-review "is for rust/cpp critical path code ... dont take too
                   seriously here". Reviewers fix in their own scope; cross-cutting
                   duplication is report-only.
D20 settled 10-14  Difficulty model via neuro first, then ML: "first start the neuro
                   thread and based on what neuro thinks we can start an ml thread
                   to investigate creating a pipeline for it". Neuro report ->
                   docs/research/learning-science.md; ML thread starts from it.
D21 settled 10-14  UX design team: "we can get ux design team thread to evaluate and
                   improve the ux and ui". Journey lens -> docs/design/ux-journey.md,
                   visual system -> docs/design/ui-system.md; implementation
                   dispatched by package owner once UI packages are free.
D22 settled 10-14  Prestige holds the mid biome at clear >= 0.6 and adds risk instead
                   (lives, ducks, cursed/blind, timers): "Hold mid >= 0.6, add risk".
D23 settled 10-14  The ML thread may fit priors from the public Codeforces API (offline,
                   read-only; not W6 platform work): "Yes, offline priors are fine".
D24 settled 10-14  Hide tags by mastery on known topics; learning topics keep tags:
                   "Hide tags by mastery".
D25 settled 10-14  Bytes: spend now (this boot) vs commit for permanent unlocks bought
                   at GRUB: "Spend now vs commit for unlocks".
D26 settled 10-14  Dark terminals only; follow the terminal's 16-color theme: "Dark
                   only, follow my theme".
D27 settled 10-14  Difficulty stays '◆1300' with Codeforces rank colors everywhere
                   (capabilities need another glyph): "Keep ◆1300, rank colors
                   everywhere".
D28 settled 10-14  cin slowness: try a GCC toolchain (libstdc++) for judging: "Try a
                   GCC toolchain". Its own thread; bigger binary accepted: "def its own
                   thread, would increase bin size a decent amount but i think its
                   worth it" (docs/research/gcc-toolchain.md).
D29 settled 10-14  After the ML thread: a psychologist + neuro researcher thread on
                   what makes roguelikes and hard games compulsively replayable,
                   findings funnelled into implementation agents.
D30 settled 10-14  jj workspace per implementation agent at .agents/ws/<name>, merged by
                   rebase: "Yes, workspace per agent".
D31 settled 10-14  Keep the 60-rated-door prestige gate but show progress; doors keep
                   running while the game is closed, with a warning on quit and
                   resume: "Yes to both".
D32 settled 10-14  Storyteller thread: "read some of the classics (K&R, SICP, alice in
                   wonderland) and make a cheeky on the nose tongue and cheek story".
                   Alice is public domain, SICP CC BY-SA, K&R homage only ->
                   docs/story/; wired into flavor later.
D33 settled 10-14  Story front end + UI is its own thread after the story: "coordinate
                   front end + UI for the story in its own thing after, might be cool
                   to have some TUI animations". Runs after phase C (visual system);
                   skippable, reduced-motion setting, 80x24-safe.
D34 settled 10-14  Content-floor aid: +1 life when the forecast Userland crash chance
                   exceeds 0.20: "Yes, +1 life when needed" (ml-pipeline.md (d)).
D35 settled 10-14  A failed review (regression) door costs a life, not the boot:
                   "Costs a life, not the boot".
D36 settled 10-14  Predicted clear chance hidden on doors, shown afterwards (crash,
                   clear, top): "Hide on doors, show after".
D37 settled 10-14  Optional self-reported Codeforces rating as the cold-start prior,
                   asked once on first launch: "Yes, optional prompt".
D38 proposed       Model refits between boots with a held-out acceptance test and a
                   pin option; one interleave door per topic room a third of the time,
                   kept only if the learning simulator supports it (ml-pipeline (f),(h)).
D39 proposed       Story wiring (S9): wire the Queen's verdict lines first among the
                   optional extras; one-time scenes recorded on Meta (no schema
                   change); biome intros sit alongside the route blurbs.
D40 settled 10-14  Audio thread: SFX plus 8-bit Touhou arrangements as a pool per stage
                   (biome): "touhou 8bit mixes for each stage maybe (we could do a pool
                   for each stage ...)"; "remember to credit author for those, and use
                   arrangements that allow for that sort of usage". Both ZUN's fan-work
                   guideline and the arranger's licence must allow it; never bundled.
D41 settled 10-14  Lanes, not "after D". Wave 1 (A2+R1 merged): B rest, ML1 engine
                   logging + ML2 events table + D37 rating prompt (boots before ML1
                   cannot be used, ml-pipeline.md:90-93), ML3 learning sim, C; audio
                   merges when it reports. Wave 2 (C merged): D with story text (D39),
                   D36 and retention UI folded in; S9 animations beside D on split
                   screens; ML4 shadow + ML5 fit; retention engine items. After real
                   play: ML6 aim switch (needs ML4 log loss on real boots <= Elo),
                   ML7-ML10, ML12; ML11 CF prior in the content lane. Retention
                   metrics go in ML2's events table. Alpha (the user playing) after D +
                   core-loop QA: "lets just target D for alpha testing (aka. just me
                   playing it)".
D42 settled 10-14  Alpha holds everything except ML that needs real play: "we should
                   try to get everything we can in except for the ML stuff that would
                   require it". In: A2, R1, B, GCC, C, D (+story text, D36, retention UI),
                   S9, audio, retention items, ML1-ML5, ML7-ML11 behind rules flags and
                   sim-verified, ML6 code behind Aim (default "elo"). Out: flipping ML6
                   to "model" (needs real-boot log loss), ML12, CSES (needs the user's
                   login), platforms S6/S7.
                   10-14: CSES and platforms stay out; "game can be self contained".
D43 proposed     Retention funnel (docs/research/retention.md). Guardrails GR1-GR12
                   bind every agent (no streaks, no date pressure, no fabrication, bytes
                   only for learning acts, easing announced and declinable, telemetry
                   local). RT0 first in the S10 lane and under Aim "elo" (alpha keeps
                   ML6 off): sim Userland crash 61% at 1000, 26% at 1600 vs the 15%
                   design; fix with D34 +1 life, Userland buy-back and governor until
                   G1 <= 0.20 at 900-1600. RT2, RT3 -> A2 (sent). RT6 -> S9 (sent).
                   RT1, RT4, RT7, RT8 UI, RT10 UI, RT12, RT15 -> D, one owner of
                   crash.go, cleared.go, grub.go (game.Debrief, Upsolve 1.5x, State.Ease
                   engine parts with it). RT5 -> ML9. RT8 engine, RT11 -> S10 lane.
                   RT9, RT13 -> ML2 events table. RT14 motd -> D. Defaults taken for the
                   user-held items: standard boot length kept with short and patch
                   first-class; 3 h session nudge; 1.5x upsolve without the editorial;
                   apt catalog >= 40 boots deep.

D45 settled 10-15  Once D (both halves), S9 part 2 and the S10 engine lane are merged,
                   start the final pass without asking: cleanup by package with the QA
                   list, then core-loop QA, then ping for alpha: "when D + remaining
                   engine and UI work finishes then you can get started on finishing
                   the final pass".

D46 settled 10-15  Alpha feedback, boot #1: "the timer shouldnt run in between levels lol";
                   "10 min for A is crazy"; "20% time addition shouldnt come w 2 live
                   penalty". Uptime counts only in-door time (session budget stays wall
                   time); Userland 15 min in every preset; TimerFitLo 0.8, TimerHeadroom
                   1.5; nice -20 costs bytes (x0.8), not lives; audit the other perks.

D47 open 10-15     Meta research before any change to lives: "3 lives seems a little harsh";
                   "a pool of 30 lives, and each time out costs like 10"; "pvz 1 has a
                   great meta"; "we need to consider the meta do some research first in
                   a thread". Thread: docs/research/meta.md (health economies in Dead
                   Cells, PvZ1, Hades, StS, Into the Breach; ICPC/CF scoring; error-cost
                   learning science; 2-3 candidates prototyped in the sim only).
                   10-15 user: "Give Up should -10 HP" (door only, the boot goes on);
                   "you should be able to start and resume runs on the map" (suspend
                   between doors, the map as the hub): both folded into the thread.
                   "it should save progress in map but like you cant quit inside a
                   level": suspend only between doors; no quit inside a door (solve,
                   retreat or give up); crash-safety resume stays.

D48 settled 10-15  Lives are renamed cycles (CPU cycles), still drawn as ♥: "can change
                   lives name to cycles lol / keep icon". Lands with the D47 meta change.

## Slices
S0 M0 training room            done: judge, problem, session, hud, practice cmd
S1 W1 contracts                done: catalog, rules, game types, flavor/screen APIs
S2 W2 parallel packages        done: engine+sim, store, flavor, screens/boot, screens/meta,
                               reader, learn, content/usaco+guide · running: E (judge+control)
S3 rename to katharsis         done: module path, cmd/katharsis, ~/katharsis, caches, banners
S3b Codeforces dataset content   done: 6761 judgeable rated problems; any-case YES/NO judging
S4 W3 integrate internal/app   done: real door sessions (vim + overlay, embedded HUD, border
                               countdown, hidden full tests), `katharsis` command, union
                               catalog; smoke-tested in isolated tmux (doors, :q!, crash)
S5 W4 hardening                balance sim (D15), e2e tests, fixes → game done
S6 W5 content                  CSES (login); Codeforces dataset moved to S3b per D16
S7 W6 platforms                CF sync/submit, AtCoder browser submit
S9 story presentation (after storyteller + phase C): typewriter prologue,
   rabbit-hole transitions, boss cards, Cheshire fade, reduced-motion setting
S10 ML pipeline implementation (after A2 merges): the five slices in
   docs/research/ml-pipeline.md behind rules flags, decisions D22-D24, D34-D38
S8 consolidation (after all code QA lands; from the cross-package report):
   a units pkg: one Bytes (currency), Mem, Clock, Countdown; fixes 05:00 vs
     04:59 and 15.2 KiB vs 15 KiB; hearts drawn one way
   b content/web: one polite HTTP client, atomic write, debris cleanup for
     usaco, codeforces, guide (~250 lines)
   c screens/ui: palette, frame/pad/spread, rating style, one HP bar (~150)
   d contracts: catalog.ID helpers + Mixed + typed Verdict; intents use
     catalog.ID; one Lookup; Attempt.Biome (drop app's biome replay); trial
     sizes from rules.Trial(); DivisionBand from usaco; ONE module→topic
     table (copies disagree on 5 modules: a real bug); learn emits TeX
     verbatim (drop ~300 lines); rename learn→curriculum, screens/learn→rtfm
   e delete internal/deps, go mod tidy, refresh flake vendorHash
   f store dumped bytes so git log shows them
   g judge hardening: kill the whole process group (forking solutions),
     evict the build cache, clean build-* dirs left by SIGKILL, treat
     ErrWaitDelay on a clean exit as a verdict
   h (done in A1)
   i Codeforces opens cache-only at start; index builds in the background and
     joins the union when ready; GRUB shows "Codeforces offline" (chaos H2)
   j known limitation: a solution can read cached hidden tests at run time
     (qa/chaos/oracle.sh); single-player honor system, not fixed

## Now
Merged since last: S10 engine (RT0: CF 800 read as censored, X ~ Exp(250); union G1
1000 53.8 -> 13.1, 1600 26.4 -> 5.8; D44; D22 holds 0.65+ at p0-p4; easing
machinery built, only D34 life + governor on; Meta.Retired; ML1 offer logging
with propensities (chi2 p 0.75); ML3 learning player, EOMM ratio 2.16; ML7
TestGuardrailG1; RT8 comeback; RT11 boot #1 short, 66 min for the 1000 model;
apt prices up to keep the ladder 40+ boots deep). All wave 2 merged.
Final pass merged: g-learn (reader: \[ is an escaped bracket, not math: fixed 568 lines on 61
module pages and 4 USACO statements; product denominators parenthesised; rtfm
offers t only when a trial grants a capability; parseFrontmatter 73 -> 18,
parseOrdering, Load split; TeX conversion kept: cutting it put raw TeX on 16
pages), g-content (USACO tags aligned, schema 3, 8 problems move; web.Flights
single-flight closes a codeforces double-download window; old guide snapshots
swept; fsync before publish; Codeforces retries once after 10 min). g-judge (30 s compile timeout across every compiler run, compilers in their own
process group so a cancel kills cc1plus too, compiler TMPDIR = build dir, timing
tests hold at load 37-67, problem.Load and FileIO "%s.in" fixes). f-hooks (D44 notice on cards/HUD/strip/map; RT8 easing lines + x declines;
"first boot in N days"; short + patch built-ins (patch: 3 doors, ~1 h), short.toml
dropped; route aim from AimClear; ◆≤800 floor doors; Forecast after the doors
and in top; milestone.retired.1; no SPONGE left in the repo). g-store (stale vim border writes dropped by sequence; Cleanup waits up to 5 s
for the passes it cancels, so no compile outlives the game; a late :Debug pane
is closed; session tests hold at load 35-57; HUD can answer :Submit with the
cost; migration 2 keeps every v1 column, tested). g-qa (qa/content judges with the :Submit toolchain + FileIO; a usaco 1131 QA
solution only passed on clang's vectorizer; qa/judge timing tests use the submit
profile; the chaos harness reaches doors again (--no-prompts, past story scenes);
REPO default no longer a hard-coded home path; audio: a lead never repeats, a
deviceless engine stays silent, 0 Hz WAV rejected; qa lint 30 -> 0). g-flavor (a finished story page could freeze half-typed after a resize; the
first-crash title cut to 24 cells; 92 quotes checked live; SPONGEs added on
merge.full and regression.door for the storyteller). g-ui (overtime clears showed the unpaid reward; the HUD froze at 00:00 past a soft
deadline; compact rows dropped their numbers for long titles; one h:mm; OLE gets
the MLE line; ASCII separators; route stats aligned; Solves meter |#*; git log
rating column; real GRUB art fits 80x24 with up to 13 entries). story-polish (daemon.cron, daemon.chrt, upgrade.shuf, verdict.OLE written; every
draft revised; milestones carry no counts; the Turing-note thread closes its own
questions; {days} replaces "Day three" / "Seventeen days on"; regression.door
and merge.full SPONGEs resolved; dead hooks cut or given a home; text.md
pointers fixed). g-engine (sim panic on presets shorter than boot #1; cron/chrt pay multipliers
bounded; fork not offered once tmux applies (full clears +3 to +10); a trial
before the first boot no longer cost boot #1's rules; LifetimeBytes never falls).
story-wire2 (reveal.prestige first lines fit the 59-column teaser). g-app (app tests 30 s -> 5.8 s in synctest bubbles, -race clean; vim :Submit reply
names the cost; timer.warn on the vim border; music fades out on every exit;
reader skips TeX on RTFM pages; katharsis.log -> XDG state dir; RT1 RTFM next
step after 2 crashes on a topic in 14 days; r after a trial retries it; bugs: a
new room kept the downloading choice, upsolves paid the editorial rate without
one shown; story.first-rtfm and rtfm.trial.start wired).
Cleanup done; story-wire merged ({days} from the regression schedule; merge.full
at MaxDaemons; regression.door in the strip; trial pass/fail Gryphon lines; the
Mock Turtle on module pages; RTFM index WORDS/CURRICULUM/EXAMPLES/BUGS with tail
scrolling so COLOPHON stays reachable; mid-boot boss defeat over the merge
window's upgrade). Core-loop QA done: verdict ready to play; 9 fixes merged (resize resets the
scroll region so the end line no longer lands mid-panic; merge cursor; vim reply
heads; flash wraps; merge leave row; top "-"; trial cleared title; crash tick
cost; refusal text). Speed: GRUB 0.2 s, :w warm 0.55 s, :Submit 10 tests 3.5 s,
idle 0.5-1.4% CPU. Disk: /tmp agent scratch removed (1.2 -> 5.3 GiB free); go
build cache 16 GB is the user's call. Merged: fix-orphans (each run starts via dash with ulimit -t, +0.9 ms; macOS sends
SIGXCPU only, so Sweep at start kills orphaned runs by executable path),
fix-polish (O2: an upsolve pays the editorial rate only after an editorial_open
event; RTFM hides the guide's pages about itself; route and door timers are the
ones the doors open with; RTFM next step names the biome and the nearest-band
module; strip ellipsis; resume note clears; a passed trial offers the next
module). Pushed to origin (private GitHub repo, the URL the user gave); tagged and released
v0.1.0-alpha (prerelease) at 39ce7a0a. ALPHA READY 10-15: full gate green, -race clean, nix build ok, binary
30.2 MB. Open for the user: practice trials for the 33 capability-less modules;
rate retreats or not; J18 fixed-order redraw; J11 refuse a quack that reveals
nothing; go clean -cache (16 GB); nix-collect-garbage; the GitHub remote; wave 1 done.
FINAL PASS (D45) running: f-hooks (engine -> UI: D44 notice, easing + decline,
comeback, short/patch built-ins, route aim, Door.Floor, Forecast after, the
retired milestone); cleanup wave 1: g-store, g-qa
(brief: ~/.claude/jobs/e2124f03/tmp/cleanup-brief.md). After f-hooks merges:
cleanup wave 2 g-engine (game, sim, rules), g-flavor (flavor, anim, story), g-ui
(ui, units, screens/boot, screens/meta), g-app (app, cmd, demos; -race in the
robust_test harness, app tests that wait on real time ~30 s, fork with tmux,
RT1 RTFM next step, D36 in top). Then core-loop QA, then alpha.
D in-boot (D36: no clear % on door cards; dmesg strip with verdict costs,
-O3, timer.warn, the Queen's lines; one-time tip.* onboarding lines; :Submit! over
failing samples, pane s s and R R in ui.Alert; statement-first pane, template
unjudged until saved; per-door fetch state + bytes, esc cancels, pick a ready door;
white Knight/Queen/Hatter blocks; SPONGE slots for D44 + RT8 in
screens/boot/notice.go). D is complete. D end-of-boot (game.Debrief + State.AtStart; crash/clear debrief: kept,
lever, next; double-r guard; lost dump headline; goal bars; 1.5x independent upsolve;
GRUB order, subtitles, /proc/story, nightly best, regressions capped at 3;
katharsis motd; settings.toml + --session-budget; git log r replays a seed;
Carroll + SICP on the c credits page), S9 part 2 (scenes at their moments via app/story.go: splash, prologue,
first door, ACCEPTED on AC, first merge, biome drops, boss cards, the Cat 1.5 s
after a first crash; never over an open door; esc drops the queue; --motion
full|reduced, --fps; story styles on ui; Carroll + SICP in the RTFM COLOPHON;
milestone.mastered.1; story.Read{ID}; first-regression-retired waits on
Meta state from the engine lane), C part 2 (every screen on internal/ui: rank-coloured ◆ ratings, one HP
bar per pane, toned border clock, no blink, C8 problem pane, HUD 36-48 cols, pager
zoom, credits on GRUB `c` with the banner back at 80x24, crash bottom-anchored,
framed cleared banner, Mono under NO_COLOR), B content (content/web shared client with a 30 s idle cut, fsync
writes and debris sweeps; one module->topic table in guide, the curriculum's
mapping proved right for all 5; usaco.DivisionBand; Codeforces opens from cache
and indexes in the background: cold-cache start 20.7 s -> 0.4 s; internal/deps
gone, go mod tidy, vendorHash recomputed with a fake hash after the merge), S10 store (migration 2: events, offers, skill_history,
model_versions, attempt columns; app events through a 256-deep async queue,
failures to katharsis.log; D37 prompt = identity CF prior clamped 800..3500,
asked once when nothing is rated, --no-prompts; load average = share of practice
days over 7/30/90 (RT9, not boots per day); top `r` retention panel + GR11
alarms; boot-end worth-it/felt line; the player's DB migrates forward-only on
next launch), submit-busy (a :Submit while one judges is refused; the app's
m.judging owns "busy"; chaos r01/r04/r05/r09/r13b updated; risk: no compile
timeout in judge.Compile, so a hung compile locks resubmits until the grace),
judge starve guard for AC cases, B-judge (PCH of each source's leading includes for g++ and clang
samples: warm :w 1.4 s -> 0.77 s, :Submit 0.93 s -> 0.49 s on the template;
-include bits rejected; qa/judge accepts libstdc++ wording + a g++ O2 profile;
CPU-time TLE check; libc++ hardening on the fallback; BuildCacheCap 256 -> 512
MiB), app tests under a scratch HOME (they wrote the real build cache, and a warm
cache broke two submit tests), audio (oto v3 in-process engine, procedural SFX, Go port of SUZUKI PLAN's
VGS MML compiler + decoder; 175 Touhou VGS songs + SwapXFO Misty Lake (CC BY) in
per-biome pools; fetched from pinned sources with SHA-256, never bundled; ZUN
fan-work notice; binary 29.6 MB; conflicts with A2/R1/S9 resolved by hand; GRUB
banner test moved to 80x25 with a SPONGE for C part 2 P4), A2 (D22, D24, D25 spend + apt shop of 19 packages / 10,272 B,
D31 progress + quit confirm, Attempt.Biome, Verdict type, RT2/RT3 sim tests; 4
small conflicts with R1/S9 resolved by hand; qa/chaos Verdict fix), R1 (14 robustness items: store lock, unwritable store, W1 submit
snapshot + resume re-judge, editor shell line, orphan panes, idle saves, guide
stall cut; one hud conflict with the GCC change resolved), S9 part 1 (flavor story table + RT6 pacing, Meta.Seen, internal/anim,
screens/story scenes + cmd/demo-story incl. the ACCEPTED beat; Part 2 map and
defaults in .agents/work/s9-part2.md), C part 1 (internal/ui on units; reader, rtfm, demo status bars;
Part 2 checklist in .agents/work/c-part2.md; defaults: AC flash goes to S9, door
history shows bare rank-coloured numbers under a ◆ header), retention doc (D43 funnel), GCC judge (g++ 14 judges :Submit
and :Debug; samples clang ASan on libstdc++ + _GLIBCXX_ASSERTIONS; cin 1e6 ints
38 ms vs 967 ms on libc++; clang 21 out; package closure 2013 -> 1989 MiB;
binary 28.4 MB). Main tests green (judge TLE timing flaked at load 23; passed
alone); nix build ok.
Agents keep .agents/work/progress-<ws>.md for
restarts (spamming esc stops every agent).
For B (from the GCC report): qa/judge expects ASan wording on the libstdc++
sample profile (re_heap_oob, re_container_oob, re_after_20KB_debug_stderr) and
lacks a g++ -O2 profile; stale "clang 22 judges" at DESIGN.md:17 and :96 and in
the internal/app template note on bits/stdc++.h; optional PCH for
bits/stdc++.h (-0.8 s per g++ build, 175 MB .gch) and libc++ hardening for the
clang fallback.
Final QA must make the tmux/lldb session tests and the judge timing tests
robust under load (both flaked at load ~20, pass alone).
A2 leftovers: RT0 root cause is the pool floor (94-98% of a 1000 player's Userland
doors are off target; 63% of first boots crash before a merge) -> S10 engine; D22
fails for the 1000 model at p2-3 (0.57, 0.55) -> S10 engine; sim always takes
daemons -> S10 engine (ML3); RT3 goal bar + ETA, upsolve-pays-for line -> D;
install event -> ML2; linked libs covariate -> ML4; GRUB cursor reset after
install, fork text "(normally 3)" with tmux -> C part 2.
Next: merge A2; then wave 1: S10 lane (RT0 first,
ML1 -> ML3 -> ML4/ML5 -> ML6 code -> ML7-ML10; ML2 + D37 in the store lane;
ML11 after B), C part 2; wave 2: D (+story text, D36, retention UI), S9 part 2;
audio merges when ready; core-loop QA; ping alpha (D42).

## QA list (pre-alpha, after wave 2)
Run with the unslop pass (package by package), then core-loop QA, then alpha.
- Load robustness: session tmux/lldb tests and remaining judge wall-clock
  tests flake at load >= 30; -race finds data races in the robust_test.go
  harness (h.stall).
- USACO tag table still maps Sliding Window, Sweep Line, Merging, Bitset the
  old way: align with guide.ModuleTopic (moves 227 266 943 -> geometry, 600
  767 1021 -> data-structures, 862 -> math, 923 -> prefix-sums; usaco schema
  bump).
- qa/content TestAudit: drop cf:646A, cf:1769B2 (Russian-only, already out of
  play). qa-tagged files: 16 lint issues. ubsan+hardened OOB cases report no
  line. qa/chaos r13 first half greps the wrong thing.
- Duplicates: single-flight Fetch + isCtxErr in usaco and codeforces (~40
  lines each); judge cleanDebris vs web.CleanDebris.
- Old guide snapshot dirs stay after a Commit bump; Codeforces "offline" never
  retries until the next start; problem dir files are not fsynced;
  katharsis.log may belong in XDG_STATE_HOME.
- top's Solves meter tells parts apart by colour only; expanded git log
  rating not in its own column (E18); real 4-line GRUB art fits 80x24 only
  with <= 11 entries.
- Engine hooks for the final pass: boot/debrief.go easeItems (list State.Ease
  with Why, "declined"), the comeback boot on GRUB's status row, register
  short and patch as built-in presets (the end screens offer s/p only then).
- D boot leftovers: the vim reply after a failed submit does not name the life
  lost; timer.warn not on the vim border; merge.full line promises a daemon
  swap the engine lacks (storyteller).
- D end leftovers: RT1 "RTFM as the next step after 2+ crashes on a topic in
  14 days"; D36 predicted-vs-observed in top; a real-binary smoke run.
- judge: the background PCH build (pch.go:184) runs under context.Background and
  can outlive the game by one header build.
- HUD compile-error lines carry the full absolute source path (all profiles);
  show it relative to the door dir.
- Story text for the storyteller: hooks missing (daemon.cron, daemon.chrt,
  upgrade.shuf, verdict.OLE); dead hooks (grub.subtitle.locked, regression.door,
  rtfm.*, help.vocab, white-knight.song, merge.full, story.first-rtfm never
  queued); regression.stage.1/.2 hardcode "Day three" / "Seventeen days on";
  text.md "where:" pointers stale (screens/learn -> screens/rtfm, merge.go:160).
- ASCII mode still draws Unicode in the parody screens (GRUB box, git log gutter,
  top's braille chart, ▲/▼).
- When deps change: recompute vendorHash with a fake hash (nix reuses a
  cached vendor dir and hides a stale hash).
