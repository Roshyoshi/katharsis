//go:build qa

// Package render audits how the reader draws every cached statement and
// editorial: each page goes through the reader's own stages at 80 columns
// and is checked for the ways a terminal rendering of a math-heavy page
// goes wrong (raw TeX, scripts, fractions, environments, images, tables,
// lists, code). It reads the cache and never writes it, but opening the
// Codeforces catalog sweeps debris from its directory, so run it with HOME
// at a scratch dir whose Library/Caches/katharsis holds a clone (cp -c) of
// codeforces/{index.json,text.bin} and the usaco/*/*.md files, and GOPATH,
// GOMODCACHE and GOCACHE set to the real ones:
//
//	HOME=$scratch go test -tags qa ./qa/render -run TestRenderAudit -v
//
// QA_ONLY=cf:1899C,usaco: narrows it to those ids or sources; QA_OUT=file
// writes every hit as a TSV row (class, id, page, key, example).
package render

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/catalog"
	"katharsis/internal/content/codeforces"
	"katharsis/internal/screens/reader"
)

// width is the reader's width under audit: tmux beside vim at 80x24 is
// the narrowest full-screen reader the game promises.
const width = 80

// page is one markdown file to render.
type page struct {
	id   catalog.ID
	kind string // "statement" or "editorial"
	md   string
}

// hit is one failure found on one page; key sorts hits within the class
// (the TeX command, the environment) and example shows it.
type hit struct {
	class, key, example string
}

// drawn is a page through the reader's stages.
type drawn struct {
	page
	prep  string   // reader.Prepare(md)
	lines []string // the page as the reader draws it, ANSI stripped
	prose []string // the page drawn with its code blanked, ANSI stripped
	spans []span   // the math in md, each converted alone
}

// span is one math span of the source and its conversion.
type span struct{ tex, out string }

// class is one way a page can render wrong.
type class struct {
	name, what string
	find       func(d *drawn) []hit
}

var classes = []class{
	{"tex-cmd", `a TeX command reaches the page (\foo)`, findTeXCommands},
	{"tex-dollar", "a math $ reaches the page", findDollars},
	{"tex-brace", "^{ or _{ reaches the page", findScriptBraces},
	{"tex-lost", "a formula the source lost ($$$$)", findLostFormulas},
	{"escape", `a markdown escape shows its backslash (\$5)`, findShownEscapes},
	{"script", "a sub/superscript reads wrong (raw _, ^∘, [])", findBrokenScripts},
	{"frac", "a fraction binds to its neighbour (3/2r, 22/3)", findFractions},
	{"env-rows", "an environment's rows run together", findMergedRows},
	{"image", "an image drawn as a bare URL", findImages},
	{"table-wide", "a table wider than 80 columns", findWideTables},
	{"wide", "a line wider than 80 columns", findWide},
	{"list", "an ordered list renumbered", findLists},
	{"code-mangled", "a fenced code line not on the page as written", findMangledCode},
	{"code-unfenced", "code drawn as paragraphs (3+ code lines in prose)", findUnfencedCode},
	{"html", "an HTML tag reaches the page", findHTML},
}

func TestRenderAudit(t *testing.T) {
	pages := corpus(t)
	if len(pages) == 0 {
		t.Skip("no cached pages")
	}
	hits := make([][]hit, len(pages))
	var wg sync.WaitGroup
	next := make(chan int)
	for range runtime.NumCPU() {
		wg.Go(func() {
			for i := range next {
				d, err := draw(pages[i])
				if err != nil {
					hits[i] = []hit{{"render-error", "", err.Error()}}
					continue
				}
				for _, c := range classes {
					hits[i] = append(hits[i], c.find(d)...)
				}
			}
		})
	}
	for i := range pages {
		next <- i
	}
	close(next)
	wg.Wait()
	report(t, pages, hits)
}

