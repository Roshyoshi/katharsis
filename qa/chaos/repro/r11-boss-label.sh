#!/usr/bin/env bash
# r11: the boss biome is labelled "Userland" on the map, the merge window's
# "leave →" line and the boss room's header, because the engine gives a
# boss biome Topic "mixed" and biomeLabel checks Topic before Kind.
set -u
. "$(dirname "$0")/../lib.sh"
kc_case r11 usaco
KC_BOOTWAIT=0 kc_start
kc_wait_for "GNU GRUB" 90
kc_keys Enter; kc_through "enter open door" 20
echo "== map on the first doors screen:"; kc_cap | grep -E "Userland|⚔" | head -3
kc_kill
