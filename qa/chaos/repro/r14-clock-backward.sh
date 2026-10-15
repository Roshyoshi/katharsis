#!/usr/bin/env bash
# r14 (time abuse): (a) a door whose deadline passed while the game was
# closed times out on resume, as designed: in Userland, where a timeout
# costs a life while one is spare (D44), the room is drawn again;
# elsewhere the boot crashes. The attempt's elapsed time is the whole
# absence. (b) a clock that jumped backward (Opened in the future):
# the AC records a negative elapsed time and earns the -O3 free upgrade.
set -u
. "$(dirname "$0")/../lib.sh"
db() { sqlite3 "$KC/data/katharsis/katharsis.db" "$@"; }
kc_case r14 usaco
kc_first_door || exit 1
kc_kill
past=$(date -v-1H +%Y-%m-%dT%H:%M:%S.000000%z | sed 's/\(..\)$/:\1/')
db "update boots set state = json_set(state, '\$.Current.Deadline', '$past') where outcome = ''"
KC_BOOTWAIT=0 kc_start
for _ in $(seq 1 180); do
	[ "$(db "select phase from boots order by id desc limit 1;")" != problem ] && break
	kc_pause 0.5
done
echo "== (a) resumed past the deadline (boot, then lives):"; kc_state | head -2
db "select 'attempt elapsed s', (closed-opened)/1e9 from attempts"
kc_kill

kc_case r14b usaco
kc_first_door || exit 1
kc_kill
fut=$(date -v+1d +%Y-%m-%dT%H:%M:%S.000000%z | sed 's/\(..\)$/:\1/')
db "update boots set state = json_set(state, '\$.Current.Opened', '$fut') where outcome = ''"
KC_BOOTWAIT=0 kc_start; kc_wait_ed 90; kc_pause 1
kc_solve ac; kc_pause 5
kc_ed Escape ":Submit" Enter
kc_wait_for "Pick an upgrade" 30
echo "== (b) clock behind the door's open time:"; kc_cap | grep "✓ AC"
db "select 'solve elapsed ns', json_extract(json, '\$.Solves[0].Elapsed') from meta"
kc_kill
