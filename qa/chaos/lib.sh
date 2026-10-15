# qa/chaos/lib.sh: isolated harness for chaos-testing katharsis.
#
# Source it from bash:  . qa/chaos/lib.sh
#
# Every game runs on a private tmux server (-L katharsis-chaos-<pid>,
# -f /dev/null) whose socket lives under the case's own TMUX_TMPDIR, with
# HOME, XDG_DATA_HOME and XDG_CONFIG_HOME pointing into the case directory,
# so nothing touches the player's tmux, store, caches or ~/cp.
#
#   KC_BIN    the katharsis binary under test (built from the tree)
#   KC_WARM   a HOME whose Library/Caches/katharsis is a warm cache copy
#   KC_TMPL   dir with template.cpp and atcoder-template.cpp
#
# kc_case NAME [warm|cold|usaco]  make a fresh case dir and set KC (its path)
# kc_start [game args...]    start the game in the case's tmux server
# kc_t ARGS...               tmux against the case's server
# kc_cap [pane]              capture a pane (default: the game pane)
# kc_keys KEYS...            send-keys to the game pane
# kc_ed KEYS...              send-keys to the editor pane
# kc_db SQL                  sqlite3 on the case's store
# kc_kill                    kill the case's tmux server and its processes
# kc_direct [game args...]   run the game from this shell; print its output
# kc_through TEXT [secs]     wait for TEXT on the game pane, past story scenes

REPO=${REPO:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}
# KC_ROOT is made by kc_setup (below).
KC_ROOT=${KC_ROOT:?set KC_ROOT to a dir kc_setup made}
KC_BIN=${KC_BIN:-$KC_ROOT/bin/kat}
KC_WARM=${KC_WARM:-$KC_ROOT/warmhome}
KC_TMPL=${KC_TMPL:-$KC_ROOT/tmpl}
KC_EDITOR=${KC_EDITOR:-vim -u NONE -N}

kc_case() {
	local name=$1 cache=${2:-warm}
	KC=$KC_ROOT/cases/$name
	rm -rf "$KC"
	mkdir -p "$KC"/{home,data,config,runs,tmux,log}
	if [ "$cache" = warm ] || [ "$cache" = usaco ]; then
		mkdir -p "$KC/home/Library/Caches"
		cp -cR "$KC_WARM/Library/Caches/katharsis" "$KC/home/Library/Caches/"
	fi
	# usaco: no Codeforces cache and no route to it, so doors are USACO
	# problems, which fetch in a second or two.
	KC_EXTRA_ENV=
	if [ "$cache" = usaco ]; then
		rm -rf "$KC/home/Library/Caches/katharsis/codeforces"
		KC_EXTRA_ENV="HTTPS_PROXY=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9 NO_PROXY=usaco.org"
	fi
	echo "$KC_EXTRA_ENV" >"$KC/extraenv"
	KC_SOCK=katharsis-chaos-$$-$name
	echo "$KC_SOCK" >"$KC/sock"
	export KC KC_SOCK
}

# kc_use NAME re-enters a case made earlier (another shell).
kc_use() {
	KC=$KC_ROOT/cases/$1
	KC_SOCK=$(cat "$KC/sock")
	KC_EXTRA_ENV=$(cat "$KC/extraenv" 2>/dev/null)
	export KC KC_SOCK
}

# kc_env prints the env(1) assignments that isolate a game.
kc_env() {
	echo HOME="$KC/home" XDG_DATA_HOME="$KC/data" XDG_CONFIG_HOME="$KC/config" \
		TMUX_TMPDIR="$KC/tmux" TERM=xterm-256color LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 \
		$KC_EXTRA_ENV
}

kc_t() { TMUX_TMPDIR="$KC/tmux" tmux -L "$KC_SOCK" "$@"; }

# kc_gamecmd prints the shell command line that runs the game. Prompts are
# off: the first-launch rating question would stand between every fresh
# case and GRUB.
kc_gamecmd() {
	local q=""
	for a in "$@"; do q="$q '${a//\'/\'\\\'\'}'"; done
	echo "exec '$KC_BIN' --root '$KC/runs' --quiet --no-prompts --editor '$KC_EDITOR'" \
		"--template '$KC_TMPL/atcoder-template.cpp' --template-fileio '$KC_TMPL/template.cpp'$q" \
		${KC_NOLOG:+"#"} "2>>'$KC/log/stderr'"
}

kc_start() {
	local cmd
	cmd=$(kc_gamecmd "$@")
	# shellcheck disable=SC2046
	# The plain user environment, as a player's shell has; KC_BIN wraps the
	# binary with the judge's tools on PATH, as the nix package does.
	env $(kc_env) tmux -L "$KC_SOCK" -f /dev/null new-session -d -s chaos \
		-x "${KC_W:-160}" -y "${KC_H:-45}" "$cmd"
	kc_t set-option -g remain-on-exit off >/dev/null 2>&1
	sleep "${KC_BOOTWAIT:-3}"
}

