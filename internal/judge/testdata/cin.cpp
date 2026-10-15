// Sums n ints read through cin, synced off: how contest code reads input.
#include <iostream>

int main() {
    std::ios::sync_with_stdio(false);
    std::cin.tie(nullptr);
    long n, s = 0;
    std::cin >> n;
    for (long i = 0; i < n; i++) {
        int x;
        std::cin >> x;
        s += x;
    }
    std::cout << s << "\n";
}
