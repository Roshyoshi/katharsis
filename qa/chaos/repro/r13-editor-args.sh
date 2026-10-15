#!/usr/bin/env bash
# r13: --editor is split with strings.Fields. A quoted argument breaks
# (vim opens a buffer named `nu"` before the solution), and an editor that
# cannot start just reads "editor closed · e reopens it", with no reason.
set -u
. "$(dirname "$0")/../lib.sh"
kc_case r13 usaco
KC_EDITOR='vim -u NONE -N -c "set nu"' kc_first_door
kc_ed Escape ":ls" Enter; kc_pause 0.5
echo "== quoted editor arg, vim's buffers:"; kc_cap "$(kc_edpane)" | grep -E '^ +[0-9]+ ' | head -3
kc_kill

kc_case r13b usaco
KC_EDITOR=nosuchvim KC_BOOTWAIT=0 kc_start
kc_wait_for "GNU GRUB" 90
kc_keys Enter; kc_wait_for "enter open door" 20; kc_keys Enter; kc_pause 6
echo "== missing editor binary:"; kc_cap | head -1; kc_cap | grep "editor closed"
kc_keys e; kc_pause 3; echo "== after e:"; kc_cap | grep "editor closed"
kc_kill
