package flavor

import "strings"

// Banner limits: screens center banners in panes as narrow as a tmux
// split, so art stays within these and carries no color codes.
const (
	BannerMaxWidth = 60
	BannerMaxLines = 7
)

// The lettering is figlet's "Calvin S" face: box-drawing runes are one
// column wide in every terminal font, unlike emoji or block shading, so the
// art aligns the same everywhere. Leading newlines are trimmed below; the
// raw strings stay readable as drawn.
var banners = map[string]string{
	"ac": trimArt(`
┌──────────────────────────────┐
│   ╔═╗╔═╗╔═╗╔═╗╔═╗╔╦╗╔═╗╔╦╗   │
│   ╠═╣║  ║  ║╣ ╠═╝ ║ ║╣  ║║   │
│   ╩ ╩╚═╝╚═╝╚═╝╩   ╩ ╚═╝═╩╝   │
└─────────── exit 0 ───────────┘
`),
	"boot": trimArt(`
   _____
  /     \     ╔╦╗╔═╗╔═╗╔╦╗   ╔═╗╔═╗╔═╗╔═╗╔═╗
 | () () |     ║║║╣ ╠═╣ ║║───║  ╠═╣╚═╗║╣ ╚═╗
  \  ^  /     ═╩╝╚═╝╩ ╩═╩╝   ╚═╝╩ ╩╚═╝╚═╝╚═╝
   |||||      a competitive-programming roguelike
`),
	"cleared": trimArt(`
╔═╗╦  ╔═╗╔═╗╦═╗╔═╗╔╦╗
║  ║  ║╣ ╠═╣╠╦╝║╣  ║║
╚═╝╩═╝╚═╝╩ ╩╩╚═╚═╝═╩╝
[  OK  ] Reached target Clean Shutdown.
reboot: System halted
`),
	"grub": trimArt(`
╔╦╗╔═╗╔═╗╔╦╗   ╔═╗╔═╗╔═╗╔═╗╔═╗
 ║║║╣ ╠═╣ ║║───║  ╠═╣╚═╗║╣ ╚═╗
═╩╝╚═╝╩ ╩═╩╝   ╚═╝╩ ╩╚═╝╚═╝╚═╝
    GNU GRUB  version 2.12
`),
}

// trimArt drops the newline after the opening backquote and the one before
// the closing backquote, and trailing spaces, which some editors strip and
// others keep.
func trimArt(s string) string {
	lines := strings.Split(strings.Trim(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}
