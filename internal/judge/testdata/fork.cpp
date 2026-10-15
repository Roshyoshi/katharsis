// Forks a child that outlives the answer, holding stdout and stderr; the
// argument to sleep marks it for pgrep. Then answers a+b.
#include <cstdio>
#include <unistd.h>

int main() {
    long long a, b;
    if (scanf("%lld %lld", &a, &b) != 2) return 2;
    fflush(stdout);
    if (fork() == 0) {
        execl("/bin/sleep", "sleep", "31.41592", (char *)0);
        _exit(0);
    }
    printf("%lld\n", a + b);
}
