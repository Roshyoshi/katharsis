package guide

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"katharsis/internal/catalog"
)

func TestModuleTopicsAreCatalogTopics(t *testing.T) {
	t.Parallel()
	for id, topic := range ModuleTopics() {
		if !slices.Contains(catalog.Topics, topic) {
			t.Errorf("module %s maps to %q, not a catalog topic", id, topic)
		}
	}
}

// pairRE is a "key": "value" pair of string literals, as a Go map literal
// writes them.
var pairRE = regexp.MustCompile(`"([^"\s]+)"\s*:\s*"([^"\s]+)"`)

// The USACO catalog and learn mode once kept a module -> topic table each,
// and the two disagreed on five modules, so a problem's biome and its
// module's trial named different topics. This fails when any Go file in
// the module other than topics.go maps more than a handful of module ids
// to topics. (A handful is allowed: the USACO tag table shares a few
// names with module ids, such as "PURS" and "SCC".)
func TestOneModuleTopicTable(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("module root not at %s: %v", root, err)
	}
	self, _ := filepath.Abs("topics.go")
	const handful = 6
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); p != root && (strings.HasPrefix(name, ".") || name == "testdata" || name == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || p == self {
			return nil
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var ids []string
		for _, m := range pairRE.FindAllStringSubmatch(string(src), -1) {
			if _, ok := moduleTopics[m[1]]; ok && slices.Contains(catalog.Topics, m[2]) && !slices.Contains(ids, m[1]) {
				ids = append(ids, m[1])
			}
		}
		if len(ids) > handful {
			rel, _ := filepath.Rel(root, p)
			t.Errorf("%s maps %d module ids to topics (%s, ...): use guide.ModuleTopic", rel, len(ids), strings.Join(ids[:3], ", "))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
