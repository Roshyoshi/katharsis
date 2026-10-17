package cses

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/content/web"
	"katharsis/internal/problem"
)

const (
	// The caps only stop a broken or hostile response from filling the
	// disk; CSES's largest test sets are tens of MB.
	maxZip       = 1 << 30
	maxTestFile  = 512 << 20
	maxTestTotal = 2 << 30
)

// renderVersion changes whenever what Fetch writes into a statement dir
// changes, so a dir an older build wrote is fetched again.
const renderVersion = 1

// statementMeta is statement/meta.json, written last.
type statementMeta struct {
	Render  int       `json:"render"`
	Facts   facts     `json:"facts"`
	Samples int       `json:"samples"`
	Fetched time.Time `json:"fetched"`
}

// testsDone is tests/done.json, written last.
type testsDone struct {
	Tests   int       `json:"tests"`
	Fetched time.Time `json:"fetched"`
}

// Fetch materializes id under the cache: the statement and samples from
// the public task page, and for a Judgeable task the tests, downloaded on
// first need with the account's session and kept for good. A task that is
// not Judgeable gets no tests and no error. Concurrent calls for one id
// share one download. When the session has expired Fetch logs in again
// with the stored password; when it cannot, the error is ErrLoginNeeded.
func (c *Catalog) Fetch(ctx context.Context, id catalog.ID) (catalog.Assets, error) {
	if _, ok := c.entry(id); !ok {
		return catalog.Assets{}, errors.New(string(id) + ": not a CSES task in the catalog")
	}
	return c.flights.Do(ctx, id, func() (catalog.Assets, error) { return c.fetch(ctx, id) })
}

// fetch runs as the only flight for id, so its cache checks cannot miss a
// download of the same task finishing a moment earlier.
func (c *Catalog) fetch(ctx context.Context, id catalog.ID) (catalog.Assets, error) {
	native := id.Native()
	dir := filepath.Join(c.root, native)
	a, m, ok := loadStatement(filepath.Join(dir, "statement"))
	if !ok {
		var err error
		if a, m, err = c.fetchStatement(ctx, id, dir); err != nil {
			return catalog.Assets{}, err
		}
	}
	if err := c.verify(native, m.Facts); err != nil {
		return catalog.Assets{}, err
	}
	p, _ := c.Get(id)
	if !p.Judgeable {
		return a, nil
	}
	tests, ok := loadTests(filepath.Join(dir, "tests"))
	if !ok {
		var err error
		if tests, err = c.fetchTests(ctx, id, dir); err != nil {
			return catalog.Assets{}, fmt.Errorf("%s tests: %w", id, err)
		}
	}
	a.Tests = tests
	return a, nil
}

func loadStatement(dir string) (catalog.Assets, statementMeta, bool) {
	var m statementMeta
	b, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil || json.Unmarshal(b, &m) != nil || m.Render != renderVersion {
		return catalog.Assets{}, m, false
	}
	a := catalog.Assets{Statement: filepath.Join(dir, "statement.md")}
	if a.Samples, err = problem.LoadTests(filepath.Join(dir, "samples")); err != nil || len(a.Samples) != m.Samples {
		return catalog.Assets{}, m, false
	}
	return a, m, true
}

func loadTests(dir string) ([]problem.Test, bool) {
	var d testsDone
	b, err := os.ReadFile(filepath.Join(dir, "done.json"))
	if err != nil || json.Unmarshal(b, &d) != nil {
		return nil, false
	}
	tests, err := problem.LoadTests(dir)
	if err != nil || len(tests) != d.Tests || d.Tests == 0 {
		return nil, false
	}
	return tests, true
}

