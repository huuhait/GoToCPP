# tests/pkgs

A multi-package test is a **module in a folder**: `gocpp.mod`, a `main.gp` in the folder, and the
packages it imports in folders below it (Phase 06 step 6.2). The harness runs `gocpp run .` in the
folder, once per C++ compiler, like a [run test](../run/README.md): stdout must match the
`// Output:` block at the end of `main.gp`, with the same `// phase:`, `// ExitCode:`, `// Stderr:`,
`// lsan: cycle`, `// divergence:` and `//go:build` lines at the top of `main.gp`. The folder's
other files (C and C++ files, headers, a vendored or `replace`d module below it) are part of the test.

```
tests/pkgs/basic_cross/
├── gocpp.mod          module example.com/shop
├── main.gp            // phase: 06 ... package main ... // Output: ...
├── geom/geom.gp       package geom
└── shape/shape.gp     package shape (imports geom)
```

`GOCPP_RUN_VS_GO=1 go test ./tests/... -run PkgsVsGo` builds each folder with real Go
(`gocpp.mod` becomes a `go.mod` with its `module` line, the `.gp` files are copied as `.go`) and
checks the same `// Output:` block, so that an expected output can't drift from Go's. A folder that
Go rejects (a C header import, `C.cstr`) is skipped there. Write the expected output with
that run: the fixtures of this folder were made by running the module with `go run`.

The tests need `GOCPP_TEST_PHASE=06` until Phase 06 is finished (`gocpp` rejects programs of more
than one package before: the gate "programs of more than one package").

What the folders test: `basic_cross` (constants, types, methods, variables, interfaces across
packages), `qualified_uses` (every way to use `pkg.Var` and `pkg.F`), `vars_funcs_init` (variable
initialization across packages and files, func values, aliases), `rich_types` (defined types over
slices, maps, funcs; embedding of types and interfaces of another package), `embed_init_recover`
(init order, `recover` in another package), `anon_shared` (unnamed struct and interface types and
unexported names in several packages), `odd_names` (import paths, folder names and C++ keywords as
names), `public_flags` (flags and a local header of a package whose header another includes),
`package_sources` (`.c` and `.cpp` files of non-main packages), `shared_lib` (a cxx recipe library
used by two packages), `callbacks_cross` (a function of another package as a C callback), `constants_cross` (constants of another package and the constants in `pkg.hpp`), `more_uses` (defined array, slice and struct types, variadic functions, closures and errors of another package), `chan_cross` (Phase 08: a named channel type exported by a package, channels of another package's struct, `go` calls of another package's functions and methods, type switches on channel types; run with `GOCPP_TEST_PHASE=08`), `module_replace` and `module_vendor` (packages of a `replace`d and of a vendored module).

`regexp_internal` (Phase 09, step 9.4c, ADR-083) runs Go's internal tests of package `regexp` (they name unexported functions and fields): the folder `regexp/` holds a copy of the port (`stdlib/regexp`, file for file; `TestRegexpInternalCopy` keeps it equal) beside the converted tests (`*_gotest.gp`) and the harness (`harness.gp`), and `testdata/` the data files of Go's tests.

## errors

`tests/pkgs/errors/<name>/` is a module whose `.gp` files have `// ERROR "regexp"` (and
`// WARNING`) comments like an [error test](../errors/README.md): `gocpp check -e ./...` must report
exactly those errors, each in the file and on the line of its comment. The folder has a `main.gp`
whose first line is the `// phase: NN` line. Load errors that have no place in a file (an import
cycle) are tested in `cmd/gocpp` (`TestPackagesImportCycle`).
