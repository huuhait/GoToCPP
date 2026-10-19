# ADR-082: Windows interop: Win32 defaults as package flags, wide strings that work everywhere

Status: accepted (2026-10-19). Carries out [Phase 14 §2.4](../../plans/2026-10-13-gocpp/phase-14-windows/README.md#24-interop-with-windows-headers)
(step 14.4).

## Context

`<windows.h>` is a header package like any other, but with its defaults it breaks C++ code: the macros `min` and `max`, `winsock.h`
pulled in next to `winsock2.h` (a MinGW warning), `A` functions behind names like `MessageBox`. The plan said gocpp defines `WIN32_LEAN_AND_MEAN`,
`NOMINMAX` and `UNICODE` and provides `C.wstr` and `C.GoStringW`, and that the macro protection of Phase 04 handles the rest. Step 14.4 had to
decide where the defaults live (the runtime must not get them), how a user changes them, what `C.wstr` is where `wchar_t` is 32 bits, and what
else Windows needs from interop. Everything was tried on a real Windows 11 PC over ssh.

## Decision

1. **The defaults are compile flags of the package** (`build.Win32Defaults`, called by `ReadPackageFlags`): `-DWIN32_LEAN_AND_MEAN -DNOMINMAX
   -DUNICODE -D_UNICODE`, first in the package's flags, when the package's target is windows (`load.Package.GOOS`, new) and it imports a header or has
   C or C++ files. Because they are package flags, every consumer gets the same view without a second mechanism: the generated C++, the kind
   probe and the macro probe (a `min` that is no macro in the build is none in the probe), the namespace probe of `check`, and the package's own
   `.c`/`.cpp` files (`#ifndef NOMINMAX #error` in `winstruct.c` proves it). The runtime's compiles and the sources of a `cxx` recipe do not
   get them. The cost: a package with package flags does not use the runtime's precompiled header, so a Windows package that imports a header
   builds without it.
2. **Change them macro by macro.** `//gocpp:no-win32-defaults` (the directive that Phase 01 reserved) turns all four off. A flag of the user
   that defines or undefines one (`-DNAME`, `-DNAME=v`, `-UNAME`, `-D NAME`) in `//gocpp:cflags`, pkg-config's output, the public flags of a recipe, or
   `GOCPP_CFLAGS`/`GOCPP_CXXFLAGS` makes gocpp leave that macro out (gocpp cannot come *after* the user's flags, because the order
   of `-D`/`-U` would then override the user). `_UNICODE` is added to the plan's list: `<tchar.h>` follows it.
3. **No new macro-collision rule.** The macro protection of Phase 04 (a name of the package that is a macro of the headers gets a `_`; the
   other macros that the generated code uses are `#undef`ed) is enough. The header zoo (`tests/testdata/windows/winmacros.gp`) declares a
   constant, a variable, a type, a field, a method and a function for each of: `ERROR`, `DELETE`, `CONST`, `TRUE`, `FALSE`, `MAX_PATH`,
   `INFINITE`, `near`, `far`, `small`, `IN`, `OUT`, `OPTIONAL`, `VOID`, `GetObject`, `SendMessage`, `CreateWindow`, `DrawText`, `LoadImage`, `RGB`,
   `min` and `max`, next to `windows.h`: all were renamed correctly. `interface` is a Go keyword and cannot collide.
4. **`C.wstr`, `C.GoStringW`, and the new `C.WString` and `C.GoStringWN`, work on every target.** The unit is `wchar_t`: UTF-16 on Windows, UTF-32 on Linux. That makes
   the logic testable on Linux (`runtime/test/interop_test.cpp`, `tests/interop/wstr_wchar.gp`) as well as on the PC. A compile error on other targets would
   have hidden the code from the sanitizer jobs, and `wchar.h` APIs on Linux want the same helpers. The conversions are Go's
   (`syscall.UTF16FromString` and `UTF16ToString` with WTF-8): an invalid byte is U+FFFD, a WTF-8 surrogate is a lone unit and returns as the same three
   bytes, a non-BMP character is a pair, a NUL ends the C string. `C.wstr` is a holder (`gp::wstr_holder`, `gocpp/wstr.hpp`) under `C.cstr`'s rules
   (same placement checks, `.gp_ptr()` through `...`), but always owns a copy. `C.WString` is `C.CString` for wide characters (a `malloc`ed copy, `C.free`): the plan's example kept a holder
   in a variable for `WNDCLASSW.lpszClassName`, which the placement checks of `C.cstr` refuse for a struct field, so a retained copy is the honest form. `C.GoStringW`'s pointer is opaque to the checker (any pointer to 2-byte or 4-byte
   integers: `LPCWSTR`, `&buf[0]` of a `[260]windows.WCHAR`, `*uint16`). The gate of Phase 14 stays until the phase is finished.
