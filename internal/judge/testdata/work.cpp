// A fixed amount of work (not a fixed CPU time, unlike burn.cpp): about
// 0.3 s at -O2 on an M4 performance core. TestParallelDrift times it.
#include <cstdio>

int main() {
    unsigned long long x = 1;
    for (long i = 0; i < 200000000L; i++) {
        x = x * 6364136223846793005ULL + 1442695040888963407ULL;
        x ^= x >> 29;
    }
    printf("%llu\n", x & 1);
}
