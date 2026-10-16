// Counts with a loop variable it never set: undefined, and only a warning
// finds it, as neither ASan nor UBSan sees an uninitialized read.
#include <bits/stdc++.h>
using namespace std;
int main() {
    int t;
    cin >> t;
    for (int i; i < t; i++) cout << i << '\n';
}
