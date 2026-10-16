// Reads n and keeps a std::map of n ints, then prints n: n small
// allocations, where ASan's redzones and chunk headers weigh the most.
#include <cstdio>
#include <map>

int main() {
    int n;
    if (scanf("%d", &n) != 1) return 2;
    std::map<int, int> m;
    for (int i = 0; i < n; i++) m[i] = i;
    long sum = 0;
    for (auto &[k, v] : m) sum += v;
    printf("%d\n", sum == long(n) * (n - 1) / 2 ? n : -1);
}
