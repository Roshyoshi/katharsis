package judge

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

var (
	// sanitizerReport is the start of a sanitizer's report: ASan's
	// "==123==ERROR: AddressSanitizer: ..." or UBSan's "...: runtime
	// error: ...".
	sanitizerReport = regexp.MustCompile(`ERROR: \w+Sanitizer: |: runtime error: `)

	// "SUMMARY: AddressSanitizer: heap-buffer-overflow /w/sol.cpp:7:12 in main"
	summaryRE = regexp.MustCompile(`^SUMMARY: (?:\w+Sanitizer: )?(\S+)\s*(.*)$`)
	// "==1==ERROR: AddressSanitizer: stack-overflow on address 0x..."
	asanErrorRE = regexp.MustCompile(`ERROR: \w+Sanitizer: (\S+)`)
	// "/w/sol.cpp:5:13: runtime error: division by zero"
	runtimeErrorRE = regexp.MustCompile(`^(.*?): runtime error: (.+)$`)
	// "#0 0x100003f2c in main sol.cpp:7:12" or "... in f(int) /w/sol.cpp:7"
	frameRE = regexp.MustCompile(`^#\d+ 0x[0-9a-fA-F]+ in .* (\S+?):(\d+)(?::\d+)?$`)
	// "/w/sol.cpp:7:12" or "sol.cpp:7"
	locationRE = regexp.MustCompile(`^(\S+?):(\d+)(?::\d+)?$`)
	// "libc++abi: terminating due to uncaught exception of type
	// std::runtime_error: boom"
	uncaughtRE = regexp.MustCompile(`libc\+\+abi: terminating due to uncaught exception of type (.+)$`)
	// macOS: "Assertion failed: (x != 7), function main, file sol.cpp, line 6."
	appleAssertRE = regexp.MustCompile(`Assertion failed: \((.*)\), function .*, file (.+), line (\d+)\.$`)
	// glibc: "sol: sol.cpp:6: int main(): Assertion `x != 7' failed."
	glibcAssertRE = regexp.MustCompile(`: (\S+):(\d+): .*: Assertion [` + "`" + `'](.*)' failed\.$`)
)

// crashReason is the one line the HUD shows for a crash: what happened and,
// when known, where, as "<what> at <file>:<line>". A sanitizer's or the C++
// runtime's own words beat the raw exit status, since "exit 1" says nothing
// about a heap overflow; the location is the first stack frame in the
// player's code, which for an overflow inside std::vector is the caller.
func crashReason(stderr string, ps *os.ProcessState, p Profile) string {
	var ub, ubAt, summary, summaryAt, asanErr, hardening, uncaught, assertion, frame string
	for line := range strings.Lines(stderr) {
		line = strings.TrimSpace(line)
		if m := frameRE.FindStringSubmatch(line); m != nil && frame == "" && userCode(m[1]) {
			frame = filepath.Base(m[1]) + ":" + m[2]
		}
		switch m := (match{line: line}); {
		case ub == "" && m.of(runtimeErrorRE):
			ub, ubAt = m.sub[2], location(m.sub[1])
		case summary == "" && m.of(summaryRE):
			summary = m.sub[1]
			if f := strings.Fields(m.sub[2]); len(f) > 0 {
				summaryAt = location(f[0])
			}
		case asanErr == "" && m.of(asanErrorRE):
			asanErr = m.sub[1]
		case hardening == "" && strings.Contains(line, "libc++ Hardening assertion "):
			_, s, _ := strings.Cut(line, "libc++ Hardening assertion ")
			if _, msg, ok := strings.Cut(s, " failed: "); ok {
				s = msg
			}
			hardening = s
		case uncaught == "" && m.of(uncaughtRE):
			uncaught = m.sub[1]
		case assertion == "" && m.of(appleAssertRE):
			assertion = at("assertion "+m.sub[1]+" failed", filepath.Base(m.sub[2])+":"+m.sub[3])
		case assertion == "" && m.of(glibcAssertRE):
			assertion = at("assertion "+m.sub[3]+" failed", filepath.Base(m.sub[1])+":"+m.sub[2])
		}
	}
	switch {
	case ub != "":
		return at(ub, or(ubAt, frame))
	case summary != "" && summary != "undefined-behavior":
		return at(summary, or(frame, summaryAt))
	case asanErr != "":
		return at(asanErr, frame)
	case hardening != "":
		return at("libc++: "+hardening, frame)
	case uncaught != "":
		return "uncaught " + uncaught
	case assertion != "":
		return assertion
	}
	if ps == nil {
		return "killed"
	}
	ws, ok := ps.Sys().(syscall.WaitStatus)
	switch {
	case ok && ws.Signaled() && p.trapsDivision() && (ws.Signal() == syscall.SIGTRAP || ws.Signal() == syscall.SIGILL):
		// The only trap in the build is the division check (arm64 brk,
		// x86 ud2).
		return "integer division by zero"
	case ok && ws.Signaled():
		return ws.Signal().String()
	}
	return fmt.Sprintf("exit %d", ps.ExitCode())
}

// match is a line and the submatches of the last regexp it matched.
type match struct {
	line string
	sub  []string
}

func (m *match) of(re *regexp.Regexp) bool {
	m.sub = re.FindStringSubmatch(m.line)
	return m.sub != nil
}

// location is "sol.cpp:7" for "/w/sol.cpp:7:12" in the player's code, ""
// for anything else (a libc++ header, "<unknown>").
func location(s string) string {
	m := locationRE.FindStringSubmatch(s)
	if m == nil || !userCode(m[1]) {
		return ""
	}
	return filepath.Base(m[1]) + ":" + m[2]
}

// userCode reports whether a source path is the player's: a C++ source
// file, not a header of the standard library or a sanitizer runtime.
func userCode(path string) bool {
	switch filepath.Ext(path) {
	case ".cpp", ".cc", ".cxx", ".c++", ".C", ".c":
		return !strings.Contains(path, "compiler-rt") && !strings.Contains(path, "/c++/v1/")
	}
	return false
}

func at(what, where string) string {
	if where == "" {
		return what
	}
	return what + " at " + where
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
