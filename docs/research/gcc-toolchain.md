# GCC toolchain for the judge (D28)

Toolchain thread, 2026-10-14. Apple M4 (10 cores), macOS 26.5, nixpkgs as
pinned in `flake.lock`. Status: applied in the commits that add this file.

**Why.** Through Apple's libc++, `cin >> int` is ~16x slower than `scanf` even
with `sync_with_stdio(false)`, so solutions that pass on Codeforces' and
USACO's g++ (libstdc++) TLE here. GCC also brings what contest code expects
of the judge natively: `<bits/stdc++.h>`, `__gnu_pbds`, `std::__gcd`,
`std::__lg`, and libstdc++'s `__int128` behavior.

## Result

```
:Submit  g++ 14.4 -std=gnu++23 -O2 (div trap, 512 MB stack)    profile "O2 (g++)"
:Debug   g++ 14.4 -O0 -g                                         "debug (g++)"
save     clang 22 ASan+UBSan against GCC's libstdc++             "asan+ubsan (libstdc++)"
         + -D_GLIBCXX_SANITIZE_VECTOR -D_GLIBCXX_ASSERTIONS
fallback without a working g++: clang + libc++, as before (judge.Profiles)
```

`judge.SampleProfile` first builds and runs a hello world with g++ (checking
`__GLIBCXX__` and `!__clang__`, since `/usr/bin/g++` is Apple clang). If it
runs, the candidates are the libstdc++ ones (ASan, then g++ with UBSan in trap
mode); otherwise the old clang chain. `judge.Profiles` derives Full and Debug
from that choice, so samples, submit and debug always share one library.

| | before (`klykvvxv`) | after |
|---|---|---|
| package closure (`nix path-info -S`) | 2013 MiB | 1989 MiB |
| dev shell closure | 2314 MiB | 2266 MiB |
| `katharsis` binary | 28,394,000 B | 28,410,624 B |

The binary was already linked with `-s -w` (`flake.nix` ldflags); without
them it is 40,068,322 B. The closure changes are: clang 21, clang 21's lib
and LLVM 21's tools leave (432 MiB); GCC 14 and its lib join (~305 MiB).

## Findings

**A native GCC.** nixpkgs' `gcc13` to `gcc16` are all in the binary cache for
aarch64-darwin. `g++ -dumpmachine` is `arm64-apple-darwin`; binaries link
`.../gcc-14.4.0-lib/lib/libstdc++.6.dylib` (`otool -L`). The "exec format
error" a QA agent saw came from `gcc-wrapper-15.2.0` in the store, an
aarch64-linux ELF (`file` on it). The flake takes `pkgs.gcc14` from the host's
`legacyPackages`, so that cannot happen there.

**GCC 14, not 15.** Codeforces' newest g++ is 14.2 (`GNU G++23 14.2 (64 bit,
msys2)`, codeforces.com/blog/entry/121114; Kattis uses g++-14 14.2.0). GCC 15
accepts code that 14.2 rejects (`std::flat_map`). Reading speed is the same:
1e6 ints through cin took 67.1 ms (14.4) and 66.8 ms (15.3) side by side.

**What works under g++** (all checked by running):

- `<bits/stdc++.h>`, `__gnu_pbds` trees, `__int128` as integral under
  `gnu++23` (not under strict `c++17`, and `cout << __int128` is ambiguous,
  both as on Codeforces).
- `-Wl,-stack_size,0x20000000` through Apple's ld: a 1e6-deep DFS gets
  SIGSEGV on the default stack and passes with it, at -O2 and -O0.
- `-O0 -g` writes a dSYM; Apple lldb stops at breakpoints and prints
  `vector`, `map` and `string` (one DWARF warning on `std::string`'s
  `_M_local_buf`, values still shown), and stops on the division trap's line.
- VLAs of `vector` (`vector<int> adj[n]`) compile under g++ and under clang
  against libstdc++.
- Uncaught exceptions: "terminate called after throwing an instance of
  'std::out_of_range'" then "what(): ..."; the judge reports both.

**What does not.** GCC has no sanitizer runtimes for arm64 macOS:
`-fsanitize=address` fails with `ld: library not found for -lasan`,
`undefined` with `-lubsan`, `thread` with undefined `___tsan_*` symbols.
`-fsanitize=undefined -fsanitize-trap=undefined` works, without saying where.
And arm64 g++ rejects x86 target pragmas (`#pragma GCC target("avx2")`: "is not
valid") with an error that `-Wno-pragmas`, `-fpermissive` and `-Wno-error` all
leave in place; clang ignores them. The judge blanks those lines for g++
builds and adds `#line 1 "<abs path>"`, so diagnostics, debug info and lldb
breakpoints keep the original file and line numbers.

**cin against scanf**, 1e6 and 1e7 ints, sync off, hyperfine means, machine
load ~3 (GCC 15.3; GCC 14 reads at the same speed, above):

