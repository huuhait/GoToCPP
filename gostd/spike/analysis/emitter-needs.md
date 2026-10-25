# The GC emitter's needs: `container/list`, `strings`, `strconv`, `sort` and binarytrees

Job 16.1f2-plan. Base branch `p16`, branch `job/16.1f2-plan`. Read with the phase-16 README
§2.2, §2.3, §2.7, §2.8 and §3 step 16.1. Nothing here is implemented: this is the scope, the
evidence and the reusable corpus for the 16.1f jobs.

Sources: Go **go1.25.14** from the dev shell (`GOROOT` below), the four packages' own files (no
`_test.go`, no `makeisprint.go`/`gen_sort_variants.go`), the closure under the tags of README
§2.7 (compiler `gocpp`, `purego`, `math_big_pure_go`, linux/amd64), the 16.1c emitter
(`internal/cppgen/gc.go` on `p16`) and `gocpp run -gc-engine` built from this worktree.

```
GOROOT=/nix/store/26p9xn49sqb8rbbd2k6jbnzn0rkx3z29-go-1.25.14/share/go
```

## 0. The short version

- **Today** (16.1c): of the 22 programs in `programs/`, only `00_baseline` compiles with
  `gocpp run -gc-engine` (it prints exactly what Go prints). Every other program stops at a
  positioned error, listed in §4. The emitter's subset is one package, no imports, no
  package-level declarations, three types (`int`, `string`, `bool`), no selectors, no
  composite literals, no pointers, no `switch`, no `range`, no `go`/`defer`, no multiple
  results and no methods.
- **The four packages need, roughly, all of Go's language.** The closure is not the three
  hand-written stubs the plan names: `strings` imports `io`, and `io` imports `sync`, so the
  spike's `strings` build pulls `sync` (goroutines, `defer`/recover, unsafe, `//go:linkname`,
  zero-size fields), `iter` (generic `Seq` types, `//go:linkname newcoro/coroswitch`),
  `slices`/`cmp` (79 generic functions), `unicode` (six package-level **map** literals and
  10k lines of tables), `math` and `internal/cpu`.
- **Generics are not optional.** Go 1.25's `sort.Ints/Strings/Float64s` delegate to
  `slices.Sort` (§3.13); `strconv.quote.go` declares and instantiates a generic `bsearch`;
  `sync.OnceValue`, `sync/atomic.Pointer[T]` and `internal/sync.HashTrieMap` are generic. Step
  16.1 names neither generics, `iter`, range-over-func nor variadic functions.
- **Maps are needed after all — through `unicode`, not the four packages.** None of the four
  uses a map; `unicode/tables.go` declares six package-level map literals
  (`map[string]*RangeTable`, `map[string]string`). 16.1e's `gp::gc::map` is only a placeholder
  (its own comment says so); 16.1f must implement map creation, literal insertion, lookup,
  assignment, `delete`, `len` and `range`, or document a stub/skip of unicode's variables.
- **The measured binarytrees imports `os` and `time`** (`bench/gc/binarytrees/main.gp:22-25`,
  `:136`, `:152`), which drags `syscall`, `internal/poll`, `time` and more. The spike's
  "binarytrees with builtin `println`" is the no-import program `16_binarytrees`; using the
  measured file for 16.1f would need a much larger closure.

## 1. Method and what was read

| What | How |
|---|---|
| Packages and files | `go list -f '{{.GoFiles}}'` in GOROOT/src under the §2.7 tags |
| Constructs and positions | an AST + `go/types` scan (`go/build` selection, source importer) of each package, plus `grep` for `defer`, `go`, `select`, `map[`, `//go:linkname`, `func init`, tags and `unsafe.` |
| Today's support | `internal/cppgen/gc.go` read in full; every program run with `gocpp run -gc-engine` (the exact outputs are in `programs/manifest.json`) |
| Oracle | `go run .` on every program, twice, in the dev shell; the expected output in the manifest is Go's output |

The corpus is `programs/`: 22 folders, one `main.go` each (Go 1.25 language level, no network,
no `fmt`; they print with the built-in `print`/`println`, so output goes to stderr and is
deterministic). `programs/manifest.json` maps name → constructs → imports → expected output →
today's gocpp result; `programs_test.go` runs all of them with `go run` and, when `GOCPP` is
set, checks the recorded gocpp result; `genmanifest/` regenerates the manifest.

## 2. The import closure

Counted with the four packages as roots and `runtime`, `internal/bytealg`,
`internal/reflectlite` as the walk's stops (the plan's three stubs; 16.1f-stubs walks the same
graph for the identifiers). **Lines are the files `go list -f '{{.GoFiles}}'` selects for
linux/amd64** (so `//go:build ignore` files such as `strconv/makeisprint.go` and
`sort/gen_sort_variants.go` are out, and the architecture files are in), summed with `wc -l`.
In this closure the only **compiler-tag-sensitive** file is `internal/abi/funcpc.go`
(`!gccgo`, included under `gocpp` too); experiment-gated files such as
`internal/abi/map_select_swiss.go` are part of the §2.7 tag set, and no package here has a
`gc`-only file.

| Package | Lines | Imports (under §2.7 tags) | Why it is in the closure | Disposition |
|---|---|---|---|---|
| `container/list` | 235 | — | root | translate |
| `strings` | 2 455 | errors, internal/abi, internal/bytealg, internal/stringslite, io, iter, math/bits, sync, unicode, unicode/utf8, unsafe | root | translate |
| `strconv` | 5 251 | errors, internal/bytealg, internal/stringslite, math, math/bits, unicode/utf8 | root | translate |
| `sort` | 1 457 | internal/reflectlite, math/bits, slices | root | translate |
| `errors` | 299 | internal/reflectlite, unsafe | `errors.New` in strings/reader.go:53 and strconv/atoi.go:21 | translate |
| `io` | 1 065 | errors, sync | `strings.Reader`, `io.Writer`/`io.StringWriter` in reader.go:9 and replace.go:8 | translate |
| `sync` | 1 680 | internal/race, internal/sync, internal/synctest, runtime, sync/atomic, unsafe | `sync.Once` in strings/replace.go:15 | translate (runtime stubs, atomic overlay) |
| `sync/atomic` | 770 | unsafe | sync, internal/sync | **overlay** (README §2.7: C++ atomics); the overlay's own Go source (generic `Pointer[T]`, the zero-size shapes) is what gets emitted |
| `internal/sync` | 1 010 | internal/abi, internal/goarch, internal/race, sync/atomic, unsafe | sync.Map → HashTrieMap | translate |
| `internal/race` | 66 | internal/abi, unsafe | sync, internal/sync, iter (`!race` build) | translate |
| `internal/synctest` | 105 | internal/abi, unsafe | sync | translate |
| `iter` | 471 | internal/race, runtime, unsafe | `iter.Seq[string]` in strings/iter.go:18 and slices | translate (runtime stub) |
| `slices` | 1 799 | cmp, iter, math/bits, unsafe | sort/sort.go:170-197 | translate |
| `cmp` | 77 | — | slices | translate |
| `unicode` | 10 335 | — | strings/iter.go:9, strings/strings.go:14 | translate (six map vars) |
| `unicode/utf8` | 540 | — | strings, strconv/quote.go:10 | translate |
| `math` | 6 046 | internal/cpu, math/bits, unsafe | strconv/atof.go:13, eisel_lemire.go:21, ftoa.go:13 | **overlay** (README §2.7: its assembly has no pure-Go fallback — the bodyless `archMax`/`archMin`/`archExp`/`archFloor`/`archCeil`/`archTrunc`/`archHypot`/`archLog`/`archModf` are what the overlay replaces); the overlay's Go source is emitted |
| `math/bits` | 693 | unsafe | strings, strconv, sort, slices, math | translate |
| `internal/abi` | 1 425 | internal/goarch, unsafe | strings/builder.go:8 (`abi.NoEscape`) | translate (`FuncPC*` are natives, §2.7) |
| `internal/goarch` | 107 | — | internal/abi, internal/sync, internal/race, iter | translate |
| `internal/unsafeheader` | 37 | unsafe | internal/reflectlite | only with a real reflectlite |
| `internal/stringslite` | 150 | internal/bytealg, unsafe | strings/clone.go:8, strconv/atoi.go:9 | translate |
| `internal/cpu` | 483 | unsafe | math, internal/bytealg | **overlay** (README §2.7; bodyless `cpuid`, `xgetbv`, `getGOAMD64level` in cpu_x86.go:12-18). Once math and bytealg are replaced it may not be emitted at all |
| `internal/bytealg` | 297 | internal/cpu, unsafe | strings, strconv | **stub** (plan); must be behavior-correct, see §7.5 |
| `internal/reflectlite` | 1 201 | internal/abi, internal/goarch, internal/unsafeheader, runtime, unsafe | sort/slice.go:8, errors | **stub** (plan); must work, see §7.6 |
| `runtime` | 71 836 | 20 internal packages | iter, sync, internal/sync, reflectlite | **stub** (plan) |

