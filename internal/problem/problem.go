// Package problem loads a problem directory: the solution source, its tests,
// and how the program does I/O.
//
// Layout (matches ~/cp/problems/<name>):
//
//	<dir>/<name>.cpp     or sol.cpp, or the only .cpp in <dir>
//	<dir>/tests/1.in     paired with tests/1.out
package problem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Test struct {
	Name string // "1", "2", ...
	In   string // path to input
	Out  string // path to expected output
}

type Problem struct {
	Dir      string
	Name     string
	Source   string
	Tests    []Test
	TimeLim  time.Duration
	MemLimit int64 // bytes
}

const (
	DefaultTimeLimit = 2 * time.Second
	DefaultMemLimit  = 256 << 20
)

func Load(dir string) (*Problem, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("%s: not a directory", dir)
	}
	src, err := findSource(dir)
	if err != nil {
		return nil, err
	}
	tests, err := findTests(filepath.Join(dir, "tests"))
	if err != nil {
		return nil, err
	}
	return &Problem{
		Dir:      dir,
		Name:     filepath.Base(dir),
		Source:   src,
		Tests:    tests,
		TimeLim:  DefaultTimeLimit,
		MemLimit: DefaultMemLimit,
	}, nil
}

func findSource(dir string) (string, error) {
	for _, name := range []string{filepath.Base(dir) + ".cpp", "sol.cpp"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	cpps, _ := filepath.Glob(filepath.Join(dir, "*.cpp"))
	switch len(cpps) {
	case 1:
		return cpps[0], nil
	case 0:
		return "", fmt.Errorf("%s: no .cpp source", dir)
	default:
		return "", fmt.Errorf("%s: several .cpp files; name one %s.cpp or sol.cpp",
			dir, filepath.Base(dir))
	}
}

// findTests pairs tests/N.in with tests/N.out. A missing directory means no
// tests; an input without an output is an error so it is not silently skipped.
func findTests(dir string) ([]Test, error) {
	ins, err := filepath.Glob(filepath.Join(dir, "*.in"))
	if err != nil {
		return nil, err
	}
	var tests []Test
	for _, in := range ins {
		name := strings.TrimSuffix(filepath.Base(in), ".in")
		out := filepath.Join(dir, name+".out")
		if _, err := os.Stat(out); errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s: missing %s.out", dir, name)
		}
		tests = append(tests, Test{Name: name, In: in, Out: out})
	}
	sort.Slice(tests, func(i, j int) bool { return natLess(tests[i].Name, tests[j].Name) })
	return tests, nil
}

// natLess orders "2" before "10"; non-numeric names fall back to strings.
func natLess(a, b string) bool {
	x, errA := strconv.Atoi(a)
	y, errB := strconv.Atoi(b)
	switch {
	case errA == nil && errB == nil:
		return x < y
	case errA == nil:
		return true
	case errB == nil:
		return false
	}
	return a < b
}

var freopenRE = regexp.MustCompile(`freopen\s*\(\s*"([^"/]+)\.in"`)

// FileIO returns the stem the source reads from (old USACO style,
// freopen("milk2.in", ...)), or "" for stdin/stdout. It is re-read on every
// judge because the source changes while the player edits. A freopen behind
// a // comment does not count; block comments are not parsed.
func FileIO(source []byte) string {
	for line := range strings.Lines(string(source)) {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		if m := freopenRE.FindStringSubmatch(line); m != nil {
			return m[1]
		}
	}
	return ""
}
