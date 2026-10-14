package learn

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
	if ref := c.Divisions[2].Chapters[0].Modules[0]; ref.ID != "prefix-sums" || ref.Title != "Introduction to Prefix Sums" {
		t.Errorf("silver ref = %+v", ref)
	}

	m, ok := c.Module("prefix-sums")
	if !ok {
		t.Fatal("prefix-sums missing")
	}
	if m.Division != "silver" || m.DivisionTitle != "Silver" || m.Chapter != "Prefix Sums" ||
		m.URL != "https://usaco.guide/silver/prefix-sums" || m.Topic != "prefix-sums" ||
		m.Frequency != 4 || strings.Join(m.Authors, "|") != "Darren Yao|Dustin Miao" ||
		m.Description != "Computing range sum queries in constant time over a fixed 1D array." {
		t.Errorf("module = %+v", m)
	}
	l, ok := m.List("general")
	if !ok || l.Problems[0].ID != "usaco:572" {
		t.Errorf("general list = %+v", l)
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
			again, _ := m.Markdown()
			if again != got {
				t.Error("Markdown is not stable across calls")
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
