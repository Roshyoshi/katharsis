package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	// The HUD's submits compile through judge.Compile, which caches under
	// os.UserCacheDir ($HOME/Library/Caches on macOS). Point HOME at a
	// scratch dir: the real cache stays out of the tests, and every run
	// starts cold. A warm cache judged a submit inside the harness's 30 ms
	// step, before the tests could look at the clock it held.
	home, err := os.MkdirTemp("", "katharsis-app-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for k, v := range map[string]string{"HOME": home, "XDG_CACHE_HOME": filepath.Join(home, ".cache")} {
		if err := os.Setenv(k, v); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}
