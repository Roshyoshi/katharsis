package flavor

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// placeholderWidths is text.md's placeholder table: the widest value each
// placeholder carries once the caller has truncated it.
var placeholderWidths = map[string]int{
	"boot": 5, "ring": 2, "place": 20, "bytes": 8, "title": 24, "tags": 24,
	"technique": 15, "n": 2, "ago": 11, "pct": 5, "module": 24,
	"division": 8, "cap": 14, "biome": 18, "name": 16, "time": 5,
}

// storyMaxWidth is the bible's tone rule 1: a line is at most 70 display
// columns once its placeholders are filled.
const storyMaxWidth = 70

var placeholder = regexp.MustCompile(`\{[^{}]*\}`)

// widest fills every placeholder in s at its maximum width.
func widest(t *testing.T, where, s string) string {
	t.Helper()
	return placeholder.ReplaceAllStringFunc(s, func(p string) string {
		w, ok := placeholderWidths[p[1:len(p)-1]]
		if !ok {
			t.Errorf("%s: unknown placeholder %s in %q", where, p, s)
		}
		return strings.Repeat("W", w)
	})
}

// allLines is every line of a hook: a pool's entries split at their line
// breaks, a block's lines.
func allLines(h storyHook) []string {
	var out []string
	for _, e := range h.entries {
		out = append(out, strings.Split(e, "\n")...)
	}
	return out
}

func TestStoryHooks(t *testing.T) {
	if len(story) < 100 {
		t.Fatalf("%d hooks; text.md has well over 100", len(story))
	}
	for hook, h := range story {
		if len(h.entries) == 0 {
			t.Errorf("%s has no entries", hook)
		}
		if h.kind == fixed && len(h.entries) != 1 {
			t.Errorf("%s is fixed with %d entries", hook, len(h.entries))
		}
		for _, e := range h.entries {
			if strings.TrimSpace(e) == "" {
				t.Errorf("%s has an empty entry", hook)
			}
		}
		for _, l := range allLines(h) {
			if l != strings.TrimRight(l, " ") || strings.ContainsAny(l, "\t\x1b\r") {
				t.Errorf("%s: trailing space or control character in %q", hook, l)
			}
			if strings.TrimSpace(l) == "" {
				t.Errorf("%s has a blank line inside an entry", hook)
			}
		}
		// Each hook answers through its own accessor, and only that one.
		switch s, b := Story(hook, 0), StoryBlock(hook); h.kind {
		case block:
			if s != "" || !slices.Equal(b, h.entries) {
				t.Errorf("block %s: Story %q, StoryBlock %q", hook, s, b)
			}
		default:
			if s == "" || b != nil {
				t.Errorf("%s: Story %q, StoryBlock %q", hook, s, b)
			}
		}
	}
	if Story("no.such.hook", 1) != "" || StoryBlock("no.such.hook") != nil {
		t.Error("an unknown hook has text")
	}
	b := StoryBlock("story.prologue")
	b[0] = "mutated"
	if StoryBlock("story.prologue")[0] == "mutated" {
		t.Error("StoryBlock hands out the table itself")
	}
}

func TestStoryWidths(t *testing.T) {
	for hook, h := range story {
		for _, l := range allLines(h) {
			if w := ansi.StringWidth(widest(t, hook, l)); w > storyMaxWidth {
				t.Errorf("%s: %d columns filled, want <= %d: %q", hook, w, storyMaxWidth, l)
			}
		}
	}
}

func TestStoryPlaceholdersFill(t *testing.T) {
	var pairs []string
	for name := range placeholderWidths {
		pairs = append(pairs, name, "v")
	}
	for hook, h := range story {
		for _, e := range h.entries {
			if got := Fill(e, pairs...); strings.ContainsAny(got, "{}") {
				t.Errorf("%s: %q survives filling as %q", hook, e, got)
			}
		}
	}
	if got := Fill("+{bytes} on {title}", "bytes", "15.2 KiB", "title", "Bovine Shuffle"); got != "+15.2 KiB on Bovine Shuffle" {
		t.Errorf("Fill = %q", got)
	}
	// A value is never read as a placeholder of its own.
	if got := Fill("{name} {title}", "name", "{title}", "title", "x"); got != "{title} x" {
		t.Errorf("Fill refilled a value: %q", got)
	}
	if got := Fill("{bytes} left", "title", "x"); got != "{bytes} left" {
		t.Errorf("Fill dropped an unfilled placeholder: %q", got)
	}
}

