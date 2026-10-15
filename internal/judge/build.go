// Package judge compiles a solution, runs it against tests under limits, and
// checks its output.
package judge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
}

// macOS ld refuses -stack_size above 512 MB; the 8 MB default segfaults on
// 1e6-deep recursion, which judges with large stacks accept.
const stackFlag = "-Wl,-stack_size,0x20000000"

var (
	// Samples: catch UB and memory errors early; timing is not trusted.
	Samples = Profile{
		Name: "asan+ubsan",
		Flags: []string{
			"-std=gnu++23", "-O1", "-g", "-fno-omit-frame-pointer",
			"-fsanitize=address,undefined", "-fno-sanitize-recover=undefined",
			stackFlag,
		},
		Slack:     4,
		Sanitized: true,
	}
	// Full: what the time limit is checked against.
	Full = Profile{
		Name:  "O2",
		Flags: []string{"-std=gnu++23", "-O2", stackFlag},
		Slack: 1,
	}
	// Debug: what :Debug runs under lldb. -O0 keeps every variable and line
	// steppable; no sanitizers, so the program behaves like the judged one
	// short of optimization; the big stack so deep recursion reaches the bug
	// instead of overflowing first.
	Debug = Profile{
		Name:  "debug",
		Flags: []string{"-std=gnu++23", "-O0", "-g", "-fno-omit-frame-pointer", stackFlag},
		Slack: 1,
	}
)

type Build struct {
	Binary   string
	Src      []byte // the exact bytes compiled
	OK       bool
	Cached   bool
	Log      string // compiler stderr
	Duration time.Duration
}

// Compiler is the C++ compiler; the dev shell pins clang++.
var Compiler = "clang++"

// Compile builds source with p into a content-addressed cache, so rejudging
// an unchanged file skips the compiler.
func Compile(ctx context.Context, source string, p Profile) (Build, error) {
	src, err := os.ReadFile(source)
	if err != nil {
		return Build{}, err
	}
	dir, err := cacheDir()
	if err != nil {
		return Build{}, err
	}
	h := sha256.New()
	// Key on the resolved compiler path: nix store paths change with the
	// compiler version, so an upgrade never reuses an old build.
	h.Write([]byte(resolve(p.compiler()) + "\x00" + strings.Join(p.Flags, " ") + "\x00"))
	h.Write(src)
	bin := filepath.Join(dir, hex.EncodeToString(h.Sum(nil))[:24])
	if _, err := os.Stat(bin); err == nil {
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
	args := append(append([]string{}, p.Flags...),
		"-fno-color-diagnostics", "-x", "c++", source, "-o", out)
	cmd := exec.CommandContext(ctx, p.compiler(), args...)
	cmd.Env = compileEnv(p)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	start := time.Now()
	err = cmd.Run()
	b := Build{Binary: bin, Src: src, Log: stderr.String(), Duration: time.Since(start)}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && ctx.Err() == nil {
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
	return b, nil
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
	return dir, os.MkdirAll(dir, 0o755)
}

func (p Profile) compiler() string {
	if p.Compiler != "" {
		return p.Compiler
	}
	return Compiler
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
