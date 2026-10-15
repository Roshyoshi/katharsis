package curriculum

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/golden from current output")

func loadFixture(t *testing.T) *Curriculum {
	t.Helper()
	c, err := Load("testdata/guide")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestLoadFixture(t *testing.T) {
	c := loadFixture(t)
	if len(c.Divisions) != 6 {
		t.Fatalf("divisions = %d", len(c.Divisions))
	}
	var ids []string
	for _, m := range c.Modules() {
		ids = append(ids, m.ID)
	}
	want := "fast-io,casework,prefix-sums,two-pointers,hashmaps,matrix-expo,count-min"
	if got := strings.Join(ids, ","); got != want {
		t.Errorf("modules = %s, want %s", got, want)
	}
	if ref := c.Divisions[2].Chapters[0].Modules[0]; ref.ID != "prefix-sums" {
		t.Errorf("silver ref = %+v", ref)
	}

	m, ok := c.Module("prefix-sums")
	if !ok {
		t.Fatal("prefix-sums missing")
	}
	if m.Title != "Introduction to Prefix Sums" ||
		m.Division != "silver" || m.DivisionTitle != "Silver" || m.Chapter != "Prefix Sums" ||
		m.URL != "https://usaco.guide/silver/prefix-sums" || m.Topic != "prefix-sums" ||
		m.Frequency != 4 || strings.Join(m.Authors, "|") != "Darren Yao|Dustin Miao" ||
		m.Description != "Computing range sum queries in constant time over a fixed 1D array." {
		t.Errorf("module = %+v", m)
	}
	if len(m.Problems) != 2 || m.Problems[1].Key != "general" || m.Problems[1].Problems[0].ID != "usaco:572" {
		t.Errorf("problem lists = %+v", m.Problems)
	}
	if f, _ := c.Module("fast-io"); f.Topic != "" || f.Division != "general" {
		t.Errorf("fast-io topic=%q division=%q", f.Topic, f.Division)
	}
	if _, ok := c.Module("nope"); ok {
		t.Error("unknown module found")
	}
}

// TestGolden pins the full conversion of the fixture modules; run with
// -update after an intended change and review the diff.
func TestGolden(t *testing.T) {
	c := loadFixture(t)
	for _, m := range c.Modules() {
		t.Run(m.ID, func(t *testing.T) {
			got, err := m.Markdown()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", "golden", m.ID+".md")
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run with -update to create)", err)
			}
			if got != string(want) {
				t.Errorf("output differs from %s; rerun with -update and review the diff", path)
			}
			if lost := checkCode(t, m, got); lost > 0 {
				t.Errorf("%d C++ blocks lost", lost)
			}
		})
	}
}

func TestAttribution(t *testing.T) {
	a := Attribution()
	for _, s := range []string{"USACO Guide", "https://usaco.guide", "CC BY-NC-SA 4.0", "creativecommons.org/licenses/by-nc-sa/4.0", "Competitive Programming Initiative"} {
		if !strings.Contains(a, s) {
			t.Errorf("Attribution() lacks %q", s)
		}
	}
	m, _ := loadFixture(t).Module("casework")
	md, err := m.Markdown()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "CC BY-NC-SA 4.0") || !strings.Contains(md, m.URL) {
		t.Error("module markdown lacks its attribution footer")
	}
}

