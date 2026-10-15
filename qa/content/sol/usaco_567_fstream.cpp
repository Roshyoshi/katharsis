#include <algorithm>
#include <fstream>
// USACO's own analysis style: file streams instead of freopen. Correct on
// usaco.org.
int main() {
    std::ifstream fin("paint.in");
    std::ofstream fout("paint.out");
    int a, b, c, d;
    fin >> a >> b >> c >> d;
    fout << (b - a) + (d - c) - std::max(0, std::min(b, d) - std::max(a, c)) << "\n";
}
