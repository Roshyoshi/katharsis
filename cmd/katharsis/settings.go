package main

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"

	"katharsis/internal/rules"
	"katharsis/internal/store"
)

// settings are the player's game preferences, read from settings.toml
// beside the presets dir ($XDG_CONFIG_HOME/katharsis, else
// ~/.config/katharsis). A file lists only what it changes. Unlike rules
// they are not locked per boot: they change no outcome.
type settings struct {
	// SessionBudget is the day's play past which the end of a boot says
	// so, in one line that never blocks (retention RT12). "0" turns it
	// off. The default is retention's: 3 h.
	SessionBudget rules.Duration `toml:"session_budget"`
}

func defaultSettings() settings {
	return settings{SessionBudget: rules.Duration{Duration: store.DefaultRetention().SessionBudget}}
}

// settingsPath is settings.toml in the config dir the presets dir sits in.
func settingsPath() (string, error) {
	dir, err := presetDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(dir), "settings.toml"), nil
}

// loadSettings reads path over the defaults. A missing file is the
// defaults; a key settings do not have is an error, as in a preset, so a
// misspelt one is not silently ignored.
func loadSettings(path string) (settings, error) {
	s := defaultSettings()
	md, err := toml.DecodeFile(path, &s)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return defaultSettings(), nil
	case err != nil:
		return defaultSettings(), fmt.Errorf("%s: %w", path, err)
	}
	if un := md.Undecoded(); len(un) > 0 {
		return defaultSettings(), fmt.Errorf("%s: unknown setting %q", path, un[0].String())
	}
	if s.SessionBudget.Duration < 0 {
		return defaultSettings(), fmt.Errorf("%s: session_budget %v is negative", path, s.SessionBudget.Duration)
	}
	return s, nil
}

// openSettings is the settings file with the flags over it: a negative
// flag value is unset. A bad file is reported and the defaults used.
func openSettings(budget time.Duration) settings {
	s := defaultSettings()
	path, err := settingsPath()
	if err == nil {
		s, err = loadSettings(path)
	}
	if err != nil {
		warnKey("settings: ", []error{err}, "Playing with the default settings.")
	}
	if budget >= 0 {
		s.SessionBudget.Duration = budget
	}
	return s
}
