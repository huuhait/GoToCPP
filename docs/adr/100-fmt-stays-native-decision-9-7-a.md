# ADR-100: `fmt` stays native: decision 9.7 = A

Status: accepted (2026-10-19, decided by the user). Carries out step 9.7 of [Phase 09](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md).
Builds on [ADR-028](028-fmt-on-descriptors.md) (the engine) and [ADR-034](034-portstd-generic-arch-and-rewritten-constraints.md) (the porting tool, which is not used for `fmt`).

## Context

Step 9.7 had two options: port Go's `fmt` on the reflect subset of step 9.3 (A is the other one), or keep the engine of step 5.11, a C++
port of `fmt/print.go` and `fmt/format.go` that walks values by their type descriptors (`runtime/include/gocpp/fmt.hpp`,
`runtime/src/fmt.cpp`), and which also prints C/C++ values. The engine had 210 programs in `tests/run/fmt_*` whose expected output
comes from real Go, but nobody had run Go's own tests against it, and the Scan family did not exist at all.

## Decision

**A: `fmt` is not ported. The native engine is the product.** It is faster than a ported `fmt` over `reflect` (the benchmark rows
`fmt_sprintf` and `fmt_println` did not move in this step), it prints C/C++ values, and its output was already Go's. What the audit
asked of it was evidence, and the holes that the evidence shows.

## What was done

