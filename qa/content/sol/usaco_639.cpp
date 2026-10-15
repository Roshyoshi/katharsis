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
    freopen("diamond.in", "r", stdin);
    freopen("diamond.out", "w", stdout);
    int n, k;
    cin >> n >> k;
    vector<int> a(n);
    for (auto &x : a) cin >> x;
    sort(a.begin(), a.end());
    int best = 0;
    for (int i = 0, j = 0; i < n; i++) {
        while (a[i] - a[j] > k) j++;
        best = max(best, i - j + 1);
    }
    cout << best << "\n";
}
