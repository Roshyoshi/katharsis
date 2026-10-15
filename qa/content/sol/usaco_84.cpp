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
    freopen("ctiming.in", "r", stdin);
    freopen("ctiming.out", "w", stdout);
    int d, h, m;
    cin >> d >> h >> m;
    int t = (d * 24 + h) * 60 + m - ((11 * 24 + 11) * 60 + 11);
    cout << (t < 0 ? -1 : t) << "\n";
}
