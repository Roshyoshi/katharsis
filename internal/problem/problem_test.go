package problem_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"katharsis/internal/problem"
)

// mkdir builds dir/<name> with the given files (path relative to it -> body)
// and returns its path.
func mkdir(t *testing.T, name string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadSource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		files   []string
		want    string // base name of the chosen source
		wantErr string // substring; "" means no error
	}{
		{"dirname wins over sol and others", []string{"milk2.cpp", "sol.cpp", "brute.cpp"}, "milk2.cpp", ""},
		{"sol wins over others", []string{"sol.cpp", "brute.cpp", "gen.cpp"}, "sol.cpp", ""},
		{"only cpp", []string{"main.cpp"}, "main.cpp", ""},
		{"several cpp is an error", []string{"a.cpp", "b.cpp"}, "", "several .cpp"},
		{"no cpp is an error", []string{"notes.txt"}, "", "no .cpp"},
		{"non-cpp files ignored", []string{"main.cpp", "main.py", "x.cc"}, "main.cpp", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := map[string]string{}
			for _, f := range tc.files {
				files[f] = "int main() {}\n"
			}
			dir := mkdir(t, "milk2", files)
			p, err := problem.Load(dir)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Load: err = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := filepath.Base(p.Source); got != tc.want {
				t.Errorf("Source = %s, want %s", got, tc.want)
			}
			if !filepath.IsAbs(p.Source) {
				t.Errorf("Source %q is not absolute", p.Source)
			}
		})
	}
}

