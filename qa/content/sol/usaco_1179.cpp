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
    string a[3], g[3];
    for (auto &s : a) cin >> s;
    for (auto &s : g) cin >> s;
    int green = 0, ca[26] = {}, cg[26] = {};
    for (int i = 0; i < 3; i++)
        for (int j = 0; j < 3; j++) {
            if (a[i][j] == g[i][j]) green++;
            else ca[a[i][j] - 'A']++, cg[g[i][j] - 'A']++;
        }
    int yellow = 0;
    for (int k = 0; k < 26; k++) yellow += min(ca[k], cg[k]);
    cout << green << '\n' << yellow << '\n';
}
