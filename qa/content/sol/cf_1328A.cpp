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
int main() { int t; std::cin >> t; while (t--) { long long a, b; std::cin >> a >> b; std::cout << (b - a % b) % b << "\n"; } }
