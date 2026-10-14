package flavor

import "katharsis/internal/game"

// Every quote below was checked against a primary source or a reputable
// secondary one (Wikiquote "sourced" sections, Quote Investigator, the
// Jargon File, Wikipedia with a primary citation). The comment above each
// entry is the URL the wording was checked against. Wording is verbatim;
// "…" marks an omission, and Source starts with "attributed" when no
// primary record exists.
//
// Deliberately left out, so nobody re-adds them:
//   - "Computer science is no more about computers than astronomy is about
//     telescopes": not Dijkstra's; no source.
//   - "640K ought to be enough for anybody": Gates denies it; no source.
//   - "UNIX is very simple, it just needs a genius to understand its
//     simplicity": Wikiquote cites only quote sites ({{fix cite}}).
//   - Wirth's law: the 1995 paper is a scan, and copies disagree on the
//     exact wording.
//   - Torvalds, "Theory and practice sometimes clash…": the lore.kernel.org
//     record could not be fetched to check it.

// Shorthand for the tag sets below.
const (
	tOut  = game.TimedOut
	tLife = game.OutOfLives
	tQuit = game.GaveUp
)

// entry is a quote plus where it fits. A quote with causes and no verdicts
// is about the crash itself and fits whatever the last verdict was, so a
// timeout gets Hofstadter after a WA too. A quote with verdicts is about
// that failure and fits only the listed causes. general quotes are the
// filler QuoteFor mixes into every pool.
type entry struct {
	Quote
	causes   []game.Outcome
	verdicts []string
	general  bool
}

func on(c ...game.Outcome) []game.Outcome { return c }
func vs(v ...string) []string             { return v }

