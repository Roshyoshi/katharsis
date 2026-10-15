package judge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestLeadingIncludes(t *testing.T) {
	usaco := "/*\nID: your_id\nPROG: problem\n*/\n\n#include <algorithm>\n#include <array>\n\nusing namespace std;\n"
	for _, tc := range []struct {
		name, src string
		want      []string
	}{
		{"block comment, then includes", usaco, []string{"algorithm", "array"}},
		{"bits", "#include <bits/stdc++.h>\nusing namespace std;\n#include <ext/rope>\n", []string{"bits/stdc++.h"}},
		{"spacing, CRLF, a trailing comment", "#include<vector>\r\n  #  include <map> // maps\r\n#include <set> //\nint x;\n", []string{"vector", "map", "set"}},
		{"line comments and blank lines", "// a\n\n// b\n#include <set>\n\n#include <map>", []string{"set", "map"}},
		{"a comment before, on the line", "/* a */ #include <cstdio>\n", []string{"cstdio"}},
		{"#line, as stripTargetPragmas writes it", "#line 1 \"/w/sol.cpp\"\n\n#include <cstdio>\n", []string{"cstdio"}},
		{"a byte order mark", "\xef\xbb\xbf#include <cstdio>\n", []string{"cstdio"}},
		{"a define ends the list", "#include <cstdio>\n#define N 5\n#include <vector>\n", []string{"cstdio"}},
		{"a quoted include ends it", "#include <cstdio>\n#include \"my.h\"\n#include <vector>\n", []string{"cstdio"}},
		{"a define first", "#define _GLIBCXX_DEBUG\n#include <vector>\n", nil},
		{"#if first", "#ifndef ONLINE_JUDGE\n#include <cstdio>\n#endif\n", nil},
		{"code first", "int x;\n#include <cstdio>\n", nil},
		{"include_next", "#include_next <cstdio>\n", nil},
		{"a macro include", "#include HEADER\n", nil},
		{"a comment continued onto the include", "// x \\\n#include <cstdio>\n", nil},
		{"an include's comment continued onto the next", "#include <cstdio> // \\\n#include <vector>\n", nil},
		{"an unterminated comment", "/* x\n#include <cstdio>\n", nil},
		{"empty", "", nil},
	} {
		for _, gnu := range []bool{true, false} {
			if got := leadingIncludes([]byte(tc.src), gnu); !slices.Equal(got, tc.want) {
				t.Errorf("%s (gnu %v): %q, want %q", tc.name, gnu, got, tc.want)
			}
		}
	}
	// g++ applies an optimize pragma to the headers after it; clang
	// ignores both pragmas.
	cf := "#pragma GCC optimize(\"O3,unroll-loops\")\n#pragma GCC target(\"avx2\")\n#include <bits/stdc++.h>\n"
	if got := leadingIncludes([]byte(cf), true); got != nil {
		t.Errorf("g++, optimize pragma first: %q, want none", got)
	}
	if got := leadingIncludes([]byte(cf), false); !slices.Equal(got, []string{"bits/stdc++.h"}) {
		t.Errorf("clang, pragmas first: %q, want bits/stdc++.h", got)
	}
}

// pchToolchain is Profiles, skipping t unless its sample and full
// profiles both precompile headers.
func pchToolchain(t *testing.T) Toolchain {
	t.Helper()
	if _, err := exec.LookPath(Compiler); err != nil {
		t.Skipf("%s not on PATH", Compiler)
	}
	tc := Profiles(context.Background())
	if !tc.Samples.PCH || !tc.Full.PCH {
		t.Skipf("no PCH here: samples %q, full %q", tc.Samples.Name, tc.Full.Name)
	}
	return tc
}

// pchOf is the PCH a build of src under p loads, as Compile finds it.
func pchOf(t *testing.T, p Profile, src string) pch {
	t.Helper()
	dir, err := cacheDir()
	if err != nil {
		t.Fatal(err)
	}
	inc, err := includeDir()
	if err != nil {
		t.Fatal(err)
	}
	base := slices.Concat(p.Flags, commonFlags(p, inc), []string{"-fdiagnostics-color=never"})
	key := pchKey(p, base, leadingIncludes([]byte(src), p.GNU))
	return pch{dir: filepath.Join(dir, "pch-"+key), gnu: p.GNU}
}