func writeGuide(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const tinyOrdering = "const MODULE_ORDERING = { bronze: [{ name: 'A', items: ['m1'] }] };\n"

func TestLoadAcceptsContentDir(t *testing.T) {
	dir := writeGuide(t, map[string]string{
		"content/ordering.ts":       tinyOrdering,
		"content/2_Bronze/M1.mdx":   "---\nid: m1\ntitle: One\nauthor: A\ndescription: d\n---\n\nHi.\n",
		"content/2_Bronze/Stray.md": "not a module",
	})
	for _, d := range []string{dir, filepath.Join(dir, "content")} {
		c, err := Load(d)
		if err != nil {
			t.Fatalf("Load(%s): %v", d, err)
		}
		m, ok := c.Module("m1")
		if !ok || m.Frequency != -1 || len(m.Problems) != 0 {
			t.Errorf("m1 = %+v", m)
		}
		if md, err := m.Markdown(); err != nil || !strings.Contains(md, "Hi.") {
			t.Errorf("markdown = %q, %v", md, err)
		}
	}
}

func TestLoadErrors(t *testing.T) {
	mod := "---\nid: m1\ntitle: One\nauthor: A\ndescription: d\n---\n\nHi.\n"
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"no ordering", map[string]string{"content/x.mdx": mod}, "has no content/ordering.ts"},
		{"bad ordering", map[string]string{"content/ordering.ts": "const MODULE_ORDERING = {\n  bronze: [oops],\n};"}, "ordering.ts:2: unexpected"},
		{"missing module", map[string]string{"content/ordering.ts": tinyOrdering}, `lists module "m1" but no .mdx has that id`},
		{"duplicate id", map[string]string{"content/ordering.ts": tinyOrdering, "content/a.mdx": mod, "content/b.mdx": mod}, `module id "m1" also used by`},
		{"bad frontmatter", map[string]string{"content/ordering.ts": tinyOrdering, "content/a.mdx": "---\nid: m1\n  junk\n"}, "unterminated frontmatter"},
		{"bad problems", map[string]string{"content/ordering.ts": tinyOrdering, "content/a.mdx": mod, "content/a.problems.json": `{"MODULE_ID": "m1", "g": [{}]}`}, "has no uniqueId"},
		{"bad frequency", map[string]string{"content/ordering.ts": tinyOrdering, "content/a.mdx": strings.Replace(mod, "author: A", "author: A\nfrequency: often", 1)}, `frequency "often"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(writeGuide(t, tc.files))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestMarkdownConversionErrorIsReported(t *testing.T) {
	dir := writeGuide(t, map[string]string{
		"content/ordering.ts": tinyOrdering,
		"content/a.mdx":       "---\nid: m1\ntitle: One\nauthor: A\ndescription: d\n---\n\n<Spoiler title=\"x\">\n\nnever closed\n",
	})
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := c.Module("m1")
	if _, err := m.Markdown(); err == nil || !strings.Contains(err.Error(), "content/a.mdx:8: <Spoiler> is never closed") {
		t.Errorf("err = %v", err)
	}
	var zero Module
	if _, err := zero.Markdown(); err == nil {
		t.Error("zero Module should not render")
	}
}

// Doors prefetch from several goroutines; the lazy cache must hand every
// caller the same result.
func TestMarkdownConcurrent(t *testing.T) {
	m, _ := loadFixture(t).Module("prefix-sums")
	want, err := m.Markdown()
	if err != nil {
		t.Fatal(err)
	}
	c := loadFixture(t)
	m2, _ := c.Module("prefix-sums")
	done := make(chan string)
	for range 8 {
		go func() {
			md, _ := m2.Markdown()
			done <- md
		}()
	}
	for range 8 {
		if got := <-done; got != want {
			t.Fatal("concurrent Markdown differs")
		}
	}
}

// metaGuide is a guide whose General section opens on the guide's page
// about itself and a chapter on writing for it, as the real one does:
// Introducing Modules needs Using This Guide, and the contributor pages
// need it in turn. Expected Knowledge needs a meta page and a real one.
func metaGuide(t *testing.T) string {
	t.Helper()
	mod := func(id, title string, prereqs ...string) string {
		s := "---\nid: " + id + "\ntitle: " + title + "\nauthor: A\ndescription: d\n"
		if len(prereqs) > 0 {
			s += "prerequisites:\n  - " + strings.Join(prereqs, "\n  - ") + "\n"
		}
		return s + "---\n\nHi.\n"
	}
	return writeGuide(t, map[string]string{
		"content/ordering.ts": `const MODULE_ORDERING = {
  general: [
    { name: 'Getting Started', items: ['using-this-guide', 'intro-cp', 'expected-knowledge'] },
    { name: 'Contributing', items: ['contributing', 'modules', 'working-mdx'] },
  ],
  bronze: [{ name: 'Simulation', items: ['intro-sim'] }],
};
`,
		"content/1_General/Using_This_Guide.mdx":   mod("using-this-guide", "Using This Guide"),
		"content/1_General/Intro_CP.mdx":           mod("intro-cp", "Introduction to Competitive Programming"),
		"content/1_General/Expected_Knowledge.mdx": mod("expected-knowledge", "Expected Knowledge", "using-this-guide", "intro-cp"),
		"content/1_General/Contributing.mdx":       mod("contributing", "Contributing", "modules"),
		"content/1_General/Modules.mdx":            mod("modules", "Introducing Modules", "using-this-guide"),
		"content/1_General/Working_MDX.mdx":        mod("working-mdx", "Working With MDX", "modules"),
		"content/2_Bronze/Intro_Sim.mdx":           mod("intro-sim", "Simulation", "intro-cp"),
	})
}

// The guide's pages about itself are Meta, as the guide's own
// prerequisites say: Using This Guide and every page that needs only
// meta pages. Nothing else is, even with a meta page among its
// prerequisites.
func TestMetaModules(t *testing.T) {
	c, err := Load(metaGuide(t))
	if err != nil {
		t.Fatal(err)
	}
	var meta []string
	for _, m := range c.Modules() {
		if m.Meta {
			meta = append(meta, m.ID)
		}
	}
	if got, want := strings.Join(meta, ","), "using-this-guide,contributing,modules,working-mdx"; got != want {
		t.Errorf("meta modules = %s, want %s", got, want)
	}
}
