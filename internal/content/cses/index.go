package cses

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"katharsis/internal/catalog"
	"katharsis/internal/content/web"
)

// row is one task as the problem list shows it.
type row struct {
	ID       string `json:"id"` // "1068"
	Title    string `json:"title"`
	Section  string `json:"section"`  // "Introductory Problems"
	Position int    `json:"position"` // 1-based, within the section
	Solvers  int    `json:"solvers"`  // users with an accepted solution
	Attempts int    `json:"attempts"` // users who submitted at all
}

// index is index.json: the list as last read, and the facts of every task
// whose page a Fetch has read since. The problems are derived from these
// at load, so a change to the derivation (ratings, topics, facts.json)
// needs no network.
type index struct {
	Key      string           `json:"key"`
	Listed   time.Time        `json:"listed"` // zero: rows are facts.json's
	Rows     []row            `json:"rows"`
	Verified map[string]facts `json:"verified,omitempty"`
}

// schema changes whenever index's format changes, so an index another
// build wrote is read again from the network rather than misread.
const schema = 1

var indexKey = fmt.Sprintf("schema=%d", schema)

// ErrNoIndex means the cache holds no index this build can read.
var ErrNoIndex = errors.New("no CSES index cached for this build")

// factsRow is one task in facts.json.
type factsRow struct {
	row
	facts
}

// facts.json is every task's list row and page facts, read from cses.fi
// by TestRegenFacts. It is the fallback list when a first start is
// offline, and the facts of each task until a Fetch reads its page.
//
//go:embed facts.json
var factsJSON []byte

func loadFactsTable() ([]factsRow, error) {
	var t []factsRow
	if err := json.Unmarshal(factsJSON, &t); err != nil {
		return nil, fmt.Errorf("facts.json: %w", err)
	}
	return t, nil
}

func loadIndex(p string) (*index, bool) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	var idx index
	if json.Unmarshal(b, &idx) != nil || idx.Key != indexKey || len(idx.Rows) == 0 {
		return nil, false
	}
	return &idx, true
}

func saveIndex(p string, idx *index) error {
	b, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	return web.WriteFile(p, b)
}

// ---- the problem list ----------------------------------------------------

var (
	taskHrefRE = regexp.MustCompile(`^/problemset/task/(\d+)/?$`)
	detailRE   = regexp.MustCompile(`^(\d+) / (\d+)$`)
)

// parseList reads /problemset/list: an <h2> per section, each followed by
// a ul.task-list whose li.task items hold the task link and a
// span.detail "solvers / attempts". Sections without tasks ("General")
// are skipped.
func parseList(page []byte) ([]row, error) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	content := find(doc, elem(atom.Div, "content"))
	if content == nil {
		return nil, errors.New("no content on the problem list")
	}
	var rows []row
	seen := map[string]bool{}
	section := ""
	pos := 0
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.ElementNode && n.DataAtom == atom.H2:
			section, pos = squash(textOf(n)), 0
			return
		case n.Type == html.ElementNode && n.DataAtom == atom.Li && hasClass(n, "task"):
			a := find(n, elem(atom.A, ""))
			if a == nil || section == "" {
				return
			}
			m := taskHrefRE.FindStringSubmatch(attr(a, "href"))
			if m == nil || seen[m[1]] {
				return
			}
			r := row{ID: m[1], Title: squash(textOf(a)), Section: section}
			if d := find(n, elem(atom.Span, "detail")); d != nil {
				if dm := detailRE.FindStringSubmatch(squash(textOf(d))); dm != nil {
					r.Solvers, _ = strconv.Atoi(dm[1])
					r.Attempts, _ = strconv.Atoi(dm[2])
				}
			}
			pos++
			r.Position = pos
			seen[r.ID] = true
			rows = append(rows, r)
			// li.task is not closed in the markup, so the parser nests the
			// next one inside it.
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(content)
	if len(rows) == 0 {
		return nil, errors.New("no tasks on the problem list")
	}
	return rows, nil
}

// ---- deriving problems ---------------------------------------------------

// entry is one problem as the catalog serves it. Playable is whether the
// task itself can be judged locally (not interactive, one right answer);
// Judgeable on top of it depends on the account and the cache, so All
// and Get set it on the way out.
type entry struct {
	catalog.Problem
	Playable bool
	Verified bool // facts come from the task page, not the default
}

