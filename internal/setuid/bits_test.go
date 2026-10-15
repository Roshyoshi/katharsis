package setuid_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/setuid"
)

// The fixtures are hand-built States, as the engine leaves them, so each
// bit's case can sit just on either side of its barrier.

var t0 = time.Date(2026, 10, 14, 20, 0, 0, 0, time.UTC)

// mapNodes is the route mapClear takes, tier by tier.
var mapNodes = []string{"userland", "scheduler", "off-by-one", "alu", "heisenbug", "kernel", game.FinalBoss}

// door is a cleared door of s's tier biome, opened at start into the boot
// for d, judged AC on its one submit, its clock the tier's.
func door(s *game.State, biome int, start, d time.Duration) game.Attempt {
	timer := s.Rules.Biomes[biome].Timer.Duration
	boss := s.Biomes[biome].Boss
	dr := game.Door{Problem: catalog.ID(fmt.Sprintf("t:%d", len(s.History))), Topic: game.Mixed, Rating: 1200, Timer: timer, Boss: boss}
	if boss != "" && boss != game.TrialBoss {
		dr.Twist = boss // the map's twists are on
	}
	return game.Attempt{
		Door: dr, Opened: t0.Add(start), Deadline: t0.Add(start + timer), Closed: t0.Add(start + d),
		Outcome: game.Cleared, Biome: biome, Submits: 1, Total: 10, BestPass: 10, LastVerdict: "AC",
		CyclesAtOpen: s.Cycles,
		Events:       []game.AttemptEvent{{At: t0.Add(start + d), Kind: "submit", Verdict: "AC", Passed: 10, Total: 10}},
	}
}

// boot lays out s's biomes on nodes and clears one door a room, 10
// minutes each with a minute between, so it ends cleared.
func boot(r rules.Rules, nodes []string) *game.State {
	s := &game.State{Boot: 7, Seed: 0x51c3, Rules: r, Started: t0, Phase: game.PhaseCleared,
		Cycles: r.Cycles, MaxCycles: r.MaxCycles, CountsSamples: true}
	for i, rb := range r.Biomes {
		b := game.BiomeState{Name: rb.Name, Kind: rb.Kind, Rooms: rb.Rooms}
		if len(nodes) > 0 {
			n, _ := r.Node(nodes[i])
			b.Name, b.Rooms, b.Node, b.Boss = n.Name, n.Rooms, n.ID, n.Boss
			if n.Skip {
				b.Rooms = 0
			}
		} else if rb.Kind == rules.Boss {
			b.Boss = "heisenbug"
		}
		s.Biomes = append(s.Biomes, b)
	}
	at := time.Duration(0)
	for i, b := range s.Biomes {
		for range b.Rooms {
			s.History = append(s.History, door(s, i, at, 10*time.Minute))
			at += 11 * time.Minute
		}
	}
	s.Biome, s.Room = len(s.Biomes)-1, 1
	return s
}

// mapClear is a three-act boot of the standard map, cleared: Userland, The
// Scheduler, Off-by-One, The ALU, Heisenbug, Kernel Space, The Halting
// Problem; 8 doors, 1:20:00 inside them.
func mapClear() *game.State { return boot(rules.Map(), mapNodes) }

// shortClear is the short preset cleared, its boss k.
func shortClear(k string) *game.State {
	s := boot(rules.Short(), nil)
	s.Biomes[2].Boss = k
	s.History[len(s.History)-1].Door.Boss, s.History[len(s.History)-1].Door.Twist = k, k
	return s
}

// crash ends s on its last door, timed out.
func crash(s *game.State) *game.State {
	a := &s.History[len(s.History)-1]
	a.Outcome, a.Closed = game.TimedOut, a.Deadline
	s.Phase = game.PhaseCrashed
	s.Crash = &game.Crash{Cause: game.TimedOut, Door: a.Door, At: a.Closed, Uptime: s.Uptime(a.Closed)}
	return s
}

// lasting stretches s's last door so its doors take d in all.
func lasting(s *game.State, d time.Duration) *game.State {
	a := &s.History[len(s.History)-1]
	a.Closed = a.Closed.Add(d - s.BootTime(t0))
	a.Deadline = a.Closed.Add(time.Hour)
	return s
}

