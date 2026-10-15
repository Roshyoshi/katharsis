#!/usr/bin/env bash
# r04: kill -9 of the game mid-door, with unsaved edits and :Debug open.
# The editor and lldb panes outlive it, the stale socket and overlay stay
# until the next start sweeps them, the debug dir stays in $TMPDIR, and a
# relaunch inside tmux opened a second vim on the same file (E325).
#
# The relaunch now saves and closes the orphan panes, and sweeps debug dirs
# over an hour old: one younger may belong to a debugger still open. The
# orphan dir is backdated two hours to show the sweep, beside a young one
# that must stay.
set -u
. "$(dirname "$0")/../lib.sh"
debugdirs() { find "$TMPDIR" -maxdepth 1 -type d -name "katharsis-debug-*" | sort; }
kc_case r04 usaco
kc_first_door || exit 1
kc_solve wa; kc_pause 5
before=$(debugdirs)
kc_ed Escape ":Debug 1" Enter; kc_pause 8
kc_ed Escape "Go// unsaved line" Escape
kill -9 "$(kc_gamepid)"; kc_pause 1
echo "== panes after kill -9:"; kc_panes | cut -c1-90
echo "== run dir:"; ls "$KC/home/Library/Caches/katharsis/run"
orphans=$(comm -13 <(echo "$before") <(debugdirs))
echo "== debug dirs the killed game left in TMPDIR: $(echo "$orphans" | grep -c .)"
for d in $orphans; do touch -t "$(date -v-2H +%Y%m%d%H%M.%S)" "$d"; done
young=$(mktemp -d "$TMPDIR/katharsis-debug-young.XXXXXX")
echo "   backdated:"; for d in $orphans; do echo "   $(stat -f '%Sm' "$d")  $(basename "$d")"; done
echo "   young:      $(stat -f '%Sm' "$young")  $(basename "$young")"

orphan_ed=$(kc_edpane)
kc_t new-window -t chaos "$(kc_gamecmd)"
for _ in $(seq 1 120); do
	kc_panes | grep KATHARSIS_SOCK | grep -v lldb | cut -d' ' -f1 | grep -qvx "$orphan_ed" && break
	kc_pause 0.5
done
kc_pause 2
new=$(kc_panes | grep KATHARSIS_SOCK | grep -v lldb | cut -d' ' -f1 | grep -vx "$orphan_ed" | head -1)
echo "== resumed door's new editor ($new), no E325:"
{ kc_cap "$new" | grep -E "E325|STILL RUNNING|modified" || echo "   (none)"; } | head -3
echo "== panes after the resume:"; kc_panes | cut -c1-90
echo "== the orphan's unsaved line reached the file:"
grep -c "unsaved line" "$(find "$KC/runs" -name "*.cpp" -path "*boot-*" -not -path "*/.submit/*" | head -1)"
echo "== debug dirs after the resume:"
for d in $orphans; do echo "   $(basename "$d") (2 h old): $([ -d "$d" ] && echo kept || echo swept)"; done
echo "   $(basename "$young") (young): $([ -d "$young" ] && echo kept || echo swept)"
rm -rf "$young"
echo "== store (lives, bytes kept):"; kc_state | head -2
kc_kill
