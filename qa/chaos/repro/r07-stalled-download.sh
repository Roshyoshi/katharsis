#!/usr/bin/env bash
# r07: first start, cold cache; the USACO Guide download stalls after a
# few seconds (the connection stays open, no bytes move). There is no idle
# timeout on the body read, so the game sits on "loading problems…" for the
# full 10-minute context, then exits. Takes ~10 minutes.
set -u
. "$(dirname "$0")/../lib.sh"
proxy=$KC_ROOT/bin/proxy
mode=$KC_ROOT/r07-mode; echo pass >"$mode"
"$proxy" -addr 127.0.0.1:9891 -mode "$mode" 2>"$KC_ROOT/r07-proxy.log" &
ppid=$!
kc_case r07 cold
echo "HTTPS_PROXY=http://127.0.0.1:9891 HTTP_PROXY=http://127.0.0.1:9891" >"$KC/extraenv"
kc_use r07
kc_t set-option -g remain-on-exit on 2>/dev/null
t0=$(date +%s); KC_NOLOG=1 KC_BOOTWAIT=0 kc_start
kc_t set-option -g remain-on-exit on
kc_pause 5; echo stall >"$mode"
until kc_cap | grep -q "GNU GRUB\|Pane is dead"; do kc_pause 5; done
echo "== stalled at 5 s; game reacted after $(($(date +%s) - t0)) s:"
kc_cap | grep -v '^ *$' | tail -3
kc_kill
kill "$ppid"
