// Touches every page of a 300 MiB buffer, then prints ok.
#include <cstdio>
#include <vector>

int main() {
    int seed;
    if (scanf("%d", &seed) != 1) return 2;
    const size_t n = size_t(300) << 20;
    std::vector<char> v(n);
    for (size_t i = 0; i < n; i += 4096) v[i] = static_cast<char>((i >> 12) * seed + 1);
    long sum = 0;
    for (size_t i = 0; i < n; i += 4096) sum += v[i];
    volatile long sink = sum;
    (void)sink;
    puts("ok");
}
