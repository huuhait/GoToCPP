# The 16.1 spike: the closure of `container/list`, `strings`, `strconv` and `sort`, and the stubs

Job **16.1f-stubs** of Phase 16 (README §3 step 16.1: *"Hand-written stubs stand in for
runtime, internal/bytealg and internal/reflectlite"*). Base: Go **1.25.14** (`VERSION` of the
Go tree the analysis read), Linux/amd64, under the GC engine's build constraints of Phase 16
README §2.7 (compiler tag `gocpp`, `purego`, `math_big_pure_go`, `CGO_ENABLED=0`, the release
tags `go1.1`…`go1.25`, and go1.25's default tool tags). No `go` command is needed to *use* the
analysis; it was produced with the Go toolchain the dev shell provides.

| file | what |
|---|---|
| `closure.json` | the machine-readable analysis: the import graph, every identifier of the three stub packages the closure uses with its call sites, the linkname hooks in both directions, the bodyless declarations outside the stubs, and the summary. |
| `closure/` | the tool that produces `closure.json` from a Go source tree (`go run ./gostd/spike/closure -goroot $GOROOT -out gostd/spike/closure.json`). |
| `stub/` | the pure-Go stubs, at the directory layout the loader maps to the import paths (below). |
| `spiketest/` | the compile check: copies the four packages, their closure and the stubs into a temporary module and builds them with the real Go toolchain. |
| `LICENSE` | Go's BSD license text, for the files adapted from Go's own source (the stubs carry Go's copyright header, which points here). |

The tests next to each stub are the second half of the evidence: they run with the real Go
toolchain and compare the stubs' behavior with the real packages' (strings/bytes for
`internal/bytealg`, `reflect` and `errors` for `internal/reflectlite`).

## 1. The import graph

22 packages are translated by the GC engine; the three stub packages are replaced wholesale and
are not part of the graph (the walk stops at them). 42 direct-import edges.

```
container/list                    (no imports)
strings   -> errors, internal/abi, internal/bytealg*, internal/stringslite, io, iter,
             math/bits, sync, unicode, unicode/utf8
strconv   -> errors, internal/bytealg*, internal/stringslite, math, math/bits, unicode/utf8
sort      -> internal/reflectlite*, math/bits, slices
errors    -> internal/reflectlite*
io        -> errors, sync
sync      -> internal/race, internal/sync, internal/synctest, runtime*, sync/atomic
iter      -> internal/race, runtime*
slices    -> cmp, iter, math/bits
math      -> internal/cpu, math/bits
internal/stringslite -> internal/bytealg*
internal/abi -> internal/goarch
internal/race -> internal/abi
internal/sync -> internal/abi, internal/goarch, internal/race, sync/atomic
internal/synctest -> internal/abi
cmp, math/bits, unicode, unicode/utf8, internal/cpu, internal/goarch, sync/atomic  (leaves)
```

`*` is one of the three stub packages: `runtime`, `internal/bytealg`, `internal/reflectlite`.

The closure is:

| kind | packages |
|---|---|
| roots (4) | `container/list`, `strings`, `strconv`, `sort` |
| Go source (18) | `cmp`, `errors`, `internal/abi`, `internal/cpu`, `internal/goarch`, `internal/race`, `internal/stringslite`, `internal/sync`, `internal/synctest`, `io`, `iter`, `math`, `math/bits`, `slices`, `sync`, `sync/atomic`, `unicode`, `unicode/utf8` |

### Packages the plan's stub sentence does not name

The ticket asks that these be listed, and that each be classified as Go source or a further
stub. None of them is a fourth stub: all are Go source the engine translates, but four of them
need an overlay or a native under README §2.7, and two are new in Go 1.24/1.25 (the plan's
prose predates them).

| package | why it is here | Go source? | what the engine needs beyond translation |
|---|---|---|---|
| `internal/stringslite` | new in Go 1.24; `strings` and `strconv` route `Index`, `HasPrefix`, `Trim*`, `Cut*`, `Clone` through it, and it imports `internal/bytealg` | yes, 1 file, pure Go | nothing; it is the second direct user of the `bytealg` stub |
| `internal/synctest` | new in Go 1.25; `sync.WaitGroup` calls `IsInBubble`/`Associate` | yes, pure Go | 9 bodyless functions, all provided by runtime pushes (the stub declares them); outside a bubble they answer false/Unbubbled, which is what the spike sees |
| `internal/sync` | Go 1.25 split the hash-trie map and the mutex internals out of `sync`; `sync` imports it | yes | 8 bodyless functions: 7 runtime pushes and `runtime_rand` (a runtime pull), same treatment |
| `internal/abi` | imported by `strings` (`abi.NoEscape`) and by `internal/sync`, `internal/race`, `internal/synctest`; the source of the type-descriptor layout of README §2.8 | yes | 2 compiler intrinsics (`FuncPCABI0`, `FuncPCABIInternal`), natives per §2.7 |
| `internal/cpu` | imported by `math` (the arch-specific math functions consult it) | yes | 3 assembly functions (`cpuid`, `xgetbv`, `getGOAMD64level`); README §2.7 already lists `internal/cpu` as an overlay |
| `internal/goarch`, `internal/race` | constants and the no-race stub; imported by `internal/abi`/`internal/sync`/`sync`/`iter` | yes, pure Go | nothing under the no-`race` build |
| `math` | `strconv` imports it for the float conversions | yes | 8 assembly functions (`archMax`, `archMin`, `archExp`, `archFloor`, `archCeil`, `archTrunc`, `archHypot`, `archLog`); README §2.7 already lists `math` as an overlay (`purego` selects no fallback) |
| `sync` | `strings.Replacer` uses `sync.Once` (`internal/bytealg`'s job is done once per Replacer); `io` imports it too | yes | see the runtime stub's 42 pushes: `sync` expects 18 bodyless functions from the runtime, 15 of them pushes |
| `sync/atomic` | used by `sync`, `internal/sync`, `strings`' closure indirectly | yes | 41 bodyless functions (its `asm.s` jumps into `internal/runtime/atomic`); README §2.7 already lists `sync/atomic` as an overlay with C++ atomics. The closure also pulls 2 symbols from `internal/runtime/atomic` (`LoadAcquintptr`, `StoreReluintptr`), which §2.7 says are natives |

## 2. `internal/bytealg`: the exact set (12 identifiers, 19 uses)

Confirmed from Go's source and the type check — the ticket's guesses `IndexByte`, `Count`,
`Compare`, `Equal`, `HashStr`, `HashStrBytes` are **not** the names in Go 1.25. The real set is
`*String`/generic and is:

| identifier | kind | signature | used by |
|---|---|---|---|
| `CompareString` | func | `(a, b string) int` | `strings.Compare` |
| `CountString` | func | `(s string, c byte) int` | `strings.Count` |
| `Cutover` | func | `(n int) int` | `internal/stringslite.Index`, `strings.IndexRune` |
| `HashStrRev` | generic func | `[T string \| []byte](sep T) (uint32, uint32)` | `strings.LastIndex` |
| `IndexByteString` | func | `(s string, c byte) int` | `internal/stringslite.IndexByte`, `strconv.index` |
| `IndexRabinKarp` | generic func | `[T string \| []byte](s, sep T) int` | `internal/stringslite.Index` |
| `IndexString` | func | `(a, b string) int` | `internal/stringslite.Index` (2 call sites), `strings.IndexRune` |
| `LastIndexByteString` | func | `(s string, c byte) int` | `strings.LastIndex`, `strings.LastIndexByte` |
| `MakeNoZero` | func | `(n int) []byte` | `strings.Builder.grow` |
| `MaxBruteForce` | const | `= 64` (amd64) | `internal/stringslite.Index`, `strings.IndexRune` |
| `MaxLen` | var | `0` in the stub (amd64: 63 or 31 from `cpu.X86.HasAVX2`) | `internal/stringslite.Index` |
| `PrimeRK` | const | `= 16777619` | `strings.LastIndex` |

Semantics the closure needs, and where the real version is assembly:

- `CompareString`, `CountString`, `IndexByteString`, `LastIndexByteString`, `IndexString` are
  **assembly** in Go (compare_amd64.s, count_amd64.s, indexbyte_amd64.s, index_amd64.s) and
  `MakeNoZero` is bodyless (allocated by the runtime). The stub implements all of them in pure
  Go. `MaxLen`/`MaxBruteForce`/`Cutover` are amd64-specific data; the stub uses 0/64/(n+16)/8,
  which keeps every caller's *result* the same (0 sends `stringslite.Index` down its general
  brute-force path), only potentially slower.
- `HashStr`, `HashStrRev`, `IndexRabinKarp` are **Go source** in Go (`bytealg.go`); the stub
  copies them verbatim, generics included (the four packages use generics anyway).
- `MakeNoZero` is allowed to return zeroed memory: its contract only promises length, capacity
  and that the caller must not leak uninitialised bytes; `make([]byte, n)` is a valid
  implementation (stronger).
- `IndexRabinKarp` keeps Go's precondition `0 < len(sep) <= len(s)`; its callers in
  `internal/stringslite`/`strings` establish it.

## 3. `internal/reflectlite`: the exact set (16 identifiers, 28 uses)

`sort` needs only `ValueOf`, `Swapper` and `Value.Len`; `errors` needs the rest. This is the
confirmed set (the ticket's list guessed `Index`/`Swap`; `reflectlite` has neither — `Swapper`
returns the swap closure):

| identifier | kind | used by | semantics needed |
|---|---|---|---|
| `TypeOf(any) Type` | func | `errors.Is`, `errors.as`, `errors.errorType` init | the dynamic type of an interface value; nil for nil |
| `ValueOf(any) Value` | func | `errors.As`, `errors.as`, `sort.Slice`, `sort.SliceStable`, `sort.SliceIsSorted` | a Value over the eface's {type, data} |
| `Swapper(any) func(i, j int)` | func | `sort.Slice`, `sort.SliceStable` | swap slice elements; panic if not a slice |
| `Value.Len() int` | method | `sort.Slice` ×3 | slice length (the closure only needs slices; the stub also does array/string/chan/map) |
| `Value.Kind() Kind` | method | `errors.As` ×2 | `Ptr`, `Interface` |
| `Value.Type() Type` | method | `errors.As` | the static type |
| `Value.Elem() Value` | method | `errors.as` ×2 | deref a pointer to an interface/type |
| `Value.IsNil() bool` | method | `errors.As` | a nil pointer target is an error for `As` |
| `Value.Set(Value)` | method | `errors.as` | assign the matching error into `target` |
| `Type.Kind() Kind` | method | `errors.As` ×2 | `Ptr`, `Interface` |
| `Type.Elem() Type` | method | `errors.As`, `errors.as`, `errors.errorType` | pointer/array/slice/map/chan element |
| `Type.Comparable() bool` | method | `errors.Is` | drives the `err == target` shortcut |
| `Type.Implements(Type) bool` | method | `errors.As` | "*T implements error" |
| `Type.AssignableTo(Type) bool` | method | `errors.as` | the matching test before `Set` |
| `Type` (the type) | type | `errors.as`'s parameter | the Type interface |
| `Value` (the type) | type | `errors.as`'s parameter | the Value struct |
| `Ptr`, `Interface` | consts | `errors.As` | Kind constants |

The stub is Go's own `internal/reflectlite` (type.go/value.go/swapper.go, BSD) reduced to this
set, with three changes:

1. **The type descriptors.** Go's reflectlite imports `internal/abi` and reads `abi.Type`. A
   package in this module cannot import `internal/abi` (Go's internal rule forbids it, and the
   engine translates internal/abi separately), so `abi.go` restates `abi.Type`,
   `abi.PtrType/SliceType/ArrayType/ChanType/FuncType/StructType/InterfaceType`,
   `abi.SwissMapType`, `abi.UncommonType`, `abi.Name` and `abi.Kind` field for field at Go
   1.25's offsets (Go's `abi.Type` is 48 B on 64-bit; the doc comment of `abi.go` has the
   table). The tests below prove the offsets against the real runtime's descriptors: they hand
   real `any` values to the stub, so a wrong offset or Kind value fails immediately. The engine's
   descriptors have the same layout (README §2.8), which is what makes this a stub and not a
   rewrite.
2. **Headers.** `internal/unsafeheader` is not in the closure; the two headers are local
   (`sliceHeader`, `stringHeader`, Go's layout).
3. **The seven runtime hooks** are declared bodyless with `//go:linkname` pulls of the
   `internal/reflectlite.*` symbols Go's runtime pushes: `resolveNameOff`, `resolveTypeOff`,
   `unsafe_New`, `typedmemmove`, `ifaceE2I`, `chanlen`, `maplen` (`hooks.go`). With the real Go
   toolchain the real runtime's pushes satisfy them, which is how the same code can be tested
   against real Go. In the engine the runtime stub declares them as natives (§5).

One documented divergence: panic *text* from `Value.Set`/`Elem` names the method as
`reflectlite.Value` instead of walking the stack with `runtime.Caller` (Go's `methodName`); the
behavior and the panic type are the same.

Not stubbed, because the closure does not use them: array/map/chan/struct/func/field methods,
`Kind` beyond the constants above, method values, `MakeFunc`. A new use is a compile error.

## 4. `runtime`: what the closure uses (2 direct identifiers, 4 pulls, 42 pushes)

The four packages import `runtime` only through `sync` and `iter`:

| identifier | uses |
|---|---|
| `runtime.GOMAXPROCS` | `sync.(*Pool).pinSlow` (1 call) |
| `runtime.Goexit` | `iter.Pull` ×2, `iter.Pull2` ×2 |

`//go:linkname` pulls into the runtime (`runtime.pulls_out_of_closure`; the closure pulls 6
symbols out of the closure in all):

| pull | from | provided by |
|---|---|---|
| `runtime.newcoro`, `runtime.coroswitch` | `iter` (iter.Pull) | runtime natives: goroutine coroutines |
| `runtime.rand` | `internal/sync` (hash-trie map seed) | Go source in the stub (splitmix64) |
| `runtime.randn` | `sync.Pool` (`-race` path) | Go source in the stub |
| `internal/runtime/atomic.LoadAcquintptr`, `.StoreReluintptr` | `sync` | natives (README §2.7: "the two functions that `sync.Pool` pulls from it are natives") |

`//go:linkname` pushes from the runtime into the closure: **42 symbols** (`runtime.pushes_into_closure`,
39 definitions + the 3 bodyless `sync/atomic` uintptr declarations that the runtime references):

| target package | symbols |
|---|---|
| `sync` (18) | `event`, `throw`, `fatal`, `runtime_Semacquire`, `runtime_SemacquireWaitGroup`, `runtime_SemacquireRWMutex(R)`, `runtime_Semrelease`, `runtime_canSpin`, `runtime_doSpin`, `runtime_notifyListAdd/Wait/NotifyAll/NotifyOne/Check`, `runtime_procPin`, `runtime_procUnpin`, `runtime_registerPoolCleanup` |
| `internal/sync` (7) | `throw`, `fatal`, `runtime_SemacquireMutex`, `runtime_Semrelease`, `runtime_canSpin`, `runtime_doSpin`, `runtime_nanotime` |
| `internal/synctest` (9) | `Run`, `Wait`, `IsInBubble`, `acquire`, `release`, `inBubble`, `associate`, `disassociate`, `isAssociated` |
| `sync/atomic` (8) | `StorePointer`, `SwapPointer`, `CompareAndSwapPointer`, `runtime_procPin`, `runtime_procUnpin` (definitions) and `StoreUintptr`, `SwapUintptr`, `CompareAndSwapUintptr` (the runtime's own bodyless references to the package's assembly) |

The stub (`stub/runtime/`) provides:

- **Go source with real bodies**: `GOMAXPROCS`, `rand`, `randn`, and the push bodies in
  `hooks_body.go` that the closure's ordinary path runs: `sync`'s init
  (`runtime_notifyListCheck`, `runtime_registerPoolCleanup`), `canSpin=false`/`doSpin`=no-op,
  `procPin`=0/`procUnpin`=no-op (the stub has one P: `GOMAXPROCS(0) == 1`, so a single-P
  `sync.Pool` works), the synctest "not in a bubble" answers
  (`IsInBubble=false`, `associate=Unbubbled`, `isAssociated=false`), and the three pointer
  atomics, which are expressed over the `sync/atomic` uintptr forms exactly as Go's runtime
  does.
- **Natives (bodyless)**: `newcoro`, `coroswitch`, and the seven `internal/reflectlite.*`
  hooks.
- **Honest panics**: the semaphore and notify-list waits (`sync`'s *uncontended* fast paths
  never reach them; `strings.Replacer`'s `sync.Once` takes the mutex only uncontended), synctest
  `Run`/`Wait`/`inBubble`, and `runtime.Goexit` (which is a panic until step 16.6). The message
  names the native and the step.

The runtime stub's push table is split by build constraint: `hooks_body.go` holds the 39
definition bodies without directives, and `hooks_link.go` (`//go:build gocpp`) attaches their 39
linker names with `//go:linkname` (the other 3 of the 42, the `sync/atomic` uintptr references,
are bodyless pulls in `hooks.go` and need no body). That is necessary because with the real `gc` toolchain the real runtime already
defines those symbols (a body would be a duplicate symbol at link, and Go 1.25's linkname check
forbids most of them from a package that is not their target), while the engine's compiler tag
is `gocpp`.

The engine also needs the two //go:linkname pushes *out of* the closure, which the analysis
records in each package's `linkname` list: `math/bits` pushes the variables
`runtime.overflowError`/`runtime.divideError` (they appear in the runtime's arithmetic panic
text), and `sync` pushes `syscall.hasWaitingReaders`. They are not the stub's job, but the
engine's linkname resolution must accept them (README §2.7's push rules).

## 5. The stubs' directory layout and the loader mapping

```
gostd/spike/stub/runtime/              <- import path "runtime"
gostd/spike/stub/internal/bytealg/     <- import path "internal/bytealg"
gostd/spike/stub/internal/reflectlite/ <- import path "internal/reflectlite"
```

The loader maps an import of `runtime`, `internal/bytealg` or `internal/reflectlite` to
`gostd/spike/stub/<import path>` and takes the package name from the source (`runtime`,
`bytealg`, `reflectlite`). The layout keeps Go's `internal` rule meaningful: only code under
`gostd/spike/stub/` can import the two internal stubs, so the stubs' tests live next to them.

`spiketest` does the same mapping in a temporary module named `std`: every closure package P is
copied to `std/<P>` and every import of a copied package or a stub in the copied source is
rewritten to `std/<path>`. It then runs `go build` of the four roots twice, once with the plain
`gc` constraints and once with `-tags gocpp` (so the runtime stub's `//go:build gocpp` push
wrappers are compiled too), with `-gcflags=all=-complete=false` because the copied `sync` is no
longer recognised by `cmd/go` as the standard `sync` package whose forward declarations come
from the runtime (the same flag lets the stubs' own bodyless natives compile). This is a compile
and type check, not a link: bodyless natives are resolved by the engine. With the real
toolchain the stubs' `//go:linkname` references resolve to the real runtime, which is what lets
the tests in `stub/` link and run.

## 6. What 16.1f must implement on the C++ side

The Go sources above are enough to *compile* the four packages; these are the places where the
engine has to step in. Everything else stays Go source (the stubs included).

1. **The runtime interface used by the closure** (natives over the shared runtime):
   `runtime.GOMAXPROCS` (the thread cap), `runtime.Goexit` (unwind, run defers, step 16.6),
   `runtime.newcoro`/`runtime.coroswitch` (iter's coroutines over engine goroutines, step 16.8),
   and, if `sync.Pool`/the hash-trie map are reached, `runtime.rand`/`randn` (the pure-Go
   versions are fine).
2. **The 42 runtime pushes** (hooks_body.go's bodies): the semaphores and notify lists
   (`sync`, `internal/sync`, over the engine's semaphore table of step 16.4), the synctest hooks
   (step 16.10), the pointer atomics (C++ atomics with the collector's barriers), and
   `procPin`/`procUnpin`. The bodies in `hooks_body.go` document which are real Go source for
   the spike and which are placeholders; the placeholders panic with the native's name.
3. **The seven `internal/reflectlite.*` hooks**, which are what makes the `reflectlite` stub
   work (their Go side is already written):
   - `resolveNameOff(*abi.Type, int32) *byte` and `resolveTypeOff(*abi.Type, int32) *abi.Type`:
     the module-data pointer tables of README §2.8 (`resolve*Off` natives);
   - `unsafe_New(*abi.Type) unsafe.Pointer`: an allocation of `Size_` zeroed bytes in the GC
     heap;
   - `typedmemmove(*abi.Type, dst, src)`: a copy of `Size_` bytes (Boehm needs no write
     barrier; a moving/generational collector would need `PtrBytes`/`GCData`);
   - `ifaceE2I(*abi.Type, any, unsafe.Pointer)`: build a non-empty interface value, i.e. the
     itab for (concrete type, interface) — `getitab` of README §2.8;
   - `chanlen`/`maplen`: only `Value.Len` on channels/maps uses them; the closure uses slices
     and strings.
4. **The 91 bodyless declarations outside the stubs** (`natives_outside_stubs`) split exactly as
   39 + 52:
   - **39 are entries of the 42-symbol push table that land on a bodyless declaration**: 15 in
     `sync`, 7 in `internal/sync`, 9 in `internal/synctest`, 8 in `sync/atomic`. Of the 8
     `sync/atomic` entries only 5 are runtime push *definitions*; the other 3 (`StoreUintptr`,
     `SwapUintptr`, `CompareAndSwapUintptr`) are the package's own assembly, which the runtime
     only *references* bodylessly — `closure.json` records them with `"kind": "pull"` and
     `provided_by: "the target package's own symbol (runtime references it as …)"`.
   - **52 are everything else**: `math` 8 and `internal/cpu` 3 (the overlays README §2.7
     already names), `sync/atomic` 33 (the rest of its `asm.s`, which jumps into
     `internal/runtime/atomic` — the C++ atomics overlay of §2.7), `sync` 3 (`runtime_randn` and
     the two `internal/runtime/atomic` pulls), `internal/sync` 1 (`runtime_rand`), `internal/abi`
     2 (the compiler intrinsics `FuncPCABI0`/`FuncPCABIInternal`), and `iter` 2 (`newcoro`,
     `coroswitch`).
   - The two symbols `sync` pulls from `internal/runtime/atomic`
     (`LoadAcquintptr`/`StoreReluintptr`) are **not** among the 91: their package is outside the
     closure, so they are targets, not declarations. They are listed in
     `runtime.pulls_out_of_closure` and are natives per README §2.7.
   The full list with signatures and source lines is `natives_outside_stubs` in `closure.json`.

Not needed by the four packages and deliberately absent: reflection of arrays/maps/structs,
`reflect.Value.Call`, finalizers/cleanups, `os`, `fmt`. The binarytrees program of the spike
prints with `println`, so it adds no closure.

## 7. Tests and how to reproduce

```sh
GOROOT=$(scripts/in-devenv go env GOROOT)

# 1. the analysis (writes the committed JSON)
GOMAXPROCS=16 go run ./gostd/spike/closure -goroot "$GOROOT" -out gostd/spike/closure.json

# 2. the stubs' behavior against the real packages
go test -count=1 ./gostd/spike/stub/...      # bytealg vs strings/bytes, reflectlite vs
                                             # reflect/errors, runtime stub hooks

# 3. the compile check: the four packages + closure + stubs in a temporary module
go test -count=1 -run TestCompile ./gostd/spike/spiketest
```

What the tests cover:

- **bytealg**: `IndexByteString`/`LastIndexByteString`/`CountString`/`CompareString`/
  `IndexString` against `strings`/`bytes` on ASCII, UTF-8, NUL/0xFF, empty strings and 1,000-byte
  strings; 500 random string pairs against `strings.Index` and `bytes.Index` through
  `IndexString` and `IndexRabinKarp` (both the `string` and the `[]byte` instantiation);
  `HashStrRev` against `HashStr` of the reversed bytes; the constants and `Cutover`;
  `MakeNoZero`'s length/capacity/writability.
- **reflectlite**: `Kind`, `Type().Size/Name/PkgPath/String/Comparable`, `Elem`, `Len`,
  `IsNil` against `reflect` over 30 values of all kinds; `Implements`/`AssignableTo` against
  `reflect.Type`; `Swapper` against `reflect.Swapper` for `[]int`, `[]string`, `[]byte`,
  `[]int16`, `[]int64`, `[]*int`, `[]struct{int;string}` and `[]*struct`, including the
  memmove/`unsafe_New` fallback, plus the panic cases; `Set` of a concrete value into `*any`,
  `*error` (the `ifaceE2I` path) and a same-typed target, against `reflect`. Then the whole of
  `errors.Is`/`errors.As` is re-implemented on the stub (as `errors/wrap.go` does) and compared
  with the real `errors` package over wrapped, joined and custom-`Is` error trees, including
  `As` into `*error`, `*any` and `*concrete`.
- **runtime**: `GOMAXPROCS`, `rand`/`randn`, `notifyListCheck`'s size check,
  `registerPoolCleanup`, the synctest "no bubble" answers and the pointer atomics (which call
  the real `sync/atomic` through the stub's uintptr pulls).
- **compile**: as §5, plus the drift guard `TestStubsDeclareTheAnalysisAPI`: every one of the 30 stub identifiers in `closure.json` must be declared by the stub source with the same kind (a renamed or dropped stub identifier fails before the engine does), and `TestClosureCoversStubs` checks that the JSON still lists the 22 packages and the three stubs as such.

## 8. Numbers

| number | value |
|---|---|
| packages to translate | 22: 4 roots + 18 imports |
| stub packages | 3 (not translated) |
| direct-import edges | 42 |
| `internal/bytealg` identifiers / uses | 12 / 19 |
| `internal/reflectlite` identifiers / uses | 16 / 28 |
| `runtime` identifiers used directly / uses | 2 / 5 |
| linkname pulls out of the closure | 6: 4 into `runtime`, 2 into `internal/runtime/atomic` |
| linkname pushes into the closure + stubs | 42: 39 definitions (18 `sync`, 7 `internal/sync`, 9 `internal/synctest`, 5 `sync/atomic`) + 3 `sync/atomic` uintptr references |
| linkname pushes out of the closure | 3: `math/bits` → `runtime.overflowError`/`divideError` (variables), `sync` → `syscall.hasWaitingReaders` |
| external packages behind the stubs (type-checked, not translated) | 20: runtime's 19 internal dependencies plus `internal/unsafeheader` through `internal/reflectlite` |
| bodyless declarations outside the stubs | 91 = 39 push-table entries (15 `sync`, 7 `internal/sync`, 9 `internal/synctest`, 8 `sync/atomic`) + 52 others (`sync/atomic` 33, `math` 8, `sync` 3, `internal/cpu` 3, `internal/abi` 2, `iter` 2, `internal/sync` 1) |
| C++ natives 16.1f must provide for the stubs | 9: 2 coroutines + 7 reflectlite hooks; plus the 2 `internal/runtime/atomic` symbols `sync` pulls, and the §6.2 push bodies for the semaphores, notify lists, synctest and atomics (16.4/16.10) |

## 9. Limits, and what is not covered

- The analysis is **Linux/amd64, Go 1.25.14** under the tag set above. Windows differs in the
  closure only by `internal/cpu`'s file selection (the same 3 assembly functions) and the
  `unix` tag; the stubs are OS-independent.
- `_test.go` files of the Go packages are not analysed (as in the dry run): they use a larger
  bytealg/reflectlite API, which the engine's `testing` work (step 16.12) will need later.
- The `reflectlite` stub is faithful for the used subset, and its descriptor mirror is validated
  against the real runtime's descriptors by the tests. It is not a `reflect` replacement: no
  method values, no `In`/`Out`/`Key`/field access.
- The runtime stub's semaphores, notify lists and synctest bodies are placeholders that panic;
  the closure's ordinary path does not reach them (uncontended `sync.Mutex`/`Once`), but any
  contended use in the spike will panic with the native's name. That is deliberate: a silent
  wrong answer would be worse.
- `iter.Pull` cannot work before `newcoro`/`coroswitch` are natives; the four packages only
  *build* `iter`, they do not call `Pull`.
- The compile check builds with the real `gc` toolchain, so the engine's `gocpp`-specific build
  constraints are approximated by the tag set (`-tags gocpp`); the analysis itself uses the
  exact constraints of README §2.7.
