#!/usr/bin/env bash
# r09: a store file the game cannot write (read-only file, a full disk).
# SQLite opened it read-only without complaint, so the game started, booted
# and played; every save failed as a one-line flash that the next key
# cleared, and nothing of the boot survived a restart.
#
# The game now refuses such a store at start, before its TUI, so it is run
# directly (kc_direct): in the case's tmux server the pane is gone before
# it can be read.
set -u
. "$(dirname "$0")/../lib.sh"
kc_case r09 usaco
KC_BOOTWAIT=0 kc_start
kc_wait_for "GNU GRUB" 90
kc_kill # leaves a clean, checkpointed store
db=$KC/data/katharsis/katharsis.db
rm -f "$db-wal" "$db-shm"; chmod 444 "$db"
echo "== start on a read-only store:"; kc_direct | tail -1
echo "== store has no boot:"; sqlite3 "$db" "select count(*) from boots;"
chmod 644 "$db"