// corpus is every Codeforces statement and editorial the index holds, and
// every USACO page fetched into the cache.
func corpus(t *testing.T) []page {
	t.Helper()
	var out []page
	add := func(id catalog.ID, kind, md string) {
		if md != "" && only(id) {
			out = append(out, page{id, kind, md})
		}
	}
	cf, err := codeforces.Cached()
	if err != nil {
		t.Logf("codeforces: %v", err)
	} else {
		for _, p := range cf.All() {
			if !only(p.ID) {
				continue
			}
			st, ed, err := cf.Markdown(p.ID)
			if err != nil {
				t.Errorf("%s: %v", p.ID, err)
				continue
			}
			add(p.ID, "statement", st)
			add(p.ID, "editorial", ed)
		}
	}
	cache, _ := os.UserCacheDir()
	files, _ := filepath.Glob(filepath.Join(cache, "katharsis", "usaco", "*", "*.md"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		kind := strings.TrimSuffix(filepath.Base(f), ".md")
		add(catalog.ID("usaco:"+filepath.Base(filepath.Dir(f))), kind, string(b))
	}
	return out
}

// only reports whether id passes the QA_ONLY filter.
func only(id catalog.ID) bool {
	f := os.Getenv("QA_ONLY")
	if f == "" {
		return true
	}
	for _, s := range strings.Split(f, ",") {
		s = strings.TrimSpace(s)
		if s == string(id) || strings.HasSuffix(s, ":") && strings.HasPrefix(string(id), s) {
			return true
		}
	}
	return false
}

func draw(p page) (*drawn, error) {
	d := &drawn{page: p, prep: reader.Prepare(p.md)}
	full, err := reader.Render(d.prep, width)
	if err != nil {
		return nil, err
	}
	prose, err := reader.Render(blankCode(d.prep), width)
	if err != nil {
		return nil, err
	}
	d.lines = strings.Split(ansi.Strip(full), "\n")
	d.prose = strings.Split(ansi.Strip(prose), "\n")
	d.spans = mathSpans(p.md)
	return d, nil
}

// blankCode drops fenced blocks and inline code spans from markdown, so
// what is left on the page is prose: sample data and code may hold $ and
// backslashes by right.
func blankCode(md string) string {
	var b strings.Builder
	code, text := reader.Fences(md)
	for i, s := range text {
		if code[i] {
			b.WriteString("\n")
			continue
		}
		b.WriteString(dropCodeSpans(s))
	}
	return b.String()
}

func dropCodeSpans(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\\' && i+1 < len(s) {
			b.WriteString(s[i : i+2])
			i += 2
			continue
		}
		if s[i] != '`' {
			b.WriteByte(s[i])
			i++
			continue
		}
		n := run(s, i, '`')
		end := -1
		for j := i + n; j < len(s); {
			k := strings.IndexByte(s[j:], '`')
			if k < 0 {
				break
			}
			j += k
			if m := run(s, j, '`'); m == n {
				end = j
				break
			} else {
				j += m
			}
		}
		if end < 0 {
			b.WriteString(s[i : i+n])
			i += n
			continue
		}
		i = end + n
	}
	return b.String()
}

