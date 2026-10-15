package libs

import (
	"reflect"
	"strings"
	"testing"

	"katharsis/internal/game"
)

func TestExtract(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []game.Lib
		err       string
	}{
		{name: "none", src: "int main() {}\n"},
		{
			name: "one",
			src:  "#include <vector>\n// lib: dsu\nstruct DSU {};\n// end lib\nint main() {}\n",
			want: []game.Lib{{Name: "dsu", Source: "struct DSU {};\n"}},
		},
		{
			name: "two, spacing tolerated",
			src:  "  //lib:  seg-tree \nint a;\n  // end lib\n//lib: bit\nint b;\n\n//end lib\n",
			want: []game.Lib{{Name: "seg-tree", Source: "int a;\n"}, {Name: "bit", Source: "int b;\n"}},
		},
		{
			name: "later body wins",
			src:  "// lib: x\nold\n// end lib\n// lib: x\nnew\n// end lib\n",
			want: []game.Lib{{Name: "x", Source: "new\n"}},
		},
		{name: "unterminated", src: "a\n// lib: dsu\nb\n", err: "line 2: lib \"dsu\" has no // end lib"},
		{name: "nested", src: "// lib: a\n// lib: b\n// end lib\n", err: "line 2: lib \"b\" starts inside lib \"a\""},
		{name: "stray end", src: "x\n// end lib\n", err: "line 2: end lib without a lib"},
		{name: "marker in code is not a marker", src: "int x; // lib: nope\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Extract(tc.src)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("err = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestInjectAboveMain(t *testing.T) {
	tmpl := "#include <iostream>\nusing namespace std;\n\nint main() {\n    return 0;\n}\n"
	got := Inject(tmpl, []game.Lib{{Name: "dsu", Source: "struct DSU {};"}, {Name: "bit", Source: "int b;\n"}})
	want := "#include <iostream>\nusing namespace std;\n\n" +
		"// lib: dsu\nstruct DSU {};\n// end lib\n\n// lib: bit\nint b;\n// end lib\n\n" +
		"int main() {\n    return 0;\n}\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	// What was injected can be mined again unchanged.
	libs, err := Extract(got)
	if err != nil || len(libs) != 2 || libs[0].Source != "struct DSU {};\n" {
		t.Fatalf("re-extract = %+v, %v", libs, err)
	}
}

func TestInjectNoMainAppends(t *testing.T) {
	got := Inject("// nothing", []game.Lib{{Name: "x", Source: "int x;\n"}})
	if !strings.HasSuffix(got, "// lib: x\nint x;\n// end lib\n\n") || !strings.HasPrefix(got, "// nothing\n") {
		t.Fatalf("got %q", got)
	}
}

func TestInjectNothing(t *testing.T) {
	if got := Inject("int main(){}", nil); got != "int main(){}" {
		t.Fatalf("got %q", got)
	}
}
