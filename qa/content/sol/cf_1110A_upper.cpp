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
    long long b, k; std::cin >> b >> k;
    std::vector<long long> a(k); for (auto &x : a) std::cin >> x;
    long long par = 0;
    if (b % 2 == 0) par = a[k - 1] % 2;
    else { for (auto x : a) par += x; par %= 2; }
    std::cout << (par ? "ODD" : "EVEN") << "\n";
}
