package boot

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/game"
	"katharsis/internal/screens"
	"katharsis/internal/ui"
	"katharsis/internal/units"
)

// Cleared is the victory screen after the last boss: how long the boot ran,
// what it banked, what it solved, and the prestige it unlocked or what
// unlocking the next still needs (D31). r or enter emits
// screens.StartBoot at the highest unlocked prestige, l screens.OpenLog,
// q screens.Quit.
type Cleared struct{ env Env }

func NewCleared(env Env) *Cleared { return &Cleared{env: env} }

// Refresh swaps in new state and clock.
func (c *Cleared) Refresh(env Env) { c.env = env }

// next is the prestige a reboot starts at: the highest unlocked, which the
// engine has already raised if this clear unlocked a new one.
func (c *Cleared) next() int {
	return min(max(c.env.Meta.Prestige, c.env.State.Prestige), game.MaxPrestige)
}

func (c *Cleared) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return c, nil
	}
	// Raw keys: l is git log here, not the vim move keyName makes it.
	switch k.String() {
	case "r", "enter":
		return c, screens.Emit(screens.StartBoot{Prestige: c.next(), Preset: c.env.State.Rules.Name})
	case "l":
		return c, screens.Emit(screens.OpenLog{})
	case "q":
		return c, screens.Emit(screens.Quit{})
	}
	return c, nil
}

func (c *Cleared) View(width, height int) string {
	st := c.env.State
	ss := steps(st)
	var solves, hinted, retreats int
	var boss string
	end := c.env.Now
	for _, s := range ss {
		a := s.Attempt
		switch a.Outcome {
		case game.Cleared:
			solves++
			if a.Quacks > 0 {
				hinted++
			}
			if a.Door.Boss != "" {
				boss = a.Door.Boss
			}
		case game.Retreated:
			retreats++
		}
		if !a.Closed.IsZero() {
			end = a.Closed
		}
	}
	uptime := end.Sub(st.Started)
	if st.Started.IsZero() {
		uptime = 0
	}

	var body []string
	if banner := fl.Banner("cleared"); banner != "" {
		bl := strings.Split(banner, "\n")
		for i, l := range bl {
			// systemd paints its OK green; the climax keeps it (E17).
			bl[i] = strings.Replace(l, "[  OK  ]", "[  "+ui.OK.Render("OK")+"  ]", 1)
		}
		// The climax gets the double frame (§2.5).
		if fw := maxWidth(bl) + 4; len(bl)+2 <= height/3 && fw <= width {
			body = append(body, centerBlock(ui.Frame(ui.FrameBoss, fw, "", "", bl), width)...)
		}
	}
	// Green paints the glyph, the words are Strong (R2, R3).
	body = append(body, "", ui.Center(ui.OK.Render(ui.G.Pass)+" "+ui.Strong.Render(fmt.Sprintf("Boot #%d cleared", st.Boot))+
		ui.Muted.Render(ui.G.Sep)+prestigeLabel(st.Prestige), width))
	if boss != "" {
		body = append(body, ui.Center(ui.Modifier(ui.ModBoss, bossLabel(boss)), width))
	}
	body = append(body, "")

	sol := units.Plural(solves, "door")
	if hinted > 0 {
		sol += ui.G.Sep + fmt.Sprintf("%d hinted", hinted)
	}
	if retreats > 0 {
		sol += ui.G.Sep + units.Plural(retreats, "retreat")
	}
	rows := [][2]string{
		{"uptime", units.Clock(uptime)},
		{"committed", ui.OK.Render(units.Bytes(st.Committed)) +
			ui.Muted.Render(ui.G.Sep+"bank ") + units.Bytes(c.env.Meta.Bank)},
		{"solved", sol},
		{"lives left", ui.Hearts(st.Lives, st.MaxLives)},
	}
	var table []string
	for _, r := range rows {
		table = append(table, ui.Muted.Render(fmt.Sprintf("%-12s", r[0]))+r[1])
	}
	body = append(body, centerBlock(table, width)...)

	// An achievement: the ★ is Warn, the words Strong (R2, R3).
	star := ui.Warn.Render(ui.G.Star) + " "
	switch g := game.NextPrestige(st.Rules, &c.env.Meta); {
	case st.Unlocked > 0:
		body = append(body, "", ui.Center(star+ui.Strong.Render("prestige unlocked: "+prestigeLabel(st.Unlocked)), width))
	case st.Prestige >= game.MaxPrestige:
		body = append(body, "", ui.Center(star+ui.Strong.Render("highest prestige cleared"), width))
	case g.Next > 0 && st.Trial == nil:
		body = append(body, "", ui.Center(ui.Muted.Render(screens.GateText(g, c.env.Meta.Prestige, prestigeLabel)), width))
	}

	if mp := centerBlock(renderMap(st, width), width); len(body)+1+len(mp) <= height-1 {
		body = append(body, "")
		body = append(body, mp...)
	}
	if pad := (height - 1 - len(body)) / 2; pad > 0 {
		body = append(make([]string, pad), body...)
	}
	// The keys sit at the left, as on every game screen (C2).
	footer := ui.Footer(width, "r", "reboot at "+prestigeLabel(c.next()), "l", "git log", "q", "quit")
	return ui.Layout(width, height, body, footer)
}
