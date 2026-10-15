// What only libstdc++ has: policy-based trees, and __int128 as an integral
// type under gnu++, as on Codeforces' g++.
#include <bits/stdc++.h>
#include <ext/pb_ds/assoc_container.hpp>
#include <ext/pb_ds/tree_policy.hpp>
using namespace std;
using namespace __gnu_pbds;

int main() {
    tree<int, null_type, less<int>, rb_tree_tag, tree_order_statistics_node_update> t;
    int n;
    cin >> n;
    for (int i = 0; i < n; i++) t.insert(i * 10);
    static_assert(is_integral_v<__int128>);
    cout << *t.find_by_order(2) << " " << t.order_of_key(25) << "\n";
}
