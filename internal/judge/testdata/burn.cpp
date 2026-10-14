// Burns about 500 ms of CPU, then answers a+b correctly: slow, not hung.
#include <cstdio>
#include <ctime>

int main() {
    while (clock() < CLOCKS_PER_SEC / 2) {
    }
    long long a, b;
    if (scanf("%lld %lld", &a, &b) != 2) return 2;
    printf("%lld\n", a + b);
}
