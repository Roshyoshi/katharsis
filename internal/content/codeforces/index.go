package codeforces

import (
	"bytes"
	"cmp"
	"compress/flate"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"katharsis/internal/catalog"
)

// entry is one problem in the cached index: the catalog view plus what
// Fetch needs to find its text and tests.
type entry struct {
	catalog.Problem
	Key       string   `json:"key"` // dataset id, "1899/C"
	ContestID int      `json:"contest_id"`
	Index     string   `json:"index"` // "C", "F1"
	Name      string   `json:"contest_name"`
	Text      [2]int64 `json:"text"` // offset and length in text.bin

	Executable bool   `json:"executable"`  // ≥3 human solutions passed the official tests
	InputMode  string `json:"input_mode"`  // "stdio" or "file" (input.txt/output.txt)
	HasChecker bool   `json:"has_checker"` // the dataset generated a Python checker
	Multi      bool   `json:"multi,omitempty"`

	TestsetSize int    `json:"testset_size"`    // official tests on Codeforces
	Complete    bool   `json:"complete"`        // every official test is in the dataset
	GenTests    int    `json:"gen_tests"`       // generated tests for this problem
	GenBytes    int64  `json:"gen_bytes"`       // its contest's generated-tests file, 0 if none
	VFile       string `json:"vfile,omitempty"` // verifiable_tests file with its official tests
	VRow        int    `json:"vrow,omitempty"`  // row within VFile
	Skip        string `json:"skip,omitempty"`  // why it is not judgeable
}

type index struct {
	Key      string  `json:"key"`
	TextSize int64   `json:"text_size"` // text.bin must match, or rebuild
	TextHash string  `json:"text_hash"`
	Entries  []entry `json:"entries"`
}

// schema changes whenever entry, text.bin or the derivation rules change,
// so a stale index is rebuilt rather than misread. What Fetch writes into a
// problem dir has its own number (dirSchema), so a rule change does not
// throw away hundreds of MB of downloaded tests.
//
//	2: catalog.Problem.AnyCase
//	3: multi-answer problems need a checker, checker or not in the
//	   dataset; multiRE and caseRE rewritten; Russian-only statements
//	   skipped
//	4: a dataset checker is a float checker only if no phrase, loose ones
//	   in the description included, says the answer may vary
const schema = 4

func indexKey(l layout) string {
	b, _ := json.Marshal(l) // strings, ints and a string-keyed map cannot fail
	sum := sha256.Sum256(fmt.Appendf(b, "\n%d", maxGenFile))
	return fmt.Sprintf("rev=%s schema=%d layout=%x", rev, schema, sum[:6])
}

// maxGenFile caps the generated-tests file Fetch will download for one
// problem; problems in bigger contest files are not offered. The whole
// file is the unit of download (see generatedTests), so the cap is about
// download time, not memory: streamColumn keeps memory flat. Contest
// files at rev: median 32 MB, p90 174 MB, p99 456 MB, max 1.75 GB. At
// 256 MiB 94% of the problems that need generated tests stay playable
// (5361 of 5720); 512 MiB would keep 99% at up to twice the wait.
const maxGenFile = 256 << 20

const (
	defaultTime = 2 * time.Second
	defaultMem  = 256 << 20
)

// Skip reasons, in the order judgeable checks them.
const (
	skipNotExecutable = "not-executable" // no human solution passed its tests
	skipInteractive   = "interactive"
	skipChecker       = "needs-checker" // several valid answers
	skipFileIO        = "file-io"       // input.txt/output.txt; the judge only knows <stem>.in
	skipUnrated       = "unrated"
	skipJoke          = "april-fools"
	skipNoStatement   = "no-statement"
	skipRussian       = "russian"       // the statement is in Russian only
	skipNoTests       = "no-tests"      // official tests truncated, none generated
	skipTestsTooBig   = "tests-too-big" // generated tests over maxGenFile
	skipTestsMissing  = "tests-missing" // a fetch found the tests absent
)

// ---- building ----------------------------------------------------------------

// text is what Fetch renders from; it lives in text.bin (12 MB deflated)
// rather than the index, so Open parses 7 MB of JSON instead of 45.
type text struct {
	Description string    `json:"description,omitempty"`
	Input       string    `json:"input,omitempty"`
	Output      string    `json:"output,omitempty"`
	Interaction string    `json:"interaction,omitempty"`
	Note        string    `json:"note,omitempty"`
	Editorial   string    `json:"editorial,omitempty"`
	Examples    []example `json:"examples,omitempty"`
}

// split is the rows of one dataset split in table order.
type split struct {
	name string
	rows []tableRow
}

