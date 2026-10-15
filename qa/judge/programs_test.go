//go:build qa

package judgeqa

// Crafted C++ programs. Inputs come from stdin so -O2 cannot fold the
// behavior away.

const progAB = `#include <cstdio>
int main() {
    long long a, b;
    if (scanf("%lld %lld", &a, &b) != 2) return 2;
    printf("%lld\n", a + b);
}
`

// progCat copies stdin to stdout byte for byte: the input is the output,
// so a case can hand the checker any output it likes.
const progCat = `#include <cstdio>
int main() {
    int c;
    while ((c = getchar()) != EOF) putchar(c);
}
`

// progBurn burns the given number of CPU milliseconds (clock() is process
// CPU time), then prints ok.
const progBurn = `#include <cstdio>
#include <ctime>
int main() {
    double ms;
    if (scanf("%lf", &ms) != 1) return 2;
    clock_t lim = (clock_t)(ms / 1000.0 * CLOCKS_PER_SEC);
    while (clock() < lim) {
    }
    puts("ok");
}
`

// progWork does a fixed amount of work: n million LCG steps.
const progWork = `#include <cstdio>
int main() {
    long n;
    if (scanf("%ld", &n) != 1) return 2;
    unsigned long long x = 1;
    for (long i = 0; i < n * 1000000L; i++) {
        x = x * 6364136223846793005ULL + 1442695040888963407ULL;
        x ^= x >> 29;
    }
    printf("%llu\n", x & 1);
}
`

const progSpin = `int main() {
    volatile unsigned long spin = 0;
    for (;;) spin++;
}
`

const progSleep = `#include <cstdio>
#include <unistd.h>
int main() {
    long us;
    if (scanf("%ld", &us) != 1) return 2;
    usleep(us);
    puts("ok");
}
`

// progGrow allocates and touches 1 MiB at a time up to n MiB.
const progGrow = `#include <cstdio>
#include <cstring>
#include <cstdlib>
static char *volatile keep;
int main() {
    long n;
    if (scanf("%ld", &n) != 1) return 2;
    for (long i = 0; i < n; i++) {
        char *p = (char *)malloc(1 << 20);
        memset(p, 1, 1 << 20);
        keep = p;
    }
    puts("ok");
}
`

// progPushBack grows one vector<int> to n million elements.
const progPushBack = `#include <cstdio>
#include <vector>
int main() {
    long n;
    if (scanf("%ld", &n) != 1) return 2;
    std::vector<int> v;
    for (long i = 0; i < n * 1000000L; i++) v.push_back((int)i);
    printf("%d\n", v[n] & 0 ? 1 : 0);
}
`

// progHuge allocates n MiB in one block; touch=1 writes every page.
const progHuge = `#include <cstdio>
#include <cstring>
#include <cstdlib>
static char *volatile keep;
int main() {
    long n, touch;
    if (scanf("%ld %ld", &n, &touch) != 2) return 2;
    char *p = (char *)malloc((size_t)n << 20);
    if (!p) { puts("null"); return 0; }
    if (touch) memset(p, 1, (size_t)n << 20);
    p[0] = 1;
    keep = p;
    puts("ok");
}
`

// progReserve reserves n million ints and uses one.
const progReserve = `#include <cstdio>
#include <vector>
int main() {
    long n;
    if (scanf("%ld", &n) != 1) return 2;
    std::vector<int> v;
    v.reserve(n * 1000000L);
    v.push_back(7);
    printf("%d\n", v[0] == 7 ? 0 : 1);
}
`

// progStatic declares a 1.2 GB static array and touches k entries.
const progStatic = `#include <cstdio>
static int a[300000000];
int main() {
    int k;
    if (scanf("%d", &k) != 1) return 2;
    for (int i = 0; i < k; i++) a[i] = i;
    long s = 0;
    for (int i = 0; i < k; i++) s += a[i];
    printf("%d\n", s >= 0 ? 0 : 1);
}
`

const progSegv = `#include <cstdio>
int main() {
    unsigned long addr;
    if (scanf("%lu", &addr) != 1) return 2;
    int *p = reinterpret_cast<int *>(addr);
    *p = 42;
    printf("%d\n", *p);
}
`

const progAbort = `#include <cstdio>
#include <cstdlib>
int main() {
    int x;
    if (scanf("%d", &x) != 1) return 2;
    if (x) abort();
    puts("0");
}
`

const progAssert = `#include <cassert>
#include <cstdio>
int main() {
    int n;
    if (scanf("%d", &n) != 1) return 2;
    assert(n < 5 && "n too big");
    printf("%d\n", n);
}
`

