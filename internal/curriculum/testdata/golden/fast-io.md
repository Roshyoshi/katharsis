# Fast Input & Output

*Demonstrates how I/O speed can be the difference between TLE and AC.*

**Authors:** Benjamin Qi, Nathan Chen\
**Prerequisites:** input-output

The [USACO Instructions Page](http://www.usaco.org/index.php?page=instructions)
briefly mentions some ways of speeding up I/O:

> For some of the more advanced problems with larger input sizes, competitors may benefit from using fast input/output, to more easily pass within the time limit. For C++ users, you may want to add "ios_base::sync_with_stdio(false); cin.tie(0);" to the top of your main method if you are using cin/cout. For Java users, you may want to use BufferedReader instead of Scanner.

What do these do and how much of a difference do these actually make? We'll use the following
task to benchmark I/O speed:

## Example Task

The input consists of two integers M (0 ≤ M ≤ 1) and N
(1 ≤ N ≤ 10^6), followed by a sequence of N non-negative integers each
less than 10^9+7.

- If M=0, output the sum of the input sequence modulo 10^9+7.
- If M=1, output the sum of each prefix of the input sequence modulo 10^9+7.

**Sample Input 1:**

```
1 6
1
2
3
4
5
1000000000
```

**Sample Output 1:**

```
1
3
6
10
15
8
```

**Sample Input 2:**

```
0 6
1
2
3
4
5
1000000000
```

**Sample Output 2:**

```
8
```

Randomly generating test data results in input and output files each ~10MB
large. It is possible to see input files this large (the 11th input file for
[Robotic Cow Herd](http://www.usaco.org/index.php?page=viewproblem2&cpid=674) is
~10.3MB large), though not output files (the largest we know of is due to
[Minimum Cost Paths](http://usaco.org/index.php?page=viewproblem2&cpid=1093),
which has an output file ~2.8MB large).

> **Focus Problem**
>
> [Robotic Cow Herd](http://www.usaco.org/index.php?page=viewproblem2&cpid=674) · Platinum · Insane

## Standard I/O

### Slow

Some simple methods of I/O don't come close to running under the time limit:

**▸ Spoiler: cin/cout + endl (5.8s)**

> ```cpp
> #include <bits/stdc++.h>
> using namespace std;
>
> const int MOD = 1e9 + 7;
>
> int main() {
> 	int M, N;
> 	cin >> M >> N;
> 	int ans = 0;
> 	for (int i = 0; i < N; i++) {
> 		int x;
> 		cin >> x;
> 		ans = (ans + x) % MOD;
> 		if (M == 1) { cout << ans << endl; }
> 	}
> 	if (M == 0) { cout << ans << endl; }
> }
> ```

### Fast

#### `cin`/`cout`

If using `cin` and `cout`, include the following two lines.

```cpp
ios::sync_with_stdio(false);
cin.tie(nullptr);
```

Brief explanation:

- If you include `ios::sync_with_stdio(false)`, then mixing C (`scanf`,
  `printf`) and C++ (`cin`, `cout`) style I/O may produce unexpected results.
  The upside is that both `cin` / `cout` become faster.
- Including `cin.tie(nullptr)` will reduce the runtime if you are interleaving
  `cin` and `cout` (as is the case in the task at hand).

You can find more information about these lines at the end of this module.

**▸ Spoiler: cin/cout + unsync + \\n (0.41s)**

> ```cpp
> #include <bits/stdc++.h>
> using namespace std;
>
> const int MOD = 1e9 + 7;
>
> int main() {
> 	ios::sync_with_stdio(false);
> 	cin.tie(nullptr);
>
> 	int M, N;
> 	cin >> M >> N;
> 	int ans = 0;
> 	for (int i = 0; i < N; i++) {
> 		int x;
> 		cin >> x;
> 		ans = (ans + x) % MOD;
> 		if (M == 1) { cout << ans << "\n"; }
> 	}
> 	if (M == 0) { cout << ans << "\n"; }
> }
> ```

> **Warning**
>
> Using `endl` instead of `"\n"` will flush the output buffer and cause the above
> method to be quite slow:
>
> **▸ Spoiler: cin/cout + unsync + endl (5.0s)**
>
> > ```cpp
> > #include <bits/stdc++.h>
> > using namespace std;
> >
> > const int MOD = 1e9 + 7;
> >
> > int main() {
> > 	ios::sync_with_stdio(false);
> > 	cin.tie(nullptr);
> >
> > 	int M, N;
> > 	cin >> M >> N;
> > 	int ans = 0;
> > 	for (int i = 0; i < N; i++) {
> > 		int x;
> > 		cin >> x;
> > 		ans = (ans + x) % MOD;
> > 		if (M == 1) { cout << ans << endl; }
> > 	}
> > 	if (M == 0) { cout << ans << endl; }
> > }
> > ```
>
> Though for [interactive problems](https://codeforces.com/blog/entry/45307), you
> _need_ to flush the output buffer every time you use `cout`. Any one of the
> following will have the same effect:
>
> 1.  Not including `cin.tie(nullptr)`
> 2.  Writing `cout << endl` instead of `cout << "\n"`
> 3.  Writing `cout << "\n" << flush` instead of `cout << "\n"`

#### `scanf`/`printf`

**▸ Spoiler: scanf/printf (0.52s)**

> ```cpp
> #include <bits/stdc++.h>
> using namespace std;
>
> const int MOD = 1e9 + 7;
>
> int main() {
> 	int M, N;
> 	scanf("%d%d", &M, &N);
>
> 	int ans = 0;
> 	for (int i = 0; i < N; i++) {
> 		int x;
> 		scanf("%d", &x);
> 		ans = (ans + x) % MOD;
> 		if (M == 1) { printf("%d\n", ans); }
> 	}
> 	if (M == 0) { printf("%d\n", ans); }
> }
> ```

## File I/O

Pretty similar to standard I/O.

### Slow

**▸ Spoiler: freopen + cin/cout (5.7s)**

> ```cpp
> #include <bits/stdc++.h>
> using namespace std;
>
> const int MOD = 1e9 + 7;
>
> int main() {
> 	freopen("speed.in", "r", stdin);
> 	freopen("speed.out", "w", stdout);
>
> 	int M, N;
> 	cin >> M >> N;
> 	int ans = 0;
> 	for (int i = 0; i < N; i++) {
> 		int x;
> 		cin >> x;
> 		ans = (ans + x) % MOD;
> 		if (M == 1) { cout << ans << "\n"; }
> 	}
> 	if (M == 0) { cout << ans << "\n"; }
> }
> ```

### Fast

**▸ Spoiler: freopen + cin/cout + unsync + \\n (0.42s)**

> ```cpp
> #include <bits/stdc++.h>
> using namespace std;
>
> const int MOD = 1e9 + 7;
>
> int main() {
> 	ios::sync_with_stdio(false);
> 	cin.tie(nullptr);
>
> 	freopen("speed.in", "r", stdin);
> 	freopen("speed.out", "w", stdout);
>
> 	int M, N;
> 	cin >> M >> N;
> 	int ans = 0;
> 	for (int i = 0; i < N; i++) {
> 		int x;
> 		cin >> x;
> 		ans = (ans + x) % MOD;
> 		if (M == 1) { cout << ans << "\n"; }
> 	}
> 	if (M == 0) { cout << ans << "\n"; }
> }
> ```

**▸ Spoiler: freopen + scanf/printf (0.52s)**

> ```cpp
> #include <bits/stdc++.h>
> using namespace std;
>
> const int MOD = 1e9 + 7;
>
> int main() {
> 	freopen("speed.in", "r", stdin);
> 	freopen("speed.out", "w", stdout);
> 	int M, N;
> 	scanf("%d%d", &M, &N);
> 	int ans = 0;
> 	for (int i = 0; i < N; i++) {
> 		int x;
> 		scanf("%d", &x);
> 		ans = (ans + x) % MOD;
> 		if (M == 1) { printf("%d\n", ans); }
> 	}
> 	if (M == 0) { printf("%d\n", ans); }
> }
> ```

**▸ Spoiler: ifstream/ofstream (0.43s)**

> ```cpp
> #include <bits/stdc++.h>
> using namespace std;
>
> const int MOD = 1e9 + 7;
>
> int main() {
> 	ifstream fin("speed.in");
> 	ofstream fout("speed.out");
> 	int M, N;
> 	fin >> M >> N;
> 	int ans = 0;
> 	for (int i = 0; i < N; ++i) {
> 		int x;
> 		fin >> x;
> 		ans = (ans + x) % MOD;
> 		if (M) fout << ans << "\n";
> 	}
> 	if (!M) fout << ans << "\n";
> }
> ```

## Even Faster Methods

The input methods described above are easy to type up from scratch and are
usually fast enough for USACO contests. But if you're looking for something even
faster ...

Using `fread` and `fwrite` reduces the runtime even further.

**▸ Spoiler: fread/fwrite (0.17s)**

> ```cpp
> #include <bits/stdc++.h>
> using namespace std;
>
> const int MOD = 1e9 + 7;
> const int BUF_SZ = 1 << 15;
>
> // BeginCodeSnip{Input}
> inline namespace Input {
> char buf[BUF_SZ];
> int pos;
> int len;
> char next_char() {
> 	if (pos == len) {
> 		pos = 0;
> 		len = (int)fread(buf, 1, BUF_SZ, stdin);
> 		if (!len) { return EOF; }
> 	}
> 	return buf[pos++];
> }
>
> int read_int() {
> 	int x;
> 	char ch;
> 	int sgn = 1;
> 	while (!isdigit(ch = next_char())) {
> 		if (ch == '-') { sgn *= -1; }
> 	}
> 	x = ch - '0';
> 	while (isdigit(ch = next_char())) { x = x * 10 + (ch - '0'); }
> 	return x * sgn;
> }
> }  // namespace Input
> // EndCodeSnip
> // BeginCodeSnip{Output}
> inline namespace Output {
> char buf[BUF_SZ];
> int pos;
>
> void flush_out() {
> 	fwrite(buf, 1, pos, stdout);
> 	pos = 0;
> }
>
> void write_char(char c) {
> 	if (pos == BUF_SZ) { flush_out(); }
> 	buf[pos++] = c;
> }
>
> void write_int(int x) {
> 	static char num_buf[100];
> 	if (x < 0) {
> 		write_char('-');
> 		x *= -1;
> 	}
> 	int len = 0;
> 	for (; x >= 10; x /= 10) { num_buf[len++] = (char)('0' + (x % 10)); }
> 	write_char((char)('0' + x));
> 	while (len) { write_char(num_buf[--len]); }
> 	write_char('\n');
> }
>
> // auto-flush output when program exits
> void init_output() { assert(atexit(flush_out) == 0); }
> }  // namespace Output
> // EndCodeSnip
>
> int main() {
> 	init_output();
> 	int M = read_int();
> 	int N = read_int();
> 	int ans = 0;
> 	for (int i = 0; i < N; i++) {
> 		ans = (ans + read_int()) % MOD;
> 		if (M == 1) { write_int(ans); }
> 	}
> 	if (M == 0) { write_int(ans); }
> }
> ```

## Additional Notes

**Resources**

- ★ [Yet again on C++ I/O](https://codeforces.com/blog/entry/5217) (CF) — timing various I/O methods

### `ios::sync_with_stdio(false)`

**Resources**

- [ios\_base::sync\_with\_stdio](http://www.cplusplus.com/reference/ios/ios_base/sync_with_stdio/) (CPP) — documentation
- ★ [Significance of ios\_base::sync\_with\_stdio(false);](https://stackoverflow.com/questions/31162367/significance-of-ios-basesync-with-stdiofalse-cin-tienull) (SO)

From the second resource:

> This disables the synchronization between the C and C++ standard streams. By
> default, all standard streams are synchronized, which in practice allows you
> to mix C- and C++-style I/O and get sensible and expected results. If you
> disable the synchronization, then C++ streams are allowed to have their own
> independent buffers, which makes mixing C- and C++-style I/O an adventure.

### `cin.tie(nullptr)`

**Resources**

- [ios::tie](http://www.cplusplus.com/reference/ios/ios/tie/) (CPP) — documentation
- ★ [Significance of cin.tie(NULL);](https://stackoverflow.com/questions/31162367/significance-of-ios-basesync-with-stdiofalse-cin-tienull) (SO)

From the second resource:

> This unties `cin` from `cout`. Tied streams ensure that one stream is flushed
> automatically before each I/O operation on the other stream.
>
> By default `cin` is tied to `cout` to ensure a sensible user interaction. For
> example:
>
> ```cpp
> std::cout << "Enter name:";
> std::cin >> name;
> ```
>
> If `cin` and `cout` are tied, you can expect the output to be flushed (i.e.,
> visible on the console) before the program prompts input from the user. If you
> untie the streams, the program might block waiting for the user to enter their
> name but the "Enter name" message is not yet visible (because `cout` is
> buffered by default, output is flushed/displayed on the console only on demand
> or when the buffer is full).
>
> So if you untie `cin` from `cout`, you must make sure to flush `cout` manually
> every time you want to display something before expecting input on `cin`.

> **Warning: cout.tie(nullptr)**
>
> You may see some competitive programmers including this line. This doesn't
> actually do anything since `cout` isn't tied to anything. See
> [this post](https://codeforces.com/blog/entry/90775?#comment-791764) for
> details.

## Problems

| Problem | Source | Difficulty |
| --- | --- | --- |
| [Soldier and Number Game](https://codeforces.com/contest/546/problem/D) | CF | Medium |

---

*“Fast Input & Output” by Benjamin Qi, Nathan Chen, USACO Guide (https://usaco.guide/general/fast-io), CC BY-NC-SA 4.0 (https://creativecommons.org/licenses/by-nc-sa/4.0/); converted from MDX to Markdown.*
