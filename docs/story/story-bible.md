# katharsis story bible

Status: draft for wiring, 2026-10-14. The in-game lines are in
[text.md](text.md), keyed by hook. Sources and licenses are at the end of
this file. Nothing here changes a rule. Story is a layer of words over
mechanics that already exist.

## Premise

```
ring 3, a hot afternoon
└─ you (PID 1, Comm: you) are reading a man page with no EXAMPLES section
   └─ a White Rabbit runs past with a watch: "Oh dear! Oh dear! I shall be late!"
      └─ it dives into a syscall; you follow; nobody checks your privileges
         ├─ a hall of locked doors; over the first: hello, world      (K&R)
         ├─ every door is judged by the Queen of Hearts               "Off with its head!"
         ├─ each clean shutdown drops you one ring deeper             (prestige)
         ├─ ring 0: the Court of Hearts. ring -1: the Red King's dream
         │   └─ two Wizards pass a ball: eval, apply, eval, apply     (SICP ch. 4)
         └─ the Jabberwock: SICP Exercise 4.15, escaped               The Halting Problem
```

The spine is one line from chapter I of *Alice*: "Down, down, down. Would
the fall never come to an end?" The game asks the Halting Problem in its
first minute and answers it in its last: not in general, but this boot
halted.

**Who you are.** A process: PID 1, `Comm: you`, as the crash screen already
prints it. Since you are init, killing you is "Attempted to kill init!",
which is the existing `:q!` panic. The cast only know Carroll's script, so
they call you Alice. Your comm says `you`. Humpty Dumpty would say a name
means whatever you choose it to mean. You never speak, and you have no
gender, no face and no name.

**Why you fell.** Boredom and a rabbit with a watch. **Why you keep
falling.** The only way out is `exit 0`, and every clean shutdown reboots
you one ring lower. Privilege rings count backwards: the lower you go, the
more you may do. The White Queen calls this living backwards.

## The machine: prestige is a ring

`ring = 3 - prestige`

```
GRUB                         firmware: the old gods' country (K&R relics)
├─ 0 Bronze    ring 3   Userland, Wonderland: Rabbit, Cat, tea party, Queen
├─ 1 Silver    ring 2   through the looking-glass: same rooms, reversed
├─ 2 Gold      ring 1   the wood where things have no names (symbols stripped)
├─ 3 Platinum  ring 0   the Court of Hearts: root; Queen Alice, eighth square
├─ 4 Advanced  ring -1  the Red King's dream (hypervisor); eval/apply; Jabberwock
└─ 5+          ring -2  rumour only (SMM, then the ME at -3)
```

These are real terms, not invented ones. x86 has rings 0 to 3, and most
kernels use only rings 0 and 3. OS/2 used ring 2 for code with I/O
privilege. "Ring -1" (hypervisor), "-2" (SMM) and "-3" (the Management
Engine) are the security community's informal names, so the game treats
them as rumour.

The Looking-Glass is the prestige mechanic in costume. Prestige holds the mid
biome at clear >= 0.6 and adds risk (D22). The doors learn you and you
run harder to stay level, which is the Red Queen's line almost word for
word: "it takes all the running you can do, to keep in the same place."
Each unlock is one more step through the glass.

## Cast

| Who | Plays | Voice | Signature (exact unless marked) |
|---|---|---|---|
| you | the player, PID 1 | never speaks | `Comm: you` |
| White Rabbit | the timer (SIGALRM) | flustered, always checking | "Oh dear! Oh dear! I shall be late!" |
| Queen of Hearts | the judge | verdict first, trial never | "Off with its head!" (parody of "Off with his head!"); "Sentence first—verdict afterwards." |
| King of Hearts | the statement, the spec, CE | pedantic | "Begin at the beginning … then stop." |
| Cheshire Cat | the rubber duck; keeps core dumps | grin first, cat later | "we're all mad here."; a core dump is "a grin without a cat" |
| Hatter, March Hare, Dormouse | the merge window | always tea-time | "It's always six o'clock now." |
| Mock Turtle, Gryphon | RTFM | sighing; brisk | "Reeling and Writhing, of course, to begin with" |
| Caterpillar | `top`; Bisect Bridge | languid | "Who are you?" |
| Duchess | crash epitaphs ("morals"); the UB kitchen | a moral for everything | "Everything's got a moral, if only you can find it." |
| Humpty Dumpty | naming: libs, vocabulary, LGTM | scornful | "When I use a word, it means just what I choose it to mean" |
| White Queen | regressions (spaced repetition) | lives backwards | "one's memory works both ways." |
| Red Queen | prestige treadmill, Overclock, -O3 | running | "you must run at least twice as fast as that!" |
| White Knight | daemons: his inventions | earnest, falls off | "It's my own invention." |
| Red King | the hypervisor; dreams the machine | asleep | "you're only a sort of thing in his dream!" (Tweedledee) |
| Eval and Apply, the Wizards | the metacircular evaluator, the heart | s-expressions | "The evaluator … is just another program." (SICP ch. 4) |
| the old gods, K&R | relics only, never met | lowercase, terse, never `!` | hello, world |

