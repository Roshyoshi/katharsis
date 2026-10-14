package game

import "math/rand/v2"

// splitmix is a math/rand/v2 Source whose whole state is one uint64 owned by
// State, so a boot serializes to JSON and resumes mid-run on the same
// sequence. rand.Rand keeps no state of its own in v2, which is what makes
// wrapping a borrowed pointer safe.
type splitmix struct{ s *uint64 }

// Uint64 is SplitMix64 (Steele, Lea, Flood 2014): one add and a finalizer,
// equidistributed over the full period, and every seed (0 included) is fine.
func (m splitmix) Uint64() uint64 {
	*m.s += 0x9e3779b97f4a7c15
	z := *m.s
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func newRand(state *uint64) *rand.Rand { return rand.New(splitmix{state}) }

// sample returns up to n distinct elements of xs in random order, leaving
// xs as it was.
func sample[T any](rng *rand.Rand, xs []T, n int) []T {
	n = min(n, len(xs))
	tmp := append([]T(nil), xs...)
	for i := range n {
		j := i + rng.IntN(len(tmp)-i)
		tmp[i], tmp[j] = tmp[j], tmp[i]
	}
	return tmp[:n:n]
}
