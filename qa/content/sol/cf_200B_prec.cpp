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
// 4 decimals: within the stated 1e-4.
int main() { int n; std::cin >> n; double s = 0; for (int i = 0; i < n; i++) { int p; std::cin >> p; s += p; } printf("%.4f\n", s / n); }
