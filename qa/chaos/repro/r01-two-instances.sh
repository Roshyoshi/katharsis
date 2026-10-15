#!/usr/bin/env bash
# r01: two game processes on one store. Inside tmux nothing stops a second
# `katharsis`; both resume the same boot. Instance A clears the door; B,
# still showing it open, checkpoints its stale state on every Tick, so the
# store loses A's clear, bytes and rating, and the attempts table keeps a
# "cleared" row that the boot's own history no longer has.
set -u
. "$(dirname "$0")/../lib.sh"
kc_case r01 usaco
kc_first_door || exit 1
echo "== A has door open"; kc_state
kc_t new-window -t chaos "$(kc_gamecmd)"
for _ in $(seq 1 120); do
	[ "$(kc_panes | grep -c KATHARSIS_SOCK)" -ge 2 ] && break
	kc_pause 0.5
done
echo "== B resumed the same door"; kc_panes | cut -c1-70
kc_solve ac # into A's editor (the first editor pane)
kc_pause 5
kc_ed Escape ":Submit" Enter
kc_pause 10
echo "== A's screen:"; kc_cap "$(kc_t list-panes -t chaos:0 -F '#{pane_id} #{pane_start_command}' | awk '/--root/{print $1}')" | head -2
echo "== store 3 s after A's AC (B has ticked since):"; kc_pause 3; kc_state
kc_kill
KC_BOOTWAIT=0 kc_start
kc_wait_ed 90
echo "== relaunched, single instance:"; kc_cap | head -1; kc_state
# Give up the reopened door: the boot's history says gave-up, the attempts
# table still says cleared.
kc_ed Escape ":q!" Enter; kc_pause 1; kc_ed y; kc_pause 3
echo "== after :q! (attempt idx 0 vs State.History[0]):"
kc_db "select 'attempts.idx0', outcome from attempts where boot=1 and idx=0;
       select 'state.History[0]', json_extract(state,'\$.History[0].Outcome') from boots where id=1;"
kc_kill
