# Analysis: Do You Know Your ABCs?

*USACO 2020 December, Bronze, Problem 1*

(Analysis by Brian Dean and Benjamin Qi)

With some careful reasoning, we can deduce that the smallest two numbers must be $A$ and $B$, and the largest must be $A+B+C$. Subtracting the smallest two from the largest therefore gives us $C$ and we are done.

The only computational aspect of this problem therefore is locating the two smallest and the largest of the seven input numbers. One way to do this is by sorting the numbers, giving a very concise answer. E.g., in C++, this looks like:

```cpp
#include <iostream>
#include <algorithm>
using namespace std;

int main(void)
{
  int nums[7];
  for (int i=0; i<7; i++) cin >> nums[i];
  sort (nums, nums+7);
  int a = nums[0], b = nums[1];
  int c = nums[6] - a - b;
  cout << a << " " << b << " " << c << "\n";
}
```

In Python, a similarly-concise solution might be:

```python
nums = list(sorted(map(int,input().split())))
a,b = nums[0],nums[1]
c = nums[-1]-a-b
print(a,b,c)
```

Sorting isn't absolutely necessary here. One could also just scan the input to find the largest number, and then scan two more times to find the two smallest numbers (being careful to account for the possibility these might be the same value). Code for this is slightly longer but not too bad. Here's an example in C++:

```cpp
#include <iostream>
using namespace std;

int main(void)
{
  int nums[7], A, B, C;
  for (int i=0; i<7; i++) cin >> nums[i];
  
  int largest = nums[0];
  for (int i=1; i<7; i++) 
    if (nums[i] > largest) largest = nums[i];

  int smallest = nums[0], count_smallest = 1;
  for (int i=1; i<7; i++) {
    if (nums[i] == smallest) count_smallest++;
    if (nums[i] < smallest) { smallest = nums[i]; count_smallest = 1; }
  }

  if (count_smallest > 1) {
    A = B = smallest;
    C = largest - A - B;
  } else {
    int second_smallest = nums[0];
    if (second_smallest == smallest) second_smallest = nums[1];
    for (int i=1; i<7; i++) 
      if (nums[i] < second_smallest && nums[i] != smallest) 
	second_smallest = nums[i]; 
    A = smallest;
    B = second_smallest;
    C = largest - A - B;
  }
  
  cout << A << " " << B << " " << C << "\n";
}
```

---

Source: <https://usaco.org/current/data/sol_prob1_bronze_dec20.html>
