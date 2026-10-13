# Phase 09: Standard Library & `gocpp test` · STATE

**Status:** `[ ]` not started
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] Check which Go stdlib packages can be ported and what blocks them (`go list`, 2026-10-13; table in README §1)
- [ ] 9.1 `tools/portstd` (pinned source tarball + SHA-256, `go/build` file selection per target OS with `purego`/no asm, rename, linkname → native, patch, `PORT.md`)
- [ ] 9.2 Internal shims
  - [ ] `internal/bytealg` (native)
  - [ ] `internal/stringslite`, `internal/byteorder`
  - [ ] `internal/abi`, `internal/godebug`, `internal/cpu` stubs
  - [ ] minimal `runtime` + `syscall` surfaces
- [ ] 9.3 Reflect subset
  - [ ] descriptor extensions (fields + tags, elem/key, len, accessors, zero value, element assign/swap)
  - [ ] per-package API list (§2.2), incl. settable values and `Swapper`
  - [ ] `internal/reflectlite` alias + `DeepEqual`
- [ ] 9.4 Port tier-1 pure packages (`unicode/utf8`, `unicode`, `strings`, `bytes`, `strconv`, `errors`, `sort`, `math/bits`, `io`, `bufio`, `context`, `regexp`)
- [ ] 9.5 `math` (native + bit-exact pure-Go where needed)
- [ ] 9.6 `os` (interop + native, Go error types)
- [ ] 9.7 `fmt` port (+ `internal/fmtsort`, C++ value printing)
- [ ] 9.8 `time` (ported logic, native clocks/timers, zoneinfo)
- [ ] 9.9 `flag`, `testing` + `gocpp test` (T incl. `Parallel`, B + `Loop`, examples, flags, Go-style output)
- [ ] 9.10 Go's tests: tier 1 + deferred suites (slices/maps/cmp/iter, rangefunc, rest of `test/chan`); test-only packages; counting rule
- [ ] 9.11 Tier-2 packages (`encoding/json` first)
- [ ] 9.12 Docs (package list, differences, porting guide)
- [ ] 9.13 v0.1 release candidate

## Exit criteria

- [ ] Tier 1 complete; ≥ 90 % of test functions in the selected Go test files pass (counting rule, written reasons)
- [ ] Deferred suites (from Phases 07 and 08) pass
- [ ] `wc` + `grep-lite` match their Go versions
- [ ] `gocpp test` output format matches `go test`
- [ ] `fmt` differential tests (≥ 100 cases) byte-identical to Go
- [ ] `encoding/json` round-trips like Go

## Decisions / notes

- 2026-10-13: `reflect` is provided as a subset built on the compiler's type descriptors.
  Divergence D6 is updated.
- 2026-10-13 (audit): port from the pinned upstream tarball (NixOS GOROOT is patched and has no
  `test/`), select pure-Go files with `go/build`; reflect subset listed per package (it was too
  small for `sort.Slice`, `flag`, `fmt`, `json`); `flag`/`regexp` moved to tier 1 for M6;
  `encoding/json` scheduled in tier 2 before Phase 10; `os` before `fmt`; test-only dependencies and
  a skip-counting rule defined; Go's suites deferred from Phases 07/08 run here.
- 2026-10-13 (your decision, Q8): `tools/portstd` selects files per target OS, so Phase 14 can
  port Go's own `_windows.go` files.
