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
int main() { int n; std::cin >> n; double s = 0; for (int i = 1; i <= n; i++) s += 1.0 / i; printf("%.12f\n", s); }
