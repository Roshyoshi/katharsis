package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/x/term"

	"katharsis/internal/rules"
)

// presetDir is where custom rule presets are read from:
// $XDG_CONFIG_HOME/katharsis/presets, else ~/.config/katharsis/presets. It
// is XDG on macOS too, as the store's ~/.local/share is: a terminal game's
// settings belong with the player's dotfiles, not in ~/Library.
func presetDir() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" && filepath.IsAbs(d) {
		return filepath.Join(d, "katharsis", "presets"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "katharsis", "presets"), nil
}

// presets collects custom rule presets by name, for app.Deps.Presets.
type presets struct {
	byName map[string]rules.Rules
	from   map[string]string // name → the file it came from
}

func newPresets() *presets {
	return &presets{byName: map[string]rules.Rules{}, from: map[string]string{}}
}

// loadDir adds every *.toml in dir, in name order. A file that does not
// load is skipped and reported, so one bad preset never keeps the game
// from starting; a missing dir is no presets.
func (p *presets) loadDir(dir string) (errs []error) {
	files, _ := filepath.Glob(filepath.Join(dir, "*.toml"))
	for _, f := range files {
		if err := p.add(f); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// add loads one preset file on top of rules.Standard(). It is named by
// its `name` key, else its file name without .toml; the name is what GRUB
// lists and what the boot records, so "standard" stays the built-in's and
// two files cannot share one. A key the rules do not have is an error, not
// a silent no-op: a misspelt override would play as standard while the
// player believes it changed.
func (p *presets) add(path string) error {
	r, err := rules.Load(path)
	if err != nil {
		if !strings.Contains(err.Error(), path) {
			err = fmt.Errorf("%s: %w", path, err)
		}
		return err
	}
	md, err := toml.DecodeFile(path, &rules.Rules{})
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if keys := md.Undecoded(); len(keys) > 0 {
		names := make([]string, len(keys))
		for i, k := range keys {
			names[i] = k.String()
		}
		return fmt.Errorf("%s: unknown keys %s", path, strings.Join(names, ", "))
	}

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if md.IsDefined("name") {
		name = r.Name
	}
	switch prev, taken := p.from[name]; {
	case strings.TrimSpace(name) == "":
		return fmt.Errorf("%s: empty preset name", path)
	case name == "standard":
		return fmt.Errorf("%s: %q is the built-in preset; name this one something else", path, name)
	case taken:
		return fmt.Errorf("%s: preset name %q is already taken by %s", path, name, prev)
	}
	r.Name = name
	p.byName[name], p.from[name] = r, path
	return nil
}

// warnPresets reports the preset files that did not load the way GRUB
// reports a bad entry, and waits for a key: the TUI takes the screen next,
// and anything on stderr would be gone before it was read.
func warnPresets(errs []error) {
	if len(errs) == 0 {
		return
	}
	for _, err := range errs {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	fmt.Fprintln(os.Stderr, "Skipped those presets. Press any key to continue...")
	fd := os.Stdin.Fd()
	if !term.IsTerminal(fd) {
		return
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return
	}
	defer func() { _ = term.Restore(fd, old) }()
	// One read takes a whole escape sequence (an arrow key), so none of it
	// is left for the TUI to misread. Any outcome, even an error, is the
	// "key" that continues.
	_, _ = os.Stdin.Read(make([]byte, 64))
}
