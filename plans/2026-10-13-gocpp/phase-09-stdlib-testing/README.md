# Phase 09: Standard Library & `gocpp test`

> Give gocpp a Go-shaped standard library (`fmt`, `strings`, `strconv`, `os`, `io`, `bufio`,
> `time`, `sort`, `errors`, `flag`, `regexp`, `encoding/json`, …) and Go's testing workflow
> (`_test.gp` files, `gocpp test`). Most of it is **ported from Go's own source code**, which works
> because gocpp is Go syntax. Only the system-facing parts are written in C++ or through C interop.

| | |
|---|---|
| Depends on | Phases 04, 05, 06, 07, 08 |
| Unlocks | Phase 10 (`#Derive(JSON)` and `json.Unmarshal!` need `encoding/json`; derive uses `strconv`/`fmt`), Phase 14 (the stdlib's Windows files), real users |
| Milestone | **M6: v0.1, usable for real small programs** |
| Size | large (about 7–9 weeks) |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

A language feels like Go when `fmt.Printf`, `strings.Split`, `os.ReadFile`, `sort.Slice` and
`go test` behave like Go. Writing all of that by hand would take years. But much of Go's
standard library is **pure Go**, and gocpp compiles Go syntax with Go semantics, so we can copy
the real source code (BSD-3-Clause) and its real tests. That way we get
**battle-tested behavior *and* a huge free conformance test suite**.

What blocks a straight copy is small and known. It was checked with `go list` on 2026-10-13
against Go 1.25.7, and the audit refined it:

| Package | What needs work | Strategy |
|---|---|---|
| `unicode/utf8`, `container/list` | nothing | copy as-is |
| `io`, `bufio`, `regexp`, `text/tabwriter`, `hash/fnv`, `encoding/base64` | only portable packages + `internal/byteorder` | copy; port `internal/byteorder` (pure Go) |
| `strconv`, `strings`, `bytes` | `internal/bytealg`, `internal/stringslite`, `internal/abi`, `unsafe` | copy; gocpp shims (`bytealg` → `memchr`/`memcmp` natively). Their `unsafe.String(unsafe.SliceData(…))` tricks are safe because Phase 04 makes `unsafe` keep gocpp memory alive |
| `slices`, `cmp` | `unsafe` (supported) | copy (Phase 07) |
| `maps`, `iter` | `//go:linkname` into the runtime (`maps.Clone`, `iter.Pull`) | copy; `tools/portstd` turns linkname declarations into native bodyless functions (Phase 07) |
| `errors`, `sort`, `context` | `internal/reflectlite` | copy; back `reflectlite` with our reflect subset (2.2) |
| `fmt`, `flag`, `encoding/json`, `encoding/binary` | `reflect`, `internal/fmtsort` | copy; reflect subset (2.2) + port `fmtsort` |
| `time`, `os`, `path/filepath`, `log` | `runtime`, `syscall`, `internal/godebug`, `internal/filepathlite` | port the logic (formatting, parsing, paths); write the system hooks natively |
| `math` | `internal/cpu`, `unsafe`, assembly files | wrap `<cmath>` natively for speed; keep Go's pure-Go code where bit-exact results matter |
| `crypto/sha256` | `crypto/internal/fips140/...` (Go 1.24+ FIPS module) | don't port the FIPS tree; port only the generic (`purego`) block function, or wrap a C library |

## 2. How it works

### 2.1 Three ways to implement a stdlib package

1. **Ported Go.** `tools/portstd` copies a package mechanically and records provenance in
   `stdlib/<pkg>/PORT.md` (upstream version, file selection, patch). Three rules, all from the audit:
   - **Source:** the **pinned upstream source tarball** of the dev shell's Go version (now `go1.25.14.src.tar.gz`), SHA-256 checked,
     not the local GOROOT. On NixOS the local GOROOT is patched (hard-coded `/nix/store/…` paths in
     `time/zoneinfo_unix.go` and four other files) and has no `test/` folder.
   - **File selection** goes through `go/build` for the target OS, with the `purego` tag. Packages
     like `math` and `internal/bytealg` use `//go:build` lines to swap pure-Go fallbacks for
     assembly (`.s`) files on amd64. gocpp has no Go assembly, so it always takes the pure-Go
     variants. The tool runs once per target OS (`linux` now, `windows` in Phase 14) and keeps
     Go's file names (`file_unix.go`, `file_windows.go`), so the Windows port reuses Go's own
     Windows code (paths, `os` errors) instead of rewriting it.
   - **Edits:** rename `.go` → `.gp`, turn `//go:linkname` declarations into native bodyless
     functions, apply a small patch set.
   Upgrading to a new Go version means re-running the tool.
2. **Native package.** A `.gp` file with **bodyless function declarations** (Go allows these;
   they're normally implemented in assembly) plus a `.cpp` file in the same directory that
   implements them. The package header (Phase 06) declares the functions, and the `.cpp`
   defines them. It's the same mechanism user packages get from Phase 04, nothing special:

   ```go
   // stdlib/internal/bytealg/bytealg.gp
   package bytealg

   func IndexByteString(s string, c byte) int // no body → implemented in bytealg.cpp
   ```
   ```cpp
   // stdlib/internal/bytealg/bytealg.cpp
   int64_t gpkg_internal__bytealg::IndexByteString(gp::string s, uint8_t c) {
       auto* p = static_cast<const char*>(std::memchr(gp::data(s), c, gp::len(s)));
       return p ? p - gp::data(s) : -1;
   }
   ```

   > **Marker (built in Phase 05 step 5.7, ADR-027):** a bodyless function in a standard package is a native
   > function only with a `//gocpp:native <C++ name>` line in its doc comment; without it, it is a Phase 01
   > declaration stub, and a package with one is not built. Today the line names a function of the runtime
   > (`gp::errors::assign`, declared in a header that `gocpp/rt.hpp` includes) and the calls become calls of it;
   > nothing is written into `pkg.hpp` for it. Phase 09 extends the directive to the form this section describes:
   > no argument means the body is in a `.cpp` file next to the `.gp` file, and the package header declares the
   > function (`internal/load/native.go` rejects the empty form with a usage error today; `load.Package.IsStub`
   > is the one test of "stub or real"). `tools/portstd` writes `//gocpp:native` for a `//go:linkname`
   > declaration, and a `PORT.md` per package (`stdlib/errors/PORT.md` is the model; it stays out of the embedded FS).

   `go/types` rejects bodyless **generic functions** ("missing function body"); bodyless methods
   of generic types are accepted. So a native generic API is a small generic gocpp function that
   calls a non-generic native one.
3. **gocpp code that uses C interop.** For example, `os` imports `<fcntl.h>`, `<unistd.h>` and
   `<sys/stat.h>` and calls them through their header packages (`unistd.read(fd, p, n)`),
   dogfooding Phase 04.

### 2.2 A reflect subset on top of type descriptors

`fmt`, `flag`, `encoding/json`, `sort`, `errors`, `context` and the ported tests need some
reflection. Phase 05 already generates a **type descriptor** for every type that flows into an
interface (chosen by Go type). This phase extends the descriptors with element/key types, array
length, a **field table** (name, tag, offset, type, embedded flag), accessor thunks (slice
length/index, map iteration), a **zero-value constructor**, and **element assign/swap**. On top of
that we implement a `reflect` **subset**. The list below is taken from what the ported packages
actually call (grep), not guessed:

| Needed by | API |
|---|---|
| everyone | `TypeOf`, `ValueOf`, `Type.Kind/Name/String/Elem/Key/Len/NumField/Field(i)/NumMethod/Implements/AssignableTo/Comparable/Bits`, `Value.Kind/Type/Interface/IsValid/IsNil/IsZero/Len/Index/Field/Elem/Int/Uint/Float/Complex/String/Bool/Bytes/Pointer/UnsafePointer/CanInterface/CanAddr/MapRange/MapKeys` |
| `sort.Slice` | `reflectlite.Swapper` |
| `flag` | `New`, `Zero` |
| `fmt` (incl. `Sscan`) | `MakeSlice` + the above |
| `encoding/json` | settable values: `New`, `Zero`, `MakeSlice`, `MakeMap`, `PointerTo`, `Addr`, `Set`, `SetInt`/`SetUint`/`SetFloat`/`SetString`/`SetBool`, `SetMapIndex`, `SetLen`, `Grow`, `Overflow*` |
| ported tests | `DeepEqual` |

`internal/reflectlite` becomes a thin alias of this subset. **Not included:** `reflect.MakeFunc`,
`Method` calls by name, `StructOf`/`FuncOf`, and anything that needs runtime code generation.
Divergence D6 in Phase 00 records this.

### 2.3 `fmt` (ported)

Copy Go's `fmt` and replace `reflect` with the subset. That gives every verb (`%v %+v %#v %T %t
%d %b %o %O %x %X %c %q %U %e %E %f %F %g %G %s %p %w`), flags, width and precision,
`Stringer`/`GoStringer`/`Formatter`/`error`, maps printed with sorted keys (ported
`internal/fmtsort`), and `&{…}` for pointers to structs. Float output comes from the ported
`strconv`, so it is **bit-identical to Go**. C++ values in `any` print through their foreign
descriptors (Phase 05); raw pointers print as addresses.

### 2.4 `os`, `io`, `bufio`

- `os`: `Args`, `Getenv`/`Setenv`/`Environ`, `Exit`, `Getwd`/`Chdir`, `Hostname`, `Getpid`,
  `Open`/`Create`/`OpenFile`, `*File` (`Read`, `Write`, `Close`, `Seek`, `Stat`, `ReadDir`),
  `ReadFile`/`WriteFile`, `Remove`/`Rename`/`Mkdir`/`MkdirAll`, `Stdin`/`Stdout`/`Stderr`.
  Errors are Go's types (`*PathError`, `ErrNotExist` through `errors.Is`) built from `errno`.
- `io` and `bufio` are ported unchanged (`Reader`, `Writer`, `Copy`, `ReadAll`, `EOF`,
  `Scanner`, …).

### 2.5 `time`

Port Go's pure-Go logic (Duration, Month/Weekday, formatting/parsing layouts, Location and
zoneinfo parsing). Implement the hooks natively: wall clock + monotonic clock
(`clock_gettime`), `Sleep`, and timers from Phase 08. Zoneinfo comes from `ZONEINFO`, the standard
system paths (upstream Go already lists NixOS's `/etc/zoneinfo`), and an optional embedded
`time/tzdata`.

### 2.6 Package tiers

| Tier | Packages |
|---|---|
| **1 (for v0.1)** | `fmt errors strings bytes strconv unicode unicode/utf8 sort slices maps cmp iter math math/bits os io bufio time sync sync/atomic context testing flag regexp` |
| **2 (before Phase 10)** | `encoding/json` (with the settable reflect subset), `log path path/filepath math/rand/v2 os/exec encoding/hex encoding/base64 encoding/binary hash/fnv hash/crc32 text/tabwriter container/list container/heap container/ring crypto/sha256 crypto/md5` |
| **3 (later)** | `encoding/csv`, `compress/gzip`, `archive/zip`, `os/signal`, `net` (sockets via interop) |
| **Not planned** | `net/http` (use a C++ HTTP library through interop, or a later wrapper package), `plugin`, `go/*`, full `reflect` |

`flag` and `regexp` are in tier 1 because the M6 target programs (`wc`, `grep-lite`) use them.
`encoding/json` is scheduled in tier 2 because Phase 10's `#Derive(JSON)` and examples rely on it.

### 2.7 `testing` and `gocpp test`

- `_test.gp` files: in-package tests and external `package x_test` tests (both, like Go).
- `gocpp test ./...` generates a test `main` per package (`TestXxx`, `BenchmarkXxx`,
  `ExampleXxx` with `// Output:` checks), builds it, runs it, and prints Go's format:
  `ok  	pkg	0.012s` / `--- FAIL: TestX (0.00s)` / `FAIL`.
- `testing.T`: `Error(f)`, `Fatal(f)` (stops the test goroutine through the Phase 05 `Goexit`
  signal, which `recover` can't swallow), `Log(f)`, `Run` (subtests), `Parallel`, `Helper`,
  `Skip`, `Cleanup`, `TempDir`, `Setenv`, `Context`.
- `testing.B`: `b.N` and Go 1.24's `b.Loop()`, `ReportAllocs`.
- Flags: `-run`, `-v`, `-count`, `-timeout`, `-short`, `-failfast`, `-bench`, `-benchtime`, `-race`.
- **testify works** (your wish, 2026-10-13): tests written with `github.com/stretchr/testify`'s
  `assert` and `require` (`assert.Equal(t, want, got)`, `require.NoError(t, err)`) build and run
  unchanged. The real testify needs more than our reflect subset (go-spew walks every value with
  reflect and unsafe, plus `reflect.DeepEqual`, `runtime.Caller`, go-difflib and yaml), so the
  first route is a gocpp-maintained drop-in with the same API and the same failure output, picked
  automatically for that import path; compiling the real module is the goal once reflect is big
  enough. (Which route: TODO open question.)
- Later (backlog): fuzzing (`testing.F`), coverage via `-fprofile-instr-generate` mapped through
  `#line`, `-json` output.

### 2.8 Running Go's own tests

**Ported Go tests run against ported Go packages.** `gocpp test stdlib/strings` runs Go's own
`strings` tests compiled by gocpp. This phase also runs the suites that earlier phases could not:
the tests of `slices`, `maps`, `cmp` and `iter` and Go's range-over-func tests (from Phase 07), and
the `test/chan` programs that need `flag`, `runtime.GC`, `container/*` or `text/template`
(from Phase 08).

Those test files need more than the tier lists: `reflect.DeepEqual` (used 17 times in `strconv`'s
tests alone), `math/rand` v1, `testing/iotest`, `testing/quick`, `internal/testenv`,
`runtime/debug`, and in a few places `net`, `os/exec`, `encoding/gob` or `math/big`. A test file
that doesn't compile takes all its tests with it, so step 9.10 provides the small test-only
packages (`iotest`, `quick`, `testenv` stubs, `math/rand`) and patches the rest out with a
written reason. **Counting rule:** skipped tests count as *not passing*, unless they are skipped
for a documented divergence (for example `AllocsPerRun` tests: allocation counts differ because
gocpp allocates differently than Go). The budget is "≥ 90 % of the test functions in the selected
files pass".

## 3. Steps

1. **9.1 `tools/portstd`**: download and verify the pinned Go source, select files with `go/build`
   per target OS (`purego`, no assembly), rename, turn linkname into native declarations, patch,
   write `PORT.md`.
2. **9.2 Internal shims**: `internal/bytealg` (native), `internal/stringslite`,
   `internal/byteorder`, `internal/abi`/`internal/godebug`/`internal/cpu` (stubs), minimal
   `runtime` and `syscall` surfaces. This step also owns the public `runtime/cgo` package (`Handle`,
   `NewHandle`, `Value`, `Delete`: a native handle table behind the `//gocpp:native` marker; the docs
   say "Phase 09" for it, since step 5.12 found that no step had built it) and, unless step 8.10 builds
   it first, `runtime.Error` and `runtime.PanicNilError` as named types that the runtime's panic values
   already implement (`r.(runtime.Error)`).
3. **9.3 Reflect subset** (descriptor extensions incl. zero value and element assign/swap; the
   per-package API list in §2.2; `internal/reflectlite` alias; `DeepEqual`).
4. **9.4 Port tier-1 pure packages**: `unicode/utf8`, `unicode`, `strings`, `bytes`, `strconv`,
   `errors` (replaces the Phase 05 version), `sort`, `math/bits`, `io`, `bufio`, `context`,
   `regexp`.
5. **9.5 `math`** (native `<cmath>` + Go code where bit-exactness matters).
6. **9.6 `os`** (interop + native) with Go error types. (Before `fmt`, because `fmt` imports `os`.)
7. **9.7 `fmt`** port on the reflect subset (+ `internal/fmtsort`, C++ value printing).
8. **9.8 `time`** (ported logic + native hooks + zoneinfo lookup).
9. **9.9 `flag`, `testing` + `gocpp test`** (T incl. `Parallel`, B with `Loop`, examples, flags,
   Go-style output).
10. **9.10 Run Go's tests** for tier 1 and the deferred suites (§2.8): test-only packages, patches
    with reasons, the counting rule; fix the gocpp bugs they find.
11. **9.11 Tier-2 packages**, `encoding/json` first.
12. **9.12 Docs**: package list, what differs from Go, how to port a package.
13. **9.13 v0.1 release candidate**: tag, changelog, install notes.

## 4. Deliverables

- `stdlib/` tier 1 + tier 2, `tools/portstd`, reflect subset, `gocpp test`
- Target programs `wc` and `grep-lite` passing
- v0.1 release candidate

## 5. Exit criteria

- All tier-1 packages are present, and **≥ 90 % of the test functions in their selected Go test
  files pass** under the counting rule in §2.8 (every exclusion has a written reason).
- The deferred suites (`slices`/`maps`/`cmp`/`iter`, rangefunc, the rest of `test/chan`) pass.
- `wc` and `grep-lite` produce the same output as their Go versions on test inputs.
- `gocpp test` output format matches `go test` for pass, fail and benchmark runs.
- `fmt` differential tests (≥ 100 format strings × values) match Go byte for byte.
- `encoding/json` round-trips the Go test corpus structs the same way Go does.

## 6. Risks

| Risk | Mitigation |
|---|---|
| A ported package uses something unsupported (linkname, runtime internals, assembly) | `portstd` file selection (`purego`, no `.s`), linkname → native functions, small patch sets in `PORT.md` |
| The reflect subset grows too big | Implement only what the ported packages call (§2.2 list from grep); add on demand |
| `<cmath>` results differ from Go in the last bit | Use Go's pure-Go code for those functions, or tolerance in tests (documented) |
| Big tables (`unicode`) slow down C++ compiles | Emit them as `constexpr` arrays in their own TU; cached by Phase 06 |
| Go 1.24+ crypto is a FIPS module tree | Port only the generic (`purego`) block functions, or wrap a C library |
| The 7–9 week estimate slips | `encoding/json` and the rest of tier 2 are the first things to move later; v0.1 needs tier 1 |
