#!/bin/sh
# mkgolden.sh makes the .golden files in testdata/gofmt again. Each one is
# what the real gofmt prints for the .input file with the same name, and
# TestGofmtInputs checks that format.Source prints exactly the same.
#
# Run it after you add or change an .input file, and when the dev shell
# gets a new Go (then look at the changes with git diff):
#
#	nix develop -c internal/format/mkgolden.sh
#
# It works from any folder. It uses the gofmt of the go command in PATH;
# set GOFMT to use another one.
set -eu

dir=$(cd "$(dirname "$0")" && pwd)/testdata/gofmt
gofmt=${GOFMT:-$(go env GOROOT)/bin/gofmt}
echo "mkgolden.sh: using $gofmt"

for input in "$dir"/*.input; do
	# gofmt formats a file that is named on the command line whatever its
	# extension is, and prints the result.
	"$gofmt" "$input" >"${input%.input}.golden"
done
echo "mkgolden.sh: wrote the .golden files in $dir"
