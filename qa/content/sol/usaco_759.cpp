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
struct R { int x1, y1, x2, y2; };
int area(R r) { return max(0, r.x2 - r.x1) * max(0, r.y2 - r.y1); }
int inter(R a, R b) {
    return area({max(a.x1, b.x1), max(a.y1, b.y1), min(a.x2, b.x2), min(a.y2, b.y2)});
}
int main() {
    freopen("billboard.in", "r", stdin);
    freopen("billboard.out", "w", stdout);
    R a, b, t;
    cin >> a.x1 >> a.y1 >> a.x2 >> a.y2 >> b.x1 >> b.y1 >> b.x2 >> b.y2 >> t.x1 >> t.y1 >> t.x2 >> t.y2;
    cout << area(a) + area(b) - inter(a, t) - inter(b, t) << "\n";
}
