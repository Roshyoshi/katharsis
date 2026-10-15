// Package curriculum turns a snapshot of the USACO Guide repository
// (github.com/cpinitiative/usaco-guide) into learn-mode content: the
// curriculum tree from content/ordering.ts, each module's MDX converted to
// terminal-friendly markdown, its problem lists mapped to catalog IDs, and
// the trial (unit doors, integration topic and band, capability) that
// passing the module grants. It reads files only; fetching the snapshot is
// the content source's job.
//
// The guide is CC BY-NC-SA 4.0. Every rendered module ends with its own
// attribution line, and Attribution gives the notice for the whole
// collection; screens that show guide content must show one of them.
package curriculum

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Division is one top-level section of ordering.ts; learn mode lists them as
// the RTFM menu in guide order.
type Division struct {
	ID       string // ordering.ts section id: general, bronze, ..., adv
	Title    string // "Bronze", "Platinum", ...
	Chapters []Chapter
}

// Chapter groups modules the way the guide's sidebar does.
type Chapter struct {
	Title       string
	Description string
	Modules     []ModuleRef
}

// ModuleRef names a module inside the tree; Curriculum.Module resolves it.
type ModuleRef struct {
	ID string
}

// Module is one guide module. It is a value, but copies share their slices
// (treat them as read-only) and the converted markdown, which is computed
// once on first use.
type Module struct {
	ID            string
	Title         string
	Description   string
	Authors       []string
	Contributors  []string
	Prerequisites []string // module ids, or external URLs
	Frequency     int      // the guide's 0-4 "how often in contests", -1 if unset

	Division      string // section id, e.g. "silver"
	DivisionTitle string // "Silver"
	Chapter       string
	URL           string // https://usaco.guide/<division>/<id>
	Path          string // the .mdx file

	// Topic is the catalog topic the module's trial tests, "" for modules
	// with none (see TopicFor).
	Topic    string
	Problems []ProblemList

	body *lazyBody
}

type lazyBody struct {
	once   sync.Once
	render func() (string, error)
	md     string
	err    error
}

// Markdown returns the module converted to markdown, rendering it on the
// first call. Conversion errors name file:line; the same result is returned
// on every call.
func (m Module) Markdown() (string, error) {
	if m.body == nil {
		return "", errors.New("curriculum: module was not loaded by curriculum.Load")
	}
	m.body.once.Do(func() { m.body.md, m.body.err = m.body.render() })
	return m.body.md, m.body.err
}

// Attribution credits a single module, as CC BY-NC-SA asks: title, authors,
// source link, license, and that it was modified.
func (m Module) Attribution() string {
	by := ""
	if len(m.Authors) > 0 {
		by = " by " + strings.Join(m.Authors, ", ")
	}
	return fmt.Sprintf("“%s”%s, USACO Guide (%s), CC BY-NC-SA 4.0 (%s); converted from MDX to Markdown.",
		m.Title, by, m.URL, licenseURL)
}

const licenseURL = "https://creativecommons.org/licenses/by-nc-sa/4.0/"

// Attribution is the notice for learn-mode content as a whole.
func Attribution() string {
	return "Learn mode content is adapted from the USACO Guide " +
		"(https://usaco.guide, github.com/cpinitiative/usaco-guide) by the " +
		"Competitive Programming Initiative and its contributors, licensed " +
		"under CC BY-NC-SA 4.0 (" + licenseURL + "). Modules were converted " +
		"from MDX to Markdown and adapted for a terminal; the adapted content " +
		"is shared under the same license."
}

// Curriculum is a loaded guide snapshot. It is read-only after Load and
// safe for concurrent use.
type Curriculum struct {
	Divisions []Division
	dir       string // the guide repository root, for file names in errors
	modules   map[string]Module
	order     []string
}

// Module looks a module up by id.
func (c *Curriculum) Module(id string) (Module, bool) {
	m, ok := c.modules[id]
	return m, ok
}

// Modules lists every module in guide order.
func (c *Curriculum) Modules() []Module {
	out := make([]Module, 0, len(c.order))
	for _, id := range c.order {
		out = append(out, c.modules[id])
	}
	return out
}

