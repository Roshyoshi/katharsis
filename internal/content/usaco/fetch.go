package usaco

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"katharsis/internal/catalog"
	"katharsis/internal/problem"
)

const (
	maxPage = 4 << 20
	// The largest zip at the pinned commit is 107 MB (cpid 1594); test
	// files unpack to a few times that. The caps only stop a broken or
	// hostile response from filling the disk.
	maxZip       = 512 << 20
	maxTestFile  = 512 << 20
	maxTestTotal = 2 << 30
)

// meta.json is written last into a problem dir; its presence means the
// dir is complete.
type meta struct {
	CPID         int       `json:"cpid"`
	Stem         string    `json:"stem"`
	Facts        facts     `json:"facts"`
	HasZip       bool      `json:"has_zip"`
	HasEditorial bool      `json:"has_editorial"`
	Samples      int       `json:"samples"`
	Tests        int       `json:"tests"`
	Fetched      time.Time `json:"fetched"`
	// Render is the renderVersion statement.md and editorial.md were
	// written with; 0 before versions were kept.
	Render int `json:"render,omitempty"`
}

// renderVersion changes whenever the markdown rendered from a cached page
// changes, so Fetch re-renders old dirs from the statement.html and
// editorial.html they keep, without the network.
//
//	1: "[" no longer escaped (it read as display math)
const renderVersion = 1

// rerender rewrites dir's statement.md and editorial.md, and the facts in
// m, from the pages saved there, and stamps m with renderVersion.
func rerender(dir string, cpid int, m *meta) error {
	page, err := os.ReadFile(filepath.Join(dir, "statement.html"))
	if err != nil {
		return err
	}
	st, err := parseStatement(page, statementURL(cpid))
	if err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(dir, "statement.md"), []byte(st.Markdown)); err != nil {
		return err
	}
	if m.HasEditorial {
		sol, err := os.ReadFile(filepath.Join(dir, "editorial.html"))
		if err != nil {
			return err
		}
		md, err := editorialMarkdown(sol, st, site+"/current/data/sol_"+m.Stem+".html")
		if err != nil {
			return err
		}
		if err := writeAtomic(filepath.Join(dir, "editorial.md"), []byte(md)); err != nil {
			return err
		}
	}
	m.Facts, m.Render = st.facts, renderVersion
	mb, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, "meta.json"), mb)
}

func loadAssets(dir string) (catalog.Assets, meta, bool) {
	var m meta
	if readJSON(filepath.Join(dir, "meta.json"), &m) != nil {
		return catalog.Assets{}, m, false
	}
	a := catalog.Assets{Statement: filepath.Join(dir, "statement.md")}
	if m.HasEditorial {
		a.Editorial = filepath.Join(dir, "editorial.md")
	}
	var err error
	if a.Samples, err = problem.LoadTests(filepath.Join(dir, "samples")); err != nil {
		return catalog.Assets{}, m, false
	}
	if a.Tests, err = problem.LoadTests(filepath.Join(dir, "tests")); err != nil {
		return catalog.Assets{}, m, false
	}
	if len(a.Samples) != m.Samples || len(a.Tests) != m.Tests {
		return catalog.Assets{}, m, false // tampered with; fetch again
	}
	return a, m, true
}

