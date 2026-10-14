# Matrix Exponentiation

*Repeatedly multiplying a square matrix by itself.*

**Authors:** Benjamin Qi, Harshini Rayasam, Neo Wang, Peng Bai\
**Prerequisites:** intro-dp

**Resources**

- [5.9 - Powers of a Square Matrix](https://usaco.guide/CP2.pdf) (CP2)
- [23 - Matrices](https://usaco.guide/CPH.pdf) (CPH)
- ★ [Errichto - Matrix Exponentiation](https://codeforces.com/blog/entry/80195) (CF) — video + problemset
- [Cool tricks using Matrix Exponential](https://codeforces.com/blog/entry/43225) (CF) — interesting applications of mat exp
- [Matrix Power Applications](https://github.com/mostafa-saad/ArabicCompetitiveProgramming/blob/master/04%20Math/Algorithms_Number_Theory_06_Matrix_Power_Applications.pptx) (Mostafa) — powerpoint of matrix exponentiation

## Example - Fibonacci

> **Focus Problem**
>
> [Fibonacci Numbers](https://cses.fi/problemset/task/1722) · CSES · Easy

The Fibonacci numbers are defined as follows:

```math
F_0 = 0
F_1 = 1
F_(n+1) = F_n + F_(n-1)
```

We can also write them using the following matrix recurrence:

```math
A = [1 1; 1 0]^n=[F_(n+1) F_n; F_n F_(n-1)]
```

### Proof by Induction

Base case (n=1):

```math
A^1 = [1 1; 1 0] = [F_2 F_1; F_1 F_0]
```

Inductive step (n+1):

```math
A^(n+1)=A A^n=[1 1; 1 0][F_(n+1) F_n; F_n F_(n-1)]=[F_(n+1)+F_n F_n+F_(n-1); F_(n+1) F_n]=[F_(n+2) F_(n+1); F_(n+1) F_n]
```

With this, we've shown that the formula holds for all n ∈ N.

## Implementation

**Time Complexity:** O(log N)

```cpp
#include <bits/stdc++.h>
using namespace std;

using ll = long long;

const ll MOD = 1e9 + 7;

using Matrix = array<array<ll, 2>, 2>;

Matrix mul(Matrix a, Matrix b) {
	Matrix res = {{{0, 0}, {0, 0}}};
	for (int i = 0; i < 2; i++) {
		for (int j = 0; j < 2; j++) {
			for (int k = 0; k < 2; k++) {
				res[i][j] += a[i][k] * b[k][j];
				res[i][j] %= MOD;
			}
		}
	}

	return res;
}

int main() {
	ll n;
	cin >> n;

	Matrix base = {{{1, 0}, {0, 1}}};
	Matrix m = {{{1, 1}, {1, 0}}};

	for (; n > 0; n /= 2, m = mul(m, m)) {
		if (n & 1) base = mul(base, m);
	}

	cout << base[0][1];
}
```

## Finding the Right Matrix - Generalized Linear Recurrences

> **Focus Problem**
>
> [Recursive Sequence](https://www.spoj.com/problems/SEQ/) · SPOJ · Easy

We can generalize the technique above to calculate values in any linear recurrence. The tricky part is finding the correct matrix for the problem. Let's see how we can find this matrix M. We know that following equation must hold:

```math
[M_(1,1) M_(1, 2) …; ⋮ ⋱ ⋮; M_(k, 1) … M_(k, k)] [a_1; ⋮; a_k] = [a_2; ⋮; a_(k + 1)]
```

Here, we use the values of a\_1, …, a\_n to find a\_(k+1). We can also discard a\_1 since it won't be used to calculate a\_(k+2). If we think about the matrix multiplication, we notice that there is a diagonal of 1s shifted to the right by 1 since a\_i → a\_(i + 1) for i \< k. So now we have (using k = 5 as an example):

```math
M = [0 1 0 0 0; 0 0 1 0 0; 0 0 0 1 0; 0 0 0 0 1; M_(5, 1) M_(5, 2) M_(5, 3) M_(5, 4) M_(5, 5)]
```

To find the bottom row, we use the recurrence formula:
```math
M = [0 1 0 0 0; 0 0 1 0 0; 0 0 0 1 0; 0 0 0 0 1; c_5 c_4 c_3 c_2 c_1]
```

With this M, the equation always holds.

## Implementation

**Time Complexity:** O(K^3 log N)

```cpp
#include <bits/stdc++.h>
using namespace std;
typedef long long ll;

const int MOD = 1e9;

template <typename T> void matmul(vector<vector<T>> &a, vector<vector<T>> b) {
	int n = a.size(), m = a[0].size(), p = b[0].size();
	assert(m == b.size());
	vector<vector<T>> c(n, vector<T>(p));
	for (int i = 0; i < n; i++) {
		for (int j = 0; j < p; j++) {
			for (int k = 0; k < m; k++) {
				c[i][j] = (c[i][j] + a[i][k] * b[k][j]) % MOD;
			}
		}
	}

	a = c;
}

template <typename T> struct Matrix {
	vector<vector<T>> mat;
	Matrix() {}
	Matrix(vector<vector<T>> a) { mat = a; }
	Matrix(int n, int m) {
		mat.resize(n);
		for (int i = 0; i < n; i++) { mat[i].resize(m); }
	}

	int rows() const { return mat.size(); }
	int cols() const { return mat[0].size(); }

	// makes the identity matrix for a n by n matrix
	void makeiden() {
		for (int i = 0; i < rows(); i++) { mat[i][i] = 1; }
	}

	void print() const {
		for (int i = 0; i < rows(); i++) {
			for (int j = 0; j < cols(); j++) { cout << mat[i][j] << ' '; }
			cout << '\n';
		}
	}

	Matrix operator*=(const Matrix &b) {
		matmul(mat, b.mat);
		return *this;
	}

	Matrix operator*(const Matrix &b) { return Matrix(*this) *= b; }
};

int main() {
	int test_num;
	cin >> test_num;
	for (int t = 0; t < test_num; t++) {
		int n, k;
		cin >> k;
		Matrix<ll> mat(k, k), vec(k, 1), cur(k, k);
		cur.makeiden();
		for (int i = 0; i < k; i++) { cin >> vec.mat[i][0]; }

		for (int i = 0; i < k; i++) { cin >> mat.mat[k - 1][k - i - 1]; }

		for (int i = 1; i < k; i++) { mat.mat[i - 1][i] = 1; }

		cin >> n;
		n--;
		while (n > 0) {
			if (n & 1) cur *= mat;
			mat *= mat;
			n >>= 1;
		}

		Matrix<ll> res = cur * vec;
		cout << res.mat[0][0] << '\n';
	}
}
```

## Takeaway

The takeaway here is not the exact matrix used in linear recurrences, but the method in deriving it. The process of thinking about a vector before and after applying M, then deducing M through logic, is a technique that generalizes far beyond standard linear recurrences. For example, see if you can find the equation needed and correct matrix (using k = 5 as an example) to solve this modified recurrence for i \> k:

```math
a_i = c_1a_(i-1) + c_2a_(i-2) + … + c_ka_(i-k) + C
```

where C is a given constant.

**▸ Spoiler: Solution**

> ```math
> [0 1 0 0 0 0; 0 0 1 0 0 0; 0 0 0 1 0 0; 0 0 0 0 1 0; c_5 c_4 c_3 c_2 c_1 1; 0 0 0 0 0 1] [a_1; a_2; a_3; a_4; a_5; C] = [a_2; a_3; a_4; a_5; a_6; C]
> ```

## Problems

| Problem | Source | Difficulty |
| --- | --- | --- |
| [Graph Paths I](https://cses.fi/problemset/task/1723) | CSES | Easy |
| [Graph Paths II](https://cses.fi/problemset/task/1724) | CSES | Easy |
| [Xor-sequences](https://codeforces.com/contest/691/problem/E) | CF | Easy |
| [Sasha and Array](https://codeforces.com/problemset/problem/718/C) | CF | Medium |
| [2007 - Connected Points](https://cses.fi/112/list/) | Baltic OI | Medium |
| ★ [2009 - Reading](https://vjudge.net/problem/%E6%B4%9B%E8%B0%B7-P6841) | Balkan OI | Medium |
| [Ray Needs Help](https://dmoj.ca/problem/ray) | DMOJ | Medium |
| [COWBASIC](http://www.usaco.org/index.php?page=viewproblem2&cpid=746) | Platinum | Hard |

---

*“Matrix Exponentiation” by Benjamin Qi, Harshini Rayasam, Neo Wang, Peng Bai, USACO Guide (https://usaco.guide/plat/matrix-expo), CC BY-NC-SA 4.0 (https://creativecommons.org/licenses/by-nc-sa/4.0/); converted from MDX to Markdown.*
