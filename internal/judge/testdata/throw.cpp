// Throws past main.
#include <cstdio>
#include <stdexcept>

int main() {
    int x;
    if (scanf("%d", &x) != 1) return 2;
    if (x == 1) throw std::runtime_error("boom");
    printf("%d\n", x);
}
