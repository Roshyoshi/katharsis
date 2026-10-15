#!/usr/bin/env bash
# r02: a fatal error after the game re-execs itself into its own tmux
# session is printed inside that session, which ends with the process. The
# player's terminal shows only "[exited]". Shown with the first launch on a
# cold cache with no network (the README's own scenario), then with a
# corrupt store.
set -u
. "$(dirname "$0")/../lib.sh"
kc_case r02 cold
echo "HTTPS_PROXY=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9" >"$KC/extraenv"
kc_use r02
kc_outside
kc_pause 15
echo "== player's terminal, cold cache + offline:"
kc_t capture-pane -p -t shell | grep -v '^ *$' | tail -3
kc_kill

kc_case r02b usaco
mkdir -p "$KC/data/katharsis"
head -c 20000 /dev/urandom >"$KC/data/katharsis/katharsis.db"
kc_outside
kc_pause 6
echo "== player's terminal, corrupt store:"
kc_t capture-pane -p -t shell | grep -v '^ *$' | tail -3
echo "== what the game printed in its session before it ended (run with TMUX set):"
env $(kc_env) TMUX=x TMUX_PANE=%0 "$KC_BIN" --root "$KC/runs" --quiet 2>&1 | tail -1
kc_kill
