#!/usr/bin/env bash
# r09: a store file the game cannot write (read-only file, a full disk).
# SQLite opens it read-only without complaint, so the game starts, boots
# and plays; every save fails as a one-line flash that the next key
# clears. Nothing of the boot survives a restart.
set -u
. "$(dirname "$0")/../lib.sh"
kc_case r09 usaco
KC_BOOTWAIT=0 kc_start
kc_wait_for "GNU GRUB" 90
kc_kill # leaves a clean, checkpointed store
db=$KC/data/katharsis/katharsis.db
rm -f "$db-wal" "$db-shm"; chmod 444 "$db"
kc_first_door
echo "== door open on a read-only store:"; kc_cap | head -1
kc_pause 2; echo "== bottom line (the flash):"; kc_cap | tail -1
kc_keys j; kc_pause 0.3; echo "== after one key:"; kc_cap | tail -1
echo "== store has no boot:"; sqlite3 "$db" "select count(*) from boots;"
kc_kill
chmod 644 "$db"