// at is s's door of boss k.
func at(s *game.State, k string) *game.Attempt {
	for i := range s.History {
		if s.History[i].Door.Boss == k {
			return &s.History[i]
		}
	}
	panic("no door of " + k)
}

func clone(m *game.Meta) *game.Meta {
	raw, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	var out game.Meta
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return &out
}

// sets reports whether Check sets id on a copy of m with boot s.
func sets(m *game.Meta, s *game.State, id string) bool {
	m = clone(m)
	setuid.Check(m, s, t0.Add(10*time.Hour))
	return setuid.IsSet(m, id)
}

// full100 is a Meta at exactly 100%, and full112 one at 112%.
func full100() *game.Meta {
	m := &game.Meta{Retired: 1, Libs: []game.Lib{{Name: "dsu"}}, TopicAttempts: map[string]int{"dp": 10}}
	m.Capabilities = slices.Sorted(maps.Values(game.TopicCapabilities))
	r := &m.Record
	r.Bosses, r.Nodes = slices.Clone(setuid.Bosses), slices.Clone(setuid.Nodes)
	r.Unlocked, r.Cleared = []int{1, 2, 3, 4}, []int{4}
	r.Recovered, r.Skips, r.Nightly = 1, 1, 1
	r.Folded = 99
	return m
}

func full112() *game.Meta {
	m := full100()
	m.Record.Unlocked = append(m.Record.Unlocked, 5, 6, 7, 8)
	m.Record.Cleared = append(m.Record.Cleared, 8)
	m.Record.Endings = []string{game.EndingWake}
	return m
}

// bitCase is one bit's fixtures: each meet sets it, each miss does not.
type bitCase struct {
	meet, miss []func() (*game.Meta, *game.State)
}

func with(m *game.Meta, s *game.State) func() (*game.Meta, *game.State) {
	return func() (*game.Meta, *game.State) { return m, s }
}

// boots is with for a fresh Meta and the boot f builds.
func boots(f func() *game.State) func() (*game.Meta, *game.State) {
	return func() (*game.Meta, *game.State) { return &game.Meta{}, f() }
}

// metas is with for the Meta f builds, outside a boot.
func metas(f func(m *game.Meta)) func() (*game.Meta, *game.State) {
	return func() (*game.Meta, *game.State) {
		m := &game.Meta{}
		f(m)
		return m, nil
	}
}

// edit is boots of mapClear changed by f.
func edit(f func(s *game.State)) func() (*game.Meta, *game.State) {
	return boots(func() *game.State {
		s := mapClear()
		f(s)
		return s
	})
}

