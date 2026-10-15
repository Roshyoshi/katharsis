# katharsis story text

The in-game lines, keyed by hook. The premise, cast and rules are in
[story-bible.md](story-bible.md). Read its tone rules before changing a
line here.

## How to read this file

Each hook looks like this:

~~~
### hook.name
where · when · pick · uses
```text
one entry per line
```
~~~

- **where**: the screen and the code that draws it today. "new" means no
  slot exists yet.
- **when**: `every` (each time the screen shows) or `once` (a seen flag,
  see the bible's unfolding table).
- **pick**: `pool` rotates one entry per seed, with no repeats back to
  back. `block` shows all lines in order. `fixed` is one line. In a pool
  marked "2-line", blank lines separate the entries.
- **uses**: quote-register IDs (below), for the lines that quote a source.
  A line marked *parody* puts our words in a character's mouth on purpose.
- Every line is at most 70 columns with its placeholders filled at the
  widths below. Quote marks are typed as ASCII here. The quote style may
  change in code, but the words may not.

| Placeholder | Max width | Example |
|---|---|---|
| `{boot}` | 5 | `42` |
| `{ring}` | 2 | `3`, `-1` |
| `{place}` | 20 | `the Red King's dream` |
| `{bytes}` | 8 | `15.2 KiB` |
| `{title}`, `{tags}` | 24, caller truncates | `Bovine Shuffle` |
| `{technique}` | 15 | `data-structures` |
| `{n}` | 2 | `3` |
| `{ago}` | 11 | `17 days ago` |
| `{pct}` | 5 | `15%` |
| `{module}` | 24, caller truncates | `Prefix Sums` |
| `{division}` | 8 | `Platinum` |
| `{cap}` | 14 | `CAP_PREFIX` |
| `{biome}` | 18 | `Brute Force Bunker` |
| `{name}` | 16 | `dsu` |
| `{time}` | 5 | `14:41` |

`{place}` by ring: 3 `Userland`, 2 `the looking-glass`, 1 `the nameless
wood`, 0 `the Court of Hearts`, -1 `the Red King's dream`.

## Quote register

These quotes are verbatim. Italics are dropped, and Gutenberg's curly quotes
and apostrophes are typed as ASCII. "…" marks an elision. A dialogue tag
dropped between two halves of one speech is not marked. In game lines, the
first letter's case and the closing punctuation may change to fit the
line. Every fragment below was found verbatim in the Gutenberg plain text
and the MIT Press SICP pages on 2026-10-14. Each chapter cell links to
the chapter.

**Alice's Adventures in Wonderland** (Lewis Carroll, 1865). Public
domain. Base: <https://www.gutenberg.org/cache/epub/11/pg11-images.html>,
anchors `#chap01` … `#chap12`.

| ID | Exact text | Speaker | Ch. |
|---|---|---|---|
| A1 | Oh dear! Oh dear! I shall be late! | White Rabbit | [I](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap01) |
| A2 | Oh my ears and whiskers, how late it's getting! | White Rabbit | [I](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap01) |
| A3 | Down, down, down. Would the fall never come to an end? | narration | [I](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap01) |
| A4 | There were doors all round the hall, but they were all locked | narration | [I](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap01) |
| A5 | in a sort of circle | narration (the Dodo's course) | [III](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap03) |
| A6 | Everybody has won, and all must have prizes. | Dodo | [III](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap03) |
| A7 | There goes Bill! | the crowd | [IV](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap04) |
| A8 | There ought to be a book written about me, that there ought! | Alice | [IV](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap04) |
| A9 | I almost wish I hadn't gone down that rabbit-hole—and yet—and yet— | Alice | [IV](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap04) |
| A10 | Who are you? | Caterpillar | [V](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap05) |
| A11 | I know who I was when I got up this morning | Alice | [V](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap05) |
| A12 | There's certainly too much pepper in that soup! | Alice | [VI](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap06) |
| A13 | it makes rather a handsome pig, I think. | Alice | [VI](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap06) |
| A14 | Cheshire Puss, | Alice | [VI](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap06) |
| A15 | That depends a good deal on where you want to get to, | Cat | [VI](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap06) |
| A16 | we're all mad here. I'm mad. You're mad. | Cat | [VI](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap06) |
| A17 | Did you say pig, or fig? | Cat | [VI](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap06) |
| A18 | a grin without a cat! | Alice | [VI](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap06) |
| A19 | No room! No room! | Hatter, March Hare | [VII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap07) |
| A20 | Have some wine, | March Hare | [VII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap07) |
| A21 | There isn't any, | March Hare | [VII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap07) |
| A22 | Why is a raven like a writing-desk? | Hatter | [VII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap07) |
| A23 | Two days wrong! | Hatter | [VII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap07) |
| A24 | It was the best butter, | March Hare | [VII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap07) |
| A25 | He's murdering the time! Off with his head! | the Queen, as the Hatter tells it | [VII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap07) |
| A26 | It's always six o'clock now. | Hatter | [VII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap07) |
| A27 | it's always tea-time | Hatter | [VII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap07) |
| A28 | as the things get used up. | Hatter | [VII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap07) |
| A29 | it's very easy to take more than nothing. | Hatter | [VII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap07) |
| A30 | Treacle, | Dormouse | [VII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap07) |
| A31 | let's all move one place on. | Hatter | [VII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap07) |
| A32 | At any rate I'll never go there again! | Alice | [VII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap07) |
| A33 | Off with his head! | Queen | [VIII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap08) |
| A34 | Nonsense! | Alice | [VIII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap08) |
| A35 | The players all played at once without waiting for turns | narration | [VIII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap08) |
| A36 | fighting for the hedgehogs | narration | [VIII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap08) |
| A37 | Everything's got a moral, if only you can find it. | Duchess | [IX](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap09) |
| A38 | The more there is of mine, the less there is of yours. | Duchess | [IX](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap09) |
| A39 | We had the best of educations | Mock Turtle | [IX](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap09) |
| A40 | I only took the regular course. | Mock Turtle | [IX](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap09) |
| A41 | Reeling and Writhing, of course, to begin with, | Mock Turtle | [IX](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap09) |
| A42 | and then the different branches of Arithmetic—Ambition, Distraction, Uglification, and Derision. | Mock Turtle | [IX](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap09) |
| A43 | Ten hours the first day, | Mock Turtle | [IX](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap09) |
| A44 | because they lessen from day to day. | Gryphon | [IX](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap09) |
| A45 | And how did you manage on the twelfth? | Alice | [IX](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap09) |
| A46 | That's enough about lessons, | Gryphon | [IX](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap09) |
| A47 | Consider your verdict, | King | [XI](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap11) |
| A48 | Where shall I begin, please your Majesty? | White Rabbit | [XII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap12) |
| A49 | Begin at the beginning, … and go on till you come to the end: then stop. | King | [XII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap12) |
| A50 | Rule Forty-two. All persons more than a mile high to leave the court. | King | [XII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap12) |
| A51 | If there's no meaning in it, … that saves a world of trouble | King | [XII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap12) |
| A52 | Sentence first—verdict afterwards. | Queen | [XII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap12) |
| A53 | You're nothing but a pack of cards! | Alice | [XII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap12) |
| A54 | Wake up, Alice dear! | her sister | [XII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap12) |
| A55 | Curiouser and curiouser! | Alice | [II](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap02) |

**Through the Looking-Glass** (Lewis Carroll, 1871). Public domain.
Base: <https://www.gutenberg.org/cache/epub/12/pg12-images.html>, anchors
`#link2HCH0001` … `#link2HCH0012`. The closing poem follows chapter XII.

| ID | Exact text | Speaker | Ch. |
|---|---|---|---|
| G1 | 'Twas brillig, and the slithy toves | "Jabberwocky" | [I](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0001) |
| G2 | Beware the Jabberwock, my son! / The jaws that bite, the claws that catch! | "Jabberwocky" | [I](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0001) |
| G3 | One, two! One, two! And through and through / The vorpal blade went snicker-snack! | "Jabberwocky" | [I](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0001) |
| G4 | He left it dead, and with its head / He went galumphing back. | "Jabberwocky" | [I](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0001) |
| G5 | O frabjous day! Callooh! Callay! | "Jabberwocky" | [I](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0001) |
| G6 | only the things go the other way. | Alice | [I](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0001) |
| G7 | Now, here, you see, it takes all the running you can do, to keep in the same place. | Red Queen | [II](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0002) |
| G8 | you must run at least twice as fast as that! | Red Queen | [II](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0002) |
| G9 | I should advise you to walk the other way. | the Rose | [II](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0002) |
| G10 | This must be the wood, … where things have no names. | Alice | [III](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0003) |
| G11 | I'm a Fawn! | the Fawn | [III](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0003) |
| G12 | Contrariwise, | Tweedledee | [IV](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0004) |
| G13 | if it was so, it might be; and if it were so, it would be; but as it isn't, it ain't. That's logic. | Tweedledee | [IV](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0004) |
| G14 | Ditto / Ditto, ditto | Tweedledum / Tweedledee | [IV](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0004) |
| G15 | He's dreaming now, … Why, about you! | Tweedledee | [IV](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0004) |
| G16 | you're only a sort of thing in his dream! | Tweedledee | [IV](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0004) |
| G17 | one's memory works both ways. | White Queen | [V](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0005) |
| G18 | It's a poor sort of memory that only works backwards, | White Queen | [V](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0005) |
| G19 | I haven't pricked it yet, … but I soon shall—oh, oh, oh! | White Queen | [V](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0005) |
| G20 | Things flow about so here! | Alice | [V](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0005) |
| G21 | whenever she looked hard at any shelf | narration (the Sheep's shop) | [V](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0005) |
| G22 | that particular shelf was always quite empty | narration | [V](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0005) |
| G23 | fourteen pairs | narration (the Sheep's needles) | [V](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0005) |
| G24 | sometimes like a doll and sometimes like a work-box | narration | [V](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0005) |
| G25 | When I use a word, … it means just what I choose it to mean—neither more nor less. | Humpty Dumpty | [VI](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0006) |
| G26 | When I make a word do a lot of work like that, … I always pay it extra. | Humpty Dumpty | [VI](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0006) |
| G27 | That seems to be done right— | Humpty Dumpty | [VI](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0006) |
| G28 | All the King's horses and all the King's men / Couldn't put Humpty Dumpty in his place again. | the rhyme, as Alice recites it | [VI](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0006) |
| G29 | it's my crown all the while! | White King | [VII](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0007) |
| G30 | It's my own invention | White Knight | [VIII](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0008) |
| G31 | The name of the song is called 'Haddocks' Eyes.' | White Knight | [VIII](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0008) |
| G32 | Which do you think it was? | narration | [XII](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0012) |
| G33 | Life, what is it but a dream? | closing poem | [after XII](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0012) |

**SICP** (Abelson and Sussman with Julie Sussman, 2nd ed., MIT Press,
1996). CC BY-SA 4.0, per the copyright page `book-Z-H-2.html`. Base:
<https://mitp-content-server.mit.edu/books/content/sectbyfn/books_pres_0/6515/sicp.zip/full-text/book/>

| ID | Exact text | Where | Page |
|---|---|---|---|
| S1 | to the spirit that lives in the computer. | dedication | [book-Z-H-3.html](https://mitp-content-server.mit.edu/books/content/sectbyfn/books_pres_0/6515/sicp.zip/full-text/book/book-Z-H-3.html) |
| S2 | In effect, we conjure the spirits of the computer with our spells. | ch. 1 opening | [book-Z-H-9.html#%_chap_1](https://mitp-content-server.mit.edu/books/content/sectbyfn/books_pres_0/6515/sicp.zip/full-text/book/book-Z-H-9.html#%25_chap_1) |
| S3 | The evaluator, … is just another program. | ch. 4 opening | [book-Z-H-25.html#%_chap_4](https://mitp-content-server.mit.edu/books/content/sectbyfn/books_pres_0/6515/sicp.zip/full-text/book/book-Z-H-25.html#%25_chap_4) |
| S4 | Exercise 4.15 (`halts?`): cited by number, not quoted | §4.1.5 | [book-Z-H-26.html#%_thm_4.15](https://mitp-content-server.mit.edu/books/content/sectbyfn/books_pres_0/6515/sicp.zip/full-text/book/book-Z-H-26.html#%25_thm_4.15) |
| S5 | Time Is of the Essence | §3.4 title | [book-Z-H-23.html#%_sec_3.4](https://mitp-content-server.mit.edu/books/content/sectbyfn/books_pres_0/6515/sicp.zip/full-text/book/book-Z-H-23.html#%25_sec_3.4) |

**K&R** (*The C Programming Language*): no register, because nothing is
quoted at length. The story uses `hello, world` (the first program's
output, §1.1) and "C is not a big language" (preface to the 2nd ed.). Both
are five words or fewer, as homage.

---

## Boot and GRUB

### boot.splash.dmesg
where: new boot-start screen under `Banner("boot")` (`./internal/flavor/banner.go:17`, drawn nowhere yet); reveal one line at a time like the crash screen (`./internal/screens/boot/crash.go`, `tick`) · when: every boot, while door 1 fetches · pick: block · uses: A19 (allusion)
```text
[    0.000] katharsis boot #{boot} (k&r@bell-labs) (cc, 1978)
[    0.000] Command line: ro quiet rabbit=late cat=grinning
[    0.001] Calibrating delay loop... the Rabbit says: late
[    0.004] Memory: plenty of room (the tea party disagrees)
[    0.013] eval: apply: eval: apply: ok (metacircular)
[    0.020] cpu: entering ring {ring}: {place}
[    0.031] random: crng init done; the Cat is grinning
[    0.042] Run /sbin/init as init process: you
```

### boot.motd
where: one line under the dmesg block; also a suffix on the GRUB status line (`./internal/screens/meta/grub.go:99`) · when: every · pick: pool · uses: S1, S2, A55, G1 (fragment), K&R preface (5 words)
```text
The Rabbit is late. The Rabbit is always late. Boot anyway.
hello, world. The old gods never needed an exclamation mark.
Rings count backwards: the lower you go, the more you may do.
Every boot is a fall. Some falls end. That's the whole question.
The Cat is grinning. It won't say at what.
The Queen has sharpened something. Probably the test data.
Forecast: brillig, with slithy toves in the afternoon.
"C is not a big language." Neither is this rabbit-hole. Mostly.
Tea is at six. It is always six.
"Curiouser and curiouser!" Every dmesg, if you read it long enough.
The old gods wrote one thin book and left. It was enough.
The Wizards are still passing the ball. Eval. Apply. Eval.
"In effect, we conjure the spirits of the computer with our spells."
"...to the spirit that lives in the computer." (SICP, dedication)
```

### grub.subtitle.prestige
where: a faint second line under each `katharsis, prestige N` entry (`./internal/screens/meta/grub.go:103`, `bootEntry`) · when: every · pick: fixed, line N for prestige N (0 to 4) · uses: G10 (paraphrase)
```text
ring 3 · Userland · down the rabbit-hole
ring 2 · through the looking-glass · things go the other way
ring 1 · the wood where things have no names
ring 0 · the Court of Hearts · sentence first, verdict afterwards
ring -1 · the Red King's dream · beware the Jabberwock
```

### grub.subtitle.prestige.beyond
where: as above, prestige 5 and up (`PrestigeName` already handles these) · when: every · pick: fixed
```text
ring {ring} · nobody who went there wrote it down
```

### grub.subtitle.locked
where: a locked prestige entry, if GRUB shows the gate (`./docs/design/ux-journey.md` J1) · when: every · pick: fixed
```text
ring {ring} · locked · {n} rated doors to go · the Red Queen runs
```

### grub.subtitle.nightly
where: `katharsis (nightly build …)` (`grub.go:107`) · when: every · pick: fixed · uses: Humpty's un-birthday (ch. VI, allusion)
```text
an un-birthday present: one seed for everyone, today
```

### grub.subtitle.rtfm
where: `katharsis (RTFM — learn mode)` (`grub.go:110`) · when: every · pick: fixed · uses: A41 (fragment)
```text
the Mock Turtle's school · Reeling and Writhing, to begin with
```

### grub.subtitle.custom
where: `katharsis (other presets…)`: short, patch and custom presets (`grub.go:198`) · when: every · pick: fixed · uses: G25 (paraphrase)
```text
a preset means just what you choose it to mean
```

### grub.subtitle.coredump
where: `upsolve core dump: …` (`grub.go:120`) · when: every · pick: fixed · uses: A18 (fragment)
```text
a grin without a cat · {bytes} still on {title}
```

### grub.subtitle.regressions
where: `regressions due: N` (`grub.go:132`) · when: every · pick: fixed
```text
the White Queen remembers the week after next
```

### grub.subtitle.gitlog
where: `git log` · when: every · pick: fixed · uses: A8
```text
"There ought to be a book written about me, that there ought!"
```

### grub.subtitle.top
where: `top` · when: every · pick: fixed · uses: A10
```text
"Who are you?" said the Caterpillar.
```

### grub.subtitle.poweroff
where: `poweroff` · when: every · pick: fixed · uses: A54
```text
"Wake up, Alice dear!" The Rabbit will wait. Late, but he'll wait.
```

---

## Story beats

### story.prologue
where: new full screen before the first route choice of boot #1; any key continues · when: once (`Meta.Boots == 0`) · pick: block · uses: A1, A3; line 3 parodies ch. I ("what is the use of a book … without pictures or conversations?")
```text
Ring 3. A hot afternoon. You are PID 1, and you are bored.
You are reading a man page. It has no EXAMPLES section.
What is the use of a man page without examples or conversations?
A White Rabbit runs past, takes a watch out of its waistcoat-pocket,
and says to itself: "Oh dear! Oh dear! I shall be late!"
It dives into a syscall. You follow. Nobody checks your privileges.
Down, down, down. Would the fall never come to an end?
(Turing left a note about that. You don't read it yet.)
You land in a hall of doors, all locked, each with a timer.
Over the first, in an old terse hand: hello, world
Somewhere a Queen is shouting about heads.
Pick a door. The Rabbit is already late.
```

### door.first.inscription
where: doors screen header (`./internal/screens/boot/doors.go:91`), boot #1 room 1 · when: once · pick: fixed
```text
Carved over the first door, in lowercase: hello, world
```

### story.first-merge
where: merge window, above the commit row (`./internal/screens/boot/merge.go:124`) · when: once · pick: block · uses: A19, A26
```text
A long table under a tree. Three seats taken. A sign: MERGE WINDOW.
"No room! No room!" cry the Hatter and the March Hare.
There's plenty of room. Sit down. Commit what you carry.
"It's always six o'clock now," sighs the Hatter. Time to commit.
```

### daemon.intro
where: merge window, above the daemon offers (`merge.go:160`), the first time daemons are offered · when: once · pick: block · uses: G30
```text
A knight falls off his horse, climbs back on, and bows.
"It's my own invention," he says, of everything. Pick one.
Each one works. Each one has a catch. Mind the upside-down box.
```

### story.first-boss
where: the first boss door card (`doors.go:160`) · when: once · pick: block · uses: A48, A49
```text
A trumpet. The White Rabbit unrolls a scroll and puts on spectacles.
"Where shall I begin, please your Majesty?" he asks.
"Begin at the beginning," the King says gravely, "and go on till you
come to the end: then stop."
Easier said than proven. Here comes the bug.
```

### story.first-crash
where: crash screen, under the sourced quote and above the core-dump line (`crash.go:172`) · when: once · pick: block · uses: A18; line 4 is a *parody* of the Cat's "if you only walk long enough"
```text
The Cat appears, grin first. "You crashed. Everyone does, here."
"You left your bytes on that problem: a grin without a cat."
"Upsolve it and the grin comes back. Crash first, and it's gone."
"You're sure to get somewhere," says the Cat, "if you boot enough."
```

### story.first-rtfm
where: RTFM browser, above the module list (`./internal/screens/learn/browser.go`) · when: once · pick: block · uses: A39, A41, A42
```text
A Gryphon asleep in the sun. A Mock Turtle, sighing on a rock.
"We had the best of educations," says the Mock Turtle.
"Reeling and Writhing, of course, to begin with," it goes on, "and
then the different branches of Arithmetic—Ambition, Distraction,
Uglification, and Derision."
```

### story.first-regression
where: doors screen, the first time a regression card is on offer (`doors.go:299`) · when: once · pick: block · uses: G17
```text
A Queen in a shawl sticks plaster on a finger she hasn't pricked.
"...one's memory works both ways," says the White Queen, kindly.
She remembers best the things that happen the week after next.
This door crashed you. It's back, with a blank file. Then again.
```

### reveal.prestige.1
where: cleared screen, under `★ prestige unlocked` (`./internal/screens/boot/cleared.go:117`) · when: once, the clear that unlocks Silver · pick: block · uses: G6, G7
```text
The mirror over the mantel goes soft, like gauze. You step through.
Same rooms. Only the things go the other way.
The Red Queen, already running: "Now, here, you see, it takes all
the running you can do, to keep in the same place."
The doors will learn you. That isn't a bug. That's ring 2.
```

### reveal.prestige.2
where: as above, the clear that unlocks Gold · when: once · pick: block · uses: G10 (allusion), G11
```text
Ring 1 is a wood where things have no names. Symbols: stripped.
A Fawn walks beside you, nuzzling, until the symbols load.
"I'm a Fawn!" it cries. It reads /proc/1/comm: you. It bolts.
On a wall at the edge of the wood sits an egg, defining words.
```

### reveal.prestige.3
where: as above, the clear that unlocks Platinum · when: once · pick: block · uses: A53 (fragment), S3
```text
Ring 0. The eighth square. You are root, which here means Queen.
The Court of Hearts is nothing but a pack of cards. Test data.
Behind the throne two robed Wizards toss a glowing ball: eval, apply.
"The evaluator," says one, "... is just another program."
Somewhere below ring 0, somebody is snoring.
```

### reveal.prestige.4
where: as above, the clear that unlocks Advanced · when: once · pick: block · uses: G15, S4 (by number), G2 (allusion)
```text
Below ring 0 is ring -1. The Red King sleeps there, in a heap.
"He's dreaming now," says Tweedledee. "... Why, about you!"
The Wizards keep his dream running: eval, apply, eval, apply.
Long ago they set Exercise 4.15: write halts?, then show you can't.
It got loose. It has jaws that bite and claws that catch.
```

### reveal.final-boss
where: the first `halting-problem` door card (`doors.go:160`); later visits use `boss.enter.halting-problem` · when: once · pick: block · uses: G2
```text
The last door. Something burbles in the tulgey wood.
"Beware the Jabberwock, my son!
The jaws that bite, the claws that catch!"
It is the Halting Problem. You can't win in general. Win this once.
```

### reveal.final-clear
where: cleared screen, under `★ highest prestige cleared` (`cleared.go:119`) · when: once, the first clear at max prestige · pick: block · uses: G5, G32, G33
```text
"O frabjous day! Callooh! Callay!" The Jabberwock is down.
You didn't solve the Halting Problem. You halted. Turing allows that.
Did you dream it, or did the Red King? Which do you think it was?
On the last door, in the same old hand as the first: hello, world
Life, what is it but a dream? Reboot.
```

---

## Milestone beats

Draft by the S9 wiring thread for the pacing in
`docs/research/retention.md` RT6; the storyteller should revise. These
fill the dry spells after the first ten boots. Each is earned by a
cumulative learning act, never by a date, a streak or bytes (RT6, GR2),
and shows at a boot end (crash or cleared) only after
`flavor.DefaultBeatGap` (8) boot ends in a row went by without a new beat.
At most one beat shows per boot end, and a triggered beat (first crash, a
reveal) goes first. Each ends on a small question. The rated-door thread
is the note the prologue mentions ("Turing left a note about that"); the
final clear answers it. The count in a first line is a floor: the beat may
show some doors later.

where: boot end, as a teaser line (the first line) with a key to read the
rest (RT1's kept block); reread in `/proc/story` · when: once · pick:
block, except `milestone.capability`

| Hook | Earned at |
|---|---|
| `milestone.doors.N` | N rated doors (`Meta.Rated`): 10, 25, 50, 75, 100, 150, 200, 250, 300, 400, 500 |
| `milestone.upsolve.N` | N upsolves of any kind: 5, 25 |
| `milestone.gate.half` | half the rated doors the first prestige gate needs (`Rules.CalibAfter`), still at prestige 0 and short of it |
| `milestone.gate.open` | all of them, still at prestige 0 |
| `milestone.lib.1` | the first lib |
| `milestone.capability` | each capability, in the order earned; a 2-line pool keyed by the capability |
| `milestone.mastered.1` | the first topic mastered (D24): `Rules.MasteryAttempts` rated attempts on a topic other than mixed, so its doors well under its rating hide their tags |

"First regression retired" (RT6) has no beat yet: `Meta` keeps no record
of a regression that passed its last stage (`advanceRegression` deletes
it), so no trigger can see one.

### milestone.doors.10
```text
Ten doors in, under a doormat: a note, typed, folded very small.
On the outside, in pencil: halts? Who asks a doormat that?
```

### milestone.upsolve.5
```text
Five upsolves. The Cat keeps a ledger now: grins lost, grins found.
It won't show you the totals. What is it saving them for?
```

### milestone.doors.25
```text
Twenty-five doors in, the note's initials come clear: A. M. T., 1936.
The Rabbit reads it over your shoulder and drops his watch. Why?
```

### milestone.gate.half
```text
The mirror over the mantel has gone cloudy, like breath on glass.
Half the rated doors it wants are behind you. What's it waiting for?
```

### milestone.doors.50
```text
Fifty doors in, you unfold the note once: a tape, a head, a table.
"That's the judge," says the Cat. "Or its grandfather." Is it?
```

### milestone.gate.open
```text
The glass has gone soft, like gauze. Clear a boot to step through.
Is the other side the same room? The Red Queen won't stop to say.
```

### milestone.lib.1
```text
Your first lib. Humpty Dumpty, on his wall, insists on naming it.
He'd call it Humpty. You didn't. Does a name change what it means?
```

### milestone.capability
pick: 2-line pool, seeded by the capability
```text
The Gryphon stamps a certificate: {cap}. Another road opens.
The Mock Turtle sobs: lessons lessen. How short can one get?

{cap}, earned. The Mock Turtle weeps: pride, or salt water.
The Gryphon can't tell which. Can you?

The Gryphon pins {cap} on a crowded wall.
What's under the oldest certificate? The Mock Turtle won't look.
```

### milestone.mastered.1
```text
One topic knows you now. Its doors well under you hide their tags.
The Cat will still read them out, for a duck. Will you need it to?
```

### milestone.doors.75
```text
Seventy-five doors in, the second fold asks one thing of any machine:
does it ever stop? The Rabbit stops winding his watch. Does it?
```

### milestone.doors.100
```text
A hundred doors in, the note answers itself: not for every machine.
The Rabbit winds his watch again. For one machine, then? Which one?
```

### milestone.upsolve.25
```text
Twenty-five upsolves. The ledger is full, so the Cat eats it.
"Every grin you take back," it says, "I keep a copy." A copy of what?
```

### milestone.doors.150
```text
A hundred and fifty doors, and every one stopped. You checked each.
The Queen calls that a proof. The Cat calls it a sample. Who's right?
```

### milestone.doors.200
```text
Two hundred doors in, something under the floor snores in 4/4 time.
The judge's clock keeps the beat. Who sleeps through every verdict?
```

### milestone.doors.250
```text
Two hundred and fifty doors in, the note has one more fold. Inside,
a sketch of PID 1, falling. It stops before you land. On purpose?
```

### milestone.doors.300
```text
Three hundred doors. The Wizards have heard about your note.
"An old question," says Eval. "Older than us," says Apply. How old?
```

### milestone.doors.400
```text
Four hundred doors. "We set it as an exercise once," says Eval.
"Four-fifteen," says Apply. "It got out." What got out?
```

### milestone.doors.500
```text
Five hundred doors. Unfolded flat, the note is the size of a door.
On the back, in an old terse hand: hello, world. Who carved it first?
```

---

## Biomes

### biome.intro.<topic>
where: doors screen header on the biome's first room (`doors.go:91`), or a one-line toast when the route is taken; `BiomeBlurb` stays on the route card · when: every biome start · pick: fixed, one per topic · uses: listed per line

| Hook | Uses |
|---|---|
| `biome.intro.complete-search` | A4 (allusion) |
| `biome.intro.simulation` | A49 (paraphrase) |
| `biome.intro.sorting-greedy` | A29 |
| `biome.intro.prefix-sums` | A43 (allusion) |
| `biome.intro.binary-search` | ch. V mushroom (allusion) |
| `biome.intro.two-pointers` | A31 |
| `biome.intro.graphs` | A15 |
| `biome.intro.trees` | ch. VII, "a door leading right into it" (allusion) |
| `biome.intro.shortest-paths` | G9 |
| `biome.intro.dsu` | G29 |
| `biome.intro.dp` | A11 |
| `biome.intro.math` | A42 (fragment) |
| `biome.intro.strings` | G1 |
| `biome.intro.data-structures` | A38 |
| `biome.intro.geometry` | G6 |
| `biome.intro.mixed` | A16 (fragment) |

The lines, in table order:

```text
Doors all round the hall, all locked. Alice tried every one. Go on.
The King's spec: begin at the beginning, go on to the end, then stop.
"It's very easy to take more than nothing." Greedy, if it's provable.
Ten hours the first day, nine the next. Sum once; answer in O(1).
The Caterpillar's mushroom: one side taller, one side shorter. Bisect.
"Let's all move one place on." Two cursors round the tea table.
"That depends a good deal on where you want to get to," said the Cat.
One of the trees has a door in it. cd in, and remember the way out.
"I should advise you to walk the other way." The Rose knew a shortcut.
"It's my crown all the while!" says the King. find() agrees.
"I know who I was when I got up this morning." dp[0], at least.
"Ambition, Distraction, Uglification, and Derision." Mod 1e9+7.
'Twas brillig, and the slithy toves. Tokenize that.
"The more there is of mine, the less there is of yours." -- malloc
"Only the things go the other way." Mirrors flip every cross product.
"We're all mad here." Ring 3: every topic, no privileges, no excuses.
```

---

## Bosses

Boss names and blurbs stay as they are in `./internal/flavor/names.go:35`.
The entrance line goes on the boss door card under the spaced name
(`doors.go:160`). The defeat line goes on the AC screen after a boss clear:
the upgrade pick for a mid-boot boss, the cleared screen for the last one.
The pick is a pool for both.

### boss.enter.off-by-one
uses: G12, G13 (fragment)
```text
Tweedledum and Tweedledee, back to back. Count them. Count again.
"Contrariwise," says Tweedledee. Your loop stops at n. Or at n-1.
"If it was so, it might be; and if it were so, it would be."
```

### boss.defeat.off-by-one
uses: G14, G13 (fragment)
```text
"Ditto," says Tweedledum. "Ditto, ditto," says Tweedledee. Accepted.
Two twins, one gap between. You counted the fenceposts this time.
"But as it isn't, it ain't. That's logic." So is your loop bound.
```

### boss.enter.heisenbug
uses: G21, G22 (allusion), G20, G23
```text
The Sheep's shop: look hard at any shelf and it's quite empty.
"Things flow about so here!" It fails on the judge. Never on yours.
The Sheep knits with fourteen pairs of needles. None is printf.
```

### boss.defeat.heisenbug
```text
You stopped looking hard. The shelf stayed full. Accepted.
Uninitialised all along. The Sheep sniffs and goes back to knitting.
```

### boss.enter.oom-killer
uses: ch. IV (allusion)
```text
You drank the bottle. Now you fill the house, one foot up the chimney.
Something scrabbles down the flue. The Rabbit has sent in a Bill.
The OOM Killer scores every process by size. You're the largest here.
```

### boss.defeat.oom-killer
uses: A7
```text
One sharp kick, and a general chorus of "There goes Bill!"
You shrank to fit the house. RSS: modest. The Rabbit: still late.
```

### boss.enter.race-condition
uses: A5, A35, A36, S5
```text
The Dodo marks a course, in a sort of circle. Nobody says "go".
A Caucus-race: everyone starts when they like, stops when they like.
The players all played at once without waiting for turns. Croquet.
SICP 3.4: "Time Is of the Essence". The Dodo never read it.
```

### boss.defeat.race-condition
uses: A6
```text
"Everybody has won, and all must have prizes." You hold the lock.
The race is over. You know, because you joined every thread.
```

### boss.enter.undefined-behavior
uses: A12
```text
The Duchess's kitchen: pepper in the air, a cook throwing saucepans.
"There's certainly too much pepper in that soup!" Nasal demons, too.
Signed overflow in the soup. Anything may happen. Mostly sneezing.
```

### boss.defeat.undefined-behavior
uses: A13
```text
"It makes rather a handsome pig, I think." The compiler agreed. AC.
You bounds-checked the soup. The sneezing stops. The pig stays a pig.
```

### boss.enter.halting-problem
pick: 2-line pool · uses: G2, S4
```text
"Beware the Jabberwock, my son!
The jaws that bite, the claws that catch!"

Exercise 4.15, escaped. It eats any procedure named halts?.

It won't tell you whether it ends. It won't tell you anything. Begin.
```

### boss.defeat.halting-problem
pick: 2-line pool · uses: G3, G4
```text
One, two! One, two! And through and through
The vorpal blade went snicker-snack!

He left it dead, and with its head
He went galumphing back.

You didn't decide halting. You decided this one. The proof survives.
```

---

## Merge window

### merge.line
where: merge window header line (`merge.go:124`) · when: every · pick: pool · uses: A26, A27, A28, A22, A23, A24, A20
```text
"It's always six o'clock now." So it's always time to commit.
"It's always tea-time." No time to wash the cups between merges.
The Hatter moves round "as the things get used up." A ring buffer.
"Why is a raven like a writing-desk?" Nobody knows. Ship it anyway.
"Two days wrong!" The Hare, meekly: "It was the best butter."
The Dormouse is asleep again. It's a daemon. That's the job.
"Have some wine," says the March Hare. There's only tea. Commit tea.
Linus closes his merge window after two weeks. This one stays at six.
rc1 is at six o'clock. So is rc2. Time stopped at tea.
```

### merge.full
where: daemon section when held daemons are at `MaxDaemons` (`merge.go:160`) · when: every · pick: fixed · uses: A19
```text
"No room! No room!" Every daemon seat is taken. Swap one, or don't.
```

### merge.nothing
where: commit row with nothing uncommitted (`merge.go:142`) · when: every · pick: fixed · uses: *parody* of A21 ("I don't see any wine" / "There isn't any")
```text
"I don't see any bytes." "There aren't any," says the March Hare.
```

### merge.commit
where: status line after `c` commits · when: every · pick: fixed · uses: A30
```text
Committed. The Dormouse murmurs "Treacle," and goes back to sleep.
```

### merge.uncommitted
where: new, leaving the window with bytes uncommitted (`./docs/design/ux-journey.md` J2) · when: every · pick: fixed
```text
The Hatter waves a cup at your pockets: {bytes} still uncommitted.
```

### merge.leave
where: the `leave →` row (`merge.go:172`) · when: every · pick: fixed · uses: A32
```text
"At any rate I'll never go there again!" Next biome. Same table.
```

---

## Crash

Epitaphs sit under the sourced quote on the kernel panic screen
(`crash.go:172`), in faint text. They add to the quote and do not replace
it. `cause` is `game.Crash.Cause`. A cursed-door crash (`Crash.Cursed`) uses
`crash.epitaph.cursed` instead of out-of-lives. Out-of-lives with a known
last verdict prefers the verdict pool and falls back to the general one.

### crash.epitaph.timed-out
pick: pool · uses: A2, A26 (allusion)
```text
"Oh my ears and whiskers, how late it's getting!" Later than that.
The Rabbit was late. You were later. The clock is not a mutex.
The Hatter says it's always six o'clock. Your timer disagreed.
And the moral of that is: the timer started before you did.
```

### crash.epitaph.out-of-lives
pick: pool · uses: ch. VIII, the Queen's "one way" (allusion), A47
```text
The Queen has only one way of settling all difficulties. Guess which.
The jury wrote it on their slates: wrong, wrong, wrong. Sentence.
And the moral of that is: the samples are not the test data.
"Consider your verdict," said the King. The Queen already had.
```

### crash.epitaph.out-of-lives.<verdict>
pick: fixed per verdict (`TLE`, `MLE`, `RE`, `WA`; OLE folds into MLE as in `normVerdict`) · uses: A25, A50, ch. VIII executioner (allusion), A33 (*parody*)
```text
"He's murdering the time! Off with his head!" As charged.
Rule Forty-two. All persons more than a mile high to leave the court.
The executioner needs a body to cut the head from. You had a null.
"Off with its head!" Wrong answer, and no heads left to spare.
```

### crash.epitaph.gave-up
pick: pool · uses: A53, A54, A34, A9
```text
"You're nothing but a pack of cards!" The whole pack rose up. Fair.
"Wake up, Alice dear!" The editorial is open. Nobody's judging. Yet.
"Nonsense!" you told the Queen, and she was silent. The doors weren't.
"I almost wish I hadn't gone down that rabbit-hole—and yet—and yet—"
```

### crash.epitaph.cursed
pick: pool · uses: A52
```text
"Sentence first—verdict afterwards." The cursed door kept its word.
No Cat on a cursed door. Not even the grin.
One wrong answer, one head. The Queen keeps very tidy books.
```

### coredump.recovered
where: after an upsolve recovers the dump (the existing `core dump recovered: +N B banked` message) · when: every · pick: fixed · uses: A18 (allusion)
```text
The grin comes back, and a cat behind it: +{bytes} banked.
```

### coredump.lost
where: new, when a crash overwrites an unrecovered dump (`./docs/design/ux-journey.md`, boot 8) · when: every · pick: block · uses: G28
```text
All the King's horses and all the King's men
Couldn't put Humpty Dumpty in his place again.
{bytes} of core dump, gone with him.
```

---

## Cleared

Lines sit under the `cleared` banner (`cleared.go:77`), above the stats.
The pick is a pool keyed by the prestige just cleared. A prestige unlock
shows `reveal.prestige.<p+1>` as well, the first time.

### cleared.0
uses: A6
```text
"Everybody has won, and all must have prizes." Just you, actually.
Userland cleared. The rabbit-hole goes deeper. It always does.
exit 0. The Rabbit checks his watch: late, but inside the limit.
```

### cleared.1
uses: G7 (allusion)
```text
You ran as fast as you could and stayed in the same place. Progress!
Ring 2 cleared. OS/2 lived here once. It left no forwarding address.
Through the glass and back. The doors noticed. They're learning you.
```

### cleared.2
uses: G27
```text
The nameless wood, cleared. Your symbols come back. Most of them.
Humpty checks your work: "That seems to be done right—" Upside down.
You found the creature wearing your old name. It was a stack frame.
```

### cleared.3
```text
Ring 0 cleared. You're root. The Queen curtsies. It looks painful.
Queen Alice, eighth square. Mind the crown: it's only a pointer.
Below ring 0, something snores. The Wizards hush you. Don't wake it.
```

### cleared.4
uses: G5, G33
```text
"O frabjous day! Callooh! Callay!" It halted. This once.
Halting: undecidable in general. This boot: decided.
Which dreamed it: you, or the Red King? git log says you.
Life, what is it but a dream? exit 0.
There's talk of a ring -2. The Wizards change the subject.
```

### cleared.beyond
where: prestige 5 and up, if `MaxPrestige` ever rises
```text
Ring {ring}. The Management Engine waves. Nobody knows what it does.
Eval hands the ball to Apply. Apply hands it back. Again?
```

---

## Rubber duck (the Cheshire Cat)

Where: the HUD after `:Quack` or `?` (`./internal/screens/boot/hudbar.go:108`
to `:123`). The mechanic text there (tags, technique) stays, and the quip
goes above it. Pick: pool.

### duck.quack.1
first quack, reveals tags · uses: A14 (fragment)
```text
Grin first, then Cat: "A {tags} sort of problem."
The grin widens. On the wall, faintly: {tags}
"Cheshire Puss?" It only grins wider. Tags: {tags}
```

### duck.quack.2
second quack, reveals the technique · uses: A16 (fragment)
```text
The Cat vanishes slowly, tail first. The grin stays: {technique}.
The grin hangs in the air a while, spelling {technique}.
"I'm mad. You're mad." The Cat taps the wall: {technique}.
```

### duck.empty
no ducks left
```text
You whistle for the Cat. Only the grin comes, and it won't talk.
"Cheshire Puss?" The tree is empty. Out of ducks.
```

### duck.cursed
`:Quack` on a cursed door
```text
Cursed door: the Cat won't come. The Queen wants it beheaded.
No body, no head to cut off. The Cat's safe. You're on your own.
```

### duck.refill
where: merge window, when ducks refill · uses: A17
```text
"Did you say pig, or fig?" The Cat is back on its branch. Ducks: {n}
A grin in the tree, then a Cat around it. Ducks refilled: {n}
```

---

## Daemons and upgrades

Flavor goes on a faint second line under the mechanic text from
`DaemonTextFor` and `UpgradeTextFor` (`./internal/flavor/names.go:50`, `:65`).
That text stays the truth (bible tone rule 5). Pick: fixed per key.

### daemon.<key>
uses: G30 (allusion); nice parodies the Knight's upside-down box (ch. VIII)

| Hook | Line |
|---|---|
| `daemon.strace` | see block, line 1 |
| `daemon.nice` | line 2 |
| `daemon.fork` | line 3 |
| `daemon.dev-random` | line 4 |
| `daemon.kexec` | line 5 |

```text
The Knight's invention: a window onto the test that tripped you.
Carried upside-down so the rain can't get in. The lives fall out.
He rides two horses at once and falls off both. More doors, though.
A box of mystery doors. He invented it. He's never opened it.
Swap the door without getting off the horse. Once per biome.
```

### upgrade.<key>
uses: G8

| Hook | Line |
|---|---|
| `upgrade.overclock` | line 1 |
| `upgrade.ecc` | line 2 |
| `upgrade.duck` | line 3 |

```text
"You must run at least twice as fast as that!" Or {pct} faster.
One flipped bit forgiven. The Queen is not informed.
Another grin in the trees. Somebody's Cat. Yours now.
```

### o3.bonus
where: the free upgrade from an AC under half the timer (the -O3 bonus) · when: every · pick: fixed · uses: G8 (allusion)
```text
Twice as fast as that! The Red Queen nods, once. Free upgrade.
```

---

## Regressions (the White Queen)

### regression.door
where: a faint line under the card's `↺ regression: crashed you N days ago` (`doors.go:299`) · when: every · pick: pool · uses: G18, G17 (allusion), White Queen's jam rule (ch. V, paraphrase)
```text
"You again?" This door remembers you. You remember it, a bit.
"It's a poor sort of memory that only works backwards." Again, then.
Jam yesterday, jam to-morrow, never jam to-day. Solve it to-day.
It crashed you {ago}. It has been practising. Have you?
```

### regression.stage.1
where: as above, stage 1 (the 3-day return) · pick: fixed · uses: G19
```text
"I haven't pricked it yet, but I soon shall—oh, oh, oh!" Day three.
```

### regression.stage.2
where: as above, stage 2 (the 17-day return) · pick: fixed
```text
Seventeen days on, the White Queen screams before you even open it.
```

### regression.cleared
where: AC on a regression door · when: every · pick: fixed
```text
Solved, blank file and all. The White Queen forgets it, backwards.
```

---

## RTFM (the Mock Turtle's school)

### rtfm.title
where: RTFM browser header (`browser.go`; `manTitle` at `:421` stays the license line) · when: every · pick: fixed · uses: A44 (allusion)
```text
the Mock Turtle's school · the regular course · lessons lessen daily
```

### rtfm.module.intro
where: module page, under the `RTFM ▸ … ▸ …` crumb (`./internal/screens/learn/module.go:92`) · when: every · pick: pool, seeded by module · uses: A40, A42 (allusion), A44 (*parody*: the Gryphon's line with the module name added)
```text
"{module}," sighs the Mock Turtle. "Every day."
{module}: a branch of Arithmetic, says the Gryphon.
{module}. "I only took the regular course."
The Gryphon: "{module}. They lessen from day to day."
{division} · {module}. Pass for {cap}.
```

### rtfm.trial.start
where: starting a trial (`module.go:237`, `[t]`) · pick: fixed · uses: A43
```text
"Ten hours the first day," says the Mock Turtle. Your timer is soft.
```

### rtfm.trial.pass
where: trial passed · pick: fixed
```text
The Gryphon claps: {cap}. {biome} joins routes.
```

### rtfm.trial.fail
where: trial failed or abandoned · pick: fixed · uses: A46
```text
"That's enough about lessons," says the Gryphon. Try a boot instead.
```

### rtfm.underflow
where: easter egg, any RTFM footer, rare · pick: fixed · uses: A45
```text
"And how did you manage on the twelfth?" Unsigned: 4294967295 hours.
```

### rtfm.syllabus
where: an optional `CURRICULUM` section on the `rtfm(7)` page (`browser.go`, next to `COLOPHON` at `:605`) · pick: block · uses: Mock Turtle's subjects, ch. IX
```text
Reeling and Writhing .......... fast I/O
Ambition ...................... addition, and its overflow
Distraction ................... subtraction, and off-by-one
Uglification .................. multiplication, mod 1e9+7
Derision ...................... division, truncated toward zero
Mystery, ancient and modern ... the editorial
Seaography .................... geometry
Drawling, Stretching .......... grids and ASCII art
Fainting in Coils ............. painting in oils: flood fill
Laughing and Grief ............ Latin and Greek: strings
```

---

## Optional new hooks

These have no slot yet. They cost nothing to skip.

### verdict.<V>
where: the HUD status after `:Submit` (`hudbar.go:136`); the Queen as judge · pick: fixed per verdict · uses: A33 (*parody*), A25 (fragment), A50 (fragment), A51

| Hook | Line |
|---|---|
| `verdict.AC` | line 1 |
| `verdict.WA` | line 2 |
| `verdict.TLE` | line 3 |
| `verdict.MLE` | line 4 |
| `verdict.RE` | line 5 |
| `verdict.CE` | line 6 |

```text
The Queen looks for a head to remove. Finds none. Sulks.
"Off with its head!"
"He's murdering the time!"
"All persons more than a mile high to leave the court."
The executioner can't cut off a head without a body to cut it from.
"If there's no meaning in it, that saves a world of trouble."
```

### timer.warn
where: the vim border and HUD at 5:00 and 1:00 left · pick: line 1 at 5:00, line 2 at 1:00 · uses: A1, A2
```text
"Oh dear! Oh dear! I shall be late!"
"Oh my ears and whiskers, how late it's getting!"
```

### quit.warn / resume.note
where: quitting with a door open, and resuming one (D31: the door keeps running while the game is closed) · pick: fixed
```text
The Rabbit won't stop his watch for you. {time} left on this door.
Welcome back. The Rabbit kept time: {time} left.
```

### door.blind / door.mystery / door.cursed
where: a faint line under the modifier on the door card (`doors.go:275` to `:296`) · pick: fixed · uses: G10 (allusion), G24
```text
Tags hidden, as in the nameless wood. Bytes extra for the trouble.
Sometimes like a doll and sometimes like a work-box. Title hidden.
The Queen is judging this one herself. No Cat. One WA.
```

### lib.saved
where: when a `// lib:` block is harvested (README, "Libs") · pick: 2-line pool · uses: G25, G26
```text
"When I use a word, it means just what I choose it to mean—
neither more nor less." lib {name}, saved.

"When I make a word do a lot of work like that, I always pay it
extra." lib {name}, linked.
```

### help.vocab
where: the top of any help or glossary screen (a future `man katharsis`) · pick: block · uses: G25, G26
```text
"When I use a word," Humpty Dumpty said in rather a scornful tone,
"it means just what I choose it to mean—neither more nor less."
Here, boot means run, bytes means money, and panic means you died.
"When I make a word do a lot of work like that," he adds, "I always
pay it extra."
```

### white-knight.song
where: easter egg; suggested in the RTFM page's `COLOPHON`, or wherever pointers come up · pick: block · uses: G31 and the rest of the Knight's speech (ch. VIII: "The name really is 'The Aged Aged Man'", "The song is called 'Ways and Means'", "The song really is 'A-sitting On A Gate'")
```text
The White Knight explains his song, K&R-style:
  song          "A-sitting On A Gate"   what the song really is
  *called       "Ways and Means"        what the song is called
  *name         "The Aged Aged Man"     what the name really is
  **name_called "Haddocks' Eyes"        what the name is called
"It's my own invention."
```