# kc_gamepane is the pane running the game (the session's first pane).
kc_gamepane() {
	kc_t list-panes -s -t chaos -F '#{pane_id} #{pane_start_command}' 2>/dev/null |
		awk '/--root/ {print $1; exit}'
}
# kc_edpane is the editor pane (the one running vim via env).
kc_edpane() {
	kc_t list-panes -s -t chaos -F '#{pane_id} #{pane_start_command}' 2>/dev/null |
		awk '/KATHARSIS_SOCK/ && !/lldb/ {print $1; exit}'
}
kc_panes() { kc_t list-panes -s -t chaos -F '#{pane_id} #{pane_width}x#{pane_height} dead=#{pane_dead} pid=#{pane_pid} #{pane_current_command} :: #{pane_start_command}' 2>&1 | cut -c1-200; }

kc_cap() { kc_t capture-pane -p -t "${1:-$(kc_gamepane)}" 2>&1; }
kc_keys() {
	local p
	p=$(kc_gamepane)
	[ -n "$p" ] || { echo "kc_keys: no game pane" >&2; return 1; }
	kc_t send-keys -t "$p" "$@"
}
kc_ed() {
	local p
	p=$(kc_edpane)
	[ -n "$p" ] || { echo "kc_ed: no editor pane" >&2; return 1; }
	kc_t send-keys -t "$p" "$@"
}
kc_db() { sqlite3 "$KC/data/katharsis/katharsis.db" "$@"; }

# kc_gamepid is the game process's pid (matched by this case's root).
kc_gamepid() { pgrep -f -- "--root $KC/runs" | head -1; }

# kc_procs lists every process that belongs to this case (by path).
kc_procs() { ps -axo pid,ppid,stat,command | grep -F "$KC" | grep -v grep | cut -c1-220; }

kc_kill() {
	kc_t kill-server >/dev/null 2>&1
	sleep 0.5
	local pids
	pids=$(ps -axo pid,command | grep -F "$KC" | grep -v grep | awk '{print $1}')
	[ -n "$pids" ] && kill -9 $pids 2>/dev/null
	true
}

# kc_wait_for TEXT [secs] waits until the game pane shows TEXT.
kc_wait_for() {
	local text=$1 n=${2:-20} pane=${3:-}
	for _ in $(seq 1 $((n * 4))); do
		kc_cap $pane | grep -qF -- "$text" && return 0
		sleep 0.25
	done
	return 1
}

# kc_doordir is the open door's directory (the editor pane's cwd).
kc_doordir() { kc_t display-message -p -t "$(kc_edpane)" '#{pane_current_path}'; }

# kc_solve [ac|wa|slow|ce] replaces the open door's source with the oracle
# through vim and writes it (:w), as a player's save would.
kc_solve() {
	local dir src
	[ -n "$(kc_edpane)" ] || { echo "kc_solve: no editor pane" >&2; return 1; }
	dir=$(kc_doordir)
	src=$KC/log/oracle-$(basename "$dir").cpp
	sh "$REPO/qa/chaos/oracle.sh" "$dir" "$KC/home/Library/Caches/katharsis" "${1:-ac}" >"$src" || return 1
	kc_ed Escape
	kc_ed ":silent %d _ | silent 0r $src | silent \$d _ | w" Enter
}

# kc_state prints the store's view of the newest boot.
kc_state() {
	kc_db "select 'boot', id, phase, outcome, cause, committed, last_door from boots order by id desc limit 1;
	       select 'live', json_extract(state,'\$.Lives'), json_extract(state,'\$.Uncommitted'),
	              json_extract(state,'\$.Current.Submits'), json_extract(state,'\$.Current.Failed'),
	              json_extract(state,'\$.Current.Deadline') from boots order by id desc limit 1;
	       select 'att', boot, idx, problem, outcome, submits, failed from attempts order by boot, idx;
	       select 'meta', json_extract(json,'\$.Bank'), json_extract(json,'\$.Boots'),
	              json_extract(json,'\$.CoreDump'), json_extract(json,'\$.Rating') from meta;"
}

