#!/usr/bin/env bash
# r13: --editor was split with strings.Fields. A quoted argument broke (vim
# opened a buffer named `nu"` before the solution), and an editor that
# could not start just read "editor closed · e reopens it", with no reason.
#
# --editor is now a shell command line, and a missing editor is refused at
# start, before the TUI, so r13b runs the game directly (kc_direct): in the
# case's tmux server the pane is gone before it can be read.
set -u
. "$(dirname "$0")/../lib.sh"
kc_case r13 usaco
KC_EDITOR='vim -u NONE -N -c "set nu"' kc_first_door
kc_ed Escape ":ls" Enter; kc_pause 0.5
echo "== quoted editor arg, vim's buffers:"; kc_cap "$(kc_edpane)" | grep -E '^ +[0-9]+ ' | head -3
kc_kill

kc_case r13b usaco
echo "== missing editor binary:"; KC_EDITOR=nosuchvim kc_direct | tail -1
