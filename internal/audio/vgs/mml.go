package vgs

import (
	"bytes"
	"fmt"
)

// The compiler follows vgsmml.c statement by statement, quirks included,
// because the songs were tuned against it: a channel's tracked volume
// starts at 0 although its first VOL note says 10, a default length
// keeps its own dot, and a note's gate is cut by its % rate with integer
// division. Where C would divide by zero, read past a buffer or loop
// forever, this returns an error instead.

// tempoSpeed is mml_tempo_speed, the C compiler's global tempo scale in
// percent.
const tempoSpeed = 100

// Error is a compile error at a 1-based line (0: not tied to a line).
type Error struct {
	Line int
	Msg  string
}

func (e *Error) Error() string {
	if e.Line == 0 {
		return "mml: " + e.Msg
	}
	return fmt.Sprintf("mml: line %d: %s", e.Line, e.Msg)
}

type channel struct {
	notes []note
	oct   int // octave
	len   int // default length
	huten int // default length is dotted
	per   int // gate: percent of a note's length held before key-off
	tone  int
	vol   int
}

type compiler struct {
	ch     [6]channel
	tempo  int // samples per whole note
	macros map[string]string
	kbase  [6]int // K+ / K- transpose
}

// Compile compiles VGS MML.
func Compile(src []byte) (*Song, error) {
	// The C compiler stops at a NUL, cuts each line at '#' (comments) or
	// '\r', and reads tabs as spaces.
	if i := bytes.IndexByte(src, 0); i >= 0 {
		src = src[:i]
	}
	lines := bytes.Split(src, []byte{'\n'})
	c := &compiler{tempo: (Rate * 60) / 120 * 4, macros: map[string]string{}}
	c.ch[0].notes = append(c.ch[0].notes, note{typ: tMVol, val: 10})
	for i := range c.ch {
		ch := &c.ch[i]
		ch.oct, ch.len, ch.per = 4, 4, 80
		ch.notes = append(ch.notes,
			note{typ: tEnv1, val: 1000},
			note{typ: tEnv2, val: 10000},
			note{typ: tVol, val: 10})
	}
	for i, raw := range lines {
		if j := bytes.IndexAny(raw, "#\r"); j >= 0 {
			raw = raw[:j]
		}
		line := trim(string(bytes.ReplaceAll(raw, []byte{'\t'}, []byte{' '})))
		if err := c.statement(line); err != nil {
			err.Line = i + 1
			return nil, err
		}
	}
	for i := range c.ch {
		c.ch[i].notes = append(c.ch[i].notes, note{typ: tWait})
	}
	merged := c.ch[0].notes
	for i := 1; i < 6; i++ {
		merged = merge(merged, c.ch[i].notes)
	}
	// The first label after merging is the loop point; the first end
	// after it becomes the jump back.
	for i, n := range merged {
		if n.typ != tLabel {
			continue
		}
		for j := i; j < len(merged); j++ {
			if merged[j].typ == tWait && merged[j].val == 0 {
				merged[j] = note{typ: tJump, val: uint32(i)}
				break
			}
		}
		break
	}
	if len(merged) >= maxNotes {
		return nil, &Error{Msg: fmt.Sprintf("%d notes; the decoder holds %d", len(merged), maxNotes-1)}
	}
	return newSong(merged), nil
}

// trim drops leading and trailing spaces (trimstring).
func trim(s string) string {
	i, j := 0, len(s)
	for i < j && s[i] == ' ' {
		i++
	}
	for j > i && s[j-1] == ' ' {
		j--
	}
	return s[i:j]
}

func isDigit(b byte) bool { return '0' <= b && b <= '9' }
func isAlpha(b byte) bool { u := b &^ 0x20; return 'A' <= u && u <= 'Z' }

func (c *compiler) statement(line string) *Error {
	if line == "" {
		return nil
	}
	switch line[0] {
	case '$':
		sp := bytes.IndexByte([]byte(line), ' ')
		if sp < 0 {
			return &Error{Msg: "invalid macro syntax"}
		}
		name, value := line[1:sp], trim(line[sp+1:])
		if name == "" {
			return &Error{Msg: "invalid macro name"}
		}
		for k := range len(name) {
			if !isAlpha(name[k]) && !isDigit(name[k]) {
				return &Error{Msg: "invalid macro name"}
			}
		}
		if _, dup := c.macros[name]; dup {
			return &Error{Msg: "duplicated variable name"}
		}
		c.macros[name] = value
		return nil
	case 'c', 'C':
		if len(line) < 3 || line[1]|0x20 != 'h' || !isDigit(line[2]) {
			return &Error{Msg: "unknown statement"}
		}
		body := line[3:]
		if len(body) > 0 && body[0] == ' ' {
			body = body[1:]
		}
		return c.channel(int(line[2]-'0'), body)
	}
	return &Error{Msg: "unknown statement"}
}