# kc_outside [game args...] runs the game the way a player starts it from a
# terminal outside tmux: in a plain shell pane of the case's server, with
# TMUX unset, so it execs its own `tmux -L katharsis` session (under the
# case's TMUX_TMPDIR). The shell pane is "shell"; its text is what the
# player's terminal shows after the game's session ends.
kc_outside() {
	env $(kc_env) tmux -L "$KC_SOCK" -f /dev/null new-session -d -s shell \
		-x "${KC_W:-160}" -y "${KC_H:-45}" "env -u TMUX -u TMUX_PANE PS1='$ ' bash --norc -i"
	sleep 0.5
	local cmd
	cmd=$(kc_gamecmd "$@")
	cmd=${cmd#exec }
	cmd=${cmd%% 2>>*}
	kc_t send-keys -t shell "$cmd" Enter
}
# kc_inner ARGS... is tmux against the game's own session server
# (-L katharsis), the one kc_outside's game starts.
kc_inner() { TMUX_TMPDIR="$KC/tmux" tmux -L katharsis "$@"; }

# kc_direct [game args...] runs the game straight from this shell, as from a
# shell pane inside tmux, and prints what it printed. It is for an error
# that ends the game before its TUI: in a pane of the case's server the
# pane, or the whole server, is gone before kc_cap can read it. TMUX names
# a server that does not exist, so the game neither re-execs into a session
# of its own nor reaches a real server. A game that gets as far as its TUI
# is killed after 20 s.
kc_direct() {
	local cmd
	cmd=$(KC_NOLOG=1 kc_gamecmd "$@")
	# shellcheck disable=SC2046
	env $(kc_env) TMUX="$KC/tmux/none,1,0" TMUX_PANE=%0 \
		perl -e 'alarm shift; exec @ARGV' 20 sh -c "$cmd" 2>&1 </dev/null
}

# kc_mash N [pane] sends N random keys (no C-c, q or Q: those quit or give
# up by design and are tested on their own) to the game pane.
kc_mash() {
	local n=$1 pane=${2:-$(kc_gamepane)} i k
	local keys=(a b c d e f g h i j k l m n o p r s t u v w x y z A B C D E F G H I J K L M N O P S T U V W X Y Z
		0 1 2 3 4 5 6 7 8 9 Up Down Left Right Enter Escape Tab BTab BSpace Space Home End PPage NPage
		F1 F2 F5 F10 F12 C-z C-d C-l C-a C-e C-k C-u C-x C-p C-n M-x M-Enter '?' '/' ':' '!' 'é' '日本' '🦆' '#' '\\' '"')
	for ((i = 0; i < n; i++)); do
		k=${keys[RANDOM % ${#keys[@]}]}
		kc_t send-keys -t "$pane" -- "$k" 2>/dev/null
	done
}

# kc_wait_ed [secs] waits until the editor pane exists.
kc_wait_ed() {
	local n=${1:-60}
	for _ in $(seq 1 $((n * 2))); do
		[ -n "$(kc_edpane)" ] && return 0
		sleep 0.5
	done
	return 1
}

# kc_through TEXT [secs] waits until the game pane shows TEXT, sending esc
# to any story scene (S9) in the way: a boot starts with its splash, and a
# fresh store's first boot with the prologue and the first-door scene too.
# KC_INNER=1 watches kc_outside's game session instead.
kc_through() {
	local text=$1 n=${2:-20} s
	for _ in $(seq 1 $((n * 4))); do
		if [ -n "${KC_INNER:-}" ]; then
			s=$(kc_inner capture-pane -p -t katharsis 2>/dev/null)
		else
			s=$(kc_cap)
		fi
		case $s in
		*"esc skip"* | *"any key continue"*)
			if [ -n "${KC_INNER:-}" ]; then
				kc_inner send-keys -t katharsis Escape
			else
				kc_keys Escape
			fi
			;;
		*"$text"*) return 0 ;;
		esac
		kc_pause 0.25
	done
	return 1
}

# kc_first_door [game args...]: start the game, boot from GRUB's first
# entry, open door 1 and wait for its editor.
kc_first_door() {
	KC_BOOTWAIT=0 kc_start "$@"
	kc_wait_for "GNU GRUB" 90 || { echo "kc_first_door: no GRUB" >&2; return 1; }
	kc_keys Enter
	kc_through "enter open door" 20
	kc_keys Enter
	kc_wait_ed 60
}

# kc_pause SECS sleeps (the harness's shell allows long sleeps; perl is
# used so it also works where sleep is wrapped).
kc_pause() { perl -e "select undef, undef, undef, $1"; }

# kc_setup makes KC_ROOT from scratch: the game built from the tree, a
# wrapper that puts the judge's tools (clang++, g++, clang++-libstdc++,
# dsymutil) first on PATH as the nix package does, the chaos proxy, copies
# of the ~/cp templates, and an APFS clone of the warm cache (read only
# from the player's cache). path: lets it build a jj workspace too.
#
#   KC_ROOT=$(mktemp -d /tmp/kchaos.XXXXXX) bash -c '. qa/chaos/lib.sh; kc_setup'
kc_setup() {
	mkdir -p "$KC_ROOT"/{bin,tmpl,warmhome/Library/Caches}
	local tools
	tools=$(cd "$REPO" && nix develop path:. -c sh -c \
		"go build -o '$KC_ROOT/bin/katharsis' ./cmd/katharsis &&
		 go build -tags qa -o '$KC_ROOT/bin/proxy' ./qa/chaos/proxy &&
		 dirname \"\$(command -v clang++-libstdc++)\"") || return 1
	printf '#!/bin/sh\nPATH=%s:$PATH exec %s/bin/katharsis "$@"\n' "$tools" "$KC_ROOT" >"$KC_ROOT/bin/kat"
	chmod +x "$KC_ROOT/bin/kat"
	cp ~/cp/template.cpp "$KC_ROOT/tmpl/template.cpp"
	cp ~/cp/atcoder/template.cpp "$KC_ROOT/tmpl/atcoder-template.cpp"
	mkdir -p "$KC_ROOT/warmhome/Library/Caches/katharsis"
	for d in usaco codeforces guide; do
		[ -d ~/Library/Caches/katharsis/$d ] &&
			cp -cR ~/Library/Caches/katharsis/$d "$KC_ROOT/warmhome/Library/Caches/katharsis/"
	done
	true
}
