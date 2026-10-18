# stdlib

The gocpp standard library: `fmt`, `strings`, `strconv`, `os`, `errors`, `sort` and the rest, as
`.gp` files, plus native `.cpp` files where a package needs the system. Like the runtime, it is
embedded in the `gocpp` binary, and the loader finds the standard packages here.

## What is here now

- **Real packages** (Phase 05 and later steps). `errors` since step 5.7: `errors.gp`, `wrap.gp` and `join.gp` are
  Go's `src/errors`, lowered with the program's packages like any package (`PORT.md` lists what differs from Go's
  source). A package is real when none of its functions is a declaration stub (below): it is built from its `.gp`
  files, and `tools/stubgen` writes no stub for it (`stubPackage.Real`; the tool still makes the stub in memory,
  because the tests compare the real package's exported declarations with it and with Go's).
- **Ported packages** (Phase 09 step 9.1). `unicode/utf8`, `unicode/utf16` and `math/bits` are Go's source, copied by
  `tools/portstd` from the pinned Go tarball: the `.gp` files start with `// Code ported from Go ...; DO NOT EDIT by
  hand`, `stdlib/<pkg>/PORT.md` says what was taken, left out and changed, and `stdlib/PORTED.txt` lists them for
  tools. A change goes into a patch rule of the tool, not into the file (`make portstd-check` fails otherwise); how
  to port a package is in [docs/guides/porting.md](../docs/guides/porting.md), and what blocks the tier-1 packages in
  [porting-blockers.md](../docs/guides/porting-blockers.md). `unicode/utf8` is a real package: `tools/stubgen` lists it as
  `Real` and writes no stub for it. `LICENSE` here is Go's BSD-3-Clause license, which the ported files carry; Go's
  `PATENTS` file (the additional patent grant) is not copied: it grants rights to users of Go itself, and the ported
  text is the same, so keep a copy of `PATENTS` from the pinned tarball with any redistribution of `stdlib/`.
- **Native functions.** A function without a body that has a `//gocpp:native` line in its doc comment is written in
  C++: the line names the runtime function that its calls become (ADR-027).

  ```go
  // comparableType reports whether the dynamic type of err can be compared with ==.
  //
  //gocpp:native gp::errors::comparable
  func comparableType(err error) bool
  ```

  The C++ function is declared in a runtime header that `gocpp/rt.hpp` includes (`gocpp/errors.hpp`) and defined in
  `runtime/src/`, takes the C++ types of the Go parameters, and is called as `::gp::errors::comparable(err)` from
  every package that calls the Go function. Only the standard library may use the directive, only on plain functions
  (no methods, no generics), and the loader checks the name. A bodyless function **without** the directive is a
  stub: its package is not lowered, and a program that calls it gets "not supported yet". Phase 09 will add the
  form without an argument, for a body in a `.cpp` file next to the `.gp` file.
- **Declaration stubs** (Phase 01) for 24 packages, one file each (`fmt/fmt.gp`, `encoding/json/json.gp`):
  every standard package that the target programs in `examples/` import (also in their
  `_test.gp` files and in the multi-pkg example's own packages), plus the packages that those
  stubs need (`io/fs` and `time` for `os`, `unicode` for `strings`, `context` for `testing`,
  `encoding` for `flag`), and `weak`, whose `Pointer`, `Make` and `Value` the compiler turns
  into the runtime's `gp::weak_ptr` (an intrinsic from Phase 03, Phase 03 README §2.2). (`errors` was the 25th
  stub; it is real since step 5.7, and `unicode/utf8` since step 9.1.)

  A stub holds Go's exported declarations with their doc comments, but no code: functions have
  no body, and variables have no value. So programs that import these packages type-check, and
  the editor can show the docs. Nothing can run yet. Each file says which phase replaces it
  with the real package.

  The details:
  - A const keeps its exact value. If Go writes the value as an expression of names the stub
    also has (`1000 * Nanosecond`), the stub writes the same expression; otherwise it writes
    the computed value (`O_RDONLY int = 0`).
  - A struct keeps only its exported, blank and embedded fields. When Go's struct has more, a
    blank field `_` stands for them. It keeps a struct that Go can't compare from being
    comparable here.
  - An unexported type that an exported struct embeds stays too, with its exported methods
    (`testing.T` embeds `common`, which has `Errorf`), so a selector finds a promoted method at
    the same depth as in Go: a selector like `s.Errorf` is ambiguous exactly when it is in Go.
  - A generic function gets the body `panic("stub")`, because go/types reports "missing
    function body" for a generic function without one.
  - A declaration that needs a package without a stub (`syscall`, `reflect`, `internal/...`)
    or an unexported name is left out. The header of each file lists what it leaves out, for
    example `os.File.SyscallConn` and the `reflect.Type` fields of `encoding/json`'s errors.
    `unsafe` needs no stub: go/types has it built in. A left-out member can change what a
    selector finds: in `struct{ *json.UnmarshalTypeError; Z }`, where `Z` has a `Type` field,
    `x.Type` is ambiguous in Go but is `Z`'s with the stubs.
  - Sizes are not Go's: `unsafe.Sizeof(sync.Mutex{})` is 0 with the stubs and 8 in Go, because
    the unexported fields are gone. Nothing in Phase 01 depends on it.
  - Values that depend on the platform are Go's for linux/amd64 (`os.O_CREATE`,
    `os.PathSeparator`, `strconv.IntSize`), because the tool reads those files. The real
    packages (Phase 09, and Phase 14 for Windows) take each platform's own files.
