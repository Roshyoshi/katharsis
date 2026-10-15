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
    int t;
    cin >> t;
    while (t--) {
        long long n;
        cin >> n;
        long long ans = 0, pw = 10; // pw = 10^(P-1)
        for (int p = 2; p <= 10; p++, pw *= 10) {
            // A P-digit x differs iff 44..45 <= x <= 5*10^(P-1) - 1.
            long long low = 0;
            for (int d = 0; d < p - 1; d++) low = low * 10 + 4;
            low = low * 10 + 5;
            long long hi = min(n, 5 * pw - 1);
            if (hi >= low) ans += hi - low + 1;
        }
        cout << ans << '\n';
    }
}