// cases holds every bit's fixtures, by id.
func cases() map[string]bitCase {
	fail := func(v game.Verdict, passed int) game.AttemptEvent {
		return game.AttemptEvent{Kind: "submit", Verdict: v, Passed: passed, Total: 10}
	}
	burn := func(a *game.Attempt, e game.AttemptEvent) {
		a.Events = append([]game.AttemptEvent{e}, a.Events...)
		a.Submits++
		a.Failed++
	}
	bossCase := func(k string) bitCase {
		return bitCase{
			meet: []func() (*game.Meta, *game.State){boots(func() *game.State { return shortClear(k) }),
				boots(func() *game.State { return crash(append2(shortClear(k))) })},
			miss: []func() (*game.Meta, *game.State){boots(func() *game.State { return crash(shortClear(k)) })},
		}
	}
	unlock := func(level int) bitCase {
		return bitCase{
			meet: []func() (*game.Meta, *game.State){edit(func(s *game.State) { s.Unlocked = level })},
			miss: []func() (*game.Meta, *game.State){edit(func(s *game.State) { s.Unlocked = level - 1 }),
				edit(func(s *game.State) { s.Unlocked = level + 1 })},
		}
	}
	return map[string]bitCase{
		"hello-world": {
			meet: []func() (*game.Meta, *game.State){boots(func() *game.State { return crash(shortClear("heisenbug")) })},
			miss: []func() (*game.Meta, *game.State){boots(func() *game.State {
				s := crash(shortClear("heisenbug"))
				s.History = s.History[len(s.History)-1:]
				return s
			})},
		},
		"exit-0": {
			meet: []func() (*game.Meta, *game.State){boots(mapClear), boots(func() *game.State { return shortClear("oom-killer") })},
			miss: []func() (*game.Meta, *game.State){boots(func() *game.State { return crash(mapClear()) }), edit(wake)},
		},
		"two-twins":        bossCase("off-by-one"),
		"there-goes-bill":  bossCase("oom-killer"),
		"shelf-stays-full": bossCase("heisenbug"),
		"caucus-race":      bossCase("race-condition"),
		"handsome-pig":     bossCase("undefined-behavior"),
		"snicker-snack": {
			meet: []func() (*game.Meta, *game.State){boots(mapClear)},
			miss: []func() (*game.Meta, *game.State){boots(func() *game.State { return shortClear(game.FinalBoss) }),
				boots(func() *game.State { return crash(mapClear()) })},
		},
		"through-the-glass":    unlock(1),
		"nameless-wood":        unlock(2),
		"eighth-square":        unlock(3),
		"red-kings-dream":      unlock(4),
		"sunny-at-midnight":    unlock(5),
		"where-the-fall-began": unlock(7),
		"you-halted": {
			meet: []func() (*game.Meta, *game.State){edit(func(s *game.State) { s.Prestige = 8 })},
			miss: []func() (*game.Meta, *game.State){edit(func(s *game.State) { s.Prestige = 7 })},
		},
		"stamped": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Capabilities = []string{"CAP_DP"} })},
			miss: []func() (*game.Meta, *game.State){metas(func(*game.Meta) {})},
		},
		"another-road": {
			// The Heap is behind CAP_DS; The ALU is open to all.
			meet: []func() (*game.Meta, *game.State){edit(func(s *game.State) { s.Biomes[3].Node = "heap" })},
			miss: []func() (*game.Meta, *game.State){boots(mapClear), edit(func(s *game.State) {
				s.Biomes[3].Node = "heap"
				s.History[4].Outcome = game.TimedOut
			})},
		},
		"grin-returns": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Record.Recovered = 1 })},
			miss: []func() (*game.Meta, *game.State){metas(func(*game.Meta) {})},
		},
		"struck-off": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Retired = 1 })},
			miss: []func() (*game.Meta, *game.State){metas(func(*game.Meta) {})},
		},
		"humpty": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Libs = []game.Lib{{Name: "dsu"}} })},
			miss: []func() (*game.Meta, *game.State){metas(func(*game.Meta) {})},
		},
		"one-topic": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.TopicAttempts = map[string]int{"dp": 10} })},
			miss: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.TopicAttempts = map[string]int{"dp": 9, game.Mixed: 40} })},
		},
		"by-railway": {
			meet: []func() (*game.Meta, *game.State){boots(func() *game.State {
				return boot(rules.Map(), []string{"userland", "scheduler", "off-by-one", "alu", "heisenbug", "skip", game.FinalBoss})
			})},
			miss: []func() (*game.Meta, *game.State){boots(mapClear)},
		},
		"un-birthday": {
			meet: []func() (*game.Meta, *game.State){edit(func(s *game.State) { s.Seed = game.NightlySeed(s.Started) })},
			miss: []func() (*game.Meta, *game.State){edit(func(s *game.State) { s.Seed = game.NightlySeed(s.Started.AddDate(0, 0, 1)) })},
		},

		"speedrun-1": {
			meet: []func() (*game.Meta, *game.State){edit(func(s *game.State) { lasting(s, 90*time.Minute-time.Second) })},
			miss: []func() (*game.Meta, *game.State){edit(func(s *game.State) { lasting(s, 90*time.Minute) })},
		},
		"speedrun-2": {
			meet: []func() (*game.Meta, *game.State){boots(func() *game.State {
				s := mapClear()
				for i := range s.History { // 7:20 a door
					s.History[i].Closed = s.History[i].Opened.Add(7*time.Minute + 20*time.Second)
				}
				return lasting(s, time.Hour-time.Second)
			})},
			miss: []func() (*game.Meta, *game.State){boots(mapClear)},
		},
		"all-o3": {
			meet: []func() (*game.Meta, *game.State){edit(func(s *game.State) {
				for i := range s.History {
					s.History[i].O3 = true
				}
				r := s.History[3]
				r.Outcome, r.O3 = game.Retreated, false
				s.History = slices.Insert(s.History, 3, r)
			})},
			miss: []func() (*game.Meta, *game.State){edit(func(s *game.State) {
				for i := range s.History[1:] {
					s.History[i+1].O3 = true
				}
			}), edit(func(s *game.State) {
				for i := range s.History {
					s.History[i].O3 = true
				}
				lost := s.History[3]
				lost.Outcome = game.TimedOut
				s.History = slices.Insert(s.History, 3, lost)
			})},
		},
		"finds-none": {
			// A CE and a submit that failed a sample cost nothing.
			meet: []func() (*game.Meta, *game.State){boots(mapClear), edit(func(s *game.State) {
				burn(&s.History[0], fail("CE", 0))
				e := fail("WA", 9)
				e.SampleFailed = true
				burn(&s.History[1], e)
			})},
			miss: []func() (*game.Meta, *game.State){edit(func(s *game.State) { burn(&s.History[5], fail("WA", 9)) }),
				edit(func(s *game.State) { burn(&s.History[5], fail("TLE", 3)) })},
		},
		"catless": {
			meet: []func() (*game.Meta, *game.State){boots(mapClear)},
			miss: []func() (*game.Meta, *game.State){edit(func(s *game.State) { s.History[2].Quacks = 1 })},
		},
		"essence": {
			meet: []func() (*game.Meta, *game.State){edit(func(s *game.State) {
				s.Upgrades = []game.Upgrade{game.Duck, game.ShufPick}
				s.Daemons = []game.Daemon{game.Strace, game.Chrt}
			})},
			miss: []func() (*game.Meta, *game.State){
				edit(func(s *game.State) { s.Upgrades = []game.Upgrade{game.Overclock} }),
				edit(func(s *game.State) { s.Upgrades = []game.Upgrade{game.ECC} }),
				edit(func(s *game.State) { s.Daemons = []game.Daemon{game.Nice} }),
				edit(func(s *game.State) { s.History[4].At = 5 * time.Minute }),
				edit(func(s *game.State) {
					r := s.History[1]
					r.Outcome = game.Retreated
					s.History = slices.Insert(s.History, 1, r)
				}),
			},
		},
		"doesnt-blink": {
			meet: []func() (*game.Meta, *game.State){edit(func(s *game.State) { s.Prestige = 8 })},
			miss: []func() (*game.Meta, *game.State){
				edit(func(s *game.State) { s.Prestige = 7 }),
				edit(func(s *game.State) { s.Prestige = 8; burn(&s.History[0], fail("RE", 0)) }),
				edit(func(s *game.State) { s.Prestige = 8; s.History[6].Quacks = 1 }),
			},
		},
		"ditto-ditto": {
			meet: []func() (*game.Meta, *game.State){boots(mapClear), edit(func(s *game.State) { burn(&s.History[0], fail("WA", 1)) })},
			miss: []func() (*game.Meta, *game.State){edit(func(s *game.State) { burn(at(s, "off-by-one"), fail("WA", 9)) }),
				// A boot saved before twists: no twist to master.
				edit(func(s *game.State) { at(s, "off-by-one").Door.Twist = "" })},
		},
		"rss-modest": {
			meet: []func() (*game.Meta, *game.State){boots(func() *game.State { return shortClear("oom-killer") })},
			miss: []func() (*game.Meta, *game.State){boots(func() *game.State {
				s := shortClear("oom-killer")
				burn(at(s, "oom-killer"), fail("CE", 0))
				return s
			})},
		},
		"stopped-looking": {
			meet: []func() (*game.Meta, *game.State){boots(mapClear)},
			miss: []func() (*game.Meta, *game.State){
				edit(func(s *game.State) { at(s, "heisenbug").Samples = 1 }),
				// Saved before the runs were counted: unknown, so not set.
				edit(func(s *game.State) { s.CountsSamples = false }),
			},
		},
		"every-thread": {
			meet: []func() (*game.Meta, *game.State){boots(func() *game.State {
				s := shortClear("race-condition")
				at(s, "race-condition").O3 = true
				return s
			})},
			miss: []func() (*game.Meta, *game.State){boots(func() *game.State { return shortClear("race-condition") })},
		},
		"bounds-checked": {
			meet: []func() (*game.Meta, *game.State){boots(func() *game.State { return shortClear("undefined-behavior") })},
			miss: []func() (*game.Meta, *game.State){boots(func() *game.State {
				s := shortClear("undefined-behavior")
				burn(at(s, "undefined-behavior"), fail("MLE", 2))
				return s
			})},
		},
		"decided-this-one": {
			meet: []func() (*game.Meta, *game.State){edit(func(s *game.State) { at(s, game.FinalBoss).O3 = true })},
			miss: []func() (*game.Meta, *game.State){boots(mapClear), edit(func(s *game.State) {
				a := at(s, game.FinalBoss)
				a.O3, a.Door.Twist = true, ""
			})},
		},
		"independent": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Record.Recovered, m.Record.RecoveredFree = 1, 1 })},
			miss: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Record.Recovered = 1 })},
		},
		"speed-completion": {
			meet: []func() (*game.Meta, *game.State){func() (*game.Meta, *game.State) {
				m := full100()
				m.Record.BootTime = 60*time.Hour - time.Second
				return m, nil
			}},
			miss: []func() (*game.Meta, *game.State){
				func() (*game.Meta, *game.State) {
					m := full100()
					m.Record.BootTime = 60 * time.Hour
					return m, nil
				},
				// Not the first check to find 100%: the bit's chance is gone.
				func() (*game.Meta, *game.State) {
					m := full100()
					m.Setuid = []game.SetuidBit{{ID: "hundred", At: t0}}
					return m, nil
				},
				// The boot in play counts toward the boot time.
				func() (*game.Meta, *game.State) {
					m := full100()
					m.Record.BootTime = 60*time.Hour - time.Second
					s := mapClear()
					s.Boot, s.Phase = 100, game.PhaseDoors
					return m, s
				},
				func() (*game.Meta, *game.State) {
					m := full100()
					m.TopicAttempts = nil // 97%
					return m, nil
				},
			},
		},
		"speed-completion-112": {
			meet: []func() (*game.Meta, *game.State){func() (*game.Meta, *game.State) {
				m := full112()
				m.Record.BootTime = 150*time.Hour - time.Second
				return m, nil
			}},
			miss: []func() (*game.Meta, *game.State){func() (*game.Meta, *game.State) {
				m := full112()
				m.Record.BootTime = 150 * time.Hour
				return m, nil
			}},
		},

		".dull-reality": {
			meet: []func() (*game.Meta, *game.State){edit(wake)},
			miss: []func() (*game.Meta, *game.State){boots(mapClear), boots(func() *game.State {
				s := shortClear("heisenbug")
				s.Ending = game.EndingWake
				return s
			})},
		},
		".two-miles-high": {
			meet: []func() (*game.Meta, *game.State){
				edit(func(s *game.State) { s.Capped = true }),
				// Saved before the engine marked it: a door opened at the cap.
				edit(func(s *game.State) { s.History[3].CyclesAtOpen = 50 }),
			},
			miss: []func() (*game.Meta, *game.State){
				edit(func(s *game.State) { s.History[3].CyclesAtOpen = 49 }),
				boots(func() *game.State {
					s := shortClear("heisenbug")
					s.Capped = true
					return s
				}),
			},
		},
		".stuff-and-nonsense": {
			meet: []func() (*game.Meta, *game.State){edit(func(s *game.State) {
				s.Biomes[4].Node, s.Phase = rules.TrialNode, game.PhaseDoors
			})},
			miss: []func() (*game.Meta, *game.State){boots(mapClear)},
		},
		".then-it-ends": {
			meet: []func() (*game.Meta, *game.State){edit(func(s *game.State) {
				a := at(s, game.FinalBoss)
				a.Deadline = a.Closed.Add(59 * time.Second)
			})},
			miss: []func() (*game.Meta, *game.State){
				edit(func(s *game.State) {
					a := at(s, game.FinalBoss)
					a.Deadline = a.Closed.Add(time.Minute)
				}),
				edit(func(s *game.State) {
					a := at(s, game.FinalBoss)
					a.Deadline, a.Overtime = a.Closed.Add(30*time.Second), true
				}),
			},
		},
		".galumphing-back": {
			meet: []func() (*game.Meta, *game.State){edit(func(s *game.State) {
				r := *at(s, game.FinalBoss)
				r.Outcome = game.Retreated
				s.History = slices.Insert(s.History, len(s.History)-1, r)
			})},
			miss: []func() (*game.Meta, *game.State){
				boots(mapClear),
				edit(func(s *game.State) { // a retreat from Heisenbug is no walk from the Jabberwock
					r := *at(s, "heisenbug")
					r.Outcome = game.Retreated
					s.History = slices.Insert(s.History, 5, r)
				}),
			},
		},
		".consider-your-verdict": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Record.Verdicts = slices.Clone(setuid.Verdicts) })},
			miss: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) {
				m.Record.Verdicts = slices.DeleteFunc(slices.Clone(setuid.Verdicts), func(v game.Verdict) bool { return v == "OLE" })
			})},
		},
		".every-seat": {
			meet: []func() (*game.Meta, *game.State){edit(func(s *game.State) {
				s.Daemons, s.Phase = []game.Daemon{game.Strace, game.Fork, game.Kexec}, game.PhaseMerge
			})},
			miss: []func() (*game.Meta, *game.State){edit(func(s *game.State) { s.Daemons = []game.Daemon{game.Strace, game.Fork} })},
		},
		".counted-again": {
			meet: []func() (*game.Meta, *game.State){edit(func(s *game.State) { burn(at(s, "off-by-one"), fail("WA", 9)) })},
			miss: []func() (*game.Meta, *game.State){
				edit(func(s *game.State) { burn(at(s, "off-by-one"), fail("WA", 8)) }),
				edit(func(s *game.State) {
					e := fail("WA", 9)
					e.SampleFailed = true // no burn at all
					burn(at(s, "off-by-one"), e)
				}),
				edit(func(s *game.State) { // no twist, no double burn
					a := at(s, "off-by-one")
					a.Door.Twist = ""
					burn(a, fail("WA", 9))
				}),
			},
		},
		".size-of-a-door": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { game.MarkSeen(m, "milestone.doors.500") })},
			miss: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { game.MarkSeen(m, "milestone.doors.100") })},
		},
		".which-dreamed-it": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) {
				m.Setuid = []game.SetuidBit{{ID: "snicker-snack"}, {ID: ".dull-reality"}}
			})},
			miss: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Setuid = []game.SetuidBit{{ID: ".dull-reality"}} })},
		},

		"every-bug": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Record.Bosses = slices.Clone(setuid.Bosses) })},
			miss: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) {
				m.Record.Bosses = append(slices.Clone(setuid.Bosses[1:]), game.TrialBoss)
			})},
		},
		"cartographer": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Record.Nodes = slices.Clone(setuid.Nodes) })},
			miss: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) {
				m.Record.Nodes = append(slices.Clone(setuid.Nodes[1:]), "skip", "heisenbug")
			})},
		},
		"regular-course": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) {
				m.Capabilities = slices.Collect(maps.Values(game.TopicCapabilities))
			})},
			miss: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) {
				m.Capabilities = slices.Collect(maps.Values(game.TopicCapabilities))[1:]
			})},
		},
		"apt-full": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Installed = packageNames() })},
			miss: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Installed = packageNames()[1:] })},
		},
		"own-invention": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Record.Daemons = slices.Clone(game.AllDaemons) })},
			miss: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Record.Daemons = slices.Clone(game.AllDaemons[1:]) })},
		},
		"her-list": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Retired = 5 })},
			miss: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Retired = 4 })},
		},
		"ledger": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Record.Recovered = 5 })},
			miss: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Record.Recovered = 4 })},
		},
		"not-in-general": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Rated = 100 })},
			miss: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Rated = 99 })},
		},
		"dictionary": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Libs = make([]game.Lib, 5) })},
			miss: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.Libs = make([]game.Lib, 4) })},
		},
		"every-topic": {
			meet: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.TopicAttempts = topics(10, 10) })},
			miss: []func() (*game.Meta, *game.State){metas(func(m *game.Meta) { m.TopicAttempts = topics(10, 9) })},
		},
		"hundred": {
			meet: []func() (*game.Meta, *game.State){with(full100(), nil)},
			miss: []func() (*game.Meta, *game.State){func() (*game.Meta, *game.State) {
				m := full100()
				m.Record.Nodes = m.Record.Nodes[1:] // 99%
				return m, nil
			}},
		},
		"hundred-twelve": {
			meet: []func() (*game.Meta, *game.State){with(full112(), nil)},
			miss: []func() (*game.Meta, *game.State){func() (*game.Meta, *game.State) {
				m := full112()
				m.Record.Nodes = m.Record.Nodes[1:] // 111%
				return m, nil
			}},
		},
	}
}