// derive builds the entries from the list rows, taking each task's facts
// from verified (read by a Fetch), else facts.json, else the defaults.
func derive(rows []row, table map[string]facts, verified map[string]facts) []entry {
	top := 0
	for _, r := range rows {
		top = max(top, r.Solvers)
	}
	out := make([]entry, 0, len(rows))
	for _, r := range rows {
		f, ok := verified[r.ID]
		e := entry{Verified: ok}
		if !ok {
			if f, ok = table[r.ID]; !ok {
				f = facts{TimeLimit: defaultTime, MemLimit: defaultMem}
			}
		}
		if r.Section == interactiveSection {
			f.Interactive = true
		}
		id := catalog.NewID(catalog.CSES, r.ID)
		e.Problem = catalog.Problem{
			ID:       id,
			Source:   catalog.CSES,
			Title:    r.Title,
			URL:      taskURL(r.ID),
			Contest:  "CSES Problem Set, " + r.Section,
			Position: r.Position,
			Tags:     []string{r.Section},
			Topic:    sectionTopic(r.Section),

			Rating:       rating(r.Solvers, top),
			RatingSource: "cses-solvers",

			TimeLimit:      f.TimeLimit,
			MemLimit:       f.MemLimit,
			Interactive:    f.Interactive,
			NeedsChecker:   f.NeedsChecker,
			FloatTolerance: f.FloatTolerance,
		}
		e.Playable = !f.Interactive && !f.NeedsChecker
		out = append(out, e)
	}
	return out
}

const interactiveSection = "Interactive Problems"

func taskURL(id string) string { return site + "/problemset/task/" + id }

// rating maps a task's solver count to the Codeforces scale. CSES lists
// how many users solved each task; fewer solvers means harder, and the
// count spans three orders of magnitude (181,740 for Weird Algorithm to
// 116 when facts.json was made), so the map is linear in the log:
//
//	rating = 800 + 600 * log10(top / solvers)
//
// where top is the most solved task's count, rounded to 50 and clamped
// to 800..3000. Measured on the 2026-10-17 list: Weird Algorithm 800,
// Apartments (67,888) 1050, Dynamic Range Sum Queries (33,173) 1250,
// median task (3,334) 1850, Houses and Schools (974) 2150, least solved
// 2700. It is monotone, and normalizing by top keeps it steady as the
// counts grow. Its known bias: users work through the list in order, so
// later sections (Additional Problems) have fewer solvers than their
// difficulty alone would give, and rate somewhat high.
func rating(solvers, top int) int {
	if solvers <= 0 || top <= 0 {
		return ratingMax // unsolved, or no count: as hard as the scale goes
	}
	r := 800 + 600*math.Log10(float64(top)/float64(solvers))
	r = math.Round(r/50) * 50
	return int(min(max(r, 800), ratingMax))
}

const ratingMax = 3000

// sectionTopic maps a list section to a biome topic. A section that
// teaches one technique maps to it, the rest are Mixed: Introductory,
// Advanced Techniques, Construction, Interactive and the two Additional
// sets hold every kind of task. Sliding Window maps to data-structures,
// as the guide's Sliding Window module does (guide.ModuleTopic).
func sectionTopic(section string) string {
	if t, ok := sectionTopics[section]; ok {
		return t
	}
	return catalog.Mixed
}

var sectionTopics = map[string]string{
	"Sorting and Searching":   "sorting-greedy",
	"Dynamic Programming":     "dp",
	"Graph Algorithms":        "graphs",
	"Range Queries":           "data-structures",
	"Tree Algorithms":         "trees",
	"Mathematics":             "math",
	"String Algorithms":       "strings",
	"Geometry":                "geometry",
	"Sliding Window Problems": "data-structures",
	"Bitwise Operations":      "math",
	"Advanced Graph Problems": "graphs",
	"Counting Problems":       "math",
}

// tableFacts indexes facts.json by task id.
func tableFacts(t []factsRow) map[string]facts {
	m := make(map[string]facts, len(t))
	for _, r := range t {
		m[r.ID] = r.facts
	}
	return m
}

func tableRows(t []factsRow) []row {
	rows := make([]row, len(t))
	for i, r := range t {
		rows[i] = r.row
	}
	return rows
}

// validID admits only task ids that are safe as a directory name.
func validID(s string) bool {
	if s == "" || len(s) > 8 {
		return false
	}
	return strings.Trim(s, "0123456789") == ""
}