// TestStoryTone checks the tone rules a test can see: plain glyphs (rule
// 10), no Disney lyric (rule 9), and the retention guardrail against guilt
// copy (docs/research/retention.md GR8).
func TestStoryTone(t *testing.T) {
	plain := func(r rune) bool {
		return r < unicode.MaxASCII || strings.ContainsRune("—·…‘’“”", r)
	}
	guilt := []string{"miss you", "missed you", "misses you", "where were you",
		"been a while", "come back soon", "streak", "don't leave"}
	for hook, h := range story {
		for _, l := range allLines(h) {
			for _, r := range l {
				if !plain(r) {
					t.Errorf("%s: %q is not a plain glyph in %q", hook, r, l)
				}
			}
			low := strings.ToLower(l)
			if strings.Contains(low, "i'm late, i'm late") || strings.Contains(low, "very important date") {
				t.Errorf("%s: the Disney song: %q", hook, l)
			}
			for _, g := range guilt {
				if strings.Contains(low, g) {
					t.Errorf("%s: guilt copy %q in %q", hook, g, l)
				}
			}
		}
	}
}

func TestStoryRotation(t *testing.T) {
	for hook, h := range story {
		if h.kind != pool || len(h.entries) < 2 {
			continue
		}
		n := uint64(len(h.entries))
		for _, base := range []uint64{0, 7, 1 << 40} {
			seen := map[string]bool{}
			for s := base; s < base+n; s++ {
				a := Story(hook, s)
				if a == Story(hook, s+1) {
					t.Errorf("%s repeats %q at seeds %d and %d", hook, a, s, s+1)
				}
				if a != Story(hook, s) {
					t.Errorf("%s: seed %d is not stable", hook, s)
				}
				seen[a] = true
			}
			if len(seen) != len(h.entries) {
				t.Errorf("%s: %d consecutive seeds from %d visit %d of %d entries",
					hook, n, base, len(seen), n)
			}
		}
	}
}

// TestStoryMatchesText holds the table to docs/story/text.md: the same
// lines, as many times each. The doc is the storyteller's copy; this test
// fails when one changes without the other.
func TestStoryMatchesText(t *testing.T) {
	raw, err := os.ReadFile("../../docs/story/text.md")
	if err != nil {
		t.Fatalf("read text.md: %v", err)
	}
	_, hooks, ok := strings.Cut(string(raw), "\n## Boot and GRUB\n")
	if !ok {
		t.Fatal(`text.md has no "## Boot and GRUB" section`)
	}
	doc := map[string]int{}
	in := false
	for _, l := range strings.Split(hooks, "\n") {
		switch {
		case l == "```text":
			in = true
		case l == "```":
			in = false
		case in && l != "":
			doc[l]++
		}
	}
	table := map[string]int{}
	for _, h := range story {
		for _, l := range allLines(h) {
			table[l]++
		}
	}
	for l, n := range doc {
		if table[l] != n {
			t.Errorf("text.md has %d of %q, the table %d", n, l, table[l])
		}
	}
	for l, n := range table {
		if doc[l] == 0 {
			t.Errorf("the table has %d of %q, text.md none", n, l)
		}
	}
}

