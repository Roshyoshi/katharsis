#!/bin/sh
# qa/chaos/oracle.sh: print a C++ "solution" that answers any test of one
# cached problem by looking its input up in the cache, so a chaos run can
# clear doors (AC) or fail them (WA) on demand.
#
#   oracle.sh DOOR_DIR CACHE_ROOT [ac|wa|slow|slowwa|spin|ce]
#
# DOOR_DIR is a door's directory (e.g. runs/boot-1/usaco1011_triangles);
# CACHE_ROOT is <cache>/katharsis. slow sleeps 3 s per test (CPU-idle, so still AC) before
# answering (a long judge); slowwa sleeps 3 s, then answers wrong; spin
# loops forever (TLE); ce does not compile.
set -eu
door=$(basename "$1")
cache=$2
mode=${3:-ac}
id=${door#mystery_}
id=${id%%_*}
case $id in
usaco*) dir=$cache/usaco/${id#usaco} ;;
cf*) dir=$cache/codeforces/${id#cf} ;;
*) echo "oracle: cannot place $door" >&2; exit 1 ;;
esac
[ -d "$dir" ] || { echo "oracle: no cache dir $dir" >&2; exit 1; }
stem=$(sed -n 's/.*"file_io": "\([^"]*\)".*/\1/p' "$dir/meta.json" | head -1)
if [ "$mode" = ce ]; then
	echo 'int main() { this does not compile; }'
	exit 0
fi
cat <<EOF
// chaos oracle ($mode) for $door: answers from $dir
#include <filesystem>
#include <fstream>
#include <iostream>
#include <sstream>
#include <string>
#include <thread>
#include <chrono>
namespace fs = std::filesystem;
static std::string trim(std::string s) {
    while (!s.empty() && (s.back() == '\n' || s.back() == ' ' || s.back() == '\r')) s.pop_back();
    return s;
}
static std::string slurp(const fs::path &p) {
    std::ifstream f(p, std::ios::binary);
    std::stringstream ss; ss << f.rdbuf(); return ss.str();
}
int main() {
EOF
if [ -n "$stem" ]; then
	echo "    freopen(\"$stem.in\", \"r\", stdin); freopen(\"$stem.out\", \"w\", stdout);"
fi
cat <<EOF
    std::string in;
    { std::stringstream ss; ss << std::cin.rdbuf(); in = ss.str(); }
    std::string out = "-1\n";
    for (const char *sub : {"samples", "tests"}) {
        fs::path d = fs::path("$dir") / sub;
        if (!fs::exists(d)) continue;
        for (auto &e : fs::directory_iterator(d)) {
            if (e.path().extension() != ".in") continue;
            if (trim(slurp(e.path())) != trim(in)) continue;
            fs::path o = e.path(); o.replace_extension(".out");
            out = slurp(o);
        }
    }
    if (std::string("$mode") == "wa" || std::string("$mode") == "slowwa") out = "-1\n";
    if (std::string("$mode") == "slowwa") std::this_thread::sleep_for(std::chrono::seconds(3));
    if (std::string("$mode") == "slow") std::this_thread::sleep_for(std::chrono::seconds(3));
    if (std::string("$mode") == "spin") for (volatile unsigned long i = 0;; i++) {}
    std::cout << out;
}
EOF
