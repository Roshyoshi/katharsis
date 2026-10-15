// Package judge compiles a solution, runs it against tests under limits, and
// checks its output.
package judge

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Profile is a set of compiler flags with a purpose.
type Profile struct {
	Name  string
	Flags []string
	// Slack multiplies the time limit when deciding TLE. Sanitizer builds
	// run several times slower than -O2, so their timing is only a hint.
	Slack float64
	// Sanitized builds carry shadow memory, so peak RSS is not compared
	// against the memory limit.
	Sanitized bool
	// Compiler overrides the package Compiler; CleanEnv drops nix's
	// compiler environment, which breaks Apple's toolchain (see probe.go).
	Compiler string
	CleanEnv bool
	// Libstdcxx builds use GCC's libstdc++, which has <bits/stdc++.h>,
	// std::__gcd and std::__lg itself, so they skip libc++'s shim (see
	// commonFlags). GNU builds compile with g++ (see gnu.go).
	Libstdcxx bool
	GNU       bool
	// PCH builds load a precompiled header of the #include lines a source
	// opens with, once one is built (see pch.go). Only Libstdcxx profiles:
	// libc++ builds force-include the shim ahead of everything.
	PCH bool
}

// macOS ld refuses -stack_size above 512 MB; the 8 MB default segfaults on
// 1e6-deep recursion, which judges with large stacks accept.
const stackFlag = "-Wl,-stack_size,0x20000000"

// divTrap makes integer division and modulo by zero crash, as they do on
// the x86-64 judges (SIGFPE). arm64's sdiv returns 0 instead, so without it
// a missed zero check passes here and fails there. Trap mode needs no
// sanitizer runtime and costs a compare per division; crashReason names
// the trap.
var divTrap = []string{"-fsanitize=integer-divide-by-zero", "-fsanitize-trap=integer-divide-by-zero"}

// asanFlags are the ASan + UBSan sample builds' flags, before the library's.
var asanFlags = []string{
	"-std=gnu++23", "-O1", "-g", "-fno-omit-frame-pointer",
	"-fsanitize=address,undefined", "-fno-sanitize-recover=undefined",
	stackFlag,
}

// libcxxDebug is libc++'s debug hardening: bounds checks on vector, string,
// array, span and the like, where ASan alone misses string and std::array
// indexing. Its fast and extensive modes check the same but trap without a
// word; this one aborts naming the check ("string index out of bounds").
var libcxxDebug = []string{"-U_LIBCPP_HARDENING_MODE", "-D_LIBCPP_HARDENING_MODE=_LIBCPP_HARDENING_MODE_DEBUG"}

var (
	// Samples: catch UB and memory errors early, libc++'s bounds checks
	// among them; timing is not trusted.
	Samples = Profile{
		Name:      "asan+ubsan",
		Flags:     slices.Concat(asanFlags, libcxxDebug),
		Slack:     4,
		Sanitized: true,
	}
	// Full: what the time limit is checked against.
	Full = Profile{
		Name:  "O2",
		Flags: slices.Concat([]string{"-std=gnu++23", "-O2"}, divTrap, []string{stackFlag}),
		Slack: 1,
	}
	// Debug: what :Debug runs under lldb. -O0 keeps every variable and line
	// steppable; no sanitizers, so the program behaves like the judged one
	// short of optimization (the division trap included, which also stops
	// lldb on the faulting line); the big stack so deep recursion reaches
	// the bug instead of overflowing first.
	Debug = Profile{
		Name:  "debug",
		Flags: slices.Concat([]string{"-std=gnu++23", "-O0", "-g", "-fno-omit-frame-pointer"}, divTrap, []string{stackFlag}),
		Slack: 1,
	}
)

// traps reports whether p's builds trap on the sanitizer check what
// ("undefined", "integer-divide-by-zero") rather than report it through
// a runtime, so a SIGTRAP or SIGILL is that check's (see crashReason).
func (p Profile) traps(what string) bool {
	return slices.ContainsFunc(p.Flags, func(f string) bool {
		s, ok := strings.CutPrefix(f, "-fsanitize-trap=")
		return ok && strings.Contains(s, what)
	})
}

type Build struct {
	Binary   string
	Src      []byte // the exact bytes compiled
	OK       bool
	Cached   bool
	Log      string // what the compiler printed
	Duration time.Duration
	PCH      bool // loaded a precompiled header (see pch.go)
}

// Compiler is the C++ compiler; the dev shell pins clang++.
var Compiler = "clang++"

