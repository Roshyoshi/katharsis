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
    long long n, d; std::cin >> n >> d;
    std::vector<long long> x(n); for (auto &v : x) std::cin >> v;
    long long c = 2;
    for (int i = 1; i < n; i++) { long long g = x[i] - x[i - 1]; if (g == 2 * d) c += 1; else if (g > 2 * d) c += 2; }
    std::cout << c << "\n";
}
