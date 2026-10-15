//go:build qa

package content

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/judge"
)

// solution is a C++ file in sol/ judged on a problem's full tests the way
// a door's :Submit is (judge.Full, the problem's limits). want is the
// verdict a fair judge gives: AC for a correct program, WA for a
// deliberately wrong one.
type solution struct {
	id   catalog.ID
	file string
	want judge.Verdict
	note string
}

var solutions = []solution{
	// ---- USACO Bronze, correct ----
	{"usaco:84", "usaco_84.cpp", judge.AC, ""},
	{"usaco:94", "usaco_94.cpp", judge.AC, ""},
	{"usaco:121", "usaco_121.cpp", judge.AC, ""},
	{"usaco:340", "usaco_340.cpp", judge.AC, ""},
	{"usaco:358", "usaco_358.cpp", judge.AC, ""},
	{"usaco:567", "usaco_567.cpp", judge.AC, ""},
	{"usaco:568", "usaco_568.cpp", judge.AC, ""},
	{"usaco:615", "usaco_615.cpp", judge.AC, ""},
	{"usaco:639", "usaco_639.cpp", judge.AC, ""},
	{"usaco:663", "usaco_663.cpp", judge.AC, ""},
	{"usaco:687", "usaco_687.cpp", judge.AC, ""},
	{"usaco:735", "usaco_735.cpp", judge.AC, ""},
	{"usaco:759", "usaco_759.cpp", judge.AC, ""},
	{"usaco:807", "usaco_807.cpp", judge.AC, ""},
	{"usaco:855", "usaco_855.cpp", judge.AC, ""},
	{"usaco:891", "usaco_891.cpp", judge.AC, ""},
	{"usaco:963", "usaco_963.cpp", judge.AC, ""},
	{"usaco:987", "usaco_987.cpp", judge.AC, ""},
	{"usaco:1059", "usaco_1059.cpp", judge.AC, ""},
	{"usaco:1083", "usaco_1083.cpp", judge.AC, ""},
	{"usaco:1131", "usaco_1131.cpp", judge.AC, ""},
	{"usaco:1155", "usaco_1155.cpp", judge.AC, ""},
	{"usaco:1179", "usaco_1179.cpp", judge.AC, ""},
	{"usaco:1251", "usaco_1251.cpp", judge.AC, ""},
	{"usaco:1299", "usaco_1299.cpp", judge.AC, ""},
	{"usaco:1347", "usaco_1347.cpp", judge.AC, ""},
	{"usaco:1395", "usaco_1395.cpp", judge.AC, ""},
	{"usaco:1443", "usaco_1443.cpp", judge.AC, ""},
	{"usaco:1210", "usaco_1210.cpp", judge.AC, "float tolerance"},

	// ---- Codeforces 800-1100, correct ----
	{"cf:4A", "cf_4A.cpp", judge.AC, ""},
	{"cf:1A", "cf_1A.cpp", judge.AC, ""},
	{"cf:158A", "cf_158A.cpp", judge.AC, ""},
	{"cf:71A", "cf_71A.cpp", judge.AC, ""},
	{"cf:266A", "cf_266A.cpp", judge.AC, ""},
	{"cf:236A", "cf_236A.cpp", judge.AC, ""},
	{"cf:118A", "cf_118A.cpp", judge.AC, ""},
	{"cf:131A", "cf_131A.cpp", judge.AC, ""},
	{"cf:200B", "cf_200B.cpp", judge.AC, "float"},
	{"cf:1293B", "cf_1293B.cpp", judge.AC, "float"},
	{"cf:1642A", "cf_1642A.cpp", judge.AC, "float"},
	{"cf:127A", "cf_127A.cpp", judge.AC, "float"},
	{"cf:1722B", "cf_1722B.cpp", judge.AC, "AnyCase"},
	{"cf:1742A", "cf_1742A.cpp", judge.AC, "AnyCase"},
	{"cf:1873A", "cf_1873A.cpp", judge.AC, "AnyCase"},
	{"cf:1986A", "cf_1986A.cpp", judge.AC, ""},
	{"cf:2007A", "cf_2007A.cpp", judge.AC, ""},
	{"cf:1004A", "cf_1004A.cpp", judge.AC, ""},
	{"cf:160A", "cf_160A.cpp", judge.AC, ""},
	{"cf:1328A", "cf_1328A.cpp", judge.AC, ""},
	{"cf:1971A", "cf_1971A.cpp", judge.AC, ""},
	{"cf:1791A", "cf_1791A.cpp", judge.AC, "AnyCase"},
	{"cf:1899C", "cf_1899C.cpp", judge.AC, ""},

	// ---- fairness probes: valid answers a real judge accepts ----
	{"cf:1722B", "cf_1722B_lower.cpp", judge.AC, "prints yes/no in lower case"},
	{"cf:1A", "cf_1A_lines.cpp", judge.AC, "token-equal output split differently"},
	{"cf:233A", "cf_233A.cpp", judge.AC, "multi-answer: the sample's pairing"},
	{"cf:233A", "cf_233A_rev.cpp", judge.AC, "multi-answer: another valid permutation"},
	{"cf:219A", "cf_219A.cpp", judge.AC, "multi-answer: sorted base string"},
	{"cf:219A", "cf_219A_rev.cpp", judge.AC, "multi-answer: reverse-sorted base string"},
	{"cf:200B", "cf_200B_prec.cpp", judge.AC, "float printed with 4 decimals"},
	{"cf:1971A", "cf_1971A_lines.cpp", judge.AC, "token-equal output, one number per line"},
	{"cf:832A", "cf_832A.cpp", judge.AC, ""},
	{"cf:832A", "cf_832A_lower.cpp", judge.AC, "statement: 'each letter in arbitrary case'"},
	{"cf:1110A", "cf_1110A.cpp", judge.AC, ""},
	{"cf:1110A", "cf_1110A_upper.cpp", judge.AC, "AnyCase, answer word is even/odd"},
	{"usaco:567", "usaco_567_fstream.cpp", judge.AC, "ifstream/ofstream file I/O, as USACO analyses do"},

	// ---- deliberately wrong: the data must discriminate ----
	{"usaco:567", "usaco_567_wrong.cpp", judge.WA, "sums lengths, ignores overlap"},
	{"usaco:1059", "usaco_1059_wrong.cpp", judge.WA, "prints first three numbers"},
	{"cf:4A", "cf_4A_wrong.cpp", judge.WA, "forgets w=2"},
	{"cf:1899C", "cf_1899C_wrong.cpp", judge.WA, "ignores parity alternation"},
	{"cf:1293B", "cf_1293B_wrong.cpp", judge.WA, "harmonic sum off by one term"},
	{"usaco:1210", "usaco_1210_wrong.cpp", judge.WA, "answer 1% low, outside 1e-6"},
	{"cf:1695D1", "cf_1695D1.cpp", judge.AC, "6 tests where Codeforces has 94"},
	{"cf:1695D1", "cf_1695D1_wrong.cpp", judge.WA, "leaves - 1"},
}

