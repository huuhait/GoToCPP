# ADR-073: `io`, `io/fs`, `path`, `strings`, `bytes`, `bufio` and `context` are Go's own packages

Status: accepted (2026-10-20)

## Context

Step 9.4b is the second slice of "port the tier-1 pure packages" (ADR-072 was the first: `unicode`, `strconv`, `errors`). Its
prerequisites are merged: `sync` and `sync/atomic` (8.7), `time` (8.8), `runtime` (8.10), the internal shims (9.2), the reflect
subset (9.3), `math` (9.5), and `os` with copies of `io` and `io/fs` (9.6, ADR-067). What was still stub or copy:

- `strings`, `bytes`, `bufio`, `context`: declaration stubs from Phase 01 (Phase 05 gave `strings` and `bytes` no bodies at all).
- `io` (3 files) and `io/fs` (7 of 9 files): copies from Go made by hand in step 9.6, with `PORT.md` files saying so. `io/fs`
  lacked `Glob`, `GlobFS`, `Sub`, `SubFS` because they import `path`, and had two local helpers (`sortDirEntries`, `pathJoin`).
- `path`: not there.

Three things were not known beforehand:

1. Whether Go's code compiles and runs unchanged. It does, for all of it. The only things gocpp cannot take are the five
   functions per package that return an iterator (`Lines`, `SplitSeq`, `SplitAfterSeq`, `FieldsSeq`, `FieldsFuncSeq`), because
   `iter.Seq[V]` is a generic type and generic types (step 7.3) are not merged.
2. Whether `strings.Builder` behaves under reference counting. It did not: see decision 3.
3. Whether the speed is acceptable. Searches and bulk operations run on the natives of step 9.2 and are within 0.3x to 2x of Go
   at -O2; everything that calls a method per byte (`Builder.WriteByte`, `bytes.Buffer.WriteByte`, `bufio` per byte) is 3 to 20 times
   slower, and the causes are not in these packages (Consequences).

## Decision

1. **The seven packages are `tools/portstd` output** (`go run ./tools/portstd -check` covers them). `io` is byte for byte the 9.6
   copy plus the header line, so `io.EOF` and every declaration keep their identity. `io/fs` and `path` replace the copy and the
   missing package. `PORT.md` of each says what was taken and patched.
2. **Patch rules, each with a reason** (`tools/portstd/patches/{strings,bytes,io__fs}.rules`):
   - `strings` and `bytes`: `@skip iter.go` (the five iterator functions per package, "needs generic types (7.3)"). Both are
     listed in `Missing` of `tools/stubgen`, so `TestRealPackagesMatchStub` still compares every other declaration with Go's.
     The rule goes with step 7.3; nothing else changes.
   - `io/fs`: `ReadDir` sorts with a local merge sort (`sortDirEntries`) instead of `slices.SortFunc`, because `slices` is a stub until
     step 7.10. The names of a directory are distinct, so the order is Go's. The rule goes with step 7.10. (The 9.6 copy had a
     second helper, `pathJoin`; with `path` ported `walk.go` is Go's.)
