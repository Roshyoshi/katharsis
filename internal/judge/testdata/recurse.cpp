// Recurses n deep with a 128-byte frame that must live across the call, so
// the recursion can be neither inlined nor turned into a loop. At n = 1e6
// that is well over the 8 MB default stack.
#include <cstdio>

static long depth(int d) {
    volatile char frame[128];
    frame[d & 127] = static_cast<char>(d);
    if (d == 0) return 0;
    long r = depth(d - 1) + 1;
    return r + (frame[d & 127] & 0);
}

int main() {
    int n;
    if (scanf("%d", &n) != 1) return 2;
    printf("%ld\n", depth(n));
}