- **`files.go`**, the small Go package `stdlib` (`github.com/huuhait/gocpp/stdlib`). It is
  here because a `//go:embed` pattern can't reach a parent folder.
  - `stdlib.FS()` is the embedded folder: the `.gp` files of each package, in the folder of
    its import path, and `LICENSE`. This README, `std.txt`, `PORTED.txt`, `PORT.md` and the Go files are not in it.
    The `//go:embed` patterns take `.gp` files up to two folders deep (`encoding/json`).
    Phase 09 adds a pattern for deeper packages (`math/rand/v2`) and for native `.cpp` files;
    until then a test fails for any file here that the patterns don't take.
  - `stdlib.IsStd(path)` says whether a path is one of Go's standard packages, whether gocpp
    has it yet or not. So the loader can tell "package regexp is not available yet" from "not a
    standard package". `stdlib.Std()` returns the whole list.
- **`std.txt`**, that list: what `go list std` prints with the dev shell's Go, without the
  `vendor/` packages (the standard packages import them without `vendor/`, and programs can't
  import them). The internal packages are in it, because Go's rule for `internal/` decides who
  may import them: the loader applies that rule first, then asks `IsStd`.
- **`LICENSE`**, Go's license (BSD-3-Clause). The stubs are copied from Go's source, so it
  ships with them, also inside the binary. It is the same file as `internal/syntax/LICENSE`,
  from Go's official source archive (Nix's Go has no LICENSE file).

## How to make the files again

Run these from the repository root, in the dev shell (`nix develop`):

```sh
go run ./tools/stubgen                       # writes the stubs from $(go env GOROOT)/src
go test ./stdlib -run TestStdList -update    # writes std.txt from go list std
```

The `-update` writes `std.txt` only when the `go` command is the Go that the stubs come from.
To move to a new Go version, run the tool first, in the new dev shell, then the `-update`.

`tools/stubgen` holds the list of packages, each with the phase that replaces it. It reads the
dev shell's Go source with `go/build` (the files for linux/amd64, without cgo) and type-checks
each package with go/types, so the stubs use exactly Go's types and values. It reads the local
GOROOT, which Nix patches in a few places (paths in `time`), but only in code and unexported
values, never in an exported declaration. To add a package, add it to the list and run the tool.
Don't edit a stub by hand: the tests fail when a stub is not what the tool writes.

## Tests

- `tools/stubgen`: the stubs are what the tool writes now, and every declaration in them has the
  same type (and a const the same value) as in Go; every exported declaration of Go is in the
  stub or in its list of what it leaves out; and small programs with promoted methods give the
  same errors with the stubs as with Go's packages. They skip when the `go` command is not the Go
  version that the stubs come from.
- `stdlib`: `std.txt` matches `go list std` (skipped without a `go` command, or with a different
  Go); the embedded FS holds every file here except the Go package's own; each stub parses with the parser fork and is formatted like
  `gocpp fmt`; the stubs type-check with go/types with 0 errors; and the target programs in
  `examples/` type-check against them with 0 errors, after the header and namespace rewrites of
  `internal/load` for the files that import headers. `macros` and `must-errors` are skipped,
  because their syntax (`macro`, `#Name(...)`, `F!(...)`) parses only from Phase 10.

## Later phases

- **Phases 05 to 08** replace the first stubs with real code: `errors` (done, step 5.7) and a basic `fmt` in
  Phase 05, `cmp`, `slices`, `maps` and `iter` in Phase 07, and `sync`, `sync/atomic` and `time`
  in Phase 08.
- **Phase 09** fills it. Most packages are **ported from Go's own source** (BSD-3-Clause) with
  `tools/portstd`. The tool takes the pinned source tarball of the dev shell's Go version (now
  `go1.25.14.src.tar.gz`, checked by SHA-256), not the local GOROOT, which Nix patches. It picks
  the files for the target OS with the `purego` tag (no assembly), renames `.go` to `.gp`, turns
  `//go:linkname` declarations into native functions, applies a small patch set, and writes
  `stdlib/<pkg>/PORT.md` (upstream version, files, patches). Moving to a new Go version means
  running the tool again. The system parts are **native**: function declarations without a body
  in a `.gp` file with `//gocpp:native` (above), and their bodies in a `.cpp` file next to it. A package deeper than two
  folders (`math/rand/v2`) needs one more pattern in `files.go`; a test finds it.
  Step 9.1 built the tool (the paragraph above is what it does); the argument-less `//gocpp:native` (a body in a `.cpp`
  file) is built by step 9.2, its first user.
- **Phase 14** runs `tools/portstd` again for Windows.
