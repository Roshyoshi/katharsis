package game

import "slices"

// link sets the boot's linked libs. It is a choice made at the boot's start:
// once a door has opened, its template already holds what was linked, so
// changing the set later would mean nothing.
func (e *engine) link(names []string) error {
	s, m := e.s, e.meta
	if s.Current != nil || len(s.History) > 0 {
		return invalid("libs link only before the first door")
	}
	if len(names) > s.LinkSlots() {
		return invalid("%d libs, %d slots", len(names), s.LinkSlots())
	}
	var linked []string
	for _, n := range names {
		if !slices.ContainsFunc(m.Libs, func(l Lib) bool { return l.Name == n }) {
			return invalid("no lib %q", n)
		}
		if !slices.Contains(linked, n) {
			linked = append(linked, n)
		}
	}
	s.Linked = linked
	if len(linked) > 0 {
		e.logf("insmod %v", linked)
	}
	return nil
}

// LinkedLibs returns the libs linked this boot, in link order, with their
// sources from meta.
func LinkedLibs(s *State, m *Meta) []Lib {
	var out []Lib
	for _, n := range s.Linked {
		for _, l := range m.Libs {
			if l.Name == n {
				out = append(out, l)
			}
		}
	}
	return out
}