5. **`C.wchar_t` is a built-in C type, and the ranges of C types follow the target.** The constant checks of `lower/cconsts.go` assumed LP64 (`C.long` 64 bits):
   on Windows `C.long` and `C.ulong` are 32 bits and `C.wchar_t` is an unsigned 16-bit unit (`gen.cConstType`).
6. **Libraries**: `//gocpp:link` and `//gocpp:ldflags` work as on Linux. Programs are linked `-static`, which skips import libraries (`libSDL2.dll.a`): name the
   file with `//gocpp:ldflags -l:libSDL2.dll.a`. MinGW has no `#pragma comment(lib)`.
7. **Libraries that MinGW does not ship** (sqlite3, zlib, SDL2, raylib): `tools/winlibs.sh DIR` builds them for MinGW-w64 in the nix shell (zlib, SDL2 and raylib
   from nixpkgs' cross builds; sqlite3 from the amalgamation, compiled with the shell's gcc, because nixpkgs' MinGW sqlite needs a Windows build of Tcl that
   does not build); the tests find them with `GOCPP_WINDOWS_LIBS=DIR` (`-I DIR/include -L DIR/lib` as `GOCPP_CFLAGS`/`GOCPP_LDFLAGS`) and skip without it. sqlite3 and zlib run on the
   PC; SDL2 and raylib are import libraries of DLLs and are only linked.

## Consequences

- A Windows program needs no `-D` flags to use `<windows.h>` with `<winsock2.h>`, `std::min` or `MessageBox`.
- `//gocpp:no-win32-defaults` and the `-U` route cover programs that want the old defaults, and `MessageBoxA`.
- Not done here, and named for the steps that own it: the hidden typed probe (`gocpp __typed-probe`, LLP64) cannot parse MinGW's headers (the clang of the
  dev shell has no MinGW include paths, and `GOCPP_CXXFLAGS` is shared with the g++ kind probe): step 14.6. What was found for it: clang parses `<windows.h>`
  with `--target=x86_64-w64-mingw32`, the MinGW include folders of the cross compiler (`x86_64-w64-mingw32-g++ -E -v`: libstdc++, `sys-include`, mingw-w64's
  `include`, mcfgthread, winpthreads) and clang's own resource headers, but not the cross gcc's `lib/gcc/.../include` (its `x86intrin.h` fails in clang), so
  `-nostdinc` must not be used; with these, `MAX_PATH == 260`, `sizeof(long) == 4`, `sizeof(DWORD) == 4`, `sizeof(HANDLE) == 8` and `sizeof(wchar_t) == 2` hold. `INVALID_HANDLE_VALUE`
  (`((HANDLE)(LONG_PTR)-1)`) is not a constant expression, so the probe must answer "not a constant" for it: it stays a C value (`h == windows.INVALID_HANDLE_VALUE`, as today). A macro in an array length (`[windows.MAX_PATH]windows.WCHAR`)
  is Phase 12 step 12.8 (a clean error that names it and the workaround `[260]` today). SDL2 and raylib are not run (DLLs next to the program and a display).
  UCRT (llvm-mingw, MSYS2) is not tested; native builds on the PC are step 14.2 ([ADR-081](081-windows-host-build-driver.md)).

## Review round (2026-10-19)

- `tools/winlibs.sh` checks the sha256 of the one file it downloads (the versioned sqlite amalgamation); the rest comes from nixpkgs, pinned by `flake.lock`.
- Two hints: after a C++ error that mentions `wchar_t`, `LPCSTR`, `TCHAR` or `UNICODE`, a line that says whether the package got the Win32 defaults and
  how to change them (`cmd/gocpp/win32hints.go`); after a link error with undefined `__imp_` symbols, `add //gocpp:link ws2_32` for the common libraries.
- The refused places of `C.wstr` name `C.WString` as the way to keep a string; `C.GoStringW` and `C.GoStringWN` check the kind of pointer in `check`.
- Known limits, listed rather than fixed: a function pointer resolved at run time (`GetProcAddress`) cannot be called from Go code; a macro named only in
  `GOCPP_CXXFLAGS` also drops the default for the package's C files, and the other way round (the default is one decision per package); a library with a `cxx`
  recipe does not get the defaults; the typed probe on a Windows target is step 14.6.
