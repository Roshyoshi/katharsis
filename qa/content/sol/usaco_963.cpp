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
    freopen("gymnastics.in", "r", stdin);
    freopen("gymnastics.out", "w", stdout);
    int k, n;
    cin >> k >> n;
    vector<vector<int>> rank(k, vector<int>(n + 1));
    for (int s = 0; s < k; s++)
        for (int p = 0; p < n; p++) { int c; cin >> c; rank[s][c] = p; }
    int cnt = 0;
    for (int a = 1; a <= n; a++)
        for (int b = 1; b <= n; b++) {
            if (a == b) continue;
            bool all = true;
            for (int s = 0; s < k; s++) if (rank[s][a] > rank[s][b]) all = false;
            if (all) cnt++;
        }
    cout << cnt << "\n";
}
