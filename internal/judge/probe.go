package judge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Not every sanitizer runtime works on every macOS. Checked 2026-10-14 on
// macOS 26.5: nix's clang 21 ASan spins at 100% CPU before main even for
// hello world, while Apple's /usr/bin/clang++ ASan works once nix's compiler
// variables are removed from its environment. UBSan works everywhere. So the
// sample profile is chosen by experiment, once per compiler + OS, not assumed.

const appleClang = "/usr/bin/clang++"

var (
	// AppleSamples: ASan + UBSan through Xcode's toolchain.
	AppleSamples = Profile{
		Name:      "asan+ubsan (apple)",
		Flags:     Samples.Flags,
		Slack:     Samples.Slack,
		Sanitized: true,
		Compiler:  appleClang,
		CleanEnv:  true,
	}
	// HardenedSamples: no ASan; UBSan plus libc++'s debug bounds checks,
	// which still catch out-of-range vector/string indexing.
	HardenedSamples = Profile{
		Name: "ubsan+hardened",
		Flags: slices.Concat([]string{
			"-std=gnu++23", "-O1", "-g", "-fno-omit-frame-pointer",
			"-fsanitize=undefined", "-fno-sanitize-recover=undefined",
		}, libcxxDebug, []string{stackFlag}),
		Slack: 2,
	}
)

var (
	probeMu sync.Mutex
	probed  *Profile // nil until a probe ran to the end
)

// Toolchain is the profiles one compiler family judges with.
type Toolchain struct {
	Samples, Full, Debug Profile
	// Hardened is Samples without ASan, for samples held to a memory
	// limit (the OOM Killer's): ASan's shadow, redzones and chunk
	// headers read as memory, most on small allocations (testdata/nodes.cpp,
	// a std::map of 1M ints, peaks at 92M under ASan and 52M as judged),
	// so a limit on them fails what :Submit passes. Hardened reads memory
	// as the judged build does, and keeps UBSan and the library's bounds
	// checks.
	Hardened Profile
}

// Profiles is this machine's toolchain: g++ and libstdc++ when the sample
// probe settled on libstdc++, which it tries only once g++ has built and
// run a program here; clang and libc++ otherwise. Hardened is the probe's
// no-ASan fallback for that family, which runs wherever its compiler does
// (UBSan's runtime works everywhere, g++'s traps need none), or the
// sample profile itself when the probe already fell back to one.
func Profiles(ctx context.Context) Toolchain {
	s := SampleProfile(ctx)
	tc := Toolchain{Samples: s, Full: Full, Debug: Debug, Hardened: HardenedSamples}
	if s.Libstdcxx {
		tc = Toolchain{Samples: s, Full: GCCFull, Debug: GCCDebug, Hardened: GCCSamples}
	}
	if !s.Sanitized {
		tc.Hardened = s
	}
	return tc
}

// SampleProfile returns the first sample profile whose binaries actually
// run here. Where GCCFull's do: LibstdcxxSamples, then GCCSamples.
// Elsewhere: nix ASan, then Apple ASan, then HardenedSamples. The answer is
// cached on disk per compiler and OS release, since a failing ASan probe
// costs a few seconds of timeout, and kept for the process unless ctx cut
// the probe short.
func SampleProfile(ctx context.Context) Profile {
	probeMu.Lock()
	defer probeMu.Unlock()
	if probed != nil {
		return *probed
	}
	p := probeSamples(ctx)
	if ctx.Err() == nil {
		probed = &p
	}
	return p
}

func probeSamples(ctx context.Context) Profile {
	key := probeKey()
	cache := ""
	if dir, err := cacheDir(); err == nil {
		cache = filepath.Join(filepath.Dir(dir), "sample-profile-"+key)
		if b, err := os.ReadFile(cache); err == nil {
			for _, p := range []Profile{Samples, AppleSamples, HardenedSamples, LibstdcxxSamples, GCCSamples} {
				if p.Name == strings.TrimSpace(string(b)) {
					return p
				}
			}
		}
	}
	candidates, fallback := []Profile{Samples, AppleSamples}, HardenedSamples
	// A missing g++, or Apple's (clang under that name), fails in the
	// compile, so the second try costs nothing there.
	if runs(ctx, GCCFull, probeTimeout) || ctx.Err() == nil && runs(ctx, GCCFull, retryTimeout) {
		candidates, fallback = []Profile{LibstdcxxSamples}, GCCSamples
	}
	var present []Profile
	for _, p := range candidates {
		if _, err := exec.LookPath(p.compiler()); err == nil {
			present = append(present, p)
		}
	}
	chosen, ok := firstRunning(ctx, present, probeTimeout)
	if !ok && ctx.Err() == nil {
		// Before settling on the fallback for good, give the candidates a
		// longer try: a machine busy enough to slow hello world past
		// probeTimeout is a passing state, while the broken runtimes this
		// guards against spin forever.
		chosen, ok = firstRunning(ctx, present, retryTimeout)
	}
	if !ok {
		chosen = fallback
	}
	// A cancelled probe proves nothing; caching its fallback would downgrade
	// this machine for good.
	if cache != "" && ctx.Err() == nil {
		_ = os.WriteFile(cache, []byte(chosen.Name+"\n"), 0o644)
	}
	return chosen
}

// probeTimeout is how long a probe's hello world may take; retryTimeout is
// the second chance before the fallback is cached.
const (
	probeTimeout = 3 * time.Second
	retryTimeout = 15 * time.Second
)

func firstRunning(ctx context.Context, ps []Profile, timeout time.Duration) (Profile, bool) {
	for _, p := range ps {
		if runs(ctx, p, timeout) {
			return p, true
		}
	}
	return Profile{}, false
}

// runs compiles a trivial program with p (probeProgram) and checks it
// prints within timeout.
func runs(ctx context.Context, p Profile, timeout time.Duration) bool {
	dir, err := os.MkdirTemp("", "katharsis-probe-")
	if err != nil {
		return false
	}
	defer func() { _ = os.RemoveAll(dir) }()
	src := filepath.Join(dir, "probe.cpp")
	bin := filepath.Join(dir, "probe")
	if os.WriteFile(src, []byte(probeProgram(p)), 0o644) != nil {
		return false
	}
	if _, err := runCompiler(ctx, p, dir, slices.Concat(p.Flags, []string{src, "-o", bin})); err != nil {
		return false
	}
	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := exec.CommandContext(rctx, bin).Output()
	return err == nil && strings.TrimSpace(string(out)) == "ok"
}

// probeKey names the compilers and OS a probe result holds for. Compiler
// paths are resolved first: a nix profile symlink keeps its path when the
// profile switches compilers, and every nix store file has mtime 1, so
// only the store path a link points to tells two clangs apart.
func probeKey() string {
	h := sha256.New()
	for _, c := range []string{Compiler, appleClang, GXX, ClangLibstdcxx} {
		path := resolve(c)
		h.Write([]byte(path + "\x00"))
		if fi, err := os.Stat(path); err == nil {
			h.Write(fmt.Appendf(nil, "%d\x00", fi.ModTime().Unix()))
		}
	}
	h.Write([]byte(osRelease()))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// cleanEnv is the environment minus nix's compiler wrapper variables, which
// point Apple's xcrun at nix's SDK and inject nix-only flags.
func cleanEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, "NIX_") || k == "SDKROOT" || k == "DEVELOPER_DIR" ||
			k == "MACOSX_DEPLOYMENT_TARGET" || k == "CC" || k == "CXX" {
			continue
		}
		env = append(env, kv)
	}
	return env
}
