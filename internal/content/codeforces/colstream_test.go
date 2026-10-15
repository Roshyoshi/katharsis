package codeforces

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/klauspost/compress/snappy"
	"github.com/parquet-go/parquet-go"
)

type optPlain struct {
	V *string `parquet:"v,optional,plain"`
}

// optDelta is parquet-go's default byte-array encoding, which Arrow never
// writes.
type optDelta struct {
	V *string `parquet:"v,optional"`
}

type optDict struct {
	V *string `parquet:"v,optional,dict"`
}

type reqDict struct {
	V string `parquet:"v,dict"`
}

type listRow struct {
	T []dictExample `parquet:"t,list"`
}

// cell is one emitted value: its row, its place in the row's list, and
// the value ("<nil>" for a null).
type cell struct {
	row int64
	seq int
	v   string
}

func collect(t *testing.T, b []byte, rg int, path []string, want func(int64) bool) ([]cell, error) {
	t.Helper()
	f, err := parquet.OpenFile(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	var got []cell
	err = streamColumn(bytes.NewReader(b), f, rg, path, want, func(row int64, seq int, v []byte) error {
		s := string(v)
		if v == nil {
			s = "<nil>"
		}
		got = append(got, cell{row, seq, s})
		return nil
	})
	slices.SortFunc(got, func(a, b cell) int {
		if a.row != b.row {
			return int(a.row - b.row)
		}
		return a.seq - b.seq
	})
	return got, err
}

func TestStreamColumn(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(3, 4))
	words := make([]*string, 300)
	for i := range words {
		if i%7 == 3 {
			continue // null
		}
		s := fmt.Sprintf("value %d %s", i, strings.Repeat("x", rng.IntN(200)))
		if i%11 == 5 {
			s = "dup" // repeated values share a dictionary entry
		}
		words[i] = &s
	}
	every := func(int64) bool { return true }
	odd := func(r int64) bool { return r%2 == 1 }
	expect := func(want func(int64) bool) []cell {
		var out []cell
		for i, w := range words {
			if !want(int64(i)) {
				continue
			}
			v := "<nil>"
			if w != nil {
				v = *w
			}
			out = append(out, cell{int64(i), 0, v})
		}
		return out
	}
	small := parquet.PageBufferSize(512) // many pages
	tests := []struct {
		name string
		file []byte
		want func(int64) bool
	}{
		{"optional plain, many pages", writeParquet(t, mapRows(words, func(s *string) optPlain { return optPlain{s} }), append(arrowLike, small)...), every},
		{"optional dict, many pages", writeParquet(t, mapRows(words, func(s *string) optDict { return optDict{s} }), append(arrowLike, small)...), odd},
		{"optional dict, one page", writeParquet(t, mapRows(words, func(s *string) optDict { return optDict{s} }), arrowLike...), odd},
		{"uncompressed", writeParquet(t, mapRows(words, func(s *string) optDict { return optDict{s} }), parquet.DataPageVersion(1), small), every},
	}
	for _, tc := range tests {
		got, err := collect(t, tc.file, 0, []string{"v"}, tc.want)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if want := expect(tc.want); !slices.Equal(got, want) {
			t.Errorf("%s: got %d cells, want %d; first diff at %d", tc.name, len(got), len(want), firstDiff(got, want))
		}
	}

	// A required column has no definition levels.
	req := writeParquet(t, []reqDict{{"a"}, {"b"}, {"a"}}, arrowLike...)
	if got, err := collect(t, req, 0, []string{"v"}, every); err != nil || !slices.Equal(got, []cell{{0, 0, "a"}, {1, 0, "b"}, {2, 0, "a"}}) {
		t.Errorf("required: %v %v", got, err)
	}

	// Several row groups: rows count from each group's start.
	multi := writeParquet(t, mapRows(words[:40], func(s *string) optDict { return optDict{s} }), append(arrowLike, parquet.MaxRowsPerRowGroup(16))...)
	got, err := collect(t, multi, 1, []string{"v"}, func(r int64) bool { return r == 0 || r == 14 })
	if err != nil || len(got) != 2 || got[0] != (cell{0, 0, *words[16]}) || got[1] != (cell{14, 0, *words[30]}) {
		t.Errorf("row group 1: %v %v", got, err)
	}

	// parquet-go's defaults (v2 pages, delta lengths) are left to the
	// fallback.
	v2 := writeParquet(t, []reqDict{{"a"}})
	if _, err := collect(t, v2, 0, []string{"v"}, every); !errors.Is(err, errUnsupported) {
		t.Errorf("v2 pages: err = %v, want errUnsupported", err)
	}
	delta := writeParquet(t, []optDelta{{ptr("a")}}, arrowLike...)
	if _, err := collect(t, delta, 0, []string{"v"}, every); !errors.Is(err, errUnsupported) {
		t.Errorf("delta lengths: err = %v, want errUnsupported", err)
	}
}

