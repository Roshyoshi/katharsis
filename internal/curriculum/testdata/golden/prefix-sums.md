# Introduction to Prefix Sums

*Computing range sum queries in constant time over a fixed 1D array.*

**Authors:** Darren Yao, Dustin Miao

## Resources

**Resources**

- [11 - Prefix Sums](https://darrenyao.com/usacobook/cpp.pdf) (IUSACO) — Source of this module
- [9.1 - Sum Queries](https://usaco.guide/CPH.pdf) (CPH) — Brief description
- [11.2.1 - Prefix Precomputation](https://usaco.guide/PAPS.pdf) (PAPS) — Motivation and implementation

[video: YouTube](https://www.youtube.com/watch?v=f0bfiuDjq9A)

## Introduction

Prefix sums are a technique used to quickly calculate the sum of any subarray.
By precomputing cumulative sums in O(N) time, subsequent queries can be computed in O(1) time.
This technique can be generalized to invertible operations,
such as [bitwise XOR](https://usaco.guide/silver/intro-bitwise#xor-operation).

## Example - Static Range Sum

> **Focus Problem**
>
> [Static Range Sum](https://judge.yosupo.jp/problem/static_range_sum) · YS · Very Easy

## Explanation

Let's say we have a one-indexed integer array arr of size N and we
want to compute the value of

```math
arr[a]+arr[a+1]+…+arr[b]
```

for Q different pairs (a,b) satisfying 1 ≤ a ≤ b ≤ N. We'll use the
following example with N = 6:

| Index i         | 1   | 2   | 3   | 4   | 5   | 6   |
| ----------------- | --- | --- | --- | --- | --- | --- |
| arr\[i\] | 1   | 6   | 4   | 2   | 5   | 3   |

Naively, for every query, we can iterate through all entries from index a to
index b to add them up. Since we have Q queries and each query requires a
maximum of O(N) operations to calculate the sum, our total time
complexity is O(NQ). For most problems of this nature, the
constraints will be N, Q ≤ 10^5, so NQ is on the order of 10^10. This
is **not** acceptable; it will almost certainly exceed the time limit.

Instead, we can use prefix sums to process these array sum queries. We designate
a prefix sum array prefix. First, because we're 1-indexing the array,
set prefix\[0\]=0, then for indices k such that 1 ≤ k ≤ N,
define the prefix sum array as follows:

```math
prefix[k]=Σ_(i=1)^k arr[i]
```

Basically, what this means is that the element at index k of the prefix sum
array stores the sum of all the elements in the original array from index 1 up
to k. This can be calculated easily in O(N) by the following
formula for each 1 ≤ k ≤ N:

```math
prefix[k]=prefix[k-1]+arr[k]
```

For the example case, our prefix sum array looks like this:

| Index i            | 0   | 1   | 2   | 3   | 4   | 5   | 6   |
| -------------------- | --- | --- | --- | --- | --- | --- | --- |
| prefix\[i\] | 0   | 1   | 7   | 11  | 13  | 18  | 21  |

Now, when we want to query for the sum of the elements of arr between
(1-indexed) indices a and b inclusive, we can use the following formula:

```math
Σ_(i=L)^R arr[i] = Σ_(i=1)^R arr[i] - Σ_(i=1)^(L-1) arr[i]
```

Using our definition of the elements in the prefix sum array, we have

```math
Σ_(i=L)^R arr[i]= prefix[R]-prefix[L-1]
```

Since we are only querying two elements in the prefix sum array, we can
calculate subarray sums in O(1) per query, which is much better than
the O(N) per query that we had before. Now, after an
O(N) preprocessing to calculate the prefix sum array, each of the
Q queries takes O(1) time. Thus, our total time complexity is
O(N+Q), which should now pass the time limit.

Let's do an example query and find the subarray sum between indices a = 2 and
b = 5, inclusive, in the 1-indexed arr. From looking at the
original array, we see that this is

```math
Σ_(i=2)^5 arr[i] = 6 + 4 + 2 + 5 = 17.
```

| Index i | 1 | 2 | 3 | 4 | 5 | 6 |
| --- | --- | --- | --- | --- | --- | --- |
| arr\[i\] | 1 | 6 | 4 | 2 | 5 | 3 |

Using prefix sums:

```math
prefix[5] - prefix[1] = 18 - 1 = 17.
```

| Index i | 0 | 1 | 2 | 3 | 4 | 5 | 6 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| prefix\[i\] | 0 | 1 | 7 | 11 | 13 | 18 | 21 |

These are also known as
[partial sums](https://mathworld.wolfram.com/PartialSum.html).

## Implementation
**Time Complexity:** O(N + Q)

In C++ we can use
[`std::partial_sum`](http://www.cplusplus.com/reference/numeric/partial_sum/),
although it doesn't shorten the code by much.

```cpp
#include <bits/stdc++.h>
using namespace std;

vector<long long> psum(const vector<int> &arr) {
	vector<long long> psums(arr.size() + 1);
	for (int i = 0; i < arr.size(); i++) { psums[i + 1] = psums[i] + arr[i]; }
	// or partial_sum(begin(a), end(a), begin(psums) + 1);
	return psums;
}

int main() {
	int N, Q;
	cin >> N >> Q;
	vector<int> nums(N);
	for (int i = 0; i < N; i++) { cin >> nums[i]; }
	vector<long long> prefix_arr = psum(nums);
	for (int i = 0; i < Q; ++i) {
		int l, r;
		cin >> l >> r;
		cout << prefix_arr[r] - prefix_arr[l] << "\n";
	}
}
```

## Problems

| Problem | Source | Difficulty |
| --- | --- | --- |
| [Breed Counting](http://www.usaco.org/index.php?page=viewproblem2&cpid=572) | Silver | Very Easy |
| [Hoof, Paper, Scissors](http://www.usaco.org/index.php?page=viewproblem2&cpid=691) | Silver | Very Easy |
| [GCD on Blackboard](https://atcoder.jp/contests/abc125/tasks/abc125_c) | AC | Easy |
| ★ [Good Subarrays](https://codeforces.com/contest/1398/problem/C) | CF | Easy |
| [Subarray Divisibility](https://cses.fi/problemset/task/1662) | CSES | Easy |
| [Subarray Sums II](https://cses.fi/problemset/task/1661) | CSES | Easy |
| [Subsequences Summing to Sevens](http://www.usaco.org/index.php?page=viewproblem2&cpid=595) | Silver | Easy |
| [Alternating String](https://codeforces.com/contest/2008/problem/E) | CF | Medium |
| [Multiple of 2019](https://atcoder.jp/contests/abc164/tasks/abc164_d) | AC | Medium |
| ★ [Running Miles](https://codeforces.com/contest/1826/problem/D) | CF | Medium |
| ★ [Farmer John's Favorite Operation](https://usaco.org/index.php?page=viewproblem2&cpid=1471) | Silver | Medium |
| [Irreducible Anagrams](https://codeforces.com/contest/1291/problem/D) | CF | Hard |
| [Sum of Segments](https://codeforces.com/contest/2026/problem/D) | CF | Hard |

## Quiz

**Question 1.** What is the optimal time complexity of calculating the prefix sum array of some array of length n?

**(a)** O(n)

**(b)** O(n^2)

**(c)** O(1)

**(d)** O(n log n)

**▸ Spoiler: Answer 1**

> **(a)** ✓ Correct! You need to iterate through all of the indices, but we can use the result of previously calculated sums to speed this up.
>
> **(b)** ✗ Incorrect. Although this is the complexity that naively computing the prefix sum array will achieve, we can speed it up by employing a technique known as dynamic programming.
>
> **(c)** ✗ Incorrect. Remember that we have to iterate through all of the indices of the array, so the time complexity is at least n.
>
> **(d)** ✗ Incorrect. There's no need to sort and/or employ any other technique that may add a log factor to the complexity.

**Question 2.** You are given the following array: A = \[2, 5, 7, 1, 4\]. What is the prefix sum array of A?

**(a)** P = \[2, 3, 2, -6, 3\]

**(b)** P = \[2, 7, 14, 15, 19\]

**(c)** P = \[0, 7, 12, 8, 5\]

**(d)** P = \[19, 17, 12, 5, 4\]

**▸ Spoiler: Answer 2**

> **(a)** ✗ Incorrect. This is known as the difference array, where the value of each index is determined by subtracting the two elements in A. This is the inverse operation of the prefix sum array, meaning that taking the prefix sum of the difference array or vice versa will result in the original array A.
>
> **(b)** ✓ Correct! The value at each index is equal to the sum of the values in A before and at the current index.
>
> **(c)** ✗ Incorrect. Recall that the prefix sum array is the sum of **all** previous element, not just the element directly before it.
>
> **(d)** ✗ Incorrect. This is the "suffix sum" array, which is the prefix sum array computed in reverse. In general, it serves the same purpose as its counterpart so we rarely use it.

**Question 3.** We are given the prefix sum array P = \[2, 7, 14, 15, 19\]. Find the sum of values _of the original array_ for the ranges of indexes \[1, 4\] and \[2, 5\] (inclusive, 1-indexed).

**(a)** 13; 12

**(b)** 17; 26

**(c)** 15; 17

**(d)** 15; 21

**▸ Spoiler: Answer 3**

> **(a)** ✗ Incorrect. Recall that to get the sum of some range of values \[l, r\], we need to calculate P\[r\] - P\[l - 1\] instead of P\[r\] - P\[l\]. In the second formula, we do not include the value at index l.
>
> **(b)** ✗ Incorrect. To get the sum of some range of values \[l, r\], we need to calculate P\[r\] - P\[l - 1\] instead of P\[r\] + P\[l\]. .
>
> **(c)** ✓ Correct! The sum of the values of some subarray \[l, r\] is determined by the formula P\[r\] - P\[l - 1\].
>
> **(d)** ✗ Incorrect. Recall that to get the sum of some range of values \[l, r\], we need to calculate P\[r\] - P\[l - 1\] instead of P\[r\] + P\[l - 1\].

**Question 4.** **Challenge Problem**: With the idea of prefix sums in mind, at first glance it may seem logical to apply this method to return range products in constant time. Instead of adding, we multiply, and instead of subtracting, we divide. Assume that we calculate the answer [modulo](https://usaco.guide/gold/modular) a given M (2 ≤ M ≤ 10^9) and the array has normal constraints (N ≤ 10^5, 1 ≤ a\_i \< M). What additional challenges may we face?

**(a)** **Integer Overflow**: Even if we use modulo, the numbers will grow too quickly when multiplying, to the point where they may not fit within a 64-bit integer.

**(b)** **Time Limit**: Multiplying, dividing, and modulo are relatively slow operations, and this will make the prefix product just as fast as iterating through the array.

**(c)** **Memory Limit**: When we switch to products, the memory goes from O(n) to O(n^2), which will result in MLE for n ≥ 10^5.

**(d)** **Division under Modulo**: We cannot directly use division as a substitute for subtraction, because division is not distributive under modulo.

**▸ Spoiler: Answer 4**

> **(a)** ✗ Incorrect. We can calculate intermediate modulos, which will mitigate this problem. This property is a result of the distribute property of modulo which states that (a · b) mod m = (a mod m) · (b mod m) mod m.
>
> **(b)** ✗ Incorrect. While it is true that multiplying and dividing are relatively slow, this difference is hardly enough to impact the runtime to the extent of causing TLE.
>
> **(c)** ✗ Incorrect. The memory remains the same regardless of whether we use sum or product.
>
> **(d)** ✓ Correct! Division under modulo is not the same as addition, subtraction, or multiplication. To divide involves multiplying by the modulo inverse, which only exists if M and the value to be divided by are relatively prime. One way we could conceivably do this is if M is prime itself.

---

*“Introduction to Prefix Sums” by Darren Yao, Dustin Miao, USACO Guide (https://usaco.guide/silver/prefix-sums), CC BY-NC-SA 4.0 (https://creativecommons.org/licenses/by-nc-sa/4.0/); converted from MDX to Markdown.*
