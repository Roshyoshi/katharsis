// A target pragma, then a compile error on line 7, which g++ must still
// report as line 7 of this file.
#pragma GCC target("avx2")
#include <cstdio>

int main() {
    undeclared();
}
