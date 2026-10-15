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
    freopen("lostcow.in", "r", stdin);
    freopen("lostcow.out", "w", stdout);
    long long x, y;
    cin >> x >> y;
    long long pos = x, step = 1, dir = 1, total = 0;
    while (true) {
        long long target = x + dir * step;
        if ((pos <= y && y <= target) || (target <= y && y <= pos)) {
            total += llabs(y - pos);
            break;
        }
        total += llabs(target - pos);
        pos = target;
        step *= 2;
        dir = -dir;
    }
    cout << total << "\n";
}