The order in which the packages can be translated (a package only after its imports): `cmp`,
`internal/goarch`, `math/bits`, `unicode/utf8`, `internal/abi`, `internal/stringslite` (with the
bytealg stub), `errors` (reflectlite stub), `unicode` (maps), `math` (cpu overlay), `iter`
(runtime stub), `slices`, `sync/atomic`, `internal/race`, `internal/synctest`, `internal/sync`,
`sync`, `io`, `strconv`, `sort`, `strings`, `container/list`.

## 3. The construct table

"Today" is the 16.1c emitter on `p16`. "Yes" means it emits Go's C++ for the construct; a
positioned error otherwise (`internal/cppgen/gc.go`, verified by the program in the last
column). `Program` is the corpus file that covers it.

### 3.1 Package structure and initialization

| Construct | Evidence (Go 1.25) | The emitter must | Today | Program |
|---|---|---|---|---|
| Imports, aliased or blank | `strings/builder.go:7-12`, `strconv/atoi.go:8-12`, `sort/sort.go:9-12`; `container/list` has none | Resolve each package (16.1f-load), emit one `gopkg_<escaped path>` namespace per package (README §2.2; `gcNamespace`), write declarations in import order, and name a selected identifier `gopkg_x::Name`. Selector imports use the *package's* names, not the local alias | no: `main.go:5:8: could not import ...` | 17-21 |
| Package-level `const`, typed and untyped | `strings/strings.go:18` (`int(^uint(0)>>1)`), `strconv/atoi.go:63,66,68`, `sort/sort.go:59-66` | Constant expressions over constants only, Go's typing rules (untyped → typed conversion at use), exported names; emit `constexpr` or a C++ constant of Go's type | no: `GenDecl` other than `var` is refused (`gc.go:94-99`) | 01, 02 |
| `iota` and implicit repetition | `sort/sort.go:62` (`unknownHint sortedHint = iota`), 01_decls | Value of `iota` per ConstSpec in a block, repeated expression lists, typed constants | no | 01 |
| Package-level `var`, with and without type | `strconv/atoi.go:21,24` (`= errors.New(...)`), `strconv/ftoa.go:22-23`, `strconv/isprint.go:11,226,362,619,735`, `strings/strings.go:377`, `unicode/tables.go:9,51,5192,7389,9440,9740` | Mutable globals in Go's layout; initialization at startup in Go's dependency order (imports first, then vars in file/declaration order); pointer-containing globals are GC roots (bdwgc static roots, README §2.4); large constant tables may be static data, but the maps and `errors.New` values are heap objects | no | 01, 05, 21 |
| `func init` and its order | `sync/pool.go:296` (registers the pool cleanup), `sync/runtime.go:53` (checks `notifyList`'s size); none in the four themselves | Run each package's init functions after its vars and its imports' initialization, file order, before `main`; several inits per package | no: `gc.go:80-85` refuses it (an emitted but uncalled init would be silently wrong) | 01 |
| Several files per package | `strings`: 8 files, `strconv`: 14, `sort`: 5 | One namespace, one init order; C++ prototypes across files (the emitter already writes prototypes) | partial: `EmitGo` takes `[]*ast.File`, `load.LoadGoDir` one folder, no imports | 01 |
| Bodyless function declarations (assembly and native hooks) | `math/dim_asm.go:11,15`, `math/exp_asm.go:11`, `math/floor_asm.go:11,15,19`, `math/hypot_asm.go:11`, `math/log_asm.go:11`; `sync/atomic/doc.go:71,77,83,87,93,…` and `doc_64.go:14,…`; `internal/abi/funcpc.go:22,31` (`FuncPCABI0`/`FuncPCABIInternal`); `sync/runtime.go:52` (`runtime_notifyListCheck`); `sync/pool.go:83` (`runtime_randn`); `iter/iter.go:232,235` (`newcoro`/`coroswitch`) | Every bodyless declaration must resolve to a translated symbol, a native or an overlay (README §2.7: the `//go:linkname` pull/push tables; `abi.FuncPC*` are compiler intrinsics; `tools/gostdcheck` checks that each bodyless function has one). Emitting a declaration with no body and no resolution must be an error naming the symbol, never a C++ link error. `math` and `sync/atomic` are overlays, so their bodyless sets are replaced; `internal/abi`'s two `FuncPC*` are natives | no: `gc.go:318-320` refuses a function without a body | 18 (through `sync.Once`); the rest are only reached through the closure |
| `package main` entry | `16.1c` writes `int main()` calling `gopkg_main::main` | Keep it; when packages are emitted, exactly one `main` | yes | 00 |

### 3.2 Types and layout

| Construct | Evidence | The emitter must | Today | Program |
|---|---|---|---|---|
| Struct types, fields, grouped names | `container/list/list.go:15-27` (`next, prev *Element; list *List; Value any`), `sort/sort.go:85-88` (func fields), `strconv/atoi.go:27-31` (error field), `strconv/ftoa.go:16` | Go's field offsets, alignment, padding and `sizeof` (README §2.2); Go's field selection, struct assignment copies the value; exported/unexported names escaped | no: `type` declarations refused | 06 |
| Embedding and promotion | `sort/sort.go:90-95` (`reverse` embeds `Interface`), `io/io.go:690,696` (embed `Reader`/`Writer`), `sync/pool.go:73`, `io/pipe.go:17,126` | Embedded field named by its type; promoted fields and methods (`r.Len()` on `reverse` = `r.Interface.Len()`); the embedded interface occupies Go's 16-byte interface field | no | 07, 20 |
| Struct tags | none in the four or the closure reachable without reflection (the backticks in `strconv/quote.go:47` are raw strings) | Store the tag with the field's descriptor for `reflect` (16.11), not needed to compile these packages | no | 06 (marker) |
| Zero-size fields and trailing padding | `sync/mutex.go:31` (`_ noCopy`), `sync/once.go:21`, `sync/atomic/type.go:47-53` (`_ [0]*T`, `_ noCopy`), `sync/waitgroup.go:49` | Go's zero-size rules: `[[no_unique_address]]` members of distinct empty types, one offset for two zero-size fields, explicit trailing padding (README §2.2; 16.1e's golayout) | no | 06 |
| Named basic types and named slice types | `sort/sort.go:59` (`sortedHint int`), `:68` (`xorshift uint64`), `:123,134,156` (`IntSlice []int`, `Float64Slice`, `StringSlice`), `unicode/letter.go:64` (`SpecialCase []CaseRange`) | A distinct C++ type with Go's underlying layout; conversions to/from the underlying type; methods on it | no | 02, 20, 21 |
| Type aliases | `internal/abi/map_select_swiss.go:22` (`type mapType = SwissMapType`; §2.7 turns the `swissmap` experiment on), `internal/abi/map_select_noswiss.go:10` under the other tag | `type A = B` is not a new type: same identity, same method set, no descriptor of its own; emit a C++ alias (`using A = B`), unlike the named `type A B`. An alias of a native type needs no code | no: the emitter has no alias or named-type support | — (internal/abi) |
| Arrays | `strings/strings.go:377` (`[256]uint8` keyed literal), `:960` (`asciiSet [8]uint32`), `strings/replace.go:438` (`byteReplacer [256]byte`), `strconv/decimal.go:18` (`d [800]byte`), `strconv/eisel_lemire.go:187` (`[...][2]uint64`) | Fixed-length, value semantics on assignment and as struct fields; keyed literals with indices; `len` is a constant; `[...]` infers the length; indexing is bounds-checked | no | 04 |
| Slices | `sort/sort.go:123`, `strings/strings.go:291` returns `[]string`, `strconv/ftoa.go:59` (`dst []byte`) | `gp::gc::slice<T>` (16.1e), 24 bytes; make/append/copy/sub/cap/index; nil versus empty | no | 04 |
| Strings | everywhere (`strings/strings.go`, `strconv/atoi.go:73`) | `gp::gc::string`, 16 bytes, immutable bytes; indexing yields `byte`; slicing keeps the backing bytes; concatenation allocates | partial: value, `len`, `+`, comparison; no indexing/slicing/range | 03 |
| Maps | `unicode/tables.go:9,51,5192,7389,9440,9740` | `gp::gc::map<K,V>`, 8 bytes, nil map; hash and equality by Go's type rules; insertion at init; lookup (`v, ok`), assignment, `delete`, `clear`, `len`, `range` in randomized order; nil map reads are fine, writes panic | no: `map[string]int` is a refused type | 05, 21 |
| Pointers | `container/list/list.go:20,31-56`, `strings/replace.go:123` (`trieNode` pointers), `sort/sort.go:70` | `T*`, 8 bytes; `&x`, `*p`, `nil`; nil dereference panics at the Go line; pointer equality; a pointer keeps its object alive | no: `&`/`*` operators and pointer types are refused; only the `nil` literal is emitted | 06, 17 |
| `any` / empty interface and non-empty interfaces | `container/list/list.go:26` (`Value any`), `sort/sort.go:17`, `strings/replace.go:21` | The 16-byte interface value and itab/type descriptor of 16.1e; boxing (`[]interface{}`, `any(int)`); dynamic dispatch | no | 08, 17 |
| `error` and custom error types | `strconv/atoi.go:27-37` (`NumError` with `Error() string`, `Unwrap() error`), `strings/reader.go:53` | `error` as a normal interface; method sets; `errors.Is/As` if a program calls them | no | 19 |
| Float, complex, byte, rune, sized ints | `strconv/atof.go`, `ftoa.go`, `ctoa.go`, `strconv/atoi.go:16` (`byte`), `strconv/quote.go:41` (`rune`) | Go's exact widths and conversions; float32 rounding; IEEE behavior (`NaN != NaN`, `f != f` in `sort/sort.go:148-150`); complex as two float64/float32 | no: only `int`, `string`, `bool` | 02, 19 |
| Function types | `sort/sort.go:85-88` (`lessSwap` struct of funcs), `sort/search.go:58` | `gp::gc::func` (16.1e), 8 bytes; nil func call panics | no | 09, 20 |
| Channel types | binarytrees (`16_binarytrees`), `io/pipe.go:17` | `gp::gc::chan` (16.1e), 8 bytes; directional types are the same value | no | 15 |

