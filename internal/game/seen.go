package game

import "slices"

// SeenScene is a one-time story scene that was shown: its ID (a flavor
// story hook, or a hook and an argument for a family of scenes) and the
// boot it was shown in (Meta.Boots at the time).
type SeenScene struct {
	ID   string
	Boot int
}

// HasSeen reports whether the scene id was shown.
func HasSeen(m *Meta, id string) bool {
	return slices.ContainsFunc(m.Seen, func(s SeenScene) bool { return s.ID == id })
}

// MarkSeen records the scene id as shown in the current boot. It reports
// false, and changes nothing, when id was already seen, so a scene that two
// screens could show is shown once.
func MarkSeen(m *Meta, id string) bool {
	if id == "" || HasSeen(m, id) {
		return false
	}
	m.Seen = append(m.Seen, SeenScene{ID: id, Boot: m.Boots})
	return true
}

// MarkOpened records that RTFM module id's page was opened in the reader
// (Meta.Opened). It reports false, and changes nothing, when it was opened
// before, so the list keeps the order first opened.
func MarkOpened(m *Meta, id string) bool {
	if id == "" || slices.Contains(m.Opened, id) {
		return false
	}
	m.Opened = append(m.Opened, id)
	return true
}
