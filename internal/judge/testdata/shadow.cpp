// Prints 2 then 1, with one warning: the inner n shadows the outer.
#include <bits/stdc++.h>
using namespace std;
int main() {
    int n = 1;
    { int n = 2; cout << n << '\n'; }
    cout << n << '\n';
}
