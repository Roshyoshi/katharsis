// Package animtest holds what tests of animations share: goldens, and a
// frame spelled out as text a golden can hold.
package animtest

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden from the current output")

// Golden compares got with testdata/<name>.golden; go test -update
// writes it instead.
func Golden(t *testing.T, name, got string) {
	t.Helper()
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

var sgr = regexp.MustCompile("^\x1b\\[([0-9;]*)m")

// Grids spells a frame out as three grids, one character a cell: the
// glyphs, then the foreground and the background, "." for none and an
// ANSI color as a letter, krgybmcw for 0-7 and KRGYBMCW for 8-15. A
// blank's foreground, which does not show, is none; attributes such as
// bold or faint are dropped. Every glyph must be one cell wide.
func Grids(frame []string) string {
	const names = "krgybmcwKRGYBMCW"
	var glyphs, fgs, bgs []string
	for _, l := range frame {
		fg, bg := byte('.'), byte('.')
		var g, fr, br strings.Builder
		for len(l) > 0 {
			if m := sgr.FindStringSubmatch(l); m != nil {
				for _, p := range strings.Split(m[1], ";") {
					n, _ := strconv.Atoi(p)
					switch {
					case p == "" || n == 0:
						fg, bg = '.', '.'
					case n >= 30 && n <= 37:
						fg = names[n-30]
					case n >= 90 && n <= 97:
						fg = names[n-90+8]
					case n == 39:
						fg = '.'
					case n >= 40 && n <= 47:
						bg = names[n-40]
					case n >= 100 && n <= 107:
						bg = names[n-100+8]
					case n == 49:
						bg = '.'
					}
				}
				l = l[len(m[0]):]
				continue
			}
			r := []rune(l)[0]
			l = l[len(string(r)):]
			g.WriteRune(r)
			if r == ' ' {
				fr.WriteByte('.')
			} else {
				fr.WriteByte(fg)
			}
			br.WriteByte(bg)
		}
		glyphs, fgs, bgs = append(glyphs, g.String()), append(fgs, fr.String()), append(bgs, br.String())
	}
	return strings.Join(glyphs, "\n") + "\n-- fg --\n" + strings.Join(fgs, "\n") +
		"\n-- bg --\n" + strings.Join(bgs, "\n") + "\n"
}
