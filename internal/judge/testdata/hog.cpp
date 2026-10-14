// Allocates and touches 64 MiB at a time without end, as a runaway push_back
// would. It stops by itself at 2 GiB and prints "survived", so a broken
// watchdog fails the test instead of eating the machine. The volatile
// pointer makes each block escape, so -O2 cannot elide the allocation.
#include <cstdio>
#include <cstdlib>
#include <cstring>

static char *volatile keep;

int main() {
    const size_t chunk = size_t(64) << 20;
    for (size_t total = 0; total < (size_t(2) << 30); total += chunk) {
        char *p = static_cast<char *>(malloc(chunk));
        if (!p) return 3;
        memset(p, 1, chunk);
        keep = p;
    }
    puts("survived");
}
