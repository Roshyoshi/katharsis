package main

import (
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"katharsis/internal/rules"
)

func writePreset(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Every file loads on top of standard, named by its name key or file; the
// built-ins' names (standard, short, patch) and a name already loaded are
// refused, each error naming its file.
func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	writePreset(t, dir, "fast.toml", "lives = 2\n")
	writePreset(t, dir, "b.toml", "name = \"hard\"\nlives = 1\n")
	writePreset(t, dir, "broken.toml", "lives = \n")
	writePreset(t, dir, "typo.toml", "lifes = 2\n")
	writePreset(t, dir, "invalid.toml", "lives = 0\n")
	writePreset(t, dir, "standard.toml", "lives = 9\n")
	writePreset(t, dir, "short.toml", "lives = 9\n")
	writePreset(t, dir, "mine.toml", "name = \"patch\"\nlives = 9\n")
	writePreset(t, dir, "z.toml", "name = \"fast\"\n")
	writePreset(t, dir, "notes.txt", "not a preset")

	ps := newPresets()
	errs := ps.loadDir(dir)
	if got, want := slices.Sorted(maps.Keys(ps.byName)), []string{"fast", "hard"}; !reflect.DeepEqual(got, want) {
		t.Errorf("loaded %v, want %v", got, want)
	}
	if r := ps.byName["hard"]; r.Name != "hard" || r.Lives != 1 || r.Ducks != rules.Standard().Ducks {
		t.Errorf("hard = %+v: want its name, lives 1 and the rest standard", r)
	}
	if r := ps.byName["fast"]; r.Name != "fast" || r.Lives != 2 {
		t.Errorf("fast = %+v: want it named after its file, lives 2", r)
	}

	// One error per bad file, each naming the file and what is wrong.
	want := map[string]string{
		"broken.toml":   "toml",
		"typo.toml":     "unknown keys lifes",
		"invalid.toml":  "need lives",
		"standard.toml": "built-in",
		"short.toml":    "built-in",
		"mine.toml":     `"patch" is a built-in`,
		"z.toml":        "already taken by " + filepath.Join(dir, "fast.toml"),
	}
	if len(errs) != len(want) {
		t.Errorf("%d errors, want %d: %v", len(errs), len(want), errs)
	}
	for _, err := range errs {
		msg := err.Error()
		file := strings.TrimPrefix(strings.SplitN(msg, ":", 2)[0], dir+string(filepath.Separator))
		if w, ok := want[file]; !ok || !strings.Contains(msg, w) {
			t.Errorf("error %q: want it to name its file and say %q", msg, want[file])
		}
	}
}

func TestLoadDirMissing(t *testing.T) {
	ps := newPresets()
	if errs := ps.loadDir(filepath.Join(t.TempDir(), "nope")); errs != nil || len(ps.byName) != 0 {
		t.Errorf("missing dir: errs %v, presets %v; want neither", errs, slices.Sorted(maps.Keys(ps.byName)))
	}
}

func TestPresetDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/x/config")
	if d, _ := presetDir(); d != "/x/config/katharsis/presets" {
		t.Errorf("with XDG_CONFIG_HOME: %s", d)
	}
	t.Setenv("XDG_CONFIG_HOME", "relative/ignored")
	t.Setenv("HOME", "/x/home")
	if d, _ := presetDir(); d != "/x/home/.config/katharsis/presets" {
		t.Errorf("default: %s", d)
	}
}
