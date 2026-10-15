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
    int n; std::cin >> n;
    while (n--) {
        std::string s; std::cin >> s;
        if (s.size() > 10) std::cout << s[0] << s.size() - 2 << s.back() << "\n";
        else std::cout << s << "\n";
    }
}
