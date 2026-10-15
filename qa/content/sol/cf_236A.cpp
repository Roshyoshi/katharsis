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
int main() { std::string s; std::cin >> s; std::set<char> st(s.begin(), s.end()); std::cout << (st.size() % 2 ? "IGNORE HIM!" : "CHAT WITH HER!") << "\n"; }
