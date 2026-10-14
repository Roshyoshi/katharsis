package learn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"deadcases/internal/catalog"
)

// ProblemList is one named list from a module's .problems.json; MDX refers
// to it by Key in <Problems problems="key"/> and <FocusProblem/>.
type ProblemList struct {
	Key      string
	Problems []ProblemRef
}

// ProblemRef is one guide problem entry. ID is the catalog ID for sources
// the game can serve (USACO, CSES); everything else keeps ID == "" and is
// shown as an external link only.
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

// Playable reports whether the problem maps to a catalog source the game
// can serve. Whether it is actually judgeable is the catalog's call.
func (p ProblemRef) Playable() bool { return p.ID != "" }

var numericID = regexp.MustCompile(`^[0-9]+$`)

// CatalogID maps a guide uniqueId to a catalog ID: "usaco-615" ->
// "usaco:615", "cses-1094" -> "cses:1094". Anything else, including
// non-numeric suffixes, is external and returns "".
func CatalogID(uniqueID string) catalog.ID {
	src, num, ok := strings.Cut(uniqueID, "-")
	if !ok || !numericID.MatchString(num) {
		return ""
	}
	switch src {
	case "usaco":
		return catalog.ID(string(catalog.USACO) + ":" + num)
	case "cses":
		return catalog.ID(string(catalog.CSES) + ":" + num)
	}
	return ""
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
				ID:         CatalogID(*r.UniqueID),
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

// difficultyRank orders the guide's labels (ProblemDifficulty in the guide's
// src/models/problem.ts); "N/A" and unknown labels sort last.
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