// cursor walks a channel line as the C code walks its string: at(i) past
// the end reads the terminating NUL.
type cursor struct {
	b []byte
	p int
}

func (r *cursor) at(off int) byte {
	if i := r.p + off; i >= 0 && i < len(r.b) {
		return r.b[i]
	}
	return 0
}

// val is getval: the number at the cursor, leaving the cursor on its last
// digit; -1 and no move when there is none.
func (r *cursor) val() int {
	n, q := 0, r.p
	for q < len(r.b) && isDigit(r.b[q]) {
		n = n*10 + int(r.b[q]-'0')
		if n > 1<<30 {
			n = 1 << 30 // C's int overflows here; any sane song stops far below
		}
		q++
	}
	if q == r.p {
		return -1
	}
	r.p = q - 1
	return n
}

// length reads a note's or rest's length and dot after the cursor's
// letter (or '^'), falling back to the channel default.
func (c *compiler) length(r *cursor, ch *channel) (int, bool) {
	r.p++
	i := r.val()
	if i < 0 {
		r.p--
		return ch.len, ch.huten != 0
	}
	if r.at(1) == '.' {
		r.p++
		return i, true
	}
	return i, false
}

func (c *compiler) duration(i int, dot bool) (int, *Error) {
	if i == 0 {
		return 0, &Error{Msg: "zero divide error"}
	}
	k := c.tempo / i
	if dot {
		k += c.tempo / (i * 2)
	}
	return k, nil
}