// append2 is a short boot whose boss fell before a door the boot then
// crashed on (a boss beaten, then a crash).
func append2(s *game.State) *game.State {
	a := s.History[len(s.History)-1]
	a.Door.Boss, a.Door.Problem = "", "t:extra"
	a.Opened, a.Closed = a.Closed.Add(time.Minute), a.Closed.Add(11*time.Minute)
	a.Deadline = a.Closed
	s.History = append(s.History, a)
	return s
}

// wake turns a map boot into one that took the Trial at Boss II and won.
func wake(s *game.State) {
	s.Biomes[4].Node, s.Biomes[4].Boss = rules.TrialNode, game.TrialBoss
	s.History = s.History[:6]
	a := &s.History[5]
	a.Door.Boss, a.Door.Cursed, a.Door.Twist = game.TrialBoss, true, ""
	s.Ending, s.Biome = game.EndingWake, 4
}

func packageNames() []string {
	var names []string
	for _, p := range rules.Standard().Packages {
		names = append(names, p.Name)
	}
	return names
}

// topics is every topic but mixed with n rated attempts, the last last.
func topics(n, last int) map[string]int {
	out := map[string]int{}
	for i, t := range setuid.Topics {
		out[t] = n
		if i == len(setuid.Topics)-1 {
			out[t] = last
		}
	}
	return out
}

