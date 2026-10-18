package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/problem"
	"katharsis/internal/units"
)

// duckTest is the duck's debugging quack (D64, rules.DuckTest): of the
// tests the open door's last submit failed, the smallest that is not one
// of its samples is copied into the door's tests as the next sample, so
// :w runs it and w shows its input, the expected output and the
// program's. Rubber-duck debugging: the bug is found on a case in hand.
func (m *Model) duckTest() tea.Cmd {
	if m.open == nil {
		return nil
	}
	r, ok := m.d.Runner.(sourceRunner)
	if !ok || r.Source() == "" {
		return fail("the duck cannot reach the door's tests")
	}
	samples := map[string]bool{}
	for _, s := range m.open.Assets.Samples {
		samples[s.In] = true
	}
	var best problem.Test
	size := int64(-1)
	for _, t := range m.open.Failed {
		fi, err := os.Stat(t.In)
		if err != nil || samples[t.In] || size >= 0 && fi.Size() >= size {
			continue
		}
		best, size = t, fi.Size()
	}
	if size < 0 {
		return fail("the duck found no hidden test the last submit failed")
	}
	dir := filepath.Join(filepath.Dir(r.Source()), "tests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fail("duck: %v", err)
	}
	n := len(m.open.Assets.Samples) + 1
	for {
		if _, err := os.Stat(filepath.Join(dir, fmt.Sprint(n)+".in")); err != nil {
			break
		}
		n++
	}
	in, out := filepath.Join(dir, fmt.Sprint(n)+".in"), filepath.Join(dir, fmt.Sprint(n)+".out")
	if err := copyTo(best.In, in); err != nil {
		return fail("duck: %v", err)
	}
	if err := copyTo(best.Out, out); err != nil {
		return fail("duck: %v", err)
	}
	m.open.Assets.Samples = append(slices.Clip(m.open.Assets.Samples), problem.Test{Name: fmt.Sprint(n), In: in, Out: out})
	return note("duck: hidden test #%s is now sample %d (%s in) · :w runs it · w shows it", best.Name, n, units.Bytes(size))
}