func (c *compiler) channel(cn int, fmtStr string) *Error {
	if cn > 5 {
		return &Error{Msg: "invalid channel number"}
	}
	ch := &c.ch[cn]
	buf := trim(fmtStr)
	// Macros: "(Name)" becomes its value, until none is left.
	for guard := 0; ; guard++ {
		open := bytes.IndexByte([]byte(buf), '(')
		if open < 0 {
			break
		}
		if guard > 10000 {
			return &Error{Msg: "macros expand without end"}
		}
		end := bytes.IndexByte([]byte(buf[open+1:]), ')')
		if end < 0 {
			return &Error{Msg: "there is no ')' corresponding to '('"}
		}
		end += open + 1
		v, ok := c.macros[buf[open+1:end]]
		if !ok {
			return &Error{Msg: "undeclared variable was specified"}
		}
		buf = buf[:open] + v + buf[end+1:]
	}
	b := bytes.ReplaceAll([]byte(trim(buf)), []byte{' '}, nil)
	for i, x := range b {
		if 'a' <= x && x <= 'z' {
			b[i] = x - ('a' - 'A')
		}
	}
	add := func(n note) { ch.notes = append(ch.notes, n) }
	r := &cursor{b: b}
	for ; r.p < len(b); r.p++ {
		switch x := b[r.p]; {
		case 'A' <= x && x <= 'G':
			n := ch.oct * 12
			n += [...]int{0, 2, -9, -7, -5, -4, -2}[x-'A']
			for r.at(1) == '+' || r.at(1) == '-' {
				if r.at(1) == '+' {
					n++
				} else {
					n--
				}
				r.p++
			}
			n += c.kbase[cn]
			n = max(0, min(n, 84))
			k, err := c.duration(c.length(r, ch))
			if err != nil {
				return err
			}
			for r.at(1) == '^' {
				r.p++ // onto the '^'; length steps past it
				d, err := c.duration(c.length(r, ch))
				if err != nil {
					return err
				}
				k += d
			}
			l := k * ch.per / 100
			m := k - l
			if m == 0 {
				m++
				l--
			}
			add(note{typ: tKeyOn, op1: uint8(cn), op2: uint8(ch.tone), op3: uint8(n)})
			add(note{typ: tWait, val: uint32(l)})
			add(note{typ: tKeyOff, op1: uint8(cn)})
			add(note{typ: tWait, val: uint32(m)})
		case x == 'R':
			r.p++
			i := r.val()
			dot := false
			switch {
			case i == 0:
				return &Error{Msg: "zero divide error"}
			case i < 0:
				r.p--
				i, dot = ch.len, ch.huten != 0
			case r.at(1) == '.':
				r.p++
				dot = true
			}
			k, err := c.duration(i, dot)
			if err != nil {
				return err
			}
			add(note{typ: tWait, val: uint32(k)})
		case x == 'P':
			r.p++
			if r.at(0) != '-' {
				return &Error{Msg: "invalid value was specified to the P operand"}
			}
			r.p++
			i := r.val()
			if i < 0 {
				return &Error{Msg: "invalid value was specified to the P operand"}
			}
			add(note{typ: tPDown, op1: uint8(cn), val: uint32(i)})
		case x == '\\':
			r.p++
			op := r.at(0)
			if op != 'S' && op != 'E' {
				return &Error{Msg: "unknown operand was specified"}
			}
			r.p++
			i := r.val()
			if i < 1 {
				return &Error{Msg: fmt.Sprintf("invalid value was specified to the \\%c operand", op)}
			}
			t := uint8(tEnv1)
			if op == 'E' {
				t = tEnv2
			}
			add(note{typ: t, op1: uint8(cn), val: uint32(i)})
		case x == '%':
			r.p++
			i := r.val()
			if i < 1 || i > 100 {
				return &Error{Msg: "invalid value was specified to the % operand"}
			}
			ch.per = i
		case x == '@':
			r.p++
			i := r.val()
			if i < 0 {
				return &Error{Msg: "invalid value was specified to the @ operand"}
			}
			ch.tone = i
		case x == 'T':
			r.p++
			i := r.val()
			if i < 1 || i*tempoSpeed/100 < 1 {
				return &Error{Msg: "invalid value was specified to the T operand"}
			}
			c.tempo = (Rate * 60) / (i * tempoSpeed / 100) * 4
		case x == 'L':
			r.p++
			i := r.val()
			if i < 1 {
				return &Error{Msg: "invalid value was specified to the L operand"}
			}
			ch.len = i
			ch.huten = 0
			if r.at(1) == '.' {
				r.p++
				ch.huten = 1
			}
		case x == 'O':
			r.p++
			i := r.val()
			if i < 0 {
				return &Error{Msg: "invalid value was specified to the O operand"}
			}
			ch.oct = i
		case x == '>':
			ch.oct++
		case x == '<':
			ch.oct--
		case x == 'V':
			r.p++
			if r.at(0) == '+' || r.at(0) == '-' {
				for r.at(0) == '+' || r.at(0) == '-' {
					if r.at(0) == '+' {
						ch.vol++
					} else {
						ch.vol--
					}
					r.p++
				}
				r.p--
				add(note{typ: tVol, op1: uint8(cn), val: uint32(ch.vol)})
			} else {
				i := r.val()
				if i < 0 {
					return &Error{Msg: "invalid value was specified to the V operand"}
				}
				add(note{typ: tVol, op1: uint8(cn), val: uint32(i)})
				ch.vol = i
			}
		case x == 'M':
			r.p++
			i := r.val()
			if i < 0 {
				return &Error{Msg: "invalid value was specified to the M operand"}
			}
			add(note{typ: tMVol, val: uint32(i)})
		case x == '|':
			add(note{typ: tLabel})
		case x == 'K':
			r.p++
			if r.at(0) != '+' && r.at(0) != '-' {
				return &Error{Msg: "invalid value was specified to the K operand"}
			}
			for r.at(0) == '+' || r.at(0) == '-' {
				if r.at(0) == '+' {
					c.kbase[cn]++
				} else {
					c.kbase[cn]--
				}
				r.p++
			}
			r.p--
		default:
			return &Error{Msg: "unknown operand was specified"}
		}
	}
	return nil
}

// merge interleaves two channels' note lists by time (merge in
// vgsmml.c): each side's events up to its next wait go out, then the
// shorter wait, with the longer one shortened by it. Both lists end with
// a zero wait; the result ends with b's.
func merge(a, b []note) []note {
	out := make([]note, 0, len(a)+len(b))
	i, j := 0, 0
	for {
		for a[i].typ != tWait {
			out = append(out, a[i])
			i++
		}
		for b[j].typ != tWait {
			out = append(out, b[j])
			j++
		}
		if a[i].val == 0 || b[j].val == 0 {
			break
		}
		switch {
		case a[i].val < b[j].val:
			b[j].val -= a[i].val
			out = append(out, a[i])
			i++
		case a[i].val > b[j].val:
			a[i].val -= b[j].val
			out = append(out, b[j])
			j++
		default:
			out = append(out, a[i])
			i++
			j++
		}
	}
	for a[i].typ != tWait || a[i].val != 0 {
		out = append(out, a[i])
		i++
	}
	for b[j].typ != tWait || b[j].val != 0 {
		out = append(out, b[j])
		j++
	}
	return append(out, b[j])
}
