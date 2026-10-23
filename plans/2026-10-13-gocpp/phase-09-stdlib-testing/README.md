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
| `math` | `internal/cpu`, `unsafe`, assembly files | **built (9.5, ADR-071):** Go's own code ported, plus natives where Go itself runs instructions or assembly (`Exp` and `Log` transcribe Go's amd64 assembly; `Sqrt`, `Floor`, `Ceil`, `Trunc`, `Abs`, `FMA`, the bit casts are inline); never `libm`: the contract is Go's results on amd64, bit for bit |
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

#### Built in step 9.3 ([ADR-035](../../../docs/adr/035-reflect-subset-on-descriptors.md))

`stdlib/reflect` (Go's API in `.gp` over `gocpp/reflect.hpp`) and `stdlib/internal/reflectlite` (an alias package)
are in; tests are `tests/run/reflect_*.gp` (104 programs compared with real Go, `// phase: 09`) and
`tests/interop/reflect_cpp_values.gp`. Per consumer of the table above:

| Consumer | Status |
|---|---|
| everyone | built: every call of the first row, plus `Bits`, `Size`, `Align`, `NumIn/In/NumOut/Out/IsVariadic`, `ChanDir`, `FieldByName/ByIndex/ByNameFunc`, `Method(i)` names, `Convert`, `Equal`, `TypeFor` |
| `sort.Slice` | built: `Swapper` (Go's panics and the 0, 1 and 2 byte-size cases are one generic swap) |
| `flag` | built: `New`, `Zero` (also `Indirect`, `PointerTo`) |
| `fmt` (`Sscan`) | built: `MakeSlice`, `Append`, `AppendSlice`, `Copy`; `fmt` itself prints a `reflect.Value` operand |
| `encoding/json` | built: `New`, `Zero`, `MakeSlice`, `MakeMap(WithSize)`, `PointerTo`, `Addr`, `Set*`, `SetMapIndex`, `SetLen`, `SetCap`, `Grow`, `Overflow*`, `MapRange`, `SetZero`, struct tags (`StructTag.Get/Lookup`). Step 9.11 found that this is all Go's `encoding/json` uses: the `Marshaler` checks are interface assertions, so `Value.Method`/`Call`, `MapOf`, `ArrayOf`, `Select`, channel `Send`/`Recv` and `Value.Seq` stay out (ADR-102) |
| ported tests | built: `DeepEqual` (cycles, NaN, nil versus empty, funcs, Go's exact rules) |
| `context`, `errors` | built through `internal/reflectlite` (the Phase 05 `errors` keeps its natives) |

Not built, by design (D6): `MakeFunc`, `StructOf`, `FuncOf`, `Value.Call`, calling methods by name; the `Type` of a method is nil.
`internal/fmtsort` (9.7) needs nothing more than `MapRange` and the accessors `Kind`, `Int`, `Uint`,
`Float`, `String`, `Pointer`, `Field`, `Index`, `Elem`, `IsNil`, which are built.

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
   write `PORT.md`. **Built:** [docs/guides/porting.md](../../../docs/guides/porting.md) is the guide, ADR-034 the two
   decisions it added (the `generic` architecture for the pure-Go variants; the kept `//go:build` lines are rewritten to
   their OS part), [porting-blockers.md](../../../docs/guides/porting-blockers.md) the generated dependency and blocker table of
   the tier-1 list (`-list`, checked by `make portstd-check`). `unicode/utf8`, `unicode/utf16` and `math/bits` are ported
   (`stdlib/PORTED.txt`); the argument-less `//gocpp:native` stays with step 9.2, its first user. Speed: gocpp builds
   at `-O0 -g` for now, and the ported `unicode/utf8` and `math/bits` run 8 to 12 times slower than Go's, which the
   compiler turns into intrinsics (`bits.LeadingZeros`, `bits.Mul64`...). A later step can give the hot functions of
   `math/bits` a native each (`natives.txt` + a patch rule) if a benchmark needs it; correctness does not depend on it.
2. **9.2 Internal shims**: `internal/bytealg` (native), `internal/stringslite`,
   `internal/byteorder`, `internal/abi`/`internal/godebug`/`internal/cpu` (stubs), minimal
   `runtime` and `syscall` surfaces. This step also owns the public `runtime/cgo` package (`Handle`,
   `NewHandle`, `Value`, `Delete`: a native handle table behind the `//gocpp:native` marker; the docs
   say "Phase 09" for it, since step 5.12 found that no step had built it) (and
   `runtime.Error` and `runtime.PanicNilError`, which step 8.10 built first: `stdlib/runtime/runtime.gp`).
3. **9.3 Reflect subset** (descriptor extensions incl. zero value and element assign/swap; the
   per-package API list in §2.2; `internal/reflectlite` alias; `DeepEqual`).
4. **9.4 Port tier-1 pure packages**: `unicode/utf8`, `unicode`, `strings`, `bytes`, `strconv`,
   `errors` (replaces the Phase 05 version), `sort`, `math/bits`, `io`, `bufio`, `context`,
   `regexp`.
   **9.4a built** (ADR-072): `unicode`, `strconv` and `errors` (the Phase 05 copy is now a port; `errors.Wrapf` stays as an
   `own` file), with 24 run tests `tests/run/{strconv,unicode,errors}_{gotest,goexamples,diff}_*.gp` (Go's own test tables
   through a harness, the documented examples, sweeps over every code point and 400000 floats; expected output from real Go).
   **9.4b built** (ADR-073): `io`, `io/fs` (with `Glob` and `Sub`), `path`, `strings`, `bytes`, `bufio` and `context`, all through `tools/portstd`
   (the 9.6 copies of `io` and `io/fs` are gone). `strings` and `bytes` lack `Lines`, `SplitSeq`, `SplitAfterSeq`, `FieldsSeq` and
   `FieldsFuncSeq` until generic types (7.3, built in 7.10: the five functions are in, with Go's iterator tests); `io/fs` sorts with `slices.SortFunc` since 7.10; `strings.Builder` keeps its address as a
   `uintptr` (Go's `*Builder` field is a reference cycle under reference counting: a leak of every Builder). Tests: 339 `Test*` and 155
   `Example*` functions of Go's own test files for the seven packages through the 9.4a harness (`tests/run/{strings,bytes,io,bufio,context,path,iofs}_{gotest,goexamples}_*.gp`)
   and five differential sweeps (`*_diff_ops.gp`), expected output from real Go.
   **9.4c built** (ADR-083): `sort`, `regexp/syntax` and `regexp`, all through `tools/portstd`; `sort` is Go's files, `regexp/syntax` and `regexp` have two patch rules against reference cycles (a syntax tree whose `Sub` and `Rune` slices point into its own `Sub0` and `Rune0`, and `makeOnePass`'s self-calling closure: every `Compile` leaked its trees). `gocpp test` now matches the patterns of `-run`, `-skip`, `-list` and `-bench` with package
   `regexp` (`testing/internal/testdeps`, as Go does): `runtime/src/testing_regex.cpp` and the `std::regex` bridge are gone, and `\pL`, flags inside a pattern and non-ASCII ranges work. Tests: 28 `Test*` and 19
   `Example*` functions of `sort`, 11 of `regexp/syntax`, the 31 examples of `regexp` and all 44 of Go's internal tests of `regexp` (Fowler POSIX suite, RE2 search, one-pass, backtracker: `tests/pkgs/regexp_internal`,
   a copy of the port that a test keeps equal to `stdlib/regexp`), six differential sweeps against real Go (`sort_diff_ops`, `regexp_diff_{match,errors,unicode}`, `regexpsyntax_diff_ops`, `regexp_concurrent` with a `-race`
   twin). Speed: `sort` is 1.0x to 2.9x of Go at `-O2`, `regexp` 4x to 12x (about half of it is counted-pointer traffic: ADR-083 lists what would help).
   Step 9.4 is complete.
5. **9.5 `math`** (Go's code ported by `tools/portstd`, plus natives in `gocpp/math.hpp` and `runtime/src/math.cpp`; results bit-identical to Go on amd64, checked by 122 differential run tests `tests/run/math_*.gp` that tools/mathcorpus generates from real Go: ADR-071). Built: every exported function of `math` (`math/cmplx` and `math/rand` are not in this step); `-ffp-contract=off` was already in the language flags (ADR-006) and is now tested (`math_fma_contraction_*`, and the whole set again with `-O2 -mfma -mavx2`); the float to integer conversions of ADR-006 are tested against Go (`math_f2i_*`). Findings: Go's amd64 `Log` is wrong for subnormal numbers and gocpp copies it (one `@replace` removes it); the bits of a NaN that an operator (`+`, `min`, `max`) produces are out of the contract.
6. **9.6 `os`** (interop + native) with Go error types. (Before `fmt`, because `fmt` imports `os`.)
   *Built (ADR-067):* not "interop": Go's own `os` files for Linux (`stdlib/os/PORT.md`) over a `syscall` subset
   (`stdlib/syscall/PORT.md`) whose system calls are natives that return the errno (`runtime/src/os/linux/posix_file.cpp`,
   `gocpp/syscall.hpp`; EINTR retried, `O_CLOEXEC`), and a blocking-mode `internal/poll`. `io`, `io/fs` and
   `internal/filepathlite` are copied from Go (`io` and `io/fs` were too, until 9.4b replaced them with `portstd`'s output: declarations and
   `io.EOF` did not change). `os.Stdout`/`Stderr` are real `*os.File` values: `fmt.Fprint*` writes through them as `io.Writer`.
   Not built: `os.Root`, `StartProcess` (ENOSYS), `os/exec`, `os/signal`, deadlines, the splice/sendfile zero-copy paths (they need `net`; `ReadFrom` uses `copy_file_range` since 2026-10-23), file
   finalizers. `RemoveAll` is Go's `openat`/`unlinkat` version on Linux since 2026-10-23 (job dev.syscalldiffs); Windows keeps the portable one. Tests: `tests/run/os_*.gp` (compared with real Go) and
   `runtime/test/syscall_test.cpp`.
7. **9.7 `fmt`** port on the reflect subset (+ `internal/fmtsort`, C++ value printing).
8. **9.8 `time`** (ported logic + native hooks + zoneinfo lookup).
9. **9.9 `flag`, `testing` + `gocpp test`** (T incl. `Parallel`, B with `Loop`, examples, flags,
   Go-style output).
   *First slice built (job 9.9): `gocpp test` and `testing` without waiting for the ports of `sync`, `time`,
   `os`, `flag` and `regexp`; `flag` itself is still 9.4's. The loader generates the main package of each
   test binary (`load.Config.TestMain`, `internal/gotest`), `stdlib/testing` is Go 1.25's package in `.gp` on a
   short list of natives (`runtime/include/gocpp/testing.hpp`), and `tests/gotest` compares the output with
   real `go test` on 56 fixtures (and a few more tests: the regexp dialect, reruns, `-o`, the order of parallel tests). Decisions, differences and gaps: ADR-056, `stdlib/testing/PORT.md`,
   `docs/guides/testing.md`. A `package main` can be tested, fuzz targets run their seed corpus, and the allocation
   columns count C++ `operator new`. Left for later: `-json`, `-race`, coverage, the fuzzing engine, `T.Context`/`T.Output`
   (need `context`/`io`), vet, the goroutine dump of a timeout, the natives replaced by their ports (the list in PORT.md).*
10. **9.10 Run Go's tests** for tier 1 and the deferred suites (§2.8): test-only packages, patches
    with reasons, the counting rule; fix the gocpp bugs they find.
    *Built (ADR-113):* `GOCPP_TEST_STDLIB` adds Go's `_test.gp` files and `testdata` to the embedded package, so `gocpp test strings` runs Go's own tests in the package gocpp ships;
    `tools/portstd/gotests/convert_std.py` makes `tests/stdgo` (converted files, shims, test-only packages, one manifest each) from a specification with a reason for every patch, drop and expected failure;
    `tools/stdgocount` computes the counts of §2.8. **25 packages, 1769 test functions in Go's files, 1661 in the selected files, 1606 pass: 98.2 % of the selected functions without the 26 divergences
    (25 allocation counts, D24), and 90.8 % of all 1769.** Qualifier: **`os` is 58.6 % of all its functions (57.6 % before the review of this step); it reaches 91.1 % of its selected functions only because
    `os.Root`, deadlines, processes and fifo/zero-copy are not built (98 functions in files that need them are not taken).** Scope: **`fmt`, `sync`, `sync/atomic` and `testing` have no manifest and are outside this count**
    (the `fmt` differential tests, the sync tests of 8.7 and `gocpp test` of 9.9 cover them), so the exit criterion "all tier-1 packages at least 90 %" is evidenced by this count for the 25 listed packages only. Deferred suites:
    `slices`/`maps`/`cmp`/`iter` 145 of 150; `test/rangegen.go` **800 of 800 units** (`rangegen_shards.py`); `test/chan` 18 of 19 (`select2.go` reads `runtime.MemStats`). Found and fixed: boxed variables
    of generic instances (`slices.Collect`, `iter.Pull`), `testing.T.Setenv` bypassed `os`, `os.File.Readdir`'s stat hook, `time`'s once-flags (`sync.Once` now). Found and open: zero-size-element slices of huge length,
    `os.RemoveAll` beyond `PATH_MAX` (fixed 2026-10-23), `runtime.AddCleanup`, the builtin `print` of an interface, `defer` of a native function. Guide: `docs/guides/stdlib-tests.md`.
11. **9.11 Tier-2 packages**, `encoding/json` first.
    *Built (ADR-102):* `encoding/json` is Go's own package (`tools/portstd`, no patch rule), on the reflect subset of 9.3 as it was (nothing was added to `reflect`);
    `encoding`, `encoding/base64`, `encoding/hex`, `encoding/binary`, `container/list`, `container/heap`, `container/ring`, `hash`, `hash/fnv`, `hash/crc32`, `text/tabwriter`, `log`, `path/filepath`, `math/rand` and `math/rand/v2` are ports too (new native `runtime.rand`).
    `#Derive(JSON)` is built: `MarshalJSON` and `UnmarshalJSON` call `encoding/json` on a method-less copy of the struct, so they are exactly Go's output (an API shim, no faster than a plain struct); the file must
    import `encoding/json`; `examples/macros` runs. Tests: `tests/run/json_*.gp` (35 programs, about 4900 lines of output, compared with real Go; 17 of them, `json_syntax_offsets` to `json_panics_recursion` in the exit-criterion round, cover Marshal/Unmarshal/Valid/Compact/Indent/HTMLEscape, Encoder and Decoder options, `Token`, `RawMessage`, `Number`, the marshaler interfaces, struct tags, map key kinds, invalid UTF-8, error offsets and fields, seeded random documents and values), `derive_json_plain.gp`, and Go's own tests of each ported package run by
    `gocpp test` in a copy of the port and compared with `go test -v` (`tests/gotest/testdata/*_internal`, made by `tools/portstd/gotests/convert.py`): all 86 tests, 13 examples and 2 fuzz seeds of `encoding/json`
    pass. One compiler gap found and fixed on the way: `[]Uint8("abc")` and `string(u8s)` for a named byte or rune element. Left: `crypto/*`, `os/exec`, `hash/maphash`, `hash/adler32`, `hash/crc64`, `text/template`; `reflect.Value.Method`/`Call` (needed only by the `TestRegress` of the rand packages).
12. **9.12 Docs**: package list, what differs from Go, how to port a package.
13. **9.13 v0.1 release candidate**: tag, changelog, install notes.

### 3.1 Notes on step 9.2: what the packages import

Made from the imports of each package's `.go` files (go1.25.14, `purego`, linux), by reading the selectors that name
another package's symbols; `docs/guides/porting-blockers.md` (step 9.1) has the package-level table. Step 9.2 built the
first row group; the rest is owned by the step in the last column, and nothing here adds a file to `runtime`
(step 8.9 and 8.10 own it).

| Package | Symbols it uses from internal packages | Provided by |
|---|---|---|
| `strings` | `bytealg.{IndexByteString, CountString, CompareString, LastIndexByteString, IndexString, HashStrRev, PrimeRK, MaxBruteForce, MaxLen, Cutover, MakeNoZero}`, `stringslite.*`, `abi.NoEscape` | 9.2 (all of them) |
| `bytes` | `bytealg.{Index, IndexByte, IndexString, IndexByteString, Count, Compare, LastIndexByte, IndexRabinKarp, LastIndexRabinKarp, MaxLen, MaxBruteForce, Cutover, MakeNoZero}`; `runtime.growslice` appears only in a comment | 9.2 |
| `strconv` | `stringslite.Clone`, `bytealg.IndexByteString` (the `bootstrap` variants avoid it) | 9.2 |
| `math/bits` | `runtime.overflowError`, `runtime.divideError` (linkname: 9.1's table turns them into native panics) | 9.1 |
| `unicode/utf8`, `unicode`, `sort`, `io`, `bufio`, `context`, `regexp` | none of the internal packages above (`sort`, `errors`, `context` need `internal/reflectlite`) | 9.3, 8.7, 8.8, 7.x |
| `slices`, `maps`, `iter` | `iter`: `race.{Acquire, Release, Write}`, `runtime.{Goexit, newcoro, coroswitch}` | 9.2 (`race`), 8.9 and 7.8 (`runtime`, coroutines) |
| `math` | `cpu.X86.HasFMA`, `cpu.ARM64.HasFMA` | 9.2 (false: the portable branch) |
| `sync` | `race.{Enabled, Enable, Disable, Acquire, Release, ReleaseMerge, Read, Write}`, `runtime.{GOMAXPROCS, randn}`, `internal/sync`, `internal/synctest` | 9.2 (`race`), 8.7 |
| `time` | `godebug.New`, `stringslite.Clone`, `bytealg.IndexByte`, `runtime.{GOOS, GOROOT, usleep}`, `syscall.{Open, Read, Seek, Close, Getenv, ENOENT, ...}` | 9.2 (the first three), 8.8, 9.6 |
| `os` | `byteorder`, `goarch.BigEndian`, `itoa`, `stringslite`, `testlog.{Open, Stat, Getenv, Logger, PanicOnExit0}`, `bytealg.{CompareString, LastIndexByteString}`, `runtime.{GOOS, KeepAlive, SetFinalizer, AddCleanup, LockOSThread, rand}`, about 150 `syscall` names, `internal/poll`, `internal/filepathlite`, `internal/syscall/*` | 9.2 (the first five), 9.6 (the rest: `os` is native over interop) |
| `testing`, `flag` | `race.Errors`, `runtime.{Goexit, Callers, CallersFrames, Caller, GOMAXPROCS, ReadMemStats, GC, MemStats, SetBlockProfileRate, ...}`, `runtime/debug`, `runtime/trace`, `internal/sysinfo` | 9.2 (`race`), 8.10, 9.9 |
| `fmt` | `internal/fmtsort`, `reflect`, `os` | 9.7, 9.3 |
| `hash/fnv`, `encoding/base64` (tier 2) | `byteorder`, `bytealg` through `strings` | 9.2 |

What step 9.2 built (`stdlib/internal/PORT.md`, ADR-045): `internal/abi` (`NoEscape`, `Escape`), `internal/bytealg`
(natives and generic Rabin-Karp), `internal/byteorder`, `internal/cpu`, `internal/godebug`, `internal/goarch`,
`internal/goos`, `internal/itoa`, `internal/oserror`, `internal/race`, `internal/stringslite`, `internal/testlog`, and
`runtime/cgo`. Tests: `tests/run/shim_*.gp` (differential against real Go through twins: `tests/twin_test.go`).
Not built by this step, because it belongs to the runtime or to other steps: `runtime.Error`/`PanicNilError` and the
functions of `runtime` listed above (built by step 8.10, merged), `syscall` (9.6), `internal/reflectlite` (built by 9.3),
`internal/sync` and `internal/synctest` (8.7).

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
