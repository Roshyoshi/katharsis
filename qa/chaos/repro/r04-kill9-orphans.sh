#!/usr/bin/env bash
# r04: kill -9 of the game mid-door, with unsaved edits and :Debug open.
# The editor and lldb panes outlive it, the stale socket and overlay stay
# until the next start sweeps them, the debug dir stays in $TMPDIR, and a
# relaunch inside tmux opens a second vim on the same file (E325).
set -u
. "$(dirname "$0")/../lib.sh"
kc_case r04 usaco
kc_first_door || exit 1
kc_solve wa; kc_pause 5
before=$(ls -d "$TMPDIR"/katharsis-debug-* 2>/dev/null | wc -l)
kc_ed Escape ":Debug 1" Enter; kc_pause 8
kc_ed Escape "Go// unsaved line" Escape
kill -9 "$(kc_gamepid)"; kc_pause 1
echo "== panes after kill -9:"; kc_panes | cut -c1-90
echo "== run dir:"; ls "$KC/home/Library/Caches/katharsis/run"
echo "== debug dirs in TMPDIR: before $before, after $(ls -d "$TMPDIR"/katharsis-debug-* 2>/dev/null | wc -l)"
kc_t new-window -t chaos "$(kc_gamecmd)"
for _ in $(seq 1 120); do
	[ "$(kc_panes | grep -c KATHARSIS_SOCK)" -ge 2 ] && break
	kc_pause 0.5
done
kc_pause 2
new=$(kc_panes | grep KATHARSIS_SOCK | grep -v lldb | tail -1 | cut -d' ' -f1)
echo "== resumed door's new editor ($new):"; kc_cap "$new" | grep -E "E325|STILL RUNNING|modified" | head -3
echo "== store (lives, bytes kept):"; kc_state | head -2
kc_kill
