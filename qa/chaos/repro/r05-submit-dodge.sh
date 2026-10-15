#!/usr/bin/env bash
# r05: a failing :Submit costs its life only when the pass finishes. The HP
# bar shows failures while it runs, and three ways out of the verdict were
# open: quitting the game (ctrl+c) and closing vim (:q) cancelled the pass
# with no result, and a fixed :Submit sent meanwhile replaced it. None was
# charged, on a cursed door too (where it would end the boot).
#
# The game saves a submit when it is made and judges it again on resume;
# closing vim leaves the pass running; a :Submit sent while one judges is
# refused. Each section below waits for its verdict.
set -u
. "$(dirname "$0")/../lib.sh"

# midpass waits for the HP bar to show a test judged.
midpass() {
	for _ in $(seq 1 100); do kc_cap | grep -q "judging [1-9]/" && break; kc_pause 0.1; done
}
# charged N waits up to 3 min for the open door's Nth verdict, or for the
# door to end.
charged() {
	local s
	for _ in $(seq 1 360); do
		s=$(kc_db "select coalesce(json_extract(state,'\$.Current.Submits'), 'gone') from boots order by id desc limit 1;")
		{ [ "$s" = "$1" ] || [ "$s" = gone ]; } && return 0
		kc_pause 0.5
	done
	return 1
}
live() {
	echo "   lives|uncommitted|submits|failed|deadline: $(kc_state | sed -n 2p | cut -d'|' -f2-)"
	echo "   SubmitAt: $(kc_db "select json_extract(state,'\$.Current.SubmitAt') from boots order by id desc limit 1;")"
}

kc_case r05 usaco
kc_first_door || exit 1
kc_solve slowwa; kc_pause 6
echo "== before:"; live
kc_ed Escape ":Submit" Enter; midpass
echo "== mid-pass HP bar:"; kc_cap | grep "submit "
kc_keys C-c; kc_pause 3
echo "== after ctrl+c: nothing charged yet, the submit saved (SubmitAt):"; live

KC_BOOTWAIT=0 kc_start; kc_wait_ed 90; kc_pause 3
echo "== resumed: the saved submit judges again:"; kc_cap | grep "submit "
# The fix, sent while that judge runs: it must not replace it.
kc_solve ac; kc_pause 2
kc_ed Escape ":Submit" Enter; kc_pause 2
echo "== a fixed :Submit during the resumed judge, vim says:"; kc_cap "$(kc_edpane)" | grep katharsis: | tail -1
charged 1
echo "== resumed verdict:"; live; kc_cap | grep "submit "

kc_solve slowwa; kc_pause 3
kc_ed Escape ":Submit" Enter; midpass
echo "== mid-pass HP bar:"; kc_cap | grep "submit "
kc_ed Escape ":q" Enter
charged 2
echo "== after :q in vim, the verdict lands:"; kc_cap | grep -E "editor closed|submit "; live
kc_kill
