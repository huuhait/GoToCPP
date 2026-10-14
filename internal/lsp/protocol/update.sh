#!/bin/sh
# update.sh copies gopls's LSP protocol code into this folder. Run it in the
# dev shell, from any folder:
#
#	nix develop -c internal/lsp/protocol/update.sh [-f] [-gen]
#
# It downloads gopls $GOPLS (default below) into the module cache with
# "go mod download", copies the files listed in COPIED.md §1 from it,
# rewrites the import paths of §2, and runs gofmt, go vet and the tests.
# With the same gopls, a second run changes nothing.
#
# With -gen, it then runs the copied generator (generate/), which clones
# microsoft/vscode-languageserver-node at the tag in generate/main.go
# (lspGitRef), or at the tag LSP_REF when it is set, and writes
# tsprotocol.go and tsjson.go again from its metaModel.json. Only those two
# of the generator's four files are kept: tsclient.go and tsserver.go need
# gopls's jsonrpc2 (COPIED.md §3).
#
# It changes only the copied files. It stops when they have changes that are
# not committed, unless -f is given.

set -eu
export LC_ALL=C
unset CDPATH

GOPLS=${GOPLS:-v0.21.1}

die() {
	echo "update.sh: $*" >&2
	exit 1
}

usage="usage: update.sh [-f] [-gen]"
force=false
gen=false
for arg in "$@"; do
	case $arg in
	-f) force=true ;;
	-gen) gen=true ;;
	*) die "$usage" ;;
	esac
done

GO=${GO:-go}
mod=github.com/huuhait/gocpp
pkg=internal/lsp/protocol

here=$(cd "$(dirname "$0")" && pwd -P) || die "cannot find the folder of the script"
root=$(cd "$here/../../.." && pwd -P) || die "cannot find the repo root"
if [ "$here" != "$root/$pkg" ] || [ ! -f "$here/COPIED.md" ]; then
	die "$here is not gocpp's $pkg"
fi
grep -qx "module $mod" "$root/go.mod" 2>/dev/null || die "$root/go.mod is not the go.mod of $mod"
cd "$root"

# The copied files: upstream path (in the gopls module) and the path here
# (in $pkg). COPIED.md §1 lists the same files.
files="
LICENSE LICENSE
internal/protocol/tsprotocol.go tsprotocol.go
internal/protocol/tsjson.go tsjson.go
internal/protocol/tsdocument_changes.go tsdocument_changes.go
internal/protocol/tsinsertreplaceedit.go tsinsertreplaceedit.go
internal/protocol/form.go form.go
internal/protocol/uri.go uri.go
internal/protocol/uri_test.go uri_test.go
internal/protocol/uri_windows_test.go uri_windows_test.go
internal/protocol/generate/README.md generate/README.md
internal/protocol/generate/generate.go generate/generate.go
internal/protocol/generate/main.go generate/main.go
internal/protocol/generate/main_test.go generate/main_test.go
internal/protocol/generate/output.go generate/output.go
internal/protocol/generate/tables.go generate/tables.go
internal/protocol/generate/typenames.go generate/typenames.go
internal/protocol/generate/types.go generate/types.go
internal/util/pathutil/util.go internal/pathutil/util.go
"

targets=$(echo "$files" | awk 'NF == 2 { print "'"$pkg"'/" $2 }')

if command -v git >/dev/null 2>&1 && git rev-parse --show-toplevel >/dev/null 2>&1; then
	# shellcheck disable=SC2086 # $targets is a list of paths without spaces
	changed=$(git diff HEAD --name-only -- $targets)
	if [ -n "$changed" ] && ! $force; then
		echo "$changed" >&2
		die "the copied files have changes that are not committed (-f overwrites them)"
	fi
fi

dir=$("$GO" mod download -json "golang.org/x/tools/gopls@$GOPLS" |
	sed -n 's/^[[:space:]]*"Dir": "\(.*\)",*$/\1/p') || die "go mod download failed"
[ -d "$dir/internal/protocol" ] || die "gopls $GOPLS has no internal/protocol (in $dir)"
echo "update.sh: upstream is gopls $GOPLS, in $dir"

echo "$files" | while read -r from to; do
	[ -n "$from" ] || continue
	mkdir -p "$(dirname "$pkg/$to")"
	cp "$dir/$from" "$pkg/$to"
	chmod 0644 "$pkg/$to" # the module cache is read-only
done

# The import paths (COPIED.md §2): only in the import declarations, which in
# these files are the lines that are just a quoted path.
sed -i \
	-e 's#^\([[:space:]]*\)"golang.org/x/tools/gopls/internal/util/pathutil"$#\1"'"$mod/$pkg"'/internal/pathutil"#' \
	-e 's#^\([[:space:]]*\)"golang.org/x/tools/gopls/internal/protocol"$#\1"'"$mod/$pkg"'"#' \
	"$pkg/uri.go" "$pkg/uri_test.go" "$pkg/uri_windows_test.go"

if $gen; then
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT
	mkdir "$tmp/out"
	if [ -n "${LSP_REF:-}" ]; then
		# Another tag of vscode-languageserver-node: the generator runs
		# from a copy whose lspGitRef is LSP_REF; generate/ stays as
		# upstream.
		mkdir "$tmp/gen"
		for f in "$pkg"/generate/*.go; do
			case $f in *_test.go) continue ;; esac
			cp "$f" "$tmp/gen/"
		done
		sed -i 's#^var lspGitRef = ".*"$#var lspGitRef = "'"$LSP_REF"'"#' "$tmp/gen/main.go"
		grep -q "^var lspGitRef = \"$LSP_REF\"$" "$tmp/gen/main.go" || die "cannot set lspGitRef in generate/main.go"
		(cd "$tmp/gen" && "$GO" run main.go generate.go output.go tables.go typenames.go types.go -o "$tmp/out")
	else
		"$GO" run "./$pkg/generate" -o "$tmp/out"
	fi
	cp "$tmp/out/tsprotocol.go" "$tmp/out/tsjson.go" "$pkg/"
	echo "update.sh: wrote tsprotocol.go and tsjson.go with the generator"
fi

# Nothing in the copy may import gopls or x/tools.
bad=$("$GO" list -f '{{.ImportPath}}: {{join .Imports " "}} {{join .TestImports " "}}' "./$pkg/..." | grep 'golang.org/x/tools' || true)
[ -z "$bad" ] || die "a copied file still imports x/tools: $bad"

gofmt -l "$pkg" | grep . && die "gofmt: the files above need formatting"
"$GO" vet "./$pkg/..."
"$GO" test "./$pkg/..."
echo "update.sh: done; check git diff -- $pkg and update COPIED.md"
