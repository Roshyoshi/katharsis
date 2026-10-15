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
int n;
bool close1(int a, int b) {
    int d = abs(a - b);
    return min(d, n - d) <= 2;
}
int main() {
    freopen("combo.in", "r", stdin);
    freopen("combo.out", "w", stdout);
    cin >> n;
    int f[3], m[3];
    for (int &x : f) cin >> x;
    for (int &x : m) cin >> x;
    int cnt = 0;
    for (int a = 1; a <= n; a++)
        for (int b = 1; b <= n; b++)
            for (int c = 1; c <= n; c++) {
                bool okF = close1(a, f[0]) && close1(b, f[1]) && close1(c, f[2]);
                bool okM = close1(a, m[0]) && close1(b, m[1]) && close1(c, m[2]);
                if (okF || okM) cnt++;
            }
    cout << cnt << "\n";
}
