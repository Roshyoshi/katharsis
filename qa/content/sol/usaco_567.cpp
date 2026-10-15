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
    freopen("paint.in", "r", stdin);
    freopen("paint.out", "w", stdout);
    int a, b, c, d;
    cin >> a >> b >> c >> d;
    int overlap = max(0, min(b, d) - max(a, c));
    cout << (b - a) + (d - c) - overlap << "\n";
}
