// Package units formats the game's numbers the one way every screen and log
// shows them (docs/design/ui-system.md, "Number formats"): the byte
// currency, memory, clocks, countdowns, long spans and hearts. It imports
// nothing from the game, so the engine can use it as well as the screens.
// Plural counts nouns the same way everywhere ("1 boot", "2 boots").
package units

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Bytes formats the game currency: whole bytes under 1 KiB, then KiB, MiB
// or GiB with one decimal (".0" trimmed) below 100 and none from 100 up,
// always with a space before the unit: "42 B", "12.5 KiB", "1 MiB",
// "312 KiB".
func Bytes(n int64) string {
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	switch {
	case n < 1<<10:
		return sign + strconv.FormatInt(n, 10) + " B"
	case n < 1<<20:
		return sign + scaled(float64(n)/(1<<10), "KiB")
	case n < 1<<30:
		return sign + scaled(float64(n)/(1<<20), "MiB")
	}
	return sign + scaled(float64(n)/(1<<30), "GiB")
}

func scaled(v float64, unit string) string {
	if v >= 100 {
		return strconv.FormatFloat(v, 'f', 0, 64) + " " + unit
	}
	return strings.TrimSuffix(strconv.FormatFloat(v, 'f', 1, 64), ".0") + " " + unit
}

// Mem formats a memory size (a solution's peak RSS, a limit) compactly and
// without a space, so it never reads as currency: "312B", "1.2K", "48M",
// "1.5G".
func Mem(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0fM", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%dB", n)
}

// Clock formats a duration that has run, such as time spent on a door or
// an uptime: mm:ss, or h:mm:ss from an hour on. It counts whole seconds
// gone, as a stopwatch does, and never goes below 00:00.
func Clock(d time.Duration) string {
	d = max(d, 0).Truncate(time.Second)
	h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// Countdown formats the time left before a deadline like Clock, but rounds
// up, so it reads 00:00 only once the deadline has passed. With the same
// limit, Clock of the time gone and Countdown of the time left always add
// up to it.
func Countdown(left time.Duration) string {
	if r := left % time.Second; r > 0 {
		left += time.Second - r
	}
	return Clock(left)
}

// HM formats a long span, a day's play or a session budget, as hours and
// minutes: "3:12". It counts whole minutes gone, as Clock counts seconds.
func HM(d time.Duration) string {
	d = max(d, 0).Truncate(time.Minute)
	return fmt.Sprintf("%d:%02d", int(d.Hours()), int(d.Minutes())%60)
}

// Hearts is the cycles pool (D48: lives renamed cycles, still drawn as ♥)
// as one heart and a count, "♥ 24/30": a row of 30 hearts fits no line.
// of is what the count is out of, the pool the boot was dealt; a heal can
// take n past it, "♥ 38/30".
func Hearts(n, of int) string {
	return fmt.Sprintf("♥ %d/%d", max(n, 0), of)
}

// Cycles is Hearts with its unit, "♥ 24/30 cycles", for a line with room
// for the word.
func Cycles(n, of int) string { return Hearts(n, of) + " cycles" }

// Plural counts n of a regular English noun: "1 boot", "2 boots",
// "3 matches", "2 crashes", "4 entries". Irregular nouns (life, lives)
// spell their own forms.
func Plural(n int, noun string) string {
	s := strconv.Itoa(n) + " "
	if n == 1 || noun == "" {
		return s + noun
	}
	switch last := noun[len(noun)-1]; {
	case strings.HasSuffix(noun, "ch"), strings.HasSuffix(noun, "sh"),
		last == 's', last == 'x', last == 'z':
		return s + noun + "es"
	case last == 'y' && len(noun) > 1 && !strings.ContainsRune("aeiou", rune(noun[len(noun)-2])):
		return s + noun[:len(noun)-1] + "ies"
	}
	return s + noun + "s"
}
