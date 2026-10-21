#!/usr/bin/env bash
# Run ONLY through the prescribed testslot + offline nix develop (see README.md).
set -euo pipefail
mode=${1:?check|bench}
compiler=${2:?g++|clang++}
out=${3:?private output directory}
perf_lock=${4:-}
case "$mode:$compiler" in
  check:g++|check:clang++|bench:g++|bench:clang++) ;;
  *) printf 'expected check|bench and g++|clang++\n' >&2; exit 2 ;;
esac
here=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$out"
out=$(cd "$out" && pwd)
export XDG_CACHE_HOME="$out/cache"
export TMPDIR="$out/tmp"
export CCACHE_DISABLE=1
export GOCPP_MODCACHE=off GOCPP_OFFLINE=1 GOCPP_TEST_TIMEOUT=6m
mkdir -p "$XDG_CACHE_HOME" "$TMPDIR"
flags=(-std=c++20 -pthread -O2 -Wall -Wextra -Werror)
{
    date -u +%FT%TZ
    printf 'mode=%s compiler=%s flags=%s\n' "$mode" "$compiler" "${flags[*]}"
    printf 'compiler_path=%s\n' "$(command -v "$compiler")"
    "$compiler" --version
    uname -a
    getconf GNU_LIBC_VERSION
    git -C "$here" rev-parse HEAD
    sha256sum "$here/rc.hpp" "$here/support.hpp" "$here/rc_test.cpp" \
        "$here/rc_bench.cpp" "$here/run.sh" "$here/../../flake.lock"
} > "$out/identity.txt"

run_test() {
    local name=$1
    shift
    local status=0
    printf '%q ' "$compiler" "${flags[@]}" "$@" "$here/rc_test.cpp" -o "$out/$name"
    printf '\n'
    timeout 120s "$compiler" "${flags[@]}" "$@" "$here/rc_test.cpp" -o "$out/$name" \
        > "$out/$name.build.log" 2>&1 || status=$?
    printf '%s\n' "$status" > "$out/$name.build.status"
    if [ "$status" -ne 0 ]; then
        printf '%s compile failed (%s); see %s\n' "$name" "$status" "$out/$name.build.log"
        return "$status"
    fi
    status=0
    ASAN_OPTIONS=detect_leaks=1:halt_on_error=1 UBSAN_OPTIONS=halt_on_error=1 \
        TSAN_OPTIONS=halt_on_error=1 timeout 60s "$out/$name" \
        > "$out/$name.out" 2> "$out/$name.err" || status=$?
    printf '%s\n' "$status" > "$out/$name.status"
    if [ "$name" = tsan ] && \
        rg -q 'unexpected memory mapping|incompatible memory layout|ADDR_NO_RANDOMIZE' \
            "$out/$name.err"; then
        printf 'TSan unavailable at startup (status %s); retained stderr\n' "$status"
        return 0
    fi
    if [ "$status" -ne 0 ] || \
        rg -q 'AddressSanitizer|LeakSanitizer|UndefinedBehaviorSanitizer|ThreadSanitizer|runtime error:' \
            "$out/$name.out" "$out/$name.err"; then
        printf '%s failed (%s); see %s\n' "$name" "$status" "$out"
        return 1
    fi
    printf '%s passed\n' "$name"
}

if [ "$mode" = check ]; then
    run_test normal
    run_test asan -g -fno-omit-frame-pointer -fsanitize=address,undefined
    run_test tsan -g -fno-omit-frame-pointer -fsanitize=thread
else
    if [ -z "$perf_lock" ] || [ ! -f "$perf_lock" ]; then
        printf 'bench requires the existing shared performance lock path\n' >&2
        exit 2
    fi
    timeout 120s "$compiler" "${flags[@]}" "$here/rc_bench.cpp" -o "$out/bench" \
        > "$out/bench.build.log" 2>&1
    sha256sum "$out/bench" >> "$out/identity.txt"
    # Assembly is evidence only; do not use it to tune the frozen workload.
    timeout 120s "$compiler" "${flags[@]}" -S "$here/rc_bench.cpp" -o "$out/bench.s" \
        > "$out/assembly.log" 2>&1
    status=0
    # One process owns the shared lock for all balanced samples; bounded wait and execution.
    flock -w 120 "$perf_lock" timeout 60s "$out/bench" \
        > "$out/samples.csv" 2> "$out/bench.err" || status=$?
    printf '%s\n' "$status" > "$out/bench.status"
    if [ "$status" -ne 0 ]; then
        printf 'benchmark failed (%s); retained logs in %s\n' "$status" "$out"
        exit "$status"
    fi
    printf 'benchmark complete: %s\n' "$out/samples.csv"
fi
