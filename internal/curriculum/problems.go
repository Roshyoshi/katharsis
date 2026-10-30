package curriculum

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"katharsis/internal/catalog"
)

// ProblemList is one named list from a module's .problems.json; MDX refers
// to it by Key in <Problems problems="key"/> and <FocusProblem/>.
type ProblemList struct {
	Key      string
	Problems []ProblemRef
}

// ProblemRef is one guide problem entry. ID is the catalog ID for sources
// the game can serve (USACO, CSES, Codeforces; refID); everything else
// keeps ID == "" and is shown as an external link only.
type ProblemRef struct {
	UniqueID string     // guide id, e.g. "usaco-615", "cf-1398C"
	ID       catalog.ID // "usaco:615", "cses:1094", or "" when external
	Name     string
	URL      string
	Source   string // guide label: "Silver", "CSES", "CF", ...
	// Difficulty is the guide's label. Labels are relative to the module,
	// so "Hard" in Bronze is easier than "Easy" in Gold.
	Difficulty string
	Starred    bool
	Tags       []string
	// SolutionKind is solutionMetadata.kind; "in-module" means the module
	// text itself walks through the solution.
	SolutionKind string
}

var numericID = regexp.MustCompile(`^[0-9]+$`)

// cfID is a Codeforces problem as the guide and the catalog both name it:
// the contest, then the index ("1398C", "1741F2").
var cfID = regexp.MustCompile(`^[0-9]+[A-Z][0-9]?$`)

// CatalogID maps a guide uniqueId to a catalog ID: "usaco-615" ->
// "usaco:615", "cses-1094" -> "cses:1094", "cf-1398C" -> "cf:1398C".
// Anything else is external and returns "": another judge, a Codeforces
// gym ("cfgym-", "cf-gym-…"), an id of the wrong shape. Whether the game
// can judge a mapped id is the catalog's to say.
func CatalogID(uniqueID string) catalog.ID {
	src, num, ok := strings.Cut(uniqueID, "-")
	switch {
	case !ok:
	case src == "usaco" && numericID.MatchString(num):
		return catalog.NewID(catalog.USACO, num)
	case src == "cses" && numericID.MatchString(num):
		return catalog.NewID(catalog.CSES, num)
	case src == "cf" && cfID.MatchString(num):
		return catalog.NewID(catalog.CF, num)
	}
	return ""
}

// cfURL is a Codeforces problem link: a contest's problem or the
// problemset's, its contest and index.
var cfURL = regexp.MustCompile(`^https?://(?:www\.)?codeforces\.com/(?:contest/([0-9]+)/problem|problemset/problem/([0-9]+))/([A-Z][0-9]?)/?$`)

// refID is the catalog ID of a guide entry. A Codeforces entry is the
// problem its URL links: the guide's uniqueIds misname some ("cf-949D"
// links 1981D, "cf-2A" an EDU lesson), and an entry the URL puts in a
// gym, group or course is external.
func refID(uniqueID, url string) catalog.ID {
	if !strings.HasPrefix(uniqueID, "cf-") {
		return CatalogID(uniqueID)
	}
	m := cfURL.FindStringSubmatch(url)
	if m == nil {
		return ""
	}
	return catalog.NewID(catalog.CF, m[1]+m[2]+m[3])
}

type rawProblem struct {
	UniqueID   *string  `json:"uniqueId"`
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	Source     string   `json:"source"`
	Difficulty string   `json:"difficulty"`
	IsStarred  bool     `json:"isStarred"`
	Tags       []string `json:"tags"`
	Solution   *struct {
		Kind string `json:"kind"`
	} `json:"solutionMetadata"`
}

// parseProblems decodes a .problems.json, keeping list order (a Go map
// would lose it). It returns the MODULE_ID the file claims.
func parseProblems(file string, data []byte) (string, []ProblemList, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	errf := func(format string, args ...any) error {
		return fmt.Errorf("%s: %s", file, fmt.Sprintf(format, args...))
	}
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return "", nil, errf("want a JSON object")
	}
	var moduleID string
	var lists []ProblemList
	seen := map[string]bool{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return "", nil, errf("%v", err)
		}
		key := kt.(string)
		if seen[key] {
			return "", nil, errf("duplicate key %q", key)
		}
		seen[key] = true
		if key == "MODULE_ID" {
			if err := dec.Decode(&moduleID); err != nil {
				return "", nil, errf("MODULE_ID: %v", err)
			}
			continue
		}
		var raws []rawProblem
		if err := dec.Decode(&raws); err != nil {
			return "", nil, errf("list %q: %v", key, err)
		}
		l := ProblemList{Key: key}
		for i, r := range raws {
			if r.UniqueID == nil || *r.UniqueID == "" {
				return "", nil, errf("list %q entry %d has no uniqueId", key, i)
			}
			p := ProblemRef{
				UniqueID:   *r.UniqueID,
				ID:         refID(*r.UniqueID, r.URL),
				Name:       r.Name,
				URL:        r.URL,
				Source:     r.Source,
				Difficulty: r.Difficulty,
				Starred:    r.IsStarred,
				Tags:       r.Tags,
			}
			if r.Solution != nil {
				p.SolutionKind = r.Solution.Kind
			}
			l.Problems = append(l.Problems, p)
		}
		lists = append(lists, l)
	}
	if _, err := dec.Token(); err != nil {
		return "", nil, errf("%v", err)
	}
	if moduleID == "" {
		return "", nil, errf("no MODULE_ID")
	}
	return moduleID, lists, nil
}

// difficultyRank follows ProblemDifficulty in the guide's
// src/models/problem.ts.
var difficultyRank = map[string]int{
	"Very Easy": 0,
	"Easy":      1,
	"Medium":    2,
	"Hard":      3,
	"Very Hard": 4,
	"Insane":    5,
}

// DifficultyRank returns the sort position of a guide difficulty label
// (Very Easy = 0 ... Insane = 5); "N/A" and unknown labels return 6.
func DifficultyRank(label string) int {
	if r, ok := difficultyRank[label]; ok {
		return r
	}
	return len(difficultyRank)
}
