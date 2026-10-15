package vgs

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestToneTables(t *testing.T) {
	for k, tab := range tones {
		for n, w := range tab {
			if len(w) != int(w[0])+1 || w[0] < 2 {
				t.Errorf("tone %d note %d: period %d, %d entries", k, n, w[0], len(w))
			}
			for _, v := range w[1:] {
				if v < -128 || v > 127 {
					t.Fatalf("tone %d note %d: sample %d", k, n, v)
				}
			}
		}
		// Higher notes have shorter periods.
		for n := 1; n < len(tab); n++ {
			if tab[n][0] > tab[n-1][0] {
				t.Errorf("tone %d: note %d's period %d is longer than note %d's %d", k, n, tab[n][0], n-1, tab[n-1][0])
			}
		}
	}
}

func compile(t *testing.T, src string) *Song {
	t.Helper()
	s, err := Compile([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCompileTiming(t *testing.T) {
	// T120: a whole note is 4 beats of 0.5 s, 44100 samples at 22050 Hz.
	s := compile(t, "Ch0 t120 l4 c\n")
	if s.Frames() != 11025 || s.Loops() {
		t.Fatalf("one quarter note: %d frames, loops %v", s.Frames(), s.Loops())
	}
	// The default gate holds 80% of the note: 8820 on, 2205 off.
	var waits []uint32
	for _, n := range s.notes {
		if n.typ == tWait {
			waits = append(waits, n.val)
		}
	}
	if !slices.Equal(waits, []uint32{8820, 2205, 0}) {
		t.Errorf("waits %v", waits)
	}
	if s := compile(t, "Ch0 t120 c4.\n"); s.Frames() != 11025+5512 {
		t.Errorf("dotted quarter: %d frames", s.Frames())
	}
	if s := compile(t, "Ch0 t120 c4^8 r2\n"); s.Frames() != 11025+5512+22050 {
		t.Errorf("tie and rest: %d frames", s.Frames())
	}
	// Channels play at once; the song is as long as the longest.
	if s := compile(t, "Ch0 t120 c1\nCh1 c2\n"); s.Frames() != 44100 {
		t.Errorf("two channels: %d frames", s.Frames())
	}
}

func TestCompileLoopAndMacros(t *testing.T) {
	s := compile(t, "$Lead @2 v12 o5\nCh0 t120 (Lead) c | d e\nCh1 r4 | r2\n")
	if !s.Loops() || s.IntroFrames() != 11025 || s.Frames() != 33075 {
		t.Fatalf("loops %v, intro %d, frames %d", s.Loops(), s.IntroFrames(), s.Frames())
	}
	var on []note
	for _, n := range s.notes {
		if n.typ == tKeyOn {
			on = append(on, n)
		}
	}
	// A key is octave*12 from A: o5's c is 60-9, its d two above.
	if len(on) != 3 || on[0].op2 != 2 || on[0].op3 != 51 || on[1].op3 != 53 {
		t.Errorf("key-ons %+v", on)
	}
	if s := compile(t, "Ch0 c d\n"); s.Loops() {
		t.Error("a song without | loops")
	}
}

func TestCompileErrors(t *testing.T) {
	for src, want := range map[string]string{
		"Ch0 c\nfoo\n":         "line 2: unknown statement",
		"Ch0 (Nope)\n":         "line 1: undeclared variable",
		"Ch0 c0\n":             "zero divide",
		"Ch7 c\n":              "invalid channel number",
		"$A c\n$A d\n":         "line 2: duplicated variable",
		"Ch0 (A\n":             "no ')'",
		"Ch0 x\n":              "unknown operand",
		"$A (A)\nCh0 (A)\n":    "without end",
		"Ch0 %0\n":             "% operand",
		"$bad-name c\nCh0 c\n": "invalid macro name",
	} {
		_, err := Compile([]byte(src))
		var e *Error
		if !errors.As(err, &e) || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", src, err, want)
		}
	}
	// Comments, CRLF and tabs are what the C compiler allows.
	compile(t, "# a song\r\nCh0\tt150 c # and a note\r\n")
}

func render(d *Decoder, n, chunk int) []int16 {
	out := make([]int16, 0, n)
	buf := make([]int16, chunk)
	for len(out) < n {
		k := min(chunk, n-len(out))
		d.Render(buf[:k])
		out = append(out, buf[:k]...)
	}
	return out
}

func TestRenderLoopsAndChunks(t *testing.T) {
	s := compile(t, "Ch0 t240 @2 v12 o4 c d | e f g\nCh1 @0 v8 o3 \\S200 \\E400 c2 | P-30 g2 r4\n")
	d := NewDecoder(s)
	n := int(s.Frames()) * 3
	a := render(d, n, 441)
	b := render(NewDecoder(s), n, 1000)
	if !slices.Equal(a, b) {
		t.Fatal("441- and 1000-sample calls render differently")
	}
	if d.Ended() {
		t.Fatal("a looping song ended")
	}
	// The second time through the loop sounds like the first, once
	// the notes that rang over the seam have died out.
	loop := int(s.Frames() - s.IntroFrames())
	from := int(s.Frames()) + loop/2
	if !slices.Equal(a[from:from+loop/2], a[from+loop:from+loop+loop/2]) {
		t.Error("the loop does not repeat")
	}
	var loud bool
	for _, v := range a {
		loud = loud || v > 1000 || v < -1000
	}
	if !loud {
		t.Error("rendered silence")
	}
}

func TestRenderEnds(t *testing.T) {
	s := compile(t, "Ch0 t120 c8\n")
	d := NewDecoder(s)
	buf := make([]int16, 10000)
	if got := d.Render(buf); got != int(s.Frames()) || !d.Ended() {
		t.Fatalf("rendered %d of %d, ended %v", got, s.Frames(), d.Ended())
	}
	for _, v := range buf[s.Frames():] {
		if v != 0 {
			t.Fatal("sound after the end")
		}
	}
	if d.Render(buf) != 0 {
		t.Error("rendered after the end")
	}
	d.Reset()
	if d.Ended() || d.Render(buf[:10]) != 10 {
		t.Error("Reset did not rewind")
	}
}

// TestConformance compares the port with the C compiler and decoder. It
// needs the reference output, so it runs only when VGS_REF names a file of
// lines "<mml path> <loop time> <time length> <loop index> <samples>
// <fnv64 hex> <peak>", as testdata/ref.c prints them; relative paths
// are from the file's directory.
func TestConformance(t *testing.T) {
	ref := os.Getenv("VGS_REF")
	if ref == "" {
		t.Skip("VGS_REF is not set")
	}
	f, err := os.Open(ref)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	songs := 0
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) != 7 {
			t.Fatalf("bad line %q", sc.Text())
		}
		path := fs[0]
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(ref), path)
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		s, err := Compile(src)
		if err != nil {
			t.Errorf("%s: %v", fs[0], err)
			continue
		}
		timeI, _ := strconv.ParseInt(fs[1], 10, 64)
		timeL, _ := strconv.ParseInt(fs[2], 10, 64)
		n, _ := strconv.Atoi(fs[4])
		if s.Frames() != timeL {
			t.Errorf("%s: %d frames, C counts %d", fs[0], s.Frames(), timeL)
		}
		if s.Loops() && s.IntroFrames() != timeI {
			t.Errorf("%s: loop at %d, C at %d", fs[0], s.IntroFrames(), timeI)
		}
		h := uint64(14695981039346656037)
		for _, v := range render(NewDecoder(s), n, 441) {
			h = (h ^ uint64(uint16(v)&0xff)) * 1099511628211
			h = (h ^ uint64(uint16(v)>>8)) * 1099511628211
		}
		if got := strconv.FormatUint(h, 16); strings.TrimLeft(fs[5], "0") != got {
			t.Errorf("%s: samples differ from C (fnv %s, C %s)", fs[0], got, fs[5])
			continue
		}
		songs++
	}
	t.Logf("%d songs match the C decoder sample for sample", songs)
}
