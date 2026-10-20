#!/usr/bin/env bash
# Differential fuzz of borrowed pointer parameters (ADR-086): see README.md.
# usage: run.sh MODE FIRST LAST [GOCPP]   MODE is ptr or val; GOCPP defaults to gocpp in PATH.
# Env: BORROWFUZZ_WORK, BORROWFUZZ_JOBS (4), BORROWFUZZ_N (30),
# BORROWFUZZ_CXX ("g++ clang++"), BORROWFUZZ_TIMEOUT (600s per command).
set -euo pipefail
command -v rg > /dev/null
here=$(cd "$(dirname "$0")" && pwd)
mode=${1:?mode ptr|val}
first=${2:?first seed}
last=${3:?last seed}
bin=${4:-gocpp}
work=${BORROWFUZZ_WORK:-$(mktemp -d)}
jobs=${BORROWFUZZ_JOBS:-4}
n=${BORROWFUZZ_N:-30}
limit=${BORROWFUZZ_TIMEOUT:-600s}

case "$mode" in
  ptr|val) ;;
  *)
    printf 'mode must be ptr or val\n' >&2
    exit 2
    ;;
esac
for value in "$first" "$last" "$jobs" "$n"; do
  if ! [[ "$value" =~ ^[0-9]+$ ]]; then
    printf 'expected an unsigned integer\n' >&2
    exit 2
  fi
done
if [ "$first" -gt "$last" ] || [ "$jobs" -eq 0 ] || [ "$n" -eq 0 ]; then
  printf 'invalid seed range, job count or program size\n' >&2
  exit 2
fi

# Resolve paths before a worker changes into its seed directory.
if [[ "$bin" != */* ]]; then
  bin=$(command -v "$bin")
fi
bin=$(cd "$(dirname "$bin")" && printf '%s/%s\n' "$PWD" "$(basename "$bin")")
if [ ! -x "$bin" ]; then
  printf 'not an executable: %s\n' "$bin" >&2
  exit 2
fi
mkdir -p "$work"
work=$(cd "$work" && pwd)
export here mode bin work n limit BORROWFUZZ_CXX="${BORROWFUZZ_CXX:-g++ clang++}"

one() {
  set -euo pipefail
  local seed=$1
  local directory
  directory=$(mktemp -d "$work/$mode$seed.XXXXXXXX")
  mkdir -p "$directory/ref"

  # Keep each stage's evidence, including failed generation and failed reference builds.
  local result=0
  timeout "$limit" python3 -I "$here/gen$mode.py" "$seed" "$n" \
    > "$directory/main.gp" 2> "$directory/generation.err" || result=$?
  printf '%s\n' "$result" > "$directory/generation.status"
  if [ "$result" -ne 0 ]; then
    printf '%s%s GENERATION-FAIL; see %s\n' "$mode" "$seed" "$directory" >&2
    return 1
  fi
  cp "$directory/main.gp" "$directory/ref/main.go"
  printf 'module x\ngo 1.25\n' > "$directory/ref/go.mod"
  result=0
  (cd "$directory/ref" && timeout "$limit" go run .) \
    > "$directory/ref.out" 2> "$directory/ref.err" || result=$?
  printf '%s\n' "$result" > "$directory/ref.status"
  if [ "$result" -ne 0 ]; then
    printf '%s%s REFERENCE-FAIL; see %s\n' "$mode" "$seed" "$directory" >&2
    return 1
  fi

  local failed=0
  local compiler
  local log
  for compiler in $BORROWFUZZ_CXX; do
    log="$directory/$(basename "$compiler")"
    result=0
    (cd "$directory" && GOCPP_CXX="$compiler" GOCPP_GATE_PHASE=15 \
      ASAN_OPTIONS=detect_leaks=0 timeout "$limit" "$bin" run -asan main.gp) \
      > "$log.out" 2> "$log.err" || result=$?
    printf '%s\n' "$result" > "$log.status"

    local verdict=OK
    if [ "$result" -ne 0 ]; then
      verdict=CANDIDATE-FAIL
    elif ! cmp -s "$log.out" "$directory/ref.out" || \
         ! cmp -s "$log.err" "$directory/ref.err"; then
      verdict=DIFF
    fi
    if rg -q 'AddressSanitizer|LeakSanitizer|UndefinedBehaviorSanitizer|ThreadSanitizer|runtime error:' \
        "$log.out" "$log.err"; then
      verdict="$verdict+SANITIZER"
    fi
    printf '%s%s %s %s\n' "$mode" "$seed" "$compiler" "$verdict"
    if [ "$verdict" != OK ]; then
      printf '  see %s\n' "$directory" >&2
      failed=1
    fi
  done
  return "$failed"
}
export -f one
seq "$first" "$last" | xargs -P "$jobs" -I{} bash -c 'one "$1"' _ {}
