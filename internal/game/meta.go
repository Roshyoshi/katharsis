package game

import (
	"cmp"
	"math"
	"slices"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/rules"
)

// Upsolve records a solve of id made outside a boot and returns the core-dump
// bytes it recovered into the bank (0 unless id holds the dump). An
// independent upsolve, the editorial never opened, recovers the dump times
// its Mult (retention RT4); the bytes past the dump are new earnings, so
// they count toward LifetimeBytes too. A Mult under 1 (a preset's) takes
// nothing back from LifetimeBytes: the doors paid the dump when cleared.
//
// A non-editorial upsolve made on or after a regression's due date counts as
// that stage's re-solve: the regression moves to its next stage, or is
// retired after the last. Earlier upsolves leave the schedule alone, since
// the point is to re-solve from a blank file days later. An Editorial
// upsolve never advances the schedule: reading the answer is not recall.
func Upsolve(m *Meta, id catalog.ID, kind SolveKind, elapsed time.Duration, now time.Time) (recovered int64) {
	if now.After(m.LastPlayed) {
		m.LastPlayed = now
	}
	m.Solves = append(m.Solves, Solve{
		Problem: id,
		Kind:    kind,
		At:      now,
		Elapsed: max(elapsed, 0), // a clock that stepped back, as in clear
		Boot:    m.Boots,
		Upsolve: true,
	})
	if d := m.CoreDump; d != nil && d.Problem == id {
		recovered = d.Recovers(kind)
		m.Bank += recovered
		m.LifetimeBytes += max(recovered-d.Bytes, 0)
		m.CoreDump = nil
	}
	if kind != Editorial {
		advanceRegression(m, id, now)
	}
	return recovered
}

// Recovers is what an upsolve of kind banks from d: Bytes, times Mult for
// an independent one (no editorial, no duck), rounded to the byte.
func (d CoreDump) Recovers(kind SolveKind) int64 {
	if kind != Independent {
		return d.Bytes
	}
	mult := d.Mult
	if mult <= 0 {
		mult = rules.Standard().DumpIndependentMult
	}
	return int64(math.Round(float64(d.Bytes) * mult))
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
		m.Retired++
		if len(m.Regressions) == 0 {
			m.Regressions = nil // same JSON as a player who never crashed
		}
		return
	}
	g.Due = now.AddDate(0, 0, days[g.Stage])
}
