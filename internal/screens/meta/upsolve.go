package meta

import (
	"fmt"
	"math"
	"slices"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// maxLost is how many doors lost in boots GRUB's upsolves list, newest
// first. As with the due regressions (maxDue, GR10), the rest wait and
// nothing counts them: an upsolve is an offer, never a debt.
const maxLost = 5

// upsolvePage is GRUB's upsolves, nil when there are none: the core dump,
// the regressions due (maxDue of them), then the doors boots lost that are
// not due yet (maxLost), each untimed. A lost door is a regression at its
// first stage not yet due: every timeout and :q! that a boot survives
// schedules one. Upsolving it early leaves the recall test where it is,
// due on its date (game's advanceRegression credits only a due one).
// due reports whether a regression is due, for the White Queen's beat.
func upsolvePage(m game.Meta, now time.Time, title func(catalog.ID) string,
	sub func(string, ...string) string, story func(string, uint64) string) (p *page, due bool) {
	p = &page{pick: true}
	d := m.CoreDump
	dumped := func(id catalog.ID) bool { return d != nil && d.Problem == id }
	if d != nil {
		p.entries = append(p.entries, entry{
			label: fmt.Sprintf("core dump: %s (%s)", title(d.Problem), units.Bytes(d.Bytes)),
			msg:   screens.Upsolve{Problem: d.Problem},
			note:  sub("coredump", "bytes", units.Bytes(d.Bytes), "title", ui.Clip(title(d.Problem), 24)),
		})
	}
	regs := slices.DeleteFunc(dueRegressions(m.Regressions, now), func(r game.Regression) bool { return dumped(r.Problem) })
	for _, r := range regs[:min(len(regs), maxDue)] {
		p.entries = append(p.entries, entry{
			label: fmt.Sprintf("regression: %s (due %s, stage %d)", title(r.Problem), ago(r.Due, now), r.Stage+1),
			msg:   screens.Upsolve{Problem: r.Problem},
			note:  regressionNote(r, story),
		})
	}
	var lost []game.Regression
	for _, r := range m.Regressions {
		if r.Due.After(now) && r.Stage == 0 && !dumped(r.Problem) {
			lost = append(lost, r)
		}
	}
	slices.SortStableFunc(lost, func(a, b game.Regression) int { return b.Due.Compare(a.Due) })
	for _, r := range lost[:min(len(lost), maxLost)] {
		label := "lost: " + title(r.Problem)
		var at time.Time // when the boot lost it; zero when unknown
		if len(r.Days) > 0 {
			at = r.Due.AddDate(0, 0, -r.Days[0])
			label = "lost " + ago(at, now) + ": " + title(r.Problem)
		}
		back := "back as a regression in " + units.Plural(int(math.Ceil(r.Due.Sub(now).Hours()/24)), "day")
		note := "untimed, from where the boot left it; " + back
		// Solved since, it stays listed until its regression is due, and
		// says so: ✓, as RTFM marks a solved problem.
		if slices.ContainsFunc(m.Solves, func(s game.Solve) bool { return s.Problem == r.Problem && !s.At.Before(at) }) {
			label += " " + ui.G.Pass
			note = "upsolved; " + back
		}
		p.entries = append(p.entries, entry{label: label, msg: screens.Upsolve{Problem: r.Problem}, note: note})
	}
	if len(p.entries) == 0 {
		return nil, false
	}
	return p, len(regs) > 0
}

// upsolveEntry is GRUB's entry for the upsolves, named by the core dump
// when there is one, the cliffhanger RT4 puts second.
func upsolveEntry(d *game.CoreDump, ups *page, title func(catalog.ID) string, sub func(string, ...string) string) entry {
	e := entry{label: "upsolve…", sub: ups, id: "upsolve", note: sub("upsolve")}
	if d != nil {
		e.label = fmt.Sprintf("upsolve… (core dump: %s, %s)", title(d.Problem), units.Bytes(d.Bytes))
		e.note = sub("coredump", "bytes", units.Bytes(d.Bytes), "title", ui.Clip(title(d.Problem), 24))
	}
	return e
}