// Compile builds source with p into a content-addressed cache, so rejudging
// an unchanged file skips the compiler. A build still compiling after
// compileTimeout fails like a compile error, with a Log that says so.
//
// Every build, whatever the profile, also gets what contest code expects
// of a judge's g++ (see commonFlags): ONLINE_JUDGE defined, so a template's
// "#ifndef ONLINE_JUDGE freopen(\"input.txt\")" stays out; <bits/stdc++.h>,
// which libc++ lacks; and libstdc++'s std::__gcd and std::__lg.
//
// Under a PCH profile, a build loads the precompiled header of the
// source's leading #include lines once one is ready, and starts one
// otherwise (see pch.go). A build means the same either way, so its
// binary is cached under the same key.
func Compile(ctx context.Context, source string, p Profile) (Build, error) {
	src, err := os.ReadFile(source)
	if err != nil {
		return Build{}, err
	}
	dir, err := cacheDir()
	if err != nil {
		return Build{}, err
	}
	inc, err := includeDir()
	if err != nil {
		return Build{}, err
	}
	flags := slices.Concat(p.Flags, commonFlags(p, inc))
	h := sha256.New()
	// Key on the resolved compiler path: nix store paths change with the
	// compiler version, so an upgrade never reuses an old build. The
	// include dir's name carries its contents' hash.
	h.Write([]byte(resolve(p.compiler()) + "\x00" + strings.Join(flags, " ") + "\x00"))
	h.Write(src)
	bin := filepath.Join(dir, hex.EncodeToString(h.Sum(nil))[:24])
	if _, err := os.Stat(bin); err == nil {
		now := time.Now()
		_ = os.Chtimes(bin, now, now) // recently used: evictBuilds keeps it
		return Build{Binary: bin, Src: src, OK: true, Cached: true}, nil
	}

	// Build in a private temp dir under the final name, then move it into
	// place. The private dir keeps concurrent builds of the same source from
	// clobbering each other; the final name matters because clang -g on
	// Darwin writes <out>.dSYM with <out> inside it, and the symbolizer only
	// finds line numbers when both names match the binary's.
	work, err := os.MkdirTemp(dir, "build-")
	if err != nil {
		return Build{}, err
	}
	defer func() { _ = os.RemoveAll(work) }()
	out := filepath.Join(work, filepath.Base(bin))
	in, compiled := source, src
	if fixed, ok := stripTargetPragmas(src, source); p.GNU && ok {
		in, compiled = filepath.Join(work, filepath.Base(source)), fixed
		if err := os.WriteFile(in, fixed, 0o644); err != nil {
			return Build{}, err
		}
	}
	// -fdiagnostics-color=never, not clang's -fno-color-diagnostics, which
	// g++ rejects.
	base := slices.Concat(flags, []string{"-fdiagnostics-color=never"})
	tail := []string{"-x", "c++", in, "-o", out}
	b := Build{Binary: bin, Src: src}
	start := time.Now()
	var pc *pch
	if p.PCH && ctx.Err() == nil {
		pc = precompiled(dir, p, base, compiled)
	}
	if pc != nil {
		b.Log, err = runCompiler(ctx, p, work, slices.Concat(base, pc.args(), tail))
		b.PCH = true
		if ctx.Err() == nil && pc.blamed(b.Log, err) {
			pc.drop()
			b.PCH = false
			if err == nil {
				b.Log = pc.unmention(b.Log) // g++ read the headers instead
			} else {
				b.Log, err = runCompiler(ctx, p, work, slices.Concat(base, tail))
			}
		}
	} else {
		b.Log, err = runCompiler(ctx, p, work, slices.Concat(base, tail))
	}
	b.Duration = time.Since(start)
	if err != nil {
		var exit *exec.ExitError
		switch {
		case errors.Is(err, errCompileTimeout):
			// A compile error, not a judge failure: the likely cause is the
			// source (a template or constexpr blowup), and the pass ends
			// with a verdict like any other compile error.
			b.Log = fmt.Sprintf("error: compile timed out after %v\n", compileTimeout) + b.Log
			return b, nil
		case errors.As(err, &exit) && ctx.Err() == nil:
			return b, nil // compile error: OK=false, Log has the reason
		}
		return b, err
	}
	// dSYM first: a reader that sees the binary should also see its symbols.
	if _, err := os.Stat(out + ".dSYM"); err == nil {
		// Best effort: a concurrent build of the same source may have put
		// identical symbols there first.
		_ = os.RemoveAll(bin + ".dSYM")
		_ = os.Rename(out+".dSYM", bin+".dSYM")
	}
	if err := os.Rename(out, bin); err != nil {
		return b, err
	}
	b.OK = true
	evictBuilds(dir, BuildCacheCap, time.Now().Add(-evictMinAge))
	return b, nil
}

