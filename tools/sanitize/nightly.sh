#!/usr/bin/env sh
# The nightly sanitizer job on this machine (step 15.11, docs/guides/sanitizers.md): cron, a systemd
# timer or by hand. It runs inside the nix dev shell, keeps the compiled objects between nights, writes
# the results of each night into its own folder and one line into history.txt, and exits with the tool's
# status: 0 clean, 1 a stage failed (a sanitizer report, a canary that was not reported, a leak above
# its ceiling), 2 a usage error or the night could not start, 143 killed by TERM (130 INT, 129 HUP).
#
#   tools/sanitize/nightly.sh                       # the whole job: both compilers, all of tests/run
#   tools/sanitize/nightly.sh -sample               # a smoke test of the job (about two minutes)
#   tools/sanitize/nightly.sh -deep -j 16           # also make ci-sanitize and make ci-race
#
# Arguments go to "sanitize nightly". The job tests the COMMIT that HEAD is on, in a snapshot (a detached
# git worktree of HEAD at <night>/src, removed at the end, also when the night is killed, and removed at
# the start of the next night if a kill -9 or a power cut left it), not the live tree, so that editing the
# repository during the night changes nothing; uncommitted changes are not tested. The tool is built
# first and run directly, so its exit status reaches the caller as it is. The commit is in history.txt and
# in the night's commit.txt. Two nights never overlap (flock).
#
# Environment:
#   SANITIZE_RESULTS   where the nights go (default <repo>/sanitize-results)
#   SANITIZE_CACHE     compiled runtime and objects (default ~/.cache/gocpp-sanitize)
#   SANITIZE_LIVE=1    test the live tree instead of a snapshot (a dirty tree is recorded as such)
#   SANITIZE_PHASE     -phase N (default: every phase; the tool counts what it skips)
#   SANITIZE_RUNNER    a command that wraps the job, for example a slot script that limits the load
#   SANITIZE_DEVSHELL  how to enter the dev shell (default scripts/in-devenv, the cached dev shell; "nix
#                      develop -c" also works; set it empty to use the
#                      tools of the current environment, as the tests of this script do)
set -u
# the script may be run through a symlink (a cron entry, ~/bin): find the repository from the real file
self=$(readlink -f "$0" 2>/dev/null || echo "$0")
root=$(cd "$(dirname "$self")/../.." && pwd) || exit 2
results=${SANITIZE_RESULTS:-$root/sanitize-results}
cache=${SANITIZE_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/gocpp-sanitize}
devshell=${SANITIZE_DEVSHELL-$root/scripts/in-devenv}
mkdir -p "$results" "$cache" || exit 2
stamp=$(date -u +%Y%m%d-%H%M%S)

# One night at a time. The lock is taken before the night's folder is made, so that a night that does
# not run leaves nothing but its line in history.txt.
if command -v flock >/dev/null 2>&1; then
	exec 9>"$results/lock"
	if ! flock -n 9; then
		echo "$stamp skipped: another night is running" >>"$results/history.txt"
		echo "nightly sanitizers: another night is running (lock $results/lock)" >&2
		exit 2
	fi
fi

out=$results/$stamp
mkdir -p "$out" || exit 2
cd "$root" || exit 2

# Snapshots of nights that were killed hard (kill -9, a power cut): a <results>/<night>/src that is a
# registered git worktree of THIS repository, nothing else; it is removed with git, and the night gets
# a line in history.txt. A src folder that git does not know is never touched.
registered=$(git worktree list --porcelain 2>/dev/null | sed -n 's/^worktree //p')
for old in "$results"/*/src; do
	[ -f "$old/.git" ] || continue
	real=$(cd "$old" 2>/dev/null && pwd -P) || continue
	if printf '%s\n' "$registered" | grep -Fxq "$real"; then
		night=$(basename "$(dirname "$old")")
		echo "$night previous night killed hard, snapshot removed" >>"$results/history.txt"
		git worktree remove --force "$old" >/dev/null 2>&1 || rm -rf "$old"
	fi
done
git worktree prune >/dev/null 2>&1

commit=$(git rev-parse HEAD 2>/dev/null || echo unknown)
dirty=""
if [ -n "$(git status --porcelain 2>/dev/null)" ]; then dirty=" (tree has uncommitted changes)"; fi
snapshot=""
child=""
code=2
written=""

# finish runs on every way out: it stops the job if it still runs, removes the snapshot and writes the
# night's line.
finish() {
	trap '' TERM INT HUP
	if [ -n "$child" ] && kill -0 "$child" 2>/dev/null; then
		kill -TERM -- "-$child" 2>/dev/null || kill -TERM "$child" 2>/dev/null
		n=0
		while kill -0 "$child" 2>/dev/null && [ $n -lt 100 ]; do
			sleep 0.1
			n=$((n + 1))
		done
		kill -KILL -- "-$child" 2>/dev/null || kill -KILL "$child" 2>/dev/null
		wait "$child" 2>/dev/null
	fi
	if [ -n "$snapshot" ]; then
		cd "$root" 2>/dev/null
		git worktree remove --force "$snapshot" >/dev/null 2>&1 || rm -rf "$snapshot"
		git worktree prune >/dev/null 2>&1
	fi
	if [ -z "$written" ]; then
		written=1
		echo "$stamp exit $code $commit$dirty${note:-}" >>"$results/history.txt"
	fi
	exit "$code"
}
note=""
trap 'code=143; note=" (killed by TERM)"; exit 143' TERM
trap 'code=130; note=" (killed by INT)"; exit 130' INT
trap 'code=129; note=" (killed by HUP)"; exit 129' HUP
trap finish EXIT

work=$root
if [ -z "${SANITIZE_LIVE:-}" ]; then
	snapshot=$out/src
	if ! git worktree add --detach "$snapshot" HEAD >"$out/log.txt" 2>&1; then
		snapshot=""
		note=" (could not make the snapshot)"
		echo "nightly sanitizers: could not make a snapshot of HEAD (see $out/log.txt)" >&2
		exit 2
	fi
	work=$snapshot
	dirty=""
else
	[ -n "$dirty" ] && echo "nightly sanitizers: testing the live tree$dirty" >&2
fi
echo "$commit$dirty" >"$out/commit.txt"

phase=""
[ -n "${SANITIZE_PHASE:-}" ] && phase="-phase $SANITIZE_PHASE"
cd "$work" || exit 2

# The tool is built first and run directly: the go command would turn its exit 2 into 1.
bin=$out/sanitize
# shellcheck disable=SC2086
if ! $devshell go build -o "$bin" ./tools/sanitize >>"$out/log.txt" 2>&1; then
	note=" (could not build tools/sanitize)"
	echo "nightly sanitizers: could not build tools/sanitize (see $out/log.txt)" >&2
	exit 2
fi

# The job runs in the background in a process group of its own (setsid), so that a TERM to this
# script reaches all of it; the script waits for it.
# shellcheck disable=SC2086
if command -v setsid >/dev/null 2>&1; then
	setsid ${SANITIZE_RUNNER:-} $devshell "$bin" nightly $phase -out "$out" -gocpp-cache "$cache" "$@" >>"$out/log.txt" 2>&1 &
else
	${SANITIZE_RUNNER:-} $devshell "$bin" nightly $phase -out "$out" -gocpp-cache "$cache" "$@" >>"$out/log.txt" 2>&1 &
fi
child=$!
wait "$child"
code=$?
child=""
rm -f "$bin"

tail -n 15 "$out/log.txt"
if [ "$code" -ne 0 ]; then
	echo "nightly sanitizers FAILED (exit $code): $out" >&2
fi
exit "$code"
