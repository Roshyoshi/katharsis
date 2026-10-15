// Blocks without using CPU until it is killed, like a deadlocked solution.
#include <unistd.h>

int main() {
    for (;;) pause();
}
