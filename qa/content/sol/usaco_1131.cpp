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
        int need = 0;
        for (int i = 0; i < h; i++) if (c[i] < h) need++;
        // only c[i] == h-1 can be below h here; each needs one citation.
        if (need <= l) best = h;
    }
    cout << best << '\n';
}
