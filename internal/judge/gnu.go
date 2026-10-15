package judge

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// The GCC toolchain (D28; docs/research/gcc-toolchain.md). Apple's libc++
// reads ints through cin ~25x slower than libstdc++ even with
// sync_with_stdio(false): 1e6 ints take 967 ms against 38 ms on an M4, so
// solutions that pass on Codeforces' g++ TLE here. When the flake's g++
// works on this machine, every build uses libstdc++: g++ for what is timed
// and debugged, and clang against the same libstdc++ for ASan, which GCC
// lacks on arm64 macOS. Otherwise everything stays on clang and libc++.

// GXX and ClangLibstdcxx are the commands the GCC profiles run; the flake
// puts both on PATH. clang++-libstdc++ is clang++ with GCC's libstdc++
// headers and dylib in place of libc++.
const (
	GXX            = "g++"
	ClangLibstdcxx = "clang++-libstdc++"
)

var (
	// GCCFull: what the time limit is checked against when g++ works here.
	GCCFull = Profile{
		Name:      "O2 (g++)",
		Flags:     Full.Flags,
		Slack:     1,
		Compiler:  GXX,
		CleanEnv:  true,
		Libstdcxx: true,
		GNU:       true,
	}
	// GCCDebug: Debug's flags through g++, so :Debug runs what :Submit
	// judged, short of optimization.
	GCCDebug = Profile{
		Name:      "debug (g++)",
		Flags:     Debug.Flags,
		Slack:     1,
		Compiler:  GXX,
		CleanEnv:  true,
		Libstdcxx: true,
		GNU:       true,
	}
	// LibstdcxxSamples: ASan + UBSan through clang, against the libstdc++
	// that GCCFull links, so a save compiles what :Submit compiles.
	// libstdc++ tells ASan less than libc++ does: its vector annotations
	// take effect only once reserve() poisons the spare capacity, so a
	// vector grown by push_back or resize reads clean past size(), and
	// string and deque have none. Its assertions catch those indexing
	// bugs instead, std::array's too (crashReason names them).
	LibstdcxxSamples = Profile{
		Name: "asan+ubsan (libstdc++)",
		Flags: slices.Concat(Samples.Flags,
			[]string{"-D_GLIBCXX_SANITIZE_VECTOR", "-D_GLIBCXX_ASSERTIONS"}),
		Slack:     Samples.Slack,
		Sanitized: true,
		Compiler:  ClangLibstdcxx,
		CleanEnv:  true,
		Libstdcxx: true,
	}
	// GCCSamples: the fallback when LibstdcxxSamples' binaries do not run.
	// No ASan; UBSan in trap mode, which needs no runtime (a trap says
	// nothing of where), and libstdc++'s assertions for out-of-range
	// indexing.
	GCCSamples = Profile{
		Name: "ubsan-trap (g++)",
		Flags: []string{
			"-std=gnu++23", "-O1", "-g", "-fno-omit-frame-pointer",
			"-fsanitize=undefined", "-fsanitize-trap=undefined",
			"-D_GLIBCXX_ASSERTIONS", stackFlag,
		},
		Slack:     2,
		Compiler:  GXX,
		CleanEnv:  true,
		Libstdcxx: true,
		GNU:       true,
	}
)

// targetPragma matches a "#pragma GCC target(...)" line. Contest templates
// carry x86 ones (avx2, bmi2, popcnt) for Codeforces; clang ignores them,
// but arm64 g++ rejects each x86 name with a hard error that no flag
// downgrades, so the file would not compile at all.
var targetPragma = regexp.MustCompile(`(?m)^[ \t]*#[ \t]*pragma[ \t]+GCC[ \t]+target\b.*$`)

// stripTargetPragmas blanks src's target pragmas for g++, keeping every
// other line where it was. The #line directive names the original file, so
// diagnostics, debug info and lldb's breakpoints all point at path. ok is
// false when src has none, and then src compiles as it is.
func stripTargetPragmas(src []byte, path string) (out []byte, ok bool) {
	if !targetPragma.Match(src) {
		return nil, false
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	quoted := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(path)
	var b bytes.Buffer
	fmt.Fprintf(&b, "#line 1 \"%s\"\n", quoted)
	b.Write(targetPragma.ReplaceAll(src, nil))
	return b.Bytes(), true
}

// trapsUB reports whether p's builds trap on undefined behavior
// (-fsanitize-trap=undefined), so a SIGTRAP is UBSan's.
func (p Profile) trapsUB() bool {
	return slices.ContainsFunc(p.Flags, func(f string) bool {
		s, ok := strings.CutPrefix(f, "-fsanitize-trap=")
		return ok && strings.Contains(s, "undefined")
	})
}

var (
	// libstdc++: "terminate called after throwing an instance of
	// 'std::runtime_error'", then "  what():  boom".
	gnuUncaughtRE = regexp.MustCompile(`^terminate called after throwing an instance of '(.+)'$`)
	gnuWhatRE     = regexp.MustCompile(`^what\(\):\s+(.*)$`)
	// libstdc++ with _GLIBCXX_ASSERTIONS: "/.../stl_vector.h:1263: ...
	// operator[](size_type) [with ...]: Assertion '__n < this->size()'
	// failed."
	gnuAssertRE = regexp.MustCompile(`^\S+:\d+: .*: Assertion '(.*)' failed\.$`)
)

// probeProgram is what runs() compiles for p: hello world, plus checks that
// a libstdc++ profile really gets libstdc++ and a GNU one really is g++
// (macOS's /usr/bin/g++ is Apple clang). <cstdio> is enough to tell: every
// libstdc++ header defines __GLIBCXX__, and <bits/stdc++.h> would add a
// second to each compile.
func probeProgram(p Profile) string {
	var b strings.Builder
	b.WriteString("#include <cstdio>\n")
	if p.Libstdcxx {
		b.WriteString("#ifndef __GLIBCXX__\n#error not libstdc++\n#endif\n")
	}
	if p.GNU {
		b.WriteString("#ifdef __clang__\n#error clang, not g++\n#endif\n")
	}
	b.WriteString("int main() { std::puts(\"ok\"); }\n")
	return b.String()
}
