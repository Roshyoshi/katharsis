// s[i] past size(): libc++'s debug hardening stops it. ASan alone does
// not, since a short string's characters live inside the object.
#include <cstdio>
#include <string>

int main() {
    int i;
    if (scanf("%d", &i) != 1) return 2;
    std::string s = "abc";
    printf("%d\n", s[i]);
}