// register is the test's copy of text.md's quote register: the exact text
// of every source quote, "…" marking an elision and " / " a line break.
// TestRegisterMatchesText checks it against the doc.
var register = map[string]string{
	"A1":  "Oh dear! Oh dear! I shall be late!",
	"A2":  "Oh my ears and whiskers, how late it's getting!",
	"A3":  "Down, down, down. Would the fall never come to an end?",
	"A4":  "There were doors all round the hall, but they were all locked",
	"A5":  "in a sort of circle",
	"A6":  "Everybody has won, and all must have prizes.",
	"A7":  "There goes Bill!",
	"A8":  "There ought to be a book written about me, that there ought!",
	"A9":  "I almost wish I hadn't gone down that rabbit-hole—and yet—and yet—",
	"A10": "Who are you?",
	"A11": "I know who I was when I got up this morning",
	"A12": "There's certainly too much pepper in that soup!",
	"A13": "it makes rather a handsome pig, I think.",
	"A14": "Cheshire Puss,",
	"A15": "That depends a good deal on where you want to get to,",
	"A16": "we're all mad here. I'm mad. You're mad.",
	"A17": "Did you say pig, or fig?",
	"A18": "a grin without a cat!",
	"A19": "No room! No room!",
	"A20": "Have some wine,",
	"A21": "There isn't any,",
	"A22": "Why is a raven like a writing-desk?",
	"A23": "Two days wrong!",
	"A24": "It was the best butter,",
	"A25": "He's murdering the time! Off with his head!",
	"A26": "It's always six o'clock now.",
	"A27": "it's always tea-time",
	"A28": "as the things get used up.",
	"A29": "it's very easy to take more than nothing.",
	"A30": "Treacle,",
	"A31": "let's all move one place on.",
	"A32": "At any rate I'll never go there again!",
	"A33": "Off with his head!",
	"A34": "Nonsense!",
	"A35": "The players all played at once without waiting for turns",
	"A36": "fighting for the hedgehogs",
	"A37": "Everything's got a moral, if only you can find it.",
	"A38": "The more there is of mine, the less there is of yours.",
	"A39": "We had the best of educations",
	"A40": "I only took the regular course.",
	"A41": "Reeling and Writhing, of course, to begin with,",
	"A42": "and then the different branches of Arithmetic—Ambition, Distraction, Uglification, and Derision.",
	"A43": "Ten hours the first day,",
	"A44": "because they lessen from day to day.",
	"A45": "And how did you manage on the twelfth?",
	"A46": "That's enough about lessons,",
	"A47": "Consider your verdict,",
	"A48": "Where shall I begin, please your Majesty?",
	"A49": "Begin at the beginning, … and go on till you come to the end: then stop.",
	"A50": "Rule Forty-two. All persons more than a mile high to leave the court.",
	"A51": "If there's no meaning in it, … that saves a world of trouble",
	"A52": "Sentence first—verdict afterwards.",
	"A53": "You're nothing but a pack of cards!",
	"A54": "Wake up, Alice dear!",
	"A55": "Curiouser and curiouser!",

	"G1":  "'Twas brillig, and the slithy toves",
	"G2":  "Beware the Jabberwock, my son! / The jaws that bite, the claws that catch!",
	"G3":  "One, two! One, two! And through and through / The vorpal blade went snicker-snack!",
	"G4":  "He left it dead, and with its head / He went galumphing back.",
	"G5":  "O frabjous day! Callooh! Callay!",
	"G6":  "only the things go the other way.",
	"G7":  "Now, here, you see, it takes all the running you can do, to keep in the same place.",
	"G8":  "you must run at least twice as fast as that!",
	"G9":  "I should advise you to walk the other way.",
	"G10": "This must be the wood, … where things have no names.",
	"G11": "I'm a Fawn!",
	"G12": "Contrariwise,",
	"G13": "if it was so, it might be; and if it were so, it would be; but as it isn't, it ain't. That's logic.",
	"G14": "Ditto / Ditto, ditto",
	"G15": "He's dreaming now, … Why, about you!",
	"G16": "you're only a sort of thing in his dream!",
	"G17": "one's memory works both ways.",
	"G18": "It's a poor sort of memory that only works backwards,",
	"G19": "I haven't pricked it yet, … but I soon shall—oh, oh, oh!",
	"G20": "Things flow about so here!",
	"G21": "whenever she looked hard at any shelf",
	"G22": "that particular shelf was always quite empty",
	"G23": "fourteen pairs",
	"G24": "sometimes like a doll and sometimes like a work-box",
	"G25": "When I use a word, … it means just what I choose it to mean—neither more nor less.",
	"G26": "When I make a word do a lot of work like that, … I always pay it extra.",
	"G27": "That seems to be done right—",
	"G28": "All the King's horses and all the King's men / Couldn't put Humpty Dumpty in his place again.",
	"G29": "it's my crown all the while!",
	"G30": "It's my own invention",
	"G31": "The name of the song is called 'Haddocks' Eyes.'",
	"G32": "Which do you think it was?",
	"G33": "Life, what is it but a dream?",

	"S1": "to the spirit that lives in the computer.",
	"S2": "In effect, we conjure the spirits of the computer with our spells.",
	"S3": "The evaluator, … is just another program.",
	"S5": "Time Is of the Essence",
}