3. **`strings.Builder` keeps its address as a `uintptr`** (`strings.rules`). Go's `copyCheck` stores `b.addr = b`, a `*Builder` in
   the Builder itself. In gocpp a pointer field is a counted reference (ADR-004), so every Builder owned itself and its buffer was
   never freed: 200000 builders of 1 KB took 296 MB (3 MB after the rule). The check and its panic text are Go's; a copy of a
   non-zero Builder has another address (step 4.10's model Builder in `tests/interop/unsafe_builder.gp` does the same).
   `tests/run/strings_builder_noleak.gp` reads `/proc/self/statm` (Linux only) and fails when 1200000 builders of 1 KB leave more than
   640 MB behind (Go's original code leaves 1.4 GB). (No other package of this step stores a pointer to itself; `context`
   keeps its parent/child references until `cancel` removes them, as in Go with its GC, so a context that is never canceled
   leaks: D1 of `docs/spec/language.md`; `context_gotest_x` carries `// lsan: cycle`.)
4. **Tests are Go's own `_test.go` files**, converted by a script (not kept, as in ADR-072: step 9.10 runs them with `gocpp test`)
   into single-file programs with the same ~70-line harness (`testT`, `runTests`, a count of the `if` statements the tests
   execute, so a test that returns early shows in the expected output). 339 `Test*` functions and 158 `Example*` functions of
   the seven packages run: `tests/run/{strings,bytes,io,bufio,context,path,iofs}_{gotest,goexamples}_*.gp`. The expected output
   comes from real Go 1.25.14 and `TestRunVsGo` checks it again. Not run, each with its reason (the first comment of the file says so too):
   - *Iterators (generic types, 7.3):* `strings` and `bytes` `TestLines`, the `Seq` parts of `TestSplit`, `TestSplitAfter`,
     `TestFields`, `TestFieldsFunc`, and the five iterator examples of each package.
   - *Unexported names through `export_test.go`:* `strings` `TestPickAlgorithm`, `TestGenericTrieBuilding`, `TestFinderNext`,
     `TestFinderCreation`; `bufio` `TestSpace`, `TestErrAtEOF`; `context` `TestParentFinishesChild`, `TestChildFinishesFirst`,
     `TestCancelRemoves`, `TestCustomContextGoroutines` (the `XTest` wrappers of `context_test.go`). Where the helper is a one-liner
     it is replaced: `Scanner.MaxTokenSize(n)` is `Scanner.Buffer(nil, n)`, the three error values of `io` are taken from the package.
   - *Allocation counts:* `strings` `TestBuilderAllocs` and `TestBuilderGrowSizeclasses`, `bufio` `TestReadStringAllocs`, `io`
     `TestMultiWriter_WriteStringSingleAlloc`. Elsewhere `testing.AllocsPerRun` runs its function and returns 0, so those
     comparisons pass and measure nothing.
   - *`runtime.AddCleanup` and the GC:* `io` `TestMultiReaderFreesExhaustedReaders`.
   - *Other packages gocpp lacks:* `bytes` `ExampleBuffer_reader` (`encoding/base64`) and `ExampleCompare_search` (`slices`);
     `context` `TestDeadlineExceededIsNetError` and `ExampleAfterFunc_connection` (`net`); `bufio` and `bytes` `net_test` /
     `boundary_test` (`net`, `syscall.Mmap`); `FuzzReplace` (fuzzing).
   - *Depends on the machine:* `io/fs` `ExampleWalkDir` (walks `/usr/local/go/bin` and has no output). The other three `io/fs`
     examples (`ExampleGlob`, `ExampleReadFile`, `ExampleValidPath`) run on a local `MapFS`.
   - *Stack:* `io/fs` `TestCVE202230630`, a 10001-level recursion of `Glob`, needs more than the 8 MB goroutine stack at -O0
     (see Consequences).
   - *Optimised builds:* the two call depth checks of `io` (`TestMultiWriterSingleChainFlatten`, `TestMultiReaderFlatten`) count
     frames with `runtime.Callers`, and inlining at -O2 / `-release` makes the count smaller than Go's. They only fail when the depth
     is greater than Go's (a flattening that stopped working); the whole set passes at -O0 and with `-release`, g++ and clang++.
   Packages the tests import that gocpp does not have are local copies in the test file:
   `testing/iotest`'s readers (for `bufio`), `testing/fstest.MapFS` (for `io/fs`), a splitmix generator for `math/rand`,
   `slicesEqual` for `slices.Equal`. Four tests whose goroutines decide how many checks run print `checks: -1`; the counter is atomic.
5. **Differential sweeps** (`tests/run/{strings,bytes,bufio,io,path}_diff_ops.gp`) run 60000, 60000, 6000, 5000 and 200000
   pseudo-random inputs (ASCII, multi-byte, invalid UTF-8; readers that return random chunks, empty reads, errors with data) through
   the API and print one FNV hash per function; the hashes come from real Go.
6. **No performance rule beyond decision 3.** The numbers of the review below show where the time goes.

## Consequences

Measured on the shared machine (load 120 to 160, best of two runs pinned to one core), the same program in Go 1.25.14 and in gocpp
at -O0 (the default build), at -O2 (`GOCPP_CXXFLAGS=-O2`) and with `-release` (`-O2 -flto`). Times for 1 MB of text unless stated;
the last three columns are ratios gocpp/Go.

| | Go | -O0 | -O2 | -release |
|---|--:|--:|--:|--:|
| `strings.Index`, miss then hit | 0.36 ms | 5.0x | 1.4x | 1.4x |
| `strings.IndexByte` | 20 us | 0.4x | 0.5x | 0.3x |
| `strings.Count` of a byte | 14 us | 22x | 8.8x | 7.4x |
| `strings.Split(" ")` | 1.2 ms | 22x | 4.6x | 4.4x |
| `strings.Fields` | 3.9 ms | 8.0x | 0.9x | 0.7x |
| `strings.ToUpper` | 4.2 ms | 53x | 5.8x | 4.7x |
| `strings.ReplaceAll` | 0.94 ms | 4.4x | 0.9x | 1.1x |
| `strings.Replacer`, 3 bytes | 1.6 ms | 9.9x | 1.0x | 0.8x |
| `strings.Builder.WriteByte`, 1M calls | 1.3 ms | 156x | 20x | 18x |
| `strings.Builder.WriteString`, 100k calls | 0.17 ms | 143x | 19x | 17x |
| `strings.Map` | 2.4 ms | 86x | 12x | 11x |
| `bytes.Index` | 0.41 ms | 6.1x | 1.4x | 1.2x |
| `bytes.Buffer.WriteString`, 100k calls | 0.53 ms | 60x | 4.8x | 4.3x |
| `bytes.Buffer.WriteByte`, 1M calls | 2.5 ms | 89x | 9.9x | 6.6x |
| `bytes.Fields` | 3.3 ms | 11x | 0.9x | 1.1x |
| `bufio.Scanner` lines, 100 MB | 30 ms | 34x | 3.9x | 3.6x |
| `bufio.Scanner` words, 100 MB | 584 ms | 32x | 2.9x | 2.8x |
| `bufio.Reader.ReadString`, 100 MB | 106 ms | 29x | 2.7x | 2.7x |
| `io.Copy` 100 MB between memory buffers | 45 ms | 1.5x | 1.7x | 2.0x |
| `bufio.Writer` 20M `WriteString` + `WriteByte` | 136 ms | 58x | 4.5x | 4.0x |
| `io.Pipe`, 32 MB in 32 KB writes | 2.5 ms | 13x | 11x | 11x |

- **Where the time goes.** The searches run on the natives of step 9.2 and are within 0.3x to 1.6x of Go at -O2 (`Count` of one byte is
  8x: the native counts by words, Go by AVX2). Everything that calls a method per byte pays two costs of the compiler and the
  runtime, not of the ports: a `*T` receiver is a counted pointer passed by value (two atomic operations per call), and `append` and
  re-slicing of `b.buf` copy the counted slice header (a plain `append(buf, c)` costs 6 ns at -O2, Go 1 ns).
  `Builder.copyCheck` adds a counted `unsafe.Pointer(b)` for its address: 9 of the 23 ns of a `WriteByte` in a micro benchmark
  (23 ns with the check, 13 ns without; `bytes.Buffer.WriteByte`, which has no check, takes 22 ns through a function value).
  A lowering of `uintptr(unsafe.Pointer(p))` that does not copy `p`, and receivers passed by reference, would remove most of it;
  both are compiler work (Phase 15), so no patch rule tries to hide them. `io.Pipe` hands data from a thread to a thread instead of
  a goroutine to a goroutine.
- `Glob` with a 10001-level pattern overflows the 8 MB stack at -O0 (about 800 bytes of stack per level). A bigger stack for
  the main goroutine (or `debug.SetMaxStack`) would make `TestCVE202230630` run.
- `bufio`, `bytes`, `strings` and `io` no longer have stubs, so the declaration check (`TestRealPackagesMatchStub`) now
  compares the ports with Go's declarations (without the iterator functions), and `TestStdPackagesMatchStubs` counts
  stubs plus real packages (at least 25).
- Left for the next slices of 9.4: `sort` (needs `slices`, 7.10), `regexp` and `regexp/syntax` (need `slices`, `sort`, and the
  generic `iter`-free parts compile once `slices` is real). `strings`/`bytes` iterator functions wait for 7.3 (one line each in
  `strings.rules` and `bytes.rules` goes away). `path/filepath` comes with `os` on Windows (Phase 14: it chooses by the system).
