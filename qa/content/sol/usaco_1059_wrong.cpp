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
    cout << a[0] << ' ' << a[1] << ' ' << a[2] << '\n';
}
