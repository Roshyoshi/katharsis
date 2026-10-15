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
    vector<long long> a(7);
    for (auto &x : a) cin >> x;
    sort(a.begin(), a.end());
    long long A = a[0], B = a[1], C = a[6] - A - B;
    cout << A << ' ' << B << ' ' << C << '\n';
}
