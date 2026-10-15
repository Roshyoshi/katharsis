package flavor

import "strings"

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
  /     \     ╦╔═╔═╗╔╦╗╦ ╦╔═╗╦═╗╔═╗╦╔═╗
 | () () |    ╠╩╗╠═╣ ║ ╠═╣╠═╣╠╦╝╚═╗║╚═╗
  \  ^  /     ╩ ╩╩ ╩ ╩ ╩ ╩╩ ╩╩╚═╚═╝╩╚═╝
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
╦╔═╔═╗╔╦╗╦ ╦╔═╗╦═╗╔═╗╦╔═╗
╠╩╗╠═╣ ║ ╠═╣╠═╣╠╦╝╚═╗║╚═╗
╩ ╩╩ ╩ ╩ ╩ ╩╩ ╩╩╚═╚═╝╩╚═╝
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

// themes are the GRUB banners that apt packages install (rules.Package
// Theme), keyed "grub:" + theme. Each carries GRUB's version line, so it
// replaces the header as the default banner does.
var themes = map[string]string{
	"grub:cowsay": trimArt(`
 ________________________
< GNU GRUB  version 2.12 >
 ------------------------
        \   ^__^
         \  (oo)\_______
            (__)\       )\/\
                ||----w |
                ||     ||
`),
	"grub:tux": trimArt(`
    .--.
   |o_o |    katharsis
   |:_/ |    GNU GRUB  version 2.12
  //   \ \
 (|     | )
/'\_   _/ \
\___)=(___/
`),
	"grub:sl": trimArt(`
      ====        ________
  _D _|  |_______/        \__I_I_____
   |(_)---  |   H\________/ |   |
   /     |  |   H  |  |     |   |
  |      |  |   H  |__-----------
 GNU GRUB  version 2.12 · sl: did you mean ls?
`),
	"grub:neofetch": trimArt(`
   _____      root@katharsis
  /     \     --------------
 | () () |    OS: katharsis
  \  ^  /     Bootloader: GNU GRUB  version 2.12
   |||||      Shell: vim
`),
	"grub:cmatrix": trimArt(`
 0 1   1 0 0   1   0 1 1   0 1   0 0 1
 1 0 1   1   0 1 0   1   0 1   1 0   1
   1 0 0 1 0   1 1 0   1   0 1   1 0
 0   1   wake up, katharsis...   0   1
 1 0     GNU GRUB  version 2.12    1 0
   0 1 1   0 1 0   1 0 1   1 0 1   0
`),
	"grub:bastet": trimArt(`
 |          |
 |    []    |
 |  [][][]  |   bastet: bastard tetris
 |[][]  [][]|   GNU GRUB  version 2.12
 |[][][][][]|
 +----------+
`),
	"grub:figlet": strings.Join([]string{
		" _                _     _                     _",
		"| | __  __ _  _  | |__    __ _  _ __  ___  (_) ___",
		"| |/ / / _` || |_| '_ \\  / _` || '__|/ __| | |/ __|",
		"|   < | (_| ||  _| | | || (_| || |   \\__ \\ | |\\__ \\",
		"|_|\\_\\ \\__,_| \\__|_| |_| \\__,_||_|   |___/ |_||___/",
		"                GNU GRUB  version 2.12",
	}, "\n"),
}
