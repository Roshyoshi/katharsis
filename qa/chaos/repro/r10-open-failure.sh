#!/usr/bin/env bash
# r10: the door's editor cannot be set up (here a read-only --root). The
# door opens in the engine, its timer runs, and the error shows once. Then
# the root is fixed and e is pressed: does the editor come back?
set -u
. "$(dirname "$0")/../lib.sh"
kc_case r10 usaco
KC_BOOTWAIT=0 kc_start
kc_wait_for "GNU GRUB" 90
kc_keys Enter; kc_wait_for "enter open door" 20
chmod 555 "$KC/runs"
kc_keys Enter; kc_pause 6
echo "== door open, no editor:"; kc_cap | head -1; kc_panes | wc -l; kc_cap | tail -2
chmod 755 "$KC/runs"
kc_keys e; kc_pause 4
echo "== after fixing the root and pressing e:"; kc_panes | cut -c1-60
kc_kill
