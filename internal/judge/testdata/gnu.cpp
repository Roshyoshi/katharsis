// What g++ and libstdc++ accept and libc++ alone does not: the catch-all
// header, std::__gcd on signed integers, std::__lg, and ONLINE_JUDGE.
#include <bits/stdc++.h>
using namespace std;

int main() {
#ifndef ONLINE_JUDGE
    freopen("input.txt", "r", stdin);
#endif
    long long a, b;
    cin >> a >> b;
    vector<long long> v{a, b};
    ranges::sort(v);
    cout << __gcd(a, b) << " " << __lg(1024) << " " << v[0] << "\n";
}