const progThrow = `#include <cstdio>
#include <stdexcept>
int main() {
    int x;
    if (scanf("%d", &x) != 1) return 2;
    if (x) throw std::runtime_error("boom at x");
    puts("0");
}
`

const progAt = `#include <cstdio>
#include <vector>
int main() {
    int i;
    if (scanf("%d", &i) != 1) return 2;
    std::vector<int> v(10);
    printf("%d\n", v.at(i));
}
`

// progRecurse: n-deep plain recursion with a small live frame.
const progRecurse = `#include <cstdio>
static long depth(long d) {
    volatile char frame[64];
    frame[d & 63] = (char)d;
    if (d == 0) return 0;
    long r = depth(d - 1) + 1;
    return r + (frame[d & 63] & 0);
}
int main() {
    long n;
    if (scanf("%ld", &n) != 1) return 2;
    printf("%ld\n", depth(n));
}
`

const progDiv = `#include <cstdio>
int main() {
    int a, b;
    if (scanf("%d %d", &a, &b) != 2) return 2;
    printf("%d\n", a / b);
}
`

const progMod = `#include <cstdio>
int main() {
    long long a, b;
    if (scanf("%lld %lld", &a, &b) != 2) return 2;
    printf("%lld\n", a % b);
}
`

const progOverflow = `#include <cstdio>
int main() {
    int a;
    if (scanf("%d", &a) != 1) return 2;
    int b = a + 1;
    printf("%d\n", b);
}
`

// progHeapOOB writes v[i] for a vector of size 10 (capacity 10).
const progHeapOOB = `#include <cstdio>
#include <vector>
int main() {
    int i;
    if (scanf("%d", &i) != 1) return 2;
    std::vector<int> v(10);
    v[i] = 1;
    printf("%d\n", v[i]);
}
`

// progContainerOOB writes past size() but within capacity().
const progContainerOOB = `#include <cstdio>
#include <vector>
int main() {
    int i;
    if (scanf("%d", &i) != 1) return 2;
    std::vector<int> v;
    for (int k = 0; k < 5; k++) v.push_back(k);
    v[i] = 1;
    printf("%d %zu\n", v[i], v.capacity());
}
`

const progGlobalOOB = `#include <cstdio>
int a[10];
int main() {
    int i;
    if (scanf("%d", &i) != 1) return 2;
    a[i] = 1;
    printf("%d\n", a[i]);
}
`

const progUAF = `#include <cstdio>
int main() {
    int i;
    if (scanf("%d", &i) != 1) return 2;
    int *p = new int[4];
    delete[] p;
    p[i] = 3;
    printf("%d\n", p[i]);
}
`

const progSpam = `#include <cstdio>
int main() {
    for (;;) fputs("spam spam spam spam spam spam spam\n", stdout);
}
`

// progStderr prints n MiB of debug lines to stderr, then a+b to stdout.
const progStderr = `#include <cstdio>
int main() {
    long n, a, b;
    if (scanf("%ld %ld %ld", &n, &a, &b) != 3) return 2;
    for (long i = 0; i < n * 1024 * 16; i++) fprintf(stderr, "dbg i=%08ld xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n", i);
    printf("%ld\n", a + b);
}
`

// progDebugThenOOB prints k debug lines to stderr, then overflows a vector.
const progDebugThenOOB = `#include <cstdio>
#include <vector>
int main() {
    int k, i;
    if (scanf("%d %d", &k, &i) != 2) return 2;
    std::vector<int> v(10);
    for (int j = 0; j < k; j++) fprintf(stderr, "dbg j=%d v.size()=%zu\n", j, v.size());
    v[i] = 1;
    printf("%d\n", v[i]);
}
`

// progForkChild forks a child that sleeps while holding stdout, then the
// parent answers and exits.
const progForkChild = `#include <cstdio>
#include <unistd.h>
int main() {
    long a, b;
    if (scanf("%ld %ld", &a, &b) != 2) return 2;
    fflush(stdout);
    if (fork() == 0) {
        execl("/bin/sleep", "sleep", "6.4242", (char *)0);
        _exit(0);
    }
    printf("%ld\n", a + b);
}
`

// progForkDetached forks a child that closes its pipes and sleeps: it
// outlives the run.
const progForkDetached = `#include <cstdio>
#include <unistd.h>
int main() {
    long a, b;
    if (scanf("%ld %ld", &a, &b) != 2) return 2;
    fflush(stdout);
    if (fork() == 0) {
        close(0); close(1); close(2);
        execl("/bin/sleep", "sleep", "7.4242", (char *)0);
        _exit(0);
    }
    printf("%ld\n", a + b);
}
`

const progCE = `#include <cstdio>
int main() {
    int x = 1
    printf("%d\n", x);
}
`

const progEmpty = `int main() {}
`