// Load reads a guide snapshot. guideDir is the repository root (holding
// content/ordering.ts); the content directory itself is accepted too.
// Structure problems (bad ordering.ts, frontmatter or problems.json, a
// listed module with no file) fail the load; MDX conversion is deferred
// to Module.Markdown.
func Load(guideDir string) (*Curriculum, error) {
	root := guideDir
	content := filepath.Join(root, "content")
	if _, err := os.Stat(filepath.Join(content, "ordering.ts")); err != nil {
		if _, err2 := os.Stat(filepath.Join(guideDir, "ordering.ts")); err2 != nil {
			return nil, fmt.Errorf("curriculum: %s has no content/ordering.ts: %w", guideDir, err)
		}
		content, root = guideDir, filepath.Dir(guideDir)
	}
	divs, err := ParseOrdering(filepath.Join(content, "ordering.ts"))
	if err != nil {
		return nil, err
	}

	type mdxFile struct {
		path string
		fm   frontmatter
	}
	mdx := map[string]mdxFile{}
	problems := map[string][]ProblemList{}
	problemFile := map[string]string{}
	err = filepath.WalkDir(content, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch {
		case strings.HasSuffix(path, ".problems.json"):
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			id, lists, err := parseProblems(path, b)
			if err != nil {
				return err
			}
			if prev, dup := problemFile[id]; dup {
				return fmt.Errorf("%s: MODULE_ID %q also used by %s", path, id, prev)
			}
			problemFile[id] = path
			problems[id] = lists
		case strings.HasSuffix(path, ".mdx"):
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmSrc, _, _, err := splitFrontmatter(string(b))
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			fm, err := parseFrontmatter(fmSrc)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			id := fm.str("id")
			if id == "" {
				return nil // not a module (no id), e.g. a stray doc page
			}
			if prev, dup := mdx[id]; dup {
				return fmt.Errorf("%s: module id %q also used by %s", path, id, prev.path)
			}
			mdx[id] = mdxFile{path: path, fm: fm}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	c := &Curriculum{Divisions: divs, dir: root, modules: map[string]Module{}}
	for _, d := range divs {
		for _, ch := range d.Chapters {
			for _, ref := range ch.Modules {
				f, ok := mdx[ref.ID]
				if !ok {
					return nil, fmt.Errorf("curriculum: ordering.ts lists module %q but no .mdx has that id", ref.ID)
				}
				m := Module{
					ID:            ref.ID,
					Title:         f.fm.str("title"),
					Description:   f.fm.str("description"),
					Authors:       f.fm.list("author"),
					Contributors:  f.fm.list("contributors"),
					Prerequisites: f.fm.list("prerequisites"),
					Frequency:     -1,
					Division:      d.ID,
					DivisionTitle: d.Title,
					Chapter:       ch.Title,
					URL:           siteBase + "/" + d.ID + "/" + ref.ID,
					Path:          f.path,
					Problems:      problems[ref.ID],
				}
				if fq := f.fm.str("frequency"); fq != "" {
					if m.Frequency, err = strconv.Atoi(fq); err != nil {
						return nil, fmt.Errorf("%s: frequency %q is not a number", f.path, fq)
					}
				}
				m.Topic = TopicFor(m.ID, m.Title, m.Division)
				m.body = &lazyBody{render: func() (string, error) {
					md, _, err := c.convert(m)
					return md, err
				}}
				c.modules[m.ID] = m
				c.order = append(c.order, m.ID)
			}
		}
	}
	return c, nil
}

// convertStats are the soft findings of one conversion, for the full-repo
// test: elements rendered as bare inner text, and dropped {expressions}.
type convertStats struct {
	unknown map[string]int
	dropped []string
}

func (c *Curriculum) convert(m Module) (string, convertStats, error) {
	b, err := os.ReadFile(m.Path)
	if err != nil {
		return "", convertStats{}, err
	}
	_, body, line, err := splitFrontmatter(string(b))
	if err != nil {
		return "", convertStats{}, fmt.Errorf("%s: %w", m.Path, err)
	}
	rel, err := filepath.Rel(c.dir, m.Path)
	if err != nil {
		rel = m.Path
	}
	nodes, sc, err := parseMDX(rel, body, line)
	if err != nil {
		return "", convertStats{}, err
	}
	r := &renderer{file: rel, moduleURL: m.URL, lists: map[string]ProblemList{}, unknown: map[string]int{}}
	for _, l := range m.Problems {
		r.lists[l.Key] = l
	}
	md, err := r.block(nodes)
	st := convertStats{unknown: r.unknown, dropped: sc.dropped}
	if err != nil {
		return "", st, err
	}
	return c.header(m) + md + "\n\n---\n\n*" + escapeMarkdown(m.Attribution()) + "*\n", st, nil
}

// header is the module's title block: title, description, authors and
// prerequisites (linked to their modules when known).
func (c *Curriculum) header(m Module) string {
	var b strings.Builder
	b.WriteString("# " + escapeMarkdown(m.Title) + "\n\n")
	if m.Description != "" {
		b.WriteString("*" + escapeMarkdown(m.Description) + "*\n\n")
	}
	var meta []string
	if len(m.Authors) > 0 {
		meta = append(meta, "**Authors:** "+escapeMarkdown(strings.Join(m.Authors, ", ")))
	}
	if len(m.Contributors) > 0 {
		meta = append(meta, "**Contributors:** "+escapeMarkdown(strings.Join(m.Contributors, ", ")))
	}
	if len(m.Prerequisites) > 0 {
		var ps []string
		for _, p := range m.Prerequisites {
			switch pm, ok := c.modules[p]; {
			case ok:
				ps = append(ps, link(pm.Title, pm.URL))
			case strings.HasPrefix(p, "http"):
				ps = append(ps, link(p, p))
			default: // an id this snapshot does not have
				ps = append(ps, escapeMarkdown(p))
			}
		}
		meta = append(meta, "**Prerequisites:** "+strings.Join(ps, ", "))
	}
	if len(meta) > 0 {
		b.WriteString(strings.Join(meta, "\\\n") + "\n\n")
	}
	return b.String()
}
