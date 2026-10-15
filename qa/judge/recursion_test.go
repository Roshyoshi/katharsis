//go:build qa

package judgeqa

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"katharsis/internal/judge"
)

// A typical tree DFS (subtree sizes) over a path, so recursion depth is n.
const progDFSVec = `#include <cstdio>
#include <vector>
std::vector<std::vector<int>> g;
std::vector<int> sz;
void dfs(int u, int p) {
    sz[u] = 1;
    for (int v : g[u]) if (v != p) { dfs(v, u); sz[u] += sz[v]; }
}
int main() {
    int n;
    if (scanf("%d", &n) != 1) return 2;
    g.resize(n); sz.resize(n);
    for (int i = 0; i + 1 < n; i++) { g[i].push_back(i + 1); g[i + 1].push_back(i); }
    dfs(0, -1);
    printf("%d\n", sz[0]);
}
`

// The same DFS over a forward-star adjacency (head/next arrays), which is
// what memory-tight solutions use, with a depth and a long long sum per
// frame.
const progDFSStar = `#include <cstdio>
#include <vector>
std::vector<int> head, nxt, to;
std::vector<long long> sub;
void dfs(int u, int p, int d) {
    sub[u] = d;
    for (int e = head[u]; e != -1; e = nxt[e]) {
        int v = to[e];
        if (v == p) continue;
        dfs(v, u, d + 1);
        sub[u] += sub[v];
    }
}
int main() {
    int n;
    if (scanf("%d", &n) != 1) return 2;
    head.assign(n, -1); nxt.reserve(2 * n); to.reserve(2 * n); sub.resize(n);
    auto add = [&](int a, int b) { to.push_back(b); nxt.push_back(head[a]); head[a] = (int)to.size() - 1; };
    for (int i = 0; i + 1 < n; i++) { add(i, i + 1); add(i + 1, i); }
    dfs(0, -1, 0);
    printf("%lld\n", sub[0] % 1000);
}
`

func TestQARecursion(t *testing.T) {
	needCompiler(t)
	// And what :Debug runs under lldb here.
	ps := append(slices.Clone(profiles), judge.Profiles(context.Background()).Debug)
	for _, prog := range []struct{ name, body string }{{"dfs_vecvec", progDFSVec}, {"dfs_forward_star", progDFSStar}} {
		for _, n := range []int{1000000, 5000000} {
			for _, p := range ps {
				t.Run(fmt.Sprintf("%s/%d/%s", prog.name, n, p.Name), func(t *testing.T) {
					heavyMu.Lock()
					defer heavyMu.Unlock()
					b := build(t, prog.name, prog.body, p)
					want := fmt.Sprint(n)
					if prog.name == "dfs_forward_star" {
						// sum of depths 0..n-1 mod 1000
						s := int64(n) * int64(n-1) / 2
						want = fmt.Sprint(s % 1000)
					}
					c := writeCase(t, fmt.Sprintf("%d\n", n), want+"\n")
					lim := judge.Limits{Time: 10 * time.Second, Mem: 4 << 30}
					r, err := judge.Run(context.Background(), b.Binary, c, lim, p)
					record(fmt.Sprintf("recursion/%s/n=%d", prog.name, n), p.Name, r, err)
					if err != nil {
						t.Fatal(err)
					}
					if r.Verdict != judge.AC {
						t.Errorf("depth %d: %s %q (rss %s)", n, r.Verdict, r.Reason, judge.Bytes(r.MaxRSS))
					}
				})
			}
		}
	}
}
