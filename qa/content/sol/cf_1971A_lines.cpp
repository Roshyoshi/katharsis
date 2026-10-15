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
// Same answers as cf_1971A.cpp, one number per line: token-equal output.
int main() { int t; std::cin >> t; while (t--) { int x, y; std::cin >> x >> y; std::cout << std::min(x, y) << "\n" << std::max(x, y) << "\n"; } }
