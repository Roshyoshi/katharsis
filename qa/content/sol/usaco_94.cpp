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
    freopen("haybales.in", "r", stdin);
    freopen("haybales.out", "w", stdout);
    int n;
    cin >> n;
    vector<long long> a(n);
    long long s = 0;
    for (auto &x : a) cin >> x, s += x;
    long long avg = s / n, moved = 0;
    for (auto x : a) if (x > avg) moved += x - avg;
    cout << moved << "\n";
}
