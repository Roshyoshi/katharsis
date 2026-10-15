package flavor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

var (
	allOutcomes = []game.Outcome{game.Cleared, game.TimedOut, game.OutOfLives,
		game.GaveUp, game.Retreated, ""}
	crashCauses = []game.Outcome{game.TimedOut, game.OutOfLives, game.GaveUp}
	allVerdicts = []string{"", "AC", "WA", "TLE", "MLE", "OLE", "RE", "CE", "tle", " re ", "???"}
	tagVerdicts = map[string]bool{"TLE": true, "RE": true, "WA": true, "MLE": true, "CE": true}
)

func TestQuoteTable(t *testing.T) {
	if len(quotes) < 40 {
		t.Fatalf("%d quotes, want at least 40", len(quotes))
	}
	seen := map[string]bool{}
	for i, e := range quotes {
		q := e.Quote
		if q.Text == "" || q.Author == "" || q.Source == "" {
			t.Errorf("quote %d: empty field: %+v", i, q)
		}
		if q.Year < 0 || q.Year > 2026 {
			t.Errorf("quote %d: year %d", i, q.Year)
		}
		key := strings.ToLower(q.Text)
		if seen[key] {
			t.Errorf("duplicate quote %q", q.Text)
		}
		seen[key] = true
		if len(e.causes) == 0 && !e.general {
			t.Errorf("quote %d is in no pool: %q", i, q.Text)
		}
		if len(e.verdicts) > 0 && len(e.causes) == 0 {
			t.Errorf("quote %d has verdicts but no causes, so they never match: %q", i, q.Text)
		}
		for _, c := range e.causes {
			if c != game.TimedOut && c != game.OutOfLives && c != game.GaveUp {
				t.Errorf("quote %d: cause %q is not a crash cause", i, c)
			}
		}
		for _, v := range e.verdicts {
			if !tagVerdicts[v] {
				t.Errorf("quote %d: verdict tag %q", i, v)
			}
		}
	}
}

// TestQuoteSources holds the rule that every quote carries the URL it was
// checked against, as a comment directly above its entry.
func TestQuoteSources(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "quotes.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	urlEnds := map[int]bool{} // lines where a comment group with a URL ends
	for _, g := range f.Comments {
		if strings.Contains(g.Text(), "http") {
			urlEnds[fset.Position(g.End()).Line] = true
		}
	}
	var elts []ast.Expr
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if ok && len(vs.Names) == 1 && vs.Names[0].Name == "quotes" {
			elts = vs.Values[0].(*ast.CompositeLit).Elts
			return false
		}
		return true
	})
	if len(elts) != len(quotes) {
		t.Fatalf("parsed %d entries, table has %d", len(elts), len(quotes))
	}
	for i, el := range elts {
		if line := fset.Position(el.Pos()).Line; !urlEnds[line-1] {
			t.Errorf("quote %d (line %d, %q) has no source URL comment above it",
				i, line, quotes[i].Text)
		}
	}
}

func TestQuoteForAdjacentSeedsDiffer(t *testing.T) {
	seeds := []uint64{0, 1, 2, 41, 1 << 32, 0xdeadbeefcafef00d, 1<<63 - 1}
	for _, c := range allOutcomes {
		for _, v := range allVerdicts {
			for _, s := range seeds {
				for d := range uint64(20) {
					a, b := QuoteFor(c, v, s+d), QuoteFor(c, v, s+d+1)
					if a == b {
						t.Errorf("QuoteFor(%q, %q) repeats %q at seeds %d and %d",
							c, v, a.Text, s+d, s+d+1)
					}
				}
			}
		}
	}
}

