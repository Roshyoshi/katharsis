#include <algorithm>
#include <array>
#include <climits>
#include <cmath>
#include <cstdio>
#include <cstdlib>
#include <iomanip>
#include <iostream>
#include <map>
#include <set>
#include <string>
#include <vector>
using namespace std;
int main() {
    string order, s;
    cin >> order >> s;
    int pos[26];
    for (int i = 0; i < 26; i++) pos[order[i] - 'a'] = i;
    int cnt = 1;
    for (size_t i = 1; i < s.size(); i++)
        if (pos[s[i] - 'a'] <= pos[s[i - 1] - 'a']) cnt++;
    cout << cnt << '\n';
}
