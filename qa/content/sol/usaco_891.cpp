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
    freopen("shell.in", "r", stdin);
    freopen("shell.out", "w", stdout);
    int n;
    cin >> n;
    vector<array<int, 3>> ops(n);
    for (auto &o : ops) cin >> o[0] >> o[1] >> o[2];
    int best = 0;
    for (int start = 1; start <= 3; start++) {
        int pos = start, score = 0;
        for (auto &o : ops) {
            if (pos == o[0]) pos = o[1];
            else if (pos == o[1]) pos = o[0];
            if (pos == o[2]) score++;
        }
        best = max(best, score);
    }
    cout << best << "\n";
}
