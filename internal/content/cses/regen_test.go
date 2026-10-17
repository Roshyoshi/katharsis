package cses

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRegenFacts rebuilds facts.json from the live problem list and every
// task page. Run it when CSES adds tasks:
//
//	KATHARSIS_REGEN_FACTS=/tmp/cses-pages go test -run TestRegenFacts \
//	    ./internal/content/cses
//
// The dir caches the pages it downloads (list.html, task/<id>.html), so a
// rerun after fixing the parser is offline; delete list.html for fresh
// solver counts. A cold run makes about 400 requests at the package's
// pace, one a second: about 7 minutes. No login is needed.
func TestRegenFacts(t *testing.T) {
	dir := os.Getenv("KATHARSIS_REGEN_FACTS")
	if dir == "" {
		t.Skip("set KATHARSIS_REGEN_FACTS=<cache dir> to rebuild facts.json")
	}
	ctx := context.Background()
	get := func(name, u string) []byte {
		p := filepath.Join(dir, name)
		if b, err := os.ReadFile(p); err == nil {
			return b
		}
		resp, err := pace.do(ctx, request{url: u})
		if err != nil {
			t.Fatal(err)
		}
		if resp.status != http.StatusOK {
			t.Fatalf("%s: status %d", u, resp.status)
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, resp.body, 0o644); err != nil {
			t.Fatal(err)
		}
		return resp.body
	}
	rows, err := parseList(get("list.html", site+"/problemset/list/"))
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	b.WriteString("[\n")
	start := time.Now()
	for i, r := range rows {
		st, err := parseTask(get("task/"+r.ID+".html", taskURL(r.ID)))
		if err != nil {
			t.Fatalf("task %s: %v", r.ID, err)
		}
		if r.Section == interactiveSection {
			st.Interactive = true
		}
		line, err := json.Marshal(factsRow{row: r, facts: st.facts})
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		if i+1 < len(rows) {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
		if (i+1)%50 == 0 {
			t.Logf("%d/%d tasks, %v", i+1, len(rows), time.Since(start).Round(time.Second))
		}
	}
	b.WriteString("]\n")
	if err := os.WriteFile("facts.json", b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote facts.json: %d tasks", len(rows))
}
