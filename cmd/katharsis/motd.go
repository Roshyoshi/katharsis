package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"katharsis/internal/game"
	"katharsis/internal/store"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// runMotd is `katharsis motd` (retention RT14): one line for a new shell
// when something is due, or nothing. It reads the store read-only and
// never writes it. Opt in from ~/.zshrc:
//
//	command -v katharsis >/dev/null && katharsis motd
func runMotd(args []string, out, errOut io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(errOut, "usage: katharsis motd")
		return 2
	}
	path, err := store.DefaultPath()
	if err != nil {
		fmt.Fprintln(errOut, "katharsis motd:", err)
		return 1
	}
	line, err := motd(path, time.Now())
	if err != nil {
		fmt.Fprintln(errOut, "katharsis motd:", err)
		return 1
	}
	if line != "" {
		fmt.Fprintln(out, line)
	}
	return 0
}

// motd is the line for the store at path as of now, "" when nothing is
// due: regressions due (at most the count, GR10), a core dump to upsolve,
// a nightly not yet played. Nothing in it expires (GR2), and no one
// played yet is nothing.
func motd(path string, now time.Time) (string, error) {
	s, err := store.OpenReadOnly(path)
	if errors.Is(err, store.ErrNoStore) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer func() { _ = s.Close() }()
	m, err := s.LoadMeta()
	if err != nil {
		return "", err
	}
	var parts []string
	if n := len(game.DueRegressions(&m, now)); n > 0 {
		parts = append(parts, units.Plural(n, "regression")+" due")
	}
	if d := m.CoreDump; d != nil && d.Bytes > 0 {
		parts = append(parts, ui.G.Crash+" "+units.Bytes(d.Bytes)+" core dump to upsolve")
	}
	played, err := s.Played(game.NightlySeed(now))
	if err != nil {
		return "", err
	}
	if !played {
		parts = append(parts, "nightly "+now.Format("01-02"))
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "katharsis: " + strings.Join(parts, ui.G.Sep), nil
}
