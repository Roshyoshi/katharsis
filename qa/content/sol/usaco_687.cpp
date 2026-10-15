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
    freopen("notlast.in", "r", stdin);
    freopen("notlast.out", "w", stdout);
    vector<string> names = {"Bessie", "Elsie", "Daisy", "Gertie", "Annabelle", "Maggie", "Henrietta"};
    map<string, int> milk;
    for (auto &s : names) milk[s] = 0;
    int n;
    cin >> n;
    for (int i = 0; i < n; i++) { string s; int x; cin >> s >> x; milk[s] += x; }
    int m = INT_MAX;
    for (auto &[s, v] : milk) m = min(m, v);
    int second = INT_MAX;
    for (auto &[s, v] : milk) if (v > m) second = min(second, v);
    vector<string> who;
    for (auto &[s, v] : milk) if (v == second) who.push_back(s);
    if (who.size() == 1) cout << who[0] << "\n";
    else cout << "Tie\n";
}
