package setuid_test

import (
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/setuid"
)

// TestTable: 61 bits, 23 progression, 16 skill, 10 secret and 12
// completion (achievements.md §(d)); ids unique, a secret's a dotfile with
// a hint and no other bit with one; every bit named and stating its
// barrier.
func TestTable(t *testing.T) {
	want := map[setuid.Kind]int{setuid.KindProgression: 23, setuid.KindSkill: 16, setuid.KindSecret: 10, setuid.KindCompletion: 12}
	got := map[setuid.Kind]int{}
	seen := map[string]bool{}
	for _, d := range setuid.Table {
		got[d.Kind]++
		if seen[d.ID] {
			t.Errorf("%s twice", d.ID)
		}
		seen[d.ID] = true
		if d.Name == "" || d.Text == "" || d.Kind.Dir() == "" {
			t.Errorf("%s: name %q, text %q, kind %q", d.ID, d.Name, d.Text, d.Kind)
		}
		if dot := strings.HasPrefix(d.ID, "."); dot != d.Secret() || (d.Hint != "") != d.Secret() {
			t.Errorf("%s: secret %v with hint %q", d.ID, d.Secret(), d.Hint)
		}
		if def, ok := setuid.Lookup(d.ID); !ok || def.ID != d.ID {
			t.Errorf("Lookup(%s) = %v, %v", d.ID, def.ID, ok)
		}
	}
	if len(setuid.Table) != 61 || !maps.Equal(got, want) {
		t.Errorf("%d bits by kind %v, want 61: %v", len(setuid.Table), got, want)
	}
	if d, _ := setuid.Lookup(".dull-reality"); d.Path() != "/home/you/.dull-reality" {
		t.Errorf("path %q", d.Path())
	}
	if d, _ := setuid.Lookup("hello-world"); d.Path() != "/bin/hello-world" {
		t.Errorf("path %q", d.Path())
	}
}

// TestTableTexts: the texts state the rules' own numbers, and none of
// them counts days or runs in a row (GR1, GR2) or says "missed" (GR8).
func TestTableTexts(t *testing.T) {
	text := func(id string) string {
		d, ok := setuid.Lookup(id)
		if !ok {
			t.Fatalf("no bit %s", id)
		}
		return d.Text
	}
	for id, want := range map[string]string{
		"through-the-glass":      "Unlock Silver: clear at Bronze once 60 doors are rated.",
		"speedrun-1":             "Clear a three-act boot in under 1:30:00 of door time.",
		"speedrun-2":             "Clear a three-act boot in under 1:00:00 of door time.",
		"speed-completion":       "Reach 100% with under 60 h of boot time (TIME+ less upsolves and trials).",
		".two-miles-high":        "Reach the cycle cap (♥ 50/50) in a boot.",
		"cartographer":           "Clear a room in every map node: 17.",
		"regular-course":         "Hold all 11 capabilities.",
		"apt-full":               "Install every package: 19.",
		"own-invention":          "Clear a boot with each daemon running: 7.",
		"every-topic":            "Master every topic but mixed: 15.",
		"one-topic":              "Master a topic: 10 rated doors on it.",
		".consider-your-verdict": "Hear every verdict on boot doors: AC, WA, TLE, MLE, RE, CE, OLE.",
	} {
		if got := text(id); got != want {
			t.Errorf("%s: %q, want %q", id, got, want)
		}
	}
	banned := []string{"streak", "in a row", "today", "missed", "daily", "monday", "tuesday", "wednesday",
		"thursday", "friday", "saturday", "sunday"}
	for _, d := range setuid.Table {
		for _, s := range []string{d.Name, d.Text, d.Hint} {
			low := strings.ToLower(s)
			for _, w := range banned {
				// "Sunny at Midnight" is the ring's name, no weekday.
				if strings.Contains(low, w) && !strings.Contains(low, "sunny") {
					t.Errorf("%s: %q says %q", d.ID, s, w)
				}
			}
		}
	}
}

// TestCompletionPoints: the divisions done make 100%, rings -2 and -3 and
// the Trial's ending the 12% beyond (§(e)); an empty save is 0%.
func TestCompletionPoints(t *testing.T) {
	m := &game.Meta{}
	if pct, parts := setuid.Completion(m); pct != 0 || len(parts) != 6 {
		t.Fatalf("empty save: %d%%, %d parts", pct, len(parts))
	}
	r := &m.Record
	r.Bosses = slices.Clone(setuid.Bosses)
	r.Nodes = slices.Clone(setuid.Nodes)
	m.Capabilities = slices.Collect(maps.Values(game.TopicCapabilities))
	r.Unlocked = []int{1, 2, 3, 4}
	r.Cleared = []int{4}
	r.Recovered, m.Retired, r.Skips, r.Nightly = 1, 1, 1, 1
	m.Libs = []game.Lib{{Name: "dsu"}}
	m.TopicAttempts = map[string]int{"dp": rules.Standard().MasteryAttempts}
	pct, parts := setuid.Completion(m)
	if pct != setuid.Full {
		t.Errorf("divisions done: %d%%, want %d: %+v", pct, setuid.Full, parts)
	}
	for _, p := range parts {
		if p.Name != "beyond" && (p.Have != p.Of || p.Points != p.Max) {
			t.Errorf("part %+v not full", p)
		}
	}
	r.Unlocked = append(r.Unlocked, 5, 6, 7, 8)
	r.Cleared = append(r.Cleared, 8)
	r.Endings = []string{game.EndingWake}
	if pct, parts := setuid.Completion(m); pct != setuid.Pure {
		t.Errorf("everything: %d%%, want %d: %+v", pct, setuid.Pure, parts)
	}
	// A deeper clear counts for the shallower ring's.
	r.Cleared = []int{8}
	if pct, _ := setuid.Completion(m); pct != setuid.Pure {
		t.Errorf("a P8 clear alone: %d%%, want %d", pct, setuid.Pure)
	}
	if !setuid.ShowsPercent(m) || setuid.ShowsPercent(&game.Meta{}) {
		t.Error("the percentage shows after a three-act clear, and only then")
	}
}

// TestEngineImportsNoBits: internal/game, and every package of this module
// it imports, never imports internal/setuid (GR7: the bits steer nothing).
func TestEngineImportsNoBits(t *testing.T) {
	seen := map[string]bool{}
	var walk func(pkg string)
	walk = func(pkg string) {
		if seen[pkg] {
			return
		}
		seen[pkg] = true
		dir := filepath.Join("..", strings.TrimPrefix(pkg, "katharsis/internal/"))
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("%s: no files in %s (%v)", pkg, dir, err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			af, err := parser.ParseFile(token.NewFileSet(), f, src, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, im := range af.Imports {
				path, _ := strconv.Unquote(im.Path.Value)
				if path == "katharsis/internal/setuid" {
					t.Errorf("%s imports %s", f, path)
				}
				if strings.HasPrefix(path, "katharsis/internal/") {
					walk(path)
				}
			}
		}
	}
	walk("katharsis/internal/game")
	if !seen["katharsis/internal/rules"] {
		t.Errorf("the walk missed game's imports: %v", slices.Sorted(maps.Keys(seen)))
	}
}
