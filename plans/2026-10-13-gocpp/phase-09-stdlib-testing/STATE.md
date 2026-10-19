# Phase 09: Standard Library & `gocpp test` · STATE

**Status:** `[~]` in progress (9.1-9.6, 9.8, 9.9 merged; open: 9.7 (decision: port Go's `fmt` or keep the native engine of 5.11), 9.10, 9.11, 9.12, 9.13; `stdlib/flag` is still a declaration stub, so `wc` and `grep-lite` stay pending)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-19

## Steps

- [x] Check which Go stdlib packages can be ported and what blocks them (`go list`, 2026-10-13; table in README §1)
- [x] 9.1 `tools/portstd` (pinned source tarball + SHA-256, `go/build` file selection per target OS with `purego`/no asm, rename, linkname → native, patch, `PORT.md`)
- [x] 9.2 Internal shims
  - [x] `internal/bytealg` (native)
  - [x] `internal/stringslite`, `internal/byteorder`
  - [x] `internal/abi`, `internal/godebug`, `internal/cpu` stubs
  - [x] minimal `runtime` + `syscall` surfaces
- [x] 9.3 Reflect subset
  - [x] descriptor extensions (fields + tags, elem/key, len, accessors, zero value, element assign/swap)
  - [x] per-package API list (§2.2), incl. settable values and `Swapper`
  - [x] `internal/reflectlite` alias + `DeepEqual`
- [x] 9.4 Port tier-1 pure packages (9.4a unicode, strconv, errors merged 2026-10-20; 9.4b io, io/fs, path, strings, bytes, bufio, context merged 2026-10-20; sort, regexp/syntax, regexp merged 2026-10-20 (9.4c)) (`unicode/utf8`, `unicode`, `strings`, `bytes`, `strconv`, `errors`, `sort`, `math/bits`, `io`, `bufio`, `context`, `regexp`)
- [x] 9.5 `math` (native + bit-exact pure-Go where needed)
- [x] 9.6 `os` (interop + native, Go error types)
- [x] 9.7 `fmt`: NOT ported. Decision 2026-10-19: keep the native engine of 5.11 as the product (fast, handles C++ values, 210 run tests compared with real Go); an audit against Go's own fmt tests through the 9.10 harness follows
- [x] 9.8 `time` (ported logic, native clocks/timers, zoneinfo)
- [x] 9.9 `flag`, `testing` + `gocpp test` (T incl. `Parallel`, B + `Loop`, examples, flags, Go-style output)
- [ ] 9.10 Go's tests: tier 1 + deferred suites (slices/maps/cmp/iter, rangefunc, rest of `test/chan`); test-only packages; counting rule
- [ ] 9.11 Tier-2 packages (`encoding/json` first)
- [ ] 9.12 Docs (package list, differences, porting guide)
- [ ] 9.13 v0.1 release candidate

## Exit criteria

- [ ] Tier 1 complete; ≥ 90 % of test functions in the selected Go test files pass (counting rule, written reasons)
- [ ] Deferred suites (from Phases 07 and 08) pass
- [x] `wc` + `grep-lite` match their Go versions (flag ported 2026-10-19: identical output on 14 and 12 argument vectors)
- [x] `gocpp test` output format matches `go test`
- [x] `fmt` differential tests (≥ 100 cases) byte-identical to Go (210 `fmt_*` run tests vs real Go)
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
- 2026-10-18: Job 9.1 (`tools/portstd`) merged after two review rounds (ADR-034). Decisions: (1) pinned go1.25.14 tarball (`tools/portstd/PINS`, SHA-256) cached in
  `$XDG_CACHE_HOME/gocpp/portstd`, never downloaded by tests or `make portstd-check`; (2) file selection uses go/build constraints with a SYNTHETIC `GOARCH=generic`
  + tag `purego` (Go's asm packages select by architecture; `purego` alone keeps bodyless declarations), never `.s` files; kept `//go:build` lines are rewritten so they
  never name tags the gocpp loader lacks; ALL `goexperiment.*` tags are false (pre-experiment variants: `sync` (8.7) gets the non-hashtriemap variant; documented);
  (3) per-package patches (`patches/<pkg>.rules`, each with a reason, failing when the old text is not found as often as stated), `//go:linkname` → `//gocpp:native` via
  `natives.txt` (the argument-less native form is listed as `local`: step 9.2 builds it), findings for asm/cgo/embed (the port is refused when unresolved: `time`, `os`,
  `internal/bytealg` are refused); (4) outputs: `stdlib/<pkg>/PORT.md`, `stdlib/PORTED.txt`, `stdlib/PORTED.sha256` (hash of every written file: `-check` works WITHOUT the
  cache by hash; with the cache it does the full byte compare; a hand edit that also rewrites its hash line passes the hash level only); `GOCPP_REQUIRE_PORTSTD=1`/`-require`
  makes a missing tarball an error (opt-in, mirrors GOCPP_REQUIRE_CXX); (5) `prepareWrite`: removes stubgen stubs, refuses to overwrite/leave a hand-written .gp without
  `-force`; porting.md has the 'Replacing a stub' checklist (stubgen `Real: true`, gates.go rows, feature-matrix.md, `TestStdPackagesMatchStubs` count); (6) ported through
  the tool: `unicode/utf8` (Real), `unicode/utf16`, `math/bits`; tier-1 blocker table (docs/guides/porting-blockers.md, kept current by `-check`): unicode/utf8, unicode, math/bits
  none; errors/sort/context need reflectlite (9.3); io/strings/regexp(/syntax)/context need sync (8.7), context also sync/atomic + time (8.8); stringslite/strings/bytes need
  bytealg/abi shims (9.2), strings/bytes also iter (7.8 + 7.3); strconv needs math (9.5); sort/regexp need slices (7.10); (7) LOW items left (TODO): hash-only `-check` does not
  cross-check against ports.txt (deleting both manifests + a package passes without the cache), `cleanStale` removes any `extract-*`/`download-*` older than 1 h from a custom
  `-cache` (match exact temp names), redirects can downgrade to http (add CheckRedirect + client timeout), unwanted tar entries/file counts uncapped; (8) ported utf8/bits run
  8-12x slower than Go at -O0 (gocpp builds -O0 -g only: a release mode closes it; optional math/bits natives later).
