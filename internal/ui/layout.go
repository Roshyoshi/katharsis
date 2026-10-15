package ui

import "strings"

// Layout is the last step of a View: every line clipped to width, the
// footer pinned to the bottom row, and the result exactly height lines, so
// a screen never wraps or scrolls the terminal. A body line holding
// newlines counts as several. height <= 0 keeps every line.
func Layout(width, height int, body []string, footer string) string {
	var out []string
	for _, l := range body {
		for _, s := range strings.Split(l, "\n") {
			out = append(out, Clip(s, width))
		}
	}
	if height > 0 {
		room := height
		if footer != "" {
			room--
		}
		if len(out) > room {
			out = out[:max(room, 0)]
		}
		out = PadLines(out, room)
	}
	if footer != "" {
		out = append(out, Clip(footer, width))
	}
	return strings.Join(out, "\n")
}
