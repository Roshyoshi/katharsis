package judge

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A probe cut short by its context proves nothing about this machine, so the
// next SampleProfile must probe again rather than keep the fallback.
//
// Not parallel: it resets the process-wide probe while parallel tests are
// still parked.
func TestSampleProfileAfterCancelledProbe(t *testing.T) {
	if _, err := exec.LookPath(Compiler); err != nil {
		t.Skipf("%s not on PATH", Compiler)
	}
	home := t.TempDir() // an empty disk cache
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	saved := probed
	probed = nil
	t.Cleanup(func() { probed = saved })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if p := SampleProfile(ctx); p.Name != HardenedSamples.Name {
		t.Fatalf("cancelled probe chose %q, want the %q fallback", p.Name, HardenedSamples.Name)
	}
	live := SampleProfile(context.Background())
	dir, err := cacheDir()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(dir), "sample-profile-"+probeKey()))
	if err != nil || strings.TrimSpace(string(b)) != live.Name {
		t.Errorf("SampleProfile = %q without probing again (disk cache %q, %v)", live.Name, b, err)
	}
}