Two pairings carry most of the comedy, so they should not drift:

- **The Cat is the duck and the custodian of the core dump.** When you crash,
  the Cat vanishes and leaves its grin on the problem that killed you. That
  grin is your uncommitted bytes. Upsolving brings the cat back around the
  grin. Carroll wrote the core dump in 1865: "a grin without a cat!"
- **The Queen is the judge, and she is nothing but a pack of cards.** The
  verdicts are only test data. Alice's line that ends the dream ("You're
  nothing but a pack of cards!") is the `:q!` epitaph, and it returns as the
  ring 0 reveal: you are root, and the court is just tests.

## Bosses (keys and names unchanged; the costume is new)

| Key | Name | Costume | Why |
|---|---|---|---|
| off-by-one | Off-by-One | Tweedledum and Tweedledee | two of them, one gap; "Contrariwise" |
| heisenbug | Heisenbug | the Sheep's shop | "whenever she looked hard at any shelf … that particular shelf was always quite empty" |
| oom-killer | OOM Killer | the Rabbit's house, and Bill | you grew to fill the house; the Rabbit sends in a little Bill down the chimney |
| race-condition | Race Condition | the Caucus-race | runners "began running when they liked, and left off when they liked" |
| undefined-behavior | Undefined Behavior | the Duchess's kitchen | pepper means sneezing means nasal demons; the baby becomes a pig |
| halting-problem | The Halting Problem | the Jabberwock | SICP Exercise 4.15, escaped from the Wizards |

## Biomes (names unchanged; the scene is new)

| Topic | Name | Scene |
|---|---|---|
| complete-search | Brute Force Bunker | the hall of locked doors; Alice tried every one |
| simulation | The Emulator | the King's spec: begin, go on, stop |
| sorting-greedy | The Scheduler | "it's very easy to take more than nothing" |
| prefix-sums | The Accumulator | lessons: ten hours, nine, eight… |
| binary-search | Bisect Bridge | the Caterpillar's mushroom: one side taller, one shorter |
| two-pointers | The Sliding Window | "let's all move one place on" |
| graphs | Routing Table | the Cat: "where you want to get to" |
| trees | The Filesystem | a tree with a door in it |
| shortest-paths | Dijkstra's Reach | the Rose: "walk the other way" |
| dsu | Union Hall | the Lion and the Unicorn; "it's my crown all the while!" |
| dp | The Cache | "I know who I was when I got up this morning" |
| math | The ALU | Ambition, Distraction, Uglification, and Derision |
| strings | The Lexer | "'Twas brillig, and the slithy toves" |
| data-structures | The Heap | "The more there is of mine, the less there is of yours." |
| geometry | The Framebuffer | Looking-glass House: "only the things go the other way" |
| mixed | Userland | "we're all mad here." |

## Tone rules

1. **One line, one joke, punchline last.** A line is at most 70 display
   columns once its placeholders are filled. Multi-line blocks are only the
   named ones: the prologue (12 lines at most), reveals (5 at most), and
   two-line poem quotes.
2. **Groans welcome.** Puns, on-the-nose mappings, and kernel jargon used
   with a straight face. Never explain a joke in the game.
3. **Never mean.** The butt of the joke is the machine, the bug, the Queen,
   or Carroll's logic. It is never the player, and never a real person,
   language or community. Failure is normal ("Everyone does here"). Our own
   lines never go after a language; the sourced quotes in
   `internal/flavor/quotes.go` stay separate and unchanged.
4. **Second person, present tense** for narration. You are `you`. Never
   insert the player's real name, handle or email, and never derive text
   from the machine's username.
5. **Flavor never lies about mechanics.** The mechanical text
   (`UpgradeTextFor`, `DaemonTextFor`, door modifiers) stays the truth. A
   story line sits beside it and promises nothing new. Placeholders carry
   only real values.
