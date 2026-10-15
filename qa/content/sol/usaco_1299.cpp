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
using namespace std;
int main() {
    long long n, t;
    cin >> n >> t;
    long long stock = 0, prev = 1, total = 0;
    for (int i = 0; i < n; i++) {
        long long d, b;
        cin >> d >> b;
        long long eat = min(stock, d - prev);
        total += eat;
        stock -= eat;
        prev = d;
        stock += b;
    }
    total += min(stock, t - prev + 1);
    cout << total << '\n';
}
