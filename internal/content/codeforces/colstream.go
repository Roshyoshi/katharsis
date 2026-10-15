package codeforces

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/bits"

	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/encoding/thrift"
	"github.com/parquet-go/parquet-go/format"
)

// A contest's generated inputs are one dictionary page: Arrow checks the
// dictionary size every 1024 rows and contests have fewer, so every input
// lands in the dictionary, a single Snappy block of up to 1.5 GB. parquet-
// go decodes a dictionary whole, about 11x the file size in memory (287
// MiB for a 25 MB contest file, measured), which a door prefetch inside
// the game cannot afford. streamColumn reads one byte-array column page
// by page instead: it walks the data pages to learn which dictionary
// entries the wanted rows use, then streams the dictionary through a
// windowed Snappy decoder and keeps only those entries. Memory is one
// value plus the window.
//
// It handles what Arrow writes: v1 data pages, RLE levels, PLAIN or
// dictionary values, Snappy or no compression. Anything else returns
// errUnsupported and the caller falls back to parquet-go.

var errUnsupported = errors.New("parquet layout the streaming reader does not handle")

// snappyWindow is how far back a Snappy copy may reach. Arrow's C++
// Snappy compresses in 64 KiB fragments and never copies across one;
// Go's s2 (parquet-go's writer, used by the tests) reaches further, so
// the window is generous.
const snappyWindow = 4 << 20

// streamColumn calls emit(row, seq, value) for every value of the
// byte-array leaf column path in row group rg whose row (counted within
// the row group) is wanted. seq numbers a row's values for a repeated
// column (official_tests.list.element.input) and is 0 otherwise. A null
// value is emitted as nil; values come in no particular row order.
func streamColumn(ra io.ReaderAt, f *parquet.File, rg int, path []string, wanted func(row int64) bool, emit func(row int64, seq int, v []byte) error) error {
	leaf, ok := f.Schema().Lookup(path...)
	if !ok {
		return fmt.Errorf("no column %v", path)
	}
	md := f.Metadata()
	if rg >= len(md.RowGroups) || leaf.ColumnIndex >= len(md.RowGroups[rg].Columns) {
		return fmt.Errorf("no row group %d", rg)
	}
	cc := md.RowGroups[rg].Columns[leaf.ColumnIndex].MetaData
	if cc.Type != format.ByteArray || leaf.MaxRepetitionLevel > 1 {
		return errUnsupported
	}
	slotDef, err := slotLevel(f.Schema(), path)
	if err != nil {
		return err
	}
	s := &colStream{
		ra: ra, codec: cc.Codec,
		maxDef: leaf.MaxDefinitionLevel, maxRep: leaf.MaxRepetitionLevel, slotDef: slotDef,
		wanted: wanted, emit: emit, need: map[uint32][]slot{},
	}
	off := cc.DataPageOffset
	if d := cc.DictionaryPageOffset; d > 0 && d < off {
		off = d
	}
	end := off + cc.TotalCompressedSize
	for off < end {
		h, hlen, err := readPageHeader(ra, off)
		if err != nil {
			return err
		}
		body := off + hlen
		switch h.Type {
		case format.DictionaryPage:
			dh := h.DictionaryPageHeader.V
			if dh.Encoding != format.Plain && dh.Encoding != format.PlainDictionary {
				return errUnsupported
			}
			s.dict = &page{off: body, csize: h.CompressedPageSize, usize: h.UncompressedPageSize, n: int(dh.NumValues)}
		case format.DataPage:
			if err := s.dataPage(h, body); err != nil {
				return err
			}
		case format.IndexPage:
		default:
			return errUnsupported // v2 data pages
		}
		off = body + int64(h.CompressedPageSize)
	}
	return s.flushDict()
}

// slotLevel is the definition level at which an entry of path is a list
// element (or, for a column with no repeated ancestor, any entry at all):
// below it the row has an empty or null list there, so no value.
func slotLevel(s *parquet.Schema, path []string) (int, error) {
	var node parquet.Node = s
	def, slot := 0, 0
	for _, name := range path {
		var next parquet.Node
		for _, fl := range node.Fields() {
			if fl.Name() == name {
				next = fl
			}
		}
		if next == nil {
			return 0, fmt.Errorf("no column %v", path)
		}
		if next.Optional() || next.Repeated() {
			def++
		}
		if next.Repeated() {
			slot = def
		}
		node = next
	}
	return slot, nil
}