6. **Carroll quotes are exact.** Words and spelling stay as printed
   ("to-day", "to-morrow"), italics are dropped, and the quote-mark style
   may change. Narration outside the quote marks may be ours. Our words
   never go inside a Carroll quote, and "…" marks an elision. Dropping the
   dialogue tag between two halves of one speech ("When I use a word," …
   "it means") needs no "…". The first letter's case and the closing
   punctuation may change to fit the line. New dialogue for the cast is
   fine: it is ours, and only text.md's quote register claims to be
   Carroll. A line that bends a real quote is labelled *parody* in text.md,
   and "Off with its head!" is one.
7. **SICP quotes are short, verbatim and attributed.** Allude to its ideas
   (eval/apply, the metacircular evaluator, memoization, `amb`) freely.
   Never rewrite an SICP sentence into a new line, because a rewrite is an
   adaptation and CC BY-SA would attach to it.
8. **K&R is homage only.** Use five words or fewer verbatim (`hello, world`,
   "C is not a big language"), and no passages or listings. The old gods
   speak lowercase, tersely, and never use an exclamation mark.
9. **No Disney.** "I'm late, I'm late, for a very important date" is the
   1951 Disney song, not Carroll, and it is under copyright. Carroll's
   Rabbit says "Oh dear! Oh dear! I shall be late!" and "Oh my ears and
   whiskers, how late it's getting!"
10. **Plain glyphs.** Use ASCII, the em dash, curly quotes and `·`. Write
    "ring -1" with an ASCII minus. No emoji in story text, because its width
    varies between terminals.
