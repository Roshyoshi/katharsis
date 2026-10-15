package curriculum

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"katharsis/internal/catalog"
)

// TestFullGuide loads every module of a real snapshot. It runs only with
// KATHARSIS_GUIDE=<repo root>, since the snapshot is ~150 MB and not
// vendored. It asserts zero conversion failures, zero elements rendered as
// bare text, and that no C++ code block is lost, and logs the counts the
// curriculum feeds the game.
func TestFullGuide(t *testing.T) {
	dir := os.Getenv("KATHARSIS_GUIDE")
	if dir == "" {
		t.Skip("set KATHARSIS_GUIDE to a usaco-guide checkout to run")
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	isUSACO := func(id catalog.ID) bool { return id.Source() == catalog.USACO }
	unknown := map[string]int{}
	var dropped []string
	var failures, lostCode int
	var modsWithUSACO, fullUSACOUnits, fullUnits int
	raw := map[catalog.Source]int{}       // entries, counting repeats
	distinct := map[catalog.ID]bool{}     // across the guide
	perModule := map[catalog.Source]int{} // distinct within each module, summed
	external := 0
	for _, m := range c.Modules() {
		md, st, err := c.convert(m)
		if err != nil {
			failures++
			t.Errorf("convert %s: %v", m.ID, err)
			continue
		}
		for k, v := range st.unknown {
			unknown[k] += v
		}
		dropped = append(dropped, st.dropped...)
		lostCode += checkCode(t, m, md)

		seen := map[catalog.ID]bool{}
		for _, l := range m.Problems {
			for _, p := range l.Problems {
				if p.ID == "" {
					external++
					continue
				}
				src := p.ID.Source()
				raw[src]++
				distinct[p.ID] = true
				if !seen[p.ID] {
					perModule[src]++
				}
				seen[p.ID] = true
			}
		}
		if u := TrialFor(m, isUSACO).Unit; len(u) > 0 {
			modsWithUSACO++
			if len(u) == DefaultUnitSize {
				fullUSACOUnits++
			}
		}
		if len(TrialFor(m, nil).Unit) == DefaultUnitSize {
			fullUnits++
		}
	}
	if lostCode > 0 {
		t.Errorf("%d C++ code blocks missing from the output", lostCode)
	}
	// The pages about the guide itself, as its prerequisites mark them
	// (markMeta): Using This Guide and the Contributing chapter, no more.
	var meta []string
	for _, m := range c.Modules() {
		if m.Meta {
			meta = append(meta, m.ID)
		}
	}
	if got, want := strings.Join(meta, ","), "using-this-guide,contributing,adding-solution,modules,working-mdx"; got != want {
		t.Errorf("meta modules = %s, want %s", got, want)
	}
	if len(unknown) > 0 {
		keys := make([]string, 0, len(unknown))
		for k := range unknown {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		t.Errorf("elements with no rule, rendered as inner text: %v", keys)
	}
	count := func(src catalog.Source) (n int) {
		for id := range distinct {
			if id.Source() == src {
				n++
			}
		}
		return n
	}
	t.Logf("divisions=%d modules=%d conversion failures=%d", len(c.Divisions), len(c.Modules()), failures)
	t.Logf("USACO refs: %d entries, %d distinct per module (summed), %d distinct problems",
		raw[catalog.USACO], perModule[catalog.USACO], count(catalog.USACO))
	t.Logf("CSES refs: %d entries, %d distinct per module (summed), %d distinct problems",
		raw[catalog.CSES], perModule[catalog.CSES], count(catalog.CSES))
	t.Logf("external refs (not playable in v1): %d", external)
	t.Logf("modules with a playable USACO problem=%d; full %d-door units: USACO only=%d, USACO+CSES=%d",
		modsWithUSACO, DefaultUnitSize, fullUSACOUnits, fullUnits)
	t.Logf("dropped {expressions}: %d %q", len(dropped), dropped)
}

var htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)

var cppFence = regexp.MustCompile("(?m)^[ \t]*(`{3,})cpp[ \t]*$")

// checkCode is independent of the scanner: it finds ```cpp blocks in the
// source with a line regex and checks each body (whitespace-insensitive,
// since bodies are re-indented) appears in the output. C++ is the kept
// language, so none may vanish.
func checkCode(t *testing.T, m Module, md string) int {
	t.Helper()
	b, err := os.ReadFile(m.Path)
	if err != nil {
		t.Fatal(err)
	}
	// Blockquote markers are stripped from both sides: callouts and
	// spoilers quote their code.
	squash := func(s string) string {
		lines := strings.Split(s, "\n")
		for i, l := range lines {
			l = strings.TrimLeft(l, " \t")
			for strings.HasPrefix(l, ">") {
				l = strings.TrimLeft(l[1:], " \t")
			}
			lines[i] = l
		}
		return strings.Join(strings.Fields(strings.Join(lines, "\n")), "")
	}
	out := squash(md)
	// Commented-out source is not shown on the site either.
	src := htmlComment.ReplaceAllString(string(b), "")
	lost := 0
	for _, loc := range cppFence.FindAllStringSubmatchIndex(src, -1) {
		fence := src[loc[2]:loc[3]]
		rest := src[loc[1]:]
		end := regexp.MustCompile("(?m)^[ \t]*" + fence + "[ \t]*$").FindStringIndex(rest)
		if end == nil {
			continue
		}
		body := squash(rest[:end[0]])
		if !strings.Contains(out, body) {
			lost++
			t.Errorf("%s: C++ block at byte %d missing from output", m.ID, loc[0])
		}
	}
	return lost
}
