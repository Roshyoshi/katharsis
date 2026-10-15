#!/usr/bin/env bash
# r12: with tmux's remain-on-exit on (a common global setting), the player
# closes vim and reopens it with e, three times.
set -u
. "$(dirname "$0")/../lib.sh"
kc_case r12 usaco
kc_first_door || exit 1
kc_t set-option -g remain-on-exit on
for _ in 1 2 3; do
	p=$(kc_panes | grep "dead=0" | grep KATHARSIS_SOCK | cut -d' ' -f1 | head -1)
	kc_t send-keys -t "$p" Escape ":q" Enter
	kc_wait_for "editor closed" 10
	kc_keys e; kc_pause 3
done
echo "== panes after 3 close/reopen cycles:"; kc_panes | cut -c1-60
kc_kill