type page struct {
	off          int64
	csize, usize int32
	n            int
}

type slot struct {
	row int64
	seq int
}

type colStream struct {
	ra                      io.ReaderAt
	codec                   format.CompressionCodec
	maxDef, maxRep, slotDef int
	wanted                  func(int64) bool
	emit                    func(int64, int, []byte) error

	row  int64 // current row; the first entry starts row 0
	seq  int
	dict *page
	need map[uint32][]slot // dictionary index -> where its value goes
}

func (s *colStream) dataPage(h format.PageHeader, body int64) error {
	dh := h.DataPageHeader.V
	n := int(dh.NumValues)
	r, err := pageReader(s.ra, body, h.CompressedPageSize, h.UncompressedPageSize, s.codec)
	if err != nil {
		return err
	}
	var reps, defs []uint32
	if s.maxRep > 0 {
		if dh.RepetitionLevelEncoding != format.RLE {
			return errUnsupported
		}
		if reps, err = readLevels(r, s.maxRep, n); err != nil {
			return err
		}
	}
	if s.maxDef > 0 {
		if dh.DefinitionLevelEncoding != format.RLE {
			return errUnsupported
		}
		if defs, err = readLevels(r, s.maxDef, n); err != nil {
			return err
		}
	}
	var dictIdx []uint32
	switch dh.Encoding {
	case format.Plain:
	case format.RLEDictionary, format.PlainDictionary:
		nonNull := n
		if s.maxDef > 0 {
			nonNull = 0
			for _, d := range defs {
				if int(d) == s.maxDef {
					nonNull++
				}
			}
		}
		rest, err := readCapped(r, 1<<30, "dictionary indexes")
		if err != nil {
			return err
		}
		if nonNull > 0 {
			if len(rest) == 0 || rest[0] > 32 {
				return fmt.Errorf("dictionary index bit width")
			}
			if dictIdx, err = decodeHybrid(rest[1:], int(rest[0]), nonNull); err != nil {
				return err
			}
		}
	default:
		return errUnsupported
	}
	k := 0 // index among non-null values
	for i := range n {
		if s.maxRep == 0 || reps[i] == 0 {
			s.row++
			s.seq = 0
		}
		def := s.maxDef
		if s.maxDef > 0 {
			def = int(defs[i])
		}
		isSlot, isValue := def >= s.slotDef, def == s.maxDef
		want := isSlot && s.wanted(s.row-1)
		switch {
		case want && !isValue:
			if err := s.emit(s.row-1, s.seq, nil); err != nil {
				return err
			}
		case isValue && dictIdx != nil:
			if want {
				s.need[dictIdx[k]] = append(s.need[dictIdx[k]], slot{s.row - 1, s.seq})
			}
		case isValue:
			v, err := plainValue(r, want)
			if err != nil {
				return err
			}
			if want {
				if err := s.emit(s.row-1, s.seq, v); err != nil {
					return err
				}
			}
		}
		if isValue {
			k++
		}
		if isSlot {
			s.seq++
		}
	}
	return nil
}

// flushDict streams the dictionary page and emits the entries the data
// pages asked for, stopping after the last one.
func (s *colStream) flushDict() error {
	if len(s.need) == 0 {
		return nil
	}
	if s.dict == nil {
		return fmt.Errorf("dictionary-encoded values without a dictionary page")
	}
	var last uint32
	for i := range s.need {
		last = max(last, i)
	}
	if int(last) >= s.dict.n {
		return fmt.Errorf("dictionary index %d of %d", last, s.dict.n)
	}
	r, err := pageReader(s.ra, s.dict.off, s.dict.csize, s.dict.usize, s.codec)
	if err != nil {
		return err
	}
	for i := uint32(0); i <= last; i++ {
		slots, ok := s.need[i]
		v, err := plainValue(r, ok)
		if err != nil {
			return err
		}
		for _, sl := range slots {
			if err := s.emit(sl.row, sl.seq, v); err != nil {
				return err
			}
		}
	}
	return nil
}