// TestQuoteForPrefersMatches checks what lands in a pool: most picks match
// the crash, the rest is general filler, a known verdict brings in quotes
// about it, and no verdict keeps them out.
func TestQuoteForPrefersMatches(t *testing.T) {
	byText := map[string]entry{}
	for _, e := range quotes {
		byText[e.Text] = e
	}
	for _, c := range crashCauses {
		for _, v := range []string{"", "WA", "TLE", "RE", "MLE"} {
			matched, aboutVerdict, n := 0, 0, 400
			for seed := range uint64(n) {
				e := byText[QuoteFor(c, v, seed).Text]
				switch {
				case matches(e, c, v):
					matched++
					if len(e.verdicts) > 0 {
						aboutVerdict++
					}
				case !e.general:
					t.Fatalf("QuoteFor(%q, %q) gave %q: neither a match nor general", c, v, e.Text)
				}
			}
			// Filler is a quarter of the pool; anything near that is fine.
			if got := float64(matched) / float64(n); got < 0.65 {
				t.Errorf("QuoteFor(%q, %q): %.0f%% matched picks, want >= 65%%", c, v, 100*got)
			}
			if v == "" && aboutVerdict > 0 {
				t.Errorf("QuoteFor(%q, \"\") picked %d verdict quotes", c, aboutVerdict)
			}
			if v != "" && c != game.GaveUp && aboutVerdict == 0 {
				t.Errorf("QuoteFor(%q, %q) never picked a quote about %s", c, v, v)
			}
		}
	}
	// The design's anchors: a timeout without a verdict reaches Hofstadter,
	// and :q! reaches Torvalds.
	for c, want := range map[game.Outcome]string{
		game.TimedOut: "Hofstadter's Law",
		game.GaveUp:   "Show me the code",
	} {
		found := false
		for seed := range uint64(len(quotes)) {
			found = found || strings.Contains(QuoteFor(c, "", seed).Text, want)
		}
		if !found {
			t.Errorf("QuoteFor(%q, \"\") never reaches %q", c, want)
		}
	}
}

func TestEveryQuoteReachable(t *testing.T) {
	hit := map[string]bool{}
	for _, c := range allOutcomes {
		for _, v := range allVerdicts {
			for seed := range uint64(len(quotes)) {
				hit[QuoteFor(c, v, seed).Text] = true
			}
		}
	}
	for _, e := range quotes {
		if !hit[e.Text] {
			t.Errorf("never picked: %q", e.Text)
		}
	}
}

func TestStrideCoprime(t *testing.T) {
	for n := uint64(1); n < 200; n++ {
		for _, h := range []uint64{0, 1, 0xdeadbeef << 32, ^uint64(0)} {
			if s := stride(h, n); s == 0 || gcd(s, n) != 1 {
				t.Fatalf("stride(%#x, %d) = %d", h, n, s)
			}
		}
	}
}

func TestBiomes(t *testing.T) {
	names := map[string]string{}
	for _, topic := range catalog.Topics {
		name, blurb := BiomeName(topic), BiomeBlurb(topic)
		if name == "" || name == topic {
			t.Errorf("BiomeName(%q) = %q", topic, name)
		}
		if other, dup := names[name]; dup {
			t.Errorf("BiomeName(%q) = BiomeName(%q) = %q", topic, other, name)
		}
		names[name] = topic
		checkLine(t, "BiomeBlurb("+topic+")", blurb)
	}
	if got := BiomeName("no-such-topic"); got != "no-such-topic" {
		t.Errorf("unknown topic: BiomeName = %q", got)
	}
}

func TestBosses(t *testing.T) {
	for _, key := range append(append([]string{}, game.BossKeys...), game.FinalBoss) {
		if name := BossName(key); name == "" || name == key {
			t.Errorf("BossName(%q) = %q", key, name)
		}
		checkLine(t, "BossBlurb("+key+")", BossBlurb(key))
	}
	if BossName(game.FinalBoss) != "The Halting Problem" {
		t.Errorf("final boss is %q", BossName(game.FinalBoss))
	}
}

// TestPrestige: P0-P4 are the USACO divisions; the levels past them (P5-P8,
// the map's ladder) have no division and fall back to "Prestige N".
func TestPrestige(t *testing.T) {
	want := []string{"Bronze", "Silver", "Gold", "Platinum", "Advanced"}
	for p := len(want); p <= game.MaxPrestige; p++ {
		want = append(want, "Prestige "+strconv.Itoa(p))
	}
	if len(prestiges) > game.MaxPrestige+1 {
		t.Fatalf("MaxPrestige %d, table %d", game.MaxPrestige, len(prestiges))
	}
	for p := 0; p <= game.MaxPrestige; p++ {
		if got := PrestigeName(p); got != want[p] {
			t.Errorf("PrestigeName(%d) = %q, want %q", p, got, want[p])
		}
	}
	if PrestigeName(-1) == "" || PrestigeName(game.MaxPrestige+1) == "" {
		t.Error("out-of-range prestige has an empty name")
	}
}