const progNoNewline = `#include <cstdio>
int main() { long long a, b; scanf("%lld %lld", &a, &b); printf("%lld", a + b); }
`

// ---- file I/O variants (the problem's stem is "milk2") ----

const progFreopen = `#include <cstdio>
int main() {
    freopen("milk2.in", "r", stdin);
    freopen("milk2.out", "w", stdout);
    long long a, b;
    if (scanf("%lld %lld", &a, &b) != 2) return 2;
    printf("%lld\n", a + b);
}
`

// progSetIO is the template usaco.guide/general/input-output teaches.
const progSetIO = `#include <cstdio>
#include <iostream>
#include <string>
using namespace std;
void setIO(string name = "") {
    cin.tie(0)->sync_with_stdio(0);
    if ((int)name.size()) {
        freopen((name + ".in").c_str(), "r", stdin);
        freopen((name + ".out").c_str(), "w", stdout);
    }
}
int main() {
    setIO("milk2");
    long long a, b;
    cin >> a >> b;
    cout << a + b << "\n";
}
`

// progFstream is the USACO training pages' style.
const progFstream = `#include <fstream>
using namespace std;
int main() {
    ifstream fin("milk2.in");
    ofstream fout("milk2.out");
    long long a, b;
    fin >> a >> b;
    fout << a + b << "\n";
}
`

const progFreopenBlockComment = `#include <cstdio>
/* old version:
   freopen("milk2.in", "r", stdin);
*/
int main() {
    long long a, b;
    if (scanf("%lld %lld", &a, &b) != 2) return 2;
    printf("%lld\n", a + b);
}
`

const progFreopenWrongStem = `#include <cstdio>
int main() {
    freopen("milk.in", "r", stdin);
    freopen("milk.out", "w", stdout);
    long long a, b;
    if (scanf("%lld %lld", &a, &b) != 2) return 2;
    printf("%lld\n", a + b);
}
`

// progOnlineJudge is the common Codeforces template guard.
const progOnlineJudge = `#include <cstdio>
int main() {
#ifndef ONLINE_JUDGE
    freopen("input.txt", "r", stdin);
    freopen("output.txt", "w", stdout);
#endif
    long long a, b;
    if (scanf("%lld %lld", &a, &b) != 2) return 2;
    printf("%lld\n", a + b);
}
`

// ---- GNU-isms every Codeforces/USACO judge (g++, libstdc++) accepts ----

const progBits = `#include <bits/stdc++.h>
using namespace std;
int main() { long long a, b; cin >> a >> b; cout << a + b << "\n"; }
`

const progGcd = `#include <algorithm>
#include <cstdio>
int main() { long long a, b; scanf("%lld %lld", &a, &b); printf("%lld\n", std::__gcd(a, b) * 0 + a + b); }
`

const progLg = `#include <algorithm>
#include <cstdio>
int main() { long long a, b; scanf("%lld %lld", &a, &b); printf("%lld\n", (long long)std::__lg(1024) * 0 + a + b); }
`

const progPbds = `#include <ext/pb_ds/assoc_container.hpp>
#include <ext/pb_ds/tree_policy.hpp>
#include <cstdio>
using namespace __gnu_pbds;
tree<int, null_type, std::less<int>, rb_tree_tag, tree_order_statistics_node_update> t;
int main() { long long a, b; scanf("%lld %lld", &a, &b); t.insert(1); printf("%lld\n", a + b); }
`

const progPragmaTarget = `#pragma GCC optimize("O3,unroll-loops")
#pragma GCC target("avx2,bmi,bmi2,popcnt,lzcnt")
#include <cstdio>
int main() { long long a, b; scanf("%lld %lld", &a, &b); printf("%lld\n", a + b); }
`

// progSqrtl: floor(sqrt(n)) via long double, correct with x86's 80-bit
// long double for n < 2^64.
const progSqrtl = `#include <cmath>
#include <cstdio>
int main() {
    long long n;
    if (scanf("%lld", &n) != 1) return 2;
    long long r = (long long)sqrtl((long double)n);
    printf("%lld\n", r);
}
`

// progLongDoubleSum adds 1 to 2^53 as long double; x86's 64-bit mantissa
// keeps it, a 53-bit double does not.
const progLongDoubleSum = `#include <cstdio>
int main() {
    long long n;
    if (scanf("%lld", &n) != 1) return 2;
    long double x = (long double)n;
    x += 1;
    printf("%lld\n", (long long)x);
}
`

// progLong: long is 32 bits on Codeforces' Windows toolchains.
const progLong = `#include <cstdio>
int main() {
    long a, b;
    if (scanf("%ld %ld", &a, &b) != 2) return 2;
    printf("%ld\n", a * b);
}
`
