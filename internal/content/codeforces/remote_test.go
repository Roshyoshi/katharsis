package codeforces

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/parquet-go/parquet-go"
)

type wideRow struct {
	A string `parquet:"a"`
	B string `parquet:"b"`
	C string `parquet:"c"`
}

// openRemote reads only the footer and the wanted columns, and copes with
// a footer bigger than its first guess.
func TestOpenRemote(t *testing.T) {
	t.Parallel()
	var rows []wideRow
	for i := range 2000 {
		rows = append(rows, wideRow{A: "a" + strings.Repeat("x", i%50), B: strings.Repeat("b", 1500), C: "c"})
	}
	tests := []struct {
		name    string
		opts    []parquet.WriterOption
		maxReqs int
		bigFoot bool
	}{
		{"small footer", nil, 3, false},
		// Key-value metadata lands in the footer: 100 KB of it outgrows
		// the 64 KiB tail read.
		{"big footer", []parquet.WriterOption{parquet.KeyValueMetadata("pad", strings.Repeat("p", 100<<10))}, 4, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := writeParquet(t, rows, append([]parquet.WriterOption{parquet.Compression(&parquet.Uncompressed)}, tc.opts...)...)
			fx := &fixture{files: map[string][]byte{"data/w.parquet": b}}
			h := newFakeHF(t, fx)
			c := &Catalog{base: h.srv.URL, client: testClient(h)}
			f, _, err := c.openRemote(t.Context(), c.fileURL("data/w.parquet"), func(p string) bool { return p != "b" })
			if err != nil {
				t.Fatal(err)
			}
			if n := h.count("data/w.parquet"); n > tc.maxReqs || tc.bigFoot && n < 3 {
				t.Errorf("%d requests", n)
			}
			// Column b (3 MB) was never fetched, beyond whatever of its end
			// the 64 KiB tail guess covers.
			span := columnSpan(t, b, "b")
			for _, rng := range h.rangesOf("data/w.parquet") {
				if strings.HasPrefix(rng, "bytes=-") {
					continue
				}
				off, end := parseRange(t, rng, int64(len(b)))
				if off < span[1] && span[0] < end {
					t.Errorf("fetched %s, inside column b [%d, %d)", rng, span[0], span[1])
				}
			}
			type ac struct {
				A string `parquet:"a"`
				C string `parquet:"c"`
			}
			got, err := readAll[ac](f)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(rows) || got[1999].A != rows[1999].A || got[5].C != "c" {
				t.Errorf("rows read back wrong: %d", len(got))
			}
			// Reading b fails loudly instead of fetching.
			if _, err := readAll[wideRow](f); !errors.Is(err, errUnfetched) {
				t.Errorf("reading an unfetched column: %v", err)
			}
		})
	}
}

func TestOpenRemoteRejects(t *testing.T) {
	t.Parallel()
	fx := &fixture{files: map[string][]byte{
		"data/junk.parquet": bytes.Repeat([]byte("j"), 100),
		"data/foot.parquet": append(append([]byte("PAR1"), bytes.Repeat([]byte{0}, 20)...), 0xff, 0xff, 0xff, 0x7f, 'P', 'A', 'R', '1'),
	}}
	h := newFakeHF(t, fx)
	c := &Catalog{base: h.srv.URL, client: testClient(h)}
	for _, p := range []string{"data/junk.parquet", "data/foot.parquet", "data/missing.parquet"} {
		if _, _, err := c.openRemote(t.Context(), c.fileURL(p), func(string) bool { return true }); err == nil {
			t.Errorf("%s opened", p)
		}
	}
}

func TestSpanFile(t *testing.T) {
	t.Parallel()
	sf := &spanFile{size: 20}
	sf.add(10, []byte("klmno"))
	sf.add(0, []byte("abcde"))
	sf.add(5, []byte("fghij")) // adjacent spans read through
	tests := []struct {
		off  int64
		n    int
		want string
		err  error
	}{
		{0, 5, "abcde", nil},
		{3, 9, "defghijkl", nil},
		{14, 1, "o", nil},
		{14, 3, "o", errUnfetched},
		{16, 2, "", errUnfetched},
		{20, 1, "", io.EOF},
	}
	for _, tc := range tests {
		p := make([]byte, tc.n)
		n, err := sf.ReadAt(p, tc.off)
		if string(p[:n]) != tc.want || !errors.Is(err, tc.err) && !(err == nil && tc.err == nil) {
			t.Errorf("ReadAt(%d, %d) = %q, %v; want %q, %v", tc.off, tc.n, p[:n], err, tc.want, tc.err)
		}
	}
}
