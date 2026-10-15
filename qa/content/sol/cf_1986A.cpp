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
    while (t--) { int a[3]; std::cin >> a[0] >> a[1] >> a[2]; std::sort(a, a + 3); std::cout << a[2] - a[0] << "\n"; }
}
