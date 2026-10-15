#!/usr/bin/env bash
# r08: another writer holds the store's write lock (a second instance, an
# sqlite3 shell, a backup tool). Every Checkpoint runs inside Update and
# waits busy_timeout (5 s), and the door checkpoints on every 1 s Tick, so
# the whole UI freezes in 5 s steps: the clock stops, keys queue up.
set -u
. "$(dirname "$0")/../lib.sh"
kc_case r08 usaco
kc_first_door || exit 1
kc_pause 2
db=$KC/data/katharsis/katharsis.db
( sqlite3 "$db" "BEGIN IMMEDIATE; WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c WHERE x < 150000000) SELECT count(*) FROM c; COMMIT;" >/dev/null 2>&1 ) &
lock=$!
kc_pause 0.5
kc_keys t # open the statement: a key the UI should answer at once
for _ in $(seq 1 10); do
	printf '%s  clock %s  screen: %s\n' "$(date +%T)" \
		"$(kc_cap | grep -o '⏱ [0-9:]*' | head -1)" "$(kc_cap | head -1 | cut -c1-40)"
	kc_pause 1.5
done
wait "$lock"
kc_kill