type outcome struct {
	s       solution
	p       catalog.Problem
	sum     judge.Summary
	first   *judge.Result
	tests   int
	err     error
	elapsed time.Duration
	ce      string
}

func TestSolve(t *testing.T) {
	u := union(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	dir, _ := filepath.Abs("sol")
	var rows []string
	for _, s := range solutions {
		if !only(s.id) {
			continue
		}
		o := solve(ctx, u, filepath.Join(dir, s.file), s)
		row := fmt.Sprintf("%-12s %-24s want=%-3s got=%-3s %3d/%-3d r=%-4d T=%-4v tol=%-6g any=%-5v cpu=%-6v %s",
			s.id, s.file, s.want, o.sum.Verdict, o.sum.Passed, o.sum.Total, o.p.Rating, o.p.TimeLimit,
			o.p.FloatTolerance, o.p.AnyCase, o.sum.MaxCPU.Round(time.Millisecond), s.note)
		rows = append(rows, row)
		switch {
		case o.err != nil:
			t.Errorf("%s %s: %v", s.id, s.file, o.err)
		case o.ce != "":
			t.Errorf("%s %s: compile error:\n%s", s.id, s.file, o.ce)
		case o.sum.Verdict != s.want:
			msg := ""
			if o.first != nil {
				msg = fmt.Sprintf("first failure test %d: %s %s", o.sum.First+1, o.first.Verdict, o.first.Reason)
				if d := o.first.Diff; d != nil {
					msg += fmt.Sprintf("\n    line %d want %q\n    line %d got  %q", d.Line, d.Want, d.Line, d.Got)
				}
			}
			t.Errorf("%s %s: verdict %s, want %s (%d/%d) %s", s.id, s.file, o.sum.Verdict, s.want, o.sum.Passed, o.sum.Total, msg)
		}
	}
	t.Log("\n" + strings.Join(rows, "\n"))
}

func solve(ctx context.Context, u catalog.Catalog, src string, s solution) outcome {
	o := outcome{s: s}
	start := time.Now()
	defer func() { o.elapsed = time.Since(start) }()
	p, ok := u.Get(s.id)
	if !ok {
		o.err = fmt.Errorf("not in catalog")
		return o
	}
	a, err := u.Fetch(ctx, s.id)
	if err != nil {
		o.err = err
		return o
	}
	p, _ = u.Get(s.id)
	o.p = p
	if _, err := os.Stat(src); err != nil {
		o.err = err
		return o
	}
	// The limits a door's HUD submits with (internal/app/runner.go).
	lim := judge.Limits{
		Time:           p.TimeLimit,
		Mem:            p.MemLimit,
		Output:         judge.DefaultOutputLimit,
		Scale:          1,
		FloatTolerance: p.FloatTolerance,
		AnyCase:        p.AnyCase,
	}
	results := map[int]judge.Result{}
	judge.Submit(ctx, src, a.Tests, judge.Full, lim, func(e judge.Event) {
		switch e := e.(type) {
		case judge.BuildDone:
			if !e.Build.OK {
				o.ce = e.Build.Log
			}
		case judge.TestDone:
			results[e.Index] = e.Result
		case judge.Finished:
			if e.Err != nil {
				o.err = e.Err
			}
			if e.Summary != nil {
				o.sum = *e.Summary
			}
		}
	})
	if r, ok := results[o.sum.First]; ok && o.sum.First >= 0 {
		o.first = &r
	}
	o.tests = len(a.Tests)
	return o
}

// TestFailures prints every failing test of one solution in full detail:
// QA_ONLY=<id> QA_SOL=<file> go test -tags qa ./qa/content -run TestFailures -v
func TestFailures(t *testing.T) {
	file := os.Getenv("QA_SOL")
	if file == "" {
		t.Skip("set QA_SOL")
	}
	u := union(t)
	ctx := context.Background()
	for _, s := range solutions {
		if !only(s.id) || s.file != file {
			continue
		}
		p, _ := u.Get(s.id)
		a, err := u.Fetch(ctx, s.id)
		if err != nil {
			t.Fatal(err)
		}
		src, _ := filepath.Abs(filepath.Join("sol", s.file))
		lim := judge.Limits{Time: p.TimeLimit, Mem: p.MemLimit, Output: judge.DefaultOutputLimit,
			Scale: 1, FloatTolerance: p.FloatTolerance, AnyCase: p.AnyCase}
		var bad []string
		judge.Submit(ctx, src, a.Tests, judge.Full, lim, func(e judge.Event) {
			if d, ok := e.(judge.TestDone); ok && d.Result.Verdict != judge.AC {
				line := fmt.Sprintf("test %3d (%s): %s %s cpu=%v", d.Index+1, a.Tests[d.Index].In, d.Result.Verdict, d.Result.Reason, d.Result.CPU)
				if df := d.Result.Diff; df != nil {
					line += fmt.Sprintf(" | want %q got %q", df.Want, df.Got)
				}
				bad = append(bad, line)
			}
		})
		sort.Strings(bad)
		t.Logf("%s %s: %d failing of %d\n%s", s.id, s.file, len(bad), len(a.Tests), strings.Join(bad, "\n"))
	}
}
