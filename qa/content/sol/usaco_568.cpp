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
    freopen("speeding.in", "r", stdin);
    freopen("speeding.out", "w", stdout);
    int n, m;
    cin >> n >> m;
    vector<int> lim, spd;
    for (int i = 0; i < n; i++) { int l, s; cin >> l >> s; lim.insert(lim.end(), l, s); }
    for (int i = 0; i < m; i++) { int l, s; cin >> l >> s; spd.insert(spd.end(), l, s); }
    int best = 0;
    for (int i = 0; i < 100; i++) best = max(best, spd[i] - lim[i]);
    cout << best << "\n";
}