func TestLoadTests(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		files   []string // under tests/
		want    []string // test names in order
		wantErr string
	}{
		{"natural order", []string{"10.in", "10.out", "2.in", "2.out", "1.in", "1.out"}, []string{"1", "2", "10"}, ""},
		{"numeric before named", []string{"b.in", "b.out", "11.in", "11.out", "a.in", "a.out", "3.in", "3.out"}, []string{"3", "11", "a", "b"}, ""},
		{"equal numbers by name", []string{"1.in", "1.out", "01.in", "01.out"}, []string{"01", "1"}, ""},
		{"a dir named .in is not a test", []string{"x.in/keep", "1.in", "1.out"}, []string{"1"}, ""},
		{"missing out is an error", []string{"1.in", "1.out", "2.in"}, nil, "missing 2.out"},
		{"empty tests dir", nil, nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := map[string]string{"sol.cpp": "int main() {}\n"}
			for _, f := range tc.files {
				files[filepath.Join("tests", f)] = f + "\n"
			}
			dir := mkdir(t, "p", files)
			if len(tc.files) == 0 {
				if err := os.MkdirAll(filepath.Join(dir, "tests"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			p, err := problem.Load(dir)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Load: err = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			var names []string
			for _, tt := range p.Tests {
				names = append(names, tt.Name)
				if want := filepath.Join(dir, "tests", tt.Name+".in"); tt.In != want {
					t.Errorf("test %s: In = %s, want %s", tt.Name, tt.In, want)
				}
				if want := filepath.Join(dir, "tests", tt.Name+".out"); tt.Out != want {
					t.Errorf("test %s: Out = %s, want %s", tt.Name, tt.Out, want)
				}
			}
			if !slices.Equal(names, tc.want) {
				t.Errorf("tests = %v, want %v", names, tc.want)
			}
		})
	}
}

func TestLoadNoTestsDir(t *testing.T) {
	t.Parallel()
	dir := mkdir(t, "fresh", map[string]string{"fresh.cpp": "int main() {}\n"})
	p, err := problem.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(p.Tests) != 0 {
		t.Errorf("Tests = %v, want none", p.Tests)
	}
}

func TestLoadMetadata(t *testing.T) {
	t.Parallel()
	dir := mkdir(t, "milk2", map[string]string{"milk2.cpp": "int main() {}\n"})
	// A trailing slash must not change the name used for <dirname>.cpp.
	p, err := problem.Load(dir + string(filepath.Separator))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Dir != dir {
		t.Errorf("Dir = %s, want %s", p.Dir, dir)
	}
	if p.Name != "milk2" {
		t.Errorf("Name = %s, want milk2", p.Name)
	}
	if p.TimeLim != problem.DefaultTimeLimit || p.MemLimit != problem.DefaultMemLimit {
		t.Errorf("limits = %v, %d; want defaults %v, %d",
			p.TimeLim, p.MemLimit, problem.DefaultTimeLimit, problem.DefaultMemLimit)
	}
}

// A dir whose name is a glob pattern used to find no tests at all, since
// "p[1]/tests/*.in" only matches files under "p1/tests".
func TestLoadPatternNamedDir(t *testing.T) {
	t.Parallel()
	dir := mkdir(t, "p[1]", map[string]string{
		"main.cpp": "int main() {}\n", "tests/1.in": "1\n", "tests/1.out": "1\n",
	})
	p, err := problem.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if filepath.Base(p.Source) != "main.cpp" {
		t.Errorf("Source = %s, want main.cpp", p.Source)
	}
	if len(p.Tests) != 1 || p.Tests[0].In != filepath.Join(dir, "tests", "1.in") {
		t.Errorf("Tests = %+v, want tests/1.in", p.Tests)
	}
}

func TestLoadNotADirectory(t *testing.T) {
	t.Parallel()
	dir := mkdir(t, "p", map[string]string{"sol.cpp": "int main() {}\n"})
	for _, path := range []string{filepath.Join(dir, "sol.cpp"), filepath.Join(dir, "missing")} {
		if _, err := problem.Load(path); err == nil {
			t.Errorf("Load(%s): want error", path)
		}
	}
}

func TestFileIO(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"usaco freopen", `freopen("milk2.in", "r", stdin);`, "milk2"},
		{"std qualified, loose spacing", `std::freopen ( "ride.in" , "r", stdin);`, "ride"},
		{"no spaces", `freopen("a.in","r",stdin);`, "a"},
		{"after code on the same line", `ios::sync_with_stdio(0); freopen("a.in", "r", stdin);`, "a"},
		{"dotted stem", `freopen("p.v2.in", "r", stdin);`, "p.v2"},
		{"commented out", `// freopen("milk2.in", "r", stdin);`, ""},
		{"commented out, indented, no space", "\t//freopen(\"milk2.in\", \"r\", stdin);", ""},
		{"comment after code", `int n; // freopen("milk2.in", "r", stdin);`, ""},
		{"output only", `freopen("milk2.out", "w", stdout);`, "milk2"},
		{"path is not a stem", `freopen("../milk2.in", "r", stdin);`, ""},
		{"stdin program", "int n;\nstd::cin >> n;\nstd::cout << n << '\\n';\n", ""},
		{"first live one wins",
			"// freopen(\"old.in\", \"r\", stdin);\nfreopen(\"new.in\", \"r\", stdin);\nfreopen(\"new.out\", \"w\", stdout);\n",
			"new"},
		{"user template", `    freopen("problem.in", "r", stdin);` + "\n" + `    freopen("problem.out", "w", stdout);`, "problem"},
		{"block comment", "/* old version:\n   freopen(\"milk2.in\", \"r\", stdin);\n*/\nint n;", ""},
		{"block comment before code", `/* x */ freopen("a.in", "r", stdin);`, "a"},
		{"comment marker in a string", `puts("//"); freopen("a.in", "r", stdin);`, "a"},
		{"fopen", `FILE *f = fopen("paint.in", "r");`, "paint"},
		{"fstreams", "ifstream fin(\"paint.in\");\nofstream fout(\"paint.out\");", "paint"},
		{"fstream braces", `std::ifstream in{"paint.in"};`, "paint"},
		{"open", "ifstream fin;\nfin.open(\"paint.in\");", "paint"},
		{"usaco guide setIO", "void setIO(string name = \"\") {\n" +
			"\tfreopen((name + \".in\").c_str(), \"r\", stdin);\n" +
			"\tfreopen((name + \".out\").c_str(), \"w\", stdout);\n}\n" +
			"int main() { setIO(\"milk2\"); }", "milk2"},
		{"define", "#define TASK \"milk2\"\nint main() { freopen(TASK \".in\", \"r\", stdin); }", "milk2"},
		{"string variable", "string name = \"milk2\";\nfreopen((name + \".in\").c_str(), \"r\", stdin);", "milk2"},
		{"setIO with no stem is stdio", "void setIO(string s = \"\") {}\nint main() { setIO(); }", ""},
		{"names without an extension are not stems", `puts("hello"); printFile("x");`, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := "#include <bits/stdc++.h>\nusing namespace std;\nint main() {\n" + tc.src + "\n}\n"
			if got := problem.FileIO([]byte(src)); got != tc.want {
				t.Errorf("FileIO = %q, want %q", got, tc.want)
			}
		})
	}
}
