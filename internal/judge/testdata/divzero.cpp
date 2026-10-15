// Prints a / b; b = 0 divides by zero, which x86-64 judges fault on.
#include <cstdio>

int main() {
    int a, b;
    if (scanf("%d %d", &a, &b) != 2) return 2;
    printf("%d\n", a / b);
}
