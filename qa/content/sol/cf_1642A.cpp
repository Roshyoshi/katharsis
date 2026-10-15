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
int main() {
    int t; std::cin >> t;
    while (t--) {
        long long x[3], y[3];
        for (int i = 0; i < 3; i++) std::cin >> x[i] >> y[i];
        long long ans = 0;
        for (int i = 0; i < 3; i++)
            for (int j = i + 1; j < 3; j++) {
                int k = 3 - i - j;
                if (y[i] == y[j] && y[k] < y[i]) ans = llabs(x[i] - x[j]);
            }
        std::cout << ans << "\n";
    }
}
