# ADR-136: `-release` builds have no debug information, and `-strip` drops the symbol table

Status: accepted (2026-10-21; job 15.size); amended 2026-10-23 (user decision: `-release` prints only the crash message, `-trace` brings the trace back).

## Context

The Windows ImGui demos built by `gocpp build -os windows` were 63 MB. A Windows hello world was 13 MB, a Linux one
5.6 MB. The cause is the default build, `-O0 -g` ([internal/build/flags.go](../../internal/build/flags.go)): debug
information is most of the file, and a MinGW `.exe` also carries the COFF symbol table (4.6 MB for a hello world,
with long mangled names in the string table). `-release` (`-O2 -flto`) already existed but kept `-g`: the Linux
hello world was still 5.1 MB, and the Windows one 4.9 MB.

Measured on the Windows hello world (13.3 MB): 6.7 MB of `.debug_*` sections, 4.6 MB of symbols, 2.0 MB of
program. The link already used `--gc-sections` with `-ffunction-sections -fdata-sections`, Windows links `-static`
(no DLLs), and Linux links libstdc++ dynamically; none of that was the problem.

## Decision

- The default stays a debug build. Debugging is a feature of gocpp (`gocpp debug`, `break hello.gp:12`, the printers
  of gocpp-gdb.py), and a panic's trace reads Go function names and `file.gp:line` from the symbol table and the
  DWARF line table of the program's own file. A person who runs `gocpp build` and debugs the result must find
  the debug information there. The sizes are documented where users look: `docs/spec/build.md` ("Program size"),
  `docs/guides/windows.md` ("Small programs"), `docs/guides/windows-cross.md` and the READMEs of the demos.
- `-release` is `-O2 -flto -g0`: the compiles and the link both get `-g0`. The link needs it because with
  `-flto` the compiler writes the debug information at the link, from the `-g` that every link line has
  (measured: `-cflags=-g0` alone left 237 KB of `.debug_info` in the Windows hello world, and made a Windows
  `-g1` build identical to `-g0`). The symbol table stays, so a panic prints Go function names, without file and
  line (`?:0`; the trace of an inlined call in an optimized build was already only approximate). This changes
  what `-release` produces, not its code.
- `-strip` is new: the link's `-s` (macOS: `-Wl,-S -Wl,-x`, since ld64 has no `-s`). It drops the symbol table and
  every `.debug_*` section, also the gdb printers (84 KB). A panic then prints `?()` and addresses, as the runtime
  already did for a stripped program (`runtime/README.md`). `-strip` with `-debug`, `-race` or `-asan` is an error:
  their tools need the symbols. `-strip` works with the default build too (585 KB for the Linux hello world).
- The small build is `-release -strip`: Linux hello 224 KB, Windows hello 1.4 MB, `demos/imgui` 2.5 MB,
  `demos/imgui-dx11` 3.3 MB. `-release -cflags=-g -ldflags=-g` is the documented way to keep the debug information of an
  optimized build (later flags win).
- `gocpp debug` has the build flags. `-strip` is an error there (exit 2). `-release` prints one line on standard error
  ("-release builds have no debug information") and goes on, unless the user's flags put the debug information back at
  the compiles and at the link: a `-g` flag (not `-g0`; the last `-g` flag wins) in `-cflags`/`-cxxflags` or `GOCPP_CFLAGS`/
  `GOCPP_CXXFLAGS`, and one in `-ldflags` or `GOCPP_LDFLAGS`. Both are needed because `-release` puts `-g0` on both
  and the link writes the debug information under `-flto`. `-release` used to keep `-g`, and a person may still want an
  optimized program under gdb.
- Both flags are in the key of the up-to-date check (`extraFlags.keyParts`). `-g0` reaches the runtime library
  through the layered flags, whose cache key already follows them.

## Rejected

