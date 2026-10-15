#!/usr/bin/env bash
# r03: the game dies (kill -9, a panic) inside its own session; vim lives
# on. Starting `katharsis` again from a terminal attaches to that session
# (new-session -A ignores argv): the player gets the orphan vim with a
# frozen border clock and no game.
set -u
. "$(dirname "$0")/../lib.sh"
kc_case r03 usaco
kc_outside
for _ in $(seq 1 120); do
	kc_inner capture-pane -p -t katharsis 2>/dev/null | grep -q "GNU GRUB" && break
	kc_pause 0.5
done
kc_inner send-keys -t katharsis Enter; kc_pause 2
kc_inner send-keys -t katharsis Enter
for _ in $(seq 1 60); do
	[ "$(kc_inner list-panes -s -t katharsis | wc -l)" -ge 2 ] && break
	kc_pause 0.5
done
echo "== game session:"; kc_inner list-panes -s -t katharsis -F '#{pane_id} #{pane_current_command}'
gp=$(kc_inner list-panes -s -t katharsis -F '#{pane_pid} #{pane_start_command}' | awk '/--root/{print $1; exit}')
kill -9 "$gp"; kc_pause 1
echo "== after kill -9 of the game (pid $gp):"; kc_inner list-panes -s -t katharsis -F '#{pane_id} #{pane_current_command}'
# A second terminal runs katharsis again.
env $(kc_env) tmux -L "$KC_SOCK" new-session -d -s shell2 -x 160 -y 45 \
	"env -u TMUX -u TMUX_PANE PS1='$ ' bash --norc -i"
kc_pause 0.5
cmd=$(kc_gamecmd); cmd=${cmd#exec }; cmd=${cmd%% 2>>*}
kc_t send-keys -t shell2 "$cmd" Enter
kc_pause 6
echo "== what the relaunch shows (top line is the frozen border):"
kc_t capture-pane -p -t shell2 | head -3
echo "== game running? $(kill -0 "$gp" 2>/dev/null && echo yes || echo no)"
kc_inner list-panes -s -t katharsis -F '#{pane_id} #{pane_current_command}'
kc_kill
