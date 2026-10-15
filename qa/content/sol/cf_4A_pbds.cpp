// Watermelon through the pb_ds ordered set many Codeforces templates carry:
// g++'s libstdc++ ships it, libc++ does not.
#include <bits/stdc++.h>
#include <ext/pb_ds/assoc_container.hpp>
#include <ext/pb_ds/tree_policy.hpp>
using namespace std;
using namespace __gnu_pbds;
template <class T>
using ordered_set = tree<T, null_type, less<T>, rb_tree_tag, tree_order_statistics_node_update>;
int main() {
    int w;
    cin >> w;
    ordered_set<int> evens;
    for (int i = 2; i < w; i += 2) evens.insert(i);
    // Two even parts: w is even and some even part is below it.
    cout << (w % 2 == 0 && evens.order_of_key(w) > 0 ? "YES" : "NO") << "\n";
}
