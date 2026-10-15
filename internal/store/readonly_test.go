package store_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"katharsis/internal/store"
)

// SQLite opens a file it cannot write read-only without complaint, so the
// game used to play a whole boot on one and save nothing. Open refuses it.
func TestOpenReadOnlyFile(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root writes read-only files")
	}
	path := tempPath(t)
	s := open(t, path)
	checkpoint(t, s, sampleMeta(), midBoot(1, t0))
	mustClose(t, s)
	for _, f := range []string{path + "-wal", path + "-shm"} {
		os.Remove(f)
	}
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	s, err := store.Open(path)
	if err == nil {
		mustClose(t, s)
		t.Fatal("Open of a read-only file succeeded")
	}
	if !errors.Is(err, store.ErrReadOnly) || !strings.Contains(err.Error(), path) {
		t.Fatalf("Open = %v, want ErrReadOnly naming the file", err)
	}
}
