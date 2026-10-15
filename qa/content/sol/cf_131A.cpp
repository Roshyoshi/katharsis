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
    std::string s; std::cin >> s;
    bool caps = true;
    for (size_t i = 1; i < s.size(); i++) if (!isupper(s[i])) caps = false;
    if (caps) for (char &c : s) c = isupper(c) ? tolower(c) : toupper(c);
    std::cout << s << "\n";
}
