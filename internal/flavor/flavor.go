// Package flavor is the game's text: crash quotes, biome and boss names,
// upgrade and daemon descriptions, banners, and sound files. Screens call
// these functions; nothing here affects rules.
package flavor

import (
	"hash/fnv"
	"strconv"
	"strings"

	"katharsis/internal/game"
	"katharsis/internal/rules"
)

type Quote struct {
	Text   string
	Author string
	Source string // where it was said or written, e.g. "Gödel, Escher, Bach"
	Year   int    // 0 if unknown
}

// QuoteFor picks a quote that fits how the boot crashed. verdict is the last
// judge verdict on the door ("TLE", "RE", ...); seed makes the pick stable
// for a given crash.
//
// The pool is the quotes that match cause and verdict, where a quote about
// the crash itself (no verdict tags) matches every verdict. Without a known
// verdict that leaves the quotes about the cause, and a pool smaller than
// minPool takes in the general quotes. General quotes outside the pool then
// fill a quarter of it, so the classics show up on every kind of crash. The
// pick walks the pool with a stride coprime to its size: consecutive seeds
// never repeat a quote, and n consecutive seeds visit all n.
func QuoteFor(cause game.Outcome, verdict string, seed uint64) Quote {
	v := normVerdict(verdict)
	h := hash64(string(cause) + "\x00" + v)
	pool := quotePool(cause, v, h)
	n := uint64(len(pool))
	i := ((seed%n)*stride(h, n) + h%n) % n
	return quotes[pool[i]].Quote
}

// minPool is the smallest pool QuoteFor settles for before widening: three
// keeps neighbouring seeds apart and still cycles.
const minPool = 3

// quotePool returns indices into quotes, in table order. h picks which
// general quotes join as filler, so different crashes get different ones.
func quotePool(c game.Outcome, v string, h uint64) []int {
	in := make([]bool, len(quotes))
	var pool []int
	add := func(keep func(entry) bool) {
		for i, e := range quotes {
			if !in[i] && keep(e) {
				in[i] = true
				pool = append(pool, i)
			}
		}
	}
	add(func(e entry) bool { return matches(e, c, v) })
	if len(pool) < minPool {
		add(func(e entry) bool { return e.general })
	}

	var spare []int
	for i, e := range quotes {
		if e.general && !in[i] {
			spare = append(spare, i)
		}
	}
	// ceil(len/3) extra makes the filler a quarter of the final pool.
	if k := min((len(pool)+2)/3, len(spare)); k > 0 {
		off := int(h % uint64(len(spare)))
		for j := range k {
			pool = append(pool, spare[(off+j)%len(spare)])
		}
	}
	return pool
}

// matches reports whether e fits a crash by c whose last verdict was v.
func matches(e entry, c game.Outcome, v string) bool {
	return hasCause(e, c) && (len(e.verdicts) == 0 || hasVerdict(e, v))
}

func hasCause(e entry, c game.Outcome) bool {
	for _, x := range e.causes {
		if x == c {
			return true
		}
	}
	return false
}

func hasVerdict(e entry, v string) bool {
	for _, x := range e.verdicts {
		if x == v {
			return true
		}
	}
	return false
}

// normVerdict folds judge verdicts onto the quote tags. OLE (output limit)
// is a resource blowout like MLE, usually an infinite print loop.
func normVerdict(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	if v == "OLE" {
		return "MLE"
	}
	return v
}

// stride is a step coprime to n, so stepping by it from any start visits
// every index before repeating one.
func stride(h, n uint64) uint64 {
	if n <= 2 {
		return 1
	}
	s := 1 + (h>>32)%(n-1)
	for gcd(s, n) != 1 {
		s = s%(n-1) + 1
	}
	return s
}

func gcd(a, b uint64) uint64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// hash64 is FNV-1a: fixed across Go releases, unlike maphash, so a crash
// keeps its quote after an upgrade.
func hash64(s string) uint64 {
	f := fnv.New64a()
	f.Write([]byte(s))
	return f.Sum64()
}

// BiomeName is the themed name for a catalog topic, e.g. "dp" → "The Cache".
// An unknown topic comes back as is, so a new topic still shows something.
func BiomeName(topic string) string {
	if b, ok := biomes[topic]; ok {
		return b.name
	}
	return topic
}

// BiomeBlurb is one line shown on the route choice.
func BiomeBlurb(topic string) string { return biomes[topic].blurb }

// BossName is the display name for a boss key (game.BossKeys or
// game.FinalBoss), e.g. "heisenbug" → "Heisenbug".
func BossName(key string) string {
	if b, ok := bosses[key]; ok {
		return b.name
	}
	return key
}

// BossBlurb is one line shown on the boss door.
func BossBlurb(key string) string { return bosses[key].blurb }

// PrestigeName labels a prestige level: 0 Bronze … 4 Advanced. A level
// past the table reads "Prestige N" rather than an empty label.
func PrestigeName(p int) string {
	if p >= 0 && p < len(prestiges) {
		return prestiges[p].name
	}
	return "Prestige " + strconv.Itoa(p)
}

// UpgradeText is an upgrade's name and effect under the standard rules;
// UpgradeTextFor quotes a boot's own numbers.
func UpgradeText(u game.Upgrade) (name, desc string) {
	return UpgradeTextFor(u, rules.Standard())
}

// DaemonText is a daemon's name and effect under the standard rules;
// DaemonTextFor quotes a boot's own numbers.
func DaemonText(d game.Daemon) (name, desc string) {
	return DaemonTextFor(d, rules.Standard())
}

// Banner is ASCII art for "ac", "boot", "cleared", "grub"; "" for any other
// kind.
func Banner(kind string) string { return banners[kind] }

// SoundFile maps a game.Sound name to an audio file for afplay, "" for none.
func SoundFile(name string) string { return soundFile(name) }