- **Stripping by default for `-os windows`.** The default build is the build that `gocpp debug` and `gocpp run` use; a
  different default per target would make the Windows build undebuggable for the people who need it most (no Windows
  debugger runs without symbols), and surprise those who copy the same command from the Linux guide.
- **Stripping in `-release`.** A shipped program that panics would print only addresses, and Go's own release
  builds keep their function names in the panic trace. The cost of keeping them is the symbol table: 2.0 MB on
  Windows (3.4 MB against 1.4 MB for a hello world), 150 KB on Linux. Users who want the smallest file add `-strip`.
- **`-gline-tables-only` (`-g1`) for `-release`.** It gives `file.gp:line` in traces, but the trace of optimized,
  inlined code points at the runtime's lines (the panic of an inlined `xs[i]` showed `trace.cpp:47`), and it adds
  1.1 MB to the Linux hello world (1.5 MB against 375 KB) and 0.4 MB to the Windows one (3.9 MB against 3.4 MB).
- **`-Os`.** 154 KB against 224 KB for the Linux hello world, but g++ 15.3 for MinGW fails with an internal compiler
  error in `-Os -flto` (Windows hello and the dx11 demo). It stays a user choice (`-cflags=-Os -ldflags=-Os`).
- **A smaller runtime.** The code of the stripped Windows hello world is 1.0 MB, and libstdc++'s locale facets
  (`time_get`, `money_get`, from the one `std::ostringstream` of `runtime/src/fmt.cpp`) are a part of it. Replacing
  that stream would shrink every Windows program; it is a runtime change of its own and not part of this ADR.

## Consequences

- `-release` programs are not debuggable by symbol and line any more (gdb still has function names from the symbol
  table). The documentation says what to add.
- A panic trace in a `-strip` program shows addresses. `addr2line -e <unstripped build> <pc>` names them when the
  unstripped build has the same flags and source.
- `.debug_gdb_scripts` (84 KB) stays in `-release` builds: it is in the runtime library of every program and not
  debug information in the compiler's sense; `-strip` removes it.

## Amendment (2026-10-23, user decision; job dev.releasetrace): `-release` prints the crash message only, `-trace` brings Go's trace back