var quotes = []entry{
	// ---- the clock ran out ----------------------------------------------

	// https://en.wikipedia.org/wiki/Hofstadter%27s_law
	{Quote: Quote{"Hofstadter's Law: It always takes longer than you expect, even when you take into account Hofstadter's Law.",
		"Douglas Hofstadter", "Gödel, Escher, Bach", 1979},
		causes: on(tOut)},
	// https://en.wikipedia.org/wiki/Ninety%E2%80%93ninety_rule
	{Quote: Quote{"The first 90 percent of the code accounts for the first 90 percent of the development time. The remaining 10 percent of the code accounts for the other 90 percent of the development time.",
		"Tom Cargill", "quoted in Jon Bentley, \"Programming Pearls\", CACM", 1985},
		causes: on(tOut)},
	// https://en.wikiquote.org/wiki/Fred_Brooks (p. 25)
	{Quote: Quote{"Adding manpower to a late software project makes it later.",
		"Fred Brooks", "The Mythical Man-Month", 1975},
		causes: on(tOut)},
	// https://en.wikiquote.org/wiki/Fred_Brooks (p. 153)
	{Quote: Quote{"How does a project get to be a year late? … One day at a time.",
		"Fred Brooks", "The Mythical Man-Month", 1975},
		causes: on(tOut)},
	// https://www.rfc-editor.org/rfc/rfc1925 (truth 2a)
	{Quote: Quote{"No matter how hard you try, you can't make a baby in much less than 9 months. Trying to speed this up *might* make it slower, but it won't make it happen any quicker.",
		"Ross Callon (ed.)", "RFC 1925, The Twelve Networking Truths", 1996},
		causes: on(tOut)},
	// https://www.rfc-editor.org/rfc/rfc1925 (truth 9a)
	{Quote: Quote{"Every networking problem always takes longer to solve than it seems like it should.",
		"Ross Callon (ed.)", "RFC 1925, The Twelve Networking Truths", 1996},
		causes: on(tOut)},
	// https://www.rfc-editor.org/rfc/rfc1925 (truth 8)
	{Quote: Quote{"It is more complicated than you think.",
		"Ross Callon (ed.)", "RFC 1925, The Twelve Networking Truths", 1996},
		causes: on(tOut), general: true},
	// https://en.wikiquote.org/wiki/Alan_Turing (p. 460)
	{Quote: Quote{"We can only see a short distance ahead, but we can see plenty there that needs to be done.",
		"Alan Turing", "Computing Machinery and Intelligence", 1950},
		causes: on(tOut), general: true},

	// ---- TLE: performance and complexity --------------------------------

	// https://www.lysator.liu.se/c/pikestyle.html (Rule 1)
	{Quote: Quote{"You can't tell where a program is going to spend its time. Bottlenecks occur in surprising places, so don't try to second guess and put in a speed hack until you've proven that's where the bottleneck is.",
		"Rob Pike", "Notes on Programming in C", 1989},
		causes: on(tOut, tLife), verdicts: vs("TLE")},
	// https://www.lysator.liu.se/c/pikestyle.html (Rule 2)
	{Quote: Quote{"Measure. Don't tune for speed until you've measured, and even then don't unless one part of the code overwhelms the rest.",
		"Rob Pike", "Notes on Programming in C", 1989},
		causes: on(tOut, tLife), verdicts: vs("TLE")},
	// https://www.lysator.liu.se/c/pikestyle.html (Rule 3)
	{Quote: Quote{"Fancy algorithms are slow when n is small, and n is usually small. Fancy algorithms have big constants.",
		"Rob Pike", "Notes on Programming in C", 1989},
		causes: on(tOut, tLife), verdicts: vs("TLE")},
	// https://en.wikiquote.org/wiki/Donald_Knuth (Computing Surveys 6:4, §1)
	{Quote: Quote{"We should forget about small efficiencies, say about 97% of the time: premature optimization is the root of all evil.",
		"Donald Knuth", "Structured Programming with go to Statements", 1974},
		causes: on(tOut, tLife), verdicts: vs("TLE")},
	// https://www.cs.yale.edu/homes/perlis-alan/quotes.html (#55)
	{Quote: Quote{"A LISP programmer knows the value of everything, but the cost of nothing.",
		"Alan Perlis", "Epigrams on Programming", 1982},
		causes: on(tOut, tLife), verdicts: vs("TLE")},
	// https://www.rfc-editor.org/rfc/rfc1925 (truth 2)
	{Quote: Quote{"No matter how hard you push and no matter what the priority, you can't increase the speed of light.",
		"Ross Callon (ed.)", "RFC 1925, The Twelve Networking Truths", 1996},
		causes: on(tOut, tLife), verdicts: vs("TLE")},
	// https://www.rfc-editor.org/rfc/rfc1925 (truth 3, first two sentences)
	{Quote: Quote{"With sufficient thrust, pigs fly just fine. However, this is not necessarily a good idea.",
		"Ross Callon (ed.)", "RFC 1925, The Twelve Networking Truths", 1996},
		causes: on(tOut, tLife), verdicts: vs("TLE")},
	// https://www.rfc-editor.org/rfc/rfc1925 (truth 7a)
	{Quote: Quote{"Good, Fast, Cheap: Pick any two (you can't have all three).",
		"Ross Callon (ed.)", "RFC 1925, The Twelve Networking Truths", 1996},
		causes: on(tOut, tLife), verdicts: vs("TLE")},
	// http://www.catb.org/~esr/writings/taoup/html/ch01s06.html
	// No primary record; Lampson's 1983 "Hints" uses the line unattributed.
	{Quote: Quote{"When in doubt, use brute force.",
		"Ken Thompson", "attributed; quoted in The Art of Unix Programming", 0},
		causes: on(tOut, tLife), verdicts: vs("TLE"), general: true},

	// ---- MLE: memory and data -------------------------------------------

	// https://www.rfc-editor.org/rfc/rfc1925 (truth 9)
	{Quote: Quote{"For all resources, whatever it is, you need more.",
		"Ross Callon (ed.)", "RFC 1925, The Twelve Networking Truths", 1996},
		causes: on(tOut, tLife), verdicts: vs("MLE")},
	// https://www.lysator.liu.se/c/pikestyle.html (Rule 5)
	{Quote: Quote{"Data dominates. If you've chosen the right data structures and organized things well, the algorithms will almost always be self-evident. Data structures, not algorithms, are central to programming.",
		"Rob Pike", "Notes on Programming in C", 1989},
		causes: on(tOut, tLife), verdicts: vs("MLE"), general: true},
	// https://lwn.net/Articles/193244/ (git mailing list, quoted by LWN)
	{Quote: Quote{"Bad programmers worry about the code. Good programmers worry about data structures and their relationships.",
		"Linus Torvalds", "git mailing list", 2006},
		causes: on(tOut, tLife), verdicts: vs("MLE"), general: true},

	// ---- RE: crashes, pointers, undefined behavior ----------------------

	// https://www.infoq.com/presentations/Null-References-The-Billion-Dollar-Mistake-Tony-Hoare/
	// Transcript wording: https://en.wikipedia.org/wiki/Null_pointer#History
	{Quote: Quote{"I call it my billion-dollar mistake. It was the invention of the null reference in 1965.",
		"Tony Hoare", "Null References: The Billion Dollar Mistake, QCon London", 2009},
		causes: on(tOut, tLife), verdicts: vs("RE")},
	// https://en.wikiquote.org/wiki/Brian_Kernighan (2nd ed., ch. 2)
	{Quote: Quote{"Everyone knows that debugging is twice as hard as writing a program in the first place. So if you're as clever as you can be when you write it, how will you ever debug it?",
		"Brian Kernighan", "The Elements of Programming Style, 2nd ed.", 1978},
		causes: on(tOut, tLife), verdicts: vs("RE", "WA")},
	// https://www.stroustrup.com/quotes.html ("in 1986 or so")
	{Quote: Quote{"C makes it easy to shoot yourself in the foot; C++ makes it harder, but when you do it blows your whole leg off.",
		"Bjarne Stroustrup", "stroustrup.com/quotes", 1986},
		causes: on(tOut, tLife), verdicts: vs("RE")},
	// https://www.nokia.com/bell-labs/about/dennis-m-ritchie/chist.html
	{Quote: Quote{"C is quirky, flawed, and an enormous success.",
		"Dennis Ritchie", "The Development of the C Language", 1993},
		causes: on(tOut, tLife), verdicts: vs("RE")},
	// http://www.catb.org/jargon/html/N/nasal-demons.html
	// The Jargon File does not name the poster.
	{Quote: Quote{"When the compiler encounters [a given undefined construct] it is legal for it to make demons fly out of your nose",
		"a comp.std.c regular", "Usenet, via the Jargon File (\"nasal demons\")", 1992},
		causes: on(tOut, tLife), verdicts: vs("RE")},
	// https://en.wikiquote.org/wiki/Alan_Turing (p. 450)
	{Quote: Quote{"Machines take me by surprise with great frequency.",
		"Alan Turing", "Computing Machinery and Intelligence", 1950},
		causes: on(tOut, tLife), verdicts: vs("RE", "WA")},
	// https://lamport.azurewebsites.net/pubs/distributed-system.txt
	{Quote: Quote{"A distributed system is one in which the failure of a computer you didn't even know existed can render your own computer unusable.",
		"Leslie Lamport", "email to DEC SRC", 1987},
		causes: on(tOut, tLife), verdicts: vs("RE")},
	// https://www.gamedeveloper.com/programming/in-depth-functional-programming-in-c-
	{Quote: Quote{"A large fraction of the flaws in software development are due to programmers not fully understanding all the possible states their code may execute in.",
		"John Carmack", "Functional Programming in C++", 2012},
		causes: on(tOut, tLife), verdicts: vs("RE", "WA")},
	// https://go-proverbs.github.io/
	{Quote: Quote{"Don't communicate by sharing memory, share memory by communicating.",
		"Rob Pike", "Go Proverbs, Gopherfest", 2015},
		causes: on(tOut, tLife), verdicts: vs("RE")},
	// https://www.rfc-editor.org/rfc/rfc2324 (§2.3.2)
	{Quote: Quote{"Any attempt to brew coffee with a teapot should result in the error code \"418 I'm a teapot\". The resulting entity body MAY be short and stout.",
		"Larry Masinter", "RFC 2324, Hyper Text Coffee Pot Control Protocol", 1998},
		causes: on(tOut, tLife), verdicts: vs("RE")},
	// http://www.catb.org/jargon/html/story-of-mel.html
	{Quote: Quote{"If a program can't rewrite its own code, what good is it?",
		"Mel Kaye", "Ed Nather, The Story of Mel (Usenet)", 1983},
		causes: on(tOut, tLife), verdicts: vs("RE")},
	// https://en.wikiquote.org/wiki/Ken_Thompson (CACM 27:8)
	{Quote: Quote{"You can't trust code that you did not totally create yourself. (Especially code from companies that employ people like me.)",
		"Ken Thompson", "Reflections on Trusting Trust", 1984},
		causes: on(tOut, tLife), verdicts: vs("RE"), general: true},
	// https://en.wikiquote.org/wiki/Brian_Kernighan
	{Quote: Quote{"The most effective debugging tool is still careful thought, coupled with judiciously placed print statements.",
		"Brian Kernighan", "Unix for Beginners", 1979},
		causes: on(tOut, tLife), verdicts: vs("RE", "WA")},

	// ---- WA: correctness ------------------------------------------------

	// https://en.wikiquote.org/wiki/Edsger_W._Dijkstra (EWD249, §3)
	{Quote: Quote{"Program testing can be used to show the presence of bugs, but never to show their absence!",
		"Edsger W. Dijkstra", "Notes on Structured Programming (EWD249)", 1970},
		causes: on(tLife)},
	// https://www-cs-faculty.stanford.edu/~knuth/faq.html
	{Quote: Quote{"Beware of bugs in the above code; I have only proved it correct, not tried it.",
		"Donald Knuth", "memo to Peter van Emde Boas", 1977},
		causes: on(tLife)},
	// https://en.wikiquote.org/wiki/Charles_Babbage (ch. 5)
	{Quote: Quote{"On two occasions I have been asked, — \"Pray, Mr. Babbage, if you put into the machine wrong figures, will the right answers come out?\" … I am not able rightly to apprehend the kind of confusion of ideas that could provoke such a question.",
		"Charles Babbage", "Passages from the Life of a Philosopher", 1864},
		causes: on(tOut, tLife), verdicts: vs("WA")},
	// https://en.wikiquote.org/wiki/Ada_Lovelace (Note G)
	{Quote: Quote{"The Analytical Engine has no pretensions whatever to originate any thing. It can do whatever we know how to order it to perform.",
		"Ada Lovelace", "Note G, Sketch of the Analytical Engine", 1843},
		causes: on(tOut, tLife), verdicts: vs("WA")},
	// https://www.cs.yale.edu/homes/perlis-alan/quotes.html (#7)
	{Quote: Quote{"It is easier to write an incorrect program than understand a correct one.",
		"Alan Perlis", "Epigrams on Programming", 1982},
		causes: on(tOut, tLife), verdicts: vs("WA")},
	// https://www.cs.yale.edu/homes/perlis-alan/quotes.html (#32)
	{Quote: Quote{"Programmers are not to be measured by their ingenuity and their logic but by the completeness of their case analysis.",
		"Alan Perlis", "Epigrams on Programming", 1982},
		causes: on(tOut, tLife), verdicts: vs("WA")},
	// https://www.cs.yale.edu/homes/perlis-alan/quotes.html (#11)
	{Quote: Quote{"If you have a procedure with ten parameters, you probably missed some.",
		"Alan Perlis", "Epigrams on Programming", 1982},
		causes: on(tOut, tLife), verdicts: vs("WA")},
	// https://www.lysator.liu.se/c/pikestyle.html (Rule 4)
	{Quote: Quote{"Fancy algorithms are buggier than simple ones, and they're much harder to implement. Use simple algorithms as well as simple data structures.",
		"Rob Pike", "Notes on Programming in C", 1989},
		causes: on(tOut, tLife), verdicts: vs("WA", "RE")},
	// https://research.google/blog/extra-extra-read-all-about-it-nearly-all-binary-searches-and-mergesorts-are-broken/
	{Quote: Quote{"The general lesson that I take away from this bug is humility: It is hard to write even the smallest piece of code correctly, and our whole world runs on big, complex pieces of code.",
		"Joshua Bloch", "Nearly All Binary Searches and Mergesorts are Broken", 2006},
		causes: on(tOut, tLife), verdicts: vs("WA", "RE")},
	// https://en.wikipedia.org/wiki/Binary_search (TAOCP vol. 3, §6.2.1)
	{Quote: Quote{"Although the basic idea of binary search is comparatively straightforward, the details can be surprisingly tricky…",
		"Donald Knuth", "The Art of Computer Programming, Vol. 3", 1998},
		causes: on(tOut, tLife), verdicts: vs("WA")},
	// https://martinfowler.com/bliki/TwoHardThings.html
	{Quote: Quote{"There are 2 hard problems in computer science: cache invalidation, naming things, and off-by-1 errors.",
		"Leon Bambrick", "quoted in Martin Fowler, TwoHardThings", 0},
		causes: on(tOut, tLife), verdicts: vs("WA"), general: true},
	// http://regex.info/blog/2006-09-15/247 (alt.religion.emacs, 12 Aug 1997)
	{Quote: Quote{"Some people, when confronted with a problem, think \"I know, I'll use regular expressions.\" Now they have two problems.",
		"Jamie Zawinski", "alt.religion.emacs", 1997},
		causes: on(tOut, tLife), verdicts: vs("WA"), general: true},
	// https://www.rfc-editor.org/rfc/rfc793 (§2.10)
	{Quote: Quote{"TCP implementations will follow a general principle of robustness: be conservative in what you do, be liberal in what you accept from others.",
		"Jon Postel (ed.)", "RFC 793, Transmission Control Protocol", 1981},
		causes: on(tOut, tLife), verdicts: vs("WA")},
	// https://en.wikiquote.org/wiki/C._A._R._Hoare (CACM 24:2, p. 77)
	{Quote: Quote{"There are two ways of constructing a software design: One way is to make it so simple that there are obviously no deficiencies, and the other way is to make it so complicated that there are no obvious deficiencies.",
		"Tony Hoare", "The Emperor's Old Clothes", 1981},
		causes: on(tOut, tLife), verdicts: vs("WA"), general: true},
	// https://www.rfc-editor.org/rfc/rfc1925 (truth 1)
	{Quote: Quote{"It Has To Work.",
		"Ross Callon (ed.)", "RFC 1925, The Twelve Networking Truths", 1996},
		causes: on(tOut, tLife), verdicts: vs("WA")},

	// ---- out of lives: testing and bugs ---------------------------------

	// https://www.cs.yale.edu/homes/perlis-alan/quotes.html (#40)
	{Quote: Quote{"There are two ways to write error-free programs; only the third one works.",
		"Alan Perlis", "Epigrams on Programming", 1982},
		causes: on(tLife)},
	// https://en.wikiquote.org/wiki/Maurice_Wilkes (p. 145)
	{Quote: Quote{"…the realization came over me with full force that a good part of the remainder of my life was going to be spent in finding errors in my own programs.",
		"Maurice Wilkes", "Memoirs of a Computer Pioneer", 1985},
		causes: on(tLife)},
	// https://www.si.edu/object/log-book-computer-bug:nmah_334663
	// Wording: https://en.wikipedia.org/wiki/Bug_(engineering)
	{Quote: Quote{"First actual case of bug being found.",
		"Harvard Mark II operators", "log book, Smithsonian NMAH", 1947},
		causes: on(tLife)},
	// https://www.cs.utexas.edu/~EWD/transcriptions/EWD03xx/EWD340.html
	{Quote: Quote{"If you want more effective programmers, you will discover that they should not waste their time debugging, they should not introduce the bugs to start with.",
		"Edsger W. Dijkstra", "The Humble Programmer (EWD340)", 1972},
		causes: on(tLife)},

	// ---- :q! — show me the code, RTFM -----------------------------------

	// https://lkml.org/lkml/2000/8/25/132
	{Quote: Quote{"Talk is cheap. Show me the code.",
		"Linus Torvalds", "linux-kernel mailing list", 2000},
		causes: on(tQuit)},
	// https://www.nokia.com/bell-labs/about/dennis-m-ritchie/odd.html
	{Quote: Quote{"You are not expected to understand this.",
		"Unix Sixth Edition", "comment in slp.c", 1975},
		causes: on(tQuit)},
	// https://www.nokia.com/bell-labs/about/dennis-m-ritchie/odd.html
	{Quote: Quote{"The real problem is that we didn't understand what was going on either.",
		"Dennis Ritchie", "Odd Comments and Strange Doings in Unix", 0},
		causes: on(tQuit)},
	// https://www.nic.funet.fi/pub/Linux/kernel/v1.3/patch-html/patch-1.3.53/linux_Documentation_CodingStyle.html
	{Quote: Quote{"First off, I'd suggest printing out a copy of the GNU coding standards, and NOT read it. Burn them, it's a great symbolic gesture.",
		"Linus Torvalds", "Documentation/CodingStyle, Linux 1.3.53", 1995},
		causes: on(tQuit)},
	// https://archive.org/details/the-c-programming-language-1978-prentice-hall
	// (§1.1)
	{Quote: Quote{"The only way to learn a new programming language is by writing programs in it.",
		"Brian Kernighan & Dennis Ritchie", "The C Programming Language", 1978},
		causes: on(tQuit)},
	// https://mitp-content-server.mit.edu/books/content/sectbyfn/books_pres_0/6515/sicp.zip/full-text/book/book-Z-H-7.html
	{Quote: Quote{"Programs must be written for people to read, and only incidentally for machines to execute.",
		"Harold Abelson & Gerald Jay Sussman", "Structure and Interpretation of Computer Programs", 1985},
		causes: on(tQuit)},
	// https://en.wikipedia.org/wiki/Henry_Spencer
	// (sci.space.shuttle, 14 Nov 1987)
	{Quote: Quote{"Those who do not understand Unix are condemned to reinvent it, poorly.",
		"Henry Spencer", "Usenet", 1987},
		causes: on(tQuit), general: true},
	// https://en.wikiquote.org/wiki/Fred_Brooks (p. 116)
	{Quote: Quote{"Plan to throw one away; you will, anyhow.",
		"Fred Brooks", "The Mythical Man-Month", 1975},
		causes: on(tQuit)},
	// https://www.rfc-editor.org/rfc/rfc1925 (truth 6)
	{Quote: Quote{"It is easier to move a problem around (for example, by moving the problem to a different part of the overall network architecture) than it is to solve it.",
		"Ross Callon (ed.)", "RFC 1925, The Twelve Networking Truths", 1996},
		causes: on(tQuit)},
	// https://www.cs.yale.edu/homes/perlis-alan/quotes.html (#54)
	{Quote: Quote{"Beware of the Turing tar-pit in which everything is possible but nothing of interest is easy.",
		"Alan Perlis", "Epigrams on Programming", 1982},
		causes: on(tQuit)},
	// https://www.cs.yale.edu/homes/perlis-alan/quotes.html (#101)
	{Quote: Quote{"Dealing with failure is easy: Work hard to improve. Success is also easy to handle: You've solved the wrong problem. Work hard to improve.",
		"Alan Perlis", "Epigrams on Programming", 1982},
		causes: on(tQuit), general: true},
	// https://www.cs.yale.edu/homes/perlis-alan/quotes.html (#116)
	{Quote: Quote{"You think you know when you can learn, are more sure when you can write, even more when you can teach, but certain when you can program.",
		"Alan Perlis", "Epigrams on Programming", 1982},
		causes: on(tQuit)},
	// https://www.stroustrup.com/quotes.html ("sometime around 1990")
	{Quote: Quote{"I have always wished for my computer to be as easy to use as my telephone; my wish has come true because I can no longer figure out how to use my telephone.",
		"Bjarne Stroustrup", "stroustrup.com/quotes", 1990},
		causes: on(tQuit)},
	// https://en.wikiquote.org/wiki/Donald_Knuth (Vol. I, §1.1)
	{Quote: Quote{"An algorithm must be seen to be believed.",
		"Donald Knuth", "The Art of Computer Programming, Vol. 1", 1968},
		causes: on(tQuit)},
	// https://en.wikiquote.org/wiki/Larry_Wall (glossary)
	{Quote: Quote{"The three chief virtues of a programmer are: Laziness, Impatience and Hubris.",
		"Larry Wall", "Programming Perl", 1991},
		causes: on(tQuit)},

	// ---- CE: the language itself ----------------------------------------

	// https://www.cs.yale.edu/homes/perlis-alan/quotes.html (#3)
	{Quote: Quote{"Syntactic sugar causes cancer of the semicolon.",
		"Alan Perlis", "Epigrams on Programming", 1982},
		causes: on(tOut, tQuit), verdicts: vs("CE")},
	// https://lwn.net/Articles/249460/ (git mailing list, 6 Sep 2007)
	{Quote: Quote{"C++ is a horrible language.",
		"Linus Torvalds", "git mailing list", 2007},
		causes: on(tOut, tQuit), verdicts: vs("CE"), general: true},
	// https://www.stroustrup.com/quotes.html (D&E p. 207)
	{Quote: Quote{"Within C++, there is a much smaller and cleaner language struggling to get out.",
		"Bjarne Stroustrup", "The Design and Evolution of C++", 1994},
		causes: on(tOut, tQuit), verdicts: vs("CE")},
	// https://www.stroustrup.com/quotes.html
	{Quote: Quote{"There are only two kinds of languages: the ones people complain about and the ones nobody uses.",
		"Bjarne Stroustrup", "stroustrup.com/quotes", 0},
		causes: on(tOut, tQuit), verdicts: vs("CE"), general: true},
	// https://www.cs.yale.edu/homes/perlis-alan/quotes.html (#19)
	{Quote: Quote{"A language that doesn't affect the way you think about programming, is not worth knowing.",
		"Alan Perlis", "Epigrams on Programming", 1982},
		causes: on(tOut, tQuit), verdicts: vs("CE")},
	// https://en.wikiquote.org/wiki/Ken_Thompson
	// (UNIX Programming Env., p. 204)
	{Quote: Quote{"I'd spell creat with an e.",
		"Ken Thompson", "quoted in Kernighan & Pike, The UNIX Programming Environment", 1984},
		causes: on(tOut, tQuit), verdicts: vs("CE"), general: true},
	// https://en.wikiquote.org/wiki/Andrew_S._Tanenbaum (2nd ed., p. 254)
	{Quote: Quote{"The nice thing about standards is that you have so many to choose from.",
		"Andrew S. Tanenbaum", "Computer Networks, 2nd ed.", 1988},
		causes: on(tOut, tQuit), verdicts: vs("CE"), general: true},

	// ---- general: filler for every crash --------------------------------

	// https://martinfowler.com/bliki/TwoHardThings.html
	// Fowler found no primary record; Tim Bray heard it around 1996-7.
	{Quote: Quote{"There are only two hard things in Computer Science: cache invalidation and naming things.",
		"Phil Karlton", "attributed; via Martin Fowler, TwoHardThings", 0},
		general: true},
	// http://www.catb.org/jargon/html/Z/Zawinskis-Law.html
	{Quote: Quote{"Every program attempts to expand until it can read mail. Those programs which cannot so expand are replaced by ones which can.",
		"Jamie Zawinski", "Zawinski's Law, via the Jargon File", 0},
		general: true},
	// http://www.catb.org/~esr/writings/taoup/html/ch01s06.html (citing Salus)
	{Quote: Quote{"This is the Unix philosophy: Write programs that do one thing and do it well. Write programs to work together. Write programs to handle text streams, because that is a universal interface.",
		"Doug McIlroy", "quoted in Peter Salus, A Quarter Century of Unix", 1994},
		general: true},
	// https://en.wikiquote.org/wiki/Donald_Knuth (foreword to A=B)
	{Quote: Quote{"Science is what we understand well enough to explain to a computer. Art is everything else we do.",
		"Donald Knuth", "foreword to A=B", 1996},
		general: true},
	// https://quoteinvestigator.com/2012/09/27/invent-the-future/
	// Kay recalls first saying it at PARC in 1971; 1982 is the first print.
	{Quote: Quote{"The best way to predict the future is to invent it.",
		"Alan Kay", "Xerox PARC maxim, reported in InfoWorld", 1982},
		general: true},
	// https://www.joelonsoftware.com/2002/11/11/the-law-of-leaky-abstractions/
	{Quote: Quote{"All non-trivial abstractions, to some degree, are leaky.",
		"Joel Spolsky", "The Law of Leaky Abstractions", 2002},
		causes: on(tOut, tLife), verdicts: vs("RE"), general: true},
	// https://lkml.iu.edu/hypermail/linux/kernel/9607.2/0292.html
	// Uncommitted bytes are lost on a crash; this is the commit lecture.
	{Quote: Quote{"Only wimps use tape backup: _real_ men just upload their important stuff on ftp, and let the rest of the world mirror it ;)",
		"Linus Torvalds", "linux-kernel mailing list", 1996},
		general: true},
	// https://go-proverbs.github.io/
	{Quote: Quote{"A little copying is better than a little dependency.",
		"Rob Pike", "Go Proverbs, Gopherfest", 2015},
		general: true},
	// https://www.cs.utexas.edu/~EWD/transcriptions/EWD03xx/EWD340.html
	{Quote: Quote{"The competent programmer is fully aware of the strictly limited size of his own skull; therefore he approaches the programming task in full humility, and among other things he avoids clever tricks like the plague.",
		"Edsger W. Dijkstra", "The Humble Programmer (EWD340)", 1972},
		causes: on(tLife), general: true},
	// https://en.wikiquote.org/wiki/C._A._R._Hoare (p. 82)
	{Quote: Quote{"The price of reliability is the pursuit of the utmost simplicity. It is a price which the very rich find most hard to pay.",
		"Tony Hoare", "The Emperor's Old Clothes", 1981},
		general: true},
	// https://www.cs.yale.edu/homes/perlis-alan/quotes.html (#31)
	{Quote: Quote{"Simplicity does not precede complexity, but follows it.",
		"Alan Perlis", "Epigrams on Programming", 1982},
		general: true},
	// https://www.cs.yale.edu/homes/perlis-alan/quotes.html (#58)
	{Quote: Quote{"Fools ignore complexity. Pragmatists suffer it. Some can avoid it. Geniuses remove it.",
		"Alan Perlis", "Epigrams on Programming", 1982},
		general: true},
	// https://www.cs.yale.edu/homes/perlis-alan/quotes.html (#16)
	{Quote: Quote{"Every program has (at least) two purposes: the one for which it was written, and another for which it wasn't.",
		"Alan Perlis", "Epigrams on Programming", 1982},
		general: true},
	// https://www.cs.yale.edu/homes/perlis-alan/quotes.html (#15)
	{Quote: Quote{"Everything should be built top-down, except the first time.",
		"Alan Perlis", "Epigrams on Programming", 1982},
		causes: on(tQuit), general: true},
	// https://www.rfc-editor.org/rfc/rfc1925 (truth 12)
	{Quote: Quote{"In protocol design, perfection has been reached not when there is nothing left to add, but when there is nothing left to take away.",
		"Ross Callon (ed.)", "RFC 1925, The Twelve Networking Truths", 1996},
		general: true},
	// https://www.rfc-editor.org/rfc/rfc1925 (truth 6a)
	{Quote: Quote{"It is always possible to add another level of indirection.",
		"Ross Callon (ed.)", "RFC 1925, The Twelve Networking Truths", 1996},
		general: true},
	// https://en.wikiquote.org/wiki/Brian_Kernighan
	// (with P. J. Plauger, p. 319)
	{Quote: Quote{"Controlling complexity is the essence of computer programming.",
		"Brian Kernighan", "Software Tools", 1976},
		general: true},
	// https://www.kernel.org/doc/html/latest/process/coding-style.html
	// 1995: https://www.nic.funet.fi/pub/Linux/kernel/v1.3/patch-html/patch-1.3.53/linux_Documentation_CodingStyle.html
	// The original sentence opens "The answer to that is that if you need…".
	{Quote: Quote{"If you need more than 3 levels of indentation, you're screwed anyway, and should fix your program.",
		"Linus Torvalds", "Linux kernel coding style", 1995},
		general: true},
}
