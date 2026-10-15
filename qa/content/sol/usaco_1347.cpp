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
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    int n, m;
    cin >> n >> m;
    vector<long long> h(n);
    for (auto &x : h) cin >> x;
    for (int j = 0; j < m; j++) {
        long long c;
        cin >> c;
        long long base = 0;
        for (int i = 0; i < n && base < c; i++) {
            if (h[i] > base) {
                long long eat = min(h[i], c) - base;
                h[i] += eat;
                base += eat;
            }
        }
    }
    for (auto x : h) cout << x << '\n';
}
