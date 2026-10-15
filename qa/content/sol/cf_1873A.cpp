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
    while (t--) {
        std::string s; std::cin >> s;
        int diff = 0; for (int i = 0; i < 3; i++) diff += s[i] != "abc"[i];
        std::cout << (diff <= 2 ? "YES" : "NO") << "\n";
    }
}