// TestBits: every bit sets on each of its fixtures that just meet its
// barrier and on none of those that just miss it.
func TestBits(t *testing.T) {
	cs := cases()
	for _, d := range setuid.Table {
		c, ok := cs[d.ID]
		if !ok {
			t.Errorf("%s has no case", d.ID)
			continue
		}
		if len(c.meet) == 0 || len(c.miss) == 0 {
			t.Errorf("%s: %d meets, %d misses", d.ID, len(c.meet), len(c.miss))
		}
		for i, f := range c.meet {
			if m, s := f(); !sets(m, s, d.ID) {
				t.Errorf("%s: meet %d does not set it", d.ID, i)
			}
		}
		for i, f := range c.miss {
			if m, s := f(); sets(m, s, d.ID) {
				t.Errorf("%s: miss %d sets it", d.ID, i)
			}
		}
	}
	if len(cs) != len(setuid.Table) {
		t.Errorf("%d cases for %d bits", len(cs), len(setuid.Table))
	}
}

// TestOnlyBuiltIns: a custom preset's boot, and a learn-mode trial, set
// no bit of a boot, whatever they did; easing changes nothing.
func TestOnlyBuiltIns(t *testing.T) {
	custom := mapClear()
	custom.Rules.Name = "mine"
	custom.Prestige, custom.Capped = 8, true
	trial := mapClear()
	trial.Trial = &game.TrialState{}
	for name, s := range map[string]*game.State{"custom": custom, "trial": trial} {
		m := &game.Meta{}
		if got := setuid.Check(m, s, t0); len(got) != 0 {
			t.Errorf("%s set %v", name, got)
		}
	}
	eased := mapClear()
	eased.Ease = []game.Ease{{Kind: game.EaseGovernor, Cycles: 10}, {Kind: game.EaseFloor, Buyback: true}}
	eased.History[0].Door.TimerBuy = 1.5
	if !sets(&game.Meta{}, eased, "snicker-snack") || !sets(&game.Meta{}, eased, "speedrun-1") {
		t.Error("an eased boot lost a bit: easing never counts against one (GR9)")
	}
}

