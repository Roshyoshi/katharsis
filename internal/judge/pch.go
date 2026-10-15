package judge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// Precompiled headers. Parsing the headers a contest source opens with is
// most of its build: <bits/stdc++.h> is ~0.7 s of a 1.2 s g++ -O2 compile.
// A profile with PCH set keeps a precompiled header of each list of leading
// #include lines it has seen, per compiler and flags, and later builds of a
// source opening with that list load it instead (minimum ms over 10 runs,
// M4, load ~5; docs/research/gcc-toolchain.md):
//
//	                         plain   PCH
//	g++ -O2, bits/stdc++.h    1230   491   .gch 188 MB
//	g++ -O2, ~/cp template     876   447        129 MB
//	clang ASan, bits          1299   713   .pch  25 MB
//	clang ASan, template      1075   657         18 MB
//
// The PCH stands in for exactly the source's first includes, which the
// source then includes again as no-ops (include guards), so a build means
// what it meant without one. That is why only the leading #include <...>
// lines go in: what comes before or between them (a #define, a #pragma GCC
// optimize, a quoted include) can change what the headers mean, so the list
// ends there. Forcing <bits/stdc++.h> into every build would be faster for
// sources that do not include it, and would change them: names such as y1
// or count collide with the library, a missing #include passes here and
// fails on the real judge, and a #define _GLIBCXX_DEBUG above the includes
// no longer reaches them.
//
// Layout, in the build cache: pch-<key>/prefix.h holds the include lines,
// written once and never touched again (clang checks its mtime);
// prefix.h.gch (g++) or prefix.pch (clang) appears by rename once built and
// checked, so a build never sees half of one; failed says why one could not
// be. Eviction counts the dir as one entry, last used when last touched.

// pch is one precompiled header in the build cache.
type pch struct {
	dir string // pch-<key>
	gnu bool   // g++'s .gch rather than clang's .pch
}

func (c pch) header() string { return filepath.Join(c.dir, "prefix.h") }

func (c pch) file() string {
	if c.gnu {
		return c.header() + ".gch" // where g++ looks for -include prefix.h
	}
	return filepath.Join(c.dir, "prefix.pch")
}

func (c pch) failed() string { return filepath.Join(c.dir, "failed") }

// args make a build load the PCH. g++ takes prefix.h.gch for prefix.h when
// it is valid and otherwise reads prefix.h, silently unless -Winvalid-pch
// asks it to say so; clang refuses a PCH it cannot use with an error.
func (c pch) args() []string {
	if c.gnu {
		return []string{"-Winvalid-pch", "-include", c.header()}
	}
	return []string{"-include-pch", c.file()}
}

// precompiled returns the PCH a build of src under p (with flags base) can
// load, or nil when none is ready. Then it starts building one in the
// background, unless one is building or failed within the hour, and the
// build goes on without it: no build waits for a PCH.
func precompiled(cache string, p Profile, base []string, src []byte) *pch {
	headers := leadingIncludes(src, p.GNU)
	if len(headers) == 0 {
		return nil
	}
	c := pch{dir: filepath.Join(cache, "pch-"+pchKey(p, base, headers)), gnu: p.GNU}
	if _, err := os.Stat(c.file()); err == nil {
		now := time.Now()
		_ = os.Chtimes(c.dir, now, now) // in use: evictBuilds spares it
		return &c
	}
	if fi, err := os.Stat(c.failed()); err == nil && time.Since(fi.ModTime()) < pchRetry {
		return nil
	}
	done := make(chan struct{})
	if _, busy := pchBuilding.LoadOrStore(c.dir, done); !busy {
		pchWG.Add(1)
		go c.build(p, base, headers, done)
	}
	return nil
}

var (
	pchBuilding sync.Map // pch dir -> chan closed when this process has built it
	pchWG       sync.WaitGroup
	// pchSlot runs one PCH build at a time: each is a compiler at full
	// tilt for seconds, beside judged runs that time their CPU.
	pchSlot = make(chan struct{}, 1)
)

