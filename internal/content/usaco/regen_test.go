package usaco

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"katharsis/internal/content/guide"
)

// TestRegenFacts rebuilds facts.json from the live statement pages of every
// problem in the pinned guide. Run it after bumping guide.Commit:
//
//	KATHARSIS_REGEN_FACTS=/tmp/usaco-pages go test -run TestRegenFacts \
//	    ./internal/content/usaco
//
// The dir caches what it downloads (statements/<cpid>.html,
// heads/<cpid>.code with the zip's HTTP status, zips/<stem>.zip for shared
// stems), so a rerun after fixing the parser is offline. A cold run makes
// about 1450 requests at the polite client's pace: roughly 20 minutes.
func TestRegenFacts(t *testing.T) {
	dir := os.Getenv("KATHARSIS_REGEN_FACTS")
	if dir == "" {
		t.Skip("set KATHARSIS_REGEN_FACTS=<cache dir> to rebuild facts.json")
	}
	ctx := context.Background()
	gdir, err := guide.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stems, err := readIDToSol(gdir)
	if err != nil {
		t.Fatal(err)
	}
	divs, err := readDivToProbs(gdir)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int
	for id := range stems {
		ids = append(ids, id)
	}
	for id := range divs {
		if _, ok := stems[id]; !ok {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)

	rows := map[int]factsRow{}
	samples := map[int][]sample{}
	for _, id := range ids {
		page, err := cached(ctx, filepath.Join(dir, "statements", strconv.Itoa(id)+".html"), func() ([]byte, error) {
			return client.getBytes(ctx, statementURL(id), maxPage)
		})
		if err != nil {
			t.Errorf("cpid %d: %v", id, err)
			continue
		}
		st, err := parseStatement(page, statementURL(id))
		if err != nil {
			t.Errorf("cpid %d: %v", id, err)
			continue
		}
		if len(st.Samples) == 0 && !st.Interactive {
			t.Errorf("cpid %d: no samples in a non-interactive statement", id)
		}
		code, err := cached(ctx, filepath.Join(dir, "heads", strconv.Itoa(id)+".code"), func() ([]byte, error) {
			err := client.head(ctx, site+"/current/data/"+stems[id]+".zip")
			switch {
			case errors.Is(err, errNotFound):
				return []byte("404\n"), nil
			case err != nil:
				return nil, err
			}
			return []byte("200\n"), nil
		})
		if err != nil {
			t.Errorf("cpid %d zip: %v", id, err)
			continue
		}
		rows[id] = factsRow{facts: st.facts, HasZip: strings.TrimSpace(string(code)) == "200" && stems[id] != ""}
		samples[id] = st.Samples
		if d, ok := divs[id]; ok {
			if d.contest != st.Contest || d.division != st.Division || d.position != st.Position {
				t.Logf("cpid %d: div_to_probs says %s %s P%d, statement %s %s P%d",
					id, d.contest, d.division, d.position, st.Contest, st.Division, st.Position)
			}
		}
	}

	for _, id := range sharedStemData(ctx, t, dir, stems, rows, samples) {
		r := rows[id]
		r.HasZip = false
		rows[id] = r
	}

	var b bytes.Buffer
	b.WriteString("{\n")
	keys := slices.Sorted(maps.Keys(rows))
	for i, id := range keys {
		line, err := json.Marshal(rows[id])
		if err != nil {
			t.Fatal(err)
		}
		sep := ","
		if i == len(keys)-1 {
			sep = ""
		}
		fmt.Fprintf(&b, "%q: %s%s\n", strconv.Itoa(id), line, sep)
	}
	b.WriteString("}\n")
	if err := os.WriteFile("facts.json", b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote facts.json: %d problems", len(rows))
}

// sharedStemData returns problems whose test zip holds another problem's
// data. A few old stems are shared. Usually that is one problem set in two
// divisions of one contest (photo, auto, bbreeds, ...), which is fine. But
// "rblock" is Dec 2011 Silver P2 (N <= 100) and Feb 2014 Silver P2 / Gold
// P1 (N <= 250): the 2014 upload replaced the 2011 zip (its files are
// dated 2014-02-16), so the 2011 problem would be judged on data outside
// its constraints. A problem keeps the zip only if it is from the newest
// contest sharing the stem and its sample input is among the test inputs.
func sharedStemData(ctx context.Context, t *testing.T, dir string, stems map[int]string,
	rows map[int]factsRow, samples map[int][]sample) []int {
	byStem := map[string][]int{}
	for id, s := range stems {
		byStem[s] = append(byStem[s], id)
	}
	var bad []int
	for stem, ids := range byStem {
		if len(ids) < 2 {
			continue
		}
		slices.Sort(ids)
		zb, err := cached(ctx, filepath.Join(dir, "zips", stem+".zip"), func() ([]byte, error) {
			return client.getBytes(ctx, site+"/current/data/"+stem+".zip", maxZip)
		})
		if err != nil {
			t.Errorf("stem %s: %v", stem, err)
			continue
		}
		inputs, err := zipInputs(zb)
		if err != nil {
			t.Errorf("stem %s: %v", stem, err)
			continue
		}
		newest := rows[ids[len(ids)-1]].Contest
		for _, id := range ids {
			inZip := false
			for _, s := range samples[id] {
				inZip = inZip || inputs[squash(s.In)]
			}
			ok := rows[id].Contest == newest && inZip
			t.Logf("stem %s: cpid %d %s %s P%d %q sample-in-zip=%v keep=%v",
				stem, id, rows[id].Contest, rows[id].Division, rows[id].Position, rows[id].Title, inZip, ok)
			if !ok {
				bad = append(bad, id)
			}
		}
	}
	return bad
}

func zipInputs(zb []byte) (map[string]bool, error) {
	zr, err := zip.NewReader(bytes.NewReader(zb), int64(len(zb)))
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, f := range zr.File {
		name := filepath.Base(f.Name)
		if !strings.HasSuffix(name, ".in") && !strings.HasPrefix(name, "I.") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(rc, 1<<20))
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		out[squash(string(b))] = true
	}
	return out, nil
}

// cached returns the file at p, or calls get and saves its result there.
func cached(ctx context.Context, p string, get func() ([]byte, error)) ([]byte, error) {
	if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
		return b, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b, err := get()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	return b, os.WriteFile(p, b, 0o644)
}

// head checks that rawURL exists without downloading it (errNotFound if
// not); facts.json records which problems have test zips this way.
func (c *politeClient) head(ctx context.Context, rawURL string) error {
	return c.do(ctx, http.MethodHead, rawURL, func(io.Reader) error { return nil })
}
