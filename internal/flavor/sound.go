package flavor

import (
	"os"
	"path/filepath"
)

// soundDir holds the macOS system alert sounds. Other systems lack it, and
// SoundFile then returns "" so the app stays silent instead of failing.
const soundDir = "/System/Library/Sounds"

// sounds picks a system alert per game.Sound name, by feel:
//   - ac: Glass, the bright chime.
//   - wa: Basso, the low thud of a failed alert.
//   - crash: Sosumi, the classic Mac error sound (and a lawsuit joke).
//   - commit: Pop, a short confirm for banked bytes.
//   - boss: Submarine, sonar from something large below you.
//   - cleared: Hero, the closest thing macOS ships to a fanfare.
var sounds = map[string]string{
	"ac":      "Glass.aiff",
	"wa":      "Basso.aiff",
	"crash":   "Sosumi.aiff",
	"commit":  "Pop.aiff",
	"boss":    "Submarine.aiff",
	"cleared": "Hero.aiff",
}

// soundFile checks the file on every call: the check is one stat, and it
// keeps a missing sound (a trimmed macOS install) from reaching afplay.
func soundFile(name string) string {
	f, ok := sounds[name]
	if !ok {
		return ""
	}
	p := filepath.Join(soundDir, f)
	if st, err := os.Stat(p); err != nil || !st.Mode().IsRegular() {
		return ""
	}
	return p
}