// compileTimeout bounds every compiler run. Contest code compiles in
// seconds even on a busy machine; a compiler still going after this is
// stuck on a template or constexpr blowup, or wedged, and a :Submit
// waiting on it would hold the door until its grace ran out (a :Submit
// while one judges is refused).
var compileTimeout = 30 * time.Second

// errCompileTimeout is runCompiler's error for a run cut at
// compileTimeout.
var errCompileTimeout = errors.New("compile timed out")

// runCompiler runs p's compiler with args and returns its output: stderr,
// and stdout, which only --version writes. The compiler leads its own
// process group, killed whole on a cancel or at compileTimeout: g++ runs
// cc1plus, as and ld as children, which outlive a g++ killed alone, keep
// compiling, and hold the output pipe, and so runCompiler, until done.
//
// tmp, unless empty, is the compiler's TMPDIR. A killed compiler leaves
// its temp files (g++'s cc*.s, clang's .o) behind, so they belong in a
// dir that goes with the build.
func runCompiler(ctx context.Context, p Profile, tmp string, args []string) (out string, err error) {
	tctx, cancel := context.WithTimeout(ctx, compileTimeout)
	defer cancel()
	cmd := exec.CommandContext(tctx, p.compiler(), args...)
	cmd.Env = compileEnv(p)
	if tmp != "" {
		cmd.Env = append(cmd.Env, "TMPDIR="+tmp)
	}
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	inGroup(cmd)
	cmd.WaitDelay = time.Second // for a child that left the group
	err = cmd.Run()
	if err != nil && ctx.Err() == nil && tctx.Err() != nil {
		err = errCompileTimeout
	}
	return buf.String(), err
}

// commonFlags go into every build after the profile's own; see Compile.
// -isystem puts the shim headers after the toolchain's (nix's wrapper adds
// libc++ with -isystem first), so they only fill the gaps libc++ leaves.
// libstdc++ leaves none, and the shim breaks it: g++ would take the shim's
// <bits/stdc++.h> over its own (-isystem comes before the system dirs),
// and katharsis_gnu.h does not parse under g++ (no __is_integral builtin;
// its !__GLIBCXX__ guard runs before any header defines that).
func commonFlags(p Profile, inc string) []string {
	if p.Libstdcxx {
		return []string{"-DONLINE_JUDGE"}
	}
	return []string{"-DONLINE_JUDGE", "-isystem", inc, "-include", filepath.Join(inc, "katharsis_gnu.h")}
}

// shim is the headers commonFlags adds: include/bits/stdc++.h and
// include/katharsis_gnu.h.
//
//go:embed include
var shim embed.FS

// includeDir writes the shim headers to <cache>/katharsis/include-<hash>
// and returns that dir; later calls only check it is still there. The hash
// names the contents, so a new version gets a new dir (and new build
// keys), and a dir that exists is complete: it is written aside and
// renamed into place.
func includeDir() (string, error) {
	incMu.Lock()
	defer incMu.Unlock()
	if incPath != "" {
		if _, err := os.Stat(incPath); err == nil {
			return incPath, nil
		}
	}
	h := sha256.New()
	err := fs.WalkDir(shim, "include", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := shim.ReadFile(p)
		h.Write([]byte(p + "\x00"))
		h.Write(b)
		return err
	})
	if err != nil {
		return "", err
	}
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	inc := filepath.Join(filepath.Dir(dir), "include-"+hex.EncodeToString(h.Sum(nil))[:12])
	if _, err := os.Stat(inc); err != nil {
		if err := writeShim(filepath.Dir(dir), inc); err != nil {
			return "", err
		}
	}
	incPath = inc
	return inc, nil
}

var (
	incMu   sync.Mutex
	incPath string // set once includeDir has written it
)

func writeShim(parent, inc string) error {
	tmp, err := os.MkdirTemp(parent, ".include-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	sub, err := fs.Sub(shim, "include")
	if err != nil {
		return err
	}
	if err := os.CopyFS(tmp, sub); err != nil {
		return err
	}
	if err := os.Rename(tmp, inc); err != nil {
		if _, serr := os.Stat(inc); serr == nil {
			return nil // another process put it there first
		}
		return err
	}
	return nil
}

// compileEnv disables nix's hardening for judge builds. Inside a nix dev
// shell or build, the setup hook enables it (its -fwrapv makes UBSan skip
// signed overflow) and the wrapper reads it from there; an empty
// NIX_HARDENING_ENABLE turns it off. Outside nix nothing enables it.
func compileEnv(p Profile) []string {
	if p.CleanEnv {
		return cleanEnv()
	}
	return append(os.Environ(), "NIX_HARDENING_ENABLE=")
}

func cacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "katharsis", "build")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	sweepOnce.Do(func() {
		cleanDebris(dir, time.Now().Add(-debrisAge), "build-")
		evictBuilds(dir, BuildCacheCap, time.Now().Add(-evictMinAge))
	})
	return dir, nil
}

