package game

import (
	"cmp"
	"slices"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/rules"
)

// Upsolve records a solve of id made outside a boot and returns the core-dump
// bytes it recovered into the bank (0 unless id holds the dump).
//
// A non-editorial upsolve made on or after a regression's due date counts as
// that stage's re-solve: the regression moves to its next stage, or is
// retired after the last. Earlier upsolves leave the schedule alone, since
// the point is to re-solve from a blank file days later. An Editorial
// upsolve never advances the schedule: reading the answer is not recall.
func Upsolve(m *Meta, id catalog.ID, kind SolveKind, elapsed time.Duration, now time.Time) (recovered int64) {
	m.Solves = append(m.Solves, Solve{
		Problem: id,
		Kind:    kind,
		At:      now,
		Elapsed: max(elapsed, 0), // a clock that stepped back, as in clear
		Boot:    m.Boots,
		Upsolve: true,
	})
	if m.CoreDump != nil && m.CoreDump.Problem == id {
		recovered = m.CoreDump.Bytes
		m.Bank += recovered
		m.CoreDump = nil
	}
	if kind != Editorial {
		advanceRegression(m, id, now)
	}
	return recovered
}

// DueRegressions lists the regressions due at now, oldest due first.
func DueRegressions(m *Meta, now time.Time) []Regression {
	var due []Regression
	for _, g := range m.Regressions {
		if !g.Due.After(now) {
			due = append(due, g)
		}
	}
	slices.SortStableFunc(due, func(a, b Regression) int {
		if c := a.Due.Compare(b.Due); c != 0 {
			return c
		}
		return cmp.Compare(a.Problem, b.Problem)
	})
	return due
}

// scheduleRegression (re)starts id's regression at stage 0, days[0] days from
// now. A problem that crashes you again starts over.
func scheduleRegression(m *Meta, id catalog.ID, now time.Time, days []int) (time.Time, bool) {
	if len(days) == 0 {
		return time.Time{}, false
	}
	g := Regression{
		Problem: id,
		Due:     now.AddDate(0, 0, days[0]),
		Days:    slices.Clone(days),
	}
	if i := slices.IndexFunc(m.Regressions, func(g Regression) bool { return g.Problem == id }); i >= 0 {
		m.Regressions[i] = g
	} else {
		m.Regressions = append(m.Regressions, g)
	}
	return g.Due, true
}

// advanceRegression credits a re-solve of id at now, if it is due.
func advanceRegression(m *Meta, id catalog.ID, now time.Time) {
	i := slices.IndexFunc(m.Regressions, func(g Regression) bool { return g.Problem == id })
	if i < 0 || now.Before(m.Regressions[i].Due) {
		return
	}
	g := &m.Regressions[i]
	days := g.Days
	if len(days) == 0 {
		days = rules.Standard().RegressionDays
	}
	g.Stage++
	if g.Stage >= len(days) {
		m.Regressions = slices.Delete(m.Regressions, i, i+1)
		if len(m.Regressions) == 0 {
			m.Regressions = nil // same JSON as a player who never crashed
		}
		return
	}
	g.Due = now.AddDate(0, 0, days[g.Stage])
}
