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
    int k; std::string s; std::cin >> k >> s;
    int cnt[26] = {}; for (char c : s) cnt[c - 'a']++;
    std::string base;
    for (int i = 0; i < 26; i++) { if (cnt[i] % k) { std::cout << -1 << "\n"; return 0; } base += std::string(cnt[i] / k, 'a' + i); }
    std::string r; for (int i = 0; i < k; i++) r += base;
    std::cout << r << "\n";
}
