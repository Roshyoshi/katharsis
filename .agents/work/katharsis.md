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
D15 proposed       Balance: calibrate the clear-rate aim from outcomes with the prestige
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
   h .golangci.yml (v2, std-error-handling preset) and a repo-wide lint gate

## Now
Phase A running in jj workspaces (.agents/ws/a1-contracts: units, catalog ID
helpers, intents, renames learn->curriculum and screens/learn->rtfm, trial
sizes, learn TeX, dumped bytes, lint config; .agents/ws/a2-economy: D25 shop,
D22 prestige risk, D24 mastery tags, D31 progress + quit warning, Attempt.Biome).
Main copy: fairness fixes (judge+content), GCC thread, chaos QA, ML thread.
Next: merge A1 then A2 by rebase; phase B (content/web, topic table, GCC judge,
deps/go mod tidy); C visual system; D UX journey; E retention research; F QA.
