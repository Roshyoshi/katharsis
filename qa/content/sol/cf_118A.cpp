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
    std::string s, r; std::cin >> s;
    for (char c : s) {
        c = tolower(c);
        if (std::string("aoyeui").find(c) != std::string::npos) continue;
        r += '.'; r += c;
    }
    std::cout << r << "\n";
}
