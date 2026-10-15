// Package libs turns code the player wrote into reusable snippets (the
// game's blueprints) and puts linked ones into new doors' templates.
//
// A lib is the lines between two marker comments in a solution:
//
//	// lib: dsu
//	struct DSU { ... };
//	// end lib
//
// The markers are plain comments, so a solution compiles the same with or
// without them, and a template with libs injected can be mined again.
package libs

import (
	"fmt"
	"regexp"
	"strings"

	"katharsis/internal/game"
)

var (
	beginRE = regexp.MustCompile(`^\s*//\s*lib:\s*([A-Za-z0-9_\-]+)\s*$`)
	endRE   = regexp.MustCompile(`^\s*//\s*end lib\s*$`)
)

// Extract returns the libs marked in src, in order. A name marked twice
// keeps the later body, matching how a player would refine a snippet. An
// unterminated or nested marker is an error naming its line, so a typo
// never silently swallows the rest of a file.
func Extract(src string) ([]game.Lib, error) {
	var (
		out   []game.Lib
		index = map[string]int{}
		name  string
		start int
		body  []string
	)
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		if m := beginRE.FindStringSubmatch(l); m != nil {
			if name != "" {
				return nil, fmt.Errorf("line %d: lib %q starts inside lib %q (line %d)", i+1, m[1], name, start)
			}
			name, start, body = m[1], i+1, nil
			continue
		}
		if endRE.MatchString(l) {
			if name == "" {
				return nil, fmt.Errorf("line %d: end lib without a lib", i+1)
			}
			lib := game.Lib{Name: name, Source: strings.Trim(strings.Join(body, "\n"), "\n") + "\n"}
			if j, ok := index[name]; ok {
				out[j] = lib
			} else {
				index[name] = len(out)
				out = append(out, lib)
			}
			name = ""
			continue
		}
		if name != "" {
			body = append(body, l)
		}
	}
	if name != "" {
		return nil, fmt.Errorf("line %d: lib %q has no // end lib", start, name)
	}
	return out, nil
}

var mainRE = regexp.MustCompile(`^\s*(int|signed|auto)\s+main\s*\(`)

// Inject puts linked libs into a template just above main, each between
// its markers. Without a main they go at the end. Linking nothing returns
// the template unchanged.
func Inject(template string, linked []game.Lib) string {
	if len(linked) == 0 {
		return template
	}
	var block strings.Builder
	for _, l := range linked {
		fmt.Fprintf(&block, "// lib: %s\n%s// end lib\n\n", l.Name, ensureNewline(l.Source))
	}
	lines := strings.SplitAfter(template, "\n")
	for i, l := range lines {
		if mainRE.MatchString(l) {
			return strings.Join(lines[:i], "") + block.String() + strings.Join(lines[i:], "")
		}
	}
	return ensureNewline(template) + "\n" + block.String()
}

func ensureNewline(s string) string {
	if s != "" && !strings.HasSuffix(s, "\n") {
		return s + "\n"
	}
	return s
}
