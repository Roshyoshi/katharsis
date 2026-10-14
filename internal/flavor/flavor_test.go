package flavor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"runtime"
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

func TestQuoteForEveryPair(t *testing.T) {
	for _, c := range allOutcomes {
		for _, v := range allVerdicts {
			for seed := range uint64(50) {
				q := QuoteFor(c, v, seed)
				if q.Text == "" || q.Author == "" {
					t.Fatalf("QuoteFor(%q, %q, %d) = %+v", c, v, seed, q)
				}
				if q != QuoteFor(c, v, seed) {
					t.Fatalf("QuoteFor(%q, %q, %d) not deterministic", c, v, seed)
				}
			}
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

func TestPrestige(t *testing.T) {
	want := []string{"Bronze", "Silver", "Gold", "Platinum", "Advanced"}
	if len(want) != game.MaxPrestige+1 || len(prestiges) != game.MaxPrestige+1 {
		t.Fatalf("MaxPrestige %d, table %d", game.MaxPrestige, len(prestiges))
	}
	for p := 0; p <= game.MaxPrestige; p++ {
		if got := PrestigeName(p); got != want[p] {
			t.Errorf("PrestigeName(%d) = %q, want %q", p, got, want[p])
		}
		if got := PrestigeLabel(p); !strings.HasPrefix(got, want[p]+" · ring ") {
			t.Errorf("PrestigeLabel(%d) = %q", p, got)
		}
	}
	if PrestigeName(-1) == "" || PrestigeName(game.MaxPrestige+1) == "" {
		t.Error("out-of-range prestige has an empty name")
	}
}

func TestUpgradesAndDaemons(t *testing.T) {
	for _, u := range []game.Upgrade{game.Overclock, game.ECC, game.Duck} {
		name, desc := UpgradeText(u)
		if name == "" || desc == "" {
			t.Errorf("UpgradeText(%q) = %q, %q", u, name, desc)
		}
	}
	for _, d := range []game.Daemon{game.Strace, game.Nice, game.Fork, game.DevRandom, game.Kexec} {
		name, desc := DaemonText(d)
		if name == "" || desc == "" {
			t.Errorf("DaemonText(%q) = %q, %q", d, name, desc)
		}
	}

	// The descriptions quote the rules, standard and custom alike.
	r := rules.Standard()
	checks := []struct{ got, want string }{
		{second(UpgradeTextFor(game.Overclock, r)), "+15% timer"},
		{second(DaemonTextFor(game.Nice, r)), "+20% timer; a failed submit costs 2 lives"},
		{second(DaemonTextFor(game.Fork, r)), "4 doors per room after an AC (normally 3)"},
		{second(DaemonTextFor(game.DevRandom, r)), "pay 2x bytes"},
	}
	r.OverclockFrac, r.NiceTimerFrac, r.NiceLifeCost = 0.1, 0.35, 1
	r.ForkDoors, r.DoorsPerRoom, r.DevRandomMult = 5, 2, 1.5
	checks = append(checks, []struct{ got, want string }{
		{second(UpgradeTextFor(game.Overclock, r)), "+10% timer"},
		{second(DaemonTextFor(game.Nice, r)), "+35% timer; a failed submit costs 1 life."},
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

func TestBanners(t *testing.T) {
	for _, kind := range []string{"ac", "boot", "cleared", "grub"} {
		b := Banner(kind)
		if b == "" {
			t.Errorf("Banner(%q) is empty", kind)
			continue
		}
		lines := strings.Split(b, "\n")
		if len(lines) > BannerMaxLines {
			t.Errorf("Banner(%q): %d lines > %d", kind, len(lines), BannerMaxLines)
		}
		for i, l := range lines {
			if w := utf8.RuneCountInString(l); w > BannerMaxWidth {
				t.Errorf("Banner(%q) line %d: %d cols > %d", kind, i, w, BannerMaxWidth)
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

func TestSoundFile(t *testing.T) {
	_, err := os.Stat(soundDir)
	mac := runtime.GOOS == "darwin" && err == nil
	for _, name := range []string{"ac", "wa", "crash", "commit", "boss", "cleared"} {
		p := SoundFile(name)
		if p == "" {
			if mac {
				t.Errorf("SoundFile(%q) is empty on macOS", name)
			}
			continue
		}
		if _, err := os.Stat(p); err != nil {
			t.Errorf("SoundFile(%q) = %q: %v", name, p, err)
		}
	}
	if SoundFile("nope") != "" {
		t.Error("unknown sound is not empty")
	}
}

var frame = regexp.MustCompile(`^[a-z_][a-z0-9_]*\+0x([0-9a-f]+)/0x([0-9a-f]+)$`)

func TestKernelHelpers(t *testing.T) {
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
				if strings.Join(tr, ",") != strings.Join(CallTrace(c, v, seed), ",") {
					t.Fatalf("CallTrace(%q, %q, %d) not deterministic", c, v, seed)
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
	if got := PanicReason(game.GaveUp, "WA"); !strings.HasPrefix(got, "Attempted to kill init!") {
		t.Errorf("PanicReason(GaveUp) = %q", got)
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
