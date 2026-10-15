package judge

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEvictBuilds(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	mk := func(name string, size int, age time.Duration, dsym bool) {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, make([]byte, size), 0o755); err != nil {
			t.Fatal(err)
		}
		if dsym {
			d := filepath.Join(p+".dSYM", "Contents")
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(d, "dwarf"), make([]byte, size), 0o644); err != nil {
				t.Fatal(err)
			}
			_ = os.Chtimes(p+".dSYM", now.Add(-age), now.Add(-age))
		}
		if err := os.Chtimes(p, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	// A PCH dir is one entry, its size its files', last used at its mtime.
	mkPCH := func(name string, size int, age time.Duration) {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "prefix.h.gch"), make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	mkPCH("pch-stale", 400, 5*time.Hour) // evicted first of all
	mk("old", 400, 3*time.Hour, true)    // 800 with its dSYM: evicted third
	mk("older", 300, 4*time.Hour, false) // evicted second
	mk("mid", 300, 2*time.Hour, false)   // fits under the cap once those go
	mk("fresh", 900, time.Minute, false) // over the cap alone, but in use
	mkPCH("pch-live", 200, time.Minute)  // in use
	if err := os.MkdirAll(filepath.Join(dir, "gone.dSYM"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(filepath.Join(dir, "gone.dSYM"), now.Add(-time.Hour), now.Add(-time.Hour))

	evictBuilds(dir, 1500, now.Add(-evictMinAge))
	for name, want := range map[string]bool{
		"old": false, "old.dSYM": false, "older": false, "mid": true, "fresh": true, "gone.dSYM": false,
		"pch-stale": false, "pch-live": true,
	} {
		_, err := os.Stat(filepath.Join(dir, name))
		if got := err == nil; got != want {
			t.Errorf("%s kept = %v, want %v", name, got, want)
		}
	}
}

// Sweep clears the temp dir of runs a kill -9 cut short (they hold copies
// of hidden test inputs) and the build cache of killed compiles, and
// nothing young or not its own.
func TestSweep(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	cache, err := cacheDir()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	mk := func(dir, name string, age time.Duration) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
		return p
	}
	gone := []string{
		mk(tmp, "katharsis-run-1", 2*time.Hour),
		mk(tmp, "katharsis-probe-1", 2*time.Hour),
	}
	kept := []string{
		mk(tmp, "katharsis-run-2", time.Minute), // a run in progress
		mk(tmp, "someone-else", 2*time.Hour),
		mk(cache, "build-new", time.Minute),
	}
	Sweep()
	// The cache's own sweep runs once per process; check it directly.
	old := mk(cache, "build-old", 2*time.Hour)
	cleanDebris(cache, now.Add(-debrisAge), "build-")
	gone = append(gone, old)
	for _, p := range gone {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s survived the sweep", p)
		}
	}
	for _, p := range kept {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was swept", p)
		}
	}
}

// crashReason on reports this machine's toolchains do not produce.
func TestCrashReasonText(t *testing.T) {
	for _, tc := range []struct{ name, stderr, want string }{
		{"glibc assert", "sol: /w/sol.cpp:6: int main(): Assertion `x != 7' failed.\n", "assertion x != 7 failed at sol.cpp:6"},
		{"libc++ hardening", "/nix/store/x/include/c++/v1/vector:1531: libc++ Hardening assertion __n < size() failed: vector[] index out of bounds\n",
			"libc++: vector[] index out of bounds"},
		{"ASan location from the first user frame", "==1==ERROR: AddressSanitizer: container-overflow on address 0x1\n" +
			"    #0 0x1 in std::vector<int>::operator[] /nix/store/x/include/c++/v1/vector:1531:5\n" +
			"    #1 0x2 in main /w/sol.cpp:8:3\n" +
			"SUMMARY: AddressSanitizer: container-overflow /nix/store/x/include/c++/v1/vector:1531:5 in operator[]\n",
			"container-overflow at sol.cpp:8"},
		{"ASan summary cut off", "==1==ERROR: AddressSanitizer: stack-overflow on address 0x1\n    #0 0x1 in f(int) /w/sol.cpp:4:1\n",
			"stack-overflow at sol.cpp:4"},
		{"UBSan in a header", "/nix/store/x/include/c++/v1/__algorithm/sort.h:9:3: runtime error: load of null pointer of type 'int'\n" +
			"    #0 0x1 in std::sort /nix/store/x/include/c++/v1/__algorithm/sort.h:9:3\n    #1 0x2 in main /w/sol.cpp:12:5\n",
			"load of null pointer of type 'int' at sol.cpp:12"},
		{"unsymbolized", "SUMMARY: AddressSanitizer: SEGV (sol+0x1234) in main\n", "SEGV"},
	} {
		if got := crashReason(tc.stderr, nil, Full); got != tc.want {
			t.Errorf("%s: crashReason = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestFileStems(t *testing.T) {
	for _, tc := range []struct {
		in   []string
		want []string
	}{
		{[]string{"", ""}, nil},
		{[]string{"paint", ""}, []string{"paint"}},
		{[]string{"paint", "paint"}, []string{"paint"}},
		{[]string{"milk", "milk2"}, []string{"milk", "milk2"}},
		{[]string{"../x", "a/b", ".", "..", "x\x00"}, nil},
	} {
		got := fileStems(tc.in...)
		if len(got) != len(tc.want) {
			t.Errorf("fileStems(%q) = %q, want %q", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("fileStems(%q) = %q, want %q", tc.in, got, tc.want)
			}
		}
	}
}