// waitPCH waits for this process's build of c, if one is running.
func waitPCH(c pch) {
	if done, ok := pchBuilding.Load(c.dir); ok {
		<-done.(chan struct{})
	}
}

// compileSrc writes body to dir/name and compiles it.
func compileSrc(t *testing.T, dir, name, body string, p Profile) Build {
	t.Helper()
	src := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Compile(context.Background(), src, p)
	if err != nil {
		t.Fatalf("Compile %s: %v", name, err)
	}
	return b
}

// ready makes the PCH for head under p and fails t if it cannot.
func ready(t *testing.T, dir, head string, p Profile) pch {
	t.Helper()
	c := pchOf(t, p, head)
	// A new source each time: a cached binary skips the compiler, and
	// with it the PCH.
	warmup := fmt.Sprintf("%sint main() {}\n// %d\n", head, time.Now().UnixNano())
	if b := compileSrc(t, dir, "warmup.cpp", warmup, p); !b.OK {
		t.Fatalf("warmup build failed:\n%s", b.Log)
	}
	waitPCH(c)
	if _, err := os.Stat(c.file()); err != nil {
		why, _ := os.ReadFile(c.failed())
		t.Fatalf("no PCH after a build: %v\n%s", err, why)
	}
	return c
}

func runSrc(t *testing.T, b Build, in string, p Profile) Result {
	t.Helper()
	dir := t.TempDir()
	c := Case{In: filepath.Join(dir, "1.in"), Out: filepath.Join(dir, "1.out")}
	if err := os.WriteFile(c.In, []byte(in), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.Out, []byte("3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Run(context.Background(), b.Binary, c, Limits{Time: 5 * time.Second}, p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// A build loads the PCH once one is ready, never before, and judges the
// same with it as without: verdicts, crash locations, compile errors.
func TestPCH(t *testing.T) {
	tc := pchToolchain(t)
	t.Parallel()
	// Headers no other test opens with, so only this test makes the PCH.
	const head = "// TestPCH\n#include <cstdio>\n#include <valarray>\n#include <vector>\n"
	const ab = "int main() { long a, b; if (scanf(\"%ld %ld\", &a, &b) != 2) return 2; printf(\"%ld\\n\", a + b); }\n"
	// v[5] trips libstdc++'s assertion, v.at(5) throws.
	const oob = "int main() { std::vector<int> v(3); int i; if (scanf(\"%d\", &i) != 1) return 2;\n" +
		"    printf(\"%d\\n\", i > 0 ? v[i] : v.at(-i)); }\n"
	const ce = "int main() { undeclared(); }\n"
	for _, p := range []Profile{tc.Samples, tc.Full} {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			c := pchOf(t, p, head)
			cold := compileSrc(t, dir, "cold.cpp", head+ab, p)
			if !cold.OK || cold.PCH {
				t.Fatalf("first build: OK %v, PCH %v; want a plain build\n%s", cold.OK, cold.PCH, cold.Log)
			}
			waitPCH(c)
			warm := compileSrc(t, dir, "warm.cpp", head+ab+"// warm\n", p)
			if !warm.OK || !warm.PCH {
				why, _ := os.ReadFile(c.failed())
				t.Fatalf("second build: OK %v, PCH %v; want the PCH\n%s\nfailed: %s", warm.OK, warm.PCH, warm.Log, why)
			}
			t.Logf("cold %v, warm %v", cold.Duration.Round(time.Millisecond), warm.Duration.Round(time.Millisecond))
			if r := runSrc(t, warm, "1 2\n", p); r.Verdict != AC {
				t.Errorf("warm build: %s %q", r.Verdict, r.Reason)
			}

			// Same file name in two dirs, a comment apart: separate builds
			// whose reports name the same file and lines.
			plain := p
			plain.PCH = false
			with := compileSrc(t, dir, "a/oob.cpp", head+oob, p)
			without := compileSrc(t, dir, "b/oob.cpp", head+oob+"// plain\n", plain)
			if !with.PCH || without.PCH {
				t.Fatalf("PCH %v and %v, want true and false", with.PCH, without.PCH)
			}
			for _, in := range []string{"5\n", "-5\n"} {
				r1, r2 := runSrc(t, with, in, p), runSrc(t, without, in, p)
				if r1.Verdict != r2.Verdict || r1.Reason != r2.Reason {
					t.Errorf("input %q: %s %q with the PCH, %s %q without", in, r1.Verdict, r1.Reason, r2.Verdict, r2.Reason)
				}
			}
			// A target pragma above the includes: g++ builds compile a copy
			// that #line points back at the source, and still load the PCH.
			const pragma = "#pragma GCC target(\"avx2\")\n"
			with = compileSrc(t, dir, "a/ce.cpp", pragma+head+ce, p)
			without = compileSrc(t, dir, "b/ce.cpp", pragma+head+ce+"// plain\n", plain)
			got := strings.ReplaceAll(with.Log, filepath.Join(dir, "a"), "")
			want := strings.ReplaceAll(without.Log, filepath.Join(dir, "b"), "")
			first := firstLine(got, "error")
			if with.OK || !with.PCH || first != firstLine(want, "error") || !strings.Contains(first, "ce.cpp:6:") {
				t.Errorf("compile error with the PCH (OK %v, PCH %v):\n%s\nwithout:\n%s", with.OK, with.PCH, got, want)
			}
		})
	}
}

func firstLine(log, with string) string {
	for l := range strings.Lines(log) {
		if strings.Contains(l, with) {
			return l
		}
	}
	return ""
}

// A PCH the compiler cannot use is dropped and rebuilt, and the build that
// found it out still compiles the source as written, with a log that says
// nothing of it.
func TestPCHStale(t *testing.T) {
	tc := pchToolchain(t)
	t.Parallel()
	const head = "// TestPCHStale\n#include <cstdio>\n#include <deque>\n"
	const body = "int main() { std::deque<int> d{1, 2}; printf(\"%d\\n\", d[0] + d[1]); }\n"
	type spoil struct {
		name string
		do   func(t *testing.T, c pch, p Profile)
	}
	truncate := spoil{"truncated", func(t *testing.T, c pch, _ Profile) {
		b, err := os.ReadFile(c.file())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(c.file(), b[:len(b)/3], 0o644); err != nil {
			t.Fatal(err)
		}
	}}
	// g++ passes over a .gch built with other flags and reads the
	// headers, saying so under -Winvalid-pch only.
	mismatched := spoil{"built with other flags", func(t *testing.T, c pch, p Profile) {
		args := slices.Concat(p.Flags, commonFlags(p, ""), []string{"-std=gnu++20", "-c", "-x", "c++-header", c.header(), "-o", c.file()})
		if log, err := runCompiler(context.Background(), p, "", args); err != nil {
			t.Fatalf("%v\n%s", err, log)
		}
	}}
	// clang refuses a PCH whose headers changed since.
	touched := spoil{"header touched", func(t *testing.T, c pch, _ Profile) {
		later := time.Now().Add(time.Minute)
		if err := os.Chtimes(c.header(), later, later); err != nil {
			t.Fatal(err)
		}
	}}
	for _, p := range []Profile{tc.Samples, tc.Full} {
		spoils := []spoil{truncate, touched}
		if p.GNU {
			spoils = []spoil{truncate, mismatched}
		}
		for i, s := range spoils {
			t.Run(p.Name+"/"+s.name, func(t *testing.T) {
				t.Parallel()
				// Each spoil gets its own header list, so its own PCH.
				head := head + strings.Repeat("#include <deque>\n", i)
				dir := t.TempDir()
				c := ready(t, dir, head, p)
				s.do(t, c, p)
				b := compileSrc(t, dir, "spoiled.cpp", head+body, p)
				if !b.OK || b.PCH {
					t.Fatalf("build on the spoiled PCH: OK %v, PCH %v; want a plain build\n%s", b.OK, b.PCH, b.Log)
				}
				if strings.Contains(b.Log, c.dir) || strings.Contains(b.Log, "precompiled") || strings.Contains(b.Log, "PCH") {
					t.Errorf("log mentions the PCH:\n%s", b.Log)
				}
				if r := runSrc(t, b, "", p); r.Verdict != AC {
					t.Errorf("%s %q", r.Verdict, r.Reason)
				}
				if _, err := os.Stat(c.file()); err == nil {
					t.Errorf("spoiled PCH %s kept", c.file())
				}
				if again := ready(t, dir, head, p); again != c {
					t.Errorf("rebuilt at %s, want %s", again.dir, c.dir)
				}
				if b := compileSrc(t, dir, "again.cpp", head+body+"// again\n", p); !b.PCH {
					t.Errorf("no PCH after the rebuild\n%s", b.Log)
				}
			})
		}
	}
}
