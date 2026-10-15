# Two Pointers

*Iterating two monotonic pointers across an array to search for a pair of indices satisfying some condition in linear time.*

**Authors:** Darren Yao, Qi Wang, Ryan Chou, David Zhou\
**Contributors:** Aadit Ambadkar, Juheon Rhee\
**Prerequisites:** intro-sorting

## Resources

**Resources**

- ★ [8.1 - Two Pointers](https://usaco.guide/CPH.pdf) (CPH) — solutions to the problems above
- [14.1 - Two Pointers](https://darrenyao.com/usacobook/cpp.pdf) (IUSACO) — above + mention of max subarray sum
- ★ [Two Pointers Method](https://codeforces.com/edu/course/2/lesson/9) (CF EDU) — video explanation of two pointers

[video: YouTube](https://www.youtube.com/watch?v=zmadiUuUeAA)

## Two Pointers

The **Two Pointers** method iterates two pointers across an array to track indices satisfying some condition. There are two common variations:
1. Two pointers starting at different ends of the array and moving towards each other.
2. Two pointers moving in the same direction at different speeds. This variation is known as the **Sliding Window** algorithm.

## Sum of Two Values

> **Focus Problem**
>
> [Sum of Two Values](https://cses.fi/problemset/task/1640) · CSES · Very Easy

### Solution - Sum of Two Values

The "Opposite Ends" method allows us to find the target pair in linear time if the array is sorted. Instead of checking every possible pair (which would take O(N^2) time), we use the sorted property to narrow down the search space in a single pass.

We want to find two indices i and j such that a\_i + a\_j = x.

We can start by sorting the array. Then, we can initialize a left pointer at the beginning of the array (l=0) and a right pointer at the end (r=N-1).

While l \< r:
1. If a\[l\] + a\[r\] = x, we have found the target sum.
2. If a\[l\] + a\[r\] \< x, the sum is too small. To increase the sum, we increment l.
3. If a\[l\] + a\[r\] \> x, the sum is too large. To decrease the sum, we decrement r.

Since the array is sorted, moving the left pointer to the right will never decrease the sum, and moving the right pointer to the left will never increase the sum.

### Implementation

**Time Complexity:** O(N log N)

```cpp
#include <algorithm>
#include <iostream>
#include <utility>
#include <vector>

using namespace std;

int main() {
	int n, x;
	cin >> n >> x;
	vector<pair<int, int>> nums(n);
	for (int i = 0; i < n; i++) {
		cin >> nums[i].first;
		nums[i].second = i;
	}
	sort(nums.begin(), nums.end());
	int l = 0, r = n - 1;
	while (l < r) {
		int sum = nums[l].first + nums[r].first;
		if (sum == x) {
			cout << nums[l].second + 1 << " " << nums[r].second + 1 << endl;
			return 0;
		} else if (sum < x) {
			l++;
		} else if (sum > x) {
			r--;
		}
	}
	cout << "IMPOSSIBLE" << endl;
}
```

## Sliding Window

The **Sliding Window** method is a variation of the two pointers technique where two pointers move in the same direction to maintain a specific range or "window" of elements. While standard two pointers often move towards each other, the sliding window technique is used to find a contiguous subarray that satisfies a condition.

> **Focus Problem**
>
> [Books](https://codeforces.com/contest/279/problem/B) · CF · Very Easy

### Solution - Books

We want to find the longest contiguous segment of books that can be read within t minutes.

To accomplish this, we can define left and right to represent the beginning and end of the segment. Both will start at the beginning of the array. These numbers can be thought of as pointers, hence the name "two pointers."

For every value of left in increasing order, let's increase right until the total time for the segment is maximized while being less than or equal to t.

ans will store the maximum value of right - left+1 (segment size) that we have encountered so far.

After incrementing left by one, the time used decreases, hence the right pointer never has to move leftwards. Thus:

**Since both pointers will move at most N times, the overall time complexity is O(N).**

As an example, consider the first case in the sample inputs:

| Left Pointer |  |  |  |  |
| --- | --- | --- | --- | --- |
| arr\[i\] | 3 | 1 | 2 | 1 |
| Right Pointer |  |  |  |  |

We can move the right pointer to index 1:

| Left Pointer |  |  |  |  |
| --- | --- | --- | --- | --- |
| arr\[i\] | 3 | 1 | 2 | 1 |
| Right Pointer |  |  |  |  |

The sum of the values in this range is 4, and there are 2 values. So, the current maximum segment length is ans=2. By incrementing the left pointer by 1, we can subtract 3 from the sum of values to get 1. The array then looks like:

| Left Pointer |  |  |  |  |
| --- | --- | --- | --- | --- |
| arr\[i\] | 3 | 1 | 2 | 1 |
| Right Pointer |  |  |  |  |

Now, we can move the right pointer to the end. This makes the sum of values 1+2+1=4 and the length of the segment equal to 3. So, ans is now 3.

| Left Pointer |  |  |  |  |
| --- | --- | --- | --- | --- |
| arr\[i\] | 3 | 1 | 2 | 1 |
| Right Pointer |  |  |  |  |

Since the right pointer has reached the end of the array, we are done at this point. This leaves us with ans=3.

Here's an animation of the above example:

[video](https://usaco.guide/animations/two_pointers.mp4)

### Implementation

**Time Complexity:** O(N)

```cpp
#include <bits/stdc++.h>
using namespace std;

int main() {
	ios::sync_with_stdio(false);
	cin.tie(nullptr);

	int N, T;
	cin >> N >> T;
	vector<int> A(N);
	for (int &a : A) cin >> a;

	int r = -1, sum = 0, ans = 0;
	// sum stores sum of A[l ... r inclusive]
	for (int l = 0; l < N; sum -= A[l++]) {
		while (r + 1 < N && sum + A[r + 1] <= T) sum += A[++r];
		ans = max(ans, r - l + 1);
	}
	cout << ans << "\n";
}
```

## Problems

| Problem | Source | Difficulty |
| --- | --- | --- |
| [Subarray Sums I](https://cses.fi/problemset/task/1660) | CSES | Very Easy |
| ★ [Sum of Three Values](https://cses.fi/problemset/task/1641) | CSES | Easy |
| [Paired Up](http://www.usaco.org/index.php?page=viewproblem2&cpid=738) | Silver | Easy |
| [Cellular Network](https://codeforces.com/contest/702/problem/C) | CF | Easy |
| [They Are Everywhere](https://codeforces.com/problemset/problem/701/C) | CF | Easy |
| [Quiz Master](https://codeforces.com/contest/1777/problem/C) | CF | Easy |
| ★ [Diamond Collector](http://www.usaco.org/index.php?page=viewproblem2&cpid=643) | Silver | Medium |
| [Sleepy Cow Herding](http://www.usaco.org/index.php?page=viewproblem2&cpid=918) | Silver | Medium |
| [An impassioned circulation of affection](https://codeforces.com/problemset/problem/814/C) | CF | Medium |
| [Cow Checkups](https://usaco.org/index.php?page=viewproblem2&cpid=1470) | Silver | Hard |
| [2010 - A Huge Tower](https://oj.uz/problem/view/CEOI10_tower) | CEOI | Hard |
| [MEX vs MED](https://codeforces.com/contest/1744/problem/F) | CF | Hard |

## Quiz

**Question 1.** What is the two pointers algorithm?

**(a)** Keeping track of two indices to find a subarray that satisfies some condition.

**(b)** Keeping track of two indices to find a pair of indices that satisfy some condition.

**(c)** A way to answer range sum queries in O(1) time.

**(d)** A divide and conquer algorithm that splits an array into two halves to efficiently find an element satisfying a certain condition.

**▸ Spoiler: Answer 1**

> **(a)** ✗ Incorrect. The indices don't necessarily have to represent a subarray, they could just represent elements in the array.
>
> **(b)** ✓ Correct.
>
> **(c)** ✗ Incorrect. You might be thinking of [Prefix Sums](https://usaco.guide/silver/prefix-sums), another silver topic.
>
> **(d)** ✗ Incorrect.

**Question 2.** What should go in place of `cond`?

```cpp
/*
 * find two elements at distinct positions in an array
 * which add up to a target sum
 */
sort(begin(nums), end(nums));
int l = 0;
int r = n - 1;
while (cond) {
	if (nums[l] + nums[r] == target) {
		// found pair
		break;
	} else if (nums[l] + nums[r] > target) {
		r--;
	} else {
		l++;
	}
}
```

**(a)** `nums[r] - nums[l] != 0`

**(b)** `l < r`

**(c)** `l < 0 || l >= n || r < 0 || r >= n`

**▸ Spoiler: Answer 2**

> **(a)** ✗ Incorrect. We already have an exit condition in the loop and we're looking for two elements which add up to a target sum.
>
> **(b)** ✓ Correct. When the pointers overlap, they're looking for another solution to test when it doesn't exist.
>
> **(c)** ✗ Incorrect. The pointers are moving towards the middle of the array.

**Question 3.** Which of the following isn't true?

**(a)** The time complexity of a standard two pointers algorithm is O(N) when not considering any other operations on the array.

**(b)** The space complexity of a standard two pointers algorithm is O(1) when not considering the space taken up by the array.

**(c)** The two pointers algorithm is only useful in a sorted array.

**▸ Spoiler: Answer 3**

> **(a)** ✗ Incorrect. Assuming that both pointers move one step at a time, both pointers would take a maximum of N steps.
>
> **(b)** ✗ Incorrect. Assuming that both pointers move one step at a time, there would be a constant space complexity.
>
> **(c)** ✓ Correct. The array doesn't necessarily have to be sorted, such as when looking for a valid subarray with two pointers.

---

*“Two Pointers” by Darren Yao, Qi Wang, Ryan Chou, David Zhou, USACO Guide (https://usaco.guide/silver/two-pointers), CC BY-NC-SA 4.0 (https://creativecommons.org/licenses/by-nc-sa/4.0/); converted from MDX to Markdown.*
