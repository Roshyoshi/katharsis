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
        int n; std::string a, b; std::cin >> n >> a >> b;
        for (auto &c : a) if (c == 'G') c = 'B';
        for (auto &c : b) if (c == 'G') c = 'B';
        std::cout << (a == b ? "yes" : "no") << "\n";
    }
}