var sweepOnce sync.Once

// Sweep removes what killed processes leave behind, older than an hour:
// katharsis-run-* and katharsis-probe-* dirs in the temp dir (a run's
// working dir holds a copy of a hidden test's input) and build-* dirs in
// the build cache (SIGKILL skips a compile's deferred cleanup), and trims
// the build cache, builds and PCHs, to BuildCacheCap. A process's first
// build sweeps the cache by itself; a command calls Sweep at startup to
// cover the temp dir too. Other processes judging meanwhile are safe:
// nothing they use is that old.
func Sweep() {
	cleanDebris(os.TempDir(), time.Now().Add(-debrisAge), "katharsis-run-", "katharsis-probe-")
	_, _ = cacheDir()
}

// BuildCacheCap is the size the build cache is trimmed to, least recently
// used first. A build is a few hundred KB to a few MB with its dSYM; a
// PCH is 20-200 MB (see pch.go), and one source's pair, g++'s and
// clang's, is 150-210 MB. The cap holds two pairs and the builds.
var BuildCacheCap int64 = 512 << 20

const (
	// debrisAge: a build-* or run dir older than this is from a compile or
	// a run that was killed (SIGKILL skips the deferred RemoveAll); none
	// takes that long.
	debrisAge = time.Hour
	// evictMinAge protects recent builds from eviction even over the cap:
	// one may be the binary a judge pass or a debugger is running now.
	evictMinAge = 10 * time.Minute
)

// cleanDebris removes the dirs in dir named with one of prefixes and last
// touched before before. content/web.CleanDebris does the same for the
// content caches; the judge keeps its own rather than import the content
// sources' network package, and sweeps sooner (debrisAge, against web's
// 6 h): a run's dir holds a copy of a hidden test.
func cleanDebris(dir string, before time.Time, prefixes ...string) {
	des, _ := os.ReadDir(dir) // a missing dir has nothing to clean
	for _, d := range des {
		ours := slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(d.Name(), p) })
		if !d.IsDir() || !ours {
			continue
		}
		if fi, err := d.Info(); err == nil && fi.ModTime().Before(before) {
			_ = os.RemoveAll(filepath.Join(dir, d.Name()))
		}
	}
}

// evictBuilds deletes the least recently used entries until the cache is
// within capBytes, sparing any used since keepAfter. An entry is a build
// (a binary and its dSYM, by the binary's mtime, which a cache hit
// refreshes) or a pch-* dir (by its own mtime, which a build loading the
// PCH refreshes).
func evictBuilds(dir string, capBytes int64, keepAfter time.Time) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type build struct {
		name string
		used time.Time
		size int64
	}
	var builds []build
	var total int64
	for _, d := range des {
		name := d.Name()
		isPCH := d.IsDir() && strings.HasPrefix(name, "pch-")
		if !isPCH && (d.IsDir() || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "build-")) {
			continue
		}
		fi, err := d.Info()
		if err != nil {
			continue
		}
		b := build{name: name, used: fi.ModTime(), size: fi.Size() + dirSize(filepath.Join(dir, name+".dSYM"))}
		if isPCH {
			b.size = dirSize(filepath.Join(dir, name))
		}
		builds = append(builds, b)
		total += b.size
	}
	if total <= capBytes {
		return
	}
	slices.SortFunc(builds, func(a, b build) int { return a.used.Compare(b.used) })
	for _, b := range builds {
		if total <= capBytes || !b.used.Before(keepAfter) {
			break
		}
		_ = os.RemoveAll(filepath.Join(dir, b.name+".dSYM"))
		if os.RemoveAll(filepath.Join(dir, b.name)) == nil {
			total -= b.size
		}
	}
	// A dSYM whose binary is gone is unreachable. A young one may be a
	// build moving into place: its dSYM goes first.
	for _, d := range des {
		if base, ok := strings.CutSuffix(d.Name(), ".dSYM"); ok && d.IsDir() {
			if fi, err := d.Info(); err != nil || !fi.ModTime().Before(keepAfter) {
				continue
			}
			if _, err := os.Stat(filepath.Join(dir, base)); errors.Is(err, fs.ErrNotExist) {
				_ = os.RemoveAll(filepath.Join(dir, d.Name()))
			}
		}
	}
}

func dirSize(p string) int64 {
	var n int64
	_ = filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, err := d.Info(); err == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}

func (p Profile) compiler() string {
	return cmp.Or(p.Compiler, Compiler)
}

// resolve maps a compiler name to its absolute path, symlinks followed.
func resolve(name string) string {
	p, err := exec.LookPath(name)
	if err != nil {
		return name
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}
