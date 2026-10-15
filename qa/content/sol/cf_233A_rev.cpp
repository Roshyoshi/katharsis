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
// Valid: for even n, p_i = n+1-i is a perfect permutation.
int main() {
    int n; std::cin >> n;
    if (n % 2) { std::cout << -1 << "\n"; return 0; }
    for (int i = 1; i <= n; i++) std::cout << n + 1 - i << (i == n ? "\n" : " ");
}