// use is one place a hook quotes the register: the whole quote, or only
// frag, which must be a piece of it.
type use struct{ id, frag string }

// quoted lists, per hook, the register quotes its lines carry verbatim.
// text.md's "uses" also name allusions and parodies; those are not quotes,
// so they are not here (see alludedOnly).
var quoted = map[string][]use{
	"boot.motd":                      {{"S1", ""}, {"S2", ""}, {"A55", ""}, {"G1", "slithy toves"}},
	"grub.subtitle.prestige.2":       {{"G10", "where things have no names"}},
	"grub.subtitle.rtfm":             {{"A41", "Reeling and Writhing"}},
	"grub.subtitle.coredump":         {{"A18", ""}},
	"grub.subtitle.gitlog":           {{"A8", ""}},
	"grub.subtitle.top":              {{"A10", ""}},
	"grub.subtitle.poweroff":         {{"A54", ""}},
	"story.prologue":                 {{"A1", ""}, {"A3", ""}},
	"story.first-merge":              {{"A19", ""}, {"A26", ""}},
	"daemon.intro":                   {{"G30", ""}},
	"story.first-boss":               {{"A48", ""}, {"A49", ""}},
	"story.first-crash":              {{"A18", ""}},
	"story.first-rtfm":               {{"A39", ""}, {"A41", ""}, {"A42", ""}},
	"story.first-regression":         {{"G17", ""}},
	"reveal.prestige.1":              {{"G6", ""}, {"G7", ""}},
	"reveal.prestige.2":              {{"G11", ""}},
	"reveal.prestige.3":              {{"A53", "nothing but a pack of cards"}, {"S3", ""}},
	"reveal.prestige.4":              {{"G15", ""}},
	"reveal.final-boss":              {{"G2", ""}},
	"reveal.final-clear":             {{"G5", ""}, {"G32", ""}, {"G33", ""}},
	"biome.intro.sorting-greedy":     {{"A29", ""}},
	"biome.intro.prefix-sums":        {{"A43", ""}},
	"biome.intro.two-pointers":       {{"A31", ""}},
	"biome.intro.graphs":             {{"A15", ""}},
	"biome.intro.shortest-paths":     {{"G9", ""}},
	"biome.intro.dsu":                {{"G29", ""}},
	"biome.intro.dp":                 {{"A11", ""}},
	"biome.intro.math":               {{"A42", "Ambition, Distraction, Uglification, and Derision"}},
	"biome.intro.strings":            {{"G1", ""}},
	"biome.intro.data-structures":    {{"A38", ""}},
	"biome.intro.geometry":           {{"G6", ""}},
	"biome.intro.mixed":              {{"A16", "we're all mad here"}},
	"boss.enter.off-by-one":          {{"G12", ""}, {"G13", "if it was so, it might be; and if it were so, it would be"}},
	"boss.defeat.off-by-one":         {{"G14", ""}, {"G13", "but as it isn't, it ain't. That's logic."}},
	"boss.enter.heisenbug":           {{"G20", ""}, {"G23", ""}},
	"boss.defeat.oom-killer":         {{"A7", ""}},
	"boss.enter.race-condition":      {{"A5", ""}, {"A35", ""}, {"S5", ""}},
	"boss.defeat.race-condition":     {{"A6", ""}},
	"boss.enter.undefined-behavior":  {{"A12", ""}},
	"boss.defeat.undefined-behavior": {{"A13", ""}},
	"boss.enter.halting-problem":     {{"G2", ""}},
	"boss.defeat.halting-problem":    {{"G3", ""}, {"G4", ""}},
	"merge.line": {{"A26", ""}, {"A27", ""}, {"A28", ""}, {"A22", ""}, {"A23", ""},
		{"A24", ""}, {"A20", ""}},
	"merge.full":                     {{"A19", ""}},
	"merge.commit":                   {{"A30", ""}},
	"merge.leave":                    {{"A32", ""}},
	"crash.epitaph.timed-out":        {{"A2", ""}},
	"crash.epitaph.out-of-lives":     {{"A47", ""}},
	"crash.epitaph.out-of-lives.TLE": {{"A25", ""}, {"A33", ""}},
	"crash.epitaph.out-of-lives.MLE": {{"A50", ""}},
	"crash.epitaph.gave-up":          {{"A53", ""}, {"A54", ""}, {"A34", ""}, {"A9", ""}},
	"crash.epitaph.cursed":           {{"A52", ""}},
	"coredump.lost":                  {{"G28", ""}},
	"cleared.0":                      {{"A6", ""}},
	"cleared.2":                      {{"G27", ""}},
	"cleared.4":                      {{"G5", ""}, {"G33", ""}},
	"duck.quack.1":                   {{"A14", ""}},
	"duck.quack.2":                   {{"A16", "I'm mad. You're mad."}},
	"duck.empty":                     {{"A14", ""}},
	"duck.refill":                    {{"A17", ""}},
	"upgrade.overclock":              {{"G8", ""}},
	"o3.bonus":                       {{"G8", "twice as fast as that!"}},
	"regression.door":                {{"G18", ""}},
	"regression.stage.1":             {{"G19", ""}},
	"rtfm.module.intro":              {{"A40", ""}},
	"rtfm.trial.start":               {{"A43", ""}},
	"rtfm.trial.fail":                {{"A46", ""}},
	"rtfm.underflow":                 {{"A45", ""}},
	"verdict.TLE":                    {{"A25", "He's murdering the time!"}},
	"verdict.MLE":                    {{"A50", "All persons more than a mile high to leave the court."}},
	"verdict.CE":                     {{"A51", ""}},
	"timer.warn.5m":                  {{"A1", ""}},
	"timer.warn.1m":                  {{"A2", ""}},
	"door.mystery":                   {{"G24", ""}},
	"lib.saved":                      {{"G25", ""}, {"G26", ""}},
	"help.vocab":                     {{"G25", ""}, {"G26", ""}},
	"white-knight.song":              {{"G30", ""}, {"G31", "Haddocks' Eyes"}},
}

