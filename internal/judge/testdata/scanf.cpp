// Sums n ints read through scanf: cin.cpp's control.
#include <cstdio>

int main() {
    long n, s = 0;
    if (scanf("%ld", &n) != 1) return 2;
    for (long i = 0; i < n; i++) {
        int x;
        if (scanf("%d", &x) != 1) return 2;
        s += x;
    }
    printf("%ld\n", s);
}