11. **Rotation.** A pool never shows the same line twice in a row, and a
    pick is stable per seed (reuse `QuoteFor`'s stride walk). A Carroll
    quote appears in at most two hooks.

## How the story unfolds

Each beat fires once (a "seen" flag) unless marked "every". The player
should be able to explain the plot after ten boots without having read a
cutscene longer than five lines.

| Moment | Trigger | What the player learns | Hook |
|---|---|---|---|
| first boot | `Meta.Boots == 0`, before the first route | you are PID 1; the Rabbit is the timer; the fall may not end | `story.prologue` |
| first door | boot 1, room 1 | the old gods were here first | `door.first.inscription` |
| first merge window | first `merge` screen | commit means tea; it is always six | `story.first-merge` |
| first daemon offer | first daemon pick | the White Knight invents things, and they have flaws | `daemon.intro` |
| first boss | first boss door opened | the King's rule: begin, go on, stop | `story.first-boss` |
| first crash | first crash screen | the Cat keeps your grin: that is the core dump | `story.first-crash` |
| first upsolve of a dump | core dump recovered | the cat comes back around the grin | `coredump.recovered` (every) |
| dump lost | crash before upsolving | Humpty can't be put back | `coredump.lost` (every) |
| first RTFM | first `OpenLearn` | the Mock Turtle's school; lessons lessen | `story.first-rtfm` |
| first regression | first regression door on offer | the White Queen remembers forwards | `story.first-regression` |
| every clear | cleared screen | where you are, in ring terms | `cleared.<p>` (every) |
| prestige unlock p | `Meta.Prestige` rises to p | the next ring's secret | `reveal.prestige.<p>` |
| final boss, first time | first `halting-problem` door | the Red King's dream; Exercise 4.15 escaped | `reveal.final-boss` |
| final clear, first time | first clear at max prestige | which dreamed it; the last door says hello, world | `reveal.final-clear` |

What each ring reveals:

- **ring 3 (Bronze).** It looks like Wonderland. You meet the Rabbit (the
  timer), the Queen (the judge), the Cat (hints), the Hatter (commits) and
  the bugs. The question is only how to get out. Clearing shows a clean
  shutdown, then the mirror over the mantel goes soft.
- **ring 2 (Silver).** Through the glass. The rooms are the same, only the
  things go the other way. The Red Queen explains why the doors got harder.
  You learn that the game is a treadmill on purpose.
- **ring 1 (Gold).** The wood where things have no names: the binary's
  symbols are stripped. A Fawn walks with you until the symbols load, sees
  what you are, and bolts. Humpty sits on the wall at the edge of the wood,
  defining words and reviewing code upside down.
- **ring 0 (Platinum).** The Court of Hearts. You are root, and in
  Looking-Glass terms you reached the eighth square and became Queen. The
  Court turns out to be a pack of cards. Behind the throne two Wizards pass
  a glowing ball back and forth: eval, apply. "The evaluator … is just
  another program."
- **ring -1 (Advanced).** The Red King is asleep, and the Wizards keep his
  dream running. Tweedledee: "you're only a sort of thing in his dream!" The
  Wizards once set Exercise 4.15 (write `halts?`, then show you can't). It
  got loose, and it has jaws. Every Advanced boot ends at the Jabberwock.
- **after the final clear.** The answer to the fall: you halted, this once.
  The last door bears the same inscription as the first, `hello, world`.
  The cleared screen starts rumouring ring -2. The story stays in the loop,
  because the dream does: "Life, what is it but a dream?"

## RTFM: the Mock Turtle's school

Learn mode is the Mock Turtle's regular course. The Gryphon runs the trials,
and a pass earns a capability (`CAP_DP`), the school's certificate. Its voice
is sighing nostalgia ("We had the best of educations") with Gryphon
interruptions ("That's enough about lessons").

The curriculum in Carroll's chapter IX maps onto CP almost without help:

| Carroll | Means | In katharsis |
|---|---|---|
| Reeling and Writhing | reading and writing | fast I/O |
| Ambition | addition | overflow |
| Distraction | subtraction | off-by-one |
| Uglification | multiplication | mod arithmetic |
| Derision | division | truncation toward zero |
| Mystery, ancient and modern | history | the editorial |
| Seaography | geography | geometry |
| Drawling, Stretching, Fainting in Coils | drawing, sketching, painting in oils | grid and ASCII-art problems |
| Laughing and Grief | Latin and Greek | strings and alphabets |

The running gag is "lessons lessen": ten hours the first day, nine the next.
Alice asks how they managed on the twelfth, and the Gryphon changes the
subject. The game's answer is unsigned underflow (`rtfm.underflow`). This
is a joke only. RTFM timers do not shrink unless a rule says they do (tone
rule 5).

## Wiring notes (for the implementation agent)

- One table in `internal/flavor` keyed by hook, plus `Story(hook, seed)` for
  pools and `StoryBlock(hook)` for ordered blocks. Placeholders use
  `{name}` and are filled by the caller. Add the funcs to `flavorSource` in
  `internal/screens/boot/env.go` so golden tests keep stubbing flavor.
- The "seen" beats need state. `Meta.Boots == 0` covers the prologue. The
  others need a `Seen` set in `Meta` or a store query. Storage owner
  decides.
- Tests: every hook is non-empty; every line is at most 70 columns
  (`ansi.StringWidth`) with each placeholder at the widths listed in
  text.md; no placeholder survives filling; every quote in text.md's
  register appears verbatim in the test's copy of it.
- Credits: add SICP (CC BY-SA 4.0) and Carroll (public domain) to the README
  "Content" section, and to the RTFM COLOPHON next to the USACO Guide line.

## Sources and licenses (checked 2026-10-14)

- **Lewis Carroll**, *Alice's Adventures in Wonderland* (1865) and *Through
  the Looking-Glass* (1871). Public domain. The wording was checked against
  Project Gutenberg eBooks #11 (<https://www.gutenberg.org/ebooks/11>) and
  #12 (<https://www.gutenberg.org/ebooks/12>). The game quotes Carroll and
  credits Carroll. It does not use the Project Gutenberg name, which is a
  trademark tied to their eBook files.
- **Abelson and Sussman with Julie Sussman**, *Structure and Interpretation
  of Computer Programs*, 2nd ed., MIT Press, 1996. The copyright page
  (<https://mitp-content-server.mit.edu/books/content/sectbyfn/books_pres_0/6515/sicp.zip/full-text/book/book-Z-H-2.html>)
  says: "licensed under a Creative Commons Attribution-ShareAlike 4.0
  International License". Quote short passages verbatim with attribution
  and do not adapt them (tone rule 7). quotes.go already has one SICP quote
  (preface); it stays.
- **Kernighan and Ritchie**, *The C Programming Language* (1978; 2nd ed.
  1988). Under copyright; homage only (tone rule 8). quotes.go already has
  the §1.1 sentence, sourced; the story does not reuse it. `hello, world` is
  the output of the book's first program. It is widely reported as older
  still: Kernighan's 1972 B tutorial. That makes it a relic older than C,
  which is why the old gods carved it.
- **Real systems facts** used as jokes: `Run /sbin/init as init process`,
  `random: crng init done` and `Calibrating delay loop` are real Linux boot
  messages. The Linux merge window is about two weeks. "Nasal demons" comes
  from comp.std.c, 1992. These are allusions, not quotes, so they need no
  credit line.
- **Disney**: not used (tone rule 9).
