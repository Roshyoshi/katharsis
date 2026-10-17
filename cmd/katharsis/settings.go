package main

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"katharsis/internal/anim"
	"katharsis/internal/app"
	"katharsis/internal/audio"
	"katharsis/internal/judge"
	"katharsis/internal/rules"
	"katharsis/internal/screens/meta"
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
	// SetuidNotices is whether the boot screens say a setuid bit set: the
	// strip's chmod u+s line and the cleared screen's kept line
	// (docs/design/achievements.md (h)). Off, the find / -perm -4000
	// screen still records every bit (GR6: every notice tunable or off).
	SetuidNotices bool `toml:"setuid_notices"`
	// SetuidSpoilers is what that screen shows of a secret not set yet:
	// "none" (the default, -?????????), "hints" or "all".
	SetuidSpoilers string `toml:"setuid_spoilers"`
	// SampleWarnings are the warning flags the samples build with (:w),
	// never :Submit, which matches the online judge. The default is
	// judge.SampleWarnings: an uninitialized read fails the build, the rest
	// are reported beside the samples. [] turns them off.
	SampleWarnings []string `toml:"sample_warnings"`
}

func defaultSettings() settings {
	return settings{
		SessionBudget:  rules.Duration{Duration: store.DefaultRetention().SessionBudget},
		SetuidNotices:  true,
		SetuidSpoilers: meta.SpoilersNone,
		SampleWarnings: judge.SampleWarnings,
	}
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
	if !slices.Contains(meta.Spoilers, s.SetuidSpoilers) {
		return defaultSettings(), fmt.Errorf("%s: setuid_spoilers %q: want none, hints or all", path, s.SetuidSpoilers)
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

// launch is what passwd shows of the flags: the values runGame took, and
// which the player gave.
type launch struct {
	set    map[string]bool // flag names given on the command line
	motion anim.Motion
	editor string
	split  int
	scale  float64
}

// settingRows are passwd's rows: each setting in effect and what set it. A
// value the same as the default reads as the default, wherever it was
// written.
func settingRows(s settings, a audio.Settings, l launch) []app.Setting {
	d, ad := defaultSettings(), audio.DefaultSettings()
	row := func(key, value string, changed bool, file string, flags ...string) app.Setting {
		note := "default"
		for _, f := range flags {
			if l.set[f] {
				return app.Setting{Key: key, Value: value, Note: "flag --" + f}
			}
		}
		if changed {
			note = file
		}
		return app.Setting{Key: key, Value: value, Note: note}
	}
	const st, at = "settings.toml", "audio.toml"
	budget := "off"
	if b := s.SessionBudget.Duration; b > 0 {
		budget = strings.TrimSuffix(b.Round(time.Minute).String(), "0s") // "1h30m", "45m"
		if strings.HasSuffix(budget, "h0m") {
			budget = strings.TrimSuffix(budget, "0m") // "3h"
		}
	}
	warn := strings.Join(s.SampleWarnings, " ")
	if warn == "" {
		warn = "none"
	}
	motion := "full"
	if l.motion.Reduced {
		motion = "reduced"
	}
	vol := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	return []app.Setting{
		row("session_budget", budget, s.SessionBudget != d.SessionBudget, st, "session-budget"),
		row("setuid_notices", strconv.FormatBool(s.SetuidNotices), s.SetuidNotices != d.SetuidNotices, st),
		row("setuid_spoilers", s.SetuidSpoilers, s.SetuidSpoilers != d.SetuidSpoilers, st),
		row("sample_warnings", warn, !slices.Equal(s.SampleWarnings, d.SampleWarnings), st),
		row("music", strconv.FormatBool(a.Music), a.Music != ad.Music, at, "quiet", "no-music"),
		row("sfx", strconv.FormatBool(a.SFX), a.SFX != ad.SFX, at, "quiet", "no-sfx"),
		row("music_volume", vol(a.MusicVolume), a.MusicVolume != ad.MusicVolume, at, "music-volume"),
		row("sfx_volume", vol(a.SFXVolume), a.SFXVolume != ad.SFXVolume, at, "sfx-volume"),
		row("motion", motion, false, "", "motion", "fps"),
		row("editor", l.editor, false, "", "editor"),
		row("split", strconv.Itoa(l.split)+"%", false, "", "split"),
		row("scale", vol(l.scale), false, "", "scale"),
	}
}