| | cin 1e6 | scanf 1e6 | cin 1e7 | scanf 1e7 |
|---|---|---|---|---|
| g++ / libstdc++ | 38 ms | 66 ms | 286 ms | 600 ms |
| clang 22 / libc++ (nix) | 967 ms | 59 ms | 9724 ms | 572 ms |
| Apple clang / libc++ | 999 ms | | | |
| clang 22 / libstdc++ | 37 ms | | 282 ms | |

The library decides it, not the compiler: clang against libstdc++ reads as
fast as g++. Under the judge itself (`TestCinSpeed`, CPU time, GCC 14):
cin 36 ms, scanf 60 ms. A rerun with GCC 14 at load ~16 kept the ratio
(minimums: cin 53 ms against 1695 ms).

**Compute.** libc++ is faster at sorting; the judge now times what
Codeforces' g++ runs, not the faster library. Means at load ~3 (GCC 15.3):

| | g++ / libstdc++ | clang / libc++ | clang / libstdc++ |
|---|---|---|---|
| O(n²) loop, n = 1e5 | 1300 ms min (noisy mean 1656) | 732 ms | 751 ms |
| `std::sort` 1e7 | 533 ms | 193 ms | 546 ms |
| `std::set` 2e5 ops | 73 ms | 50 ms | 67 ms |

GCC 14 at load ~16 (minimums): 1396 / 866 ms, 595 / 221 ms, 85 / 55 ms.

**Compile time** (hyperfine medians, load ~3, GCC 15.3). "typical" is a
15-header solution; "template" is the 35-header `~/cp/template.cpp` with a
Dijkstra body.

| | typical | template | `bits/stdc++.h` |
|---|---|---|---|
| clang O2 | 722 ms | 731 ms | |
| g++ O2 | 727 ms | 630 ms | 1415 ms |
| g++ O0 -g | 841 ms | 1427 ms (min 749) | 2592 ms |
| clang ASan, libc++ | 1385 ms | 1246 ms | |
| clang ASan, libstdc++ | 996 ms | 1089 ms | 2400 ms |

**Precompiled headers** (`internal/judge/pch.go`, 2026-10-14). Builds under
the sample and full profiles load a PCH of the `#include <...>` lines the
source opens with, once a background build has made one for that compiler,
flag set and header list. Compiler alone, the judge's exact flags, minimum
of 10 runs (hyperfine), load ~5; "template" is `~/cp/template.cpp`'s 35
includes with the same Dijkstra body as the bits source:

| | plain | own includes' PCH | `-include bits` PCH | PCH size |
|---|---|---|---|---|
| g++ O2, bits | 1230 ms | 491 ms | | 188 MB |
| g++ O2, template | 876 ms | 447 ms | 565 ms | 129 MB |
| g++ O0 -g, bits | 1370 ms | 629 ms | | 204 MB |
| g++ O0 -g, template | 980 ms | 545 ms | | 140 MB |
| clang ASan, bits | 1299 ms | 713 ms | | 25 MB |
| clang ASan, template | 1075 ms | 657 ms | 714 ms | 18 MB |
| g++ O2, 15 headers | 715 ms | 411 ms | | 85 MB |
| g++ O2, `<iostream>` | 413 ms | 222 ms | | 56 MB |
| g++ O2, `<cstdio>` | 128 ms | 122 ms | | 5 MB |

A PCH never lost, even for `<cstdio>`, and its size tracks what it saves.
Forcing `-include bits/stdc++.h` into a template build saves less than the
template's own list, and changes the program: names like `y1` (`<cmath>`)
or a global `count` collide with the library under `using namespace std`, a
missing `#include` passes here and fails on Codeforces, and a
`#define _GLIBCXX_DEBUG` or `#pragma GCC optimize` above the includes stops
reaching them. So the judge precompiles each source's own leading list and
forces nothing; the list ends at the first line that is not an include, a
comment or `#line` (clang builds also pass `#pragma GCC optimize`/`target`,
which clang ignores).

