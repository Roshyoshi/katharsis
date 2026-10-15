# (Optional) Hashmaps

*Maintaining collections of distinct elements with hashing.*

**Authors:** Darren Yao, Benjamin Qi\
**Contributors:** Neo Wang, Nathan Gong\
**Prerequisites:** intro-sets, hashing

> **Warning**
>
> You can almost always use ordered sets and maps instead, but it's good to know that these exist.

**Resources**

- [4.4 - Sets & Maps](https://darrenyao.com/usacobook/cpp.pdf) (IUSACO) — module is based off this

## Hashing

**Hashing** refers to assigning a unique code to every variable/object which
allows insertions, deletions, and searches in O(1) time, albeit with
a high constant factor, as hashing requires a large constant number of
operations. However, as the name implies, elements are not ordered in any
meaningful way, so traversals of an unordered set will return elements in some
arbitrary order.

## Custom Hashing

There is no built in method for hashing pairs or vectors. Namely,
`unordered_set<vector<int>>` does not work. In this case, we can use an
[sorted map](https://usaco.guide/silver/intro-sorted-sets) (which supports all of the functions used
in the code above) or declare our own hash function.

**Resources**

- ★ [Hash Functions for C++ Unordered Containers](https://web.archive.org/web/20240524031508/https://marknelson.us/posts/2011/09/03/hash-functions-for-c-unordered-containers.html) (Mark Nelson) — How to create user-defined hash function for `unordered_map`.

The link provides an example of hashing pairs of strings, as well as other data
structures like pairs.

```cpp
#include <bits/stdc++.h>
using namespace std;

typedef pair<int, int> pi;
#define f first
#define s second

struct hashPi {
	size_t operator()(const pi &p) const { return p.f ^ p.s; }
};

int main() { unordered_map<pi, int, hashPi> um; }
```

```cpp
#include <bits/stdc++.h>
using namespace std;

typedef pair<int, int> pi;
#define f first
#define s second

namespace std {
template <> struct hash<pi> {
	size_t operator()(const pi &p) const { return p.f ^ p.s; }
};
}  // namespace std

int main() { unordered_map<pi, int> um; }
```

However, this hash function is quite bad; if we insert
(0,0), (1,1), (2,2) … then they will all be mapped to the same bucket (so
it would easily be hacked in a Codeforces contest).

A better way to hash pairs would be to use polynomial hashing with a randomized
base (as described in [this module](https://usaco.guide/gold/hashing)).

## Anti-Hash Tests

The built-in hashing algorithm for integers in C++ is vulnerable to pathological tests, causing abnormally slow runtimes. We describe the issue below and how to fix it. Java users are not affected (see [this](https://codeforces.com/blog/entry/62393?#comment-464875) comment).

> **Info: Should I worry about anti-hash tests in USACO?**
>
> No - historically, no USACO problem has included an anti-hash test. However, these sorts of tests often appear in Codeforces, especially in educational rounds, where open hacking is allowed.

**Resources**

- ★ [Blowing up Unordered Map](https://codeforces.com/blog/entry/62393) (CF) — Explanation of this problem and how to fix it.

**▸ Spoiler: Hack Case Generator**

> ```cpp
> // source: https://codeforces.com/blog/entry/62393
> #include <cassert>
> #include <ctime>
> #include <iostream>
> #include <unordered_map>
> #include <vector>
> using namespace std;
>
> const int N = 2e5;
>
> vector<int> gen_multiples(int x) {
> 	int cur = 1, rem = 1;
> 	vector<int> res;
> 	for (int i = 1; i <= N; i++) {
> 		assert(1 <= cur && cur <= 1e9);
> 		res.push_back(cur);
> 		cur += x;
> 		if (cur > 1e9) {
> 			++rem;
> 			cur = rem;
> 		}
> 	}
> 	return res;
> }
>
> double insert_numbers(vector<int> nums) {
> 	clock_t begin = clock();
> 	unordered_map<long long, int> m;
> 	for (int n : nums) m[n];
> 	return (double)(clock() - begin) / CLOCKS_PER_SEC;
> }
>
> int main() {
> 	const long prime_list[]{
> 	    107897ul, 116731ul, 126271ul, 136607ul, 147793ul, 159871ul, 172933ul,
> 	    187091ul, 202409ul, 218971ul, 236897ul, 256279ul, 277261ul, 299951ul,
> 	};  // https://github.com/gcc-mirror/gcc/blob/5bea0e90e58d971cf3e67f784a116d81a20b927a/libstdc%2B%2B-v3/src/shared/hashtable-aux.cc
> 	for (int prime : prime_list) {
> 		cerr << "testing " << prime << ": ";
> 		vector<int> multiples = gen_multiples(prime);
> 		double runtime = insert_numbers(multiples);
> 		cerr << runtime << "\n";
> 		if (runtime > 1) {
> 			// output hack case for https://cses.fi/problemset/task/1640/
> 			cout << N << " 1\n";
> 			for (int i = 0; i < N; ++i) {
> 				if (i) cout << " ";
> 				cout << multiples[i];
> 			}
> 			cout << "\n";
> 			exit(0);
> 		}
> 	}
> }
> ```

In order to make your `unordered_map` unhackable, the hash function H you select must have the following property: it is hard to find integers x and y such that x ≠ y but H(x) ≡ H(y) (mod P), where P is the prime modulus corresponding to the number of buckets in your unordered map. Note that:
 - The default hash function, H(x)=x, obviously does not satisfy this property, because you can simply choose x=0 and y=P.
 - If open hacking is allowed, then any deterministic hash function does not satisfy this property, because a hacker could generate a test case specifically to break your solution. You should introduce randomness to ensure that your hash function changes from run to run.

Here is a drop-in replacement for `unordered_map` that seems to work well in practice:

**Resources**

- [HashMap](https://github.com/bqi343/USACO/blob/master/Implementations/content/data-structures/STL%20(5)/HashMap.h) (Benq (from KACTL))

```cpp
struct chash {
	// any random-ish large odd number will do
	const uint64_t C = uint64_t(2e18 * PI) + 71;
	// random 32-bit number
	const uint32_t RANDOM = chrono::steady_clock::now().time_since_epoch().count();
	size_t operator()(uint64_t x) const {
		// see https://gcc.gnu.org/onlinedocs/gcc/Other-Builtins.html
		return __builtin_bswap64((x ^ RANDOM) * C);
	}
};
template <class K, class V> using cmap = unordered_map<K, V, chash>;
// example usage: cmap<int, int>
```

## A Faster HashMap in C++

In C++, it's usually faster to use `gp_hash_table<K, V>` instead of
`unordered_map<K, V>`. Read / writes are much faster than `unordered_map`.
The documentation is rather confusing, so I'll just summarize the most useful
functions here. If you need a replacement for `unordered_set<K>`, use
`gp_hash_table<K, null_type>`.

**Resources**

- ★ [Chilli - Order of magnitude faster hash tables](https://codeforces.com/blog/entry/60737) (CF) — Introduces gp_hash_table
- [gp\_hash\_table Interface](https://gcc.gnu.org/onlinedocs/libstdc++/manual/policy_data_structures_design.html#container.hash.interface) (GCC) — documentation
- ★ [HashMap](https://github.com/bqi343/USACO/blob/master/Implementations/content/data-structures/STL%20(5)/HashMap.h) (Benq (from KACTL))

### Custom Hashing

Like `unordered_map`, `gp_hash_table` is vulnerable to hash collisions if a
custom hash function isn't used. We recommend using the same custom hash
we used for unordered map above:

```cpp
#include <ext/pb_ds/assoc_container.hpp>
using namespace __gnu_pbds;

gp_hash_table<int, int, chash> table;
```

### Resizing

Unordered map has
[`reserve`](http://www.cplusplus.com/reference/unordered_map/unordered_map/reserve/).
Calling this function before inserting any elements can result in a constant
factor speedup.

We can modify the declaration of `gp_hash_table` so that it supports the
`resize` function, which operates similarly.

```cpp
template <class K, class V>
using ht =
    gp_hash_table<K, V, hash<K>, equal_to<K>, direct_mask_range_hashing<>,
                  linear_probe_fn<>,
                  hash_standard_resize_policy<hash_exponential_size_policy<>,
                                              hash_load_check_resize_trigger<>, true>>;
```

These are the same template arguments as the default `gp_hash_table`, except
`false` has been changed to `true`. This modification allows us to change the
actual size of the hash table.

```cpp
int main() {
	ht<int, null_type> g;
	g.resize(5);
	cout << g.get_actual_size() << "\n";  // 8
	cout << g.size() << "\n";             // 0
}
```

When calling `g.resize(x)`, `x` is rounded up to the nearest power of 2. Then
the actual size of `g` is changed to be equal to `x` (unless `x < g.size()`, in
which case an error is thrown).

**Resources**

- [Resize Policy](https://gcc.gnu.org/onlinedocs/libstdc++/manual/policy_data_structures_design.html#container.hash.details.resize_policies) (GCC) — documentation

Furthermore, if we construct `g` with the following arguments:

```cpp
ht<int, null_type> g({}, {}, {}, {}, {1 << 16});
```

then the actual size of `g` is always at least `1<<16` (regardless of calls to
`resize`). The last argument **must** be a power of 2 (or else errors will be
thrown).

### Solving 3SUM

> **Focus Problem**
>
> [Farmer John Solves 3SUM](http://www.usaco.org/index.php?page=viewproblem2&cpid=994) · Gold · Medium

Since all the values are quite small, you can use an array instead of a hashmap.
But if you didn't read the constraints carefully enough, you're in luck!

> **Warning**
>
> This code passes in 1180ms when initial capacity is specified, and TLEs
> when it is not.

```cpp
#include <bits/stdc++.h>  // see C++ Tips & Tricks
using namespace std;

using ll = long long;

using vi = vector<int>;
#define pb push_back
#define rsz resize
#define all(x) begin(x), end(x)
#define sz(x) (int)(x).size()

using pi = pair<int, int>;
#define f first
#define s second
#define mp make_pair

void setIO(string name = "") {  // name is nonempty for USACO file I/O
	ios_base::sync_with_stdio(0);
	cin.tie(0);  // see Fast Input & Output
	// alternatively, cin.tie(0)->sync_with_stdio(0);
	if (sz(name)) {
		freopen((name + ".in").c_str(), "r", stdin);  // see Input & Output
		freopen((name + ".out").c_str(), "w", stdout);
	}
}

#include <ext/pb_ds/assoc_container.hpp>
using namespace __gnu_pbds;

int N, Q;
long long ans[5000][5000];
vector<int> A;

int main() {
	setIO("threesum");
	cin >> N >> Q;
	A.resize(N);
	for (int i = 0; i < N; ++i) cin >> A[i];
	for (int i = 0; i < N; ++i) {
		gp_hash_table<int, int> g({}, {}, {}, {}, {1 << 13});
		// initialize with certain capacity, must be power of 2
		for (int j = i + 1; j < N; ++j) {
			int res = -A[i] - A[j];
			auto it = g.find(res);
			if (it != end(g)) ans[i][j] = it->second;
			g[A[j]]++;
		}
	}
	for (int i = N - 1; i >= 0; --i)
		for (int j = i + 1; j < N; ++j)
			ans[i][j] += ans[i + 1][j] + ans[i][j - 1] - ans[i + 1][j - 1];
	for (int i = 0; i < Q; ++i) {
		int a, b;
		cin >> a >> b;
		cout << ans[a - 1][b - 1] << "\n";
	}
}
```

## Problems

| Problem | Source | Difficulty |
| --- | --- | --- |
| [Sum of Four Values](https://cses.fi/problemset/task/1642) | CSES | Medium |

---

*“(Optional) Hashmaps” by Darren Yao, Benjamin Qi, USACO Guide (https://usaco.guide/gold/hashmaps), CC BY-NC-SA 4.0 (https://creativecommons.org/licenses/by-nc-sa/4.0/); converted from MDX to Markdown.*
