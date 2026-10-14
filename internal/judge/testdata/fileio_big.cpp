// Writes 4 MiB to x.out, then exits normally.
#include <cstdio>
#include <cstring>

int main() {
    freopen("x.in", "r", stdin);
    freopen("x.out", "w", stdout);
    static char line[1024];
    memset(line, 'x', sizeof line - 1);
    line[sizeof line - 2] = '\n';
    for (int i = 0; i < 4096; i++) fputs(line, stdout);
}
