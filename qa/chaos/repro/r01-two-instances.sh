#!/usr/bin/env bash
# r01: two game processes on one store. Inside tmux nothing stopped a
# second `katharsis`; both resumed the same boot. Instance A cleared the
# door; B, still showing it open, checkpointed its stale state on every
# Tick, so the store lost A's clear, bytes and rating, and the attempts
# table kept a "cleared" row that the boot's own history no longer had.
#
# One game per store now: B is refused at start, before its TUI, so it is
# run directly (kc_direct): in the case's tmux server its window is gone
# before it can be read. A's clear stays.
set -u
. "$(dirname "$0")/../lib.sh"
kc_case r01 usaco
kc_first_door || exit 1
echo "== A has door open"; kc_state
echo "== B, started on the same store:"; kc_direct | tail -1
kc_solve ac; kc_pause 5
kc_ed Escape ":Submit" Enter
for _ in $(seq 1 120); do
	[ "$(kc_db "select count(*) from attempts where boot=1 and idx=0 and outcome='cleared';")" = 1 ] && break
	kc_pause 0.5
done
echo "== A's screen:"; kc_cap | head -2
echo "== store 3 s after A's AC:"; kc_pause 3; kc_state
kc_kill
KC_BOOTWAIT=0 kc_start; kc_wait_for "Boot #1" 60
echo "== relaunched:"; kc_cap | head -1
echo "== attempt idx 0 vs State.History[0], both cleared:"
kc_db "select 'attempts.idx0', outcome from attempts where boot=1 and idx=0;
       select 'state.History[0]', json_extract(state,'\$.History[0].Outcome') from boots where id=1;"
kc_kill
