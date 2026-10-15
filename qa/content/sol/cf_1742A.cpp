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
        int a, b, c; std::cin >> a >> b >> c;
        std::cout << (a == b + c || b == a + c || c == a + b ? "YES" : "NO") << "\n";
    }
}
