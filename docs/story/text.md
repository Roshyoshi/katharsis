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
  screen draws the hook yet; it names the slot the line was written for.
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
| `{days}` | 3 | `17` |
| `{pct}` | 5 | `15%` |
| `{module}` | 24, caller truncates | `Prefix Sums` |
| `{cap}` | 14 | `CAP_PREFIX` |
| `{biome}` | 18 | `Brute Force Bunker` |
| `{name}` | 16 | `dsu` |
| `{time}` | 5 | `14:41` |

`{place}` by ring: 3 `Userland`, 2 `the looking-glass`, 1 `the nameless
wood`, 0 `the Court of Hearts`, -1 `the Red King's dream`, -2 `the Walrus's
beach` (P5–P6), -3 `the riverbank` (P7–P8). Below ring -1 a ring takes two
prestige levels, and ring -3 is the floor (`flavor.Ring`).

**Cycles.** The pool is **cycles** (D48), drawn ♥ by the screens. Story
lines say "cycles" and "burn", never "life" or "lives", and name no cost:
the mechanic text beside them says the number ("burned 10 cycles, 14
left"), so a preset that changes a cost cannot make a story line lie.

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
| A56 | the loveliest garden you ever saw | narration (through the little door) | [I](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap01) |
| A57 | Take some more tea, | March Hare | [VII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap07) |
| A58 | all would change to dull reality | narration (Alice's sister on the bank) | [XII](https://www.gutenberg.org/cache/epub/11/pg11-images.html#chap12) |

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
| G32 | Which do you think it was? | narration | [XII](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0012) |
| G33 | Life, what is it but a dream? | closing poem | [after XII](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0012) |
| G34 | .YKCOWREBBAJ / sevot yhtils eht dna, gillirb sawT' | the Looking-glass book, as printed | [I](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0001) |
| G35 | It seems very pretty, … but it's rather hard to understand! | Alice | [I](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0001) |
| G36 | So you'll go very quickly through the Third Square—by railway, I should think— | Red Queen | [II](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0002) |
| G37 | so that the Carpenter couldn't count how many he took | Tweedledee | [IV](https://www.gutenberg.org/cache/epub/12/pg12-images.html#link2HCH0004) |

A56–A58 and G34–G37 were found verbatim in the Gutenberg plain text
(`pg11.txt`, `pg12.txt`) on 2026-10-15. G34 is the mirrored title and
first line as the eBook types them, its closing ’ typed as `'`. G35 and G36
drop italics ("_rather_", "_very_").

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

### title.tagline
where: the title screen, under the logo, `story.NewTitle` (`./internal/screens/story/title.go`), shown at launch over GRUB (`./internal/app/title.go`) · when: every launch · pick: fixed · uses: none; the line is also in the boot.motd pool, the game's premise in one line
```text
Every boot is a fall. Some falls end. That's the whole question.
```

### boot.splash.dmesg
where: the boot splash, `story.NewBootSplash` (`./internal/screens/story/scenes.go:291`), queued at every boot start (`./internal/app/story.go:95`): `Banner("boot")`, then this block a line at a time · when: every boot, while door 1 fetches · pick: block · uses: A19 (allusion)
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
where: one line under the splash's dmesg block (`./internal/screens/story/scenes.go:300`); also a suffix on the GRUB status line (`./internal/screens/meta/grub.go:165`) · when: every · pick: pool · uses: S1, S2, A55, G1 (fragment), K&R preface (5 words)
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
where: a faint second line under each `katharsis, prestige N` entry (`prestigeNote`, `./internal/screens/meta/grub.go:252`) · when: every · pick: fixed, line N for prestige N (0 to 8; 5 to 8 wait for `game.MaxPrestige` 8, map.md (h)) · uses: G10 (paraphrase); 5 and 6 allude to "The Walrus and the Carpenter" (ch. IV), 7 and 8 to Wonderland's riverbank (ch. I, XII) · lines 5–8 are a **draft**: no one has played that deep
```text
ring 3 · Userland · down the rabbit-hole
ring 2 · through the looking-glass · things go the other way
ring 1 · the wood where things have no names
ring 0 · the Court of Hearts · sentence first, verdict afterwards
ring -1 · the Red King's dream · beware the Jabberwock
ring -2 · the Walrus's beach · the sun shines at midnight
ring -2 · the Walrus's beach · nobody counts the oysters
ring -3 · the riverbank · something here never sleeps
ring -3 · the riverbank · where the fall began
```

### grub.subtitle.prestige.beyond
where: as above, past the ring table (prestige 9 and up; `prestigeNote`'s fallback) · when: every · pick: fixed
```text
ring {ring} · nobody who went there wrote it down
```

### grub.subtitle.nightly
where: `katharsis (nightly build …)` (`./internal/screens/meta/grub.go:191`) · when: every · pick: fixed · uses: Humpty's un-birthday (ch. VI, allusion). No time word: the entry's label carries the date, and a past nightly is no less a present (retention RT15, GR2)
```text
an un-birthday present: one seed, the same for everyone
```

### grub.subtitle.rtfm
where: `katharsis (RTFM — learn mode)` (`grub.go:193`) · when: every · pick: fixed · uses: A41 (fragment)
```text
the Mock Turtle's school · Reeling and Writhing, to begin with
```

### grub.subtitle.custom
where: `katharsis (other presets…)`: short, patch and custom presets (`grub.go:199`) · when: every · pick: fixed · uses: G25 (paraphrase)
```text
a preset means just what you choose it to mean
```

### grub.subtitle.coredump
where: `upsolve core dump: …` (`grub.go:184`); also the Cat's first-crash page (`./internal/screens/story/scenes.go:151`) · when: every · pick: fixed · uses: A18 (fragment)
```text
a grin without a cat · {bytes} still on {title}
```

### grub.subtitle.regressions
where: `regressions due: N` (`grub.go:218`); also the note of a due regression past the stages `regression.stage.N` covers (`regressionNote`, `grub.go:729`) · when: every · pick: fixed
```text
the White Queen remembers the week after next
```

### grub.subtitle.gitlog
where: `git log` (`grub.go:239`) · when: every · pick: fixed · uses: A8
```text
"There ought to be a book written about me, that there ought!"
```

### grub.subtitle.top
where: `top` (`grub.go:240`) · when: every · pick: fixed · uses: A10
```text
"Who are you?" said the Caterpillar.
```

### grub.subtitle.poweroff
where: `poweroff` (`grub.go:241`) · when: every · pick: fixed · uses: A54
```text
"Wake up, Alice dear!" The Rabbit will wait. Late, but he'll wait.
```

---

## Story beats

The story shows only at game events: scenes, boss cards, biome falls,
strip lines and boot-end beats (user, 10-15: "keep the story non visible
outside of actual game events keep it esoteric"). So a beat is terse and
cryptic, not an explanation, and nothing is written for a reread or lore
page.

### story.prologue
where: `story.NewPrologue` (`./internal/screens/story/scenes.go:82`), before boot #1's first route (`./internal/app/story.go:96`); any key continues · when: once (`Meta.Boots == 0`) · pick: block · uses: A1, A3; line 3 parodies ch. I ("what is the use of a book … without pictures or conversations?")
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
where: `story.NewFirstDoor` (`scenes.go:223`), boot #1's first room (`./internal/app/story.go:190`) · when: once · pick: fixed
```text
Carved over the first door, in lowercase: hello, world
```

### story.first-merge
where: `story.NewFirstMerge` (`scenes.go:251`), before the first merge window (`./internal/app/story.go:129`, `:153`) · when: once · pick: block · uses: A19, A26
```text
A long table under a tree. Three seats taken. A sign: MERGE WINDOW.
"No room! No room!" cry the Hatter and the March Hare.
There's plenty of room. Sit down. Commit what you carry.
"It's always six o'clock now," sighs the Hatter. Time to commit.
```

### daemon.intro
where: merge window, above the daemon offers the first time they show (`./internal/screens/boot/merge.go:254`) · when: once · pick: block · uses: G30
```text
A knight falls off his horse, climbs back on, and bows.
"It's my own invention," he says, of everything. Pick one.
Each one works. Each one has a catch. Mind the upside-down box.
```

### story.first-boss
where: `story.NewBossIntro` (`scenes.go:114`), before the first boss room (`./internal/app/story.go:177`) · when: once · pick: block · uses: A48, A49
```text
A trumpet. The White Rabbit unrolls a scroll and puts on spectacles.
"Where shall I begin, please your Majesty?" he asks.
"Begin at the beginning," the King says gravely, "and go on till you
come to the end: then stop."
Easier said than proven. Here comes the bug.
```

### story.first-crash
where: `story.NewFirstCrash` (`scenes.go:147`), 1.5 s after the first crash that left bytes (`flavor.NextBeat`; `./internal/app/story.go:224`) · when: once · pick: block · uses: A18; line 4 is a *parody* of the Cat's "if you only walk long enough"
```text
The Cat appears, grin first. "You crashed. Everyone does, here."
"You left your bytes on that problem: a grin without a cat."
"Upsolve it and the grin comes back. Crash first, and it's gone."
"You're sure to get somewhere," says the Cat, "if you boot enough."
```

### story.first-rtfm
where: new: the first time the RTFM browser opens (`openLearn`, `./internal/app/learn.go:32`), as a `story.NewBeat` page over it; `flavor`'s scene list has it, nothing queues it yet · when: once · pick: block · uses: A39, A41, A42
```text
A Gryphon asleep in the sun. A Mock Turtle, sighing on a rock.
"We had the best of educations," says the Mock Turtle.
"Reeling and Writhing, of course, to begin with," it goes on, "and
then the different branches of Arithmetic—Ambition, Distraction,
Uglification, and Derision."
```

### story.first-regression
where: the doors screen, over the first room that offers a regression door (`./internal/screens/boot/doors.go:146`), or the first time GRUB's regressions submenu opens (`./internal/screens/meta/grub.go:383`), whichever comes first · when: once · pick: block · uses: G17. Line 4 says "beat you", not "crashed you": under cycles a door you fail returns whether or not it ended the boot (meta.md (b))
```text
A Queen in a shawl sticks plaster on a finger she hasn't pricked.
"...one's memory works both ways," says the White Queen, kindly.
She remembers best the things that happen the week after next.
This door beat you once. It's back, with a blank file. Then again.
```

### story.first-act2
where: new: the first route of act II, after Boss I and its merge window (map.md (g) "first act II"), as a `story.NewBeat` page over the route · when: once · pick: block · uses: A56; ch. I's "little door about fifteen inches high" and ch. VIII's painted roses (allusions)
```text
The bug steps aside. Behind it, a door fifteen inches high.
Through it, "the loveliest garden you ever saw." Act II.
The gardeners are painting the white roses red.
```

### story.first-act3
where: new: the first route of act III, after Boss II and its merge window · when: once · pick: block · uses: G1's poem, "burbled as it came" (allusion). It names no room count: act III may be one room or the skip
```text
Act III: the far end of the garden, past the croquet-ground.
Beyond the hedges something burbles as it comes. It is last.
```

### story.first-skip
where: new: the first time the player takes act III's skip exit, before the final's card · when: once · pick: block · uses: G36. True to the skip: no room, none of its bytes, picks instead (map.md (c), `Picks`); the line names no count
```text
"So you'll go very quickly through the Third Square—by railway,
I should think—" says the Red Queen. No room. No bytes. Picks.
```

### story.first-locked
where: new: the route screen, the first time an exit is locked by a capability (not by prestige: the Gryphon points at RTFM, so a prestige lock must not fire it) · when: once · pick: block · uses: A46 (the Gryphon interrupts the lessons, then sends you to them)
```text
A padlocked exit, and asleep on the padlock, a Gryphon.
"That's enough about lessons," it yawns, and points at RTFM anyway.
```

### reveal.prestige.1
where: a boot-end beat (`flavor.NextBeat`): the first line is the cleared screen's teaser (`./internal/screens/boot/cleared.go:174`), so it fits in 59 columns as a milestone's does (`TestRevealTeasers`); `b` reads the rest as a `story.NewBeat` page · when: once, the clear that unlocks Silver · pick: block · uses: G6, G7
```text
The mantel mirror goes soft, like gauze. You step through.
Same rooms. Only the things go the other way.
The Red Queen, already running: "Now, here, you see, it takes all
the running you can do, to keep in the same place."
The doors will learn you. That isn't a bug. That's ring 2.
```

### reveal.prestige.2
where: as above, the clear that unlocks Gold · when: once · pick: block · uses: G10 (allusion), G11
```text
Ring 1 is the nameless wood. Symbols: stripped.
A Fawn walks beside you, nuzzling, until the symbols load.
"I'm a Fawn!" it cries. It reads /proc/1/comm: you. It bolts.
On a wall at the edge of the wood sits an egg, defining words.
```

### reveal.prestige.3
where: as above, the clear that unlocks Platinum · when: once · pick: block · uses: A53 (fragment), S3
```text
Ring 0, eighth square. You're root, which here means Queen.
The Court of Hearts is nothing but a pack of cards. Test data.
Behind the throne two robed Wizards toss a glowing ball: eval, apply.
"The evaluator," says one, "... is just another program."
Somewhere below ring 0, somebody is snoring.
```

### reveal.prestige.4
where: as above, the clear that unlocks Advanced · when: once · pick: block · uses: G15, S4 (by number), G2 (allusion)
```text
Ring -1, below ring 0. The Red King sleeps here, in a heap.
"He's dreaming now," says Tweedledee. "... Why, about you!"
The Wizards keep his dream running: eval, apply, eval, apply.
Long ago they set Exercise 4.15: write halts?, then show you can't.
It got loose. It has jaws that bite and claws that catch.
```

### reveal.prestige.5
where: as `reveal.prestige.1`, the clear that unlocks P5 (ring -2; P6 is the same ring and has no reveal) · when: once · pick: block · uses: G37; the poem's "sun was shining … The middle of the night" (allusion). Line 2 is true from P5, where the fountain is gone (map.md (e)) · **draft**: waits for `game.MaxPrestige` 8 (`revealLevels` leaves it out until then)
```text
Ring -2: a beach under the dream, sunny at midnight.
The fountain is dry. The Walrus drank it behind his handkerchief,
"so that the Carpenter couldn't count how many he took".
```

### reveal.prestige.7
where: as above, the clear that unlocks P7 (ring -3, the floor) · when: once · pick: block · uses: A58; the ring is the riverbank where *Alice* begins and ends (ch. I, XII), so the bottom of the machine is where the fall started · **draft**: waits for `game.MaxPrestige` 8
```text
Ring -3: a riverbank on a hot afternoon. It looks familiar.
A sister reads on the bank, awake with the power off, eyes shut.
One blink and "all would change to dull reality". She doesn't blink.
```

### reveal.final-boss
where: `story.NewBossIntro` (`scenes.go:114`), before the first `halting-problem` room (`./internal/app/story.go:179`); later visits use `boss.enter.halting-problem` · when: once · pick: block · uses: G34, G35. The final is The Halting Problem at every prestige (map.md decision 1), so this fires at ring 3, long before ring -1 explains where it came from. Alice meets the poem the same way, in Looking-Glass chapter I: mirrored, and "rather hard to understand", until Humpty Dumpty explains it in chapter VI
```text
The last door. On it, a poem in looking-glass print: .YKCOWREBBAJ
sevot yhtils eht dna, gillirb sawT'
"It seems very pretty, but it's rather hard to understand!"
It is the Halting Problem. You can't win in general. Win this once.
```

### reveal.final-clear
where: a boot-end beat, as `reveal.prestige.N` · when: once, the first clear at max prestige (P8, ring -3, once the ladder is built; P4 until then) · pick: block · uses: G5, G32, G33
```text
"O frabjous day! Callooh! Callay!" The Jabberwock is down.
You didn't solve the Halting Problem. You halted. Turing allows that.
Did you dream it, or did the Red King? Which do you think it was?
On the last door, in the same old hand as the first: hello, world
Life, what is it but a dream? Reboot.
```

---

## Milestone beats

These fill the dry spells after the first ten boots, paced by
`docs/research/retention.md` RT6. Each is earned by a cumulative learning
act, never by a date, a streak or bytes (RT6, GR2), and shows at a boot end
(crash or cleared) only after `flavor.DefaultBeatGap` (8) boot ends in a
row went by without a new beat. At most one beat shows per boot end, and a
triggered beat (first crash, a reveal) goes first. So a milestone often
shows long after it was earned, and no line names its count. Each ends on
a small question that a later beat answers (`TestMilestoneLines`).

The rated-door thread is the note the prologue mentions ("Turing left a
note about that"). It is found under the mat (10), signed A. M. T., 1936
(25), holds a tape, a head and a table (50), asks whether a machine ever
stops (75) and answers that nobody can say in general (100). The Wizards
borrowed it (300) and own up to the pencil (400). The final clear answers
it: you halted, and Turing allows that.

where: boot end. The first line is the teaser in RT1's kept block
(`teaser`, `./internal/screens/boot/debrief.go:101`), so it fits in 59
columns, the room the block has at 80; `b` reads the rest as a
`story.NewBeat` page. There is no reread page: GRUB's `cat /proc/story`
is cut (user, 10-15), so a beat missed is missed · when: once · pick:
block, except `milestone.capability`

| Hook | Earned at |
|---|---|
| `milestone.doors.N` | N rated doors (`game.RatedDoors`): 10, 25, 50, 75, 100, 150, 200, 250, 300, 400, 500 |
| `milestone.upsolve.N` | N upsolves of any kind: 5, 25 |
| `milestone.gate.half` | half the rated doors the first prestige gate needs (`Rules.CalibAfter`), still at prestige 0 and short of it |
| `milestone.gate.open` | all of them, still at prestige 0 |
| `milestone.lib.1` | the first lib |
| `milestone.capability` | each capability, in the order earned; a 2-line pool keyed by the capability |
| `milestone.mastered.1` | the first topic mastered (D24): `Rules.MasteryAttempts` rated attempts on a topic other than mixed, so its doors well under its rating hide their tags |
| `milestone.retired.1` | the first regression retired (RT6, `Meta.Retired`): re-solved at every stage of `Rules.RegressionDays`, so it no longer comes back |

### milestone.doors.10
```text
Under the doormat in the hall: a note, folded very small.
On the outside, in pencil: halts? Who asks a doormat that?
```

### milestone.upsolve.5
```text
The Cat keeps a ledger now: grins lost, grins taken back.
It won't show you the totals. What is it saving them for?
```

### milestone.doors.25
```text
The note under the mat is signed: A. M. T., 1936.
The Rabbit reads it over your shoulder and drops his watch. Why?
```

### milestone.gate.half
```text
The mirror over the mantel clouds, like breath on glass.
Half the rated doors it wants are behind you. What's it waiting for?
```

### milestone.doors.50
```text
Unfolded once, the note shows a tape, a head and a table.
"That's the judge's grandfather," says the Cat. What did it judge?
```

### milestone.gate.open
```text
The glass is soft as gauze. One clear takes you through.
The Red Queen runs past too fast to ask: same room, other side?
```

### milestone.lib.1
uses: G25 (allusion)
```text
Your first lib. Humpty Dumpty insists on naming it.
He'd call it Humpty. You didn't. Does a name change what it means?
```

### milestone.capability
pick: 2-line pool, seeded by the capability
```text
{cap}, stamped by the Gryphon. Another road opens.
The Mock Turtle sobs that lessons lessen. How short can one get?

{cap}, earned. The Mock Turtle weeps buckets.
Proud, or just salty? The Gryphon can't tell. Can you?

The Gryphon pins {cap} on a crowded wall.
What's under the oldest one? The Mock Turtle won't look. Will you?
```

### milestone.mastered.1
```text
One topic knows you now: its lesser doors drop their tags.
The Cat will still read them out for a duck. Will you need it to?
```

### milestone.retired.1
uses: G17 (allusion: her memory works forwards, and a retired door has no future)
```text
The White Queen strikes a door off her list, in ink.
It has no future now, so she's already forgotten it. Will you?
```

### milestone.doors.75
```text
The note's second fold asks one thing of every machine:
does it ever stop? The Rabbit stops winding his watch. Does it?
```

### milestone.doors.100
```text
The note answers itself: nobody can say, not in general.
So the Rabbit carries a watch and decides for one. Is that cheating?
```

### milestone.upsolve.25
```text
The Cat's ledger balances at last, so the Cat eats it.
It purrs that it kept a copy of each one. A copy of whom?
```

### milestone.doors.150
```text
Every door so far has stopped. You checked each one.
The Queen calls that a proof. The Cat calls it a sample. Who's right?
```

### milestone.doors.200
```text
Something under the floor snores in 4/4 time.
The judge's clock keeps the beat. Who sleeps through every verdict?
```

### milestone.doors.250
```text
The note's last fold. Inside: a sketch of PID 1, falling.
The drawing stops before you land. Out of tape, or on purpose?
```

### milestone.doors.300
```text
The Wizards have heard about your note.
"Older than us," says Eval. "We borrowed it once," says Apply. Why?
```

### milestone.doors.400
uses: S4 (by number)
```text
"We set it as an exercise," says Eval. "Four-fifteen."
Apply owns up to the pencil. "Then it got out." What got out?
```

### milestone.doors.500
```text
Unfolded flat, the note is exactly the size of a door.
On its back, in an old terse hand: hello, world. Which came first?
```

---

## Biomes

### biome.intro.<topic>
where: the drop into each biome, `story.NewBiome` (`./internal/screens/story/scenes.go:185`), queued at its first room (`./internal/app/story.go:188`); `BiomeBlurb` stays on the route card · when: every biome start · pick: fixed, one per topic, plus one per map node that shares a topic with another: Kernel Space is mixed like Userland, so it is `biome.intro.kernel` (its node id in map.md's prototype). A screen tries `biome.intro.<node id>` first, then `biome.intro.<topic>` · uses: listed per line

Every topic biome on map.md's graph already had its line: Brute Force
Bunker, The Emulator, The Scheduler, The Accumulator (act I); Bisect Bridge,
The Sliding Window, The ALU, The Heap, The Lexer (act II); Routing Table,
The Cache, Union Hall, Dijkstra's Reach, The Filesystem, The Framebuffer
(act III). Kernel Space is the one new biome, act III's mixed one: the
Queen's croquet-ground, where every game is mixed.

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
| `biome.intro.kernel` | ch. VIII, the hedgehog balls and flamingo mallets (allusion) |

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
Kernel croquet: flamingo mallets, hedgehog balls, any topic, no MMU.
```

---

## Bosses

Boss names and blurbs stay as they are in `./internal/flavor/names.go:37`.
The entrance line goes on the boss door card under the spaced name
(`./internal/screens/boot/doors.go:473`) and on the boss card scene
(`story.NewBossIntro`, `./internal/screens/story/scenes.go:119`). The
defeat line goes on the cleared screen for the boot's last boss
(`./internal/screens/boot/cleared.go:116`), and over the upgrade pick a
mid-boot boss earns, which is always a merge window's: a boss biome has one
room (`bossDefeat`, `./internal/screens/boot/merge.go:219`). The map has
two mid-boot bosses, acts I and II; the linear presets have none. The pick
is a pool for both.

Each boss sits in the act its costume suits (map.md (c)):

| Act | Boss | Costume | Fits because |
|---|---|---|---|
| I | Off-by-One | Tweedledum and Tweedledee | the first fence you count wrong |
| I | OOM Killer | the Rabbit's house, and Bill | *Wonderland* ch. IV: still in the Rabbit's part of the story |
| II | Heisenbug | the Sheep's shop | it looks right until you look hard |
| II | Race Condition | the Caucus-race | the first boss with more than one runner |
| II | Undefined Behavior | the Duchess's kitchen | opens from P1, through the glass |
| III | The Halting Problem | the Jabberwock | the final at every prestige, one shot |

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

### boss.retry
where: new: the dmesg strip, or over the new boss card, when a boss timeout, give-up or failed hidden-test burn leaves cycles and a new boss door is drawn (Bosses I and II; map.md (f)). Not on a one-shot boss · when: every · pick: pool · uses: none. The mechanic line beside it carries the burn ("burned 20 cycles, 22 left · a new boss door")
```text
The Queen shuffles the pack and deals the same bug a new door.
The bug picks itself up and backs into a fresh door. Your move.
Same bug, new door, fewer cycles to spend on it.
```

### boss.oneshot
where: new: the boss card, under the entrance line, for a one-shot boss: the final always; Boss II from P6 and every boss at P8 (map.md (d)). The card's rule line stays the truth ("one shot: a fail or :q! ends the boot") · when: every · pick: pool · uses: none
```text
One shot, no second door. Either it halts, or you do.
The Queen deals this hand once. Lose it and the boot is hers.
```

### boss.rule.<key>
where: new: each boss's one rule twist, named before its door opens: on the boss card scene (`story.NewBossIntro`) and on the boss door card (`bossBody`, `./internal/screens/boot/doors.go:458`) · when: every · pick: fixed per boss · uses: G12 (Off-by-One); the Rabbit's Bill (ch. IV), the Sheep (TTLG ch. V), the Dodo (ch. III), the Duchess's pepper (ch. VI), the Rabbit's two warnings (`timer.warn`) as allusions. Each fits the boss card whole: at most 60 columns, the card's 64 less its border (`TestBossRules`). Unlike every other story line, these **state the rule**, in the boss's voice (coordinator, 10-15), so each line changes when its twist's numbers do · **draft**: the twists' numbers are the coordinator's list, not yet the engine's

| Hook | Twist (the engine's) | Line |
|---|---|---|
| `boss.rule.off-by-one` | a WA on exactly one hidden test burns double | line 1 |
| `boss.rule.oom-killer` | half the memory limit | line 2 |
| `boss.rule.heisenbug` | `:w` says only pass or fail: no sample number, no diff | line 3 |
| `boss.rule.race-condition` | the clock runs 1.25× fast; the door pays ×4 | line 4 |
| `boss.rule.undefined-behavior` | samples run without ASan or UBSan | line 5 |
| `boss.rule.halting-problem` | the clock is hidden; only the 5:00 and 1:00 warnings | line 6 |

```text
"Exactly one test wrong burns double." "Contrariwise."
"Half the memory limit! Outgrow it and I send in Bill!"
"Pass or fail, that's all :w says here," says the Sheep.
"Clock a quarter fast, prize times four!" says the Dodo.
"No sanitizers in my kitchen," says the Duchess. "Pepper!"
It hides the clock. The Rabbit cries twice; then it ends.
```

---

## Merge window

### merge.line
where: over the merge window's table when there is room (`./internal/screens/boot/merge.go:164`) · when: every · pick: pool · uses: A26, A27, A28, A22, A23, A24, A20
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
where: the merge window's daemon section when the boot holds `MaxDaemons` and the window offers none, under the daemons held, where the offers would be (`daemonsFull`, `./internal/screens/boot/merge.go:202`, `:310`). There is no swap (`./internal/game/engine.go:653`), so the line promises none · when: every · pick: fixed · uses: A19; the punchline answers `story.first-merge`'s "There's plenty of room."
```text
"No room! No room!" Every daemon seat is taken. For once, it's true.
```

### merge.nothing
where: in place of `merge.line` when nothing is uncommitted (`merge.go:166`) · when: every · pick: fixed · uses: *parody* of A21 ("I don't see any wine" / "There isn't any")
```text
"I don't see any bytes." "There aren't any," says the March Hare.
```

### merge.commit
where: the dmesg strip, after `c` commits (`./internal/screens/boot/dmesg.go:371`) · when: every · pick: fixed · uses: A30
```text
Committed. The Dormouse murmurs "Treacle," and goes back to sleep.
```

### merge.uncommitted
where: the dmesg strip, leaving the window with bytes uncommitted (`dmesg.go:376`) · when: every · pick: fixed
```text
The Hatter waves a cup at your pockets: {bytes} still uncommitted.
```

### merge.leave
where: the dmesg strip, leaving the window with nothing uncommitted (`dmesg.go:378`) · when: every · pick: fixed · uses: A32
```text
"At any rate I'll never go there again!" Next biome. Same table.
```

### fountain.drink / fountain.full / fountain.dry
where: new: the merge window's fountain row (map.md (g)), only in the two windows between acts. The fountain is the March Hare's tea, a flask-style refill: free, a few cycles, never past the starting pool ("the line"). `fountain.drink` when it gives cycles; `fountain.full` when the pool is already at the line or over, so it gives none; `fountain.dry` from P5, where it is gone (the Walrus drank it, `reveal.prestige.5`) · when: every · pick: fixed · uses: A57; A21 (`fountain.dry`)
```text
"Take some more tea," says the March Hare. Never past the line.
No more tea: your cup's already at the line. The Hare looks hurt.
More tea? "There isn't any," says the March Hare. The Walrus had it.
```

### item.fsck
where: new: the dmesg strip, after `f` buys fsck in a merge window, under `ItemTextFor`'s line ("+10 cycles now.") · when: every · pick: fixed · uses: G28 (first line; the rhyme says they couldn't, so here it holds)
```text
"All the King's horses and all the King's men" run fsck. It holds.
```

---

## Crash

Epitaphs sit under the sourced quote on the kernel panic screen
(`epitaph`, `./internal/screens/boot/crash.go:426`, drawn at `:330`), in
faint text. They add to the quote and do not replace it. `cause` is
`game.Crash.Cause`. Under cycles a crash is the burn that empties the pool:
a timeout, a give-up or failed submits. Out of cycles keeps the stored
outcome and the hook key `out-of-lives` (meta.md (i): no migration). A
cursed-door crash (`Crash.Cursed`) uses `crash.epitaph.cursed` instead.
Out of cycles with a known last verdict prefers the verdict pool and falls
back to the general one.

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
pick: fixed per verdict (`TLE`, `MLE`, `RE`, `WA`; OLE folds into MLE, `crash.go:433`) · uses: A25, A50, ch. VIII executioner (allusion), A33 (*parody*)
```text
"He's murdering the time! Off with his head!" As charged.
Rule Forty-two. All persons more than a mile high to leave the court.
The executioner needs a body to cut the head from. You had a null.
"Off with its head!" Wrong answer, and no cycles left to burn.
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
where: the flash after an upsolve banks a core dump (`recoveredNote`, `./internal/app/debrief.go:247`) · when: every · pick: fixed · uses: A18 (allusion)
```text
The grin comes back, and a cat behind it: +{bytes} banked.
```

### coredump.lost
where: the crash screen, when a crash overwrites an unrecovered dump (`./internal/screens/boot/crash.go:335`) · when: every · pick: block · uses: G28
```text
All the King's horses and all the King's men
Couldn't put Humpty Dumpty in his place again.
{bytes} of core dump, gone with him.
```

---

## Cleared

Lines sit under the cleared screen's header (`clearedLine`,
`./internal/screens/boot/cleared.go:202`, drawn at `:118`), above the
stats. The pick is a pool keyed by the prestige just cleared. A prestige
unlock also earns `reveal.prestige.<p+1>`, the first time, as the boot
end's beat.

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

### cleared.5
uses: G37 (allusion) · **draft** for 5–8: they wait for `game.MaxPrestige` 8
```text
Ring -2 cleared. The Walrus weeps into his hanky and counts nothing.
SMM survived. The firmware will swear you were never here.
```

### cleared.6
uses: the Walrus poem's midnight sun and eaten oysters (allusion)
```text
The beach, cleared at midnight in full sun. Nobody upstairs noticed.
Ring -2, the whole beach. The oysters would cheer, if any were left.
```

### cleared.7
uses: ch. XII, the sister on the bank (allusion)
```text
Ring -3 cleared. The sister turns a page and doesn't look up.
The Management Engine logs your clear. Nobody can read its logs.
```

### cleared.8
uses: ch. I, the bank and the Rabbit (allusion)
```text
The bottom ring, cleared. Below it is only the bank you fell from.
Ring -3, twice. The sister's book has a rabbit in it. Of course.
```

### cleared.beyond
where: past the ring table, prestige 9 and up, if `MaxPrestige` ever rises that far (`cleared.go:208`); `Ring` keeps such a level at ring -3
```text
Ring {ring}. The Management Engine waves. Nobody knows what it does.
Eval hands the ball to Apply. Apply hands it back. Again?
```

---

## Rubber duck (the Cheshire Cat)

Where: the dmesg strip, under the duck's own line: after a quack
(`./internal/screens/boot/dmesg.go:313`), a refused one (`:473`, `:475`)
and the merge window's refill (`:367`). The mechanic text there (tags,
technique) stays. Pick: pool.

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
the merge window, when ducks refill · uses: A17
```text
"Did you say pig, or fig?" The Cat is back on its branch. Ducks: {n}
A grin in the tree, then a Cat around it. Ducks refilled: {n}
```

---

## Daemons and upgrades

Flavor goes on the dmesg strip under the pick's mechanic line, the text
from `DaemonTextFor` and `UpgradeTextFor` (`./internal/flavor/names.go:69`,
`:52`; drawn at `./internal/screens/boot/dmesg.go:341`, `:327`). That text
stays the truth (bible tone rule 5), so a line here names no number a
preset can change. Pick: fixed per key.

### daemon.<key>
uses: G30 (allusion); nice parodies the Knight's upside-down box and cron his mouse-trap "in case they do come" (ch. VIII). kexec names no count: it swaps once an act on the map and once a biome in the linear presets (map.md (d)), and boots a new kernel without a reboot, as the real one does

| Hook | Line |
|---|---|
| `daemon.strace` | see block, line 1 |
| `daemon.nice` | line 2 |
| `daemon.fork` | line 3 |
| `daemon.dev-random` | line 4 |
| `daemon.kexec` | line 5 |
| `daemon.cron` | line 6 |
| `daemon.chrt` | line 7 |

```text
The Knight's invention: a window onto the test that tripped you.
Carried upside-down so the rain can't get in. The bytes fall out.
He rides two horses at once and falls off both. More doors, though.
A box of mystery doors. He invented it. He's never opened it.
A new kernel without a reboot: swap the door and stay on the horse.
A mousetrap on the saddle, in case old doors come back. They pay more.
The horse gets real-time priority. The ground comes up faster, too.
```

### upgrade.<key>
uses: G8. Overclock now burns cycles a pick ("it burns", map.md (d)); ECC turns a failed submit's cycles into clock for the rest of the boot, so the Queen never hears and the clock pays

| Hook | Line |
|---|---|
| `upgrade.overclock` | line 1 |
| `upgrade.ecc` | line 2 |
| `upgrade.duck` | line 3 |
| `upgrade.shuf` | line 4 |

```text
"You must run at least twice as fast as that!" Or {pct}, and it burns.
Flipped bits corrected, and the Queen never hears. The clock does.
Another grin in the trees. Somebody's Cat. Yours now.
New doors for one room, pulled from a hat. The Hatter wants it back.
```

### o3.bonus
where: the dmesg strip, on the AC under half the timer that earns the free upgrade (`./internal/screens/boot/dmesg.go:264`) · when: every · pick: fixed · uses: G8 (allusion)
```text
Twice as fast as that! The Red Queen nods, once. Free upgrade.
```

---

## Regressions (the White Queen)

### regression.door
where: the dmesg strip as a regression door opens, its `↺ regression` part with this line, ahead of a blind or mystery door's (`doorMods`, `./internal/screens/boot/dmesg.go:419`). The card already says how long ago it crashed you, so the lines don't · when: every · pick: pool · uses: G18, G17 (allusion), White Queen's jam rule (ch. V, paraphrase). No line asks for the solve on a day (retention GR2)
```text
"You again?" says the door. It remembers you. You remember it, a bit.
"It's a poor sort of memory that only works backwards." Forwards, now.
Jam yesterday, jam to-morrow, never jam to-day. Bugs aren't jam.
Same door, blank file. Since last time, one of you learned something.
```

### regression.stage.1
where: GRUB's regressions submenu, the note under a due regression at stage 1 (`regressionNote`, `./internal/screens/meta/grub.go:729`); `{days}` is that stage's delay, `Regression.Days[0]` (3 in the standard rules) · pick: fixed · uses: G19
```text
"I haven't pricked it yet, but I soon shall—oh, oh, oh!" Day {days}.
```

### regression.stage.2
where: as above, stage 2; `{days}` is `Regression.Days[1]`, counted from the first fix (17 in the standard rules) · pick: fixed
```text
Day {days} after your fix: the White Queen screams before you open it.
```

### regression.cleared
where: the dmesg strip, on an AC on a regression door (`./internal/screens/boot/dmesg.go:268`), and the flash after a GRUB upsolve fixes a due regression (`upsolveNote`, `./internal/app/debrief.go:233`) · when: every · pick: fixed
```text
Solved, blank file and all. The White Queen forgets it, backwards.
```

---

## RTFM (the Mock Turtle's school)

The browser's header stays the guide's license line (`manTitle`,
`./internal/screens/rtfm/browser.go:454`). RTFM keeps only the lines its
events show: a module page, a trial starting, passing, failing.

### rtfm.module.intro
where: module page, a faint line after the NAME section's description, `{module}` the title (`introLine`, `./internal/screens/rtfm/module.go:162`) · when: every · pick: pool, seeded by module · uses: A40, A42 (allusion), A44 (*parody*: the Gryphon's line with the module name added)
```text
"{module}," sighs the Mock Turtle. "We did it at sea."
{module}: a branch of Arithmetic, the Gryphon insists.
{module}. "I only took the regular course."
The Gryphon: "{module}. They lessen from day to day."
```

### rtfm.trial.start
where: new: a flash as a trial starts (`startTrial`, `./internal/app/learn.go:88`). A trial's timer runs into overtime (`rules.Trial`, `OnExpire = ExpireOvertime`), which is what "soft" means · pick: fixed · uses: A43
```text
"Ten hours the first day," says the Mock Turtle. Your timer is soft.
```

### rtfm.trial.pass
where: the cleared screen of a trial that granted a capability (`TrialState.Granted`), in place of `cleared.<p>`; `{biome}` is the trial's integration topic, the one the capability unlocks (`trialLine`, `./internal/screens/boot/cleared.go:221`) · pick: fixed
```text
The Gryphon claps: {cap}. {biome} joins routes.
```

### rtfm.trial.fail
where: the crash screen of a trial, in place of the epitaph, whatever ended it (`epitaph`, `./internal/screens/boot/crash.go:433`) · pick: fixed · uses: A46
```text
"That's enough about lessons," says the Gryphon. Try a boot instead.
```

### Cut: help.vocab, rtfm.syllabus, white-knight.song, rtfm.underflow
The RTFM page's story tail (`WORDS`, `CURRICULUM`, `EXAMPLES`, `BUGS`;
`storyTail`, `./internal/screens/rtfm/browser.go:670`) is cut, with GRUB's
`cat /proc/story` reread page: the story shows only at game events, never
on a lore page (user, 10-15: "keep the story non visible outside of actual
game events keep it esoteric … remove the lore tab"). The four hooks are
gone from the table. Their quotes A45 ("And how did you manage on the
twelfth?") and G31 ("Haddocks' Eyes") left the register with them; G25,
G26 (`lib.saved`) and G30 (`daemon.intro`) stay.

---

## The dmesg strip and the app's flashes

These were written as optional hooks; the D lane wired them.

### verdict.<V>
where: the dmesg strip: a failed submit's verdict (`./internal/screens/boot/dmesg.go:248`) and every AC (`verdict.AC`, `:258`; `o3.bonus` takes its place on an -O3 clear); the Queen as judge · pick: fixed per verdict · uses: A33 (*parody*), A25 (fragment), A50 (fragment), A51; OLE alludes to the jury's slates (ch. XI)

| Hook | Line |
|---|---|
| `verdict.AC` | line 1 |
| `verdict.WA` | line 2 |
| `verdict.TLE` | line 3 |
| `verdict.MLE` | line 4 |
| `verdict.RE` | line 5 |
| `verdict.CE` | line 6 |
| `verdict.OLE` | line 7 |

```text
The Queen looks for a head to remove. Finds none. Sulks.
"Off with its head!"
"He's murdering the time!"
"All persons more than a mile high to leave the court."
The executioner can't cut off a head without a body to cut it from.
"If there's no meaning in it, that saves a world of trouble."
The jury runs out of slate. You're still printing.
```

### timer.warn
where: the dmesg strip at 5:00 and 1:00 left (`./internal/screens/boot/dmesg.go:392`); not on the vim border yet · pick: line 1 at 5:00, line 2 at 1:00 · uses: A1, A2
```text
"Oh dear! Oh dear! I shall be late!"
"Oh my ears and whiskers, how late it's getting!"
```

### quit.warn / resume.note
where: `quit.warn` is the story half of the in-door ctrl+c refusal (D47: no quitting inside a door; `quitWarn`, `./internal/app/debrief.go:253`), beside its mechanic half, which the screens build from the rules' costs: "no quitting inside a door: solve · R retreat −5 · :q! give up −10 · closing the terminal does not stop the clock". `resume.note` is the flash on reopening a door after an involuntary exit (kill -9, a lost terminal), whose deadline stayed absolute (`resumeNote`, `:262`). Resuming a boot from the map is `map.resume` · pick: fixed
```text
The Rabbit won't stop his watch for you. {time} left on this door.
Welcome back. The Rabbit kept time: {time} left.
```

### door.blind / door.mystery / door.cursed
where: the dmesg strip as the door opens, the line of its most dangerous modifier (`doorMods`, `./internal/screens/boot/dmesg.go:413`) · pick: fixed · uses: G10 (allusion), G24
```text
Tags hidden, as in the nameless wood. Bytes extra for the trouble.
Sometimes like a doll and sometimes like a work-box. Title hidden.
The Queen is judging this one herself. No Cat. One WA.
```

### lib.saved
where: the dmesg strip, when a `// lib:` block is harvested from an AC (`./internal/screens/boot/dmesg.go:230`) · pick: 2-line pool · uses: G25, G26
```text
"When I use a word, it means just what I choose it to mean—
neither more nor less." lib {name}, saved.

"When I make a word do a lot of work like that, I always pay it
extra." lib {name}, linked.
```

---

## Inside a door

D47's rule: no quitting inside a door. The ways out are solve, `R`
retreat and `:q!` give up. A timeout or a give-up loses the room and the
boot goes on, unless the burn empties the pool, which is a crash and the
epitaphs above. Each line sits on the dmesg strip under the engine's own
line, which carries the burn ("timer expired, burned 10 cycles, 14 left ·
room lost"). On a boss door, a burn that deals a new boss door shows
`boss.retry` instead.

### door.timeout
where: new: the dmesg strip, when a door's timer runs out and the boot goes on · when: every · pick: pool · uses: none
```text
The Rabbit's watch runs out. The room is lost; the boot is not.
Time. The Queen takes the door and her cycles. The hall goes on.
```

### door.gave-up
where: new: the dmesg strip, after a `:q!` that leaves cycles (the door only; the boot goes on) · when: every · pick: pool · uses: none. Line 2 holds only while a given-up door returns as a regression (meta.md (b): every failed door does)
```text
You walk out. The Queen keeps the door, and her cut of your cycles.
:q!, and the door shuts behind you. The White Queen pencils it in.
```

### door.retreat / door.retreat.locked
where: new: the dmesg strip on `R`: `door.retreat` as the room's doors are drawn again; `door.retreat.locked` when `R` comes after the lock (`retreat_lock`, 2:30 in map.md), `{time}` that lock. Show `.locked` only when no swap is left at all, kexec's included · when: every · pick: fixed · uses: G9 (`door.retreat`); `.locked` alludes to it
```text
"I should advise you to walk the other way," says the Rose. New doors.
{time} in, the door has shut behind you. No walking the other way now.
```

### verdict.WA.ecc
where: new: the dmesg strip, in place of `verdict.WA` when ECC turns a failed hidden-test submit into clock (`ecc_penalty`, 5:00 off the door's clock, map.md (d)) · when: every · pick: fixed · uses: A33 (allusion: the Queen gets no head)
```text
ECC corrects the bit. No head for the Queen; the Rabbit's watch pays.
```

---

## The map

The map is the hub (D47, map.md (g)). A boot is suspended between doors
only, resumed from the map, and a new boot over a suspended one ends it as
a crash. The route's exits show their gates; a locked one names what opens
it.

### map.suspend / map.resume / map.abandon
where: new: `map.suspend` as the boot is saved on `q` or ctrl+c between doors; `map.resume` on `enter` from the map, `{boot}` the suspended boot; `map.abandon` in the map's `n` confirm, `{boot}` the boot it ends, beside the mechanic text ("ends boot #42 as a crash: 46 B lost, the core dump on Bovine Shuffle destroyed") · when: every · pick: fixed · uses: A31 (`map.abandon`: the Hatter moves everyone to clean cups, and the dirty ones stay behind). No line counts the time away (GR2, GR8): the clock runs only inside doors, so nothing ran
```text
Suspended. The Rabbit pockets his watch: it only ticks behind doors.
fg: boot #{boot} picks up where it stopped. Nothing ran while it slept.
"Let's all move one place on." Boot #{boot} stays behind, as a crash.
```

### route.locked / route.locked.prestige
where: new: the route screen, when the player picks a locked exit, under the card's own gate text (map.md (g): "locked: CAP_TWOPTR · RTFM › Silver › Two Pointers (t)"). `route.locked` for a capability gate, `{module}` the RTFM module whose trial grants it; it says the trial opens the lock, not that only the trial does (`apt git` also opens Bisect Bridge). `route.locked.prestige` for a prestige gate, `{ring}` the ring the exit opens at (Undefined Behavior from ring 2, The Framebuffer from ring 1) · when: every · pick: fixed · uses: none; the glass is `milestone.gate.*`'s
```text
The Gryphon guards this lock. {module} opens it.
The glass won't let you by yet. This exit opens from ring {ring} down.
```
