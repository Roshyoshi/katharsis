package meta

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"katharsis/internal/screens"
	"katharsis/internal/units"
)

// Package is one apt package as GRUB and the shop show it (D25). The app
// builds them from rules.Package and flavor's text.
type Package struct {
	Name     string
	Price    int64
	Grants   string   // what it does, one line
	Requires []string // packages to install first
	Theme    string   // the banner theme it installs, "" for none

	Installed bool
}

// ShopConfig is what the shop shows: the packages in shelf order (the app
// lists them cheapest first, the order of the goal ladder), the bank, and
// the banner theme in use.
type ShopConfig struct {
	Packages []Package
	Bank     int64
	Theme    string
}

// Shop is apt at GRUB: the packages the bank buys, one per line as dpkg
// lists them (ii for installed), the selected one's grants underneath.
// enter emits screens.Install, t screens.UseTheme for an installed
// theme (again to go back to the default), q or esc screens.Back.
type Shop struct {
	c   ShopConfig
	sel int
	top int
}

func NewShop(c ShopConfig) *Shop { return &Shop{c: c} }

// Refresh swaps in a new config after an install, keeping the selection.
func (s *Shop) Refresh(c ShopConfig) {
	s.c = c
	s.sel = min(s.sel, max(len(c.Packages)-1, 0))
}

func (s *Shop) installed(name string) bool {
	return slices.ContainsFunc(s.c.Packages, func(p Package) bool { return p.Name == name && p.Installed })
}

// missing lists p's requirements not yet installed.
func (s *Shop) missing(p Package) []string {
	var out []string
	for _, r := range p.Requires {
		if !s.installed(r) {
			out = append(out, r)
		}
	}
	return out
}

func (s *Shop) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok || len(s.c.Packages) == 0 {
		if ok && (k.String() == "q" || k.String() == "esc") {
			return s, screens.Emit(screens.Back{})
		}
		return s, nil
	}
	last := len(s.c.Packages) - 1
	p := s.c.Packages[s.sel]
	switch k.String() {
	case "up", "k":
		s.sel = max(s.sel-1, 0)
	case "down", "j":
		s.sel = min(s.sel+1, last)
	case "home", "g":
		s.sel = 0
	case "end", "G":
		s.sel = last
	case "enter", "i":
		if !p.Installed {
			return s, screens.Emit(screens.Install{Package: p.Name})
		}
	case "t":
		switch {
		case p.Theme == "" || !p.Installed:
		case s.c.Theme == p.Theme:
			return s, screens.Emit(screens.UseTheme{})
		default:
			return s, screens.Emit(screens.UseTheme{Theme: p.Theme})
		}
	case "q", "esc", "left", "backspace":
		return s, screens.Emit(screens.Back{})
	}
	return s, nil
}

// status is the one-word state of p in its row: what stands between the
// bank and it.
func (s *Shop) status(p Package) string {
	switch {
	case p.Installed && p.Theme != "" && p.Theme == s.c.Theme:
		return green.Render("in use")
	case p.Installed:
		return green.Render("installed")
	case len(s.missing(p)) > 0:
		return faint.Render("needs " + strings.Join(s.missing(p), ", "))
	case p.Price > s.c.Bank:
		return faint.Render(units.Bytes(p.Price-s.c.Bank) + " short")
	}
	return yellow.Render("enter installs")
}

func (s *Shop) View(width, height int) string {
	w, h := max(width, 1), max(height, 1)
	ps := s.c.Packages
	done := 0
	for _, p := range ps {
		if p.Installed {
			done++
		}
	}
	out := []string{
		spread(bold.Render("apt")+faint.Render(" · packages for every boot after"),
			fmt.Sprintf("bank %s · %d/%d installed", units.Bytes(s.c.Bank), done, len(ps)), w),
		"",
	}
	nameW := 4
	for _, p := range ps {
		nameW = max(nameW, len(p.Name))
	}
	head := fmt.Sprintf("    %-*s  %8s  %s", nameW, "PACKAGE", "PRICE", "STATUS")
	out = append(out, field.Render(pad(head, w)))

	// The list takes what the header, the grants and the key bar leave.
	rows := max(h-len(out)-5, 1)
	if s.sel < s.top {
		s.top = s.sel
	}
	if s.sel >= s.top+rows {
		s.top = s.sel - rows + 1
	}
	s.top = max(min(s.top, len(ps)-rows), 0)
	for i := s.top; i < len(ps) && i < s.top+rows; i++ {
		p := ps[i]
		mark := "  "
		if p.Installed {
			mark = "ii"
		}
		line := fmt.Sprintf(" %s %-*s  %8s  ", mark, nameW, p.Name, units.Bytes(p.Price))
		if i == s.sel {
			out = append(out, reverse.Render(pad(line+ansi.Strip(s.status(p)), w)))
			continue
		}
		out = append(out, line+s.status(p))
	}
	for len(out) < h-4 {
		out = append(out, "")
	}
	if len(ps) > 0 {
		p := ps[s.sel]
		// Two lines at most; the name leads the first, in bold.
		text := strings.Split(ansi.Wordwrap(p.Name+": "+p.Grants, max(w-2, 10), ""), "\n")
		out = append(out, "", " "+bold.Render(p.Name)+strings.TrimPrefix(text[0], p.Name))
		if len(text) > 1 {
			out = append(out, " "+text[1])
		}
	} else {
		out = append(out, "", faint.Render(" no packages in these rules"))
	}
	for len(out) < h-1 {
		out = append(out, "")
	}
	out = append(out[:h-1], keyBar("j/k", "Move  ", "enter", "Install  ", "t", "Theme  ", "q", "Back  "))
	return frame(out, w, h)
}

// Goal is the next package on the ladder (D25, retention RT3): the first,
// in shelf order, not installed whose requirements are, with what it
// costs against bank. ok is false when nothing is left to install.
func Goal(ps []Package, bank int64) (p Package, short int64, ok bool) {
	s := &Shop{c: ShopConfig{Packages: ps}}
	for _, p := range ps {
		if !p.Installed && len(s.missing(p)) == 0 {
			return p, max(p.Price-bank, 0), true
		}
	}
	return Package{}, 0, false
}
