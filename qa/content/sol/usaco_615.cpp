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
    freopen("pails.in", "r", stdin);
    freopen("pails.out", "w", stdout);
    int x, y, m;
    cin >> x >> y >> m;
    int best = 0;
    for (int a = 0; a * x <= m; a++)
        for (int b = 0; a * x + b * y <= m; b++) best = max(best, a * x + b * y);
    cout << best << "\n";
}
