// Fails an assert on 7.
#include <cassert>
#include <cstdio>

int main() {
    int x;
    if (scanf("%d", &x) != 1) return 2;
    assert(x != 7);
    printf("%d\n", x);
}
