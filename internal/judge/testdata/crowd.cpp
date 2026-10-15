// Answers a+b, but only when no other run holds the lock file named in
// the input: a crowded run spins until it is killed, like a program whose
// CPU time balloons under parallel load. "-" for the path skips the lock.
#include <cstdio>
#include <cstring>
#include <fcntl.h>
#include <sys/file.h>
#include <unistd.h>

int main() {
    char path[4096];
    long long a, b;
    if (scanf("%4095s %lld %lld", path, &a, &b) != 3) return 2;
    if (strcmp(path, "-") != 0) {
        int fd = open(path, O_RDWR | O_CREAT, 0644);
        if (fd < 0) return 3;
        if (flock(fd, LOCK_EX | LOCK_NB) != 0) {
            volatile unsigned long spin = 0;
            for (;;) spin++;
        }
        usleep(200000); // hold it while the other workers start
    }
    printf("%lld\n", a + b);
}
