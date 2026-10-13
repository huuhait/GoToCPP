#!/bin/sh
# check.sh checks the hand-written output.txt of the interop examples (see
# examples/README.md). For cpp-std, sqlite, raylib, mixed-sources and win32 it
# builds a C++ program that makes the same calls as the example; for
# multi-pkg it builds the same packages as a Go module, with package codes
# using cgo on the example's own luhn.c. It runs each one the way the example
# runs and compares the output with the example's output.txt. The C++ checks
# run with g++ and with clang++.
#
# Run it from the dev shell:
#
#	nix develop -c sh tools/gen-example-output/testdata/interop/check.sh
#
# raylib also needs xvfb-run, because it opens a (hidden) window, for example
# from "nix shell nixpkgs#xvfb-run". win32 needs x86_64-w64-mingw32-g++ and
# wine, which the dev shell has; Wine runs with a prefix of its own and no
# display (its "null" graphics driver), and is stopped afterwards. Both GUI
# checks run with a scratch home folder (use_scratch_home), because Wine,
# fontconfig and the GL drivers write desktop files and caches into the home
# folder. A check whose tools are missing is skipped with a message.
set -u

here=$(cd "$(dirname "$0")" && pwd)
ex=$(cd "$here/../../../../examples" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
failed=0

have() { command -v "$1" >/dev/null 2>&1; }

# use_scratch_home points HOME and the XDG folders at a folder in $tmp and
# removes the display variables. Call it in a subshell: then Wine,
# fontconfig, Mesa and the NVIDIA driver write their files there (Wine writes
# desktop entries into the home folder whatever WINEPREFIX says), and no
# window shows up on the desktop.
use_scratch_home() {
	export HOME="$tmp/home" XDG_DATA_HOME="$tmp/home/.local/share" \
		XDG_CONFIG_HOME="$tmp/home/.config" XDG_CACHE_HOME="$tmp/home/.cache"
	mkdir -p "$XDG_DATA_HOME" "$XDG_CONFIG_HOME" "$XDG_CACHE_HOME"
	unset DISPLAY WAYLAND_DISPLAY
}

skip() { echo "skip  $1: $2"; }

# compare NAME HOW FILE: FILE must be the same as examples/NAME/output.txt.
compare() {
	if cmp -s "$3" "$ex/$1/output.txt"; then
		echo "ok    $1 ($2)"
	else
		echo "FAIL  $1 ($2): the output is not examples/$1/output.txt:"
		diff "$ex/$1/output.txt" "$3" | sed 's/^/      /'
		failed=1
	fi
}

# fail NAME HOW: a check that didn't build or run.
fail() {
	echo "FAIL  $1 ($2): build or run failed"
	failed=1
}

for cxx in g++ clang++; do
	case $cxx in
	g++) cc=gcc ;;
	clang++) cc=clang ;;
	esac
	if ! have $cxx || ! have $cc; then
		skip "the C++ checks" "no $cxx or $cc"
		continue
	fi
	o=$tmp/$cxx
	mkdir -p "$o"

	if $cxx -std=c++20 -o "$o/cpp-std" "$here/cpp-std.cpp" && "$o/cpp-std" >"$o/cpp-std.txt"; then
		compare cpp-std $cxx "$o/cpp-std.txt"
	else
		fail cpp-std $cxx
	fi

	if ! pkg-config --exists sqlite3 2>/dev/null; then
		skip sqlite "pkg-config finds no sqlite3"
	elif $cxx -std=c++20 -o "$o/sqlite" "$here/sqlite.cpp" $(pkg-config --cflags --libs sqlite3) &&
		"$o/sqlite" >"$o/sqlite.txt"; then
		compare sqlite $cxx "$o/sqlite.txt"
	else
		fail sqlite $cxx
	fi

	m=$ex/mixed-sources
	if $cc -std=c17 -c -o "$o/checksum.o" "$m/checksum.c" &&
		$cxx -std=c++20 -c -o "$o/textutil.o" "$m/textutil.cpp" &&
		$cxx -std=c++20 -I"$m" -o "$o/mixed" "$here/mixed-sources.cpp" "$o/checksum.o" "$o/textutil.o" &&
		"$o/mixed" >"$o/mixed.txt"; then
		compare mixed-sources "$cxx, $cc" "$o/mixed.txt"
	else
		fail mixed-sources $cxx
	fi

	if ! pkg-config --exists raylib 2>/dev/null; then
		skip raylib "pkg-config finds no raylib"
	elif ! $cxx -std=c++20 -o "$o/raylib" "$here/raylib.cpp" $(pkg-config --cflags --libs raylib); then
		fail raylib $cxx
	elif ! have xvfb-run; then
		skip raylib "built with $cxx, but there is no xvfb-run to run it without a desktop"
	elif (use_scratch_home && GOCPP_SMOKE=1 xvfb-run -a "$o/raylib") >"$o/raylib.txt"; then
		compare raylib "$cxx, smoke mode under Xvfb" "$o/raylib.txt"
	else
		fail raylib $cxx
	fi
done

# multi-pkg: the example's packages as a Go module; codes uses cgo.
if ! have go; then
	skip multi-pkg "no go command"
else
	mp=$tmp/multi-pkg
	mkdir -p "$mp/catalog" "$mp/codes" "$mp/geom"
	printf 'module example.com/multi-pkg\n\ngo 1.25\n' >"$mp/go.mod"
	cp "$ex/multi-pkg/main.gp" "$mp/main.go"
	cp "$ex/multi-pkg/catalog/catalog.gp" "$mp/catalog/catalog.go"
	cp "$ex/multi-pkg/geom/geom.gp" "$mp/geom/geom.go"
	cp "$ex/multi-pkg/codes/luhn.c" "$ex/multi-pkg/codes/luhn.h" "$here/multi-pkg/codes.go" "$mp/codes/"
	if (cd "$mp" && CGO_ENABLED=1 GOWORK=off GOTOOLCHAIN=local go build -o multi-pkg .) &&
		"$mp/multi-pkg" >"$mp/out.txt"; then
		compare multi-pkg "go with cgo" "$mp/out.txt"
	else
		fail multi-pkg go
	fi
fi

# win32: built with MinGW-w64 (-static, and the defines gocpp uses), run in
# smoke mode under Wine.
if ! have x86_64-w64-mingw32-g++ || ! have wine; then
	skip win32 "no x86_64-w64-mingw32-g++ or no wine"
elif ! x86_64-w64-mingw32-g++ -std=c++20 -DUNICODE -D_UNICODE -DWIN32_LEAN_AND_MEAN -DNOMINMAX -static \
	-o "$tmp/win32.exe" "$here/win32.cpp" -luser32 -lgdi32; then
	fail win32 mingw
else
	(
		use_scratch_home
		export WINEPREFIX="$tmp/wine" WINEDEBUG=-all WINEDLLOVERRIDES="winemenubuilder.exe=d;mscoree,mshtml="
		wine reg add 'HKCU\Software\Wine\Drivers' /v Graphics /d null /f >/dev/null 2>&1
		GOCPP_SMOKE=1 wine "$tmp/win32.exe" >"$tmp/win32.txt" 2>/dev/null
		status=$?
		wineserver -k
		exit $status
	) && compare win32 "mingw g++, smoke mode under Wine" "$tmp/win32.txt" || fail win32 wine
fi

exit $failed
