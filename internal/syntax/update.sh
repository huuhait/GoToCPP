#!/bin/sh
# update.sh makes the parser fork again, from the Go in PATH. Run it in the
# dev shell, from any folder:
#
#	nix develop -c internal/syntax/update.sh [-f]
#
# It copies go/scanner, go/parser and go/printer from
# $(go env GOROOT)/src, rewrites the import paths, applies local.patch and
# then patches/*.patch with git apply, and runs the checks and the tests.
#
# It changes only scanner/, parser/, printer/ and LICENSE in this
# folder: it deletes the three folders and makes them again. So it stops when
# they have changes that are not committed. With -f it goes on, but first it
# saves a copy of them in a new temporary folder. With the same Go, a second
# run changes nothing. FORK.md explains each step and how to make a patch.
#
# GO and GOFMT choose other tools: GO=/path/to/go (default: go in PATH) and
# GOFMT=/path/to/gofmt (default: the gofmt of that Go).

set -eu
export LC_ALL=C # patches/*.patch are sorted byte by byte
unset CDPATH    # with CDPATH, cd can print the folder and break $(cd ...)

die() {
	echo "update.sh: $*" >&2
	exit 1
}

usage="usage: update.sh [-f]"
force=false
while getopts f opt; do
	case $opt in
	f) force=true ;;
	*) die "$usage" ;;
	esac
