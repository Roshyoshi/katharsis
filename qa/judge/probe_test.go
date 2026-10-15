//go:build qa

package judgeqa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"katharsis/internal/judge"
	"katharsis/internal/problem"
)

// runHelper is the subprocess side of the probe tests. SampleProfile
// memoizes per process, so each probe needs a fresh one.
func runHelper(mode string) int {
	switch mode {
	case "probe":
		ctx := context.Background()
		if os.Getenv("JUDGEQA_PROBE_CANCEL") != "" {
			c, cancel := context.WithCancel(ctx)
			cancel()
			ctx = c
		}
		start := time.Now()
		p := judge.SampleProfile(ctx)
		fmt.Printf("first=%q took=%v\n", p.Name, time.Since(start).Round(time.Millisecond))
		p2 := judge.SampleProfile(context.Background())
		fmt.Printf("second=%q\n", p2.Name)
		cache, _ := os.UserCacheDir()
		files, _ := filepath.Glob(filepath.Join(cache, "katharsis", "sample-profile-*"))
		for _, f := range files {
			b, _ := os.ReadFile(f)
			fmt.Printf("cache %s = %q\n", filepath.Base(f), strings.TrimSpace(string(b)))
		}
		// Does the chosen profile actually run hello world?
		dir, err := os.MkdirTemp("", "judgeqa-helper-")
		if err != nil {
			fmt.Println("setup:", err)
			return 1
		}
		defer os.RemoveAll(dir)
		src := filepath.Join(dir, "h.cpp")
		tests := filepath.Join(dir, "tests")
		if err := errors.Join(os.WriteFile(src, []byte(hello), 0o644), os.MkdirAll(tests, 0o755),
			os.WriteFile(filepath.Join(tests, "1.in"), nil, 0o644),
			os.WriteFile(filepath.Join(tests, "1.out"), []byte("ok\n"), 0o644)); err != nil {
			fmt.Println("setup:", err)
			return 1
		}
		prob, err := problem.Load(dir)
		if err != nil {
			fmt.Println("load:", err)
			return 1
		}
		jstart := time.Now()
		judge.Judge(context.Background(), prob, p2, judge.Limits{Time: time.Second}, func(e judge.Event) {
			switch e := e.(type) {
			case judge.BuildDone:
				fmt.Printf("build ok=%v log=%q\n", e.Build.OK, firstLine(e.Build.Log))
			case judge.TestDone:
				fmt.Printf("hello: %s %q cpu=%v wall=%v\n", e.Result.Verdict, e.Result.Reason, e.Result.CPU.Round(time.Millisecond), e.Result.Wall.Round(time.Millisecond))
			case judge.Finished:
				fmt.Printf("finished err=%v after %v\n", e.Err, time.Since(jstart).Round(time.Millisecond))
			}
		})
		// And what :Submit compiles with.
		full := judge.Profiles(context.Background()).Full
		b, err := judge.Compile(context.Background(), src, full)
		fmt.Printf("full compile [%s] ok=%v err=%v\n", full.Name, b.OK, err)
		return 0
	}
	fmt.Fprintln(os.Stderr, "unknown helper", mode)
	return 2
}

// nixVar is a dev shell's compiler variable: set inside nix, absent from a
// player's shell.
func nixVar(k string) bool {
	return strings.HasPrefix(k, "NIX_") || k == "SDKROOT" || k == "DEVELOPER_DIR" || k == "MACOSX_DEPLOYMENT_TARGET"
}

// plainEnv is the environment of a shell outside nix: no stdenv compiler
// variables.
func plainEnv(home, path string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if nixVar(k) || k == "CC" || k == "CXX" || k == "HOME" || k == "XDG_CACHE_HOME" || k == "PATH" || k == helperEnv {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "HOME="+home, "PATH="+path, helperEnv+"=probe")
}

// inheritNix adds the caller's nix variables back, when the suite runs in
// the dev shell: the probe then sees what the game would see.
func inheritNix(env []string) []string {
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); nixVar(k) {
			env = append(env, kv)
		}
	}
	return env
}

func helper(t *testing.T, env []string, extra ...string) string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(env, extra...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("helper: %v", err)
	}
	return string(out)
}

// The probe cache must follow the PATH compiler: a nix profile symlink
// keeps its path when the profile switches compilers, and every nix store
// file has mtime 1.
func TestQAProbeKeyFollowsCompiler(t *testing.T) {
	good, bad := os.Getenv("JUDGEQA_CLANG_GOOD"), os.Getenv("JUDGEQA_CLANG_BAD")
	if good == "" || bad == "" {
		t.Skip("set JUDGEQA_CLANG_GOOD (clang 22 clang++) and JUDGEQA_CLANG_BAD (clang 21 clang++)")
	}
	h := t.TempDir()
	bin := filepath.Join(h, "profile-bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(bin, "clang++")
	if err := os.Symlink(good, link); err != nil {
		t.Fatal(err)
	}
	// The rest of the caller's PATH supplies dsymutil, as a nix profile does.
	env := plainEnv(h, bin+":"+os.Getenv("PATH"))
	note("probe_key/profile switch", "1 clang++ -> clang 22", oneLine(helper(t, env)))
	os.Remove(link)
	if err := os.Symlink(bad, link); err != nil {
		t.Fatal(err)
	}
	out := helper(t, env)
	note("probe_key/profile switch", "2 clang++ -> clang 21", oneLine(out))
	if strings.Contains(out, `second="asan+ubsan"`) {
		t.Errorf("after the PATH compiler changed to one whose ASan hangs, the cached nix ASan profile was reused:\n%s", out)
	}
}

func TestQAProbeLifecycle(t *testing.T) {
	needCompiler(t)
	h := t.TempDir()
	env := inheritNix(plainEnv(h, os.Getenv("PATH")))
	note("probe_lifecycle", "1 cold", oneLine(helper(t, env)))
	note("probe_lifecycle", "2 cached", oneLine(helper(t, env)))
	files, _ := filepath.Glob(filepath.Join(h, "Library", "Caches", "katharsis", "sample-profile-*"))
	for _, f := range files {
		os.Remove(f)
	}
	note("probe_lifecycle", "3 cache deleted", oneLine(helper(t, env)))

	out := helper(t, inheritNix(plainEnv(t.TempDir(), os.Getenv("PATH"))), "JUDGEQA_PROBE_CANCEL=1")
	note("probe_lifecycle", "4 cancelled ctx, cold", oneLine(out))
	if !strings.Contains(out, `second="`+sample.Name+`"`) {
		t.Errorf("a probe cancelled once leaves the process on a different profile than %q for good:\n%s", sample.Name, out)
	}

	out = helper(t, inheritNix(plainEnv(t.TempDir(), "/bin:/usr/sbin:/sbin")))
	note("probe_lifecycle", "5 no clang++ on PATH", oneLine(out))
}

func oneLine(s string) string { return strings.ReplaceAll(strings.TrimSpace(s), "\n", " | ") }
