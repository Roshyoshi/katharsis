package learn

import (
	"strings"
	"testing"
)

// The real ordering.ts at the pinned snapshot (testdata/ordering_full.ts).
func TestParseOrderingFull(t *testing.T) {
	divs, err := ParseOrdering("testdata/ordering_full.ts")
	if err != nil {
		t.Fatal(err)
	}
	var ids, titles []string
	modules := 0
	for _, d := range divs {
		ids = append(ids, d.ID)
		titles = append(titles, d.Title)
		for _, c := range d.Chapters {
			modules += len(c.Modules)
		}
	}
	if got := strings.Join(ids, ","); got != "general,bronze,silver,gold,plat,adv" {
		t.Errorf("division ids = %s", got)
	}
	if got := strings.Join(titles, ","); got != "General,Bronze,Silver,Gold,Platinum,Advanced" {
		t.Errorf("division titles = %s", got)
	}
	if modules != 149 {
		t.Errorf("modules = %d, want 149", modules)
	}
	silver := divs[2].Chapters[0]
	if silver.Title != "Prefix Sums" || len(silver.Modules) != 2 ||
		silver.Modules[0].ID != "prefix-sums" || silver.Modules[1].ID != "more-prefix-sums" {
		t.Errorf("silver chapter 0 = %+v", silver)
	}
	if d := divs[0].Chapters[0].Description; d != "Start here if you're new to competitive programming." {
		t.Errorf("general chapter 0 description = %q", d)
	}
	last := divs[5].Chapters[len(divs[5].Chapters)-1]
	if id := last.Modules[len(last.Modules)-1].ID; id != "vectorization" {
		t.Errorf("last module = %s", id)
	}
}

func TestParseOrderingTolerates(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      string // div:title:chapter=ids;... summary
	}{
		{
			name: "no type annotation, double quotes, trailing commas, comments",
			src: `/* header */
const MODULE_ORDERING = {
  // the only division
  "bronze": [ { name: "A", items: ["x", 'y',], }, ],
};`,
			want: "bronze:Bronze:A=x,y",
		},
		{
			name: "export, as const, unknown chapter keys, labels",
			src: `export const MODULE_ORDERING: Record<string, Chapter[]> = {
  plat: [{ name: 'B', items: ['z'], hidden: true, weight: 2 }],
} as const;
export const SECTION_LABELS = { plat: 'Plat!' };`,
			want: "plat:Plat!:B=z",
		},
		{
			name: "arrow types in annotation, code after",
			src: `const MODULE_ORDERING: { [k: string]: (() => void) | Chapter[] } = {
  adv: [{ name: 'C', description: 'd', items: [] }],
};
const f = (x) => ` + "`/${x}/${`nested ${x}`}`" + `;
const re = a / b;`,
			want: "adv:Advanced:C=",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			divs, err := parseOrdering("x.ts", tc.src)
			if err != nil {
				t.Fatal(err)
			}
			var parts []string
			for _, d := range divs {
				for _, c := range d.Chapters {
					var ids []string
					for _, m := range c.Modules {
						ids = append(ids, m.ID)
					}
					parts = append(parts, d.ID+":"+d.Title+":"+c.Title+"="+strings.Join(ids, ","))
				}
			}
			if got := strings.Join(parts, ";"); got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestParseOrderingErrors(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string
	}{
		{"missing declaration", "const OTHER = {};", "x.ts: no `const MODULE_ORDERING"},
		{"unterminated string", "const MODULE_ORDERING = {\n  bronze: [{ name: 'A,\n", "x.ts:2: unterminated string literal"},
		{"unterminated comment", "/* oops\n\nconst MODULE_ORDERING = {};", "x.ts:1: unterminated block comment"},
		{"spread", "const MODULE_ORDERING = {\n  ...BASE,\n};", `x.ts:2: unexpected "...", want an object key`},
		{"call", "const MODULE_ORDERING = {\n  bronze: build(),\n};", `x.ts:2: unexpected "build", want a constant literal`},
		{"missing comma", "const MODULE_ORDERING = {\n  bronze: []\n  silver: []\n};", `x.ts:3: unexpected "silver", want "," or "}"`},
		{"template substitution", "const MODULE_ORDERING = {\n  bronze: [{ name: `a${b}`, items: [] }],\n};", "x.ts:2: template literal with substitutions"},
		{"no items", "const MODULE_ORDERING = {\n  bronze: [{\n    name: 'A',\n  }],\n};", `x.ts:3: chapter "A" has no items array`},
		{"number item", "const MODULE_ORDERING = {\n  bronze: [{ name: 'A',\n    items: [3] }],\n};", `x.ts:3: chapter "A" has a non-string item`},
		{"duplicate module", "const MODULE_ORDERING = {\n  bronze: [{ name: 'A', items: ['m'] }],\n  silver: [{ name: 'B',\n    items: ['m'] }],\n};", `x.ts:4: module "m" listed in both bronze and silver`},
		{"duplicate key", "const MODULE_ORDERING = {\n  bronze: [],\n  bronze: [],\n};", `x.ts:3: duplicate key "bronze"`},
		{"not an object", "const MODULE_ORDERING = [];", "x.ts:1: MODULE_ORDERING is not an object"},
		{"empty", "const MODULE_ORDERING = {};", "x.ts:1: MODULE_ORDERING is empty"},
		{"no initializer", "let MODULE_ORDERING: Foo;", "x.ts:1: declaration has no initializer"},
		{"bad label", "const MODULE_ORDERING = { b: [] };\nconst SECTION_LABELS = {\n  b: 3 };", "x.ts:3: SECTION_LABELS.b is not a string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseOrdering("x.ts", tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
