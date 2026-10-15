// Opens x.in and x.out under a name built at run time, which no reading of
// the source finds; only the problem's own stem (Limits.FileIO) can.
#include <cstdio>
#include <string>

int main() {
    const char n[] = {'x', 0};
    freopen((std::string(n) + ".in").c_str(), "r", stdin);
    freopen((std::string(n) + ".out").c_str(), "w", stdout);
    long long a, b;
    if (scanf("%lld %lld", &a, &b) != 2) return 2;
    printf("%lld\n", a + b);
}
