//go:build qa

package judgeqa

import (
	"context"
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
		dir, _ := os.MkdirTemp("", "judgeqa-helper-")
		defer os.RemoveAll(dir)
		src := filepath.Join(dir, "h.cpp")
		os.WriteFile(src, []byte("#include <cstdio>\nint main(){puts(\"ok\");}\n"), 0o644)
		os.MkdirAll(filepath.Join(dir, "tests"), 0o755)
		os.WriteFile(filepath.Join(dir, "tests", "1.in"), nil, 0o644)
		os.WriteFile(filepath.Join(dir, "tests", "1.out"), []byte("ok\n"), 0o644)
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
		// And the Full profile (what :Submit uses).
		b, err := judge.Compile(context.Background(), src, judge.Full)
		fmt.Printf("full compile ok=%v err=%v\n", b.OK, err)
		return 0
	}
	fmt.Fprintln(os.Stderr, "unknown helper", mode)
	return 2
}

// plainEnv is the environment of a shell outside nix: no stdenv compiler
// variables.
func plainEnv(home, path string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, "NIX_") || k == "SDKROOT" || k == "DEVELOPER_DIR" ||
			k == "MACOSX_DEPLOYMENT_TARGET" || k == "CC" || k == "CXX" || k == "HOME" ||
			k == "XDG_CACHE_HOME" || k == "PATH" || k == helperEnv {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "HOME="+home, "PATH="+path, helperEnv+"=probe")
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

// The probe cache is keyed on the PATH compiler's path as found (not
// resolved) and its mtime. A nix profile symlink keeps its path when the
// profile switches compilers, and every nix store file has mtime 1.
func TestQAProbeKeyFollowsCompiler(t *testing.T) {
	good, bad := os.Getenv("JUDGEQA_CLANG_GOOD"), os.Getenv("JUDGEQA_CLANG_BAD")
	if good == "" || bad == "" {
		t.Skip("set JUDGEQA_CLANG_GOOD (clang 22 clang++) and JUDGEQA_CLANG_BAD (clang 21 clang++)")
	}
	h := t.TempDir()
	bin := filepath.Join(h, "profile-bin")
	os.MkdirAll(bin, 0o755)
	link := filepath.Join(bin, "clang++")
	if err := os.Symlink(good, link); err != nil {
		t.Fatal(err)
	}
	// The rest of the caller's PATH supplies dsymutil, as a nix profile does.
	env := plainEnv(h, bin+":"+os.Getenv("PATH"))
	out1 := helper(t, env)
	note("probe_key/profile switch", "1 clang++ -> clang 22", strings.ReplaceAll(strings.TrimSpace(out1), "\n", " | "))
	os.Remove(link)
	if err := os.Symlink(bad, link); err != nil {
		t.Fatal(err)
	}
	out2 := helper(t, env)
	note("probe_key/profile switch", "2 clang++ -> clang 21", strings.ReplaceAll(strings.TrimSpace(out2), "\n", " | "))
	if strings.Contains(out2, `second="asan+ubsan"`) {
		t.Errorf("after the PATH compiler changed to one whose ASan hangs, the cached nix ASan profile was reused:\n%s", out2)
	}
}

func TestQAProbeLifecycle(t *testing.T) {
	needCompiler(t)
	h := t.TempDir()
	env := plainEnv(h, os.Getenv("PATH"))
	env = append(env, "JUDGEQA_INHERIT_NIX=1")
	// Keep the caller's nix variables when the suite runs in the dev shell:
	// the probe there sees what the game would see.
	env = inheritNix(env)
	out := helper(t, env)
	note("probe_lifecycle", "1 cold", oneLine(out))
	out = helper(t, env)
	note("probe_lifecycle", "2 cached", oneLine(out))
	files, _ := filepath.Glob(filepath.Join(h, "Library", "Caches", "katharsis", "sample-profile-*"))
	for _, f := range files {
		os.Remove(f)
	}
	out = helper(t, env)
	note("probe_lifecycle", "3 cache deleted", oneLine(out))

	h2 := t.TempDir()
	env2 := inheritNix(plainEnv(h2, os.Getenv("PATH")))
	out = helper(t, env2, "JUDGEQA_PROBE_CANCEL=1")
	note("probe_lifecycle", "4 cancelled ctx, cold", oneLine(out))
	if !strings.Contains(out, `second="`+sample.Name+`"`) {
		t.Errorf("a probe cancelled once leaves the process on a different profile than %q for good:\n%s", sample.Name, out)
	}

	h3 := t.TempDir()
	env3 := inheritNix(plainEnv(h3, "/bin:/usr/sbin:/sbin"))
	out = helper(t, env3)
	note("probe_lifecycle", "5 no clang++ on PATH", oneLine(out))
}

func inheritNix(env []string) []string {
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, "NIX_") || k == "SDKROOT" || k == "DEVELOPER_DIR" || k == "MACOSX_DEPLOYMENT_TARGET" {
			env = append(env, kv)
		}
	}
	return env
}

func oneLine(s string) string { return strings.ReplaceAll(strings.TrimSpace(s), "\n", " | ") }
