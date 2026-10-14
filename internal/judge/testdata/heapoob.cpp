// Writes one past the end of a heap vector.
#include <cstdio>
#include <vector>

int main() {
    int i;
    if (scanf("%d", &i) != 1) return 2;
    std::vector<int> v(10);
    v[i] = 1;
    printf("%d\n", v[i]);
}