### 3.3 Methods and interfaces

| Construct | Evidence | The emitter must | Today | Program |
|---|---|---|---|---|
| Value receivers | `sort/sort.go:97` (`reverse.Less`), `:125-127,136,144-145,158-160` (`IntSlice`/`Float64Slice`/`StringSlice`), `strings/replace.go:318` (`appendSliceWriter.WriteString`) | Method takes a copy of the receiver; the method set of `T` and `*T` both contain it | no | 07 |
| Pointer receivers | all 22 of `container/list/list.go:31,39,54,62-230`; `strings/builder.go:32-112`; `strings/reader.go:25-156`; `strings/replace.go:39-101`; `strconv/atoi.go:33,37`; `sort/sort.go:70` | Method takes `T*`; auto-address of an addressable value (`c.Inc()`); mutation visible; method set of `*T` only | no | 07 |
| Method values | `strings/replace.go:96,102` (`r.once.Do(r.buildOnce)`), `strings/strings.go:773,779,785` (`Map(c.ToUpper, s)`) | Bind the receiver at the point the value is made (a copy for a value receiver), then call later | no | 07 |
| Method expressions | not used by the four; the corpus uses `(*Counter).Inc` and `Counter.Get` (07) | `T.M` is a func with the receiver as its first parameter | no | 07 |
| Promoted methods through embedding | `sort/sort.go:97-103` (`reverse` embeds `Interface`; only `Less` is overridden) | Look through embedded fields/interfaces for a method; the promoted method's receiver is the embedded value | no | 07, 20 |
| Interface declarations, method sets, satisfaction | `sort/sort.go:17-28`, `strings/replace.go:21-24`, `io/io.go:86-137`, `io/io.go:645` (`var _ ReaderFrom = discard{}`), `sync/mutex.go:37` | Build the itab for each (interface, concrete type) pair the program uses; check satisfaction at the assignment/conversion; interface-to-interface conversion | no | 08 |
| Dynamic dispatch | `sort/sort.go:50-57` (`data.Len/Less/Swap`), `strings/replace.go:101` (`r.r.WriteString`) | Call through the itab; a nil interface call panics | no | 08 |
| Type assertions and type switches | `strings/replace.go:323` (`w.(io.StringWriter)`), `strconv/atoi.go:218-220,274` (`r.(*NumError)`), `io/io.go:315`; type switches in `errors/wrap.go:61,125` | Comma-ok and direct assertion (direct panics on failure); compare type descriptors; a type switch with a nil case; a non-interface assertion is a compile error in Go | no | 08 |
| `any` in APIs | `container/list` `Value any`, `sort.Slice(x any, ...)` (`sort/slice.go:24`) | The empty interface's descriptor, boxing and unboxing | no | 08, 17, 20 |

### 3.4 Pointers and nil

| Construct | Evidence | The emitter must | Today | Program |
|---|---|---|---|---|
| Address-of, dereference, nil | `container/list/list.go:54-56` (ring: `l.root.next = &l.root`), `:62` (`new(List)`), `:108-116` (nil out links), `strings/replace.go:190` (`new(trieNode)`) | `&x` only for addressable values; `*p`; `nil` for pointers, slices, maps, funcs, chans, interfaces; nil checks where Go panics | partial (nil literal only) | 06, 17 |
| Pointers into arrays/structs and interior pointers | `container/list/list.go:31-33` (`&e.list.root`), `strings/replace.go` trie links | Pointers to fields and elements; they keep the object alive and compare by address | no | 06 |
| Pointer equality and identity | `strings/builder.go:40` (`b.addr != b`), `sort/sort.go:90-103` | Address comparison; `&a` and `&a` equal; two zero-size objects compare as Go says | no | 06 |

### 3.5 Strings, slices and arrays

