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
    freopen("times17.in", "r", stdin);
    freopen("times17.out", "w", stdout);
    string s;
    cin >> s;
    // 17N = (N << 4) + N, added in binary, little-endian.
    string a(s.rbegin(), s.rend());
    string b = "0000" + a;
    string r;
    int carry = 0;
    for (size_t i = 0; i < max(a.size(), b.size()) || carry; i++) {
        int x = (i < a.size() ? a[i] - '0' : 0) + (i < b.size() ? b[i] - '0' : 0) + carry;
        r += char('0' + x % 2);
        carry = x / 2;
    }
    while (r.size() > 1 && r.back() == '0') r.pop_back();
    reverse(r.begin(), r.end());
    cout << r << "\n";
}