1. **Go's own tests run against the engine.** `tests/gotest/testdata/fmt_gotest` holds Go 1.25's `fmt_test.go`, `errors_test.go`,
   `stringer_test.go`, `state_test.go`, `stringer_example_test.go` and `gostringer_example_test.go` as `_test.gp` files
   (the header of each says what was changed), run by `gocpp test` and compared with `go test -v` by `tests/gotest`. Every test
   passes but `TestCountMallocs`, which skips (allocation counts are not Go's: a documented divergence), and two tests of unexported
   functions (`TestIsSpace`, `TestParsenum`, through `export_test.go`), which are left out. The Scan family has `tests/run/fmt_gotest_scan.gp`
   (Go's `scan_test.go` through the harness of the other `*_gotest_*` programs) and the `fmt_scan_*` programs.
2. **A differential generator** (`tools/fmtdiff`): seeded programs that print combinations of verb, flag, width, precision and value
   with real Go and with gocpp and report the cases that differ; a small always-on part runs as a Go test.
3. **Bugs and gaps found, and closed** (each has a `tests/run` program with Go's output):
   - `import . "fmt"`: `Sprintf(...)`, `Fprintf(&buf, ...)` and `f := Sprintf` did not lower (`fmt_dot_import`).
   - `fmt.FormatString(v, 'x')` with a value of a type that implements `fmt.State` did not compile (`fmt_dot_import`).
   - `fmt.Fprintf`, `Fprint`, `Fprintln` and `FormatString` as func values (`h := fmt.Fprintf`) were refused (`fmt_dot_import`, `fmt_go_stmt`); the `import . "fmt"` and func-value lowering was written twice (here and in the Scan work) and merged into one.
   - `Fprint*` with a writer of the program's own in a program that does not import `io` failed ("the type io.Writer is not available");
     the build now adds `io` to such a program, as Go's `fmt` imports it (`fmt_writer_no_io`).
   - `Println` and the other `Print` functions returned `(len, nil)` when the write to descriptor 1 failed (a full disk, a closed descriptor); they
     return `(0, error)` with Go's text (`write /dev/stdout: no space left on device`; `fmt_stdout_write_error`).
   - `Print`, `Println` and `Printf` wrote to descriptor 1; Go writes to the file that the variable `os.Stdout` holds, so the usual
     way to capture output (`os.Stdout = w`) did not work, and a closed `os.Stdout` gave no error. The init of package `os` now registers
     with `fmt` the address of the variable, the `*os.file` inside it and a flag that `File.Close` sets for the original
     (`gp::fmt::set_os_stdout`); every `Print` compares three words, inline, and writes through `os.Stdout.Write` only when the variable holds
     another file (or nil), when the file's contents were replaced (`*os.Stdout = *w`) or when the original is closed; otherwise it takes
     the direct path to descriptor 1. `Scan`, `Scanln` and `Scanf` follow `os.Stdin` the same way, in every package and through func
     values (`gp::fmt::set_os_stdin`). Tests: `fmt_stdout_capture`, `fmt_stdout_closed`, `fmt_scan_stdin_os`, `fmt_scan_stdin_fd0`
     (a pipe dup2'ed onto descriptor 0 inside the program), and the unit test `test_stdin` of `runtime/test/fmt_test.cpp`, which reaches the cases a
     program compared with Go cannot (an empty non-blocking descriptor 0 that is waited out, a read interrupted by a signal) through the test seams of
     `gocpp/os.hpp` (`gp::os::testing`; the runtime's tests may not include OS headers).
   - `fmt.Println(a...)` read the operands of the slice before printing; Go reads each element when it prints it, so a `String`
     method that stores into the slice changes the operands after it (`fmt_spread_live`, `fmt_spread_live2`, `fmt_spread_live3`: the general verb path, argument indexes, `*` widths, `%w` and `Errorf`; a call without `a...` runs none of it: the reads of the slice are in cold functions behind one test of a null pointer).
   - `Sprintf("%w", []byte(...))` printed one bad verb for the whole slice; Go's type switch sends an unnamed `[]byte` to `fmtBytes`,
     which prints each element with the bad verb (`fmt_verb_w_bytes`).
   - A type declared inside a func literal in the value of a package variable ("not supported yet: type P"), found by Go's `fmt_test.go`
     (`types_local_in_var_init`), and an array literal in an `any` package variable (a `constinit` error), found by the generator.
   - A value of a named pointer type (`type P *T`, `type U unsafe.Pointer`) in an interface printed garbage and read outside its box
     (found by the review, `fmt_named_ptr_iface`). A `fmt.State` or `fmt.ScanState` that a method keeps after the call used a dead stack
     frame; it now stops pointing at the printer or scanner (`fmt_saved_state`): a State reads as unset, a ScanState meets the end of the input
     and `UnreadRune` panics with the nil dereference of Go's freed scanner. `Sscan("NaN")` returns `math.NaN()` with Go's bits.
   - The Scan family (`Scan`, `Scanln`, `Scanf`, `Sscan*`, `Fscan*`, `fmt.Scanner`, `fmt.ScanState`) did not exist. See section "Scan" below.

## Cost of the print path

Measured with user-mode instruction counts (3 million calls, `-O2 -g0`, compared with dev before this step): `fmt.Println` of six operands +1.9%
(g++) and +2.0% (clang++), `Sprintf` of four verbs +0.4% (g++) and +0.06% (clang++), `Printf` of three verbs +1.4% and +0.8%; wall-clock (20 alternating rounds,
pinned cores, plain and 64-byte-aligned builds) is equal within the noise of the machine, with no layout cliff left. What keeps it there: the live-operand reads
(`f(a...)`), the saved-State bookkeeping and the `os.Stdout` gate are one null test, one null test and one thread-local plain store each, with
everything else in cold functions; the printer's file stays the size it was because the gates and their slow paths are in `runtime/src/fmtstd.cpp`
(moving them out of `fmt.cpp` took the cost of a `Sprintf` from 55 to 14 instructions: GCC's inlining of `pp`'s helpers depends on the size of the unit).
A State is one heap object per Printf call, created by the first `Format` method (28 MB of memory for `Sprintf("%v", 2M elements with Format)`,
as before); a Scan call makes one ScanState for all its operands. Scanning two million `Scanner` operands in one `Sscan` uses 132-225 MB (Go: 38 MB), and a plain `*int8` list of the same size 133 MB
(Go: 37-40 MB): an operand costs about 32 bytes in the operand vector, besides the 32 of the `[]any`.

## Scan

`Scan`, `Scanln`, `Scanf`, `Sscan`, `Sscanln`, `Sscanf`, `Fscan`, `Fscanln`, `Fscanf`, `fmt.Scanner` and `fmt.ScanState` are a C++ port
of Go's `fmt/scan.go` (`runtime/src/fmtscan.cpp`, `gocpp/fmtscan.hpp`) driven by the descriptors of the pointer arguments, as the printer is
driven by the descriptors of its operands; the strconv pieces it needs (ParseInt, ParseUint, ParseFloat, Atoi, Unquote) are ported in the same file.
`Fscan` uses `ReadRune`/`UnreadRune` of the reader when it has both, else Go's `readRune` wrapper with its lost look-ahead. `Scan*` read
descriptor 0 themselves, a byte at a time and unbuffered (read errors say `read /dev/stdin: bad file descriptor` and the like), until the
program assigns another file to `os.Stdin` or closes it; they read that file then. Tests: `fmt_scan_{verbs,errors,newlines,scanner,readers,named}` (output from
real Go), `fmt_gotest_scan` (all 25 test functions of Go's `scan_test.go`, check counts equal to Go's) and, in the `fmt_gotest` fixture, Go's own
`scan_test.go` and `example_test.go` run by `gocpp test`. Seven mutations of `fmtscan.cpp` (the space rule, the EOF kinds, the hex-float prefix, the Scanner EOF
conversion, the float verb check, `nlIsEnd`, the underscore rules) were each caught.

**`io.EOF` and `io.ErrUnexpectedEOF` are the runtime's two values** (`tools/portstd/patches/io.rules`, `stdlib/io` regenerated): the scanner, written in C++, returns
them, and `err == io.EOF` must hold for what it returns.

Differences from Go: a number error has Go's text but is a plain error, not a `*strconv.NumError` (`errors.Is(err, strconv.ErrRange)` is false); scanning into
a pointer to a C/C++ type returns the error "type not a pointer" instead of scanning; the scanner compares with the runtime's `io.EOF`, so a program that assigns
another value to the variable `io.EOF` does not change what the scanner returns; on Windows stdin is read with `ReadFile` (no console UTF-16 read). The scan of
standard input goes through `Fscan` over `os.Stdin` when it was replaced, which reads a byte at a time like Go's (a `bufio.Reader` over `os.Stdin`: 0.56 s
for a million integers against Go's 0.25 s, because of the ported `bufio`'s `ReadRune`).

## Known divergences (not closed)

- **Allocation counts** (`TestCountMallocs`): gocpp allocates differently.
- **Nesting depth.** The printer recurses once per level on the C++ stack: a value nested about 10,000 levels deep (a slice of slices of `any`
  built by a loop) overflows the 8 MiB stack, where Go goes on past 100,000, and a `Format` method that formats its own value again overflows
  at about 3,900 levels (a bit shallower than before step 9.7: the printer's frames grew by the live-operand state). Job 9.11's
  `stack_room` guard is the model for a check that turns it into Go's fatal error. Fixing it would mean running the walk on a separate stack, which
  breaks the goroutine-local state that methods called by `fmt` use.
- **The internals of `sync` and `sync/atomic` types.** `fmt.Printf("%+v", struct{ mu sync.Mutex })` prints gocpp's layout
  (`{mu:{state:0 sema:0}}`), Go 1.25's is `{mu:{_:{} mu:{state:0 sema:0}}}`: the structs of package `sync` are flatter than Go's
  (`stdlib/sync/PORT.md`).
- **`reflect.Value.Method`** does not exist, so the table rows of `fmt_test.go` that print a method value through `reflect` are left out.
- **Swapping `os.Stdout`, `os.Stdin` or `os.Stderr` while other goroutines print or scan.** It is a data race in Go too, but a harmless one
  there (the garbage collector keeps the old file alive). A gocpp `*os.File` is reference counted, and a plain store frees the old file under a
  goroutine that has just read the pointer. The lowering therefore writes every store to one of the three variables as an atomic store
  (`gp::fmt::store_std`; the target of every store form is made in one place, `fg.lhs`: assignments, multiple assignments, the init and post
  statements of a `for`, the variables of a `range`, a receive in a `select`; checked by `TestStdStores` in `cmd/gocpp` and by
  `fmt_stdout_form_*`), and `fmt` reads the variable through an atomic snapshot (`std::atomic_load` of the shared pointer) and calls `Write` or
  `Read` on that snapshot, so fmt never touches a freed file (`fmt_swap_stdout_race`, `fmt_swap_stdin_race`; 0 crashes in 100 runs of each
  stress program on both compilers). Not covered: a store through `&os.Stdout`, a store into the `File` through another pointer to it
  (`*os.Stdout = *w` is covered, but only at the assignment itself), a store by `reflect`, and **the program's own reads of the variable racing
  with the store** (`fmt.Fprintln(os.Stdout, ...)` in a goroutine while another one swaps `os.Stdout` crashes, where dev and Go do not: the
  program's read of the variable copies the shared pointer without the atomic load). Because the store is atomic, `-race` does not report the race
  that Go's race detector reports for such a program (a plain store of Go's would be reported).
- **Close of the original `os.Stdout` or `os.Stdin`.** `File.Close` marks the file closed at once (later `Write` and `Print` fail with `file already
  closed`) and closes the descriptor with Go's own rule for a descriptor in use: the last user out closes it. A `Print` on the fast path announces
  itself in a slot of its thread with plain stores (no atomic read-modify-write on every `Print`); the close hook of package os
  (`poll.CloseFunc`, `stdclose_*.gp`) asks `gp::fmt::defer_close`, which sets bit 0 of the gate, makes every thread pass a memory barrier
  (`membarrier(2)` on Linux, `FlushProcessWriteBuffers` on Windows; where the system has none, bit 0 is set from the start and every `Print`
  takes a counted slow path), and either lets the caller close the descriptor (nothing in flight: `Close` takes microseconds, also on Windows) or keeps the
  closing for the last `Print` (or `Scan`, which is counted) that finishes (`finish_close`). No `Print` reaches a descriptor that the next file
  reuses (`fmt_close_stdout_race`, `fmt_close_stdout_inflight`, the unit tests `test_close_defers`, `test_gate_start`). Differences from Go: when a
  write is in flight (a non-blocking pipe with a stalled reader, say) the descriptor is not free for the next file until it ends, so that file gets another
  number than 1, and the write is completed (Go cuts it short with `file already closed`). If the process has no descriptor 1 at start (`prog >&-`),
  Go's runtime opens `/dev/null` for 0, 1 and 2, and gocpp's does not (a runtime TODO, outside fmt).
- **A saved `fmt.State` or `fmt.ScanState`** (a method that keeps its argument). One State is made for each Printf call (every `Format` gets the
  same one, as in Go) and one ScanState for each Scan call. After the call a State reads as "not set" and its Write goes nowhere (Go's printer is back
  in its pool, with stale flags that the next call overwrites); a ScanState behaves as Go's freed scanner: `ReadRune`, `SkipSpace` and `Token`
  meet io.EOF (an empty non-nil token) when the scan had reached the end of its input (or its argument limit) and panic with a nil dereference when it
  had not, `UnreadRune` always panics, `Width` reports the width of the last argument (`fmt_scanstate_dead`, `fmt_scanstate_dead_limit`).
  Differences: a State saved by a `Scan` method that panics is destroyed here, where Go's stays usable; after a token longer than 1,024 bytes Go
  never frees the scanner, so the reads of a saved ScanState go on there and panic here. **Using a State or ScanState from another goroutine
  while the call that made it is running, or racing with its end, is not supported: it can crash** (a use-after-free of the printer's buffer or
  the scanner, found by the review with `s2_goroutine_state` and `ss3_conc`), where Go's stays memory-safe though racy. Use after the call has
  returned (with a happens-before edge) is safe, as above.
- **A throw out of `fmt`.** A panic that escapes from a method called by `fmt` while another panic is running does not print Go's
  `[recovered]` chain on standard error; the exit status and the first lines are the same.
- **Package initialization order.** A package that imports only `fmt` may initialize before one that imports `os` (gocpp's order is a
  topological order of the imports, and `fmt` is not a package of the program); a `Println` in the init of such a package writes to
  descriptor 1, which is where Go's writes too unless an earlier init changed `os.Stdout`.
- **The gate.** With `GOCPP_GATE_PHASE` below 9 the build refuses `fmt.Fprint` to a writer of the program's own (the package `io` is not available yet);
  since `io` is added to such a program, it builds at those phases, which only matters while a phase is being built.
- **Speed of the scanners over a Go reader.** `Fscan(os.Stdin)` unbuffered is about 1.6 to 1.9 times Go's CPU and `Fscan` over a
  `bufio.Reader` about 2 times (0.50 s against 0.25 s for a million integers): the reader's `Read` and `ReadRune` are Go code called through the
  method table, once per rune. A cheap win was not found.
- **Println of an empty text** (`fmt.Print()`) makes the write call that Go's does (it fails on `/dev/full` or a closed descriptor, as in Go) on Linux; on Windows it makes none.
- **A C++ `operator<<` that throws** is reported as a `String method` panic.
- **`map[any]V` with keys of different dynamic types** prints the groups in an order of its own (Go orders them by type address, which is unspecified).
- **Pointer values** (`%p`, `&{...}` inside a value) print gocpp's addresses.

## Consequences

`fmt` is covered by Go's own tests where they can run, by 220 programs compared with real Go and by a generator; a change to the engine
runs the fixture (`go test ./tests/gotest -run fmt_gotest`) and `TestRun/fmt_`. Nothing from Go's `fmt` is vendored, so there is no
`PORT.md` of files taken: `stdlib/fmt/PORT.md` says what is native and what differs.
