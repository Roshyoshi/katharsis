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
    int n, k; std::cin >> n >> k;
    std::vector<int> a(n); for (auto &x : a) std::cin >> x;
    int c = 0; for (int x : a) if (x > 0 && x >= a[k - 1]) c++;
    std::cout << c << "\n";
}
