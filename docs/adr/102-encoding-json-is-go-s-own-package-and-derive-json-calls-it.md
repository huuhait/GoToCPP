# ADR-102: `encoding/json` is Go's own package, `#Derive(JSON)` calls it, and Go's tests of the ported packages run in a copy of the port

Status: accepted (2026-10-20), step 9.11. Builds [ADR-034](034-portstd-generic-arch-and-rewritten-constraints.md) (`tools/portstd`), [ADR-035](035-reflect-subset-on-descriptors.md)
(the reflect subset) and [ADR-080](080-derive-writes-methods-from-the-types.md) (`#Derive`); the tests follow
[ADR-083](083-sort-regexp-ports-and-the-test-matcher.md) (`tests/pkgs/regexp_internal`).

## Context

`encoding/json` was a declaration stub. Step 9.3 had built a reflect subset "for json" (`New`, `MakeSlice`, `MakeMap`, `Set*`, `MapRange`, struct tags...), and
said that `Value.Method`/`Call`, `MapOf`, `ArrayOf`, `Select` and the channel operations were missing for 9.11. Nobody knew whether Go's 6000 lines
(`encode.go` and `decode.go` are the reflection-heaviest code of the standard library after `fmt`) compile and behave like Go's on that subset, and ADR-080 had left
`#Derive(JSON)` as a design note ("a runtime-helper package plus hidden-import plumbing, with a differential test").

## Decision

