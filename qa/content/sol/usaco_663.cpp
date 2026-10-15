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
    freopen("square.in", "r", stdin);
    freopen("square.out", "w", stdout);
    int a1, b1, a2, b2, c1, d1, c2, d2;
    cin >> a1 >> b1 >> a2 >> b2 >> c1 >> d1 >> c2 >> d2;
    int side = max(max(a2, c2) - min(a1, c1), max(b2, d2) - min(b1, d1));
    cout << side * side << "\n";
}
