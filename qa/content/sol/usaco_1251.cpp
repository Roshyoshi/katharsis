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
    int n;
    cin >> n;
    vector<long long> c(n);
    for (auto &x : c) cin >> x;
    sort(c.begin(), c.end());
    long long best = -1, tuition = 0;
    for (int i = 0; i < n; i++) {
        long long money = c[i] * (n - i);
        if (money > best) best = money, tuition = c[i];
    }
    cout << best << ' ' << tuition << '\n';
}