// plainValue reads one PLAIN byte array: a 4-byte length and the bytes,
// returned only if keep, skipped otherwise. A non-nil empty slice is an
// empty value, distinct from a null.
func plainValue(r io.Reader, keep bool) ([]byte, error) {
	var lb [4]byte
	if _, err := io.ReadFull(r, lb[:]); err != nil {
		return nil, fmt.Errorf("value length: %w", err)
	}
	n := int64(binary.LittleEndian.Uint32(lb[:]))
	if !keep {
		_, err := io.CopyN(io.Discard, r, n)
		return nil, err
	}
	if n > maxTestFile {
		return nil, permanentError{fmt.Errorf("a %d byte value, over the %d cap", n, maxTestFile)}
	}
	v := make([]byte, n)
	_, err := io.ReadFull(r, v)
	return v, err
}

// readPageHeader decodes the thrift page header at off and returns its
// length. Headers carry min/max statistics, so they are usually small but
// not always; a short first read is retried larger.
func readPageHeader(ra io.ReaderAt, off int64) (format.PageHeader, int64, error) {
	for _, size := range []int{64 << 10, 4 << 20} {
		buf := make([]byte, size)
		n, err := ra.ReadAt(buf, off)
		if err != nil && !errors.Is(err, io.EOF) && n == 0 {
			return format.PageHeader{}, 0, err
		}
		buf = buf[:n]
		var h format.PageHeader
		br := bytes.NewReader(buf)
		if err := thrift.NewDecoder(new(thrift.CompactProtocol).NewReader(br)).Decode(&h); err != nil {
			if n == size {
				continue // maybe cut off; read more
			}
			return format.PageHeader{}, 0, fmt.Errorf("page header at %d: %w", off, err)
		}
		if h.CompressedPageSize < 0 || h.UncompressedPageSize < 0 {
			return format.PageHeader{}, 0, fmt.Errorf("page header at %d: negative size", off)
		}
		return h, int64(n - br.Len()), nil
	}
	return format.PageHeader{}, 0, fmt.Errorf("page header at %d over 4 MiB", off)
}

// pageReader streams a page body, decompressing on the fly.
func pageReader(ra io.ReaderAt, off int64, csize, usize int32, codec format.CompressionCodec) (io.Reader, error) {
	br := bufio.NewReaderSize(io.NewSectionReader(ra, off, int64(csize)), 64<<10)
	switch codec {
	case format.Uncompressed:
		return br, nil
	case format.Snappy:
		return newSnappyReader(br, int64(usize))
	}
	return nil, errUnsupported
}

// readLevels reads RLE-encoded levels of a v1 data page: a 4-byte length,
// then the hybrid encoding at the bit width of max.
func readLevels(r io.Reader, max, n int) ([]uint32, error) {
	var lb [4]byte
	if _, err := io.ReadFull(r, lb[:]); err != nil {
		return nil, fmt.Errorf("levels length: %w", err)
	}
	l := binary.LittleEndian.Uint32(lb[:])
	if l > 1<<30 {
		return nil, fmt.Errorf("levels of %d bytes", l)
	}
	b := make([]byte, l)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, fmt.Errorf("levels: %w", err)
	}
	lv, err := decodeHybrid(b, bits.Len(uint(max)), n)
	if err != nil {
		return nil, err
	}
	for _, v := range lv {
		if int(v) > max {
			return nil, fmt.Errorf("level %d over the maximum %d", v, max)
		}
	}
	return lv, nil
}

// decodeHybrid decodes n values of Parquet's RLE/bit-packing hybrid: runs
// of a uvarint header, then either one repeated value (header even,
// count = header/2) or groups of 8 bit-packed values, LSB first (header
// odd, groups = header/2).
func decodeHybrid(b []byte, width, n int) ([]uint32, error) {
	if width < 0 || width > 32 {
		return nil, fmt.Errorf("bit width %d", width)
	}
	out := make([]uint32, 0, n)
	for len(out) < n {
		h, k := binary.Uvarint(b)
		if k <= 0 {
			return nil, fmt.Errorf("hybrid run header: %d of %d values", len(out), n)
		}
		b = b[k:]
		if h&1 == 0 {
			count := h >> 1
			vb := (width + 7) / 8
			if len(b) < vb {
				return nil, fmt.Errorf("hybrid RLE value cut off")
			}
			var v uint32
			for i := range vb {
				v |= uint32(b[i]) << (8 * i)
			}
			b = b[vb:]
			for c := uint64(0); c < count && len(out) < n; c++ {
				out = append(out, v)
			}
			continue
		}
		groups := int(h >> 1)
		nb := groups * width
		if groups <= 0 || len(b) < nb {
			return nil, fmt.Errorf("hybrid bit-packed run cut off")
		}
		var acc uint64
		var have int
		p := 0
		for range groups * 8 {
			for have < width {
				acc |= uint64(b[p]) << have
				p++
				have += 8
			}
			v := uint32(acc & (1<<width - 1))
			acc >>= width
			have -= width
			if len(out) < n {
				out = append(out, v)
			}
		}
		b = b[nb:]
	}
	return out, nil
}