done
shift $((OPTIND - 1))
[ $# -eq 0 ] || die "$usage"

GO=${GO:-go}
mod=github.com/huuhait/gocpp
syn=internal/syntax

# The script deletes folders, so first make sure that it is in gocpp's
# internal/syntax. Then it works from the repo root, because the patch files
# name their files from there.
here=$(cd "$(dirname "$0")" && pwd -P) || die "cannot find the folder of the script"
root=$(cd "$here/../.." && pwd -P) || die "cannot find the repo root"
if [ "$here" != "$root/$syn" ] || [ ! -f "$here/FORK.md" ] || [ ! -f "$here/local.patch" ]; then
	die "$here is not gocpp's $syn"
fi
grep -qx "module $mod" "$root/go.mod" 2>/dev/null || die "$root/go.mod is not the go.mod of $mod"
cd "$root"

# The folders and files that update.sh deletes and writes must be real ones.
# Through a symbolic link, it would change things outside the fork.
no_links() {
	for p in scanner parser printer LICENSE; do
		if [ -L "$syn/$p" ]; then
			die "$syn/$p is a symbolic link; update.sh only changes real files and folders"
		fi
	done
}
no_links

# git: use only a repository whose top folder is the gocpp root. In a gocpp
# folder inside another repository, git apply would take the patch paths
# from the top of that repository and skip them all, without an error.
command -v git >/dev/null 2>&1 || die "git is needed (for git apply)"
unset GIT_DIR GIT_WORK_TREE
GIT_CEILING_DIRECTORIES=$(dirname "$root")
export GIT_CEILING_DIRECTORIES
top=$(git rev-parse --show-toplevel 2>/dev/null) && top=$(cd "$top" && pwd -P) || top=

goroot=$("$GO" env GOROOT) || die "\"$GO env GOROOT\" failed"
version=$("$GO" env GOVERSION) || die "\"$GO env GOVERSION\" failed"
GOFMT=${GOFMT:-$goroot/bin/gofmt}
src=$goroot/src
echo "update.sh: upstream is $version, in $src"
for f in go/scanner go/parser go/printer; do
	[ -e "$src/$f" ] || die "$src/$f does not exist"
done

# Changes that are not committed in what update.sh replaces would be lost, so
# stop, unless -f is given. With -f, save a copy first.
set -- "$syn/scanner" "$syn/parser" "$syn/printer" "$syn/LICENSE"
if [ "$top" != "$root" ]; then
	$force || die "$root is not the top folder of a git repository (a clone or a worktree),
so update.sh can't see if $syn has changes that are not committed.
Run update.sh -f to go on: it saves a copy of the old files first."
	echo "update.sh: -f: $root is not the top folder of a git repository; going on"
else
	changes=$(git status --porcelain -- "$@") || changes="(git status failed)"
	if [ -n "$changes" ]; then
		$force || die "these files have changes that are not committed:
$changes
update.sh deletes scanner/, parser/ and printer/ in $syn and makes
them again, and it can replace $syn/LICENSE, so these changes would be lost.
Commit them, or put them in a patch file (FORK.md §7).
Or run update.sh -f: it saves a copy of the old files first."
		echo "update.sh: -f: these changes are not committed, the copy keeps them:
$changes"
	fi
fi
backup=
if $force; then
	backup=$(mktemp -d) || die "mktemp -d failed"
	for p in "$@"; do
		if [ -e "$p" ]; then
			mkdir -p "$backup/${p%/*}"
			cp -R "$p" "$backup/$p"
		fi
	done
	echo "update.sh: -f: saved a copy of the old files in $backup"
fi

# 1. Copy. The files in the Nix store are read-only, so the copies get the
# normal modes: then the next run can delete them, and git sees no change.
for d in scanner parser printer; do
	if [ -e "$syn/$d" ]; then
		chmod -R u+w "$syn/$d"
		rm -rf "${syn:?}/$d"
	fi
done
cp -R "$src/go/scanner" "$syn/scanner"
cp -R "$src/go/parser" "$syn/parser"
cp -R "$src/go/printer" "$syn/printer"
no_links # cp -R copies a linked folder of GOROOT as a link
find "$syn/scanner" "$syn/parser" "$syn/printer" -type d -exec chmod 755 {} +
find "$syn/scanner" "$syn/parser" "$syn/printer" -type f -exec chmod 644 {} +

# Go's LICENSE is in GOROOT, not in GOROOT/src. Nix's Go has none; then the
# LICENSE file here stays as it is.
if [ -f "$goroot/LICENSE" ]; then
	cp "$goroot/LICENSE" "$syn/LICENSE"
	chmod 644 "$syn/LICENSE"
elif [ -f "$syn/LICENSE" ]; then
	echo "update.sh: $goroot has no LICENSE file (Nix's Go has none), so $syn/LICENSE stays"
else
	die "there is no LICENSE file in $goroot or in $syn"
fi

# 2. Rewrite the import paths of the three packages, only in the import
# declarations (an "import (" block or an "import" line) of their own Go
# files. Go allows imports only before all other declarations, so the
# rewrite stops at the first func, var, const or type line: a Go file that a
# test has in a string is always inside a declaration, so it stays as it is.
# The test inputs in testdata/ stay as they are too. gofmt then sorts the
# import block again, and go list checks that no stock import is left.
blk='/^import (/,/^)/'
one='/^import [^(]/'
sc="s|\"go/scanner\"|\"$mod/$syn/scanner\"|"
pa="s|\"go/parser\"|\"$mod/$syn/parser\"|"
pr="s|\"go/printer\"|\"$mod/$syn/printer\"|"
n=0
for f in "$syn"/scanner/*.go "$syn"/parser/*.go "$syn"/printer/*.go; do
	sed -e "/^func /,\$b" -e "/^var /,\$b" -e "/^const /,\$b" -e "/^type /,\$b" \
		-e "$blk$sc" -e "$blk$pa" -e "$blk$pr" -e "$one$sc" -e "$one$pa" -e "$one$pr" "$f" >"$f.tmp"
	if cmp -s "$f" "$f.tmp"; then
		rm -f "$f.tmp"
		continue
	fi
	mv "$f.tmp" "$f"
	chmod 644 "$f"
	"$GOFMT" -w "$f"
	n=$((n + 1))
done
echo "update.sh: rewrote the import paths in $n files"
imports=$("$GO" list -f '{{join .Imports "\n"}}{{"\n"}}{{join .TestImports "\n"}}{{"\n"}}{{join .XTestImports "\n"}}' \
	"./$syn/scanner" "./$syn/parser" "./$syn/printer") || die "go list failed"
if printf '%s\n' "$imports" | grep -qx -e go/scanner -e go/parser -e go/printer; then
	die "the fork still imports the stock go/scanner, go/parser or go/printer"
fi

# 3. Apply local.patch (the local test changes), then the patches in name
# order. A patch may only change, add or delete files in scanner/,
# parser/ and printer/. No renames or copies (git apply --numstat names only their new
# file), and no symbolic links.
apply() {
	summary=$(git apply --summary "$1") || die "$1 is not a valid patch"
	if printf '%s\n' "$summary" | grep -q -e '^ rename ' -e '^ copy ' -e ' 120000 '; then
		die "$1 renames or copies a file, or makes a symbolic link.
A patch may only change, add or delete files. For a rename or a copy, make
the patch with: git diff --no-renames"
	fi
	stat=$(git apply --numstat "$1") || die "$1 is not a valid patch"
	files=$(printf '%s\n' "$stat" | cut -f 3)
	while IFS= read -r f; do
		case $f in
		'') ;;
		*..*) die "$1 changes \"$f\": a path with \"..\" is not allowed" ;;
		"$syn"/scanner/?* | "$syn"/parser/?* | "$syn"/printer/?*) ;;
		*) die "$1 changes \"$f\", but a patch may only change $syn/scanner/, $syn/parser/ and $syn/printer/" ;;
		esac
	done <<EOF
$files
EOF
	git apply --whitespace=nowarn "$1" || die "$1 does not apply; FORK.md says how to fix it"
	echo "update.sh: applied $1"
}
apply "$syn/local.patch"
for p in "$syn"/patches/*.patch; do
	[ -e "$p" ] || continue # no patches (yet)
	apply "$p"
done

# 4. Check and test. gofmt checks the Go files of the three packages (not
# testdata/). The benchmarks run once each, because local.patch changes
# BenchmarkScanFiles.
bad=$(find "$syn/scanner" "$syn/parser" "$syn/printer" -name testdata -prune -o -type f -name '*.go' -exec "$GOFMT" -l {} +) ||
	die "gofmt failed"
[ -z "$bad" ] || die "gofmt: these files need formatting (gofmt -w):
$bad"
"$GO" vet "./$syn/..." || die "go vet failed"
"$GO" test "./$syn/..." || die "the tests failed"
out=$("$GO" test -run='^$' -bench=. -benchtime=1x "./$syn/..." 2>&1) || {
	printf '%s\n' "$out"
	die "the benchmarks failed"
}
echo "update.sh: each benchmark ran once"

grep -qwF "$version" "$syn/FORK.md" ||
	echo "update.sh: FORK.md does not name $version yet: update FORK.md and doc.go"
[ -z "$backup" ] || echo "update.sh: the copy of the old files is in $backup"
echo "update.sh: done. Read the changes with: git diff -- $syn"