// pchRetry is how long a failed PCH stands before builds try again.
const pchRetry = time.Hour

// pchKey names a PCH by everything that must match for a build to load
// it: the compiler (its resolved path, size, mtime and version), every
// flag, and the headers.
func pchKey(p Profile, base, headers []string) string {
	h := sha256.New()
	for _, s := range [][]string{{compilerID(p), fmt.Sprint(p.GNU)}, base, headers} {
		h.Write([]byte(strings.Join(s, "\x00") + "\x00\x00"))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// compilerID is p's compiler resolved, with its size, mtime and --version
// output. A nix store path names one compiler for good; a compiler
// upgraded in place keeps its path and changes the rest. --version runs
// once per process and compiler, never cut short by a build's cancel,
// which would key this process's PCHs apart from every other's.
func compilerID(p Profile) string {
	path := resolve(p.compiler())
	id := path
	if fi, err := os.Stat(path); err == nil {
		id = fmt.Sprintf("%s\x00%d\x00%d", path, fi.Size(), fi.ModTime().UnixNano())
	}
	if v, ok := compilerVersions.Load(id); ok {
		return v.(string)
	}
	out, _ := runCompiler(context.Background(), p, []string{"--version"})
	v := id + "\x00" + out
	compilerVersions.Store(id, v)
	return v
}

var compilerVersions sync.Map // path, size, mtime -> that plus --version

// blamed reports whether a build's failure or log is the PCH's doing
// rather than the source's: g++ named it in a -Winvalid-pch warning
// (a build that ran fine, reading the headers instead), or a failed
// build's errors are about it (clang's "precompiled file ... was compiled
// with", "has been modified since the precompiled header", g++'s "cannot
// read PCH file"). Either way the PCH is dropped and rebuilt.
func (c pch) blamed(log string, err error) bool {
	if err == nil {
		return c.gnu && strings.Contains(log, c.file())
	}
	var exit *exec.ExitError
	return errors.As(err, &exit) && (strings.Contains(log, c.file()) ||
		strings.Contains(log, "precompiled") || strings.Contains(log, "PCH file") || strings.Contains(log, "AST file"))
}

// unmention cuts the lines naming the PCH from a log: they are about the
// judge's cache, not the player's code.
func (c pch) unmention(log string) string {
	var b strings.Builder
	for l := range strings.Lines(log) {
		if !strings.Contains(l, c.file()) {
			b.WriteString(l)
		}
	}
	return b.String()
}

// drop removes a PCH a build blamed, so the next build makes a new one.
func (c pch) drop() { _ = os.Remove(c.file()) }

// build makes the PCH in the background, or records why it could not.
func (c pch) build(p Profile, base, headers []string, done chan struct{}) {
	defer pchWG.Done()
	defer close(done)
	defer pchBuilding.Delete(c.dir)
	pchSlot <- struct{}{}
	defer func() { <-pchSlot }()
	if err := c.make(context.Background(), p, base, headers); err != nil {
		_ = os.WriteFile(c.failed(), []byte(err.Error()+"\n"), 0o644)
		return
	}
	_ = os.Remove(c.failed())
	evictBuilds(filepath.Dir(c.dir), BuildCacheCap, time.Now().Add(-evictMinAge))
}

func (c pch) make(ctx context.Context, p Profile, base, headers []string) error {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}
	now := time.Now()
	_ = os.Chtimes(c.dir, now, now) // a dir that failed long ago is in use again
	var text strings.Builder
	for _, h := range headers {
		fmt.Fprintf(&text, "#include <%s>\n", h)
	}
	if err := writeOnce(c.header(), []byte(text.String())); err != nil {
		return err
	}
	// Built aside under its final name and checked there, then renamed
	// into place: g++ reads whatever prefix.h.gch holds when it looks.
	work, err := os.MkdirTemp(filepath.Dir(c.dir), "build-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(work) }()
	out := filepath.Join(work, filepath.Base(c.file()))
	if log, err := runCompiler(ctx, p, slices.Concat(base, []string{"-c", "-x", "c++-header", c.header(), "-o", out})); err != nil {
		return fmt.Errorf("precompiling %s: %v\n%s", c.header(), err, log)
	}
	if err := c.check(ctx, p, base, work, out); err != nil {
		return err
	}
	return os.Rename(out, c.file())
}

// check compiles an empty source against the new PCH at out with base,
// so one the compiler would pass over or refuse never goes into place.
// g++ marks the PCH it loads with "!" in -H's list.
func (c pch) check(ctx context.Context, p Profile, base []string, work, out string) error {
	args := []string{"-include-pch", out}
	if c.gnu {
		h := filepath.Join(work, filepath.Base(c.header()))
		if err := os.Link(c.header(), h); err != nil {
			return err
		}
		args = []string{"-H", "-include", h}
	}
	log, err := runCompiler(ctx, p, slices.Concat(base, args, []string{"-fsyntax-only", "-x", "c++", os.DevNull}))
	switch {
	case err != nil:
		return fmt.Errorf("checking %s: %v\n%s", out, err, log)
	case c.gnu && !strings.Contains(log, "! "+out):
		return fmt.Errorf("g++ did not load %s:\n%s", out, log)
	}
	return nil
}

// writeOnce creates path holding data unless it exists. A reader never
// sees it partly written, and an existing one keeps its mtime.
func writeOnce(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if err := os.Link(f.Name(), path); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return nil
}

// leadingIncludes is the headers src opens with: its first #include <...>
// lines, in order, past blank lines, comments and #line directives (which
// stripTargetPragmas adds). It stops at anything else, so the list is
// always where src itself starts. For clang (gnu false) it also passes
// #pragma GCC optimize and target lines, which clang ignores; g++ applies
// an optimize pragma to the headers after it, so there they end the list.
func leadingIncludes(src []byte, gnu bool) []string {
	s := string(bytes.TrimPrefix(src, []byte("\xef\xbb\xbf")))
	var headers []string
	for {
		var ok bool
		if s, ok = skipBlank(s); !ok {
			return headers
		}
		line, rest, _ := strings.Cut(s, "\n")
		line = strings.TrimSuffix(line, "\r")
		switch m := includeRE.FindStringSubmatch(line); {
		case m != nil:
			headers = append(headers, m[1])
		case lineRE.MatchString(line), !gnu && gccPragmaRE.MatchString(line):
		default:
			return headers
		}
		s = rest
	}
}

var (
	// "#include <vector>", maybe with a // comment after; one that ends in
	// a backslash would swallow the next line.
	includeRE   = regexp.MustCompile(`^#[ \t]*include[ \t]*<([^<>]+)>[ \t]*(?://(?:.*[^\\])?)?$`)
	lineRE      = regexp.MustCompile(`^#[ \t]*line[ \t].*[^\\]$`)
	gccPragmaRE = regexp.MustCompile(`^#[ \t]*pragma[ \t]+GCC[ \t]+(?:optimize|target)\b.*[^\\]$`)
)

// skipBlank skips whitespace and comments. ok is false at the end of s or
// in a comment that does not end where the scan can follow it.
func skipBlank(s string) (rest string, ok bool) {
	for {
		s = strings.TrimLeft(s, " \t\r\n\f\v")
		switch {
		case strings.HasPrefix(s, "//"):
			line, rest, _ := strings.Cut(s, "\n")
			if strings.HasSuffix(strings.TrimSuffix(line, "\r"), "\\") {
				return "", false // continues onto the next line
			}
			s = rest
		case strings.HasPrefix(s, "/*"):
			_, rest, found := strings.Cut(s[2:], "*/")
			if !found {
				return "", false
			}
			s = rest
		default:
			return s, s != ""
		}
	}
}
