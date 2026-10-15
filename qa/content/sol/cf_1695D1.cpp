#include <algorithm>
#include <array>
#include <climits>
#include <cmath>
#include <cstdio>
#include <cstdlib>
#include <iomanip>
#include <iostream>
#include <map>
#include <set>
#include <string>
#include <vector>
// Root at each vertex r (a query); every vertex may have at most one child
// subtree without a query. O(n^2), fine for D1.
std::vector<std::vector<int>> g;
int f(int v, int p) {
    int s = 0, z = 0;
    for (int c : g[v]) if (c != p) { int x = f(c, v); s += x; z += x == 0; }
    return s + std::max(0, z - 1);
}
int main() {
    int t; std::cin >> t;
    while (t--) {
        int n; std::cin >> n;
        g.assign(n + 1, {});
        for (int i = 0; i < n - 1; i++) { int x, y; std::cin >> x >> y; g[x].push_back(y); g[y].push_back(x); }
        if (n == 1) { std::cout << 0 << "\n"; continue; }
        int best = n;
        for (int r = 1; r <= n; r++) best = std::min(best, 1 + f(r, 0));
        std::cout << best << "\n";
    }
}
