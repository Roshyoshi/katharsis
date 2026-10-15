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
    freopen("word.in", "r", stdin);
    freopen("word.out", "w", stdout);
    int n, k;
    cin >> n >> k;
    int len = 0;
    for (int i = 0; i < n; i++) {
        string w;
        cin >> w;
        if (len == 0) { cout << w; len = w.size(); }
        else if (len + (int)w.size() <= k) { cout << ' ' << w; len += w.size(); }
        else { cout << '\n' << w; len = w.size(); }
    }
    cout << '\n';
}