// materialize downloads everything for e into a temp dir and renames it
// into place, so a crash or a failed request never leaves a dir that
// loadAssets would accept. A 404 for the zip or the analysis is cached as
// absent; any other failure returns an error and nothing is kept, so the
// next Fetch retries.
func (c *Catalog) materialize(ctx context.Context, e entry, dir string) (catalog.Assets, error) {
	if err := os.MkdirAll(c.root, 0o755); err != nil {
		return catalog.Assets{}, err
	}
	tmp, err := os.MkdirTemp(c.root, fmt.Sprintf(".tmp-%d-", e.CPID))
	if err != nil {
		return catalog.Assets{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	m := meta{CPID: e.CPID, Stem: e.Stem, Fetched: time.Now().UTC(), Render: renderVersion}
	page, err := c.client.getBytes(ctx, c.base+statementPath(e.CPID), maxPage)
	if err != nil {
		return catalog.Assets{}, fmt.Errorf("%s statement: %w", e.ID, err)
	}
	st, err := parseStatement(page, statementURL(e.CPID))
	if err != nil {
		return catalog.Assets{}, fmt.Errorf("%s statement: %w", e.ID, err)
	}
	m.Facts = st.facts
	if err := writeFiles(tmp, map[string][]byte{
		"statement.html": page,
		"statement.md":   []byte(st.Markdown),
	}); err != nil {
		return catalog.Assets{}, err
	}
	if err := writeSamples(filepath.Join(tmp, "samples"), st.Samples); err != nil {
		return catalog.Assets{}, err
	}
	m.Samples = len(st.Samples)

	// Tests are skipped when there is nothing to judge with them: an
	// interactive problem, or a zip that facts.json marks as holding
	// another problem's data (see sharedStemData in regen_test.go).
	// HasZip is only false for a verified entry.
	if e.Stem != "" && !st.Interactive && e.HasZip {
		n, err := c.fetchTests(ctx, e.Stem, filepath.Join(tmp, "tests"))
		switch {
		case errors.Is(err, errNotFound):
		case err != nil:
			return catalog.Assets{}, fmt.Errorf("%s tests: %w", e.ID, err)
		default:
			m.HasZip, m.Tests = true, n
		}
	}
	if e.Stem != "" {
		solPath := "/current/data/sol_" + e.Stem + ".html"
		sol, err := c.client.getBytes(ctx, c.base+solPath, maxPage)
		switch {
		case errors.Is(err, errNotFound):
		case err != nil:
			return catalog.Assets{}, fmt.Errorf("%s analysis: %w", e.ID, err)
		default:
			md, err := editorialMarkdown(sol, st, site+solPath)
			if err != nil {
				return catalog.Assets{}, fmt.Errorf("%s analysis: %w", e.ID, err)
			}
			if err := writeFiles(tmp, map[string][]byte{
				"editorial.html": sol,
				"editorial.md":   []byte(md),
			}); err != nil {
				return catalog.Assets{}, err
			}
			m.HasEditorial = true
		}
	}
	mb, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return catalog.Assets{}, err
	}
	if err := os.WriteFile(filepath.Join(tmp, "meta.json"), mb, 0o644); err != nil {
		return catalog.Assets{}, err
	}

	// A dir without meta.json is debris from an older crash; replace it.
	if _, _, ok := loadAssets(dir); !ok {
		if err := os.RemoveAll(dir); err != nil {
			return catalog.Assets{}, err
		}
	}
	if err := os.Rename(tmp, dir); err != nil {
		// Another process published first; its copy is as good.
		if a, _, ok := loadAssets(dir); ok {
			return a, nil
		}
		return catalog.Assets{}, err
	}
	if err := c.update(e.ID, m); err != nil {
		return catalog.Assets{}, err
	}
	a, _, ok := loadAssets(dir)
	if !ok {
		return catalog.Assets{}, fmt.Errorf("%s: cache dir %s unreadable after fetch", e.ID, dir)
	}
	return a, nil
}

func writeFiles(dir string, files map[string][]byte) error {
	for name, b := range files {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeSamples(dir string, samples []sample) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, s := range samples {
		n := strconv.Itoa(i + 1)
		if err := writeFiles(dir, map[string][]byte{n + ".in": []byte(s.In), n + ".out": []byte(s.Out)}); err != nil {
			return err
		}
	}
	return nil
}

// fetchTests downloads data/<stem>.zip into dst/N.in, N.out. The zip is
// spooled to disk first because archive/zip needs random access.
func (c *Catalog) fetchTests(ctx context.Context, stem, dst string) (int, error) {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return 0, err
	}
	zf, err := os.CreateTemp(filepath.Dir(dst), "tests-*.zip")
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = zf.Close()
		_ = os.Remove(zf.Name()) // keep it out of the published dir
	}()
	err = c.client.get(ctx, c.base+"/current/data/"+stem+".zip", func(r io.Reader) error {
		// A retry after a cut connection starts the spool over.
		if _, err := zf.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if err := zf.Truncate(0); err != nil {
			return err
		}
		n, err := io.Copy(zf, io.LimitReader(r, maxZip+1))
		if err != nil {
			return err
		}
		if n > maxZip {
			return permanentError{fmt.Errorf("%s.zip is over %d bytes", stem, maxZip)}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	size, err := zf.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	return unzipTests(zf, size, dst)
}

// testNameRE accepts the two layouts USACO zips use: "3.in"/"3.out", and
// for Nov-Dec 2011 "I.3"/"O.3".
var testNameRE = regexp.MustCompile(`^(?:(\d+)\.(in|out)|([IO])\.(\d+))$`)

// unzipTests extracts the tests in a zip, normalizing names to N.in/N.out.
// Entry paths are never used as paths: only a base name matching
// testNameRE is kept, so "../../x" and "__MACOSX/._1.in" are ignored, and
// two entries that normalize to the same name are an error rather than a
// silent overwrite. Sizes are capped on the bytes read, not the header.
func unzipTests(r io.ReaderAt, size int64, dst string) (int, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return 0, permanentError{err}
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
		num, ext := m[1], "."+m[2]
		if m[3] != "" {
			num, ext = m[4], ".in"
			if m[3] == "O" {
				ext = ".out"
			}
		}
		if n, err := strconv.Atoi(num); err != nil || n <= 0 || n > 10000 {
			continue
		}
		norm := num + ext
		if seen[norm] {
			return 0, permanentError{fmt.Errorf("zip has two entries for %s", norm)}
		}
		seen[norm] = true
		n, err := copyEntry(f, filepath.Join(dst, norm), min(maxTestFile, maxTestTotal-total))
		if err != nil {
			return 0, err
		}
		total += n
	}
	var count int
	for name := range seen {
		if base, ok := strings.CutSuffix(name, ".in"); ok {
			if !seen[base+".out"] {
				return 0, permanentError{fmt.Errorf("zip has %s without %s.out", name, base)}
			}
			count++
		}
	}
	if count == 0 {
		return 0, permanentError{errors.New("zip has no tests")}
	}
	if count*2 != len(seen) {
		return 0, permanentError{errors.New("zip has outputs without inputs")}
	}
	return count, nil
}

func copyEntry(f *zip.File, dst string, limit int64) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, permanentError{err}
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
		return n, permanentError{err}
	}
	if n > limit {
		return n, permanentError{fmt.Errorf("%s unpacks to over %d bytes", f.Name, limit)}
	}
	return n, nil
}