// buildIndex turns the problem table into entries and text.bin. Rows
// whose id does not parse are dropped: two at rev, 921/01 and 921/02,
// numbered rather than lettered (an ID like cf:92101 would be ambiguous)
// and not executable anyway. Entries come out in contest order, then
// problem index order.
func buildIndex(l layout, splits []split, gen map[int]int64) (*index, []byte, error) {
	var blob bytes.Buffer
	idx := &index{Key: indexKey(l)}
	for _, sp := range splits {
		// Position in the verifiable config, which is this split's
		// executable rows that have a full test set.
		vpos := map[int]int{}
		for i, r := range sp.rows {
			if r.Executable && (r.OfficialTestsComplete || r.GeneratedTests > 0) {
				vpos[i] = len(vpos)
			}
		}
		for i, r := range sp.rows {
			e, ok := newEntry(r)
			if !ok {
				continue
			}
			if p, ok := vpos[i]; ok && l.VTests[sp.name] > 0 {
				e.VFile, e.VRow = vtestFile(sp.name, p, len(vpos), l.VTests[sp.name])
			}
			e.GenBytes = gen[e.ContestID]
			off := int64(blob.Len())
			if err := appendText(&blob, r); err != nil {
				return nil, nil, err
			}
			e.Text = [2]int64{off, int64(blob.Len()) - off}
			e.judgeable(r.Description)
			idx.Entries = append(idx.Entries, e)
		}
	}
	slices.SortFunc(idx.Entries, func(a, b entry) int {
		return cmp.Or(cmp.Compare(a.ContestID, b.ContestID), indexCompare(a.Index, b.Index))
	})
	idx.TextSize, idx.TextHash = int64(blob.Len()), textHash(blob.Bytes())
	return idx, blob.Bytes(), nil
}

func textHash(blob []byte) string {
	sum := sha256.Sum256(blob)
	return hex.EncodeToString(sum[:8])
}

// indexCompare orders "B" < "C" < "C1" < "C2" < "D"; Codeforces indexes
// are a letter and an optional number.
func indexCompare(a, b string) int {
	la, na := splitIndex(a)
	lb, nb := splitIndex(b)
	return cmp.Or(cmp.Compare(la, lb), cmp.Compare(na, nb), cmp.Compare(a, b))
}

func splitIndex(s string) (string, int) {
	i := 0
	for i < len(s) && isLetter(s[i]) {
		i++
	}
	n, _ := strconv.Atoi(s[i:]) // 0 when there is no number
	return s[:i], n
}

func newEntry(r tableRow) (entry, bool) {
	contest, idxStr, ok := strings.Cut(r.ID, "/")
	cid, err := strconv.Atoi(contest)
	if !ok || err != nil || cid <= 0 || !indexRE.MatchString(idxStr) {
		return entry{}, false
	}
	e := entry{
		Key: r.ID, ContestID: cid, Index: idxStr, Name: r.ContestName,
		Executable:  r.Executable,
		InputMode:   r.InputMode,
		HasChecker:  strings.TrimSpace(r.GeneratedChecker) != "",
		TestsetSize: int(r.TestsetSize),
		Complete:    r.OfficialTestsComplete,
		GenTests:    int(r.GeneratedTests),
	}
	p := &e.Problem
	p.ID = catalog.NewID(catalog.CF, strconv.Itoa(cid)+idxStr)
	p.Source = catalog.CF
	p.Title = strings.TrimSpace(r.Title)
	p.URL = fmt.Sprintf("https://codeforces.com/contest/%d/problem/%s", cid, idxStr)
	p.Contest = contestLabel(r.ContestName, idxStr)
	p.Position = int(idxStr[0]-'A') + 1
	for _, t := range r.Tags {
		if t != "" && t != "*special" {
			p.Tags = append(p.Tags, t)
		}
	}
	p.Topic = topicFor(p.Tags)
	if r.Rating != nil && *r.Rating > 0 {
		p.Rating, p.RatingSource = int(*r.Rating), "cf"
	}
	p.TimeLimit = defaultTime
	if r.TimeLimit != nil && *r.TimeLimit > 0 {
		p.TimeLimit = time.Duration(math.Round(*r.TimeLimit*1000)) * time.Millisecond
	}
	p.MemLimit = defaultMem
	if r.MemoryLimit != nil && *r.MemoryLimit > 0 {
		p.MemLimit = int64(math.Round(*r.MemoryLimit * (1 << 20)))
	}
	p.Interactive = strings.TrimSpace(r.InteractionFormat) != "" || slices.Contains(r.Tags, "interactive")
	p.FloatTolerance = floatTolerance(r.OutputFormat, r.Description, r.Note)
	e.Multi = multiAnswer(r.OutputFormat, r.Description)
	p.AnyCase = caseInsensitive(r.OutputFormat, r.Note)
	// Several valid answers need a checker whether or not the dataset
	// wrote one: cf:233A has none, and its reference output is one
	// permutation of many, so another valid one was WA. A checker written
	// only to compare reals within the stated error is the judge's
	// built-in float checker; 240 of the 1780 checkers at rev are that.
	// Any other checker exists for a reason the statement does not spell
	// out, and is needed. A checker is taken for a float checker only when
	// no phrase, however loose, says the answer may vary: cf:306D asks to
	// "draw any such polygon" in its description, with a checker and a
	// tolerance. (No multi-answer statement at rev is explained by its
	// float tolerance alone: the 9 with both say "print any".)
	p.NeedsChecker = e.Multi ||
		e.HasChecker && (p.FloatTolerance == 0 || maybeMultiAnswer(r.OutputFormat, r.Description))
	p.HasEditorial = strings.TrimSpace(r.Editorial) != ""
	return e, true
}

