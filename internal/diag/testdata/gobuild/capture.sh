#!/bin/sh
# capture.sh runs the real go build on the packages in m/ and saves what it
# prints in want/<name>.txt, one file for each line of cases.txt.
# TestGoBuild checks that the diag package prints the same.
#
# Run it with the dev shell's Go, from the repository root:
#
#	nix develop -c sh internal/diag/testdata/gobuild/capture.sh
#
# Then "git diff" must show no change. The absolute path of m/ is saved as
# $ROOT, so the files are the same in every checkout.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
m=$here/m

# DEEP: a folder inside m/ with as many levels as the path of m/ has
# characters. From there, the relative path to m/ ("../../...") is always
# longer than the absolute one, so go build shows absolute paths.
deep=$m/deep
i=0
while [ "$i" -lt "${#m}" ]; do
	deep=$deep/d
	i=$((i + 1))
done
mkdir -p "$deep"
trap 'rm -rf "$m/deep"' EXIT

go version >"$here/want/go-version.txt"

grep -v -e '^#' -e '^[[:space:]]*$' "$here/cases.txt" |
	while read -r name cwd pkg flags; do
		case $cwd in
		DEEP) dir=$deep ;;
		*) dir=$m/$cwd ;;
		esac
		case $pkg in
		.) path=example.com/m ;;
		*) path=example.com/m/$pkg ;;
		esac
		gcflags=
		if [ "$flags" = -e ]; then
			gcflags=-gcflags=-e
		fi
		# The build fails (the packages have errors), so "|| true".
		(cd "$dir" && GOFLAGS= GOWORK=off GOTOOLCHAIN=local \
			go build -o /dev/null $gcflags "$path" 2>&1 || true) |
			sed "s|$m|\$ROOT|g" >"$here/want/$name.txt"
	done
