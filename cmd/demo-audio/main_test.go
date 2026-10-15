package main

import (
	"path/filepath"
	"testing"

	"katharsis/internal/audio"
)

// analyze only measures; a path that is not there is an error, not an
// empty table.
func TestAnalyzeMissingPath(t *testing.T) {
	if err := analyze(&audio.Library{}, []string{filepath.Join(t.TempDir(), "nope.mp3")}); err == nil {
		t.Error("analyze of a missing file succeeded")
	}
}