1. **`encoding/json` is `tools/portstd` output with one set of patch rules, the stack guard of 5** (`encode.go`, `decode.go`, `scanner.go`, `stream.go`, `indent.go`, `fold.go`, `tables.go`, `tags.go`; the
   `v2_*` files are `goexperiment.jsonv2`, which is off). The reflect subset of ADR-035 was enough as it was: `encoding/json` checks `Marshaler`, `Unmarshaler` and
   `TextMarshaler` with interface assertions, so `Value.Method`, `Value.Call`, `MapOf`, `ArrayOf`, `Select` and `Seq` are not used and **nothing was added to `reflect`**.
   The same step ports `encoding` and `encoding/base64` (needed by json), and, because the manager's tier-2 list names them and each was a one-line `ports.txt` entry,
   `encoding/hex`, `encoding/binary` (with `arch=amd64`, for Go's `native_endian_little.go`), `container/list`, `container/heap`, `container/ring`, `hash`, `hash/fnv`, `hash/crc32`
   (the portable slicing-by-8 form: no assembly), `text/tabwriter`, `log` (with `log/internal`), `path/filepath` (with Go's Windows files, which have not been run), `math/rand` and `math/rand/v2` (with `internal/chacha8rand`).
   `runtime.rand`, which both rand packages call, is a new native (`gp::runtime::runtime_rand`, runtime/src/runtimepkg.cpp): a per-thread splitmix64 seeded from `std::random_device`;
   the generators with a seed (`rand.New(rand.NewSource(42))`, `NewPCG`, `NewChaCha8`) give Go's sequences, the global functions are random but not Go's ChaCha8 stream. `internal/chacha8rand`'s
   block function is Go's `block_generic` (a patch writes the call that `chacha8_stub.s` makes; the new `@asm file.s reason` directive of the patch files says that a patch replaces
   an assembly file that the build constraints keep). Two decoders hold a slice of an array field of themselves (`base64.decoder.out`, `hex.decoder.in`): LeakSanitizer found 2 KB lost
   by every `NewDecoder` that had read, as for `regexp` in ADR-083, and `patches/encoding__base64.rules` and `encoding__hex.rules` copy the bytes instead. `tools/stubgen` has `Real: true` for `encoding` and `encoding/json`; `internal/check/gates.go` has a row
   for each package (and `docs/spec/feature-matrix.md` is regenerated). Not ported: `crypto/*` (Go's FIPS tree), `os/exec`, `hash/maphash`, `hash/adler32`, `hash/crc64`, `text/template`.
2. **One compiler gap was found, by Go's own `TestSliceOfCustomByte`, and fixed**: `[]Uint8("hello")` and `string(u8s)` where `Uint8` is a named byte or rune type (a
   `gp::newtype`) were "not supported yet". `internal/lower/slices.go` (`namedElemKind`) writes `gp::retype_slice<E>(gp::to_bytes(s))` and
   `gp::to_string(gp::retype_slice<uint8_t>(b))`; `retype_slice` (runtime/include/gocpp/slice.hpp) shares the array of the slice it is given. Test: `tests/run/slice_named_elem_convert.gp`.
3. **Tests, in three layers**, all compared with real Go 1.25.14:
   - `tests/run/json_*.gp` (16 programs, 984 lines of output, each a case, with `// Output:` from real Go): marshal of every kind, struct tags (`omitempty`, `omitzero`, `string`, `-`,
     embedded structs and pointers, shadowing and conflicts), numbers (shortest floats, exponent forms, NaN and Inf errors, 64-bit limits), strings (escapes, HTML escaping, U+2028,
     invalid UTF-8, lone surrogates), maps (sorted keys, integer and text keys), custom `Marshaler`/`TextMarshaler`/`Unmarshaler`/`TextUnmarshaler`, `RawMessage`, `Number`, time,
     the error types and their texts and offsets (`SyntaxError`, `UnmarshalTypeError`, `InvalidUnmarshalError`, `UnsupportedTypeError`, `UnsupportedValueError`, `MarshalerError`,
     cycles, the depth limit), `Indent`/`Compact`/`HTMLEscape`/`Valid`, `Decoder` (streams, `Token`, `More`, `Buffered`, `InputOffset`, one-byte readers) and `Encoder`, and 16 goroutines
     marshaling the same types at once (`json_concurrent.gp`, for the type caches).
   - **Go's own tests**, run by `gocpp test` in a copy of the port. Go's `encoding/json` tests are `package json` (they name `scanner`, `checkValid`, `cachedTypeFields`...),
     so the fixture `tests/gotest/testdata/encoding_json_internal` is a module with `stdlib/encoding/json/*.gp` copied byte for byte and Go's test files patched
     (`tools/portstd/gotests/convert.py encoding/json`; `packages/encoding__json.py` lists every patch with its reason). `tests/gotest` runs the fixture with `gocpp test -v` and with
     `go test -v` (the `.gp` renamed to `.go`) and compares the output; `TestPortCopies` fails when a file of a copy differs from the port. **86 tests, 13 examples, 2 fuzz seed
     sets and 476 subtests, the same lines as Go.** Patches: `image`, `math/big` and `net` are not in gocpp (`gotests/shim/` has `image.Point` and a
     64-bit `big.Int` with the text methods; the `net.IP` case and `TestHTTPDecoding` are dropped), `testing/synctest` does not exist (`TestSynctestMarshal` dropped),
     `type C` is `CInt` (gocpp reserves the package-level name `C`), the object nesting of `TestUnmarshalMaxDepth` is 4000 (see 5), the benchmarks are not taken. The same script
     and the same check run the tests of `container/list`, `container/heap`, `container/ring`, `encoding/base64`, `encoding/hex`, `encoding/binary` (without the two `AllocsPerRun` tests: allocations
     are not counted here), `hash/fnv`, `hash/crc32` (with Go's `internal/testhash` and `internal/byteorder` copied into the module and a fixed seed), `text/tabwriter`, `log`
     (without `TestCallDepth`, which starts the binary again with `os/exec`, `TestDiscard`, which counts allocations, and the examples, which print the file name), `path/filepath` (without
     `TestGlob` and `TestNonWindowsGlobEscape`, which glob the package's source files by name, and `TestCVE202230632`, 10001 recursions, which overflows the stack at `-O0`), `internal/chacha8rand`,
     `math/rand` and `math/rand/v2` (without `regress_test.go`: `TestRegress` calls every method of a `Rand` through `reflect.Value.Method` and `Call`, which the reflect subset does not have, D6, and
     `chacha8_test.go`, which hashes with `crypto/sha256`; `rand_basic.gp` prints the same methods and the ChaCha8 and PCG streams, equal to Go's). Where the external test package `xxx_test` imports the
     package, the patch points the import at the copy in the module root, so the tests that use `export_test.go` work. `TestPortCopies` also checks the other ports a fixture copies (`internal/chacha8rand`,
     `internal/filepathlite`, `log/internal`).
     Counting rule (Phase 09 README 2.8): nothing is skipped for a reason other than the documented ones above.
   - The sanitizers: every `json_*.gp` program runs clean under `-asan` (ASan, UBSan, LeakSanitizer) and `-race` with g++ and clang++ (`tools/sanitize run`). `json_marshal_errors.gp` builds
     cyclic values on purpose (`// lsan: cycle`, `tools/sanitize/leaks.txt`: 1704 bytes in 10 allocations for both compilers).
4. **`#Derive(JSON)` writes `MarshalJSON` and `UnmarshalJSON` that call `encoding/json` on a copy of the struct type that has no methods**:

   ```go
   func (c Cfg) MarshalJSON() ([]byte, error)   { type gp_plain Cfg; return json.Marshal(gp_plain(c)) }
   func (c *Cfg) UnmarshalJSON(data []byte) error { return json.Unmarshal(data, c.GoCPPJSONPlainPtr()) }
   func (c *Cfg) GoCPPJSONPlainPtr() any          { type gp_plain Cfg; return (*gp_plain)(c) }
   ```

   Both methods are stubs from the loader (signatures, the pointer receiver of `UnmarshalJSON`, the name of its parameter: `data`, or `gp_data` when the package declares `data`), their bodies come from
   `internal/check/derive.go` (`jsonMethods`) like the others; the local type is a fresh name (`gp_plain`, or `gp_plain1` when the package has a type of that name). The output of a value that these
   methods marshal is exactly `encoding/json`'s (tags, `omitempty`, `omitzero`, `string`, embedded structs, escaping, float formats), by construction; `tests/run/derive_json_plain.gp` prints every derived
   type next to a twin with the same fields and no `#Derive`, and Go runs the twin file (`testdata/derive_twin/derive_json_plain.go`, methods written by hand); `derive_json_names.gp` has the types and
   packages whose names collide with the generated code. It works for generic structs (`Pair[K, V]`).

   **The encoder and the decoder do not call these two methods for a value that they reach** (review of 9.11, 2026-10-21, reworked in round 3). Called as written, `MarshalJSON` and `UnmarshalJSON` make the cost of nested values
   quadratic: each level writes its subtree to a buffer and the outer `Marshal` scans it again (`compact`), and each level of `Unmarshal` scans the whole of the document under it twice (the decoder's `skip` to find
   the end of the value, then `json.Unmarshal`'s `checkValid`). A document nested 800 deep with a 1 MB string at the bottom took 101 ms for a plain struct and **75 to 84 s** for the derived one at `-O0` (6 ms and 6.4 s
   with `-release`; 2000 deep with 10 MB: 117 s): a denial of service on untrusted input. So `#Derive(JSON)` writes one more method, `GoCPPJSONPlainPtr() any` (pointer receiver; returns
   `(*gp_plain)(recv)`), and the patches of `encoding/json` (`derivedPlainPtr`, `derivedPlainType`, `derivedEncoder`, patches/encoding__json.rules) use it:
   - **Decoder.** `indirect`, which finds the `Unmarshaler` of a value, goes on with the pointer to the struct when the type is a derived one, so the struct is decoded directly, as a struct without methods is, and the loop finds the methods that the *struct* has by promotion (an embedded `time.Time`, a type with its own `UnmarshalJSON`) exactly as `json.Unmarshal(data, (*gp_plain)(p))` did.
   - **Encoder.** `newTypeEncoder` picks, once for the type, an encoder for the struct and for a pointer to it (`derivedEncoder`): it encodes the struct through the address of the value when there is one (a pointer, an addressable value: no copy) and a converted copy otherwise, and the encoder of the pointer repeats `ptrEncoder`'s nil check and cycle detection (a pointer to a type that has `MarshalJSON` is not given to `ptrEncoder`). The promoted methods of an embedded field are found when the struct is encoded, as before.
   - **Which types.** Whether a type is a derived one is decided by the *type*, once (`derivedPlainType`, cached): the method is called on a zero value inside a `recover`, and the type that it returns must be a struct whose name starts with `gp_plain` and that converts to and from the type. A type that embeds a derived one and has `MarshalJSON` or `UnmarshalJSON` of its own gets `GoCPPJSONPlainPtr` by promotion: the promoted method panics when an embedded pointer on the way is nil (or returns the other type), so the type is not derived, and its own methods are called, as before, whether the pointer is nil or not. The same rule makes a hand-written `GoCPPJSONPlainPtr` on another type harmless (it returns nil, or a non-struct, or a struct that is not called `gp_plain...`: ignored).
   - **Cost.** The first version of this change generated two more methods (a value method too), and every one of them declares a local type (`type gp_plain T`), which is most of the generated code. One is enough (the encoder converts a value), and `UnmarshalJSON` calls it instead of declaring a type of its own: the three methods declare two local types, as the old two did. `gocpp emit` of 100 types of five fields (a slice, a map, a pointer): 1.70 MB of C++ without `#Derive`, 2.15 MB with the old two methods, 2.21 MB now (the version with two extra methods: 2.38 MB with one extra method that declared a type, and the reviewer measured 3.02 MB, 33.5 s of `g++ -O0` CPU against 21.9 s for the old derive and 12.0 s for plain). (The compile CPU of a loaded machine was too noisy to quote; the size is exact.)
   The methods `MarshalJSON` and `UnmarshalJSON` stay (`x.MarshalJSON()`, `json.Marshaler`) and work as written. The repro: 105 ms plain, 105 ms derived (-O0); marshal 10 ms and 12 ms. `tests/run/derive_json_nested.gp` checks the ratio of the times of the plain and the derived type (not the times: 500 deep, 512 KB at the bottom, Marshal and Unmarshal, within 20 times plus 0.5 s; before this change the ratio was 800), and the rest of what the route must keep (options, errors, fields of every kind, embedding with and without methods of its own, the methods called by hand); `derive_json_embedded.gp` the promoted methods in both directions (`time.Time`, a type with its own methods, by value and by pointer), the types with methods of their own that embed a nil pointer to a derived type (also in the middle of a chain), the reserved name on other types, and the point where the cycle detection starts.

   **It is an API shim, not a faster encoder.** The plan's table said "reflection-free"; it is not: the methods go through `encoding/json`'s reflection once more and are **no faster than a plain
   struct, usually slower** (measured against the same struct without `#Derive` when the encoder called the methods, g++ -release / clang++ -release: `Marshal` 695 / 562 ms against 365 / 235, `Unmarshal` 1406 / 1033 against 1035 / 730, about 1.4 to 2.4 times;
   with the direct route above, g++ -release, best of 14 rounds: 300000 structs with four pointer and slice fields in a slice: `Marshal` 478 ms against 420 (1.14 times), a slice of pointers to them 468 against 444 (1.05), `Unmarshal` of the slice 1223 against 1111 (1.10); a small struct with a nested one, a slice and a map: `Marshal` 1.15 times, `Unmarshal` 1.06 times. The first version of the direct route, with a lookup and a call for every value, was 1.3 times slower on the slice of structs, which is why the encoder is chosen with the type). The manager accepted that
   in exchange for exact output. A generator that writes the encoder of each field itself would have to reproduce `typeFields` (the rules for embedded structs and conflicts), the float and string
   formats, `omitzero`, text and JSON marshalers, maps and interfaces, and then stay equal to them for every Go release; it stays possible later behind the same two method names, and the differential
   tests are what it would have to keep passing. **Addendum of 2026-10-20 to ADR-080, decision 8** (ADRs are append-only, so ADR-080 keeps its text): `JSON` is built by this step; the "runtime-helper package plus
   hidden-import plumbing" of that design note became "`encoding/json` itself plus an import the file must have"; and it is not reflection-free.

   **Differences from a plain struct** (they are in `docs/spec/language.md` and the tour lesson on macros as well):
   - The options of an outer `Decoder` (`DisallowUnknownFields`, `UseNumber`) and of an outer `Encoder` (`SetEscapeHTML(false)`) **reach a derived value reached through `Unmarshal`, `Decode`, `Marshal` or `Encode`**, as they reach a plain struct (the first version of this step ignored them, because the methods were called with the raw bytes). They do not reach one that the program decodes by calling `UnmarshalJSON` or `MarshalJSON` itself (that is a `json.Unmarshal` of the bytes: no options, HTML-escaped output, offsets of syntax errors relative to the value).
   - Error texts say `gp_plain` where they say the type name (`Go struct field gp_plain.name of type string`, `cannot unmarshal string into Go value of type main.gp_plain`); the methods called by hand wrap a failure inside a field's `MarshalJSON` once more (`json: error calling MarshalJSON for type main.T: ...`); the route of the encoder does not.
   - **Addressability.** The methods have value receivers (the plan's signature). A value that `Marshal` reaches through a pointer, a slice element or another addressable place is encoded as the struct would be (its by-value fields whose type has a pointer-receiver `MarshalJSON` are marshaled through the pointer); a value that is not addressable (a map value, the content of an interface, the argument of `Marshal` itself) is encoded as a converted copy, so such a field is marshaled by value, as it is for a plain struct there. Both are what a plain struct does. The methods `MarshalJSON` and `UnmarshalJSON` called by hand see a copy, so for them the field is always marshaled by value (`"A":{"X":0}` where a plain struct reached through `&v` gives `"A":"pr"`). A pointer receiver would fix that, but then the type would not satisfy `json.Marshaler` as a value and `T{}.MarshalJSON()` would not compile, so it was not done.
   - **A cycle through derived types** (a pointer that leads back to a value whose type has `#Derive(JSON)`) is found as Go finds it for a plain struct: `json: unsupported value: encountered a cycle via *main.N`. (A pointer to a derived type is encoded by `marshalerEncoder`, not by `ptrEncoder`, so the patch repeats `ptrEncoder`'s detection there; in the first version of this step each level was a new `Marshal` call, the detection never saw the cycle and the stack guard ended it.)
   - A linked list of derived nodes nests fewer levels than a plain one before the guard when it is *marshaled* (one more frame for each level): 0.87 to 0.94 times (found by binary search: g++ -O0 2776 against 2985, clang++ -O0 2255 against 2429, g++ -release 5783 against 6617, clang++ -release 4236 against 4793); decoding nests the same as a plain struct.
   - **Decoding goes on after a type mismatch**, as it does for a plain struct (the error is kept and the other fields are still set), where the old `UnmarshalJSON` stopped at the first error of the value; for a map key of a derived type with `UnmarshalText` the error is kept and decoding goes on instead of stopping.
   - **The method set.** The type has one more method, `GoCPPJSONPlainPtr`: `reflect` shows it (`NumMethod` of the pointer type goes from 2 to 3, of the value type it stays 1, since the new method has a pointer receiver); `gocpp doc` does not list it. A hand-written method of that name on another type is ignored by `encoding/json`; on a type with `#Derive(JSON)` it is the usual error for a method that the generator adds.
   - **Compile time.** The generated C++ and the compile time of a package with many derived types: 100 types of 5 fields each (a slice, a map, a pointer), generated C++ as above; the build time of the package follows it.
   - **The file must import `encoding/json`** (any name but `_`): the generated code is written with the file's name for the package, as ADR-080 decision 3 already requires for the types of fields. The import needs no other use: the checker does not report it as unused in a file that has a `#Derive(JSON)` (only in that file: another file's unused import is still an error, `tests/pkgs/errors/derive_json_unused_import`). Without it, `#Derive(JSON)` reports `checker/derive-json-import`. A type that is not a struct is an error, as before.
   `examples/macros` (which uses `#Derive(JSON)`) is no longer pending: `TestPendingFiles` lists nothing, `maskedExamples` is empty (its gate lines changed positions between phases because an error is reported once for a message, so
   `TestGateExamples` compares that example's gate errors without positions, `shiftingExamples`).
5. **Stack depth, and the stack guard.** Go rejects documents nested over 10000 deep before it builds anything, and its stacks grow, so nothing else can overflow. gocpp's stacks are fixed (8 MiB for the main thread
   and for each goroutine, D24) and the frames of the reflective code are several KB at `-O0`: before the guard, `{"a":{"a":...` (a document the scanner accepts, 1400 levels) decoded into `type Rec struct{ A *Rec }`
   ended the program with `fatal error: stack overflow` (g++ -O0 about 1375 levels, clang++ -O0 1065, g++ -release 4140, clang++ -release 2167, -asan 679; into `any` 9148 / 6847 / 10000 / 4720; `Marshal` of a linked list
   3193 / 2623 / 6960 / 5121 / 1689). A crash on input that a program does not control is not acceptable, so the port has a **stack guard** (`patches/encoding__json.rules`): `gp::runtime::stack_room()` (a native, runtime/src/runtimepkg.cpp)
   returns how many bytes of the calling thread's stack are left above a reserve (a sixteenth of the stack, 128 to 512 KiB, kept for what runs after the check; it asks `gp::os::stack_bounds`, so it is the real stack of a goroutine, a
   thread of `thread_start`, or the main thread, on Linux and Windows; a thread whose stack the runtime does not know has all the room there is). The decoder checks it in `value` and `valueInterface` (a `*SyntaxError`
   "exceeded max depth (the value is nested too deep for the stack)"; `valueInterface` has no error result, so it panics with a private value that `unmarshal` recovers, the way `encodeState.marshal` does) and the encoder in `ptrEncoder`,
   `sliceEncoder`, `mapEncoder` and `interfaceEncoder`, the only places where a type can recurse (an `*UnsupportedValueError` with the same text). Go's own limit of 10000 stays and comes first. What a build nests before the error is the stack's, about 6% less
   than the figures above; below 200 levels every compiler and mode succeeds, over 10000 every one gives the error. `tests/run/json_deep_input.gp` (`// divergence: D24`) runs the battery on the main goroutine and on another one at
   200, 100000 and 1000000 levels: Unmarshal into `any`, a map, `Rec`, a struct with a `[]Rec` field, `RawMessage`, `Number`, a skipped value, `Valid`, `Compact`, `Indent`, `Decoder.Token` and `Decode`, and `Marshal` / `MarshalIndent` / `Encoder.Encode` of a linked list, nested maps and nested slices. The copy of
   the package in `tests/gotest` has the guard replaced by a function that never trips (a program cannot declare a native), and `TestUnmarshalMaxDepth` nests 4000 levels there, which every mode and compiler holds. The guard costs one call into the runtime for each value that can recurse: A/B of the same three benchmark programs built with and without the guard (g++ -O2, 25 alternating runs each, the machine at a load of 200 to 600, so the figures are best-of-N): `Marshal` of a small struct +0.5% (minimum) and +1.7% (mean of the best fifth), `Unmarshal` -1.5% to +1%, the 1 MB document -1% to +1%; the first version, which passed the `reflect.Value` to the check, cost Marshal about 4%, which is why the call sites test `stackRoom()` themselves and the error carries no value. Medians at that load moved by +-5%, beyond what the guard can cost; a build with the check compiled out (`stack_room` returning a constant) was as slow as the guarded one.

## Consequences

- A program can use `encoding/json` with every feature of Go 1.25's v1 API. `bench/programs/data/json_{marshal_struct,unmarshal_struct,doc}.gp` measure it (below).
- The fixtures make a new class of check cheap: any ported package whose tests are in a module (`convert.py PKG` + `packages/PKG.py`) gets a differential run against real Go. Each fixture takes
  15 to 50 seconds (a C++ build of the package and its tests), so `go test ./tests/gotest -run TestGoTest/_internal` is the focused command (the full `./tests/gotest` runs them all).
- `testing/iotest`, `internal/testenv`, `image`, `math/big` are shimmed in the fixtures (`gotests/shim/`); `TestPortCopies` checks that the shim files in a fixture are the generator's.

## Measurements

g++ 15 and clang++ 21, on the shared machine at a load of 60 to 70 (the gobench tool says so; ratios are gocpp / Go, medians of 7 runs, `-O2 -g0`; "release" is `gocpp build -release`):

| program | Go | g++ -O2 | clang++ -O2 | g++ release | clang++ release |
|---|--:|--:|--:|--:|--:|
| `json_marshal_struct` (200000 `Marshal` of a 10-field struct with a nested struct, slice and map; ms) | 181 | 6.4x | 6.0x | 6.6x | 5.0x |
| `json_unmarshal_struct` (100000 `Unmarshal` of the same document; ms) | 363 | 4.9x | 5.0x | 5.3x | 4.7x |
| `json_doc` (a 1 MB document: `Marshal`, `Unmarshal` into structs and into `any`, `Valid`, `MarshalIndent`, 6 rounds; ms) | 204 | 3.1x | 3.0x | 3.4x | 2.8x |

The geometric mean over the three is 4.6 (g++ -O2), 4.5 (clang++ -O2), 4.9 (g++ release) and 4.1 (clang++ release). The sampling profile of the struct `Marshal` (a SIGPROF sampler preloaded
into the `-O2 -g` binary) puts most of the time in the encoders' own frames and in counted-pointer traffic: copying and dropping `reflect.Value`s, `gp::ptr<encodeState>` and `gp::func` closures
(`__exchange_and_add`, `shared_ptr` control blocks), as the `regexp` profile of ADR-083. No patch tries to hide it; borrowed pointer parameters (the review in progress) and devirtualizing the encoder
funcs would help. At `-O0` (the default) it is several times slower again.

## Alternatives

- **A reflection-free generator for `#Derive(JSON)`** (the plan's table): see 4. It stays possible later behind the same two method names; the differential test in `derive_json_plain.gp` is what it
  would have to keep passing.
- **Hidden import of `encoding/json` for the derive**: ADR-080 decision 3 keeps the language server's package metadata equal to the header of each file; an implicit import would break
  that (the failure of the first version of 10.10). A required import is one line for the user and no new mechanism.
- **Extending `reflect` (`Method`, `Call`, `MapOf`...) before porting**: not needed, so not done; D6 stands.
- **Porting Go's `*_test.go` files as single-file programs with a harness** (ADR-072/083): their expected output is Go's, but a 2800-line `decode_test.go` that names unexported identifiers needs
  the package around it, and `gocpp test` already runs Go's `testing`; the fixture keeps the test files as Go wrote them (plus the listed patches) and uses Go itself as the oracle.
