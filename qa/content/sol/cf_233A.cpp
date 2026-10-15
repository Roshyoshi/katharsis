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
    int n; std::cin >> n;
    if (n % 2) { std::cout << -1 << "\n"; return 0; }
    for (int i = 1; i <= n; i += 2) std::cout << i + 1 << " " << i << (i + 1 == n ? "\n" : " ");
}