g++ passes over a `.gch` that does not match (`-Winvalid-pch` says why:
"not used because `__cplusplus' defined as ...", "settings for -fexceptions
do not match") and reads the headers, but a truncated one is a hard error
("cannot read PCH file"). clang refuses any mismatch with an error,
including a header whose mtime changed since. So a PCH is built aside,
checked by compiling an empty file against it (g++'s `-H` marks the one it
loads with `!`), and renamed into place; a build that names it in a warning
or fails over it drops it and, if it failed, compiles again without.

**Samples: clang ASan against libstdc++, with assertions.** GCC's trap-mode
UBSan misses the memory bugs, so ASan stays on clang. Against libstdc++,
ASan alone misses more than against libc++, because libstdc++ annotates
less; `_GLIBCXX_ASSERTIONS` closes the gap. Exit status and first report
line, `ASAN_OPTIONS` as in `judge.Run`:

| bug | libc++ ASan | libstdc++ ASan | + assertions | g++ trap UBSan |
|---|---|---|---|---|
| use after free | caught | caught | caught | missed (rc 0) |
| `v.data()[i]` past size, after `reserve` | caught | caught | caught | missed |
| `v[i]`, vector built by push_back | caught | missed | caught | caught (assert) |
| `s[i]` past size, string | missed | missed | caught | |
| `d[i]` past size, deque | caught | missed | caught | |
| `std::array` index in a struct | missed | missed | caught | |
| signed overflow | caught, with line | caught, with line | caught | trap, no line |

libstdc++'s vector annotations poison spare capacity in `reserve()`, but not
when push_back (`_M_realloc_append` in `bits/vector.tcc` has no
`_GLIBCXX_ASAN_ANNOTATE_REINIT`) or `resize` reallocates, and in-place growth
only unpoisons. Measured: `v.data()[3]` after `reserve(8)` and one push_back
is a container-overflow; `p[5]` after `vector v(3); v.resize(5)` (capacity 6)
reads clean. GCC 15's headers behave the same.

A libstdc++ assertion aborts without naming the caller, so the judge sets
`handle_abort=1` for libstdc++ builds: ASan prints the stack, and
`crashReason` takes the location from it and the words from the assertion,
e.g. `libstdc++: __n < this->size() at sol.cpp:8`.

The assertions also shrank the DFS frame under ASan from 112 to 96 bytes
(a 512 MB stack then holds 5.5e6 frames instead of 4.8e6). That is
codegen luck, not a design point.

## Packaging

On Darwin, nixpkgs builds `as`, `dsymutil` and the Apple SDK's tool links
from the default LLVM (21), so the clang 22 wrapper alone pulled clang 21
into the closure (its `as` runs `clang-21 -cc1as`). The flake overrides
`darwin.binutils-unwrapped` with LLVM 22 and rewrites the SDK's
`postInstall` onto it. That compiles nothing; `nix build` took 45 s.

GCC's fixincludes copied the stock SDK's headers into `include-fixed`, store
path and all, so stock GCC brings that SDK and LLVM 21 back.
`replaceDependencies` repoints those references (a copy, not a GCC build).

ld64 and cctools still link LLVM 21's libLTO (`llvm-21.1.8-lib`, 379 MiB).
Rebuilding them on LLVM 22 removes it (clang wrapper closure 1.6 GiB to
1.2 GiB) but compiles ld64 and cctools locally: 361 s, again on every nixpkgs
bump. Not done.

The judge tools (`katharsis-judge-tools`: `clang++`, `clang++-libstdc++`,
`g++`, `dsymutil`) are one store path, shared by the package's PATH wrapper
and the dev shell. The dev shell now uses `stdenvNoCC`: a cc stdenv exports
`NIX_CFLAGS_COMPILE` and friends, which every nix compiler wrapper reads, so
tests in the shell judged with flags the game never passes. GCC profiles also
run with `CleanEnv`, which drops `NIX_*` (a login shell here exports
`NIX_LDFLAGS`).

## Verification

- `go test ./...`, `go vet`, `golangci-lint`: green. New judge tests:
  `TestToolchain`, `TestGNUSources` (pb_ds, target pragmas and GNU-isms under
  the sample, submit and debug profiles), `TestTargetPragmaKeepsLines`,
  `TestCinSpeed`.
  `TestRun` covers both clang's and g++'s Full plus the trap fallback.
- `go test -tags qa ./qa/judge/...`: every failure in the baseline subtests
  is a baseline failure or a known timing flake. The QA adds the libstdc++
  sample profile; where it fails and the other ASan profiles pass, the
  reason is the assertion's wording: `re_heap_oob`, `re_container_oob` and
  `re_after_20KB_debug_stderr` expect "heap-buffer-overflow" or "overflow",
  and get `libstdc++: __n < this->size() at sol.cpp:N`. `gnu_pb_ds` and
  `dfs_vecvec` at depth 5e6 pass on it.
- The QA's profile list has clang's Full only. A temp copy with g++'s Full
  added failed on a subset of clang Full's failures, and passed `gnu_pb_ds`,
  which clang fails.
- `nix build` and `nix flake check --all-systems` pass.

## Open items

- `qa/judge` expectations name ASan's words for the sample profile; they
  need the libstdc++ assertion form, and g++'s Full in the profile list.
- `DESIGN.md` still says clang 22 judges (the stack table and the pipeline
  sketch), and `internal/app`'s built-in templates still say libc++ has no
  `<bits/stdc++.h>`.
- No PCH for `:Debug` (g++ -O0 -g: 140-204 MB more cache for the rarest
  build, 0.4-0.7 s each), nor for g++ sources that open with `#pragma GCC
  optimize`, which applies to the headers after it.
- The clang + libc++ fallback misses string and `std::array` indexing too;
  libc++'s hardening mode would catch them there.
