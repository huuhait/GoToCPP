#!/usr/bin/env bash
# Runs each scenario of scenarios/*.gp through the simulation of the register (etrace_sim.go) and
# compares the output with expected/<name>.txt. "run.sh -update" writes the expected files.
# Needs go and python3 (nix develop -c bash run.sh).
set -eu
here=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
status=0
for gp in "$here"/scenarios/*.gp; do
	name=$(basename "$gp" .gp)
	if rg -q '^// sim: skip' "$gp"; then
		echo "skip $name (macros: the plain-Go simulation cannot expand them)"
		continue
	fi
	dir="$tmp/$name"
	mkdir -p "$dir"
	sed '1,2d' "$here/etrace_sim.go" > "$dir/etrace_sim.go"
	python3 -I "$here/gp2go.py" "$gp" > "$dir/$name.go"
	printf 'module sim\n\ngo 1.25\n' > "$dir/go.mod"
	got=$(cd "$dir" && go run -gcflags=-l . 2>&1 | sed "s/$name\.go:/$name.gp:/")
	if [ "${1:-}" = "-update" ]; then
		printf '%s\n' "$got" > "$here/expected/$name.txt"
	elif [ "$got" != "$(cat "$here/expected/$name.txt")" ]; then
		echo "FAIL $name"
		diff <(printf '%s\n' "$got") "$here/expected/$name.txt" || true
		status=1
	else
		echo "ok   $name"
	fi
done
exit $status