| Construct | Evidence | The emitter must | Today | Program |
|---|---|---|---|---|
| `make` | `strings/strings.go:28` (`make([]string, n)`), `strings/replace.go:75,191,292,331,536`, `strconv/decimal.go:31`, `strconv/quote.go:417` (`make([]byte, 0, cap)`) | `gp::gc::make_slice`/`make_map`/`make_chan`; len/cap; Go's zeroing; allocation-failure panic | no | 04, 05 |
| `append` | `strings/builder.go:89,97,114`, `strings/replace.go:36,86`, `strconv/atob.go:32`, `strconv/ftoa.go:89`, `unicode/utf8/utf8.go` | `gp::gc::append`; growth, `append(s, s...)`, `append([]byte, string...)`; the result may share the backing array | no | 04 |
| `copy` | `strings/builder.go:68`, `strings/reader.go:44,58`, `strconv/decimal.go:44-55`, `internal/stringslite/strings.go` (`copy(b, s)`) | `copy(dst, src []T)`; `copy([]byte, string)`; overlap-safe (memmove); returns min(len) | no | 04 |
| Slicing | `strings/builder.go:67`, `strings/strings.go` many, `strconv/atoi.go:133`, `unicode/utf8` | `s[lo:hi]`, `s[lo:]`, `s[:hi]`, `s[lo:hi:max]`; Go's bounds checks (no past-the-end); the three-index cap | no | 03, 04 |
| Indexing | `strings/strings.go:133,641`, `container/list` none, `strconv/atoi.go:135` | `s[i]` is a `byte`; slice/array indexing with a bounds panic | no | 03, 04 |
| `len` and `cap` | `strings/builder.go:51,56`; `len` on strings, slices, arrays, maps, chans | `gp::gc::len`/`cap` per kind; `len` of an array is a constant; `cap` of a string does not exist | partial: `len(string)` only | 03, 04, 05 |
| `range` over string (runes), slice, array, int | `strings/strings.go:133,455,541,793` (string), `:1162,1170` (`for range n`), `strings/replace.go:62,278,283` (arrays), `:515` (slice), `strconv/decimal.go:62` | Decode UTF-8 for strings (byte index + rune); copy semantics; `range` over an int (Go 1.22) | no | 03, 04 |
| String ↔ `[]byte` / `[]rune` conversions | `strings/replace.go:86` (`string([]byte{o})`), `:319,446`; `strconv/atoi.go:133` (`[]byte(s)`); `[]rune(s)` in 03 | Allocation and copy (Go's conversion semantics; not a cast) | no | 03 |
| String comparison and concatenation | `strings/compare.go:15-19`, `strings/strings.go:145,177` | Bytewise comparison; `+`, `+=` allocate a new string | partial: `+`, comparison on strings | 00, 03 |

### 3.6 Maps

| Construct | Evidence | The emitter must | Today | Program |
|---|---|---|---|---|
| `map[K]V` type, literal, nil | `unicode/tables.go:9` (`map[string]*RangeTable{...}`), `:51` (`map[string]string`), 05 | 8-byte map value; a map literal allocates the table and inserts in source order (Go evaluates the keys/values in order); nil map | no | 05, 21 |
| Index, comma-ok, assignment, delete, clear, len | `unicode/tables.go` lookups in `unicode.Is`; corpus 05 | Go's lookup (`v, ok`), insertion, `delete`, `clear`, `len`; nil map read returns zero, nil map write panics; the key must be comparable (compile-time and runtime panic for `any` keys) | no | 05 |
| `range` over a map | corpus 05 (not in the four; ordinary Go) | Iteration in Go's randomized order; deleting during iteration is allowed, inserting is not deterministic | no | 05 |
| Hashing and equality | map keys `string`, structs, pointers in unicode/corpus | The portable hash of README §2.8 (`memhash`/`strhash`), equality per type | no | 05 |

### 3.7 Func values and closures

| Construct | Evidence | The emitter must | Today | Program |
|---|---|---|---|---|
| Func literals and captured variables | `strings/strings.go:874`, `sort/search.go:123-141`, `strings/iter.go:19,37,84,117`, `io/pipe.go:72`, `sync/pool.go` | Heap (GC) closures; capture by reference (Go's variable semantics), escape analysis may keep them on the stack (README §2.8) | no | 09 |
| Func values in fields/parameters/results | `sort/sort.go:85-88` (`lessSwap`), `sort/search.go:58`, `strings/strings.go:441,532` | `gp::gc::func`, nil func, call through the value, function equality is only against nil | no | 09, 20 |
| Higher-order generic functions | `slices` (79 generic funcs), `sort/slice.go:24-29` | Generic func values, instantiation, closures over type parameters | no | 13, 20 |
| Variadic functions | `strings/replace.go:32` (`NewReplacer(oldnew ...string)`), `io/multi.go:73,127` (`MultiReader`, `MultiWriter`), `errors/join.go:19` (`Join(errs ...error)`), `slices/slices.go:135,260` (`Insert`, `Replace`), `cmp/cmp.go:69` (`Or(vals ...T)`) | A `...T` parameter is a `[]T` inside the body; a call with trailing values builds the slice (Go's allocation), `s...` passes the slice through; a func value of a variadic function; a variadic generic function | no: `gc.go:284-286` refuses variadic signatures | 12 |
| Per-iteration loop variables | `strings/iter.go` (no), corpus 09 | Go 1.22 semantics: each iteration has its own variable when captured (file version from `types.Info.FileVersions`) | no | 09 |

### 3.8 Goroutines, channels and select

| Construct | Evidence | The emitter must | Today | Program |
|---|---|---|---|---|
| `go` statement | `16_binarytrees`; `sync/waitgroup.go:237`; `io/pipe.go` (no `go`, but channels/defer) | One OS thread per goroutine (README §1.4, ADR-007); the call's arguments are evaluated in the caller; the goroutine's stack is scanned by the collector | no | 15, 16 |
| Channel make/send/receive/close/len/cap | `io/pipe.go:52,58,60,62,77-91` (unbuffered pipe); `16_binarytrees` (buffered) | `gp::gc::chan` (16.1e); buffered and unbuffered; `close`; receive from closed returns the zero value and `false`; send on closed panics; nil channel blocks | no | 15 |
| `select` | `io/pipe.go:51,57,77,86`; `16_goroutines` | Uniform pseudo-random choice among ready cases; `default`; nil cases disabled; send and receive cases | no | 15 |
| `range` over a channel | `16_goroutines` worker | Receive until closed | no | 15 |
| `//go:linkname` used by the goroutine machinery | `iter/iter.go:231-235` (`newcoro`, `coroswitch`), `sync/pool.go:82` (`runtime.randn`), `internal/sync/runtime.go:20-51` | Resolve the pull to the stub/native with its C++ name (README §2.7); iter's coroutine switch is a stored continuation and must be a native (or range-over-func is refused clearly) | no | 15, 18 |

### 3.9 defer, panic, recover

| Construct | Evidence | The emitter must | Today | Program |
|---|---|---|---|---|
| `defer` (LIFO, argument evaluation now) | `io/pipe.go:23,31,82`, `sync/once.go:75-77`, `sync/oncefunc.go:23,59,94`, `sync/waitgroup.go:84,238` — **not** in the four packages themselves | A deferred frame per goroutine; run in LIFO order at return and during panic; arguments evaluated at the `defer`; a deferred call may change named results | no | 10 |
| `panic` and `recover` | `strings/builder.go:41,78`, `strings/reader.go:145`, `strings/replace.go:34`, `strings/strings.go:498,504,628,632`, `strconv/ctoa.go:16`, `strconv/ftoa.go:70`, `sync/oncefunc.go:25,61,96`; `panic("...")` with a string in every one | Panic as a C++ exception carrying the Go value; `recover` only inside a deferred call; a recovered panic resumes at the function's return; `error`/string panic values | no | 10 |
| Panic messages | `strings.NewReplacer: odd argument count` (replace.go:34), `strconv: illegal AppendFloat/FormatFloat bitSize` (ftoa.go:70) | The exact message and exit status 2 for an unrecovered panic; `#line` positions stay Go's | no (16.1c prints a spike panic line for division by zero) | 10 |

### 3.10 Multiple results and assignments

| Construct | Evidence | The emitter must | Today | Program |
|---|---|---|---|---|
| Multiple results and named results | `strings/builder.go:87,103,112`, `strings/reader.go:39,50,66,87,116,137`, `strconv/atob.go:10`, `strconv/atoc.go:44`, `strconv/atof.go:173,313` (7 results), `sort/search.go:99`; `sync/waitgroup` | Return a struct in Go's ABI or C++ multiple values; named results are variables; assignment from a call (`n, err := f()`) | no | 10, 19 |
| Multiple assignment and `:=` with several values | `container/list/list.go:223,232` (`i, e = i-1, e.Next()` in a `for` post), `strings/reader.go:97` (`ch, size = utf8.DecodeRuneInString(...)`), `sort/sort.go:127` (`x[i], x[j] = x[j], x[i]`), `strings/reader.go:156` (`*r = Reader{s, 0, -1}`), 04, 07 | Evaluate all right-hand sides before assigning; simultaneous swap; blank identifier; `x, ok := m[k]`, `v, ok := <-ch`, `v, ok := i.(T)` | no: one value per side | 04, 05, 07 |
| Blank identifier | `strings/strings.go:502,512,572` (`for _, elem := range ...`), `strconv/atoi.go:133,259` (`for _, c := range []byte(s)`); the corpus uses `_` in 04, 05 and 08 | Discard without evaluating twice; a blank name gets a fresh temporary (in 05's `_, ok = m["nope"]` the map is still read) | no | 04, 05, 08 |

### 3.11 Control flow

| Construct | Evidence | The emitter must | Today | Program |
|---|---|---|---|---|
| `if`/`else`, init statement | `strings/reader.go:119`, everywhere | Go's scoping | yes | 00 |
| `for` (three-clause, condition-only, infinite) | `strings/strings.go:42-60`, `container/list/list.go:231` | Go's scoping; break/continue; the condition and post statements are Go expressions | partial: three-clause and condition-only, unlabeled break/continue | 00, 04 |
| `switch` (expression, tagless, init) | `strings/reader.go:119`, `strings/strings.go:84,129,488,617,641,643,843`, `strconv/atob.go:11`, `strconv/atoi.go:83,120,207`, `strconv/decimal.go:33,316`, `strconv/ftoa.go:62,77,202`, `strconv/quote.go:82,98` | Evaluate the tag once; compare cases in order; no implicit fallthrough; tagless means `true`; init scope is the switch | no | 11 |
| `fallthrough` | `strconv/atoc.go:89`, `strconv/atof.go:52`, `strconv/quote.go:105` | Run the next case body without testing it (last case illegal) | no | 11 |
| Labels, labeled `break`/`continue`, `goto` | labels `strings/strings.go:164,174,183,1222`, `strconv/atof.go:204,395,401`; labeled break `atof.go:213`; labeled continue `strings.go:188`; goto `strings.go:160,169,1201`, `atof.go:321-393` | C++ labels are close, but Go's `goto` cannot jump over declarations and a labeled `continue` targets the loop, not a C++ block; emit C++ labels and loop labels deliberately | no | 11 |
| `if err != nil` chains and early returns | everywhere | Nothing special beyond scoping | yes | 00 |

### 3.12 Constants, iota and conversions

| Construct | Evidence | The emitter must | Today | Program |
|---|---|---|---|---|
| Untyped constants and default types | `strings/strings.go:18`, `strconv/atoi.go:63` | Go's untyped constant arithmetic and conversion to the context type; overflow is a compile-time error | partial: int/string/bool constants, `INT64_MIN` | 02 |
| `iota` | `sort/sort.go:62` | As §3.1 | no | 01 |
| Integer conversions and wrap | `strconv/atoi.go:126,147,228-235`, `strings/strings.go:102,131`, `strconv/decimal.go:89,141`, `sort/sort.go:74` | Truncate to the target width, signed/unsigned two's complement; `uintptr` size | no | 02 |
| Float/complex conversions | `strconv/atof.go:432,558,705`, `strconv/ctoa.go:14-26`, `strconv/ftoa.go:49-64` | IEEE rounding to float32/float64; complex from two floats; `real`/`imag` | no | 02, 19 |
| `byte`, `rune`, `string(rune)` conversions | `strconv/quote.go:41,69,233,273`; `strings/strings.go:208,241,261`; `strings/replace.go:63,285` | `byte`/`rune` are aliases (uint8/int32); `string(r)` is UTF-8 encoding (not a numeric cast — `string(65)` is `"A"`) | no: only conversions to `int` | 02, 03 |
| Conversions between named types and their underlying types | `sort/sort.go:123` (`IntSlice` ↔ `[]int`), `unicode/letter.go:64` | Same layout, new type identity | no | 20, 21 |
| Builtins `min`, `max`, `len`, `cap`, `append`, `copy`, `make`, `new`, `delete`, `clear`, `close`, `panic`, `recover`, `print`, `println` | `strings/search.go:114` (`max`), `strconv/atof.go:21` (`min`), `strings/strings.go:680,1197`; others per sections above | Each with Go's typing and semantics (`min`/`max` are generic builtins in 1.21) | partial: `print`, `println`, `len(string)` | many |
| `unsafe`-adjacent conversions | `strings/builder.go:39,47` | See §3.14 | no | 14 |

### 3.13 Generics

| Construct | Evidence | The emitter must | Today | Program |
|---|---|---|---|---|
| Generic function declarations | `strconv/quote.go:499` (`bsearch[S ~[]E, E ~uint16 | ~uint32]`), `cmp/cmp.go:28,40`, `slices` (79 funcs), `sync/oncefunc.go:46,80` | Type parameters, constraints, instantiation per type, monomorphized C++ or a dictionary; Go's type inference | no | 13 |
| Generic type declarations and methods | `sync/atomic/type.go:47-58` (`Pointer[T any]`, `Load`), `internal/sync/hashtriemap.go:21` (`HashTrieMap[K comparable, V any]`) | Generic structs/slices with methods; method sets of instantiated types | no | 13 |
| Constraint interfaces with type terms and `~` | `strconv/quote.go:499`, `cmp/cmp.go:18-26` (`Ordered`), `slices/sort.go:16` (`S ~[]E, E cmp.Ordered`) | Type-set constraints and `~T`; the interface is not a runtime interface value | no | 13 |
| Inferred and explicit instantiation | `sort/sort.go:170,176,181,186,192,197` (`slices.Sort/IsSorted(x)`), `strconv/quote.go:544,548,553,561,583` (`bsearch`), `sync/waitgroup.go:90,139,174,178,197` (`synctest.Associate`/`Disassociate`) | Infer type arguments from operands; explicit `f[int](...)`; one C++ instantiation per type set actually used | no | 13, 20 |
| Generic type instantiations in signatures | `strings/iter.go:18,36,67,75,83,116` (`iter.Seq[string]`), `iter/iter.go:221-227` (`Seq[V]`, `Seq2[K,V]`) | A generic type used as a return/parameter type | no | 18 |
| Range over a func (range-over-func) | `18_strings_pkg` (`for line := range strings.Lines(...)`); `iter/iter.go:221-227`, `:259` (`Pull` uses `newcoro`/`coroswitch`) | The Go 1.23 range-over-func protocol: call the iterator with a yield closure; `break`/`return` semantics | no | 18 |

### 3.14 `unsafe`

| Construct | Evidence | The emitter must | Today | Program |
|---|---|---|---|---|
| `unsafe.Pointer` conversions | `strings/builder.go:39` (`(*Builder)(abi.NoEscape(unsafe.Pointer(b)))`) | Go's rules: `*T` ↔ `unsafe.Pointer` ↔ `uintptr`; `NoEscape` is a no-op returning its argument | no: `unsafe` import refused | 14 |
| `unsafe.String`, `unsafe.SliceData`, `unsafe.StringData`, `unsafe.Slice` | `strings/builder.go:47` (`unsafe.String(unsafe.SliceData(b.buf), len(b.buf))`) | Build a 16-byte string over existing bytes with no copy (Go 1.20 semantics; `nil`/empty rules) | no | 14 |
| `unsafe.Sizeof`, `Alignof`, `Offsetof` | `sync/pool.go:301` (`unsafe.Sizeof(poolLocal{})` in pointer arithmetic), `sync/runtime.go:54-55` (`unsafe.Sizeof(notifyList{})`) | Compile-time constants from Go's layout, not C++ `sizeof` | no | 14 |
| Pointer arithmetic through `uintptr` | `sync/pool.go:301` (`unsafe.Pointer(uintptr(l) + uintptr(i)*unsafe.Sizeof(...))`), `sync/pool.go:314-317` (atomic loads/stores) | `uintptr` arithmetic and back to a pointer; the collector's stack scan keeps the base alive (README §2.7's argument) | no | 14 |

### 3.15 Semantics the emitter must keep

| Semantics | Evidence / where it bites | What it means |
|---|---|---|
| Evaluation order | README §2.2; 16.1c already fixes call arguments and binary operands left to right (`gc.go:636-867`) | Go evaluates function calls and channel ops left to right; C++ does not. Keep the temporaries. Assignment evaluates all RHS first, then assigns left to right. |
| Integer arithmetic | `strconv/decimal.go:44-55`, `atoi.go:126`; 16.1c's `gp::gc::div/rem/shl/shr` | Go's `/`, `%`, `<<`, `>>` differ from C++ for negatives, zero divisors and shift counts (already handled for int). Unsigned division and float division must not use these helpers. |
| Float semantics | `sort/sort.go:148-150`, `strconv/ftoa.go` | IEEE 754; `NaN` comparisons; no `-ffast-math`; `+0 == -0`. |
| Nil behavior | `container/list` nil links, nil maps, nil slices, nil funcs, nil interfaces | Reads of nil maps/slices are fine; writes/calls/dereferences panic with Go's message; typed nil in an interface is not nil (`16.1e` iface.hpp). |
| Copies | `sort/sort.go:127`, `container/list/list.go:41-46`, `strings/reader.go:156` (`*r = Reader{...}`) | Struct and array assignment copy; slicing does not; interface boxing copies non-pointer values. |
| Package init order | §3.1; `unicode/tables.go` maps, `strconv/atoi.go:21` | Imports before importer, vars in declaration order, then inits; one pass; cycles are a compile error. |
| Panic exit | §3.9 | Unrecovered panic: the Go message on stderr and exit status 2; the stack trace is step 16.6. |
| `println` formats | `runtime/include/gocpp/gc/print.hpp`; corpus 02, 13, 19 | Go's built-in `println` prints floats as `+2.500000e+000`, complex as `(1.5-2.5i)` and pointers as two words; the GC engine's `append_print` handles only int/bool/string today. The 16.1f corpus includes floats and complex on purpose. |
| Zero-size shape | `sync/mutex.go:31`, `sync/atomic/type.go:47-53` | README §2.2's `[[no_unique_address]]` rules; two zero-size fields share an offset; trailing padding. |

## 4. Today's support (measured, not guessed)

### 4.1 The 16.1c subset

`internal/cppgen/gc.go` emits: one `package main` folder with no imports; func declarations
(not methods, `init`, generic, variadic or bodyless); statements block, return one value,
single assignment/`:=`/`var`, expression statements, `if`/`else`, `for` (three-clause and
condition-only), `++`/`--`, unlabeled `break`/`continue`; expressions names, int/string
literals, parens, binary and unary operators, calls of `print`, `println`, `len(string)`, own
functions and `int(x)`; types `int`/`int64`, `string`, `bool`. It keeps Go's evaluation order,
Go's `/`, `%`, shifts, `INT64_MIN`, `#line` directives and the `gopkg_` namespace. Everything
else is a positioned error, never silently different C++.

### 4.2 The corpus today

`gocpp run -gc-engine .` in each `programs/<name>/`, gocpp built from this worktree (branch
`job/16.1f2-plan`, base `p16`, which has 16.1c but not 16.1f-load). The exact strings are also
in `programs/manifest.json`.

| Program | Today | Positioned error |
|---|---|---|
| `00_baseline` | **ok** | — (output equals Go's) |
| `01_decls` | error | `main.go:7:1: the GC engine does not support const declarations yet (job 16.1c)` |
| `02_values` | error | `main.go:5:1: the GC engine does not support type declarations yet (job 16.1c)` |
| `03_strings` | error | `main.go:8:10: the GC engine does not support this expression yet (job 16.1c)` |
| `04_slices` | error | `main.go:6:1: the GC engine supports only int, string and bool yet, not []int (job 16.1c; step 16.5 adds Go's other values)` |
| `05_maps` | error | `main.go:6:2: the GC engine supports only int, string and bool yet, not map[string]int (job 16.1c; step 16.5 adds Go's other values)` |
| `06_structs` | error | `main.go:6:1: the GC engine does not support type declarations yet (job 16.1c)` |
| `07_methods` | error | `main.go:5:1: the GC engine does not support type declarations yet (job 16.1c)` |
| `08_interfaces` | error | `main.go:6:1: the GC engine does not support type declarations yet (job 16.1c)` |
| `09_closures` | error | `main.go:6:1: the GC engine supports only int, string and bool yet, not func(int) int (job 16.1c; step 16.5 adds Go's other values)` |
| `10_defer` | error | `main.go:6:1: the GC engine does not support multiple results yet (job 16.1c; step 16.8 adds them)` |
| `11_control` | error | `main.go:6:2: the GC engine does not support a switch statement yet (job 16.1c)` |
| `12_variadic` | error | `main.go:5:1: the GC engine does not support variadic functions yet (job 16.1c)` |
| `13_generics` | error | `main.go:6:1: the GC engine does not support type declarations yet (job 16.1c)` |
| `14_unsafe` | error | `main.go:6:8: could not import unsafe (package unsafe: imports are not supported by the GC engine yet (job 16.1c: Go source without imports; Go's standard library comes with steps 16.9-16.13))` |
| `15_goroutines` | error | `main.go:7:1: the GC engine supports only int, string and bool yet, not <-chan int (job 16.1c; step 16.5 adds Go's other values)` |
| `16_binarytrees` | error | `main.go:6:1: the GC engine does not support type declarations yet (job 16.1c)` |
| `17_list` | error | `main.go:5:8: could not import container/list (package container/list: imports are not supported by the GC engine yet (job 16.1c: Go source without imports; Go's standard library comes with steps 16.9-16.13))` |
| `18_strings_pkg` | error | `main.go:6:8: could not import strings (package strings: imports are not supported by the GC engine yet (job 16.1c: Go source without imports; Go's standard library comes with steps 16.9-16.13))` |
| `19_strconv_pkg` | error | `main.go:6:8: could not import strconv (package strconv: imports are not supported by the GC engine yet (job 16.1c: Go source without imports; Go's standard library comes with steps 16.9-16.13))` |
| `20_sort_pkg` | error | `main.go:7:8: could not import sort (package sort: imports are not supported by the GC engine yet (job 16.1c: Go source without imports; Go's standard library comes with steps 16.9-16.13))` |
| `21_unicode` | error | `main.go:7:8: could not import unicode (package unicode: imports are not supported by the GC engine yet (job 16.1c: Go source without imports; Go's standard library comes with steps 16.9-16.13))` |

So today: 1 of 22 passes. That is the starting line, not a failure of the corpus: the corpus
is the milestone list.

## 5. Closure order: what each package needs

| Layer | Packages | Constructs they add beyond the layer below |
|---|---|---|
| 0 | `container/list` | nothing beyond its own source: structs, pointers, `new`, methods, `any`, type assertions |
| 1 | `internal/goarch`, `math/bits`, `unicode/utf8`, `internal/abi`, `internal/stringslite` | constants, byte/rune arithmetic, UTF-8 decoding, `abi.NoEscape` |
| 2 | `errors`, `internal/reflectlite` (stub) | `error`, `errors.New`, type switches, `Unwrap`/`Is`/`As`; reflectlite's `ValueOf`/`Swapper` (sort.Slice) |
| 3 | `unicode` | six package-level map literals, 10k lines of slice tables, `SpecialCase` methods |
| 4 | `math` (overlay, §2.7), `internal/cpu` (overlay if emitted at all) | float arithmetic, `math.IsNaN`/`Inf`, `float64bits` through unsafe-free unions; the assembly `arch*` functions are replaced |
| 5 | `iter` (+ runtime stub) | generic `Seq`/`Seq2`, `Pull`'s coroutine linknames |
| 6 | `slices`, `cmp` | 79 generic functions, `cmp.Ordered`, `slices.Sort/IsSorted` |
| 7 | `sync/atomic` (**overlay**, §2.7), `internal/race`, `internal/synctest`, `internal/sync` | generic atomic types (overlay Go source), `unsafe.Pointer` atomics as C++ natives, zero-size fields, `HashTrieMap` (generic, unsafe) |
| 8 | `sync` | `Once`, `Mutex`, `WaitGroup`, `Pool`, `Map`, `func init`, `defer`/`recover`, `go`, `//go:linkname` |
| 9 | `io` | 22 interfaces with embedding, `Pipe` (`select`, channels, `defer`), method values |
| 10 | `strconv` | floats/complex, generic `bsearch`, named results, many switches/gotos |
| 11 | `sort` | interface dispatch, `reverse` embedding, `sort.Slice` through `any`+reflectlite, generic `slices.Sort` |
| 12 | `strings` | `Builder` (`unsafe.String`), `Replacer` (`sync.Once` + method values), `iter.Seq` signatures, `unicode` case mapping, `utf8` |

`container/list` is the only root with no imports, so it is the first integration target.
The value/slice layer (`unicode`, strings, slices, maps) comes next because both `strconv` and
`sort` need it. `strconv` (layer 10) is then the cheapest of the heavy three (no
`io`/`sync`) and brings the generic-function basic case. `sort` (layer 11) needs generics and
`slices` but no `io`. `strings` (layer 12) is last: it needs everything above plus
`unsafe.String`, `sync` and `io`.

## 6. Recommended split of 16.1f

Two pieces already run in parallel and are assumed here: **16.1f-stubs** (the
`runtime`/`internal/bytealg`/`internal/reflectlite` stubs and `gostd/spike/closure.json`) and
**16.1f-load** (GOROOT multi-package loading and `go/types` over the graph). The jobs below
are the emitter work. Each one ends with its programs compiling under `gocpp run -gc-engine`
and printing the manifest's expected output; the corpus makes "done" objective.

| Job | Packages / closure | Constructs | Corpus programs | Depends on |
|---|---|---|---|---|
| **f1: packages and container/list** | the graph from f-load; `container/list`; `gopkg_` namespaces for every package | imports and cross-package selection; package-level funcs (already), types, consts (incl. `iota`), vars, `init` and init order; structs (fields, embedding, zero-size fields including `[0]T`, the type only); pointers (`&`, `*`, `new`); methods (value/pointer receivers, method values/expressions, promotion); `any`/interfaces (declaration, satisfaction, dispatch, type assertion/switch); composite literals; `switch`/tagless/`fallthrough`/labels/`goto` | `01_decls`, `06_structs`, `07_methods`, `11_control`, `17_list` | f-load, f-stubs |
| **f2: numeric values, slices/arrays/strings/maps** | `unicode` (maps!); the `gp::gc::map` implementation from 16.1e's placeholder; the string/slice/array values of 16.1e | typed/untyped constants and constant expressions, all integer widths/unsigned/`uintptr`, float32/64 and complex (including Go's builtin `println` float/complex format, `gp::gc::print.hpp`, first needed by 02 and reused by f3/f5), conversions (integer, float, `byte`/`rune`/`string(rune)`), `min`/`max`; slice/array/string types and values; `make`, `append`, `copy`, `len`, `cap`, `delete`, `clear`, `close`; indexing, slicing (two and three index), `range` (string/slice/array/int), string↔[]byte/[]rune, string comparison/concat; map creation from literals, lookup/comma-ok, assignment, `delete`, `range`, nil map; large static tables; package init of unicode's six maps; interface values in slices (08) | `02_values`, `03_strings`, `04_slices`, `05_maps`, `08_interfaces`, `21_unicode` | f1 |
| **f3: generic functions, errors and strconv** | `errors`, `internal/stringslite`, `math/bits`, `unicode/utf8`, `math` (overlay), `internal/cpu` overlay, `internal/bytealg` stub | generic func declarations with type-set constraints and instantiation (`strconv.bsearch`), `error`/`NumError`, named results, multi-value calls and multi-assignment from calls, type assertions on errors, float/complex conversion and formatting (reusing f2's `println` float/complex format), the switches and gotos of `strconv`, `min`/`max` on floats | `19_strconv_pkg` | f2 |
| **f4: funcs, closures, defer/panic, goroutines/channels** | `gp::gc::func`/`chan`, `panic.hpp` | func literals/closures/captured vars, func values/fields, per-iteration loop vars, variadic funcs and `...`, `defer` (LIFO, named results), `panic`/`recover`, `go`, channel make/send/receive/close/`range`, `select` with `default` | `09_closures`, `10_defer`, `12_variadic`, `15_goroutines`, `16_binarytrees` | f1, f2 |
| **f5: generics in depth and sort** | `slices`, `cmp`, `iter` (with the runtime stub), `internal/reflectlite` stub | generic type declarations and methods, instantiation (inferred/explicit), constraint interfaces with type terms; interface embedding and method sets; `sort.Interface`, `reverse`, `sort.Slice` through `any` + reflectlite (the stub must actually work), `slices.Sort/IsSorted`; `println` of a generic float64 (reuses f2's format). Range-over-func is **not** needed here: 13/20 do not range over a func | `13_generics`, `20_sort_pkg` | f1, f2, f4 (closures for `sort.Slice`) |
| **f6: unsafe, sync, io, strings** | `sync/atomic` (overlay), `internal/sync`, `internal/race`, `internal/synctest`, `sync`, `io`, `unsafe`, `strings` | `unsafe.Pointer`/`String`/`SliceData`/`Sizeof`/`Alignof`/`Offsetof`/pointer arithmetic; zero-size `noCopy`; `sync.Once`/`Mutex`/`WaitGroup`/`Pool`/`Map`; `func init` and `//go:linkname`; `io`'s 22 interfaces, embedding, `Pipe` channels/select/defer; `strings.Builder`, `Replacer`, `iter.Seq` signatures and case mapping; **range-over-func and the iterator lowering** (`for line := range strings.Lines(...)`, `iter.Pull`'s coroutine natives) — this is the construct 18 needs and f5 does not | `14_unsafe`, `18_strings_pkg` | f1-f5 |
| **f7: integration** | all of the above, plus the gcbench binarytrees question | run the four packages and binarytrees in one program; the differential test against `go run`; record the first failure position for each remaining program | `17_list`-`20_sort_pkg`, `16_binarytrees` together | f1-f6 |

Why this order: `container/list` proves the package machinery with zero stubs. f2 then covers
the value layer both `strconv` and `sort` stand on (consts, numeric/float values, slices,
arrays, strings, maps, `unicode`); without it `strconv` cannot even be emitted
(`strconv/decimal.go:18` has `d [800]byte`, its public API is `[]byte`-based and it indexes and
ranges over strings throughout). f3 is `strconv` and its generic `bsearch`, which proves generic
function declarations and the float/error stack without `io`. f4 adds closures, `defer`/panic
and goroutines. f5 is the generic `slices`/`iter`/`sort` layer. `strings` is last because it
pulls `io`→`sync`, `unsafe.String` and range-over-func. This follows §5's layer order
(`strconv` is layer 10 and must come after the value/slice strings layer). The jobs are sized
like the existing ones (a few hundred lines of emitter plus tests each). If a job grows too big,
f2 splits into f2a (values + maps/unicode) and f2b (slices/arrays/strings), f4 into f4a
(`defer`/`panic`/func values) and f4b (goroutines/channels/select), and f6 into f6a
(`unsafe`+`sync`) and f6b (`io`+`strings`+range-over-func).

## 7. Surprises, conditional constructs, and plan issues

1. **Generics are on the critical path, and step 16.1 does not name them.** `sort.Ints`,
   `sort.Strings`, `sort.Float64s`, `IntsAreSorted`, … delegate to generic `slices.Sort` /
   `slices.IsSorted` (`sort/sort.go:170,176,181,186,192,197`); `strconv` declares and calls
   its own generic `bsearch` (`strconv/quote.go:499,544-583`); `sync.OnceValue`,
   `sync.Pool`'s `atomic.Pointer[poolChainElt]` (`sync/poolqueue.go:201,217`), `sync.Map`'s
   `atomic.Pointer[readOnly]` (`sync/map.go:54`), `sync/atomic.Pointer[T]` and
   `internal/sync.HashTrieMap` are generic (`sync.WaitGroup` itself uses `atomic.Uint64` at
   `sync/waitgroup.go:55`, not `atomic.Pointer`).
   Neither the 16.1c emitter nor the 16.1 text mentions generics.
2. **`iter` and range-over-func are not named.** `strings/iter.go:18-127` returns
   `iter.Seq[string]`; `slices` and `cmp` use `iter`; `iter.Pull` needs `runtime.newcoro` and
   `runtime.coroswitch` linknames (`iter/iter.go:231-235`). Compiling `strings` needs the
   `iter.Seq` type and `iter`'s linkname declarations even if no program ranges over a func;
   the program `18_strings_pkg` does range over `strings.Lines`, which exercises the protocol.
3. **`strings` pulls `io`, and `io` pulls `sync`.** The plan's list of stubs (runtime,
   internal/bytealg, internal/reflectlite) is not the whole closure. `strings/reader.go:9` and
   `strings/replace.go:8` import `io`; `io` imports `errors` and `sync`; `sync` imports
   runtime, `internal/sync`, `internal/race`, `internal/synctest` and `sync/atomic`. So
   goroutines/channels/select/`defer`/recover arrive with `io.Pipe` (`io/pipe.go:23,31,51-91`)
   and `sync` (`sync/waitgroup.go:237-238`, `sync/once.go:75-77`), not only with binarytrees.
   Either translate `io` and `sync` (f6) or add two more stubs (which changes the plan's stub
   list). The plan says only "hand-written stubs stand in for runtime, internal/bytealg and
   internal/reflectlite".
4. **Maps are needed only through `unicode` — and 16.1e's map is a placeholder.**
   `unicode/tables.go:9,51,5192,7389,9440,9740` builds six maps at package init. The four
   packages themselves use none (the plan's "maps (unicode's tables)" is right; "for the four"
   would be wrong). `gp::gc::map` in 16.1e has only `map_core` and `len`; 16.1f must implement
   creation/insertion/lookup/delete/clear/range, or document that unicode's variables are
   stubbed or skipped. `05_maps` and `21_unicode` keep this honest.
5. **The `internal/bytealg` stub must be behavior-correct, not just compile.** `strings` calls
   it for search (`strings/strings.go:11`, `compare.go:7`, `builder.go:9`) and `strconv` for
   `index` (`strconv/bytealg.go:9`). Go 1.25's own bytealg is generic (`HashStr[T string|[]byte]`,
   `HashStrRev`, `IndexRabinKarp` in `internal/bytealg/bytealg.go:35,52,69`) and has
   architecture assembly; a stub with `string`-only non-generic functions is the cheap route,
   but it must return correct results or strings' tests will fail subtly. The 16.1f-stubs job
   owns the identifier list.
6. **The `internal/reflectlite` stub must work, at least for `sort.Slice`.** `sort/slice.go:25-26`
   calls `reflectlite.ValueOf(x)` and `reflectlite.Swapper(x)`; a compile-only stub makes
   `20_sort_pkg` fail at run time. The stub needs a value carrying kind/size/element size and a
   swap closure for slices (and `Len`). `sort.IsSorted`/`Sort` do not use it.
7. **`internal/cpu` is an overlay, and it is in `math`'s import list.** `math` imports
   `internal/cpu`; `cpu_x86.go:12-18` declares bodyless `cpuid`, `xgetbv`, `getGOAMD64level`.
   README §2.7 already lists `internal/cpu` as an overlay, but step 16.1's stub list does not;
   f2 needs it (or a `math` variant that drops the import).
8. **The measured binarytrees is not the spike's binarytrees.** `bench/gc/binarytrees/main.gp`
   imports `os` and `time` (lines 22-25, `time.Now` at 136, `time.Since` at 152) and prints
   `elapsed_ms` unless `-quiet`; that pulls
   `syscall`, `internal/poll`, the timer machinery and more. The spike's "binarytrees with
   builtin println" is `programs/16_binarytrees` (no imports: structs, pointers, methods,
   recursion, `go`, channels). If 16.1f is expected to build the *measured* file, its closure is
   much larger than these four packages — the plan should say which one 16.1f means.
