// The GNU extensions to namespace std that contest code leans on and libc++
// lacks or restricts. katharsis's judge force-includes this file (-include)
// in every build, so they work with any header, as they do with libstdc++.
#ifndef KATHARSIS_GNU_H
#define KATHARSIS_GNU_H

#if defined(__cplusplus) && __cplusplus >= 202002L && !defined(__GLIBCXX__)
namespace std {

// std::__gcd(m, n) is libstdc++'s Euclid, on any integer type. libc++ has
// an internal __gcd that static_asserts an unsigned argument, so
// std::__gcd(a, b) on two long longs fails to compile. This template is
// more constrained than that one, so it wins wherever both apply, and a
// program's own non-template __gcd still beats both.
template <class T>
  requires(__is_integral(T))
constexpr T __gcd(T m, T n) {
  while (n != 0) {
    T t = m % n;
    m = n;
    n = t;
  }
  return m;
}

// std::__lg(n) is floor(log2(n)), -1 for 0, as in libstdc++ 13 (which
// returns n's type).
template <class T>
  requires(__is_integral(T) && sizeof(T) <= 8)
constexpr T __lg(T n) {
  unsigned long long u = static_cast<unsigned long long>(n);
  if constexpr (sizeof(T) < 8)
    u &= (1ULL << (8 * sizeof(T))) - 1;
  return u == 0 ? T(-1) : T(63 - __builtin_clzll(u));
}

} // namespace std
#endif

#endif
