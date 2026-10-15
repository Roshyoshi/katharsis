package game

import (
	"slices"

	"katharsis/internal/rules"
	"katharsis/internal/units"
)

// Packages are the bank's sink (D25): bytes committed at merge windows buy
// them at GRUB, between boots, and every boot after plays with them. A
// boot snapshots Meta.Installed when it begins (State.Packages), so what
// it plays with is fixed for its length, as its rules are.

// Install buys pkg with banked bytes and records it in m. It refuses, with
// an ErrInvalid error and m unchanged, a package already installed, one
// whose requirements are not, and one the bank cannot cover: installing is
// once, and the bank never goes below zero. Installing a theme also puts
// it in use.
func Install(m *Meta, pkg rules.Package) error {
	switch {
	case Installed(m, pkg.Name):
		return invalid("%s is already installed", pkg.Name)
	case m.Bank < pkg.Price:
		return invalid("%s costs %s, bank %s", pkg.Name, units.Bytes(pkg.Price), units.Bytes(m.Bank))
	}
	for _, req := range pkg.Requires {
		if !Installed(m, req) {
			return invalid("%s needs %s first", pkg.Name, req)
		}
	}
	m.Bank -= pkg.Price
	m.Installed = append(m.Installed, pkg.Name)
	if pkg.Theme != "" {
		m.Theme = pkg.Theme
	}
	return nil
}

// Installed reports whether m has the package named name.
func Installed(m *Meta, name string) bool { return slices.Contains(m.Installed, name) }

// UseTheme puts the banner theme of an installed package of r in use; ""
// goes back to the default.
func UseTheme(r rules.Rules, m *Meta, theme string) error {
	if theme != "" && !slices.ContainsFunc(r.Packages, func(p rules.Package) bool {
		return p.Theme == theme && Installed(m, p.Name)
	}) {
		return invalid("theme %q is not installed", theme)
	}
	m.Theme = theme
	return nil
}

// LinkSlots is how many libs the boot may link: the rules' slots plus its
// packages'.
func (s *State) LinkSlots() int { return s.Rules.LinkSlots + s.Rules.Grants(s.Packages).LinkSlots }

// DaemonLocked, ItemLocked and UpgradeLocked report whether the boot
// leaves d, it or u out (of the merge windows' offers and shelf, of the
// upgrade picks) because the package that provides it was not installed.
func (s *State) DaemonLocked(d Daemon) bool {
	daemons, _, _ := s.Rules.Locked(s.Packages)
	return slices.Contains(daemons, string(d))
}

func (s *State) ItemLocked(it Item) bool {
	_, items, _ := s.Rules.Locked(s.Packages)
	return slices.Contains(items, string(it))
}

func (s *State) UpgradeLocked(u Upgrade) bool {
	_, _, upgrades := s.Rules.Locked(s.Packages)
	return slices.Contains(upgrades, string(u))
}