// TestCheckSafePoints: no bit while a door is open (GR6); Check twice is
// once; it never clears a bit; it stamps the boot and the time.
func TestCheckSafePoints(t *testing.T) {
	s := mapClear()
	s.Phase, s.Current = game.PhaseProblem, &s.History[0]
	m := &game.Meta{}
	if got := setuid.Check(m, s, t0); got != nil || m.Record.Folded != 0 {
		t.Fatalf("an open door set %v, folded %d", got, m.Record.Folded)
	}
	s = mapClear()
	now := t0.Add(3 * time.Hour)
	first := setuid.Check(m, s, now)
	if len(first) == 0 || first[0].ID != "hello-world" || first[0].Boot != s.Boot || !first[0].At.Equal(now) || first[0].Backfill {
		t.Fatalf("first check %+v", first)
	}
	before := slices.Clone(m.Setuid)
	if again := setuid.Check(m, s, now.Add(time.Hour)); len(again) != 0 {
		t.Errorf("a second check set %v", again)
	}
	setuid.Check(m, crash(mapClear()), now.Add(2*time.Hour)) // folded already: boot 7
	if !slices.Equal(m.Setuid[:len(before)], before) {
		t.Error("a later check changed a bit set before")
	}
	order := map[string]int{}
	for i, d := range setuid.Table {
		order[d.ID] = i
	}
	for i := 1; i < len(first); i++ {
		if order[first[i-1].ID] > order[first[i].ID] {
			t.Errorf("set out of the table's order: %s before %s", first[i-1].ID, first[i].ID)
		}
	}
}

