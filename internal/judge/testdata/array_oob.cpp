// a[4] of a std::array<int, 4>: libc++'s debug hardening stops it. ASan
// alone does not, since the element past the end is the struct's next
// field.
#include <array>
#include <cstdio>

struct S {
    std::array<int, 4> a;
    int after;
};

int main() {
    int i;
    if (scanf("%d", &i) != 1) return 2;
    S x{};
    printf("%d\n", x.a[i]);
}