The `-release` trace of this ADR named the functions but had no file and line (`main.main()  ?:0`), and printed the
inlined functions' code under the caller's name. The user decided against both half measures (keeping the symbol
table's names only, or paying for line tables in every `-release` program):

- **Plain `-release` prints only the message lines of a crash**, then exits 2 as before: `panic: <msg>` (with Go's
  wording of a chain: `panic: A [recovered]` / `\tpanic: B`, `panic: A [recovered, repanicked]`), the `[signal ...]`
  line of a nil dereference, `fatal error: <msg>` (a deadlock and `concurrent map writes` too), and nothing else: no
  `goroutine N [running]:`, no frames, no `created by`, no other goroutines. It is what Go prints with
  `GOTRACEBACK=none`, which is how `TestRunVsGo` compares these tests with real Go. `GOTRACEBACK` and
  `debug.SetTraceback` don't bring the trace back. The program's own APIs are unchanged: `runtime.Stack`,
  `runtime.Callers`, `runtime.Caller` and `debug.PrintStack` work as before (function names from the symbol table,
  no lines without debug information).
- **How the runtime knows**: the driver adds `-DGOCPP_NO_TRACE=1` to the compiles of a `-release` build without
  `-trace` (the runtime library is built with the program's flags, so the macro is in its cache key). In the runtime,
  the traceback level is then the constant 0 (`trace.cpp` `tb_get`), the stack overflow report drops its goroutine
  line, and the reader of the DWARF of the inlined calls (below) is compiled out. The symbol table stays (the
  program APIs need it). Measured on Linux, against this ADR's `-release` (bytes):

  | program | compiler | ADR-136 `-release` | `-release` now | `-release -trace` |
  |---|---|---|---|---|
  | `examples/hello` | g++ 15.3 | 396,904 | 389,696 (-1.8%) | 1,773,288 (4.6x) |
  | `examples/wc` | g++ 15.3 | 2,866,656 | 2,859,624 (-0.2%) | 11,597,952 (4.1x) |
  | `examples/hello` | clang++ 21 | 406,536 | 324,144 (-20%) | 1,507,032 (4.6x) |
  | `examples/wc` | clang++ 21 | 4,177,704 | 4,091,680 (-2.1%) | 13,917,064 (3.4x) |

- **`-trace`** (with `-release`) is the full Go trace: the compiles and the link get `-O2 -flto -g1
  -fno-optimize-sibling-calls` instead of `-g0` (clang's `-g1` is `-gline-tables-only`; for clang the build adds
  `-fdebug-info-for-profiling`, which puts the linkage names of inlined functions into the line tables, as g++'s
  `-g1` has them). No `GOCPP_NO_TRACE`, so `GOTRACEBACK` and `debug.SetTraceback` work as in a debug build, and a
  fatal error prints every goroutine. The driver defines `GOCPP_TRACE`, and the runtime then reads the DWARF of the
  inlined calls (`DW_TAG_inlined_subroutine` in `.debug_info`, with `.debug_abbrev`, `.debug_rnglists`/`.debug_ranges`,
  `.debug_addr` and `.debug_str_offsets`): a call that the compiler inlined is a frame of its own, `main.get(...)` at
  the line of its code, without an offset, as Go prints an inlined call, and the function around it shows the line of
  the call, not the line of an inlined runtime header (the `trace.cpp:47` of the rejected `-g1` above). A call in tail
  position would leave no frame of its caller, which Go's trace always has, hence no sibling calls.
  `defer_frame::gp_run` is `noinline` now, so that `-flto` can't inline it into the function with the defers (its
  mark's return address and the frame that separates a deferred call from the panicking function need it). `-trace`
  without `-release` changes nothing (a debug build has the trace); `-trace -strip` is an error (the trace needs what
  `-strip` drops). The user pays the size only when asking for the trace.
- Limits of `-release -trace`: g++'s `-g1` names a lambda's `operator()` without a linkage name, so a func literal
  that g++ inlined would have no frame. The lambda of a func literal that a `go` or `defer` statement runs is
  therefore kept out of line in a g++ `-release -trace` build (`GP_TRACE_FRAME` in `gocpp/rt.hpp`, which the
  emitter puts on those lambdas: `__attribute__((noinline))`, empty otherwise), so it has its frame,
  `main.main.func1()` with its offset, as in Go (which inlines neither). Any other func literal that g++ inlines (one
  called at once, or one passed to a function) still has no frame of its own; its code's line is in the frame of
  the function around it. clang++ names inlined lambdas from its line tables (`main.main.func1(...)`, no offset,
  where Go shows `()` and an offset). When no frame of a stack is a Go one, a program with its symbols and line
  tables shows none (the `?()` addresses are only for a program without them). runtime.Callers gives a frame for
  each inlined Go call too (an address made up for it, which CallersFrames, Caller and FuncForPC decode), so
  CallersFrames and Caller answer as Go's do. `gocpp debug -release -trace` says that the build has line tables but
  no variables. On Windows, MinGW's g++ under `-flto` writes the inlined calls without their functions (no
  `DW_AT_abstract_origin`: PE objects have no early debug information), so an inlined Go call has no frame there,
  but the frame around it has the line of the call (`main.main()` at `main.gp:9`, not a runtime header's line);
  the Windows hello world is 1,222,595 bytes with `-release` and 1,817,570 with `-release -trace` (1.5x).
- The first trace (or runtime.Stack) of a program reads its file and indexes its tables. Measured, the time to the
  first `runtime.Stack` of a JSON/regexp program: 135-157 ms in a debug build (g++, clang++); 230-242 ms (g++, 48 MB of
  `.debug_info`) and 156-163 ms (clang++, 29 MB) for a debug build with `-cflags=-O2 -ldflags=-O2` when it also
  indexed the inlined calls; 47-84 ms for `-release -trace` (5 MB). So only `-release -trace` (`GOCPP_TRACE`) reads
  the inlined calls; an optimized debug build does not.
- **Recursion (user decision, 2026-10-23, accepting the cost).** An endless recursion ends with Go's `fatal error:
  stack overflow` in `-release` too: the functions that call themselves or are in a call cycle of their package carry
  `GP_RECURSIVE` (no tail calls in them: clang `disable_tail_calls`, g++ `optimize("no-optimize-sibling-calls")`) and
  start with `GP_RECURSION_MARK()` (clang: an empty `asm volatile`, so that the forward-progress rule cannot drop the
  recursion), from `internal/lower/recursion.go`; the other functions keep their tail calls. Measured on gobench
  `-release` (59 programs, 7 reps, interleaved, one CPU set): 9 of 118 binaries change code; clang++ `fib` 68.2 ->
  113.6 ms (+67%), clang++ `strings_ops` +5%, g++ `fib` 30.5 -> 1.4 ms, the others within the noise. Not covered: a
  cycle through another package, a func value or an interface method (language.md D7).
- **No identical code folding with g++.** g++'s `-O2` folds functions with the same code into one (`-fipa-icf`), and a
  trace, `runtime.Caller` and `FuncForPC` then name the kept one for the other (two Go functions with one body). An
  optimized g++ build gets `-fno-ipa-icf` at the compiles and at the link (`internal/build` `noICF`), plain
  `-release` too: the names are observable there, by `runtime.Caller` and `FuncForPC` from the symbol table
  (`release_identical_funcs_plain`: g++ named `main.first` for both functions). The narrower ways were tried by
  the review: `__attribute__((no_icf))` on the Go functions did not keep the names apart; the flag on the
  generated C++ only did keep them right (`main.first`/`main.second` under `-release -trace`), but `time_now`
  stayed slow (286 ms), so it was not taken. clang folds
  nothing without the linker's `--icf`, which gocpp never passes. Measured (g++ `-release`, gobench, 59 programs,
  interleaved, one CPU set): geomean time 0.992 (run-to-run noise 0.963), files 2.5% smaller (the folded aliases go;
  at most +2%); `matmul` 147 -> 218 ms (+48%) with the same instructions in its loop, moved by 0x120 bytes (code
  placement, not folding), `chan_pingpong_unbuf` +13% and `time_now` +9% (noisy, run-to-run 486-557 ms).
- **Loop alignment: measured, not taken.** `matmul`'s +48% above is a code-placement lottery that dev has too: its
  inner loop runs at 137-140 ms when it starts on 64 bytes, 150-240 ms elsewhere. `-falign-loops=64` for `-release`
  was measured (gobench, 59 programs, Go as the control in each run, dev / this branch / this branch with the flag in
  turns, two rounds, 7 reps, pinned to CPUs 184-187, load 19-65): geomean against this branch 0.999 (g++) and 1.003
  (clang++), `tree_alloc` +23% (g++) and +35% (clang++) in both rounds, `chan_pingpong_unbuf` +12-15%, `json_doc` +9%;
  `mutex_counter` and `method_value` faster. Not kept: clang's geomean is over 1.0 and one program loses a third.
  With this branch's code `matmul` already sits on a fast placement (1.00x Go with g++, 0.55x with clang++).
- Possible later size optimization for `-trace`: keep only the rows of `.gp` files in `.debug_line` (and the inlined
  calls of Go functions), after the link. A prototype of the line part (job dev.releasetrace, round 1) wrote a `.gp`-only
  DWARF 4 line table of +2.5% to +6% of a `-release` program (wc, a JSON/regexp program; g++ and clang++), which the
  unchanged runtime read; it needs a post-link step in the driver for ELF and PE, so it was not done.