func run(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

// prose is md outside fences and code spans.
func prose(md string) string {
	var b strings.Builder
	code, text := reader.Fences(md)
	for i, s := range text {
		if !code[i] {
			b.WriteString(dropCodeSpans(s))
		}
	}
	return b.String()
}

// mathSpans finds the $x$, $$x$$ and \(x\) spans of a page outside code,
// the delimiters the sources write, and converts each alone.
func mathSpans(md string) []span {
	var out []span
	for _, m := range mathRE.FindAllStringSubmatch(prose(md), -1) {
		tex := cmp.Or(m[1], m[2], m[3])
		out = append(out, span{tex, convert(tex)})
	}
	return out
}

var mathRE = regexp.MustCompile(`(?s)\$\$(.+?)\$\$|\$([^$\s](?:[^$\\]|\\.)*?)\$|\\\((.+?)\\\)`)

// convert is one formula as the reader converts it.
func convert(tex string) string { return reader.TeXToUnicode(`\(` + tex + `\)`) }

// ---- classes -------------------------------------------------------------------

var (
	texCmdRE      = regexp.MustCompile(`\\([A-Za-z]{2,})`)
	scriptBraceRE = regexp.MustCompile(`[\^_]\{`)
	rawSubRE      = regexp.MustCompile(`[A-Za-z0-9)\]]\\?_[A-Za-z0-9({]`)
	shownRE       = regexp.MustCompile(`\\["$%&,/:;=?@^~]`)
	lostRE        = regexp.MustCompile(`(?:\\\$){2,}`)
	htmlRE        = regexp.MustCompile(`</?(?:sup|sub|br|p|div|span|img|b|i|em|strong|code|pre|ul|ol|li|table|tr|td|th|a|center|font)(?:\s+[a-z-]+=|\s*/?>)`)
	listItemRE    = regexp.MustCompile(`^(\s*)(\d{1,9})([.)])\s`)
	imageRE       = regexp.MustCompile(`!\[\s*\]\(([^)]*)\)`)
	tableSepRE    = regexp.MustCompile(`^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)+\|?\s*$`)
	codeLineRE    = regexp.MustCompile(`^\s*(#include\b|int main\b|using namespace\b|return 0;|for ?\(.*;.*;.*\)|while ?\(.*\) ?\{?$|if ?\(.*\) ?\{$|\}$|\{$|(int|long long|ll|vector<.*>|cin|cout|printf|scanf|def|import|print)\b.*[;:]$)`)
	vulgarRE      = regexp.MustCompile(`[½⅓⅔¼¾⅕⅖⅗⅘⅙⅚⅐⅛⅜⅝⅞⅑⅒⁄]`)
)

func findTeXCommands(d *drawn) []hit {
	var out []hit
	seen := map[string]bool{}
	for _, l := range d.prose {
		for _, m := range texCmdRE.FindAllStringSubmatchIndex(l, -1) {
			name := l[m[2]:m[3]]
			if !seen[name] {
				seen[name] = true
				out = append(out, hit{"tex-cmd", `\` + name, clip(l, m[0])})
			}
		}
	}
	return out
}

// findDollars counts the page's dollars against the source's literal
// (escaped) ones: any more are math delimiters left as text.
func findDollars(d *drawn) []hit {
	literal := strings.Count(prose(d.md), `\$`)
	n, at := 0, ""
	for _, l := range d.prose {
		if k := strings.Count(l, "$"); k > 0 {
			n += k
			if at == "" {
				at = clip(l, strings.IndexByte(l, '$'))
			}
		}
	}
	if n > literal {
		return []hit{{"tex-dollar", "", fmt.Sprintf("%d $ for %d literal: %s", n, literal, at)}}
	}
	return nil
}

func findScriptBraces(d *drawn) []hit {
	for _, l := range d.prose {
		if m := scriptBraceRE.FindStringIndex(l); m != nil {
			return []hit{{"tex-brace", "", clip(l, m[0])}}
		}
	}
	return nil
}

// findLostFormulas finds a formula the dataset lost: as four dollars, or
// marked as missing by the Codeforces converter, which reads better but
// still needs the page.
func findLostFormulas(d *drawn) []hit {
	src := prose(d.md)
	if m := lostRE.FindStringIndex(src); m != nil {
		return []hit{{"tex-lost", "as $$$$", clip(src, m[0])}}
	}
	if i := strings.Index(src, lostMark); i >= 0 {
		return []hit{{"tex-lost", "marked", clip(src, i)}}
	}
	return nil
}

// lostMark is how the Codeforces converter marks a lost formula.
const lostMark = "formula missing from the dataset"

func findShownEscapes(d *drawn) []hit {
	for _, l := range d.prose {
		if m := shownRE.FindStringIndex(l); m != nil {
			return []hit{{"escape", l[m[0]:m[1]], clip(l, m[0])}}
		}
	}
	return nil
}

func findBrokenScripts(d *drawn) []hit {
	var out []hit
	seen := map[string]bool{}
	add := func(key string, s span) {
		if !seen[key] {
			seen[key] = true
			out = append(out, hit{"script", key, short(s.tex + " → " + s.out)})
		}
	}
	for _, s := range d.spans {
		switch {
		// An underscore in \text{max_flow} is a name's, and [] in the
		// source is an empty array; the converter makes neither.
		case len(rawSubRE.FindAllString(s.out, -1)) > strings.Count(s.tex, "_")-strings.Count(dropText(s.tex), "_"):
			add("raw _", s)
		case strings.Contains(s.out, "^∘"):
			add("^∘", s)
		case strings.Count(s.out, `\[]`) > len(emptyRE.FindAllString(s.tex, -1)) || strings.Contains(s.out, "^()") || strings.HasSuffix(s.out, "^") && strings.HasSuffix(strings.TrimSpace(dropText(s.tex)), "^"):
			add("empty", s)
		}
	}
	return out
}

var fracRE = regexp.MustCompile(`\\[dt]?frac\b`)

// findFractions finds a fraction touching a factor in the source (3\frac12,
// \frac{a}{b}x, \frac{n}{2}^2) and checks the page sets it apart: in
// parentheses, or as a mixed number's ⅔.
func findFractions(d *drawn) []hit {
	var out []hit
	seen := map[string]bool{}
	for _, s := range d.spans {
		for _, m := range fracRE.FindAllStringIndex(s.tex, -1) {
			end, ok := fracEnd(s.tex, m[1])
			if !ok {
				continue
			}
			before := strings.TrimRight(s.tex[:m[0]], " ")
			after := strings.TrimLeft(s.tex[end:], " ")
			key := ""
			switch {
			case strings.HasPrefix(after, "^"):
				key = "frac^"
			case after != "" && (isWordByte(after[0]) || after[0] == '(' || strings.HasPrefix(after, `\frac`) ||
				strings.HasPrefix(after, `\sqrt`) || strings.HasPrefix(after, `\left(`) || strings.HasPrefix(after, `\binom`)):
				key = "frac·x"
			case before != "" && before[len(before)-1] == '/',
				wholeNumber(before): // not a script: \prod_{i=1}^8 \frac{L}{i}
				key = "n·frac"
			}
			if key == "" || seen[key] {
				continue
			}
			// Alone, a fraction can render its indexes otherwise (a₀, not
			// a[0]); then the page cannot be judged by it.
			alone := convert(s.tex[m[0]:end])
			if !strings.Contains(s.out, alone) || strings.Contains(s.out, "("+alone+")") ||
				key == "n·frac" && vulgarRE.MatchString(s.out) {
				continue
			}
			seen[key] = true
			out = append(out, hit{"frac", key, short(s.tex + " → " + s.out)})
		}
	}
	return out
}

// fracEnd skips the two arguments after a \frac at i.
func fracEnd(s string, i int) (int, bool) {
	for range 2 {
		for i < len(s) && s[i] == ' ' {
			i++
		}
		if i >= len(s) {
			return 0, false
		}
		if s[i] == '\\' {
			i++ // a command argument: \frac\pi 4
			for i < len(s) && isWordByte(s[i]) && !isDigit(s[i]) {
				i++
			}
			continue
		}
		if s[i] != '{' {
			i++ // a one-character argument: \frac12
			continue
		}
		depth := 0
		for ; i < len(s); i++ {
			if s[i] == '\\' {
				i++
				continue
			}
			if s[i] == '{' {
				depth++
			}
			if s[i] == '}' {
				depth--
				if depth == 0 {
					i++
					break
				}
			}
		}
		if depth != 0 {
			return 0, false
		}
	}
	return i, true
}

var envRE = regexp.MustCompile(`\\begin\{([a-zA-Z*]+)\}`)

// rowMark marks where an environment's row starts, so the page shows what
// stands between two rows.
const rowMark = "ROWMARK"

// findMergedRows renders each environment with a mark at the start of
// every row after the first, and finds rows the page runs together: the
// mark after the last row's text with nothing but a space between.
func findMergedRows(d *drawn) []hit {
	var out []hit
	seen := map[string]bool{}
	for _, s := range d.spans {
		env := envRE.FindStringSubmatch(s.tex)
		if env == nil || seen[env[1]] {
			continue
		}
		o := convert(markRows(s.tex))
		for i := strings.Index(o, rowMark); i >= 0; {
			before := strings.TrimRight(o[:i], " ")
			if r := lastRune(before); before != "" && !strings.ContainsRune(";,.{([|‖", r) {
				seen[env[1]] = true
				out = append(out, hit{"env-rows", env[1], short(s.tex + " → " + s.out)})
				break
			}
			j := strings.Index(o[i+len(rowMark):], rowMark)
			if j < 0 {
				break
			}
			i += len(rowMark) + j
		}
	}
	return out
}

// markRows puts \text{ROWMARK} after each \\ that ends a row of an
// environment, outside braces, unless only space follows it.
func markRows(tex string) string {
	var b strings.Builder
	envs, depth := 0, 0
	for i := 0; i < len(tex); i++ {
		c := tex[i]
		switch {
		case c == '{':
			depth++
		case c == '}':
			depth--
		case strings.HasPrefix(tex[i:], `\begin{`):
			envs++
		case strings.HasPrefix(tex[i:], `\end{`):
			envs--
		case strings.HasPrefix(tex[i:], `\\`):
			b.WriteString(`\\`)
			i++
			rest := strings.TrimSpace(tex[i+1:])
			if envs > 0 && !strings.HasPrefix(rest, `\end{`) && rest != "" {
				b.WriteString(` \text{` + rowMark + `}`)
			}
			continue
		case c == '\\' && i+1 < len(tex):
			b.WriteString(tex[i : i+2])
			i++
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// findImages finds an image the page shows as a bare URL: one without alt
// text, which Glamour draws without its "Image:" label.
func findImages(d *drawn) []hit {
	if m := imageRE.FindStringSubmatch(prose(d.prep)); m != nil {
		return []hit{{"image", "", m[0]}}
	}
	return nil
}

func findWideTables(d *drawn) []hit {
	if !hasTable(d.md) {
		return nil
	}
	for _, l := range d.lines {
		if strings.ContainsAny(l, "│|") && ansi.StringWidth(l) > width {
			return []hit{{"table-wide", "", fmt.Sprintf("%d cols: %s", ansi.StringWidth(l), clip(l, 0))}}
		}
	}
	return nil
}

func hasTable(md string) bool {
	for _, l := range strings.Split(md, "\n") {
		if tableSepRE.MatchString(l) {
			return true
		}
	}
	return false
}

// findWide finds a line the reader's 80-column viewport cuts off.
func findWide(d *drawn) []hit {
	if hasTable(d.md) {
		return nil
	}
	for _, l := range d.lines {
		if w := ansi.StringWidth(l); w > width {
			return []hit{{"wide", "", fmt.Sprintf("%d cols: %s", w, clip(l, 0))}}
		}
	}
	return nil
}

// findLists finds ordered items whose number the page does not show.
func findLists(d *drawn) []hit {
	page := strings.Join(d.lines, "\n")
	code, text := reader.Fences(d.prep)
	for i, s := range text {
		if code[i] {
			continue
		}
		prevBlank := true
		for _, l := range strings.Split(s, "\n") {
			if m := listItemRE.FindStringSubmatch(l); m != nil && prevBlank {
				if !strings.Contains(page, m[2]+".") && !strings.Contains(page, m[2]+")") {
					return []hit{{"list", "renumbered", strings.TrimSpace(l)}}
				}
			}
			prevBlank = strings.TrimSpace(l) == ""
		}
	}
	return nil
}

// findMangledCode checks every line of every fenced block reaches the page
// as written, give or take spacing: a line broken by the reader continues
// after a ↪, and is joined back here.
func findMangledCode(d *drawn) []hit {
	have := map[string]bool{}
	var prev string
	for _, l := range d.lines {
		t := strings.TrimSpace(l)
		if rest, ok := strings.CutPrefix(t, "↪"); ok {
			prev += strings.TrimSpace(rest)
		} else {
			prev = t
		}
		have[strings.ReplaceAll(prev, " ", "")] = true
	}
	code, text := reader.Fences(d.prep)
	for i, s := range text {
		if !code[i] {
			continue
		}
		lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
		for _, l := range lines[1 : len(lines)-1] {
			if t := strings.ReplaceAll(l, " ", ""); t != "" && !have[t] {
				return []hit{{"code-mangled", "", short(strings.TrimSpace(l))}}
			}
		}
	}
	return nil
}

func findUnfencedCode(d *drawn) []hit {
	n, ex := 0, ""
	for _, l := range d.prose {
		if codeLineRE.MatchString(l) {
			n++
			if ex == "" {
				ex = strings.TrimSpace(l)
			}
		}
	}
	if n >= 3 {
		return []hit{{"code-unfenced", "", fmt.Sprintf("%d lines, e.g. %s", n, ex)}}
	}
	return nil
}

func findHTML(d *drawn) []hit {
	for _, l := range d.prose {
		if m := htmlRE.FindStringIndex(l); m != nil {
			return []hit{{"html", l[m[0]+1 : m[1]], clip(l, m[0])}}
		}
	}
	return nil
}

func isWordByte(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || isDigit(c) }

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func lastRune(s string) rune {
	r := []rune(s)
	if len(r) == 0 {
		return 0
	}
	return r[len(r)-1]
}

// clip is the line around byte i, trimmed, at most about 70 bytes.
func clip(l string, i int) string {
	lo, hi := max(0, i-30), min(len(l), i+40)
	for lo > 0 && !isRuneStart(l[lo]) {
		lo--
	}
	for hi < len(l) && !isRuneStart(l[hi]) {
		hi++
	}
	return strings.TrimSpace(strings.ReplaceAll(l[lo:hi], "\n", " "))
}

// short is s on one line, at most about 200 bytes.
func short(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= 200 {
		return s
	}
	i := 200
	for i < len(s) && !isRuneStart(s[i]) {
		i++
	}
	return s[:i] + "…"
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

// ---- report --------------------------------------------------------------------

type tally struct {
	pages    int
	problems map[catalog.ID]bool
	ids      []string
	keys     map[string]int
	keyEx    map[string]string
}

func report(t *testing.T, pages []page, hits [][]hit) {
	tallies := map[string]*tally{}
	var tsv strings.Builder
	for i, hs := range hits {
		p := pages[i]
		seen := map[string]bool{}
		for _, h := range hs {
			ta := tallies[h.class]
			if ta == nil {
				ta = &tally{problems: map[catalog.ID]bool{}, keys: map[string]int{}, keyEx: map[string]string{}}
				tallies[h.class] = ta
			}
			if !seen[h.class] {
				seen[h.class] = true
				ta.pages++
				if len(ta.ids) < 4 {
					ta.ids = append(ta.ids, fmt.Sprintf("%s %s", p.id, p.kind[:2]))
				}
			}
			ta.problems[p.id] = true
			if h.key != "" {
				ta.keys[h.key]++
				if _, ok := ta.keyEx[h.key]; !ok {
					ta.keyEx[h.key] = string(p.id) + ": " + h.example
				}
			}
			fmt.Fprintf(&tsv, "%s\t%s\t%s\t%s\t%s\n", h.class, p.id, p.kind, h.key, h.example)
		}
	}
	nst := 0
	for _, p := range pages {
		if p.kind == "statement" {
			nst++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d pages (%d statements, %d editorials) at %d columns\n", len(pages), nst, len(pages)-nst, width)
	fmt.Fprintf(&b, "%-14s %6s %8s  %s\n", "class", "pages", "problems", "examples")
	names := []string{"render-error"}
	for _, c := range classes {
		names = append(names, c.name)
	}
	count := func(n string) int {
		if ta := tallies[n]; ta != nil {
			return ta.pages
		}
		return 0
	}
	slices.SortStableFunc(names, func(a, b string) int { return cmp.Compare(count(b), count(a)) })
	for _, n := range names {
		ta := tallies[n]
		if ta == nil {
			fmt.Fprintf(&b, "%-14s %6d %8d\n", n, 0, 0)
			continue
		}
		fmt.Fprintf(&b, "%-14s %6d %8d  %s\n", n, ta.pages, len(ta.problems), strings.Join(ta.ids, ", "))
		keys := make([]string, 0, len(ta.keys))
		for k := range ta.keys {
			keys = append(keys, k)
		}
		slices.SortFunc(keys, func(a, b string) int { return cmp.Or(cmp.Compare(ta.keys[b], ta.keys[a]), cmp.Compare(a, b)) })
		for i, k := range keys {
			if i >= 12 {
				break
			}
			fmt.Fprintf(&b, "    %-16s %5d  %s\n", k, ta.keys[k], short(ta.keyEx[k]))
		}
	}
	t.Log("\n" + b.String())
	if f := os.Getenv("QA_OUT"); f != "" {
		if err := os.WriteFile(f, []byte(tsv.String()), 0o644); err != nil {
			t.Error(err)
		}
	}
}

// TestRenderShow prints the pages QA_ONLY names as the reader draws them
// at 80 columns, each line ending in a | to show where it stops; QA_KIND
// picks statement or editorial:
//
//	QA_ONLY=usaco:1011 go test -tags qa ./qa/render -run TestRenderShow -v
func TestRenderShow(t *testing.T) {
	if os.Getenv("QA_ONLY") == "" {
		t.Skip("set QA_ONLY")
	}
	for _, p := range corpus(t) {
		if k := os.Getenv("QA_KIND"); k != "" && k != p.kind {
			continue
		}
		d, err := draw(p)
		if err != nil {
			t.Errorf("%s %s: %v", p.id, p.kind, err)
			continue
		}
		t.Logf("==== %s %s\n%s|", p.id, p.kind, strings.Join(d.lines, "|\n"))
	}
}

var emptyRE = regexp.MustCompile(`\[\s*(?:~|\\[,;: ])?\s*\]`)

// dropText is TeX without its \text{…}-like groups, whose underscores are
// part of names.
func dropText(tex string) string {
	var b strings.Builder
	for i := 0; i < len(tex); i++ {
		m := textGroupRE.FindStringIndex(tex[i:])
		if m == nil || m[0] != 0 {
			b.WriteByte(tex[i])
			continue
		}
		depth := 0
		j := i + m[1] - 1 // the group's brace
		for ; j < len(tex); j++ {
			if tex[j] == '{' {
				depth++
			}
			if tex[j] == '}' {
				if depth--; depth == 0 {
					break
				}
			}
		}
		i = j
	}
	return b.String()
}

var textGroupRE = regexp.MustCompile(`^\\(?:text[a-z]*|math(?:rm|it|tt|sf|bf)|operatorname|mbox)\s*\{`)

// wholeNumber reports whether TeX source s ends in a number standing on its
// own, not a script's (^8).
func wholeNumber(s string) bool {
	i := len(s)
	for i > 0 && isDigit(s[i-1]) {
		i--
	}
	return i < len(s) && (i == 0 || s[i-1] != '^' && s[i-1] != '_')
}
