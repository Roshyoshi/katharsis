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
    freopen("mixmilk.in", "r", stdin);
    freopen("mixmilk.out", "w", stdout);
    long long c[3], m[3];
    for (int i = 0; i < 3; i++) cin >> c[i] >> m[i];
    for (int k = 0; k < 100; k++) {
        int a = k % 3, b = (k + 1) % 3;
        long long amt = min(m[a], c[b] - m[b]);
        m[a] -= amt, m[b] += amt;
    }
    for (int i = 0; i < 3; i++) cout << m[i] << "\n";
}
