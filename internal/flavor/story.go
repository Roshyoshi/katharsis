package flavor

import (
	"slices"
	"strings"
)

// Story is one line of a pool or fixed hook from docs/story/text.md, with
// its {name} placeholders left for the caller to Fill. A pool rotates with
// the seed the way QuoteFor does: consecutive seeds never repeat an entry,
// and a seed keeps its pick. An entry of a 2-line pool holds a "\n". A
// block hook, or a hook the table lacks, gives "".
func Story(hook string, seed uint64) string {
	h, ok := story[hook]
	if !ok || h.kind == block {
		return ""
	}
	n := uint64(len(h.entries))
	k := hash64(hook)
	return h.entries[((seed%n)*stride(k, n)+k%n)%n]
}

// StoryBlock is every line of a block hook, in order. A pool or fixed
// hook, or a hook the table lacks, gives nil.
func StoryBlock(hook string) []string {
	h, ok := story[hook]
	if !ok || h.kind != block {
		return nil
	}
	return slices.Clone(h.entries)
}

// Fill replaces each {name} in s with its value; pairs alternate name and
// value: Fill(s, "bytes", "15.2 KiB", "title", t). Placeholders carry real
// values only (the bible's tone rule 5), so a name left out stays visible
// as {name} rather than vanishing into a sentence that lies.
func Fill(s string, pairs ...string) string {
	if len(pairs)%2 != 0 {
		panic("flavor.Fill: odd name/value pairs")
	}
	if len(pairs) == 0 || !strings.Contains(s, "{") {
		return s
	}
	kv := make([]string, len(pairs))
	for i := 0; i < len(pairs); i += 2 {
		kv[i], kv[i+1] = "{"+pairs[i]+"}", pairs[i+1]
	}
	return strings.NewReplacer(kv...).Replace(s)
}

// storyKind is how a hook's entries are used.
type storyKind uint8

const (
	pool  storyKind = iota // one entry per seed (Story)
	block                  // every line, in order (StoryBlock)
	fixed                  // the one line (Story)
)

type storyHook struct {
	kind    storyKind
	entries []string
}

// The constructors take the lines as text.md prints them, one per line,
// so the table reads like the doc it copies.
func poolOf(s string) storyHook  { return storyHook{pool, textLines(s)} }
func blockOf(s string) storyHook { return storyHook{block, textLines(s)} }
func fixedOf(s string) storyHook { return storyHook{fixed, []string{s}} }

// pool2Of is a 2-line pool: entries separated by a blank line.
func pool2Of(s string) storyHook {
	return storyHook{pool, strings.Split(strings.Trim(s, "\n"), "\n\n")}
}

func textLines(s string) []string { return strings.Split(strings.Trim(s, "\n"), "\n") }

