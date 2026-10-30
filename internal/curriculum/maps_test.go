package curriculum

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"katharsis/internal/catalog"
)

// TestMaps parses every embedded map, as MapFor does on first use.
func TestMaps(t *testing.T) {
	ms, err := ParseMaps(mapFiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) == 0 {
		t.Fatal("no embedded maps")
	}
	for id, m := range ms {
		if m.Core() == 0 {
			t.Errorf("%s: no core problem", id)
		}
		for i, n := range m.Nodes {
			for _, p := range n.Parents {
				if p >= i {
					t.Errorf("%s: %s has parent %d, not before it", id, n.Problem, p)
				}
				if n.Core && !m.Nodes[p].Core {
					t.Errorf("%s: side %s leads into core %s", id, m.Nodes[p].Problem, n.Problem)
				}
			}
		}
	}
}

// TestMapsListed checks the embedded maps against a real snapshot of
// guide.Commit: each is a module's, names only problems the module lists,
// and holds a problem the catalog can name, as MapFor plays it. It runs
// only with KATHARSIS_GUIDE=<snapshot root>.
func TestMapsListed(t *testing.T) {
	dir := os.Getenv("KATHARSIS_GUIDE")
	if dir == "" {
		t.Skip("set KATHARSIS_GUIDE to a usaco-guide snapshot to run")
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for id, raw := range embedded() {
		m, ok := c.modules[id]
		if !ok {
			t.Errorf("%s: no such module", id)
			continue
		}
		listed := map[string]bool{}
		for _, l := range m.Problems {
			for _, p := range l.Problems {
				listed[p.UniqueID] = true
			}
		}
		for _, n := range raw.Nodes {
			if !listed[n.Problem] {
				t.Errorf("%s: %s is not on the module's lists", id, n.Problem)
			}
		}
		for _, s := range raw.Skip {
			if !listed[s] {
				t.Errorf("%s: skips %s, which the module does not list", id, s)
			}
		}
		if mp, _ := MapFor(m); len(mp.Playable(nil).Nodes) == 0 {
			t.Errorf("%s: no node names a catalog problem", id)
		}
	}
}

func TestParseMap(t *testing.T) {
	m, err := ParseMap("two-pointers", strings.Join([]string{
		"# Two Pointers",
		"core  usaco-738 > cses-1641 > usaco-643   # both ends",
		"side  usaco-643 > usaco-918 > cf-1744F    # windows on positions",
		"core  cses-1660 > usaco-643               # windows",
		"side  cses-1641 > cf-702C                 # nearest tower",
		"skip  cf-9D                               # binary search's",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		problem string
		core    bool
		parents []string
		lane    int
		note    string
	}
	var got []row
	for _, n := range m.Nodes {
		r := row{n.Problem, n.Core, nil, n.Lane, n.Note}
		for _, p := range n.Parents {
			r.parents = append(r.parents, m.Nodes[p].Problem)
		}
		got = append(got, r)
	}
	want := []row{
		{"usaco-738", true, nil, 0, "both ends"},
		{"cses-1641", true, []string{"usaco-738"}, 0, "both ends"},
		{"cses-1660", true, nil, 1, "windows"},
		{"usaco-643", true, []string{"cses-1641", "cses-1660"}, 0, "both ends"},
		{"usaco-918", false, []string{"usaco-643"}, 0, "windows on positions"},
		{"cf-1744F", false, []string{"usaco-918"}, 0, "windows on positions"},
		{"cf-702C", false, []string{"cses-1641"}, 0, "nearest tower"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("nodes\n got %v\nwant %v", got, want)
	}
	if want := []string{"both ends", "windows"}; !reflect.DeepEqual(m.Lanes, want) {
		t.Errorf("lanes %q, want %q", m.Lanes, want)
	}
	if want := []string{"cf-9D"}; !reflect.DeepEqual(m.Skip, want) {
		t.Errorf("skip %q, want %q", m.Skip, want)
	}
	if m.Nodes[0].ID != "usaco:738" || m.Core() != 4 {
		t.Errorf("ID %q, core %d", m.Nodes[0].ID, m.Core())
	}
}

func TestParseMapRejects(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{"kind", "path a > b", "not core, side or skip"},
		{"empty step", "core a > > b", "empty step"},
		{"side alone", "side a", "anchor and a problem"},
		{"side into core", "core a > b\nside a > c\ncore c > d", "side and core"},
		{"core into side", "core a > b\nside a > b", "core and side"},
		{"two anchors", "core a > b\nside a > c\nside b > c", "already follows"},
		{"loose anchor", "core a\nside z > c", "placed by no chain"},
		{"skip placed", "core a\nskip a", "skipped and placed"},
		{"skip chain", "core a\nskip b > c", "one problem"},
		{"loop", "core a > b\ncore b > a", "loop"},
	} {
		_, err := ParseMap("m", tc.text)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err %v, want %q", tc.name, err, tc.want)
		}
	}
}

// TestPlayable drops what the catalog cannot judge, and what led into a
// dropped node leads into its children instead.
func TestPlayable(t *testing.T) {
	m, err := ParseMap("m", strings.Join([]string{
		"core usaco-1 > cses-2 > usaco-3",
		"core usaco-4 > usaco-3",
		"side cses-2 > usaco-5 > cses-6 > usaco-7",
		"side usaco-3 > ceoi-x",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	usaco := func(id catalog.ID) bool { return id.Source() == catalog.USACO }
	p := m.Playable(usaco)
	got := map[string][]string{}
	for _, n := range p.Nodes {
		var ps []string
		for _, i := range n.Parents {
			ps = append(ps, p.Nodes[i].Problem)
		}
		got[n.Problem] = ps
	}
	want := map[string][]string{
		"usaco-1": nil,
		"usaco-4": nil,
		"usaco-3": {"usaco-1", "usaco-4"},
		"usaco-5": {"usaco-1"},
		"usaco-7": {"usaco-5"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("playable\n got %v\nwant %v", got, want)
	}
	if p.Core() != 3 {
		t.Errorf("core %d, want 3", p.Core())
	}
}
