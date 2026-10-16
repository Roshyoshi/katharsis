// A contest template as players write one, which the sample warnings must
// pass in silence: every header, a typedef it never uses, and the
// unsigned size compared with an int.
#include <bits/stdc++.h>
using namespace std;
typedef long long ll;
typedef pair<int, int> pii;
int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    int n;
    cin >> n;
    vector<ll> v(n);
    for (auto &x : v) cin >> x;
    ll s = 0;
    for (int i = 0; i < v.size(); i++) s += v[i];
    cout << s << '\n';
}
