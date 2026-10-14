# Casework

*Dividing into multiple cases and solving for each one separately.*

**Authors:** Aarav Sharma

A casework problem is one that can be broken down into different cases that can each be solved for separately.

These usually require drawing out a lot of cases and making observations about each.

We can try to spot similarities between different cases in order to solve
multiple of them with the same method.

> **Focus Problem**
>
> [Sleepy Cow Herding](https://usaco.org/index.php?page=viewproblem2&cpid=915) · Bronze · Medium

## Solution - Sleepy Cow Herding

**▸ Spoiler: Solution**

> Let's solve for the minimum and maximum numbers of moves independently. We refer to the cow positions as "elements," and say that two elements are adjacent if there
> are no elements in between them.
>
> ### Minimum
>
> There are 3 cases for the minimum amount of moves:
> 1. The 3 elements are already consecutive.
> 2. There are two adjacent elements with exactly one empty position in between them.
> 3. Any other case that did not satisfy the two above.
>
> For the first case, the answer would be 0 because the elements are already consecutive.
>
> For the second case, the answer would be 1 because the only move required is the one that would insert the isolated element into the gap between the two other elements.
>
> The third case would output 2 because for any other test case, the optimal solution would be to either take the minimum element to max-2 or the maximum element to min+2, and then we have reduced to the second case.
>
> ### Maximum
>
> The maximum will always be finite because every operation strictly decreases
> the distance between the endpoints (let's refer to this as the "span") by at least one.
> The best approach to maximize the amount of moves is to place each element as close to an endpoint as possible (while not remaining an endpoint). After one move,
> the span will equal the largest distance between two adjacent elements, and every
> subsequent move after that will decrease the span by one until it reaches two.
> Therefore, the maximum is the largest distance between two adjacent elements minus 1.
>
> ## Implementation
>
> **Time Complexity:** O(1)
>
> ```cpp
> #include <algorithm>
> #include <iostream>
> #include <vector>
>
> using namespace std;
>
> int main() {
> 	freopen("herding.in", "r", stdin);
> 	freopen("herding.out", "w", stdout);
>
> 	vector<int> a(3);
> 	for (int &b : a) { cin >> b; }
> 	sort(a.begin(), a.end());
>
> 	/*
> 	 * The minimum number of moves can only be 0, 1, or 2.
> 	 * 0 is if they're already consecutive,
> 	 * 1 is if there's a difference of 2 between any 2 numbers,
> 	 * and 2 is for all other cases.
> 	 */
> 	if (a[0] == a[2] - 2) {
> 		cout << 0 << endl;
> 	} else if ((a[1] == a[2] - 2) || (a[0] == a[1] - 2)) {
> 		cout << 1 << endl;
> 	} else {
> 		cout << 2 << endl;
> 	}
>
> 	// max is equal to largest difference between end and middle, minus one.
> 	cout << max(a[2] - a[1], a[1] - a[0]) - 1;
> }
> ```

## Problems

| Problem | Source | Difficulty |
| --- | --- | --- |
| [Three Logos](https://codeforces.com/problemset/problem/581/D) | CF | Easy |
| [Fence Painting](http://www.usaco.org/index.php?page=viewproblem2&cpid=567) | Bronze | Medium |
| [Social Distancing I](https://usaco.org/index.php?page=viewproblem2&cpid=1035) | Bronze | Medium |
| [Hoof Paper Scissors Minus One](https://usaco.org/index.php?page=viewproblem2&cpid=1515) | Bronze | Medium |
| [Leaders](https://usaco.org/index.php?page=viewproblem2&cpid=1275) | Bronze | Hard |
| [Spaced Out](http://www.usaco.org/index.php?page=viewproblem2&cpid=1088) | Silver | Very Hard |

---

*“Casework” by Aarav Sharma, USACO Guide (https://usaco.guide/bronze/casework), CC BY-NC-SA 4.0 (https://creativecommons.org/licenses/by-nc-sa/4.0/); converted from MDX to Markdown.*
