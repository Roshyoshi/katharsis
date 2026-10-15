#include <algorithm>
#include <cmath>
#include <cstdio>
#include <iostream>
#include <vector>
// E_{k+1} = sum_i p_i max(1+i, E_k). Within a phase (E between two
// integers) the map is E -> E + (a - cE), with c the tail mass and a its
// first moment, so E_{k+n} = F - (F - E_k)(1-c)^n with F = a/c. Computing
// c directly (not as 1 - b) keeps precision: doubles alone lose the tiny
// per-step gains over 1e9 steps.
int main() {
    long long T, K;
    std::cin >> T >> K;
    int n = T - 1;
    std::vector<double> lg(n + 2, 0), p(n + 1);
    for (int i = 1; i <= n + 1; i++) lg[i] = lg[i - 1] + std::log((double)i);
    for (int i = 0; i <= n; i++) p[i] = std::exp(lg[n] - lg[i] - lg[n - i] - n * std::log(2.0));
    double E = 1 + n / 2.0;
    long long k = 1;
    while (k < K) {
        long long m = (long long)std::floor(E) + 1;
        double a = 0, c = 0;
        for (int i = n; i >= 0 && 1 + i > E; i--) c += p[i], a += p[i] * (1 + i);
        long long rem = K - k;
        double F = a / c, lb = std::log1p(-c);
        long long steps = rem;
        if (F > m) {
            double need = std::log((F - m) / (F - E)) / lb; // steps to reach m
            if (need < (double)rem) steps = std::max(1LL, (long long)std::ceil(need));
        }
        if (steps > 1) {
            E = F - (F - E) * std::exp((double)(steps - 1) * lb);
        }
        E = E + (a - c * E);
        k += steps;
    }
    printf("%.12f\n", E * 0.99);
}
