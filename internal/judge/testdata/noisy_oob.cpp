// Writes 64 KB of debug lines to stderr, then one past the end of a heap
// vector: the sanitizer's report comes after the noise.
#include <cstdio>
#include <vector>

int main() {
    int i;
    if (scanf("%d", &i) != 1) return 2;
    std::vector<int> v(10);
    for (int j = 0; j < 2048; j++) fprintf(stderr, "debug j=%04d xxxxxxxxxxxxxx\n", j);
    v[i] = 1;
    printf("%d\n", v[i]);
}
