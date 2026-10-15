package meta_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/screens/meta"
	"katharsis/internal/screens/meta/fixture"
)

// retention is the store's retention fixture (store/retention_test.go) as
// the app maps it: practice on 08-01, 09-20, 10-10 and 10-12 to 10-15.
func retention() *meta.Retention {
	edt := time.FixedZone("EDT", -4*3600)
	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, edt) }
	now := time.Date(2026, 10, 15, 21, 0, 0, 0, edt)
	r := &meta.Retention{
		Now: now, Load: [3]float64{5.0 / 7, 6.0 / 30, 7.0 / 90},
		MedianSession: 57*time.Minute + 30*time.Second, MedianReturn: 40*time.Hour + 40*time.Minute,
		Replay: meta.Ratio{N: 1, Of: 4}, Upsolve: meta.Ratio{N: 1, Of: 2}, Late: meta.Ratio{N: 2, Of: 7},
		Worth:  meta.Worth{Yes: 2, No: 2, Skip: 1},
		Budget: 3 * time.Hour, LateWeek: 2,
		Fortnights: [2]meta.Fortnight{
			{Play: 210 * time.Minute, Worth: meta.Worth{Yes: 2}},
			{Play: 234 * time.Minute, Worth: meta.Worth{Yes: 2, No: 2, Skip: 1}},
		},
		Alarms: []string{"worth_it", "hours_vs_worth"},
	}
	played := map[time.Time]bool{day(8, 1): true, day(9, 20): true, day(10, 10): true,
		day(10, 12): true, day(10, 13): true, day(10, 14): true, day(10, 15): true}
	for k := 90; k >= 0; k-- {
		r.Days = append(r.Days, played[day(10, 15).AddDate(0, 0, -k)])
	}
	for i, w := range [][3]int{{0, 0, 0}, {0, 0, 0}, {0, 0, 0}, {1, 1, 210}, {0, 0, 0}, {0, 0, 0}, {1, 1, 17}, {4, 4, 217}} {
		r.Weeks = append(r.Weeks, meta.RetentionWeek{Start: day(8, 24).AddDate(0, 0, 7*i),
			Days: w[0], Sessions: w[1], Play: time.Duration(w[2]) * time.Minute})
	}
	return r
}

func retentionTop() *meta.Top {
	st := fixture.Stats()
	st.Retention = retention()
	return meta.NewTop(st)
}

func TestTopRetention(t *testing.T) {
	for _, sz := range sizes {
		name := fmt.Sprintf("top_retention_%dx%d", sz[0], sz[1])
		s := retentionTop()
		view := s.View(sz[0], sz[1])
		if !strings.Contains(plain(view), "! 2 of 4 worth-it answers in 14 days were no (+1 more) · r retention") {
			t.Errorf("%s: topics view lacks the alarm line:\n%s", name, plain(view))
		}
		press(t, s, "r")
		golden(t, name, s.View(sz[0], sz[1]))
	}

	s := retentionTop()
	press(t, s, "r", "r")
	if v := plain(s.View(80, 24)); !strings.Contains(v, "  TOPIC") || !strings.Contains(v, "rRetention") {
		t.Errorf("r twice is not back on the topics:\n%s", v)
	}
	// Without a panel, r does nothing and the key bar does not offer it.
	plainTop := meta.NewTop(fixture.Stats())
	press(t, plainTop, "r")
	if v := plain(plainTop.View(80, 24)); !strings.Contains(v, "  TOPIC") || strings.Contains(v, "Retention") {
		t.Errorf("top without retention:\n%s", v)
	}

	for _, sz := range [][2]int{{1, 1}, {20, 6}, {40, 12}, {60, 15}, {80, 24}, {140, 40}, {220, 60}} {
		s := retentionTop()
		for _, keys := range [][]string{nil, {"r"}} {
			press(t, s, keys...)
			lines := strings.Split(s.View(sz[0], sz[1]), "\n")
			if len(lines) != sz[1] {
				t.Errorf("%dx%d %v: %d lines", sz[0], sz[1], keys, len(lines))
			}
			for i, l := range lines {
				if n := ansi.StringWidth(l); n > sz[0] {
					t.Errorf("%dx%d %v: line %d is %d wide: %q", sz[0], sz[1], keys, i, n, ansi.Strip(l))
				}
			}
		}
	}
}

func TestGrubLoadAverage(t *testing.T) {
	c := fixture.GrubConfig()
	c.Load = &[3]float64{5.0 / 7, 6.0 / 30, 7.0 / 90}
	v := plain(meta.NewGrub(c).View(80, 24))
	if !strings.Contains(v, "· load average: 0.71, 0.20, 0.08") {
		t.Errorf("GRUB status lacks the load average:\n%s", v)
	}
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, "load average") && ansi.StringWidth(l) > 80 {
			t.Errorf("status line is %d wide", ansi.StringWidth(l))
		}
	}
	if strings.Contains(plain(meta.NewGrub(fixture.GrubConfig()).View(80, 24)), "load average") {
		t.Error("GRUB shows a load average with none given")
	}
}

// GR1: consistency shows as days, never as a streak. No screen in this
// package says the word.
func TestNoStreaks(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(b)), "streak") {
			t.Errorf("%s mentions a streak", f)
		}
	}
}
