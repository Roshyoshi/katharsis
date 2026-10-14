// Never terminates. The volatile store is a side effect, so the loop is not
// UB and cannot be optimized away.
int main() {
    volatile unsigned long spin = 0;
    for (;;) spin++;
}
