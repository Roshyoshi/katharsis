# Analysis: Contest Timing

*USACO 2011 November, Bronze, Problem 1*

**Solution Notes:** A key to making the problem easy to solve is to write a function that converts from (day, hour, minute) to a single integer that reflects an absolute count of number of minutes since some pre-determined starting point. In the sample C solution below, the function total\_mins() computes the total number of minutes elapsed since the beginning of the month. Using this function, it is now easy to compute the number of minutes in the difference of two dates - we first convert the two dates into integers, and then simply subtract!

```cpp

#include <stdio.h>

int total_mins(int d, int h, int m)
{
  return d * 24 * 60 + h * 60 + m;
}

int main(void)
{
  int d, h, m;
  
  freopen ("ctiming.in", "r", stdin);
  freopen ("ctiming.out", "w", stdout);

  scanf ("%d %d %d", &d, &h, &m);
  
  if (total_mins(d,h,m) < total_mins(11,11,11))
    printf ("-1\n");
  else
    printf ("%d\n", total_mins(d,h,m) - total_mins(11,11,11));

  return 0;
}
```

---

Source: <https://usaco.org/current/data/sol_ctiming.html>