func isLetter(c byte) bool { return c >= 'A' && c <= 'Z' }

// indexRE is a problem index: "C", "F1", "A12". It also makes the cache
// dir name safe, since it comes from downloaded data.
var indexRE = regexp.MustCompile(`^[A-Z][A-Z0-9]{0,3}$`)

// contestLabel is "Codeforces Round 909 (Div. 3) · C".
func contestLabel(name, idx string) string {
	if name = strings.TrimSpace(name); name == "" {
		return "Codeforces · " + idx
	}
	return name + " · " + idx
}

// judgeable sets Judgeable and Skip. Doors are picked by rating, so an
// unrated problem is not offered: the 257 at rev are mostly April Fools,
// Q#, Kotlin Heroes and VK Cup engine rounds, where no estimate would be
// honest. Rated April Fools problems are riddles, not algorithms, and are
// left out too. So are statements in Russian only (VK Cup and Technocup
// mirrors such as 646A and 1769B1; 20 were judgeable at rev): the door
// would be unreadable.
func (e *entry) judgeable(description string) {
	reason := ""
	switch {
	case !e.Executable:
		reason = skipNotExecutable
	case e.Interactive:
		reason = skipInteractive
	case e.NeedsChecker:
		reason = skipChecker
	case e.InputMode == "file":
		reason = skipFileIO
	case e.Rating == 0:
		reason = skipUnrated
	case strings.Contains(strings.ToLower(e.Name), "april fools"):
		reason = skipJoke
	case strings.TrimSpace(description) == "":
		reason = skipNoStatement
	case russian(description):
		reason = skipRussian
	case e.Complete && e.VFile != "":
	case e.GenTests == 0 || e.GenBytes == 0:
		reason = skipNoTests
	case e.GenBytes > maxGenFile:
		reason = skipTestsTooBig
	}
	e.Skip, e.Judgeable = reason, reason == ""
}

// appendText appends one deflated JSON record. Each record is compressed
// alone so Fetch can inflate one without reading the rest.
func appendText(w *bytes.Buffer, r tableRow) error {
	t := text{
		Description: r.Description, Input: r.InputFormat, Output: r.OutputFormat,
		Interaction: r.InteractionFormat, Note: r.Note, Editorial: r.Editorial,
		Examples: r.Examples,
	}
	zw, err := flate.NewWriter(w, flate.BestCompression)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(zw).Encode(t); err != nil {
		return err
	}
	return zw.Close()
}

func (c *Catalog) readText(e entry) (text, error) {
	var t text
	f, err := os.Open(filepath.Join(c.root, "text.bin"))
	if err != nil {
		return t, err
	}
	defer func() { _ = f.Close() }()
	zr := flate.NewReader(io.NewSectionReader(f, e.Text[0], e.Text[1]))
	if err := json.NewDecoder(zr).Decode(&t); err != nil {
		return t, fmt.Errorf("%s: text.bin record: %w", e.ID, err)
	}
	return t, nil
}

// ---- persistence -------------------------------------------------------------

// loadIndex returns the cached index if it matches this build and its
// text.bin is the one it was written with.
func loadIndex(root string, l layout) (*index, bool) {
	b, err := os.ReadFile(filepath.Join(root, "index.json"))
	if err != nil {
		return nil, false
	}
	var idx index
	if json.Unmarshal(b, &idx) != nil || idx.Key != indexKey(l) || len(idx.Entries) == 0 {
		return nil, false
	}
	blob, err := os.ReadFile(filepath.Join(root, "text.bin"))
	if err != nil || int64(len(blob)) != idx.TextSize {
		return nil, false
	}
	if textHash(blob) != idx.TextHash {
		return nil, false
	}
	return &idx, true
}

// saveIndex writes text.bin, then index.json. Each is renamed into place,
// and index.json carries text.bin's size and hash, so a reader never pairs
// an index with another build's text.
func saveIndex(root string, idx *index, blob []byte) error {
	if err := writeAtomic(filepath.Join(root, "text.bin"), blob); err != nil {
		return err
	}
	b, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(root, "index.json"), b)
}

// writeAtomic writes via a temp file in the same dir and a rename, so a
// reader (or a crash) never sees half a file.
func writeAtomic(p string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".tmp-"+filepath.Base(p)+"-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }() // no-op after the rename
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), p)
}
