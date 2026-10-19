# ADR-045: Internal shims are ordinary standard packages, and tests reach them through a harness-only switch

Status: accepted (2026-10-18)

## Context

The tier-1 packages of Phase 09 (`strings`, `bytes`, `strconv`, `os`, `time`, ...) import packages that only Go's
standard library may import: `internal/bytealg`, `internal/stringslite`, `internal/byteorder`, `internal/abi`,
`internal/cpu`, `internal/godebug`, `internal/race`, `internal/testlog`, `internal/itoa`, `internal/oserror`,
`internal/goarch`, `internal/goos`. Go writes some of them in assembly for each processor (`bytealg`), some read the
runtime through `//go:linkname` (`godebug`, `testlog`), and `runtime/cgo` keeps its handles in a `sync.Map`. Step 9.2
has to provide them before the ports of step 9.4 can build, and has to test them, although Go's rule for internal
packages forbids a test program from importing them (and real Go, which TestRunVsGo uses, forbids it too).

## Decision

1. **A shim is a real standard package** in `stdlib/internal/<name>`, lowered like any other (ADR-018). Where Go's
   source is pure (`stringslite`, `byteorder`, `itoa`, `oserror`, `goarch`, `goos`, `race`, `cpu`'s variables) the
   file is Go's, copied from the pinned source with its license header, and named in the header comment so that
   `tools/portstd` can regenerate it. Where Go has assembly or a linkname, the function has no body and a
   `//gocpp:native` line (ADR-027): `internal/bytealg` (memchr, memcmp, memmem and a counting loop in
   `runtime/src/bytealg.cpp`), `internal/godebug`'s read of the environment, and the handle table of `runtime/cgo`
   (`runtime/src/shims.cpp`). The generic Rabin-Karp functions of `bytealg` stay Go code: one body for `string` and
   `[]byte` through the instances of ADR-030.
2. **The shims are what the ported packages import and no more**: the dependency table in the phase-09 README (§9.2
   notes) is made from the imports of the tier-1 roots, and each symbol in it is either in a shim, or owned by a named
   step. `internal/cpu` has every feature false, so no ported package takes an architecture-specific path; `race` is
   a set of no-ops with `Enabled = false`; `godebug` has no metrics.
3. **Tests reach internal packages by a switch that only the harness sets.** A run or pkgs test with a
   `// std-internal: reason` line gets `GOCPP_ALLOW_INTERNAL=1`; `cmd/gocpp` turns it into
   `load.Config.AllowStdInternal`, which opens the standard library's internal packages that exist (not a module's
   `internal` folder, not the hidden `gocpp/cgoh`). Nothing else sets it, a file with the line must say why, and tests
   of other kinds (`errors`, `diff`, ...) may not have it, so users still see "use of internal package ... not
   allowed".
4. **Real Go checks the shims through twins.** For TestRunVsGo the harness copies the shims a test imports into a
   module as `.go` files; the files with natives are replaced by `tests/testdata/twin/*.go.txt` that call the public
   `bytes` and `strings`. The test's `// Output:` is then what Go computes, and the differential tests (10,000 seeded
   cases per function, a naive loop written in the test, a digest of all results) compare gocpp with Go. The C++
   side has its own unit test (`runtime/test/bytealg_test.cpp`: g++ and clang++, `-O0`, `-O2` and ASan), because
   gocpp's default build is `-O0` and so never runs the `-O2` branch of `Count`.

## Consequences

- The ports of step 9.4 import `internal/bytealg` as Go's code does; `strings.Index` and friends run on `memchr` and
  `memmem`.
- A new shim with natives needs a twin file for TestRunVsGo (`twinNative` in `tests/twin_test.go`).
- `GOCPP_ALLOW_INTERNAL` is not a user feature: a program that uses it is not Go (`load.Config` says so).
- `Count` uses a plain loop that the C++ compiler vectorizes when it optimizes (8 to 14 GB/s on 1 MiB at `-O2`), and
  8-byte words otherwise (gocpp's default build is `-O0`, where the loop would run a byte at a time).