func TestUpgradesAndDaemons(t *testing.T) {
	r := rules.Standard()
	for _, u := range game.AllUpgrades {
		name, desc := UpgradeTextFor(u, r)
		if name == "" || desc == "" {
			t.Errorf("UpgradeTextFor(%q) = %q, %q", u, name, desc)
		}
	}
	for _, d := range game.AllDaemons {
		name, desc := DaemonTextFor(d, r)
		if name == "" || desc == "" {
			t.Errorf("DaemonTextFor(%q) = %q, %q", d, name, desc)
		}
	}

	// The descriptions quote the rules, standard and custom alike.
	checks := []struct{ got, want string }{
		{second(UpgradeTextFor(game.Overclock, r)), "+15% timer"},
		{second(DaemonTextFor(game.Nice, r)), "+20% timer; every door pays 0.8x bytes."},
		{second(DaemonTextFor(game.Fork, r)), "4 doors per room after an AC (normally 3)"},
		{second(DaemonTextFor(game.DevRandom, r)), "pay 2x bytes"},
	}
	r.OverclockFrac, r.NiceTimerFrac, r.NiceBytesFrac, r.NiceLifeCost = 0.1, 0.35, 0, 2
	r.ForkDoors, r.DoorsPerRoom, r.DevRandomMult = 5, 2, 1.5
	checks = append(checks, []struct{ got, want string }{
		{second(UpgradeTextFor(game.Overclock, r)), "+10% timer"},
		{second(DaemonTextFor(game.Nice, r)), "+35% timer; a failed submit burns 2x."},
		{second(DaemonTextFor(game.Fork, r)), "5 doors per room after an AC (normally 2)"},
		{second(DaemonTextFor(game.DevRandom, r)), "pay 1.5x bytes"},
	}...)
	for _, c := range checks {
		if !strings.Contains(c.got, c.want) {
			t.Errorf("%q does not contain %q", c.got, c.want)
		}
	}
}

func second(_, b string) string { return b }

var narrow = regexp.MustCompile(`^[ -~\x{2500}-\x{257F}]*$`)

// Screens center banners in panes as narrow as a tmux split, so art stays
// within these and carries no color codes.
const (
	bannerMaxWidth = 60
	bannerMaxLines = 7
)

func TestBanners(t *testing.T) {
	for _, kind := range []string{"ac", "boot", "cleared", "grub"} {
		b := Banner(kind)
		if b == "" {
			t.Errorf("Banner(%q) is empty", kind)
			continue
		}
		lines := strings.Split(b, "\n")
		if len(lines) > bannerMaxLines {
			t.Errorf("Banner(%q): %d lines > %d", kind, len(lines), bannerMaxLines)
		}
		for i, l := range lines {
			if w := utf8.RuneCountInString(l); w > bannerMaxWidth {
				t.Errorf("Banner(%q) line %d: %d cols > %d", kind, i, w, bannerMaxWidth)
			}
			// ASCII and box drawing only: one column each, no escapes.
			if !narrow.MatchString(l) {
				t.Errorf("Banner(%q) line %d has a rune outside ASCII/box drawing: %q", kind, i, l)
			}
		}
	}
	// The boxed banner must close: every row of "ac" is the same width.
	ac := strings.Split(Banner("ac"), "\n")
	for _, l := range ac {
		if utf8.RuneCountInString(l) != utf8.RuneCountInString(ac[0]) {
			t.Errorf("Banner(\"ac\") rows differ in width:\n%s", Banner("ac"))
			break
		}
	}
	if Banner("nope") != "" {
		t.Error("unknown banner kind is not empty")
	}
}

var frame = regexp.MustCompile(`^[a-z_][a-z0-9_]*\+0x([0-9a-f]+)/0x([0-9a-f]+)$`)

