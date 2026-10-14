// a+b on stdin. A negative a spins forever, which the cancellation test uses
// to keep a run alive until the context is cancelled.
#include <cstdio>

int main() {
    long long a, b;
    if (scanf("%lld %lld", &a, &b) != 2) return 2;
    if (a < 0) {
        volatile unsigned long spin = 0;
        for (;;) spin++;
    }
    printf("%lld\n", a + b);
}
