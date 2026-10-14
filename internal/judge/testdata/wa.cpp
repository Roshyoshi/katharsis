// Prints a+b, then a*b off by one: wrong on line 2 only.
#include <cstdio>

int main() {
    long long a, b;
    if (scanf("%lld %lld", &a, &b) != 2) return 2;
    printf("%lld\n%lld\n", a + b, a * b + 1);
}
