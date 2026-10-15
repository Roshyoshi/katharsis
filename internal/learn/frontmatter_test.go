package learn

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	for _, tc := range []struct {
		name    string
		src     string
		scalars map[string]string
		lists   map[string][]string
	}{
		{
			name:    "plain and quoted scalars",
			src:     "id: prefix-sums\ntitle: 'Introduction to Prefix Sums'\ndescription: \"What to do when it doesn't work.\"\nfrequency: 4\n",
			scalars: map[string]string{"id": "prefix-sums", "title": "Introduction to Prefix Sums", "description": "What to do when it doesn't work.", "frequency": "4"},
		},
		{
			name:    "doubled single quote",
			src:     "title: 'Bessie''s Guide'\n",
			scalars: map[string]string{"title": "Bessie's Guide"},
		},
		{
			name:    "folded over indented lines",
			src:     "description:\n  'Computing range sum queries in constant time over a fixed\n  1D array.'\ncontributors:\n  Darren Yao, Sam Zhang,\n  Dustin Miao\n",
			scalars: map[string]string{"description": "Computing range sum queries in constant time over a fixed 1D array.", "contributors": "Darren Yao, Sam Zhang, Dustin Miao"},
		},
		{
			name:    "quoted scalar continuing at column 0 (Complete_Rec.mdx)",
			src:     "description: 'Harder problems,\nincluding subsets.'\nfrequency: 2\n",
			scalars: map[string]string{"description": "Harder problems, including subsets.", "frequency": "2"},
		},
		{
			name:  "indented and compact block lists (Matroid.mdx)",
			src:   "prerequisites:\n  - intro-sorting\n  - binary-search-sorted-array\nredirects:\n- /silver/2P\n- '/silver/two-pointers'\n",
			lists: map[string][]string{"prerequisites": {"intro-sorting", "binary-search-sorted-array"}, "redirects": {"/silver/2P", "/silver/two-pointers"}},
		},
		{
			name:    "dash without a blank starts a scalar, not a list",
			src:     "description:\n  -1 means unset\n",
			scalars: map[string]string{"description": "-1 means unset"},
		},
		{
			name:  "comment lines inside a block list",
			src:   "prerequisites:\n  - a\n  # - b\n  - c\n",
			lists: map[string][]string{"prerequisites": {"a", "c"}},
		},
		{
			name:    "flow list, empty value, comment",
			src:     "# note\nprerequisites: [a, 'b, c']\ncontributors:\nwide: true # layout\n",
			scalars: map[string]string{"contributors": "", "wide": "true"},
			lists:   map[string][]string{"prerequisites": {"a", "b, c"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parseFrontmatter(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			if tc.scalars == nil {
				tc.scalars = map[string]string{}
			}
			if tc.lists == nil {
				tc.lists = map[string][]string{}
			}
			if !reflect.DeepEqual(f.scalars, tc.scalars) {
				t.Errorf("scalars = %q, want %q", f.scalars, tc.scalars)
			}
			if !reflect.DeepEqual(f.lists, tc.lists) {
				t.Errorf("lists = %q, want %q", f.lists, tc.lists)
			}
		})
	}
}

func TestFrontmatterList(t *testing.T) {
	f, err := parseFrontmatter("author: Darren Yao, Dustin Miao\ncontributors:\n  - A\n  - B\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := f.list("author"); !reflect.DeepEqual(got, []string{"Darren Yao", "Dustin Miao"}) {
		t.Errorf("author = %q", got)
	}
	if got := f.list("contributors"); !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Errorf("contributors = %q", got)
	}
	if got := f.list("missing"); got != nil {
		t.Errorf("missing = %q", got)
	}
}

func TestParseFrontmatterErrors(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"indented first line", "  id: x\n", "frontmatter line 2: unexpected indented line"},
		{"no colon", "id: x\njunk\n", "frontmatter line 3: want `key: value`"},
		{"duplicate", "id: x\nid: y\n", `frontmatter line 3: duplicate key "id"`},
		{"unterminated quote", "title: 'abc\n", "unterminated quoted scalar"},
		{"stray quote", "title: 'a'b'\n", "stray quote"},
		{"mixed list", "p:\n  - a\n  b\n", "mixed list and scalar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseFrontmatter(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSplitFrontmatter(t *testing.T) {
	fm, body, line, err := splitFrontmatter("---\nid: x\ntitle: y\n---\n\n# Body\n")
	if err != nil || fm != "id: x\ntitle: y\n" || body != "\n# Body\n" || line != 5 {
		t.Errorf("got fm=%q body=%q line=%d err=%v", fm, body, line, err)
	}
	fm, body, line, err = splitFrontmatter("\ufeff---\r\nid: x\r\n--- \r")
	if err != nil || fm != "id: x\r\n" || body != "" || line != 4 {
		t.Errorf("BOM, CRLF, closing at EOF: got fm=%q body=%q line=%d err=%v", fm, body, line, err)
	}
	if _, _, _, err := splitFrontmatter("# no frontmatter\n"); err == nil {
		t.Error("want error without frontmatter")
	}
	if _, _, _, err := splitFrontmatter("---\nid: x\n"); err == nil {
		t.Error("want error for unterminated frontmatter")
	}
}
