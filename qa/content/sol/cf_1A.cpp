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
int main() { long long n, m, a; std::cin >> n >> m >> a; std::cout << ((n + a - 1) / a) * ((m + a - 1) / a) << "\n"; }