// story is docs/story/text.md as data, keyed by hook. A family in the doc
// ("biome.intro.<topic>", "daemon.<key>", "line N for prestige N") is one
// hook per member here. TestStoryMatchesText holds the two to the same
// lines.
var story = map[string]storyHook{
	// ---- Boot and GRUB ----
	"title.tagline": fixedOf("Every boot is a fall. Some falls end. That's the whole question."),
	"boot.splash.dmesg": blockOf(`
[    0.000] katharsis boot #{boot} (k&r@bell-labs) (cc, 1978)
[    0.000] Command line: ro quiet rabbit=late cat=grinning
[    0.001] Calibrating delay loop... the Rabbit says: late
[    0.004] Memory: plenty of room (the tea party disagrees)
[    0.013] eval: apply: eval: apply: ok (metacircular)
[    0.020] cpu: entering ring {ring}: {place}
[    0.031] random: crng init done; the Cat is grinning
[    0.042] Run /sbin/init as init process: you
`),
	"boot.motd": poolOf(`
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
`),
	"grub.subtitle.prestige.0":      fixedOf("ring 3 · Userland · down the rabbit-hole"),
	"grub.subtitle.prestige.1":      fixedOf("ring 2 · through the looking-glass · things go the other way"),
	"grub.subtitle.prestige.2":      fixedOf("ring 1 · the wood where things have no names"),
	"grub.subtitle.prestige.3":      fixedOf("ring 0 · the Court of Hearts · sentence first, verdict afterwards"),
	"grub.subtitle.prestige.4":      fixedOf("ring -1 · the Red King's dream · beware the Jabberwock"),
	"grub.subtitle.prestige.5":      fixedOf("ring -2 · the Walrus's beach · the sun shines at midnight"),
	"grub.subtitle.prestige.6":      fixedOf("ring -2 · the Walrus's beach · nobody counts the oysters"),
	"grub.subtitle.prestige.7":      fixedOf("ring -3 · the riverbank · something here never sleeps"),
	"grub.subtitle.prestige.8":      fixedOf("ring -3 · the riverbank · where the fall began"),
	"grub.subtitle.prestige.beyond": fixedOf("ring {ring} · nobody who went there wrote it down"),
	"grub.subtitle.nightly":         fixedOf("an un-birthday present: one seed, the same for everyone"),
	"grub.subtitle.rtfm":            fixedOf("the Mock Turtle's school · Reeling and Writhing, to begin with"),
	"grub.subtitle.custom":          fixedOf("a preset means just what you choose it to mean"),
	"grub.subtitle.coredump":        fixedOf("a grin without a cat · {bytes} still on {title}"),
	"grub.subtitle.regressions":     fixedOf("the White Queen remembers the week after next"),
	"grub.subtitle.gitlog":          fixedOf(`"There ought to be a book written about me, that there ought!"`),
	"grub.subtitle.top":             fixedOf(`"Who are you?" said the Caterpillar.`),
	"grub.subtitle.poweroff":        fixedOf(`"Wake up, Alice dear!" The Rabbit will wait. Late, but he'll wait.`),

	// ---- Story beats ----
	"story.prologue": blockOf(`
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
`),
	"door.first.inscription": fixedOf("Carved over the first door, in lowercase: hello, world"),
	"story.first-merge": blockOf(`
A long table under a tree. Three seats taken. A sign: MERGE WINDOW.
"No room! No room!" cry the Hatter and the March Hare.
There's plenty of room. Sit down. Commit what you carry.
"It's always six o'clock now," sighs the Hatter. Time to commit.
`),
	"daemon.intro": blockOf(`
A knight falls off his horse, climbs back on, and bows.
"It's my own invention," he says, of everything. Pick one.
Each one works. Each one has a catch. Mind the upside-down box.
`),
	"story.first-boss": blockOf(`
A trumpet. The White Rabbit unrolls a scroll and puts on spectacles.
"Where shall I begin, please your Majesty?" he asks.
"Begin at the beginning," the King says gravely, "and go on till you
come to the end: then stop."
Easier said than proven. Here comes the bug.
`),
	"story.first-crash": blockOf(`
The Cat appears, grin first. "You crashed. Everyone does, here."
"You left your bytes on that problem: a grin without a cat."
"Upsolve it and the grin comes back. Crash first, and it's gone."
"You're sure to get somewhere," says the Cat, "if you boot enough."
`),
	"story.first-rtfm": blockOf(`
A Gryphon asleep in the sun. A Mock Turtle, sighing on a rock.
"We had the best of educations," says the Mock Turtle.
"Reeling and Writhing, of course, to begin with," it goes on, "and
then the different branches of Arithmetic—Ambition, Distraction,
Uglification, and Derision."
`),
	"story.first-regression": blockOf(`
A Queen in a shawl sticks plaster on a finger she hasn't pricked.
"...one's memory works both ways," says the White Queen, kindly.
She remembers best the things that happen the week after next.
This door beat you once. It's back, with a blank file. Then again.
`),
	// The map's act boundaries and gates (docs/design/map.md (g)): scenes
	// the screens show once, at their moment.
	"story.first-act2": blockOf(`
The bug steps aside. Behind it, a door fifteen inches high.
Through it, "the loveliest garden you ever saw." Act II.
The gardeners are painting the white roses red.
`),
	"story.first-act3": blockOf(`
Act III: the far end of the garden, past the croquet-ground.
Beyond the hedges something burbles as it comes. It is last.
`),
	"story.first-skip": blockOf(`
"So you'll go very quickly through the Third Square—by railway,
I should think—" says the Red Queen. No room. No bytes. Picks.
`),
	"story.first-locked": blockOf(`
A padlocked exit, and asleep on the padlock, a Gryphon.
"That's enough about lessons," it yawns, and points at RTFM anyway.
`),
	"reveal.prestige.1": blockOf(`
The mantel mirror goes soft, like gauze. You step through.
Same rooms. Only the things go the other way.
The Red Queen, already running: "Now, here, you see, it takes all
the running you can do, to keep in the same place."
The doors will learn you. That isn't a bug. That's ring 2.
`),
	"reveal.prestige.2": blockOf(`
Ring 1 is the nameless wood. Symbols: stripped.
A Fawn walks beside you, nuzzling, until the symbols load.
"I'm a Fawn!" it cries. It reads /proc/1/comm: you. It bolts.
On a wall at the edge of the wood sits an egg, defining words.
`),
	"reveal.prestige.3": blockOf(`
Ring 0, eighth square. You're root, which here means Queen.
The Court of Hearts is nothing but a pack of cards. Test data.
Behind the throne two robed Wizards toss a glowing ball: eval, apply.
"The evaluator," says one, "... is just another program."
Somewhere below ring 0, somebody is snoring.
`),
	"reveal.prestige.4": blockOf(`
Ring -1, below ring 0. The Red King sleeps here, in a heap.
"He's dreaming now," says Tweedledee. "... Why, about you!"
The Wizards keep his dream running: eval, apply, eval, apply.
Long ago they set Exercise 4.15: write halts?, then show you can't.
It got loose. It has jaws that bite and claws that catch.
`),
	// Rings -2 and -3 take two prestige levels each, so only their first
	// level has a reveal (beats.go, revealLevels).
	"reveal.prestige.5": blockOf(`
Ring -2: a beach under the dream, sunny at midnight.
The fountain is dry. The Walrus drank it behind his handkerchief,
"so that the Carpenter couldn't count how many he took".
`),
	"reveal.prestige.7": blockOf(`
Ring -3: a riverbank on a hot afternoon. It looks familiar.
A sister reads on the bank, awake with the power off, eyes shut.
One blink and "all would change to dull reality". She doesn't blink.
`),
	// The final is the Halting Problem at every prestige (map.md decision
	// 1), so this fires at ring 3: the poem as Alice first meets it in
	// Looking-Glass chapter I, mirrored, before anyone explains it.
	"reveal.final-boss": blockOf(`
The last door. On it, a poem in looking-glass print: .YKCOWREBBAJ
sevot yhtils eht dna, gillirb sawT'
"It seems very pretty, but it's rather hard to understand!"
It is the Halting Problem. You can't win in general. Win this once.
`),
	"reveal.final-clear": blockOf(`
"O frabjous day! Callooh! Callay!" The Jabberwock is down.
You didn't solve the Halting Problem. You halted. Turing allows that.
Did you dream it, or did the Red King? Which do you think it was?
On the last door, in the same old hand as the first: hello, world
Life, what is it but a dream? Reboot.
`),

	// ---- Milestone beats: the dry spells between story beats (beats.go) ----
	"milestone.doors.10": blockOf(`
Under the doormat in the hall: a note, folded very small.
On the outside, in pencil: halts? Who asks a doormat that?
`),
	"milestone.upsolve.5": blockOf(`
The Cat keeps a ledger now: grins lost, grins taken back.
It won't show you the totals. What is it saving them for?
`),
	"milestone.doors.25": blockOf(`
The note under the mat is signed: A. M. T., 1936.
The Rabbit reads it over your shoulder and drops his watch. Why?
`),
	"milestone.gate.half": blockOf(`
The mirror over the mantel clouds, like breath on glass.
Half the rated doors it wants are behind you. What's it waiting for?
`),
	"milestone.doors.50": blockOf(`
Unfolded once, the note shows a tape, a head and a table.
"That's the judge's grandfather," says the Cat. What did it judge?
`),
	"milestone.gate.open": blockOf(`
The glass is soft as gauze. One clear takes you through.
The Red Queen runs past too fast to ask: same room, other side?
`),
	"milestone.lib.1": blockOf(`
Your first lib. Humpty Dumpty insists on naming it.
He'd call it Humpty. You didn't. Does a name change what it means?
`),
	"milestone.capability": pool2Of(`
{cap}, stamped by the Gryphon. Another road opens.
The Mock Turtle sobs that lessons lessen. How short can one get?

{cap}, earned. The Mock Turtle weeps buckets.
Proud, or just salty? The Gryphon can't tell. Can you?

The Gryphon pins {cap} on a crowded wall.
What's under the oldest one? The Mock Turtle won't look. Will you?
`),
	"milestone.mastered.1": blockOf(`
One topic knows you now: its lesser doors drop their tags.
The Cat will still read them out for a duck. Will you need it to?
`),
	"milestone.retired.1": blockOf(`
The White Queen strikes a door off her list, in ink.
It has no future now, so she's already forgotten it. Will you?
`),
	"milestone.doors.75": blockOf(`
The note's second fold asks one thing of every machine:
does it ever stop? The Rabbit stops winding his watch. Does it?
`),
	"milestone.doors.100": blockOf(`
The note answers itself: nobody can say, not in general.
So the Rabbit carries a watch and decides for one. Is that cheating?
`),
	"milestone.upsolve.25": blockOf(`
The Cat's ledger balances at last, so the Cat eats it.
It purrs that it kept a copy of each one. A copy of whom?
`),
	"milestone.doors.150": blockOf(`
Every door so far has stopped. You checked each one.
The Queen calls that a proof. The Cat calls it a sample. Who's right?
`),
	"milestone.doors.200": blockOf(`
Something under the floor snores in 4/4 time.
The judge's clock keeps the beat. Who sleeps through every verdict?
`),
	"milestone.doors.250": blockOf(`
The note's last fold. Inside: a sketch of PID 1, falling.
The drawing stops before you land. Out of tape, or on purpose?
`),
	"milestone.doors.300": blockOf(`
The Wizards have heard about your note.
"Older than us," says Eval. "We borrowed it once," says Apply. Why?
`),
	"milestone.doors.400": blockOf(`
"We set it as an exercise," says Eval. "Four-fifteen."
Apply owns up to the pencil. "Then it got out." What got out?
`),
	"milestone.doors.500": blockOf(`
Unfolded flat, the note is exactly the size of a door.
On its back, in an old terse hand: hello, world. Which came first?
`),

	// ---- Biomes: one fixed line per catalog topic ----
	"biome.intro.complete-search": fixedOf("Doors all round the hall, all locked. Alice tried every one. Go on."),
	"biome.intro.simulation":      fixedOf("The King's spec: begin at the beginning, go on to the end, then stop."),
	"biome.intro.sorting-greedy":  fixedOf(`"It's very easy to take more than nothing." Greedy, if it's provable.`),
	"biome.intro.prefix-sums":     fixedOf("Ten hours the first day, nine the next. Sum once; answer in O(1)."),
	"biome.intro.binary-search":   fixedOf("The Caterpillar's mushroom: one side taller, one side shorter. Bisect."),
	"biome.intro.two-pointers":    fixedOf(`"Let's all move one place on." Two cursors round the tea table.`),
	"biome.intro.graphs":          fixedOf(`"That depends a good deal on where you want to get to," said the Cat.`),
	"biome.intro.trees":           fixedOf("One of the trees has a door in it. cd in, and remember the way out."),
	"biome.intro.shortest-paths":  fixedOf(`"I should advise you to walk the other way." The Rose knew a shortcut.`),
	"biome.intro.dsu":             fixedOf(`"It's my crown all the while!" says the King. find() agrees.`),
	"biome.intro.dp":              fixedOf(`"I know who I was when I got up this morning." dp[0], at least.`),
	"biome.intro.math":            fixedOf(`"Ambition, Distraction, Uglification, and Derision." Mod 1e9+7.`),
	"biome.intro.strings":         fixedOf("'Twas brillig, and the slithy toves. Tokenize that."),
	"biome.intro.data-structures": fixedOf(`"The more there is of mine, the less there is of yours." -- malloc`),
	"biome.intro.geometry":        fixedOf(`"Only the things go the other way." Mirrors flip every cross product.`),
	"biome.intro.mixed":           fixedOf(`"We're all mad here." Ring 3: every topic, no privileges, no excuses.`),
	// Kernel Space is mixed too, so it is keyed by its map node, not its
	// topic: biome.intro.mixed is Userland's, and says ring 3.
	"biome.intro.kernel": fixedOf("Kernel croquet: flamingo mallets, hedgehog balls, any topic, no MMU."),

	// ---- Bosses: entrance on the door card, defeat after the clear ----
	"boss.enter.off-by-one": poolOf(`
Tweedledum and Tweedledee, back to back. Count them. Count again.
"Contrariwise," says Tweedledee. Your loop stops at n. Or at n-1.
"If it was so, it might be; and if it were so, it would be."
`),
	"boss.defeat.off-by-one": poolOf(`
"Ditto," says Tweedledum. "Ditto, ditto," says Tweedledee. Accepted.
Two twins, one gap between. You counted the fenceposts this time.
"But as it isn't, it ain't. That's logic." So is your loop bound.
`),
	"boss.enter.heisenbug": poolOf(`
The Sheep's shop: look hard at any shelf and it's quite empty.
"Things flow about so here!" It fails on the judge. Never on yours.
The Sheep knits with fourteen pairs of needles. None is printf.
`),
	"boss.defeat.heisenbug": poolOf(`
You stopped looking hard. The shelf stayed full. Accepted.
Uninitialised all along. The Sheep sniffs and goes back to knitting.
`),
	"boss.enter.oom-killer": poolOf(`
You drank the bottle. Now you fill the house, one foot up the chimney.
Something scrabbles down the flue. The Rabbit has sent in a Bill.
The OOM Killer scores every process by size. You're the largest here.
`),
	"boss.defeat.oom-killer": poolOf(`
One sharp kick, and a general chorus of "There goes Bill!"
You shrank to fit the house. RSS: modest. The Rabbit: still late.
`),
	"boss.enter.race-condition": poolOf(`
The Dodo marks a course, in a sort of circle. Nobody says "go".
A Caucus-race: everyone starts when they like, stops when they like.
The players all played at once without waiting for turns. Croquet.
SICP 3.4: "Time Is of the Essence". The Dodo never read it.
`),
	"boss.defeat.race-condition": poolOf(`
"Everybody has won, and all must have prizes." You hold the lock.
The race is over. You know, because you joined every thread.
`),
	"boss.enter.undefined-behavior": poolOf(`
The Duchess's kitchen: pepper in the air, a cook throwing saucepans.
"There's certainly too much pepper in that soup!" Nasal demons, too.
Signed overflow in the soup. Anything may happen. Mostly sneezing.
`),
	"boss.defeat.undefined-behavior": poolOf(`
"It makes rather a handsome pig, I think." The compiler agreed. AC.
You bounds-checked the soup. The sneezing stops. The pig stays a pig.
`),
	"boss.enter.halting-problem": pool2Of(`
"Beware the Jabberwock, my son!
The jaws that bite, the claws that catch!"

Exercise 4.15, escaped. It eats any procedure named halts?.

It won't tell you whether it ends. It won't tell you anything. Begin.
`),
	"boss.defeat.halting-problem": pool2Of(`
One, two! One, two! And through and through
The vorpal blade went snicker-snack!

He left it dead, and with its head
He went galumphing back.

You didn't decide halting. You decided this one. The proof survives.
`),
	// A boss fail that leaves cycles deals a new boss door; a one-shot
	// boss (the final, and more of them deeper down) ends the boot.
	"boss.retry": poolOf(`
The Queen shuffles the pack and deals the same bug a new door.
The bug picks itself up and backs into a fresh door. Your move.
Same bug, new door, fewer cycles to spend on it.
`),
	"boss.oneshot": poolOf(`
One shot, no second door. Either it halts, or you do.
The Queen deals this hand once. Lose it and the boot is hers.
`),
	// Each boss's one rule twist, named on its card before the door opens,
	// in its own voice. Unlike other story lines these state the rule, so
	// each changes with its twist's numbers.
	"boss.rule.off-by-one":         fixedOf(`"Exactly one test wrong burns double." "Contrariwise."`),
	"boss.rule.oom-killer":         fixedOf(`"Half the memory limit! Outgrow it and I send in Bill!"`),
	"boss.rule.heisenbug":          fixedOf(`"Pass or fail, that's all :w says here," says the Sheep.`),
	"boss.rule.race-condition":     fixedOf(`"Clock a quarter fast, prize times four!" says the Dodo.`),
	"boss.rule.undefined-behavior": fixedOf(`"No sanitizers in my kitchen," says the Duchess. "Pepper!"`),
	"boss.rule.halting-problem":    fixedOf("It hides the clock. The Rabbit cries twice; then it ends."),

	// ---- Merge window ----
	"merge.line": poolOf(`
"It's always six o'clock now." So it's always time to commit.
"It's always tea-time." No time to wash the cups between merges.
The Hatter moves round "as the things get used up." A ring buffer.
"Why is a raven like a writing-desk?" Nobody knows. Ship it anyway.
"Two days wrong!" The Hare, meekly: "It was the best butter."
The Dormouse is asleep again. It's a daemon. That's the job.
"Have some wine," says the March Hare. There's only tea. Commit tea.
Linus closes his merge window after two weeks. This one stays at six.
rc1 is at six o'clock. So is rc2. Time stopped at tea.
`),
	// At MaxDaemons a window offers no daemons, and there is no swap
	// (game/engine.go:653), so the line promises none.
	"merge.full":        fixedOf(`"No room! No room!" Every daemon seat is taken. For once, it's true.`),
	"merge.nothing":     fixedOf(`"I don't see any bytes." "There aren't any," says the March Hare.`),
	"merge.commit":      fixedOf(`Committed. The Dormouse murmurs "Treacle," and goes back to sleep.`),
	"merge.uncommitted": fixedOf("The Hatter waves a cup at your pockets: {bytes} still uncommitted."),
	"merge.leave":       fixedOf(`"At any rate I'll never go there again!" Next biome. Same table.`),
	// The fountain is the March Hare's tea, only in the windows between
	// acts, and never past the starting pool ("the line").
	"fountain.drink": fixedOf(`"Take some more tea," says the March Hare. Never past the line.`),
	"fountain.full":  fixedOf("No more tea: your cup's already at the line. The Hare looks hurt."),
	"fountain.dry":   fixedOf(`More tea? "There isn't any," says the March Hare. The Walrus had it.`),
	"item.fsck":      fixedOf(`"All the King's horses and all the King's men" run fsck. It holds.`),

	// ---- Crash ----
	"crash.epitaph.timed-out": poolOf(`
"Oh my ears and whiskers, how late it's getting!" Later than that.
The Rabbit was late. You were later. The clock is not a mutex.
The Hatter says it's always six o'clock. Your timer disagreed.
And the moral of that is: the timer started before you did.
`),
	"crash.epitaph.out-of-lives": poolOf(`
The Queen has only one way of settling all difficulties. Guess which.
The jury wrote it on their slates: wrong, wrong, wrong. Sentence.
And the moral of that is: the samples are not the test data.
"Consider your verdict," said the King. The Queen already had.
`),
	"crash.epitaph.out-of-lives.TLE": fixedOf(`"He's murdering the time! Off with his head!" As charged.`),
	"crash.epitaph.out-of-lives.MLE": fixedOf("Rule Forty-two. All persons more than a mile high to leave the court."),
	"crash.epitaph.out-of-lives.RE":  fixedOf("The executioner needs a body to cut the head from. You had a null."),
	"crash.epitaph.out-of-lives.WA":  fixedOf(`"Off with its head!" Wrong answer, and no cycles left to burn.`),
	"crash.epitaph.gave-up": poolOf(`
"You're nothing but a pack of cards!" The whole pack rose up. Fair.
"Wake up, Alice dear!" The editorial is open. Nobody's judging. Yet.
"Nonsense!" you told the Queen, and she was silent. The doors weren't.
"I almost wish I hadn't gone down that rabbit-hole—and yet—and yet—"
`),
	"crash.epitaph.cursed": poolOf(`
"Sentence first—verdict afterwards." The cursed door kept its word.
No Cat on a cursed door. Not even the grin.
One wrong answer, one head. The Queen keeps very tidy books.
`),
	"coredump.recovered": fixedOf("The grin comes back, and a cat behind it: +{bytes} banked."),
	"coredump.lost": blockOf(`
All the King's horses and all the King's men
Couldn't put Humpty Dumpty in his place again.
{bytes} of core dump, gone with him.
`),

	// ---- Cleared, keyed by the prestige just cleared ----
	"cleared.0": poolOf(`
"Everybody has won, and all must have prizes." Just you, actually.
Userland cleared. The rabbit-hole goes deeper. It always does.
exit 0. The Rabbit checks his watch: late, but inside the limit.
`),
	"cleared.1": poolOf(`
You ran as fast as you could and stayed in the same place. Progress!
Ring 2 cleared. OS/2 lived here once. It left no forwarding address.
Through the glass and back. The doors noticed. They're learning you.
`),
	"cleared.2": poolOf(`
The nameless wood, cleared. Your symbols come back. Most of them.
Humpty checks your work: "That seems to be done right—" Upside down.
You found the creature wearing your old name. It was a stack frame.
`),
	"cleared.3": poolOf(`
Ring 0 cleared. You're root. The Queen curtsies. It looks painful.
Queen Alice, eighth square. Mind the crown: it's only a pointer.
Below ring 0, something snores. The Wizards hush you. Don't wake it.
`),
	"cleared.4": poolOf(`
"O frabjous day! Callooh! Callay!" It halted. This once.
Halting: undecidable in general. This boot: decided.
Which dreamed it: you, or the Red King? git log says you.
Life, what is it but a dream? exit 0.
There's talk of a ring -2. The Wizards change the subject.
`),
	"cleared.5": poolOf(`
Ring -2 cleared. The Walrus weeps into his hanky and counts nothing.
SMM survived. The firmware will swear you were never here.
`),
	"cleared.6": poolOf(`
The beach, cleared at midnight in full sun. Nobody upstairs noticed.
Ring -2, the whole beach. The oysters would cheer, if any were left.
`),
	"cleared.7": poolOf(`
Ring -3 cleared. The sister turns a page and doesn't look up.
The Management Engine logs your clear. Nobody can read its logs.
`),
	"cleared.8": poolOf(`
The bottom ring, cleared. Below it is only the bank you fell from.
Ring -3, twice. The sister's book has a rabbit in it. Of course.
`),
	"cleared.beyond": poolOf(`
Ring {ring}. The Management Engine waves. Nobody knows what it does.
Eval hands the ball to Apply. Apply hands it back. Again?
`),

	// ---- Rubber duck (the Cheshire Cat) ----
	"duck.quack.1": poolOf(`
Grin first, then Cat: "A {tags} sort of problem."
The grin widens. On the wall, faintly: {tags}
"Cheshire Puss?" It only grins wider. Tags: {tags}
`),
	"duck.quack.2": poolOf(`
The Cat vanishes slowly, tail first. The grin stays: {technique}.
The grin hangs in the air a while, spelling {technique}.
"I'm mad. You're mad." The Cat taps the wall: {technique}.
`),
	"duck.empty": poolOf(`
You whistle for the Cat. Only the grin comes, and it won't talk.
"Cheshire Puss?" The tree is empty. Out of ducks.
`),
	"duck.cursed": poolOf(`
Cursed door: the Cat won't come. The Queen wants it beheaded.
No body, no head to cut off. The Cat's safe. You're on your own.
`),
	"duck.refill": poolOf(`
"Did you say pig, or fig?" The Cat is back on its branch. Ducks: {n}
A grin in the tree, then a Cat around it. Ducks refilled: {n}
`),

	// ---- Daemons and upgrades: a faint line under the mechanic text ----
	"daemon.strace":     fixedOf("The Knight's invention: a window onto the test that tripped you."),
	"daemon.nice":       fixedOf("Carried upside-down so the rain can't get in. The bytes fall out."),
	"daemon.fork":       fixedOf("He rides two horses at once and falls off both. More doors, though."),
	"daemon.dev-random": fixedOf("A box of mystery doors. He invented it. He's never opened it."),
	"daemon.kexec":      fixedOf("A new kernel without a reboot: swap the door and stay on the horse."),
	"daemon.cron":       fixedOf("A mousetrap on the saddle, in case old doors come back. They pay more."),
	"daemon.chrt":       fixedOf("The horse gets real-time priority. The ground comes up faster, too."),
	"upgrade.overclock": fixedOf(`"You must run at least twice as fast as that!" Or {pct}, and it burns.`),
	"upgrade.ecc":       fixedOf("Flipped bits corrected, and the Queen never hears. The clock does."),
	"upgrade.duck":      fixedOf("Another grin in the trees. Somebody's Cat. Yours now."),
	"upgrade.shuf":      fixedOf("New doors for one room, pulled from a hat. The Hatter wants it back."),
	"o3.bonus":          fixedOf("Twice as fast as that! The Red Queen nods, once. Free upgrade."),

	// ---- Regressions (the White Queen) ----
	// No line asks for a solve on a day (retention GR2): the jam rule's
	// punchline is the bug, not the date.
	"regression.door": poolOf(`
"You again?" says the door. It remembers you. You remember it, a bit.
"It's a poor sort of memory that only works backwards." Forwards, now.
Jam yesterday, jam to-morrow, never jam to-day. Bugs aren't jam.
Same door, blank file. Since last time, one of you learned something.
`),
	// {days} is the stage's own delay (Regression.Days[Stage]): after the
	// crash for stage 1, after the last fix for stage 2.
	"regression.stage.1": fixedOf(`"I haven't pricked it yet, but I soon shall—oh, oh, oh!" Day {days}.`),
	"regression.stage.2": fixedOf("Day {days} after your fix: the White Queen screams before you open it."),
	"regression.cleared": fixedOf("Solved, blank file and all. The White Queen forgets it, backwards."),

	// ---- RTFM (the Mock Turtle's school) ----
	"rtfm.module.intro": poolOf(`
"{module}," sighs the Mock Turtle. "We did it at sea."
{module}: a branch of Arithmetic, the Gryphon insists.
{module}. "I only took the regular course."
The Gryphon: "{module}. They lessen from day to day."
`),
	"rtfm.trial.start": fixedOf(`"Ten hours the first day," says the Mock Turtle. Your timer is soft.`),
	"rtfm.trial.pass":  fixedOf("The Gryphon claps: {cap}. {biome} joins routes."),
	"rtfm.trial.fail":  fixedOf(`"That's enough about lessons," says the Gryphon. Try a boot instead.`),
	// The RTFM page's story tail (help.vocab, rtfm.syllabus,
	// white-knight.song, rtfm.underflow) is cut: the story shows only at
	// game events, never on a lore page (user, 10-15).

	// ---- The dmesg strip and the app's flashes ----
	"verdict.AC":    fixedOf("The Queen looks for a head to remove. Finds none. Sulks."),
	"verdict.WA":    fixedOf(`"Off with its head!"`),
	"verdict.TLE":   fixedOf(`"He's murdering the time!"`),
	"verdict.MLE":   fixedOf(`"All persons more than a mile high to leave the court."`),
	"verdict.RE":    fixedOf("The executioner can't cut off a head without a body to cut it from."),
	"verdict.CE":    fixedOf(`"If there's no meaning in it, that saves a world of trouble."`),
	"verdict.OLE":   fixedOf("The jury runs out of slate. You're still printing."),
	"timer.warn.5m": fixedOf(`"Oh dear! Oh dear! I shall be late!"`),
	"timer.warn.1m": fixedOf(`"Oh my ears and whiskers, how late it's getting!"`),
	"quit.warn":     fixedOf("The Rabbit won't stop his watch for you. {time} left on this door."),
	"resume.note":   fixedOf("Welcome back. The Rabbit kept time: {time} left."),
	"door.blind":    fixedOf("Tags hidden, as in the nameless wood. Bytes extra for the trouble."),
	"door.mystery":  fixedOf("Sometimes like a doll and sometimes like a work-box. Title hidden."),
	"door.cursed":   fixedOf("The Queen is judging this one herself. No Cat. One WA."),

	// ---- Inside a door: what each way out burns ----
	// No quitting inside a door (D47): the ways out are solve, R retreat
	// and :q! give up. A timeout or a give-up loses the room, not the boot.
	"door.timeout": poolOf(`
The Rabbit's watch runs out. The room is lost; the boot is not.
Time. The Queen takes the door and her cycles. The hall goes on.
`),
	"door.gave-up": poolOf(`
You walk out. The Queen keeps the door, and her cut of your cycles.
:q!, and the door shuts behind you. The White Queen pencils it in.
`),
	"door.retreat":        fixedOf(`"I should advise you to walk the other way," says the Rose. New doors.`),
	"door.retreat.locked": fixedOf("{time} in, the door has shut behind you. No walking the other way now."),
	"verdict.WA.ecc":      fixedOf("ECC corrects the bit. No head for the Queen; the Rabbit's watch pays."),

	// ---- The map: the hub between doors ----
	"map.suspend":           fixedOf("Suspended. The Rabbit pockets his watch: it only ticks behind doors."),
	"map.resume":            fixedOf("fg: boot #{boot} picks up where it stopped. Nothing ran while it slept."),
	"map.abandon":           fixedOf(`"Let's all move one place on." Boot #{boot} stays behind, as a crash.`),
	"route.locked":          fixedOf("The Gryphon guards this lock. {module} opens it."),
	"route.locked.prestige": fixedOf("The glass won't let you by yet. This exit opens from ring {ring} down."),
	"lib.saved": pool2Of(`
"When I use a word, it means just what I choose it to mean—
neither more nor less." lib {name}, saved.

"When I make a word do a lot of work like that, I always pay it
extra." lib {name}, linked.
`),
}

// rings are the story's privilege ring and {place} by prestige
// (docs/story/text.md, the placeholder table): one level a ring from ring 3
// down to ring -1, then two levels each for ring -2 (SMM) and ring -3 (the
// Management Engine), the floor (docs/design/map.md (d)).
var rings = []struct {
	ring  int
	place string
}{
	{3, "Userland"}, {2, "the looking-glass"}, {1, "the nameless wood"},
	{0, "the Court of Hearts"}, {-1, "the Red King's dream"},
	{-2, "the Walrus's beach"}, {-2, "the Walrus's beach"},
	{-3, "the riverbank"}, {-3, "the riverbank"},
}

// Ring is the privilege ring prestige p puts you in and its {place}. Past
// the table the ring stays at the floor and the place is "", and a line
// that needs it should not be shown: the story has no name for a level
// nobody reached. A negative p reads as 3 - p, with no place.
func Ring(p int) (ring int, place string) {
	switch {
	case p < 0:
		return 3 - p, ""
	case p >= len(rings):
		return rings[len(rings)-1].ring, ""
	}
	return rings[p].ring, rings[p].place
}
