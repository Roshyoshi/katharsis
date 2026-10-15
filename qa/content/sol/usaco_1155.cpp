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
    int n;
    string s;
    cin >> n >> s;
    vector<long long> L(n), R(n);
    for (int i = 0; i < n; i++) {
        long long k = 0;
        for (int j = i - 1; j >= 0 && s[j] != s[i]; j--) k++;
        L[i] = k;
    }
    // O(n) left/right runs of the other breed.
    vector<long long> left(n), right(n);
    for (int i = 0; i < n; i++) {
        if (i > 0 && s[i - 1] != s[i]) left[i] = (i >= 2 && s[i - 2] == s[i - 1]) ? left[i - 1] + 0 : 0;
    }
    // Recompute directly with run lengths.
    vector<int> runStart(n), runEnd(n);
    for (int i = 0; i < n; i++) runStart[i] = (i > 0 && s[i - 1] == s[i]) ? runStart[i - 1] : i;
    for (int i = n - 1; i >= 0; i--) runEnd[i] = (i + 1 < n && s[i + 1] == s[i]) ? runEnd[i + 1] : i;
    long long total = 0;
    for (int i = 0; i < n; i++) {
        long long l = (i > 0 && s[i - 1] != s[i]) ? (i - 1) - runStart[i - 1] + 1 : 0;
        long long r = (i + 1 < n && s[i + 1] != s[i]) ? runEnd[i + 1] - (i + 1) + 1 : 0;
        total += l * r + max(l - 1, 0LL) + max(r - 1, 0LL);
    }
    cout << total << '\n';
}