// alludedOnly are register quotes no line carries verbatim: text.md marks
// every use of them as an allusion or a parody (A21's "There aren't any"),
// or names the quote only as a source (A37, the Duchess's morals).
var alludedOnly = []string{"A4", "A21", "A36", "A37", "A44", "G16", "G21", "G22"}

// TestStoryQuotesVerbatim is the bible's tone rule 6: a line that quotes
// Carroll or SICP carries the words exactly. Only the first letter's case
// and the closing punctuation of each piece may change, and a "…" in the
// register lets the line drop what it marks (or the dialogue tag there).
func TestStoryQuotesVerbatim(t *testing.T) {
	checked := map[string]bool{}
	for hook, uses := range quoted {
		h, ok := story[hook]
		if !ok {
			t.Errorf("quoted names hook %s, which the table lacks", hook)
			continue
		}
		for _, u := range uses {
			q, ok := register[u.id]
			if !ok {
				t.Errorf("%s: no register quote %s", hook, u.id)
				continue
			}
			checked[u.id] = true
			want := q
			if u.frag != "" {
				if !strings.Contains(strings.ToLower(q), strings.ToLower(u.frag)) {
					t.Errorf("%s: %q is not a piece of %s %q", hook, u.frag, u.id, q)
				}
				want = u.frag
			}
			if !slices.ContainsFunc(h.entries, func(e string) bool { return carries(joinLines(e, h), want) }) {
				t.Errorf("%s does not carry %s verbatim: %q", hook, u.id, want)
			}
		}
	}
	for _, id := range alludedOnly {
		if checked[id] {
			t.Errorf("%s is listed as alluded only, but a hook quotes it", id)
		}
		checked[id] = true
	}
	for id := range register {
		if !checked[id] {
			t.Errorf("register quote %s is neither quoted by a hook nor listed as alluded only", id)
		}
	}
}

