package judge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	AppleSamples = withCompiler(Samples, "asan+ubsan (apple)", appleClang, true)
	// HardenedSamples: no ASan; UBSan plus libc++'s debug bounds checks,
	// which still catch out-of-range vector/string indexing.
	HardenedSamples = Profile{
		Name: "ubsan+hardened",
		Flags: []string{
			"-std=gnu++23", "-O1", "-g", "-fno-omit-frame-pointer",
			"-fsanitize=undefined", "-fno-sanitize-recover=undefined",
			"-U_LIBCPP_HARDENING_MODE",
			"-D_LIBCPP_HARDENING_MODE=_LIBCPP_HARDENING_MODE_DEBUG",
			stackFlag,
		},
		Slack: 2,
	}
)

func withCompiler(p Profile, name, cxx string, clean bool) Profile {
	p.Name, p.Compiler, p.CleanEnv = name, cxx, clean
	return p
}

var (
	probeOnce sync.Once
	probed    Profile
)

// SampleProfile returns the first sample profile whose binaries actually
// run here: nix ASan, then Apple ASan, then HardenedSamples. The answer is
// cached on disk per compiler and OS release, since a failing ASan probe
// costs a few seconds of timeout.
func SampleProfile(ctx context.Context) Profile {
	probeOnce.Do(func() { probed = probeSamples(ctx) })
	return probed
}

func probeSamples(ctx context.Context) Profile {
	candidates := []Profile{Samples, AppleSamples}
	key := probeKey()
	cache := ""
	if dir, err := cacheDir(); err == nil {
		cache = filepath.Join(filepath.Dir(dir), "sample-profile-"+key)
		if b, err := os.ReadFile(cache); err == nil {
			for _, p := range append(candidates, HardenedSamples) {
				if p.Name == strings.TrimSpace(string(b)) {
					return p
				}
			}
		}
	}
	chosen := HardenedSamples
	for _, p := range candidates {
		if p.Compiler != "" {
			if _, err := os.Stat(p.Compiler); err != nil {
				continue
			}
		}
		if runs(ctx, p) {
			chosen = p
			break
		}
	}
	// A cancelled probe proves nothing; caching its fallback would downgrade
	// this machine for good.
	if cache != "" && ctx.Err() == nil {
		os.WriteFile(cache, []byte(chosen.Name+"\n"), 0o644)
	}
	return chosen
}

// runs compiles a trivial program with p and checks it prints within 3 s.
func runs(ctx context.Context, p Profile) bool {
	dir, err := os.MkdirTemp("", "katharsis-probe-")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "probe.cpp")
	bin := filepath.Join(dir, "probe")
	prog := "#include <cstdio>\nint main() { std::puts(\"ok\"); }\n"
	if os.WriteFile(src, []byte(prog), 0o644) != nil {
		return false
	}
	cc := exec.CommandContext(ctx, p.compiler(), append(append([]string{}, p.Flags...), src, "-o", bin)...)
	cc.Env = compileEnv(p)
	if cc.Run() != nil {
		return false
	}
	rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(rctx, bin).Output()
	return err == nil && strings.TrimSpace(string(out)) == "ok"
}

func probeKey() string {
	h := sha256.New()
	for _, c := range []string{Compiler, appleClang} {
		path, _ := exec.LookPath(c)
		fmt.Fprintf(h, "%s\x00", path)
		if fi, err := os.Stat(path); err == nil {
			fmt.Fprintf(h, "%d\x00", fi.ModTime().Unix())
		}
	}
	fmt.Fprintf(h, "%s", osRelease())
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
