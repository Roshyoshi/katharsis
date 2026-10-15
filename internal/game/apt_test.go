package game_test

import (
	"errors"
	"math"
	"slices"
	"testing"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/rules"
)

func pkg(t *testing.T, name string) rules.Package {
	t.Helper()
	p, ok := rules.Standard().Package(name)
	if !ok {
		t.Fatalf("no package %q in the standard rules", name)
	}
	return p
}

// Install takes a package's price from the bank once and records it; a
// second install, one whose requirements are missing and one the bank
// cannot cover are refused and change nothing.
func TestInstall(t *testing.T) {
	t.Parallel()
	duck, procps := pkg(t, "rubber-duck"), pkg(t, "procps")
	m := &game.Meta{Bank: duck.Price + 10}
	refused := func(p rules.Package) {
		t.Helper()
		before := mustJSON(t, m)
		if err := game.Install(m, p); !errors.Is(err, game.ErrInvalid) {
			t.Errorf("Install(%s): err %v, want ErrInvalid", p.Name, err)
		}
		if got := mustJSON(t, m); got != before {
			t.Errorf("refused Install(%s) changed meta %s to %s", p.Name, before, got)
		}
	}
	if err := game.Install(m, duck); err != nil {
		t.Fatal(err)
	}
	if m.Bank != 10 || !slices.Equal(m.Installed, []string{"rubber-duck"}) || !game.Installed(m, "rubber-duck") {
		t.Fatalf("after install: bank %d installed %v", m.Bank, m.Installed)
	}
	refused(duck) // idempotent: once only
	m.Bank = 1 << 20
	refused(procps) // needs rng-tools and strace
	for _, name := range procps.Requires {
		if err := game.Install(m, pkg(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	m.Bank = procps.Price - 1
	refused(procps) // one byte short
	m.Bank++
	if err := game.Install(m, procps); err != nil || m.Bank != 0 {
		t.Fatalf("Install(procps) with exactly its price: err %v, bank %d", err, m.Bank)
	}
}

// A banner theme is put in use when installed; UseTheme switches between
// installed themes and back to the default, and refuses one not installed.
func TestThemes(t *testing.T) {
	t.Parallel()
	r := rules.Standard()
	m := &game.Meta{Bank: 1 << 20}
	if err := game.UseTheme(r, m, "tux"); !errors.Is(err, game.ErrInvalid) || m.Theme != "" {
		t.Errorf("theme not installed: err %v, theme %q", err, m.Theme)
	}
	for _, name := range []string{"cowsay", "tux"} {
		if err := game.Install(m, pkg(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	if m.Theme != "tux" {
		t.Errorf("theme %q after installing tux last", m.Theme)
	}
	for _, theme := range []string{"cowsay", "", "tux"} {
		if err := game.UseTheme(r, m, theme); err != nil || m.Theme != theme {
			t.Errorf("UseTheme(%q): err %v, theme %q", theme, err, m.Theme)
		}
	}
}

// Packages change the boots that start after they are installed: the
// starting ducks and the link slots. A boot keeps what it started with.
func TestPackagesInBoot(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	libs := []game.Lib{{Name: "dsu"}, {Name: "bit"}, {Name: "seg"}}
	m := &game.Meta{Libs: libs, Bank: 1 << 20}
	h := newHarness(t, plain(), m, ps, 0, 1)
	ducks, slots := h.s.Ducks, h.s.LinkSlots()
	for _, name := range []string{"rubber-duck", "binutils"} {
		if err := game.Install(m, pkg(t, name)); err != nil {
			t.Fatal(err)
		}
	}
	if h.s.Ducks != ducks || h.s.LinkSlots() != slots || len(h.s.Packages) != 0 {
		t.Errorf("a running boot changed with the install: ducks %d slots %d packages %v", h.s.Ducks, h.s.LinkSlots(), h.s.Packages)
	}
	if err := h.try(game.Link{Libs: []string{"dsu", "bit", "seg"}}); !errors.Is(err, game.ErrInvalid) {
		t.Errorf("3 libs in %d slots: err %v", slots, err)
	}

	h = newHarness(t, plain(), m, ps, 0, 2)
	if h.s.Ducks != ducks+1 || h.s.LinkSlots() != slots+1 || !slices.Equal(h.s.Packages, m.Installed) {
		t.Errorf("next boot: ducks %d (want %d), slots %d (want %d), packages %v",
			h.s.Ducks, ducks+1, h.s.LinkSlots(), slots+1, h.s.Packages)
	}
	if err := h.try(game.Link{Libs: []string{"dsu", "bit", "seg"}}); err != nil {
		t.Errorf("3 libs in %d slots: %v", h.s.LinkSlots(), err)
	}
	// The package's duck comes at the start; a merge window refills ducks
	// to the base, keeping an unspent one.
	h.toMerge()
	picked := 0
	for _, u := range h.s.Upgrades {
		if u == game.Duck {
			picked++
		}
	}
	if want := ducks + 1 + picked; h.s.Ducks != want {
		t.Errorf("ducks %d at the merge window with the package's unspent, want %d", h.s.Ducks, want)
	}
}

// Every package of the standard rules is a real unlock: what it unlocks is
// one of the game's daemons, items or upgrades, and it does something.
func TestStandardPackages(t *testing.T) {
	t.Parallel()
	for _, p := range rules.Standard().Packages {
		if p.Daemon != "" && !slices.Contains(game.AllDaemons, game.Daemon(p.Daemon)) {
			t.Errorf("%s unlocks %q, not a daemon", p.Name, p.Daemon)
		}
		if p.Item != "" && !slices.Contains(game.AllItems, game.Item(p.Item)) {
			t.Errorf("%s unlocks %q, not an item", p.Name, p.Item)
		}
		if p.Upgrade != "" && !slices.Contains(game.AllUpgrades, game.Upgrade(p.Upgrade)) {
			t.Errorf("%s unlocks %q, not an upgrade", p.Name, p.Upgrade)
		}
		if p.Daemon == "" && p.Item == "" && p.Upgrade == "" && p.Theme == "" && p.Ducks == 0 &&
			p.LinkSlots == 0 && p.DaemonOffers == 0 && p.RouteOffers == 0 && p.UpgradeOffers == 0 && p.Doors == 0 {
			t.Errorf("%s does nothing", p.Name)
		}
		if p.Price <= 0 {
			t.Errorf("%s is free", p.Name)
		}
	}
}

// Each kind of package does what it says in the boots after it, and
// nothing without it.
func TestPackageEffects(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	boot := func(installed ...string) *harness {
		return newHarness(t, plain(), &game.Meta{Installed: installed}, ps, 0, 1)
	}

	// tmux: one more door per room.
	if n, m := len(boot().s.Doors), len(boot("tmux").s.Doors); m != n+1 {
		t.Errorf("tmux: %d doors, want %d", m, n+1)
	}
	// It does not stack with fork's door after an AC: the room holds the
	// more of the two.
	h := boot("tmux")
	h.s.Daemons = []game.Daemon{game.Fork}
	h.clearDoor(0, 10*time.Minute)
	if want := max(h.s.Rules.DoorsPerRoom+1, h.s.Rules.ForkDoors); len(h.s.Doors) != want {
		t.Errorf("tmux and fork after an AC: %d doors, want %d", len(h.s.Doors), want)
	}

	// git: one more topic at each route.
	h, g := boot(), boot("git")
	for i, b := range h.s.Biomes {
		if len(b.Offers) > 0 && len(g.s.Biomes[i].Offers) != len(b.Offers)+1 {
			t.Errorf("git: biome %d offers %v, without %v", i, g.s.Biomes[i].Offers, b.Offers)
		}
	}

	// coreutils: shuf joins the upgrade picks; a pick holds a shuf. At
	// full lives the pool is overclock, duck and shuf, all on offer.
	h = boot()
	h.s.MaxCycles = h.s.Cycles
	h.clearDoor(0, 10*time.Minute)
	if slices.Contains(h.s.UpgradeOffers, game.ShufPick) {
		t.Errorf("shuf offered without coreutils: %v", h.s.UpgradeOffers)
	}
	h = boot("coreutils")
	h.s.MaxCycles = h.s.Cycles
	h.clearDoor(0, 10*time.Minute)
	if !slices.Contains(h.s.UpgradeOffers, game.ShufPick) {
		t.Fatalf("coreutils: offers %v, want shuf among them", h.s.UpgradeOffers)
	}
	h.do(game.PickUpgrade{Upgrade: game.ShufPick})
	if h.s.Rerolls != 1 {
		t.Errorf("shuf pick: %d shufs held", h.s.Rerolls)
	}

	// aptitude: one more upgrade on offer, once the pool has it (a life
	// lost puts ECC in).
	h = boot("coreutils", "aptitude")
	h.open(0)
	h.do(game.SubmitResult{Verdict: "WA", Passed: 3, Total: 10})
	h.now = h.now.Add(10 * time.Minute)
	h.do(game.SubmitResult{Verdict: "AC", Passed: 10, Total: 10})
	if want := h.s.Rules.UpgradeOffers + 1; len(h.s.UpgradeOffers) != want {
		t.Errorf("aptitude at %d/%d lives: offers %v, want %d", h.s.Cycles, h.s.MaxCycles, h.s.UpgradeOffers, want)
	}

	// at: sold only once installed; it adds its time to the next door
	// opened, once.
	h = boot()
	h.toMerge()
	h.s.Uncommitted = 1000
	if h.s.Price(game.At) != 0 || h.try(game.Spend{Item: game.At}) == nil {
		t.Errorf("at for sale without its package")
	}
	h = boot("at")
	h.toMerge()
	h.pick()
	h.s.Uncommitted = 1000
	h.do(game.Spend{Item: game.At})
	if h.s.AtTime != h.s.Rules.AtTime.Duration {
		t.Fatalf("at bought: %v held", h.s.AtTime)
	}
	h.do(game.LeaveMerge{})
	for h.s.Phase == game.PhaseRoute {
		h.do(game.ChooseRoute{})
	}
	offered := h.s.Doors[0].Timer
	if shown := h.s.OpenTimer(h.s.Doors[0]); shown != offered+h.s.Rules.AtTime.Duration {
		t.Errorf("at: the card shows %v (offered %v), not the timer the door opens with", shown, offered)
	}
	h.open(0)
	if got := h.s.Current.Door.Timer; got != offered+h.s.Rules.AtTime.Duration || h.s.AtTime != 0 {
		t.Errorf("at: door timer %v (offered %v), %v still held", got, offered, h.s.AtTime)
	}
	if got := h.s.Current.Deadline.Sub(h.s.Current.Opened); got != h.s.Current.Door.Timer {
		t.Errorf("at: deadline %v after opening, timer %v", got, h.s.Current.Door.Timer)
	}

	// chrt: a fifth off every timer, every door pays 1.5x.
	h = boot("util-linux")
	h.s.Daemons = []game.Daemon{game.Chrt}
	h.open(0)
	h.do(game.Retreat{})
	for _, d := range h.s.Doors {
		pay := int64(math.Round(64 * min(max(0.5/d.SolveP, 0.25), 4) * 1.5))
		if d.Timer != 12*time.Minute || d.Bytes != pay { // 15m × 0.8
			t.Errorf("chrt door: %v, %d B; want 12m, %d B", d.Timer, d.Bytes, pay)
		}
	}
}

// cron pays CronMult on a regression door and leaves the others alone.
func TestCronPaysRegressions(t *testing.T) {
	t.Parallel()
	ps := dense(catalog.Topics, 800, 2600, 20)
	pay := func(d game.Door, mult float64) int64 {
		return int64(math.Round(64 * min(max(0.5/d.SolveP, 0.25), 4) * mult))
	}
	for _, cron := range []bool{false, true} {
		m := &game.Meta{
			Installed:   []string{"cron"},
			Regressions: []game.Regression{{Problem: ps[40].ID, Due: t0.Add(-time.Hour), Days: []int{3, 17}}},
		}
		h := newHarness(t, plain(), m, ps, 0, 1)
		h.toMerge()
		h.pick()
		if cron {
			h.s.Daemons = []game.Daemon{game.Cron}
		}
		h.do(game.LeaveMerge{})
		h.do(game.ChooseRoute{Index: 0})
		seen := false
		for _, d := range h.s.Doors {
			want := pay(d, 1)
			if d.Regression {
				seen = true
				if cron {
					want = pay(d, h.s.Rules.CronMult)
				}
			}
			if d.Bytes != want {
				t.Errorf("cron %v: door %+v pays %d, want %d", cron, d, d.Bytes, want)
			}
		}
		if !seen {
			t.Fatalf("cron %v: no regression door in %+v", cron, h.s.Doors)
		}
	}
}
