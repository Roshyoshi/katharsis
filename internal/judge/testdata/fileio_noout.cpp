// Reads x.in but forgets to redirect stdout to x.out.
#include <cstdio>

int main() {
    freopen("x.in", "r", stdin);
    long long a, b;
    if (scanf("%lld %lld", &a, &b) != 2) return 2;
    printf("%lld\n", a + b);
}
