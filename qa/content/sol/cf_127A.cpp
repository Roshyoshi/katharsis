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
    int n, k; std::cin >> n >> k;
    std::vector<double> x(n), y(n);
    for (int i = 0; i < n; i++) std::cin >> x[i] >> y[i];
    double d = 0;
    for (int i = 1; i < n; i++) d += std::hypot(x[i] - x[i - 1], y[i] - y[i - 1]);
    printf("%.9f\n", d * k / 50.0);
}