// editorialMarkdown converts an analysis page. These are bare HTML: an
// "(Analysis by ...)" line, prose with MathJax, and <pre> code with no
// language, which gets one guessed so the terminal can highlight it.
func editorialMarkdown(page []byte, st *statement, srcURL string) (string, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return "", err
	}
	body := find(doc, func(n *html.Node) bool { return n.DataAtom == atom.Body })
	if body == nil {
		body = doc // a <frameset> page, say an error page served with a 200
	}
	for _, n := range findAll(body, func(n *html.Node) bool {
		return n.DataAtom == atom.Script || n.DataAtom == atom.Style || n.DataAtom == atom.Head
	}) {
		n.Parent.RemoveChild(n)
	}
	cleanHTML(body)
	for _, pre := range findAll(body, func(n *html.Node) bool { return n.DataAtom == atom.Pre }) {
		if lang := guessLang(textOf(pre)); lang != "" {
			setAttr(pre, "class", "language-"+lang)
		}
	}
	md, err := htmlToMarkdown(body, false)
	if err != nil {
		return "", err
	}
	return st.heading("Analysis: ") + strings.TrimSpace(md) +
		"\n\n---\n\nSource: <" + srcURL + ">\n", nil
}

func guessLang(code string) string {
	switch {
	case strings.Contains(code, "#include"):
		return "cpp"
	case strings.Contains(code, "public class") || strings.Contains(code, "System.out"):
		return "java"
	case strings.Contains(code, "def ") || strings.Contains(code, "print(") ||
		strings.Contains(code, "input()"):
		return "python"
	}
	return ""
}

func setAttr(n *html.Node, key, val string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}