func TestStreamColumnLists(t *testing.T) {
	t.Parallel()
	rows := []listRow{
		{T: []dictExample{{"1", "a"}, {"2", "b"}}},
		{T: nil},
		{T: []dictExample{}},
		{T: []dictExample{{"", "empty input"}, {"3", "c"}, {"1", "a again"}}},
		{T: []dictExample{{"4", "d"}}},
	}
	for _, opts := range [][]parquet.WriterOption{arrowLike, append(arrowLike, parquet.PageBufferSize(64))} {
		b := writeParquet(t, rows, opts...)
		got, err := collect(t, b, 0, []string{"t", "list", "element", "input"}, func(r int64) bool { return r != 0 })
		if err != nil {
			t.Fatal(err)
		}
		// parquet-go writes an empty optional string as a null.
		want := []cell{{3, 0, "<nil>"}, {3, 1, "3"}, {3, 2, "1"}, {4, 0, "4"}}
		if !slices.Equal(got, want) {
			t.Errorf("inputs = %v, want %v", got, want)
		}
		got, err = collect(t, b, 0, []string{"t", "list", "element", "output"}, func(r int64) bool { return r == 0 })
		if err != nil || !slices.Equal(got, []cell{{0, 0, "a"}, {0, 1, "b"}}) {
			t.Errorf("outputs = %v %v", got, err)
		}
	}
}

func mapRows[T any](in []*string, f func(*string) T) []T {
	out := make([]T, len(in))
	for i, s := range in {
		out[i] = f(s)
	}
	return out
}

func firstDiff(a, b []cell) int {
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}

// ---- snappy ------------------------------------------------------------------

// block assembles a raw Snappy block from elements, so every tag form can
// be tested whatever an encoder chooses to emit.
type block struct {
	out []byte
	n   int
}

func (b *block) literal(s string) {
	n := len(s) - 1
	switch {
	case n < 60:
		b.out = append(b.out, byte(n<<2))
	case n < 1<<8:
		b.out = append(b.out, 60<<2, byte(n))
	default:
		b.out = append(b.out, 61<<2, byte(n), byte(n>>8))
	}
	b.out = append(b.out, s...)
	b.n += len(s)
}

func (b *block) copy1(offset, length int) { // length 4-11, offset < 2048
	b.out = append(b.out, byte(1|(length-4)<<2|(offset>>8)<<5), byte(offset))
	b.n += length
}

func (b *block) copy2(offset, length int) { // length 1-64
	b.out = append(b.out, byte(2|(length-1)<<2), byte(offset), byte(offset>>8))
	b.n += length
}

func (b *block) copy4(offset, length int) {
	b.out = append(b.out, byte(3|(length-1)<<2))
	b.out = binary.LittleEndian.AppendUint32(b.out, uint32(offset))
	b.n += length
}

func (b *block) bytes() []byte {
	return append(binary.AppendUvarint(nil, uint64(b.n)), b.out...)
}

func decodeStream(t *testing.T, blk []byte, window int) ([]byte, error) {
	t.Helper()
	n, _ := binary.Uvarint(blk)
	r, err := newSnappyReader(bufio.NewReader(bytes.NewReader(blk)), int64(n))
	if err != nil {
		return nil, err
	}
	r.window = window
	var out bytes.Buffer
	_, err = io.CopyBuffer(&out, r, make([]byte, 7)) // odd reads cross elements
	return out.Bytes(), err
}

