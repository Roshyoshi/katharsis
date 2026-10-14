// Prints 1/n to 9 places and the word "ok"; the expected outputs round
// differently, so only a tolerant checker accepts them.
#include <cstdio>

int main() {
    int n;
    if (scanf("%d", &n) != 1) return 2;
    printf("%.9f ok\n", 1.0 / n);
}