// fetchStatement downloads the task page and publishes <dir>/statement.
func (c *Catalog) fetchStatement(ctx context.Context, id catalog.ID, dir string) (catalog.Assets, statementMeta, error) {
	native := id.Native()
	resp, err := c.pace.do(ctx, request{url: c.base + "/problemset/task/" + native})
	if err != nil {
		return catalog.Assets{}, statementMeta{}, fmt.Errorf("%s statement: %w", id, err)
	}
	if resp.status != http.StatusOK {
		return catalog.Assets{}, statementMeta{}, fmt.Errorf("%s statement: status %d", id, resp.status)
	}
	st, err := parseTask(resp.body)
	if err != nil {
		return catalog.Assets{}, statementMeta{}, fmt.Errorf("%s statement: %w", id, err)
	}
	e, _ := c.entry(id)
	if e.Contest == "" {
		return catalog.Assets{}, statementMeta{}, errors.New(string(id) + ": not in the catalog")
	}
	section := strings.TrimPrefix(e.Contest, "CSES Problem Set, ")
	if section == interactiveSection {
		st.Interactive = true
	}
	m := statementMeta{Render: renderVersion, Facts: st.facts, Samples: len(st.Samples), Fetched: time.Now().UTC()}
	files := map[string][]byte{
		"task.html":    resp.body,
		"statement.md": []byte(render(st, section, taskURL(native))),
	}
	for i, s := range st.Samples {
		n := strconv.Itoa(i + 1)
		files["samples/"+n+".in"] = []byte(s.In)
		files["samples/"+n+".out"] = []byte(s.Out)
	}
	mb, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return catalog.Assets{}, m, err
	}
	files["meta.json"] = mb
	sdir := filepath.Join(dir, "statement")
	if err := c.publish(sdir, func(tmp string) error {
		if err := os.MkdirAll(filepath.Join(tmp, "samples"), 0o755); err != nil {
			return err
		}
		for name, b := range files {
			if err := os.WriteFile(filepath.Join(tmp, filepath.FromSlash(name)), b, 0o644); err != nil {
				return err
			}
		}
		return nil
	}, func() bool { _, _, ok := loadStatement(sdir); return ok }); err != nil {
		return catalog.Assets{}, m, err
	}
	a, m, ok := loadStatement(sdir)
	if !ok {
		return catalog.Assets{}, m, fmt.Errorf("%s: cache dir %s unreadable after fetch", id, sdir)
	}
	return a, m, nil
}

// publish builds dst in a temp dir under the cache root with fill, syncs
// it and renames it into place. valid says whether a dst already there is
// a finished copy: one that is not is debris and is replaced; one that is
// (another process published first) is kept.
func (c *Catalog) publish(dst string, fill func(tmp string) error, valid func() bool) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(c.root, web.TempPrefix+filepath.Base(filepath.Dir(dst))+"-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := fill(tmp); err != nil {
		return err
	}
	if err := web.SyncDir(tmp); err != nil {
		return err
	}
	if !valid() {
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, dst); err != nil {
		if valid() {
			return nil
		}
		return err
	}
	return nil
}

// errExpired means CSES answered as to a logged-out user.
var errExpired = errors.New("CSES session expired")

// fetchTests downloads id's test zip with the account's session and
// publishes <dir>/tests. An expired session is renewed once (relogin).
func (c *Catalog) fetchTests(ctx context.Context, id catalog.ID, dir string) ([]problem.Test, error) {
	tdir := filepath.Join(dir, "tests")
	for attempt := 0; ; attempt++ {
		jar, gen, ok := c.session()
		if !ok {
			return nil, ErrLoginNeeded
		}
		var n int
		err := c.publish(tdir, func(tmp string) error {
			var err error
			n, err = c.downloadTests(ctx, jar, id.Native(), tmp)
			if err != nil {
				return err
			}
			b, err := json.Marshal(testsDone{Tests: n, Fetched: time.Now().UTC()})
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(tmp, "done.json"), b, 0o644)
		}, func() bool { _, ok := loadTests(tdir); return ok })
		if errors.Is(err, errExpired) && attempt == 0 {
			if err := c.relogin(ctx, gen); err != nil {
				return nil, err
			}
			continue
		}
		if errors.Is(err, errExpired) {
			return nil, fmt.Errorf("CSES refused the tests after logging in again (%w)", err)
		}
		if err != nil {
			return nil, err
		}
		tests, ok := loadTests(tdir)
		if !ok {
			return nil, fmt.Errorf("cache dir %s unreadable after fetch", tdir)
		}
		c.mu.Lock()
		c.hasTests[id] = true
		c.mu.Unlock()
		return tests, nil
	}
}

