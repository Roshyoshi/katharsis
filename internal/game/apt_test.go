package game_test

import (
	"errors"
	"slices"
	"testing"

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
