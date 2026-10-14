// Signed int overflow when the input is INT_MAX.
#include <cstdio>

int main() {
    int a;
    if (scanf("%d", &a) != 1) return 2;
    int b = a + 1;
    printf("%d\n", b);
}
