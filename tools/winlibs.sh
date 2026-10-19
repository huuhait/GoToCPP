#!/bin/sh
# winlibs.sh DIR: builds the libraries that the Windows interop tests use (sqlite3, zlib, SDL2, raylib) for
# MinGW-w64 into DIR/include and DIR/lib, with the nix dev shell. Step 14.4 (ADR-082). Run it inside the dev shell
# (nix develop) from the repository root:
#
#	nix develop -c sh tools/winlibs.sh /tmp/winlibs
#	GOCPP_WINDOWS_LIBS=/tmp/winlibs GOCPP_WINDOWS_SSH=user@host GOCPP_TEST_TARGET=windows \
#	    go test -run 'TestWindowsPrograms/lib' ./tests
#
# zlib, SDL2 (sdl2-compat) and raylib come from nixpkgs' MinGW-w64 cross builds (nixpkgs' sqlite for MinGW needs a
# Windows build of Tcl, which does not build). sqlite3 is the amalgamation from sqlite.org compiled here with the
# MinGW-w64 gcc of the shell. zlib and sqlite3 are static libraries; SDL2 and raylib are import libraries of DLLs
# (the tests only link them). It needs network access once, for one download: the sqlite3 amalgamation, a versioned
# file whose sha256 is pinned below and checked before it is unpacked. Everything else comes from nixpkgs, which the
# flake.lock of the repository pins (nix checks the hashes of what it fetches).
set -eu
dir=${1:?usage: winlibs.sh DIR}
sqlite_version=3510200
sqlite_year=2026
sqlite_sha256=6e2a845a493026bdbad0618b2b5a0cf48584faab47384480ed9f592d912f23ec # sqlite-amalgamation-3510200.zip
mkdir -p "$dir/include" "$dir/lib"
out() { nix build --inputs-from . "nixpkgs#pkgsCross.mingwW64.$1" --no-link --print-out-paths | head -n 1; }
zlib=$(out zlib)
zlib_dev=$(out 'zlib^dev')
sdl=$(out SDL2)
sdl_dev=$(out 'SDL2^dev')
ray=$(out raylib)
cp -rL "$zlib_dev"/include/* "$dir/include/"
cp -L "$zlib"/lib/libz.a "$zlib"/lib/libz.dll.a "$dir/lib/"
cp -rL "$sdl_dev"/include/* "$dir/include/"
cp -L "$sdl"/lib/libSDL2.dll.a "$sdl"/lib/libSDL2main.a "$dir/lib/"
cp -L "$ray"/include/* "$dir/include/"
cp -L "$ray"/lib/libraylib.dll.a "$dir/lib/"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
curl -sSfL -o "$work/sqlite.zip" "https://www.sqlite.org/$sqlite_year/sqlite-amalgamation-$sqlite_version.zip"
got=$(sha256sum "$work/sqlite.zip" | cut -d' ' -f1)
if [ "$got" != "$sqlite_sha256" ]; then
	echo "winlibs.sh: sqlite-amalgamation-$sqlite_version.zip has sha256 $got, want $sqlite_sha256" >&2
	exit 1
fi
(cd "$work" && unzip -q sqlite.zip && cd "sqlite-amalgamation-$sqlite_version" &&
	x86_64-w64-mingw32-gcc -O1 -DSQLITE_THREADSAFE=1 -c sqlite3.c -o sqlite3.o &&
	x86_64-w64-mingw32-ar rcs libsqlite3.a sqlite3.o &&
	cp libsqlite3.a "$dir/lib/" && cp sqlite3.h sqlite3ext.h "$dir/include/")
chmod -R u+w "$dir"
