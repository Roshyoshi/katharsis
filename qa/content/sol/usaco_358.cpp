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
    freopen("records.in", "r", stdin);
    freopen("records.out", "w", stdout);
    int n;
    cin >> n;
    map<array<string, 3>, int> cnt;
    int best = 0;
    for (int i = 0; i < n; i++) {
        array<string, 3> g;
        cin >> g[0] >> g[1] >> g[2];
        sort(g.begin(), g.end());
        best = max(best, ++cnt[g]);
    }
    cout << best << "\n";
}