// downloadTests fetches /problemset/tests/<id>/ and posts its form back,
// as the page's download button does: csrf_token plus download=true,
// answered with a zip of N.in and N.out (the shape frederickpek/cses's
// scripts/load.py uses). CSES answers a logged-out user with a 404 for
// the page, and a page instead of a zip is checked for the same, so an
// expired session is reported as errExpired and nothing is cached.
func (c *Catalog) downloadTests(ctx context.Context, jar http.CookieJar, native, dst string) (int, error) {
	u := c.base + "/problemset/tests/" + native + "/"
	resp, err := c.pace.do(ctx, request{url: u, jar: jar})
	if err != nil {
		return 0, err
	}
	switch {
	case resp.status == http.StatusNotFound:
		return 0, errExpired
	case resp.status != http.StatusOK:
		return 0, fmt.Errorf("tests page: status %d", resp.status)
	}
	if _, in := loggedIn(resp.body); !in {
		return 0, errExpired
	}
	csrf := csrfToken(resp.body)
	if csrf == "" {
		return 0, errors.New("tests page has no download form")
	}
	zf, err := os.CreateTemp(c.root, web.TempPrefix+"zip-")
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = zf.Close()
		_ = os.Remove(zf.Name())
	}()
	resp, err = c.pace.do(ctx, request{url: u, jar: jar, into: zf, limit: maxZip,
		form: url.Values{"csrf_token": {csrf}, "download": {"true"}}})
	if err != nil {
		return 0, err
	}
	if resp.status == http.StatusNotFound {
		return 0, errExpired
	}
	if resp.status != http.StatusOK {
		return 0, fmt.Errorf("tests download: status %d", resp.status)
	}
	size, err := zf.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	head := make([]byte, 4)
	if _, err := zf.ReadAt(head, 0); err != nil || !bytes.Equal(head, []byte("PK\x03\x04")) {
		page, _ := readHead(zf, maxPage)
		if _, in := loggedIn(page); !in {
			return 0, errExpired
		}
		return 0, fmt.Errorf("CSES sent %s instead of the tests zip", sniff(resp.header, page))
	}
	return unzipTests(zf, size, dst)
}

func readHead(f *os.File, n int64) ([]byte, error) {
	return io.ReadAll(io.NewSectionReader(f, 0, n))
}

// sniff names what a non-zip body was, for the error.
func sniff(h http.Header, body []byte) string {
	ct := h.Get("Content-Type")
	if ct == "" {
		ct = http.DetectContentType(body)
	}
	return fmt.Sprintf("%d bytes of %s", len(body), ct)
}

var testNameRE = regexp.MustCompile(`^(\d+)\.(in|out)$`)

// unzipTests extracts N.in/N.out from the zip into dst. Entry paths are
// never used as paths: only a base name matching testNameRE is kept, so
// "../../x" and "__MACOSX/._1.in" are ignored, and two entries with one
// name are an error rather than a silent overwrite. Sizes are capped on
// the bytes read, not the header.
func unzipTests(r io.ReaderAt, size int64, dst string) (int, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return 0, fmt.Errorf("tests zip: %w", err)
	}
	seen := map[string]bool{}
	var total int64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := f.Name
		if i := strings.LastIndexAny(name, `/\`); i >= 0 {
			name = name[i+1:]
		}
		m := testNameRE.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		if n, err := strconv.Atoi(m[1]); err != nil || n <= 0 || n > 10000 {
			continue
		}
		if seen[name] {
			return 0, fmt.Errorf("tests zip has two entries for %s", name)
		}
		seen[name] = true
		n, err := copyEntry(f, filepath.Join(dst, name), min(maxTestFile, maxTestTotal-total))
		if err != nil {
			return 0, err
		}
		total += n
	}
	count := 0
	for name := range seen {
		if base, ok := strings.CutSuffix(name, ".in"); ok {
			if !seen[base+".out"] {
				return 0, fmt.Errorf("tests zip has %s without %s.out", name, base)
			}
			count++
		}
	}
	if count == 0 {
		return 0, errors.New("tests zip has no tests")
	}
	if count*2 != len(seen) {
		return 0, errors.New("tests zip has outputs without inputs")
	}
	return count, nil
}

func copyEntry(f *zip.File, dst string, limit int64) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer func() { _ = rc.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, io.LimitReader(rc, limit+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return n, err
	}
	if n > limit {
		return n, fmt.Errorf("%s unpacks to over %d bytes", f.Name, limit)
	}
	return n, nil
}