func TestSnappyReader(t *testing.T) {
	t.Parallel()
	var b block
	b.literal("abcd")
	b.copy1(4, 8)  // abcdabcd, overlapping its own output
	b.copy2(12, 3) // abc
	b.copy4(15, 2) // ab
	b.literal(strings.Repeat("z", 100))
	b.literal(strings.Repeat("y", 300))
	b.copy2(400, 64)
	want := "abcd" + "abcdabcd" + "abc" + "ab" + strings.Repeat("z", 100) + strings.Repeat("y", 300)
	want += want[len(want)-400 : len(want)-400+64]
	got, err := decodeStream(t, b.bytes(), 1<<20)
	if err != nil || string(got) != want {
		t.Fatalf("decoded %q, %v", got, err)
	}
	if dec, err := snappy.Decode(nil, b.bytes()); err != nil || string(dec) != want {
		t.Fatalf("the hand-built block is not valid Snappy: %v", err)
	}

	// The window: copies within it survive trimming, one past it is
	// refused rather than misread.
	var w block
	w.literal("0123456789")
	for range 200 {
		w.copy1(10, 10)
	}
	long := strings.Repeat("0123456789", 201)
	if got, err := decodeStream(t, w.bytes(), 32); err != nil || string(got) != long {
		t.Errorf("windowed decode: %d bytes, %v", len(got), err)
	}
	var far block
	far.literal(strings.Repeat("q", 100))
	far.literal(strings.Repeat("r", 100))
	far.copy2(150, 5)
	if _, err := decodeStream(t, far.bytes(), 64); !errors.Is(err, errUnsupported) {
		t.Errorf("copy past the window: err = %v", err)
	}

	// What an encoder emits round-trips.
	rng := rand.New(rand.NewPCG(5, 6))
	src := make([]byte, 3<<20)
	for i := range src {
		src[i] = "ab \n0123456789"[rng.IntN(14)]
	}
	if got, err := decodeStream(t, snappy.Encode(nil, src), snappyWindow); err != nil || !bytes.Equal(got, src) {
		t.Errorf("s2-encoded block: %v", err)
	}

	// Corrupt blocks are errors, not panics.
	for _, bad := range [][]byte{
		{5, 0x08, 'a'},                   // literal cut off
		{5, 0x00, 'a', 0x05},             // copy1 cut off
		{2, 0x05, 0x01},                  // copy before any output
		{1, 0x04, 'a', 'b'},              // literal longer than the block
		{0x80},                           // bad length varint
		{3, 0x00, 'a', 0x02, 0x00, 0x00}, // copy2 offset 0
	} {
		n, _ := binary.Uvarint(bad)
		r, err := newSnappyReader(bufio.NewReader(bytes.NewReader(bad)), int64(n))
		if err == nil {
			_, err = io.ReadAll(r)
		}
		if err == nil {
			t.Errorf("block %x decoded", bad)
		}
	}
}

func TestDecodeHybrid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		b     []byte
		width int
		n     int
		want  []uint32
		err   bool
	}{
		{"rle run", []byte{3 << 1, 1}, 1, 3, []uint32{1, 1, 1}, false},
		{"rle wide value", []byte{2 << 1, 0x34, 0x12}, 13, 2, []uint32{0x1234, 0x1234}, false},
		{"bit-packed width 1", []byte{1<<1 | 1, 0b10110001}, 1, 8, []uint32{1, 0, 0, 0, 1, 1, 0, 1}, false},
		// 0..7 at width 3: 000 001 010 011 100 101 110 111, LSB first.
		{"bit-packed width 3", []byte{1<<1 | 1, 0b10001000, 0b11000110, 0b11111010}, 3, 8, []uint32{0, 1, 2, 3, 4, 5, 6, 7}, false},
		{"padding dropped", []byte{1<<1 | 1, 0xff}, 1, 5, []uint32{1, 1, 1, 1, 1}, false},
		{"runs mixed", []byte{2 << 1, 0, 1<<1 | 1, 0xff}, 1, 4, []uint32{0, 0, 1, 1}, false},
		{"width 0", []byte{4 << 1}, 0, 4, []uint32{0, 0, 0, 0}, false},
		{"cut off", []byte{1<<1 | 1}, 3, 8, nil, true},
		{"too few", []byte{2 << 1, 1}, 1, 3, nil, true},
	}
	for _, tc := range tests {
		got, err := decodeHybrid(tc.b, tc.width, tc.n)
		if (err != nil) != tc.err || !tc.err && !slices.Equal(got, tc.want) {
			t.Errorf("%s: %v, %v; want %v", tc.name, got, err, tc.want)
		}
	}
}
