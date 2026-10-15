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
int main() { int t; std::cin >> t; while (t--) { char c; std::cin >> c; std::cout << (std::string("codeforces").find(c) != std::string::npos ? "YES" : "NO") << "\n"; } }