9. **Struct tags are listed in the 16.1 constructs but none of the four or their closure (as
   reached without reflect) uses a tag.** They matter for 16.11's `reflect` and for mirrors
   (§2.6), not for this spike. `06_structs` contains one as a marker, not a requirement.
10. **`defer`/`recover` and `select` are not used by the four packages themselves**; they are
    only in `io`/`sync` in the closure. If `io` and `sync` are stubbed, f4 can be reduced to
    func values/closures/goroutines/channels; the corpus still covers the rest.
11. **`println`'s float/complex formats are part of the differential oracle.** Go's built-in
    `println` prints `+2.500000e+000` and `(1.5-2.5i)`; 16.1e's `print.hpp` handles only
    int/bool/string. Programs 02, 13, 19 make the emitter reproduce Go's `printfloat`/
    `printcomplex`, or the expected outputs must be weakened (they are not weakened here).
12. **The emitter's current error names are transient.** All "job 16.1c"/"step 16.5/16.8"
    messages in §4.2 are the 16.1c subset's; the 16.1f jobs replace them. Keep the messages
    positioned (`.go:line:col`), as the corpus's test checks them by substring.
13. **Windows/arm64 not measured.** The manifest is linux/amd64 (Go 1.25.14). `14_unsafe`
    prints `int`-size-dependent numbers; on arm64 they are the same for these types, but the
    program is marked in the manifest's notes. README §2.7's Windows target does not change the
    constructs.

## 8. Reproduction

```
export GOROOT=$(scripts/in-devenv -- go env GOROOT)     # go1.25.14
export PATH=$GOROOT/bin:$PATH

# the oracle: 22 programs, each twice, exact output
go test -count=1 ./gostd/spike/analysis/

# the today table: build this worktree's gocpp and re-check the recorded results
go build -o .scratch/bin/gocpp ./cmd/gocpp
GOCPP=$PWD/.scratch/bin/gocpp go test -count=1 -run TestGocppStatus ./gostd/spike/analysis/

# regenerate the manifest after a construct changes (construct table: genmanifest/main.go)
GOCPP=$PWD/.scratch/bin/gocpp go run ./gostd/spike/analysis/genmanifest gostd/spike/analysis/programs
```

`programs_test.go` runs every program with `go run .` and compares the combined output to the
manifest twice; `TestGocppStatus` is skipped unless `GOCPP` is set. When a 16.1f job makes a
program pass, `TestGocppStatus` fails until its manifest entry (and §4.2 here) are updated —
that is the intended guard against a stale support table.
