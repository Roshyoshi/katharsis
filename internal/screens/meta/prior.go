package meta

import (
	"strconv"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/screens"
	"katharsis/internal/ui"
)

// PriorAnswer is the rating question's answer (D37): CF is the rating
// typed, 0 when skipped.
type PriorAnswer struct{ CF int }

// Prior asks once, on first launch, for the player's Codeforces rating, so
// the first doors start near it rather than at the cold start's guess.
// Digits type it, enter answers, esc skips; a skip is never asked again.
type Prior struct {
	digits string
}

// priorDigits fits every Codeforces rating there is.
const priorDigits = 4

func NewPrior() *Prior { return &Prior{} }

func (p *Prior) Update(msg tea.Msg) (screens.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	switch s := k.String(); s {
	case "enter":
		cf, _ := strconv.Atoi(p.digits) // "" skips
		return p, screens.Emit(PriorAnswer{CF: cf})
	case "esc":
		return p, screens.Emit(PriorAnswer{})
	case "backspace", "ctrl+h":
		if p.digits != "" {
			p.digits = p.digits[:len(p.digits)-1]
		}
	case "ctrl+u":
		p.digits = ""
	default:
		if len(k.Text) == 1 && k.Text[0] >= '0' && k.Text[0] <= '9' && len(p.digits) < priorDigits &&
			(p.digits != "" || k.Text != "0") {
			p.digits += k.Text
		}
	}
	return p, nil
}

func (p *Prior) View(width, height int) string {
	w, h := max(width, 1), max(height, 1)
	body := []string{
		ui.Strong.Render("one question before the first boot"),
		"",
		"What is your Codeforces rating, if you have one?",
		ui.Muted.Render("Your first doors start near it instead of at a guess."),
		ui.Muted.Render("A rating from fewer than 6 contests reads low; skip it then."),
		"",
		"rating: " + ui.Select.Render(p.digits+" "),
		"",
		ui.Muted.Render("enter saves" + ui.G.Sep + "esc skips" + ui.G.Sep + "asked once"),
	}
	top := max((h-len(body))/2, 0)
	lines := make([]string, top, h)
	for _, l := range body {
		lines = append(lines, ui.Center(l, w))
	}
	return ui.Layout(w, h, lines, "")
}
