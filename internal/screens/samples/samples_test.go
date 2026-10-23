package samples

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/hud"
	"katharsis/internal/judge"
	"katharsis/internal/screens"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden from the current output")

var sizes = [][2]int{{80, 24}, {140, 40}}

func golden(t *testing.T, name, view string) {
	t.Helper()
	lines := strings.Split(ansi.Strip(view), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	got := strings.Join(lines, "\n") + "\n"
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

// food is cf:1675A's sample, which an uninitialized loop counter answered
// with nothing under ASan (and 29/29 under -O2).
const (
	foodIn   = "7\n1 1 4 2 3\n0 0 0 0 0\n5 5 0 4 6\n1 1 1 1 1\n50000000 50000000 100000000 100000000 100000000\n0 0 0 100000000 100000000\n1 3 2 2 5\n"
	foodWant = "YES\nYES\nNO\nYES\nYES\nNO\nNO\n"
)

func build(log string) *judge.Build {
	return &judge.Build{OK: true, Log: log, Duration: 600 * time.Millisecond}
}

func pass(tests ...hud.PassTest) hud.Pass {
	return hud.Pass{Name: "cf1675A_food_for_animals", Profile: "asan+ubsan (libstdc++)", Build: build(""), Tests: tests}
}

func cost(r judge.Result) judge.Result {
	r.CPU, r.Wall, r.MaxRSS = 3*time.Millisecond, 41*time.Millisecond, 2<<20
	return r
}

var fixtures = map[string]hud.Pass{
	// Nothing printed: the output ends where the expected output's first
	// token is.
	"nothing": pass(hud.PassTest{Name: "1", In: foodIn, Want: foodWant, Done: true,
		Result: cost(judge.Result{Verdict: judge.WA, Reason: "line 1",
			Diff: &judge.Diff{Line: 1, Want: "YES", Got: judge.EOF, GotLine: 1}})}),
	// A wrong line among right ones, debug prints on stderr, a warning,
	// and a second sample that passed.
	"wa": func() hud.Pass {
		p := pass(
			hud.PassTest{Name: "1", In: foodIn, Want: foodWant, Done: true,
				Result: cost(judge.Result{Verdict: judge.WA, Reason: "line 3", Stdout: "YES\nYES\nYES\nYES\nYES\nNO\nNO\n",
					Stderr: "c=-1\nc=4\n", Diff: &judge.Diff{Line: 3, Want: "NO", Got: "YES", GotLine: 3}})},
			hud.PassTest{Name: "2", In: "1\n0 0 0 0 0\n", Want: "YES\n", Done: true,
				Result: cost(judge.Result{Verdict: judge.AC, Stdout: "YES\n"})})
		p.Build = build("food.cpp:12:9: warning: unused variable 'k' [-Wunused-variable]\n   12 |     int k;\n")
		return p
	}(),
	"ce": func() hud.Pass {
		p := pass(hud.PassTest{Name: "1", In: foodIn, Want: foodWant})
		p.Build = &judge.Build{OK: false, Log: "food.cpp:47:17: error: variable 'i' is uninitialized when used here [-Werror,-Wuninitialized]\n" +
			"   47 |     for (int i; i < t; i++) {\n      |                 ^\n" +
			"food.cpp:47:15: note: initialize the variable 'i' to silence this warning\n1 error generated.\n"}
		return p
	}(),
	"re": pass(hud.PassTest{Name: "1", In: "3\n1 2 3\n", Want: "6\n", Done: true,
		Result: cost(judge.Result{Verdict: judge.RE, Reason: "heap-buffer-overflow", Stdout: "partial\n",
			Stderr: "==1==ERROR: AddressSanitizer: heap-buffer-overflow on address 0x6020\nREAD of size 4\n    #0 0x1 in main food.cpp:9\nSUMMARY: AddressSanitizer: heap-buffer-overflow food.cpp:9 in main\n"})}),
	"running": func() hud.Pass {
		p := pass(
			hud.PassTest{Name: "1", In: "1\n", Want: "1\n", Done: true, Result: cost(judge.Result{Verdict: judge.AC, Stdout: "1\n"})},
			hud.PassTest{Name: "2", In: "2\n", Want: "4\n"})
		p.Running = true
		return p
	}(),
	"quiet": func() hud.Pass {
		p := fixtures0()
		p.Quiet = true
		return p
	}(),
	// A case of the player's own with no answer (:Case): it ran, and there
	// is nothing to compare.
	"own": pass(hud.PassTest{Name: "c1", In: "1\n100000000 0 0 100000000 100000000\n", NoWant: true, Done: true,
		Result: cost(judge.Result{Verdict: judge.AC, Unchecked: true, Stdout: "NO\n", Stderr: "need=100000000\n"})}),
}

func fixtures0() hud.Pass {
	return pass(hud.PassTest{Name: "1", In: "1\n", Want: "seven\n", Done: true,
		Result: judge.Result{Verdict: judge.WA, Stdout: "six\n", Diff: &judge.Diff{Line: 1, Want: "seven", Got: "six", GotLine: 1}}})
}

func view(p hud.Pass, w, h int) string {
	return New(func() (hud.Pass, bool) { return p, true }).View(w, h)
}

func TestGoldens(t *testing.T) {
	for name, p := range fixtures {
		for _, s := range sizes {
			golden(t, fmt.Sprintf("%s_%dx%d", name, s[0], s[1]), view(p, s[0], s[1]))
		}
	}
	golden(t, "closed_80x24", New(func() (hud.Pass, bool) { return hud.Pass{}, false }).View(80, 24))
}

// What the view must say in each state, whatever the goldens drift to.
func TestSays(t *testing.T) {
	for name, want := range map[string][]string{
		"nothing": {"✗ nothing printed", "got end of output", "#1 WA", "YES"},
		"wa":      {"1 warning", "12:9 unused variable 'k'", "✗ NO", "✗ YES", "stderr", "c=-1"},
		"ce":      {"CE", "variable 'i' is uninitialized"},
		"re":      {"heap-buffer-overflow", "SUMMARY: AddressSanitizer", "partial"},
		"running": {"running…", "#2 ·"},
		"quiet":   {"✗ fail", "pass or fail is all :w says"},
		"own":     {"#c1 ran", "(none: run only)", "NO", "need=100000000"},
	} {
		for _, s := range sizes {
			v := ansi.Strip(view(fixtures[name], s[0], s[1]))
			for _, w := range want {
				// The 80-column view may scroll stderr off the first page.
				if s[0] < Wide && (w == "stderr" || w == "c=-1") {
					continue
				}
				if !strings.Contains(v, w) {
					t.Errorf("%s at %dx%d lacks %q:\n%s", name, s[0], s[1], w, v)
				}
			}
		}
	}
	if v := ansi.Strip(view(fixtures["quiet"], 140, 40)); strings.Contains(v, "six") || strings.Contains(v, "seven") {
		t.Errorf("the quiet door leaks its outputs:\n%s", v)
	}
	if v := ansi.Strip(view(fixtures["wa"], 140, 40)); !strings.Contains(v, "input") || !strings.Contains(v, "│ expected") || !strings.Contains(v, "│ output") {
		t.Errorf("a wide view is not side by side:\n%s", v)
	}
}

// A program's output cannot move the cursor or recolour the screen.
func TestOutputIsInert(t *testing.T) {
	p := pass(hud.PassTest{Name: "1", In: "1\n", Want: "1\n", Done: true,
		Result: judge.Result{Verdict: judge.AC, Stdout: "\x1b[2J\x1b[31mred\tx\r\n"}})
	v := view(p, 140, 40)
	if strings.Contains(v, "\x1b[2J") || strings.Contains(v, "\x1b[31mred") {
		t.Errorf("raw escapes in the view: %q", v)
	}
	if !strings.Contains(ansi.Strip(v), `\x1b[2J\x1b[31mred  x`) {
		t.Errorf("escapes not shown:\n%s", ansi.Strip(v))
	}
}

func press(m *Model, keys ...string) tea.Cmd {
	var cmd tea.Cmd
	for _, k := range keys {
		msg := tea.KeyPressMsg{Text: k, Code: []rune(k)[0]}
		switch k {
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		}
		_, cmd = m.Update(msg)
	}
	return cmd
}

func TestKeys(t *testing.T) {
	p := fixtures["wa"]
	p.Tests[0].Result.Stderr = strings.Repeat("line\n", 100)
	m := New(func() (hud.Pass, bool) { return p, true })
	m.View(80, 24)
	if m.current(p) != 0 {
		t.Fatal("not on the first failure")
	}
	press(m, "l")
	if !strings.Contains(ansi.Strip(m.View(80, 24)), "#2 AC") || m.current(p) != 1 {
		t.Errorf("l did not show #2")
	}
	press(m, "l") // no #3: stays
	press(m, "1")
	if m.current(p) != 0 {
		t.Errorf("1 did not show #1")
	}
	press(m, "j", "j", "down")
	m.View(80, 24)
	if m.off != 3 {
		t.Errorf("scrolled to %d, want 3", m.off)
	}
	press(m, "G")
	m.View(80, 24)
	last := m.off
	press(m, "j")
	m.View(80, 24)
	if m.off != last || last == 0 {
		t.Errorf("scrolled past the end: %d then %d", last, m.off)
	}
	for _, k := range []string{"w", "q", "esc"} {
		cmd := press(m, k)
		if cmd == nil {
			t.Fatalf("%s did nothing", k)
		}
		if _, ok := cmd().(screens.Back); !ok {
			t.Errorf("%s did not go back", k)
		}
	}
}

func TestErr(t *testing.T) {
	p := hud.Pass{Name: "x", Err: errors.New("no compiler")}
	if v := ansi.Strip(view(p, 80, 24)); !strings.Contains(v, "no compiler") {
		t.Errorf("error not shown:\n%s", v)
	}
}
