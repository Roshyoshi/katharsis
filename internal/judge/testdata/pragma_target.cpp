// Codeforces templates' x86 pragmas: clang ignores them, and arm64 g++
// rejects every x86 name unless the judge drops the line.
#pragma GCC optimize("O3,unroll-loops")
#pragma GCC target("avx2,bmi,bmi2,popcnt,lzcnt")
#include <cstdio>

int main() {
    long long a, b;
    if (scanf("%lld %lld", &a, &b) != 2) return 2;
    printf("%lld\n", a + b);
}
