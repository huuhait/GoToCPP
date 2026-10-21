#!/usr/bin/env bash
# Run inside the offline dev shell, under testslot.sh. WORK must not exist.
set -euo pipefail
ROOT=$(git -C "$(dirname "$0")" rev-parse --show-toplevel)
WORK=$(realpath -m "${1:?usage: prepare.sh NEW_WORK_DIRECTORY}")
MODSOURCE=${2:-$(go env GOMODCACHE)}
mkdir "$WORK"
export GOCPP_MODCACHE=off GOCPP_OFFLINE=1 GOCPP_TEST_TIMEOUT=6m GOCPP_GATE_PHASE=15
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local
export GOCACHE="$WORK/go-cache" GOMODCACHE="$WORK/go-mod"
export GOCPP_NOOBJCACHE=1 GOCPP_NOUPTODATE=1
mkdir -p "$GOMODCACHE/golang.org/x" "$GOMODCACHE/cache/download/golang.org/x/tools/@v"
# The snapshot has one Go dependency; copy only an already installed module/cache.
cp -a "$MODSOURCE/golang.org/x/tools@v0.44.0" "$GOMODCACHE/golang.org/x/"
cp "$MODSOURCE"/cache/download/golang.org/x/tools/@v/v0.44.0.* \
    "$GOMODCACHE/cache/download/golang.org/x/tools/@v/"
mkdir -p "$WORK/input/t20" "$WORK/input/hello"
cd "$ROOT"
git rev-parse HEAD > "$WORK/revision.txt"
timeout 360s go build -o "$WORK/gocpp" ./cmd/gocpp
python3 -I "$ROOT/tools/genericsgen/ctime_gen.py" t 20 10 "$WORK/input/t20/main.gp"
printf 'package main\nfunc main() { println("hello") }\n' > "$WORK/input/hello/main.gp"
for compiler in g++ clang++; do
    name=gcc
    if [[ "$compiler" == clang++ ]]; then name=clang; fi
    mkdir -p "$WORK/$name/tmp"
    export TMPDIR="$WORK/$name/tmp" XDG_CACHE_HOME="$WORK/$name/cache"
    export GOCPP_CXX="python3 -I $ROOT/tools/modules-experiment/capture.py $(command -v "$compiler")"
    export MODULE_CAPTURE="$WORK/$name/commands.jsonl"
    "$compiler" --version > "$WORK/$name/version.txt"
    "$compiler" -dumpmachine >> "$WORK/$name/version.txt"
    for workload in hello t20; do
        timeout 360s "$WORK/gocpp" build -work -x -p 4 \
            -o "$WORK/$name/$workload.bin" "$WORK/input/$workload/main.gp" \
            > "$WORK/$name/$workload.stdout" 2> "$WORK/$name/$workload.trace"
        "$WORK/$name/$workload.bin" > "$WORK/$name/$workload.output" \
            2> "$WORK/$name/$workload.error"
    done
done
sha256sum "$WORK/gocpp" "$WORK/input"/*/main.gp > "$WORK/identities.txt"
