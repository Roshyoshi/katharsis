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
    while (t--) { int l, r; std::cin >> l >> r; int odd = (r + 1) / 2 - l / 2; std::cout << odd / 2 << "\n"; }
}
