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
// Wrong: leaves - 1 (right for stars and paths, wrong with two branch points).
int main() {
    int t; std::cin >> t;
    while (t--) {
        int n; std::cin >> n;
        std::vector<int> deg(n + 1);
        for (int i = 0; i < n - 1; i++) { int x, y; std::cin >> x >> y; deg[x]++; deg[y]++; }
        if (n == 1) { std::cout << 0 << "\n"; continue; }
        int leaves = 0; for (int v = 1; v <= n; v++) leaves += deg[v] == 1;
        std::cout << leaves - 1 << "\n";
    }
}
