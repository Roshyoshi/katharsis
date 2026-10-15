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
int main() { int n; std::string s; std::cin >> n >> s; int c = 0; for (int i = 1; i < n; i++) c += s[i] == s[i - 1]; std::cout << c << "\n"; }
