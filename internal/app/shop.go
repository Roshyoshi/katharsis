package app

import (
	tea "charm.land/bubbletea/v2"

	"katharsis/internal/flavor"
	"katharsis/internal/game"
	"katharsis/internal/rules"
	"katharsis/internal/screens/meta"
)

// apt's packages are bought between boots, so GRUB sells the standard
// preset's: a custom preset's are its own boots' business (rules.Package).
func (m *Model) aptRules() rules.Rules { return m.d.Presets["standard"] }

// packages is apt's shelf for GRUB and the shop, cheapest first.
func (m *Model) packages() []meta.Package {
	r := m.aptRules()
	var out []meta.Package
	for _, p := range r.Ladder() {
		out = append(out, meta.Package{
			Name:      p.Name,
			Price:     p.Price,
			Grants:    flavor.PackageText(p, r),
			Requires:  p.Requires,
			Theme:     p.Theme,
			Installed: game.Installed(&m.meta, p.Name),
		})
	}
	return out
}

func (m *Model) shopConfig() meta.ShopConfig {
	return meta.ShopConfig{Packages: m.packages(), Bank: m.meta.Bank, Theme: m.meta.Theme}
}

func (m *Model) openShop() tea.Cmd {
	if m.state != nil {
		return fail("apt installs between boots")
	}
	return m.push(meta.NewShop(m.shopConfig()))
}

// install buys a package with the bank and saves it; the shop and GRUB
// under it show the new bank.
func (m *Model) install(name string) tea.Cmd {
	if m.state != nil {
		return fail("apt installs between boots")
	}
	r := m.aptRules()
	p, ok := r.Package(name)
	if !ok {
		return fail("apt: no package %s", name)
	}
	if err := game.Install(&m.meta, p); err != nil {
		return fail("apt: %v", err)
	}
	if err := m.d.Store.SaveMeta(m.meta); err != nil {
		return fail("save: %v", err)
	}
	m.shopChanged()
	return note("installed %s: %s", name, flavor.PackageText(p, r))
}

// useTheme puts an installed banner theme in use, "" for the default.
func (m *Model) useTheme(theme string) tea.Cmd {
	if err := game.UseTheme(m.aptRules(), &m.meta, theme); err != nil {
		return fail("apt: %v", err)
	}
	if err := m.d.Store.SaveMeta(m.meta); err != nil {
		return fail("save: %v", err)
	}
	m.shopChanged()
	if theme == "" {
		return note("GRUB banner: default")
	}
	return note("GRUB banner: %s", theme)
}

// shopChanged rebuilds GRUB under the shop and refreshes the shop.
func (m *Model) shopChanged() {
	if m.state == nil {
		m.stack[0] = m.grub()
	}
	for _, s := range m.stack {
		if shop, ok := s.(*meta.Shop); ok {
			shop.Refresh(m.shopConfig())
		}
	}
}
