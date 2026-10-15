package anim

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// part is a piece of the Cheshire Cat, in the order it vanishes: "it
// vanished quite slowly, beginning with the end of the tail, and ending
// with the grin, which remained some time after the rest of it had gone"
// (Alice, ch. VI).
type part uint8

const (
	none part = iota
	tail
	ears
	head
	eyes
	whiskers
	grin
	numParts
)

// piece is some of the cat's glyphs: text at a row and column.
type piece struct {
	p        part
	row, col int
	text     string
}

// cat is the Cheshire Cat's head and tail, 35 cells by 9 rows, plain
// glyphs only.
var cat = []piece{
	{ears, 0, 5, `/\`}, {ears, 0, 24, `/\`},
	{ears, 1, 4, `/`}, {ears, 1, 7, `\`}, {ears, 1, 23, `/`}, {ears, 1, 26, `\`},
	{head, 1, 8, `_______________`},
	{head, 2, 3, `|`}, {head, 2, 27, `|`},
	{head, 3, 3, `|`}, {head, 3, 27, `|`},
	{head, 4, 3, `|`}, {head, 4, 27, `|`},
	{head, 5, 3, `|`}, {head, 5, 27, `|`},
	{head, 6, 3, `|`}, {head, 6, 27, `|`},
	{head, 7, 4, `\`}, {head, 7, 26, `/`},
	{head, 8, 5, `'-.._____________..-'`},
	{eyes, 3, 8, `(O)`}, {eyes, 3, 20, `(O)`},
	{whiskers, 4, 5, `==`}, {whiskers, 4, 15, `^`}, {whiskers, 4, 24, `==`},
	{grin, 5, 6, `\_________________/`},
	{grin, 6, 7, `\_|_|_|_|_|_|_|_/`},
	{tail, 1, 32, `_`},
	{tail, 2, 31, `( )`},
	{tail, 3, 32, `) )`},
	{tail, 4, 31, `( (`},
	{tail, 5, 32, `) )`},
	{tail, 6, 31, `( (`},
	{tail, 7, 29, `_.' /`},
	{tail, 8, 26, `--' .'`},
}

// catGrid is the cat drawn into rows, with the part each cell belongs to.
func catGrid() (rows []string, parts [][]part) {
	h, w := 0, 0
	for _, pc := range cat {
		h = max(h, pc.row+1)
		w = max(w, pc.col+len([]rune(pc.text)))
	}
	cells := make([][]rune, h)
	parts = make([][]part, h)
	for r := range cells {
		cells[r] = []rune(strings.Repeat(" ", w))
		parts[r] = make([]part, w)
	}
	for _, pc := range cat {
		for i, c := range []rune(pc.text) {
			cells[pc.row][pc.col+i] = c
			if c != ' ' {
				parts[pc.row][pc.col+i] = pc.p
			}
		}
	}
	rows = make([]string, h)
	for r := range cells {
		rows[r] = string(cells[r])
	}
	return rows, parts
}

// Cheshire fades the Cheshire Cat in or out over D, a part at a time.
// Vanishing goes tail first and the grin last; with KeepGrin the grin
// stays, which is what a core dump is: "a grin without a cat". Appearing
// goes the other way, grin first.
type Cheshire struct {
	Appear   bool
	KeepGrin bool
	D        time.Duration
	Style    lipgloss.Style
}

// order is the parts in the order they change.
func (c Cheshire) order() []part {
	var ps []part
	for p := tail; p < numParts; p++ {
		if p == grin && c.KeepGrin && !c.Appear {
			continue
		}
		ps = append(ps, p)
	}
	if c.Appear {
		for i, j := 0, len(ps)-1; i < j; i, j = i+1, j-1 {
			ps[i], ps[j] = ps[j], ps[i]
		}
	}
	return ps
}

// fades are the level changes per part: Normal, Faint, Dust, Gone.
const fades = 3

// window is when part p fades: an equal slot each, in order, except that
// the grin takes two slots, lingering as Carroll's did.
func (c Cheshire) window(p part) (from, to time.Duration, ok bool) {
	slots, at := 0, -1
	for _, q := range c.order() {
		if q == p {
			at = slots
		}
		slots++
		if q == grin {
			slots++
		}
	}
	if at < 0 {
		return 0, 0, false
	}
	n := 1
	if p == grin {
		n = 2
	}
	slot := c.D / time.Duration(slots)
	return slot * time.Duration(at), slot * time.Duration(at+n), true
}

// level is part p's level at t.
func (c Cheshire) level(p part, t time.Duration) Level {
	if p == none {
		return Gone
	}
	from, to, ok := c.window(p)
	if !ok { // a kept grin never fades
		return Normal
	}
	k := fades // steps done
	if t < to {
		k = 0
		if t >= from {
			k = int(int64(t-from) * fades / int64(to-from))
		}
	}
	if c.Appear {
		return Gone + Level(k)
	}
	return Normal - Level(k)
}

func (c Cheshire) Len() time.Duration { return c.D }

func (c Cheshire) Frame(t time.Duration) []string {
	rows, parts := catGrid()
	out := make([]string, len(rows))
	for r, row := range rows {
		out[r] = paint(row, func(i int) Level { return c.level(parts[r][i], t) }, c.Style, uint64(r)<<8)
	}
	return out
}

func (c Cheshire) Next(t time.Duration) time.Duration {
	next := Never
	for _, p := range c.order() {
		from, to, _ := c.window(p)
		for k := 1; k <= fades; k++ {
			at := from + time.Duration((int64(to-from)*int64(k)+fades-1)/fades)
			if at > t {
				next = min(next, at)
				break
			}
		}
	}
	return next
}
