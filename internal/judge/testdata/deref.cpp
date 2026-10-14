// Stores through an address read from input: 0 is a null deref (UBSan's null
// check), a small nonzero address lands in __PAGEZERO (ASan SEGV).
#include <cstdio>

int main() {
    unsigned long addr;
    if (scanf("%lu", &addr) != 1) return 2;
    int *p = reinterpret_cast<int *>(addr);
    *p = 42;
    printf("%d\n", *p);
}
