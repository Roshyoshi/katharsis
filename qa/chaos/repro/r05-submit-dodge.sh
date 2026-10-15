#!/usr/bin/env bash
# r05: a failing :Submit costs its life only when the pass finishes. The HP
# bar shows failures while it runs, and quitting the game (ctrl+c) or
# closing vim (:q) cancels the pass with no result: the failure is never
# charged, on a cursed door too (where it would end the boot).
set -u
. "$(dirname "$0")/../lib.sh"
kc_case r05 usaco
kc_first_door || exit 1
kc_solve slowwa; kc_pause 6
echo "== before:"; kc_state | sed -n 2p
kc_ed Escape ":Submit" Enter
for _ in $(seq 1 100); do kc_cap | grep -q "judging [1-9]/" && break; kc_pause 0.1; done
echo "== mid-pass HP bar:"; kc_cap | grep "submit "
kc_keys C-c; kc_pause 3
echo "== after ctrl+c (submits/failed/lives unchanged):"; kc_state | sed -n 2p

KC_BOOTWAIT=0 kc_start; kc_wait_ed 90; kc_pause 3
kc_ed Escape ":Submit" Enter
for _ in $(seq 1 100); do kc_cap | grep -q "judging [1-9]/" && break; kc_pause 0.1; done
echo "== mid-pass HP bar:"; kc_cap | grep "submit "
kc_ed Escape ":q" Enter; kc_pause 8
echo "== after :q in vim:"; kc_cap | grep -E "editor closed|submit" ; kc_state | sed -n 2p
kc_kill
