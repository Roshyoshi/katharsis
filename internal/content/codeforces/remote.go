package codeforces

import (
	"cmp"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/parquet-go/parquet-go"
)

// The dataset's problem table is 2.7 GB, and 99% of that is one column:
// official_tests.output (complete outputs of small inputs, some of them
// many MB). Everything else, statements and editorials included, is about
// 17 MB. Parquet stores each column of a row group contiguously and the
// footer says where, so a reader that fetches the footer and then only
// the byte ranges of the columns it wants downloads the 17 MB in a few
// requests: about 3 per shard.

const (
	tailGuess = 64 << 10 // one request usually covers the footer
	// mergeGap joins two wanted ranges when the bytes between them are
	// fewer than this: a little extra data is cheaper than a request.
	mergeGap = 1 << 20
	// maxSpan caps one ranged read and maxSpans all of them for one file;
	// they only stop a broken footer from asking for gigabytes.
	maxSpan  = 512 << 20
	maxSpans = 1 << 30
)

// spanFile is an io.ReaderAt over the parts of a remote file that have been
// fetched. A read outside them is an error, so asking parquet-go for a
// column that was not downloaded fails loudly instead of fetching.
type spanFile struct {
	size  int64
	spans []span // sorted by off; a column range may overlap the tail
}

type span struct {
	off int64
	b   []byte
}

var errUnfetched = errors.New("read outside the fetched byte ranges")

func (f *spanFile) add(off int64, b []byte) {
	f.spans = append(f.spans, span{off, b})
	slices.SortFunc(f.spans, func(a, b span) int { return cmp.Compare(a.off, b.off) })
}

func (f *spanFile) ReadAt(p []byte, off int64) (int, error) {
	if off >= f.size {
		return 0, io.EOF
	}
	n := 0
	for n < len(p) {
		pos := off + int64(n)
		i, ok := slices.BinarySearchFunc(f.spans, pos, func(s span, pos int64) int {
			switch {
			case s.off+int64(len(s.b)) <= pos:
				return -1
			case s.off > pos:
				return 1
			}
			return 0
		})
		if !ok {
			if pos >= f.size {
				return n, io.EOF
			}
			return n, fmt.Errorf("%w: [%d, %d)", errUnfetched, pos, off+int64(len(p)))
		}
		s := f.spans[i]
		n += copy(p[n:], s.b[pos-s.off:])
	}
	return n, nil
}

// openRemote reads the footer of the parquet file at rawURL, then the
// column chunks for which want(path) is true, and opens the result.
// Column paths are dotted, e.g. "official_tests.list.element.output".
func (c *Catalog) openRemote(ctx context.Context, rawURL string, want func(path string) bool) (*parquet.File, *spanFile, error) {
	tail, size, err := c.client.Tail(ctx, rawURL, tailGuess)
	if err != nil {
		return nil, nil, err
	}
	if len(tail) < 12 || string(tail[len(tail)-4:]) != "PAR1" {
		return nil, nil, fmt.Errorf("%s: not a parquet file", rawURL)
	}
	sf := &spanFile{size: size}
	sf.add(size-int64(len(tail)), tail)
	footer := int64(binary.LittleEndian.Uint32(tail[len(tail)-8:])) + 8
	if footer > size-4 {
		return nil, nil, fmt.Errorf("%s: footer of %d bytes in a %d byte file", rawURL, footer, size)
	}
	if footer > int64(len(tail)) {
		// A footer bigger than the guess: fetch the rest of it.
		off := size - footer
		b, err := c.client.Range(ctx, rawURL, off, footer-int64(len(tail)))
		if err != nil {
			return nil, nil, err
		}
		sf.add(off, b)
	}
	f, err := parquet.OpenFile(sf, sf.size,
		parquet.SkipMagicBytes(true), // the header is 4 bytes nobody needs
		parquet.SkipPageIndex(true),
		parquet.SkipBloomFilters(true),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", rawURL, err)
	}

	ranges, err := columnRanges(f, want)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", rawURL, err)
	}
	for _, r := range ranges {
		if covered(sf, r) {
			continue // inside the tail already
		}
		b, err := c.client.Range(ctx, rawURL, r.off, r.end-r.off)
		if err != nil {
			return nil, nil, err
		}
		sf.add(r.off, b)
	}
	return f, sf, nil
}

type byteRange struct{ off, end int64 }

// columnRanges lists the byte ranges of the wanted column chunks in every
// row group, merged when close together.
func columnRanges(f *parquet.File, want func(string) bool) ([]byteRange, error) {
	var rs []byteRange
	for _, rg := range f.Metadata().RowGroups {
		for _, cc := range rg.Columns {
			m := cc.MetaData
			if !want(strings.Join(m.PathInSchema, ".")) {
				continue
			}
			off := chunkStart(m)
			end := off + m.TotalCompressedSize
			if off < 4 || m.TotalCompressedSize < 0 || end > f.Size() {
				return nil, fmt.Errorf("column %v at [%d, %d) outside the file", m.PathInSchema, off, end)
			}
			rs = append(rs, byteRange{off, end})
		}
	}
	slices.SortFunc(rs, func(a, b byteRange) int { return cmp.Compare(a.off, b.off) })
	var merged []byteRange
	for _, r := range rs {
		if n := len(merged); n > 0 && r.off-merged[n-1].end < mergeGap && max(r.end, merged[n-1].end)-merged[n-1].off <= maxSpan {
			merged[n-1].end = max(merged[n-1].end, r.end)
			continue
		}
		merged = append(merged, r)
	}
	var total int64
	for _, r := range merged {
		if r.end-r.off > maxSpan {
			return nil, fmt.Errorf("column chunk of %d bytes is over the %d cap", r.end-r.off, maxSpan)
		}
		total += r.end - r.off
	}
	if total > maxSpans {
		return nil, fmt.Errorf("wanted columns total %d bytes, over the %d cap", total, maxSpans)
	}
	return merged, nil
}

func covered(sf *spanFile, r byteRange) bool {
	for _, s := range sf.spans {
		if s.off <= r.off && r.end <= s.off+int64(len(s.b)) {
			return true
		}
	}
	return false
}

// readAll reads every row of f into T, which names only fetched columns.
func readAll[T any](f *parquet.File) ([]T, error) {
	rows := make([]T, f.NumRows())
	r := parquet.NewGenericReader[T](f)
	defer func() { _ = r.Close() }()
	n, err := r.Read(rows)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if int64(n) != f.NumRows() {
		return nil, fmt.Errorf("read %d of %d rows", n, f.NumRows())
	}
	return rows, nil
}
