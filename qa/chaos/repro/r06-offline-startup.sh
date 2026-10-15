#!/usr/bin/env bash
# r06: with a warm USACO cache but no Codeforces index (never built, or
# corrupt), every start retries Codeforces before the TUI: about 18 s when
# the network refuses (4 retries, 1+2+4+8 s backoff), 3 min when it
# swallows packets (each attempt waits out the 3-minute context). Only
# "loading problems…" is on screen, and the "playing USACO only" warning is
# wiped by the TUI.
set -u
. "$(dirname "$0")/../lib.sh"
proxy=$KC_ROOT/bin/proxy # go build -tags qa -o $KC_ROOT/bin/proxy ./qa/chaos/proxy

kc_case r06 usaco # no codeforces/, proxy refuses
t0=$(date +%s); KC_BOOTWAIT=0 kc_start
kc_wait_for "GNU GRUB" 300
echo "== refused network: GRUB after $(($(date +%s) - t0)) s"
kc_kill

[ "${R06_HOLE:-0}" = 1 ] || exit 0
mode=$KC_ROOT/r06-mode; echo hole >"$mode"
"$proxy" -addr 127.0.0.1:9890 -mode "$mode" 2>"$KC_ROOT/r06-proxy.log" &
ppid=$!
kc_case r06b usaco
echo "HTTPS_PROXY=http://127.0.0.1:9890 HTTP_PROXY=http://127.0.0.1:9890 NO_PROXY=usaco.org" >"$KC/extraenv"
kc_use r06b
t0=$(date +%s); KC_BOOTWAIT=0 kc_start
kc_wait_for "GNU GRUB" 400
echo "== blackholed network: GRUB after $(($(date +%s) - t0)) s"
kc_kill
kill "$ppid"
