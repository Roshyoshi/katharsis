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
using namespace std;
int main() {
    int n, l;
    cin >> n >> l;
    vector<int> c(n);
    for (auto &x : c) cin >> x;
    sort(c.rbegin(), c.rend());
    int best = 0;
    for (int h = 1; h <= n; h++) {
        if (c[h - 1] < h - 1) break;
        // c is sorted descending, so the papers below h among the first h
        // are a suffix, each at h-1 and short one citation.
        int at = partition_point(c.begin(), c.begin() + h, [&](int x) { return x >= h; }) - c.begin();
        if (h - at <= l) best = h;
    }
    cout << best << '\n';
}
