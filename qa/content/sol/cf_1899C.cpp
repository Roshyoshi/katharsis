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
int main() {
    std::ios::sync_with_stdio(false); std::cin.tie(nullptr);
    int t; std::cin >> t;
    while (t--) {
        int n; std::cin >> n; std::vector<long long> a(n); for (auto &x : a) std::cin >> x;
        long long best = a[0], cur = a[0];
        for (int i = 1; i < n; i++) {
            bool alt = ((a[i] % 2) + 2) % 2 != ((a[i - 1] % 2) + 2) % 2;
            cur = alt ? std::max(cur, 0LL) + a[i] : a[i];
            best = std::max(best, cur);
        }
        std::cout << best << "\n";
    }
}
