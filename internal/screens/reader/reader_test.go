package reader

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"deadcases/internal/screens"
	"deadcases/internal/screens/meta/fixture"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden from the current output")

func doc(d fixture.Doc) Doc {
	return Doc{Title: d.Title, Source: d.Source, URL: d.URL, Markdown: d.Markdown}
}

func golden(t *testing.T, name, view string) {
	t.Helper()
	got := plain(view)
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test -update to create it)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs from golden (go test -update to accept):\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}

func plain(view string) string {
	lines := strings.Split(ansi.Strip(view), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n") + "\n"
}

func key(s string) tea.KeyPressMsg {
	named := map[string]rune{
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "backspace": tea.KeyBackspace,
		"up": tea.KeyUp, "down": tea.KeyDown, "home": tea.KeyHome, "end": tea.KeyEnd,
		"pgdown": tea.KeyPgDown, "pgup": tea.KeyPgUp,
	}
	if c, ok := named[s]; ok {
		return tea.KeyPressMsg{Code: c}
	}
	if s == "space" {
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

// press feeds keys at w×h and returns what the last one emitted.
func press(m *Model, w, h int, keys ...string) tea.Msg {
	var msg tea.Msg
	m.View(w, h)
	for _, k := range keys {
		_, cmd := m.Update(key(k))
		msg = nil
		if cmd != nil {
			msg = cmd()
		}
		m.View(w, h)
	}
	return msg
}

func TestGoldens(t *testing.T) {
	cases := []struct {
		name string
		doc  fixture.Doc
		keys []string
	}{
		{"statement", fixture.Statement, nil},
		{"statement_end", fixture.Statement, []string{"G"}},
		{"learn", fixture.Learn, nil},
		{"learn_code", fixture.Learn, []string{"/", "c", "o", "u", "t", "enter"}},
		{"editorial", fixture.Editorial, nil},
	}
	for _, c := range cases {
		for _, sz := range [][2]int{{80, 24}, {140, 40}} {
			name := fmt.Sprintf("%s_%dx%d", c.name, sz[0], sz[1])
			t.Run(name, func(t *testing.T) {
				m := New(doc(c.doc))
				press(m, sz[0], sz[1], c.keys...)
				golden(t, name, m.View(sz[0], sz[1]))
			})
		}
	}
}

func TestFits(t *testing.T) {
	for _, d := range []fixture.Doc{fixture.Statement, fixture.Learn, fixture.Editorial, {Title: "empty"}} {
		for _, sz := range [][2]int{{1, 1}, {20, 6}, {40, 12}, {80, 24}, {100, 30}, {140, 40}, {220, 60}} {
			w, h := sz[0], sz[1]
			m := New(doc(d))
			for _, keys := range [][]string{nil, {"G"}, {"/", "a", "enter"}} {
				press(m, w, h, keys...)
				lines := strings.Split(m.View(w, h), "\n")
				if len(lines) != h {
					t.Errorf("%s at %dx%d after %v: %d lines, want %d", d.Title, w, h, keys, len(lines), h)
				}
				for i, l := range lines {
					if n := ansi.StringWidth(l); n > w {
						t.Errorf("%s at %dx%d after %v: line %d is %d wide: %q", d.Title, w, h, keys, i, n, ansi.Strip(l))
					}
					if strings.Contains(l, "\t") {
						t.Errorf("%s at %dx%d: line %d holds a tab, whose width the terminal decides: %q", d.Title, w, h, i, l)
					}
				}
				// View cuts lines to the width as a last resort; the viewport
				// itself must not need it, or the cut hides real overflow.
				for i, l := range strings.Split(m.vp.View(), "\n") {
					if n := ansi.StringWidth(l); w > 1 && n > w {
						t.Errorf("%s at %dx%d: viewport line %d is %d wide", d.Title, w, h, i, n)
					}
				}
			}
		}
	}
}

// TestSamplesSurvive checks the sample blocks come through rendering
// verbatim: same lines, same order, nothing reflowed or converted.
func TestSamplesSurvive(t *testing.T) {
	d := doc(fixture.Statement)
	d.Markdown += "\n### SAMPLE INPUT 2:\n\n```\n3 $5$ a_i\n#.#_*x*\n\\le 10^5\n    indented 1  2\n```\n"
	m := New(d)
	m.View(80, 200)
	got := strings.Join(m.plain, "\n")
	for _, block := range [][]string{
		{"4", "3 2 1 3", "5 1 7 2"},
		{"3 6"},
		{"3 $5$ a_i", "#.#_*x*", `\le 10^5`, "    indented 1  2"},
	} {
		idx := -1
		lines := strings.Split(got, "\n")
		for i := range lines {
			if strings.TrimSpace(lines[i]) == block[0] {
				idx = i
				break
			}
		}
		if idx < 0 {
			t.Errorf("sample block starting %q missing from:\n%s", block[0], got)
			continue
		}
		// Glamour indents code blocks by a fixed margin; strip exactly that.
		margin := len(lines[idx]) - len(strings.TrimLeft(lines[idx], " "))
		for j, want := range block {
			l := strings.TrimRight(lines[idx+j], " ")
			if len(l) < margin || l[margin:] != want {
				t.Errorf("sample line %d: got %q, want %q (margin %d)", j, l, want, margin)
			}
		}
	}
	if !strings.Contains(got, "1 ≤ N ≤ 2 · 10⁵") {
		t.Errorf("prose math was not converted:\n%s", got)
	}
}

func TestKeys(t *testing.T) {
	wantMsg := func(t *testing.T, got, want tea.Msg) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("emitted %#v, want %#v", got, want)
		}
	}
	t.Run("q and esc go back", func(t *testing.T) {
		wantMsg(t, press(New(doc(fixture.Statement)), 80, 24, "q"), screens.Back{})
		wantMsg(t, press(New(doc(fixture.Statement)), 80, 24, "esc"), screens.Back{})
	})
	t.Run("o opens the URL", func(t *testing.T) {
		wantMsg(t, press(New(doc(fixture.Statement)), 80, 24, "o"), OpenURL{URL: fixture.Statement.URL})
		d := doc(fixture.Statement)
		d.URL = ""
		wantMsg(t, press(New(d), 80, 24, "o"), nil)
	})
	t.Run("scrolling", func(t *testing.T) {
		// 80x12 leaves a 9-row viewport, so a page is 9 lines.
		m := New(doc(fixture.Statement))
		press(m, 80, 12, "j", "j")
		if got := m.vp.YOffset(); got != 2 {
			t.Errorf("j j: offset %d, want 2", got)
		}
		press(m, 80, 12, "space")
		if got := m.vp.YOffset(); got != 2+9 {
			t.Errorf("space: offset %d, want %d", got, 2+9)
		}
		press(m, 80, 12, "b", "k")
		if got := m.vp.YOffset(); got != 1 {
			t.Errorf("b k: offset %d, want 1", got)
		}
		press(m, 80, 12, "G")
		if !m.vp.AtBottom() {
			t.Error("G should reach the bottom")
		}
		press(m, 80, 12, "g")
		if !m.vp.AtTop() {
			t.Error("g should reach the top")
		}
	})
	t.Run("search", func(t *testing.T) {
		m := New(doc(fixture.Learn))
		if msg := press(m, 80, 24, "/", "q", "esc"); msg != nil {
			t.Errorf("q typed at the prompt must not leave: %#v", msg)
		}
		press(m, 80, 24, "/", "p", "[", "enter")
		if len(m.matches) < 2 {
			t.Fatalf("want several matches for p[, got %v", m.matches)
		}
		first := m.cur
		press(m, 80, 24, "n")
		if m.cur != (first+1)%len(m.matches) {
			t.Errorf("n: cur %d, want %d", m.cur, first+1)
		}
		press(m, 80, 24, "N")
		if m.cur != first {
			t.Errorf("N: cur %d, want %d", m.cur, first)
		}
		if v := plain(m.View(80, 24)); !strings.Contains(v, "match ") {
			t.Errorf("footer should count matches:\n%s", v)
		}
		// esc clears the search before it leaves.
		wantMsg(t, press(m, 80, 24, "esc"), nil)
		wantMsg(t, press(m, 80, 24, "esc"), screens.Back{})
	})
}

// TestResizeRerenders checks the wrap follows the width and the reading
// position survives a resize.
func TestResizeRerenders(t *testing.T) {
	m := New(doc(fixture.Statement))
	m.View(60, 20)
	narrow := m.vp.TotalLineCount()
	press(m, 60, 20, "j", "j", "j", "j", "j", "j")
	before := m.vp.YOffset()
	m.View(100, 20)
	if m.vp.TotalLineCount() >= narrow {
		t.Errorf("wider render should take fewer lines: %d at 100, %d at 60", m.vp.TotalLineCount(), narrow)
	}
	if after := m.vp.YOffset(); after == 0 || after > before {
		t.Errorf("offset %d at 60 cols became %d at 100; want it kept proportionally", before, after)
	}
	for _, l := range strings.Split(m.View(100, 20), "\n") {
		if ansi.StringWidth(l) > 100 {
			t.Errorf("line wider than 100: %q", ansi.Strip(l))
		}
	}
}

func TestPrepareTagsBareFences(t *testing.T) {
	in := "```\n1 2\n```\n\n```cpp\nint x;\n```\n\n~~~\n$a$\n~~~\n"
	want := "```text\n1 2\n```\n\n```cpp\nint x;\n```\n\n~~~text\n$a$\n~~~\n"
	if got := prepare(in); got != want {
		t.Errorf("prepare:\n got %q\nwant %q", got, want)
	}
}

func TestExpandTabs(t *testing.T) {
	in := "\tint x;\n\t\ty++;\nab\tc\n"
	want := "    int x;\n        y++;\nab  c\n"
	if got := expandTabs(in, 4); got != want {
		t.Errorf("expandTabs:\n got %q\nwant %q", got, want)
	}
	if got := prepare("```\n1\t2\n```\nprose\tstays\n"); got != "```text\n1   2\n```\nprose\tstays\n" {
		t.Errorf("prepare should expand tabs in code only, got %q", got)
	}
}
