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
    int n; std::cin >> n; std::vector<int> a(n); int s = 0; for (auto &x : a) std::cin >> x, s += x;
    std::sort(a.rbegin(), a.rend());
    int mine = 0, k = 0; while (mine <= s - mine) mine += a[k++];
    std::cout << k << "\n";
}
