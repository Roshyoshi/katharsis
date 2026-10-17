package main

import (
	"testing"
	"time"

	"katharsis/internal/app"
	"katharsis/internal/audio"
	"katharsis/internal/rules"
)

// passwd's rows say what set each value: the default, the setting's file
// when it differs from the default, or a flag given at launch, which wins.
func TestSettingRows(t *testing.T) {
	find := func(rows []app.Setting, key string) app.Setting {
		for _, r := range rows {
			if r.Key == key {
				return r
			}
		}
		t.Fatalf("no row %q", key)
		return app.Setting{}
	}
	base := launch{set: map[string]bool{}, editor: "vim", split: 50, scale: 1}

	rows := settingRows(defaultSettings(), audio.DefaultSettings(), base)
	for _, r := range rows {
		if r.Note != "default" {
			t.Errorf("%s = %s from %q at the defaults", r.Key, r.Value, r.Note)
		}
	}
	if r := find(rows, "session_budget"); r.Value != "3h" {
		t.Errorf("session_budget reads %q, want 3h", r.Value)
	}

	s := defaultSettings()
	s.SetuidSpoilers, s.SessionBudget = "all", rules.Duration{Duration: 90 * time.Minute}
	a := audio.DefaultSettings()
	a.Music = false
	l := base
	l.set = map[string]bool{"session-budget": true, "no-music": true, "split": true}
	l.split = 60
	rows = settingRows(s, a, l)
	for key, want := range map[string]app.Setting{
		"setuid_spoilers": {Value: "all", Note: "settings.toml"},
		"session_budget":  {Value: "1h30m", Note: "flag --session-budget"},
		"music":           {Value: "false", Note: "flag --no-music"},
		"split":           {Value: "60%", Note: "flag --split"},
		"sfx":             {Value: "true", Note: "default"},
	} {
		if r := find(rows, key); r.Value != want.Value || r.Note != want.Note {
			t.Errorf("%s = %s from %q, want %s from %q", key, r.Value, r.Note, want.Value, want.Note)
		}
	}
}
