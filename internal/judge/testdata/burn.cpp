// Burns the CPU time its input names, in ms, then answers a+b correctly:
// slow, not hung.
#include <cstdio>
#include <ctime>

int main() {
    long long ms, a, b;
    if (scanf("%lld %lld %lld", &ms, &a, &b) != 3) return 2;
    while (clock() < ms * (CLOCKS_PER_SEC / 1000)) {
    }
    printf("%lld\n", a + b);
}
