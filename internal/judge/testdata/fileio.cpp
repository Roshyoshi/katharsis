// Old USACO style: reads x.in, writes x.out.
#include <cstdio>

int main() {
    freopen("x.in", "r", stdin);
    freopen("x.out", "w", stdout);
    long long a, b;
    if (scanf("%lld %lld", &a, &b) != 2) return 2;
    printf("%lld\n", a + b);
}
