// USACO's training-page style: file streams instead of freopen (cpid 567's
// analysis does this).
#include <fstream>

int main() {
    std::ifstream fin("x.in");
    std::ofstream fout("x.out");
    long long a, b;
    fin >> a >> b;
    fout << a + b << "\n";
}
