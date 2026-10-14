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
D13 proposed       Run length: full clear ≈ 2–2.5 h, typical crash ≈ 1 h (user asked for
                   "2-3 hours per run"; not confirmed).
D14 settled 10-14  Keep quote 81 (Torvalds "real men ... ftp"): "Keep it".
D15 proposed       Balance: calibrate the clear-rate aim from outcomes with the prestige
                   offset subtracted, so prestige stays harder (engine finding 1).
D16 settled 10-14  Pull the Codeforces open-r1 dataset (local, no login) forward to fix
                   pool exhaustion and the 800 floor: "Yes, pull it forward".
D17 settled 10-14  Name is katharsis: "lets change the name to catharsis" then
                   "katharsis is better" (rename pending, see Now).

## Slices
S0 M0 training room            done: judge, problem, session, hud, practice cmd
S1 W1 contracts                done: catalog, rules, game types, flavor/screen APIs
S2 W2 parallel packages        done: engine+sim, store, flavor, screens/boot, screens/meta,
                               reader, learn, content/usaco+guide · running: E (judge+control)
S3 rename to katharsis         done: module path, cmd/katharsis, ~/katharsis, caches, banners
S3b Codeforces dataset content   after S3; alongside S4 (D16 pull forward)
S4 W3 integrate internal/app   started: routing, engine stepping, saves, effects, upsolve,
                               headless e2e suite green (clear, crash+upsolve, timeout,
                               resume) · next: real Runner/Judge on E's APIs, main
                               command, RTFM screens + trial boots, capability-name fix
S5 W4 hardening                balance sim (D15), e2e tests, fixes → game done
S6 W5 content                  CSES (login); Codeforces dataset moved to S3b per D16
S7 W6 platforms                CF sync/submit, AtCoder browser submit

## Now
Rename done; repo at ~/katharsis; all W2 agents finished and committed.
Next, in parallel: me on the real Runner/Judge/main (E's APIs), an engine agent
(trial boots, D15 calibration, faster cold start, capability names, hidden-test
rules), S3b Codeforces dataset agent.
