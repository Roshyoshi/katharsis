// Package problem loads a problem directory: the solution source, its tests,
// and how the program does I/O.
//
// Layout (matches ~/cp/problems/<name>):
//
//	<dir>/<name>.cpp     or sol.cpp, or the only .cpp in <dir>
//	<dir>/tests/1.in     paired with tests/1.out
package problem

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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
	tests, err := LoadTests(filepath.Join(dir, "tests"))
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
	cpps, err := namesWithSuffix(dir, ".cpp")
	if err != nil {
		return "", err
	}
	switch len(cpps) {
	case 1:
		return filepath.Join(dir, cpps[0]), nil
	case 0:
		return "", fmt.Errorf("%s: no .cpp source", dir)
	default:
		return "", fmt.Errorf("%s: several .cpp files; name one %s.cpp or sol.cpp",
			dir, filepath.Base(dir))
	}
}

// namesWithSuffix lists dir instead of globbing it, so a dir named like a
// pattern ("p[1]") still finds its own files.
func namesWithSuffix(dir, suffix string) ([]string, error) {
	des, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	var out []string
	for _, d := range des {
		if !d.IsDir() && strings.HasSuffix(d.Name(), suffix) {
			out = append(out, d.Name())
		}
	}
	return out, nil
}

// LoadTests pairs dir/N.in with dir/N.out in natural order ("2" before
// "10"). A missing dir means no tests; an input without an output is an
// error so it is not silently skipped.
func LoadTests(dir string) ([]Test, error) {
	ins, err := namesWithSuffix(dir, ".in")
	if err != nil {
		return nil, err
	}
	var tests []Test
	for _, in := range ins {
		name := strings.TrimSuffix(in, ".in")
		out := filepath.Join(dir, name+".out")
		if _, err := os.Stat(out); errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s: missing %s.out", dir, name)
		}
		tests = append(tests, Test{Name: name, In: filepath.Join(dir, in), Out: out})
	}
	slices.SortFunc(tests, func(a, b Test) int { return natCmp(a.Name, b.Name) })
	return tests, nil
}

// natCmp orders numeric names by value, before any other name; ties
// ("1", "01") and other names fall back to the string order.
func natCmp(a, b string) int {
	x, errA := strconv.Atoi(a)
	y, errB := strconv.Atoi(b)
	switch {
	case errA == nil && errB == nil && x != y:
		return cmp.Compare(x, y)
	case errA == nil && errB != nil:
		return -1
	case errA != nil && errB == nil:
		return 1
	}
	return strings.Compare(a, b)
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