// joinLines is one entry as running text. A block is one passage, so its
// lines join too; a line broken after an em dash joins without a space.
func joinLines(e string, h storyHook) string {
	ls := strings.Split(e, "\n")
	if h.kind == block {
		ls = h.entries
	}
	var b strings.Builder
	for i, l := range ls {
		if i > 0 && !strings.HasSuffix(ls[i-1], "—") {
			b.WriteByte(' ')
		}
		b.WriteString(l)
	}
	return b.String()
}

// carries reports whether text holds q's pieces in order. A piece is q
// split at "…" (an elision) and " / " (a line break); each piece may
// change the case of its first letter and drop its closing punctuation,
// but not run on into a longer word ("No room" is not in "No rooms").
func carries(text, q string) bool {
	at := 0
	for _, piece := range strings.FieldsFunc(strings.ReplaceAll(q, " / ", "…"), func(r rune) bool { return r == '…' }) {
		piece = strings.TrimRight(strings.TrimSpace(piece), ".,!?;:")
		if piece == "" {
			continue
		}
		i := findPiece(text, at, piece)
		if i < 0 {
			return false
		}
		at = i + len(piece)
	}
	return true
}

// findPiece is the first index at or after from where piece occurs as
// whole words, with the case of its first letter ignored; -1 if none.
func findPiece(s string, from int, piece string) int {
	r := []rune(piece)
	rest := string(r[1:])
	variants := []string{string(unicode.ToLower(r[0])) + rest, string(unicode.ToUpper(r[0])) + rest}
	word := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
	for i := from; i+len(piece) <= len(s); i++ {
		for _, v := range variants {
			if !strings.HasPrefix(s[i:], v) {
				continue
			}
			before, _ := utf8.DecodeLastRuneInString(s[:i])
			after, _ := utf8.DecodeRuneInString(s[i+len(v):])
			if word(r[0]) && word(before) || word(r[len(r)-1]) && word(after) {
				continue
			}
			return i
		}
	}
	return -1
}

// TestRegisterMatchesText keeps the test's register copy equal to the one
// in text.md, so the verbatim check above checks the doc's quotes.
func TestRegisterMatchesText(t *testing.T) {
	raw, err := os.ReadFile("../../docs/story/text.md")
	if err != nil {
		t.Fatalf("read text.md: %v", err)
	}
	row := regexp.MustCompile(`^\| ([AGS]\d+) \| (.*?) \| `)
	doc := map[string]string{}
	for _, l := range strings.Split(string(raw), "\n") {
		if m := row.FindStringSubmatch(l); m != nil {
			doc[m[1]] = m[2]
		}
	}
	for id, q := range register {
		if doc[id] != q {
			t.Errorf("register %s: test has %q, text.md %q", id, q, doc[id])
		}
	}
	for id, q := range doc {
		if _, ok := register[id]; !ok && id != "S4" { // S4 is cited by number, not quoted
			t.Errorf("text.md register %s %q is missing from the test's copy", id, q)
		}
	}
}