// TestFold: a boot's end folds once, a trial only as folded, a custom
// preset only into the boot time; the record is what the boot reached.
func TestFold(t *testing.T) {
	m := &game.Meta{}
	s := mapClear()
	s.Daemons, s.Unlocked = []game.Daemon{game.Strace, game.Fork}, 2
	burn := s.History[2]
	burn.Outcome, burn.LastVerdict = game.TimedOut, "TLE"
	s.History = slices.Insert(s.History, 2, burn)
	setuid.Fold(m, s)
	setuid.Fold(m, s)
	r := m.Record
	want := game.Record{
		Folded:   7,
		Nodes:    []string{"userland", "scheduler", "alu", "kernel"},
		Bosses:   []string{"off-by-one", "heisenbug", game.FinalBoss},
		Verdicts: []game.Verdict{"AC", "TLE"},
		Daemons:  []game.Daemon{game.Strace, game.Fork},
		Unlocked: []int{2},
		Cleared:  []int{0},
		BootTime: s.BootTime(t0),
	}
	if mustJSON(t, r) != mustJSON(t, want) {
		t.Errorf("record\n%s\nwant\n%s", mustJSON(t, r), mustJSON(t, want))
	}

	w := mapClear()
	wake(w)
	w.Boot = 8
	setuid.Fold(m, w)
	if !slices.Equal(m.Record.Endings, []string{game.EndingWake}) || len(m.Record.Cleared) != 1 || slices.Contains(m.Record.Bosses, game.TrialBoss) {
		t.Errorf("after a wake: %+v", m.Record)
	}

	tr := mapClear()
	tr.Boot, tr.Trial = 9, &game.TrialState{}
	custom := mapClear()
	custom.Boot, custom.Rules.Name = 10, "mine"
	bt := m.Record.BootTime
	setuid.Fold(m, tr)
	if m.Record.Folded != 9 || m.Record.BootTime != bt {
		t.Errorf("a trial folded into %+v", m.Record)
	}
	setuid.Fold(m, custom)
	if m.Record.Folded != 10 || m.Record.BootTime != bt+custom.BootTime(t0) || len(m.Record.Cleared) != 1 {
		t.Errorf("a custom preset folded into %+v", m.Record)
	}
	running := mapClear()
	running.Boot, running.Phase = 11, game.PhaseDoors
	setuid.Fold(m, running)
	if m.Record.Folded != 10 {
		t.Error("a running boot folded")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
