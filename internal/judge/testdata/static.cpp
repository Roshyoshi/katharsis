// A 1.2 GB global array, one byte of it touched.
#include <cstdio>

static char big[1200 << 20];

int main() {
    int i, j;
    if (scanf("%d %d", &i, &j) != 2) return 2;
    big[i] = 1;
    printf("%d\n", big[j]);
}
