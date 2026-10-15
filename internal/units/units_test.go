package units

import (
	"testing"
	"time"
)

func TestBytes(t *testing.T) {
	for n, want := range map[int64]string{
		0: "0 B", 312: "312 B", 1023: "1023 B", 1024: "1 KiB", 1331: "1.3 KiB",
		1536: "1.5 KiB", 10 << 10: "10 KiB", 15565: "15.2 KiB", 150 << 10: "150 KiB",
		1 << 20: "1 MiB", 3 << 19: "1.5 MiB", 300 << 20: "300 MiB", 3 << 30: "3 GiB",
		2048 << 30: "2048 GiB", -312: "-312 B", -1536: "-1.5 KiB",
	} {
		if got := Bytes(n); got != want {
			t.Errorf("Bytes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestMem(t *testing.T) {
	for n, want := range map[int64]string{
		0: "0B", 312: "312B", 1229: "1.2K", 48 << 20: "48M", 3 << 29: "1.5G",
	} {
		if got := Mem(n); got != want {
			t.Errorf("Mem(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestClock(t *testing.T) {
	s := time.Second
	for _, c := range []struct {
		d    time.Duration
		want string
	}{
		{0, "00:00"}, {-5 * s, "00:00"}, {999 * time.Millisecond, "00:00"},
		{521*s + 600*time.Millisecond, "08:41"}, {15 * time.Minute, "15:00"},
		{time.Hour - time.Millisecond, "59:59"}, {time.Hour, "1:00:00"},
		{4350 * s, "1:12:30"}, {30 * time.Hour, "30:00:00"},
	} {
		if got := Clock(c.d); got != c.want {
			t.Errorf("Clock(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestCountdown(t *testing.T) {
	s := time.Second
	for _, c := range []struct {
		d    time.Duration
		want string
	}{
		{5 * time.Minute, "05:00"}, {5*time.Minute - time.Millisecond, "05:00"},
		{299 * s, "04:59"}, {time.Millisecond, "00:01"}, {0, "00:00"},
		{-time.Millisecond, "00:00"}, {-2 * s, "00:00"},
		{time.Hour - time.Millisecond, "1:00:00"}, {4350 * s, "1:12:30"},
	} {
		if got := Countdown(c.d); got != c.want {
			t.Errorf("Countdown(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

// The time gone and the time left read as the whole limit at every instant,
// so a pane counting up and a border counting down never disagree.
func TestClockAndCountdownAddUp(t *testing.T) {
	limit := 15 * time.Minute
	for gone := time.Duration(0); gone <= 3*time.Second; gone += 100 * time.Millisecond {
		a, _ := time.ParseDuration(minsec(Clock(gone)))
		b, _ := time.ParseDuration(minsec(Countdown(limit - gone)))
		if a+b != limit {
			t.Errorf("at %v gone: %s + %s != %v", gone, Clock(gone), Countdown(limit-gone), limit)
		}
	}
}

// minsec turns "mm:ss" into "mmmss" for time.ParseDuration.
func minsec(c string) string { return c[:2] + "m" + c[3:] + "s" }

func TestHM(t *testing.T) {
	for _, c := range []struct {
		d    time.Duration
		want string
	}{
		{0, "0:00"}, {-time.Minute, "0:00"}, {59 * time.Second, "0:00"},
		{3*time.Hour + 12*time.Minute + 40*time.Second, "3:12"}, {27 * time.Hour, "27:00"},
	} {
		if got := HM(c.d); got != c.want {
			t.Errorf("HM(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestHearts(t *testing.T) {
	for _, c := range []struct {
		lives, max int
		want       string
	}{
		{2, 3, "♥♥♡"}, {3, 3, "♥♥♥"}, {0, 3, "♡♡♡"}, {-1, 2, "♡♡"},
		{4, 3, "♥♥♥♥"}, {7, 12, "♥ 7/12"}, {11, 3, "♥ 11/3"},
	} {
		if got := Hearts(c.lives, c.max); got != c.want {
			t.Errorf("Hearts(%d, %d) = %q, want %q", c.lives, c.max, got, c.want)
		}
	}
}

func TestPlural(t *testing.T) {
	for _, c := range []struct {
		n    int
		noun string
		want string
	}{
		{1, "boot", "1 boot"}, {2, "boot", "2 boots"}, {0, "boot", "0 boots"},
		{1, "crash", "1 crash"}, {2, "crash", "2 crashes"}, {3, "match", "3 matches"},
		{2, "box", "2 boxes"}, {2, "pass", "2 passes"}, {4, "entry", "4 entries"},
		{2, "day", "2 days"}, {-1, "module", "-1 modules"}, {2, "", "2 "},
	} {
		if got := Plural(c.n, c.noun); got != c.want {
			t.Errorf("Plural(%d, %q) = %q, want %q", c.n, c.noun, got, c.want)
		}
	}
}