func TestKernelHelpers(t *testing.T) {
	// The verdict picks the path when it explains the crash; :q! is always
	// a kill.
	for _, tc := range []struct {
		cause                game.Outcome
		verdict, reason, top string
	}{
		{game.TimedOut, "", "timer expired", "it_real_fn"},
		{game.TimedOut, "TLE", "timer expired", "watchdog_timer_fn"},
		{game.OutOfLives, " tle", "softlockup: hung tasks", "watchdog_timer_fn"},
		{game.OutOfLives, "OLE", "System is deadlocked on memory", "oom_kill_process"},
		{game.OutOfLives, "RE", "Fatal exception", "page_fault_oops"},
		{game.OutOfLives, "WA", "out of cycles", "do_exit"},
		{game.GaveUp, "RE", "Attempted to kill init! exitcode=0x00000009", "panic"},
		{game.Abandoned, "", "Attempted to kill init! exitcode=0x0000000f", "panic"},
	} {
		if got := PanicReason(tc.cause, tc.verdict); got != tc.reason {
			t.Errorf("PanicReason(%q, %q) = %q, want %q", tc.cause, tc.verdict, got, tc.reason)
		}
		if got := CallTrace(tc.cause, tc.verdict, 1)[0]; !strings.HasPrefix(got, tc.top+"+") {
			t.Errorf("CallTrace(%q, %q) starts at %q, want %s", tc.cause, tc.verdict, got, tc.top)
		}
	}

	for _, c := range allOutcomes {
		for _, v := range allVerdicts {
			if PanicReason(c, v) == "" {
				t.Errorf("PanicReason(%q, %q) is empty", c, v)
			}
			for seed := range uint64(30) {
				tr := CallTrace(c, v, seed)
				if len(tr) == 0 {
					t.Fatalf("CallTrace(%q, %q, %d) is empty", c, v, seed)
				}
				for _, f := range tr {
					m := frame.FindStringSubmatch(f)
					if m == nil {
						t.Fatalf("frame %q", f)
					}
					off, _ := strconv.ParseUint(m[1], 16, 64)
					size, _ := strconv.ParseUint(m[2], 16, 64)
					if off == 0 || off >= size {
						t.Fatalf("frame %q: offset outside the function", f)
					}
				}
			}
		}
	}
}

// checkLine holds blurbs to one short line, so route and door screens never
// wrap them.
func checkLine(t *testing.T, what, s string) {
	t.Helper()
	if s == "" {
		t.Errorf("%s is empty", what)
	}
	if strings.ContainsAny(s, "\n\t\x1b") {
		t.Errorf("%s has a control character: %q", what, s)
	}
	if n := utf8.RuneCountInString(s); n > 72 {
		t.Errorf("%s is %d runes, want <= 72: %q", what, n, s)
	}
}

// Every standard package says what it grants, in the rules' numbers, and
// every theme it installs has a GRUB banner carrying GRUB's version line
// once.
func TestPackages(t *testing.T) {
	r := rules.Standard()
	for _, p := range r.Packages {
		if PackageText(p, r) == "" {
			t.Errorf("%s: no text", p.Name)
		}
		if p.Theme != "" {
			art := Banner("grub:" + p.Theme)
			if strings.Count(art, "GNU GRUB  version 2.12") != 1 || strings.Count(art, "\n") < 2 {
				t.Errorf("%s: theme %q banner %q", p.Name, p.Theme, art)
			}
			for _, l := range strings.Split(art, "\n") {
				if len([]rune(l)) > 60 {
					t.Errorf("%s: banner line %d wide: %q", p.Name, len([]rune(l)), l)
				}
			}
		}
	}
	for _, c := range []struct {
		p    rules.Package
		want string
	}{
		{rules.Package{Daemon: string(game.Cron)}, "daemon cron joins the merge windows: A regression door pays 2x bytes."},
		{rules.Package{Item: string(game.At)}, "merge windows sell at: +05:00 on the next door's timer."},
		{rules.Package{Upgrade: string(game.ShufPick)}, "upgrade shuf joins the picks"},
		{rules.Package{LinkSlots: 1, Doors: 2}, "+1 lib link slot · +2 doors per room"},
		{rules.Package{Theme: "tux"}, "GRUB banner theme tux"},
	} {
		if got := PackageText(c.p, r); !strings.Contains(got, c.want) {
			t.Errorf("PackageText(%+v) = %q, want %q in it", c.p, got, c.want)
		}
	}
}