// ---- snappy ------------------------------------------------------------------

// snappyReader decodes one raw Snappy block as a stream, keeping only the
// last snappyWindow bytes of output for back-references. A copy reaching
// further returns errUnsupported.
type snappyReader struct {
	r       *bufio.Reader
	remain  int64  // output bytes not yet decoded
	literal int    // bytes left of a literal decoded in pieces
	window  int    // history kept for copies
	buf     []byte // history, then unread output
	rd      int    // read position in buf
}

func newSnappyReader(r *bufio.Reader, usize int64) (*snappyReader, error) {
	n, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, fmt.Errorf("snappy length: %w", err)
	}
	if int64(n) != usize {
		return nil, fmt.Errorf("snappy block of %d bytes in a %d byte page", n, usize)
	}
	return &snappyReader{r: r, remain: int64(n), window: snappyWindow}, nil
}

func (s *snappyReader) Read(p []byte) (int, error) {
	for s.rd == len(s.buf) {
		if s.remain == 0 {
			return 0, io.EOF
		}
		if err := s.element(); err != nil {
			return 0, err
		}
	}
	n := copy(p, s.buf[s.rd:])
	s.rd += n
	return n, nil
}

// element decodes one literal or copy. It runs only when every decoded
// byte has been read, so the history can be trimmed to the window first.
func (s *snappyReader) element() error {
	if w := s.window; len(s.buf) > 2*w {
		copy(s.buf, s.buf[len(s.buf)-w:])
		s.buf = s.buf[:w]
		s.rd = w
	}
	if s.literal > 0 {
		return s.readLiteral()
	}
	tag, err := s.r.ReadByte()
	if err != nil {
		return unexpected(err)
	}
	var length, offset int
	switch tag & 3 {
	case 0: // literal
		length = int(tag >> 2)
		if length >= 60 {
			nb := length - 59
			var lb [4]byte
			if _, err := io.ReadFull(s.r, lb[:nb]); err != nil {
				return unexpected(err)
			}
			length = int(binary.LittleEndian.Uint32(lb[:]))
		}
		if int64(length)+1 > s.remain {
			return fmt.Errorf("snappy literal past the end")
		}
		s.literal = length + 1
		return s.readLiteral()
	case 1:
		b, err := s.r.ReadByte()
		if err != nil {
			return unexpected(err)
		}
		length = 4 + int(tag>>2)&7
		offset = int(tag&0xe0)<<3 | int(b)
	case 2:
		var ob [2]byte
		if _, err := io.ReadFull(s.r, ob[:]); err != nil {
			return unexpected(err)
		}
		length = 1 + int(tag>>2)
		offset = int(binary.LittleEndian.Uint16(ob[:]))
	case 3:
		var ob [4]byte
		if _, err := io.ReadFull(s.r, ob[:]); err != nil {
			return unexpected(err)
		}
		length = 1 + int(tag>>2)
		offset = int(binary.LittleEndian.Uint32(ob[:]))
	}
	if offset <= 0 {
		return fmt.Errorf("snappy copy offset %d", offset)
	}
	if offset > len(s.buf) {
		return errUnsupported // reaches past the window (or the start)
	}
	if int64(length) > s.remain {
		return fmt.Errorf("snappy copy past the end")
	}
	// Copies may overlap their own output ("abab..."), so byte by byte.
	from := len(s.buf) - offset
	for i := range length {
		s.buf = append(s.buf, s.buf[from+i])
	}
	s.remain -= int64(length)
	return nil
}

func unexpected(err error) error {
	if errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return err
}

// readLiteral copies up to 1 MiB of the current literal, so an
// incompressible block (one literal as long as the page) never sits in
// memory whole.
func (s *snappyReader) readLiteral() error {
	n := min(s.literal, 1<<20)
	start := len(s.buf)
	s.buf = append(s.buf, make([]byte, n)...)
	if _, err := io.ReadFull(s.r, s.buf[start:]); err != nil {
		return unexpected(err)
	}
	s.literal -= n
	s.remain -= int64(n)
	return nil
}
