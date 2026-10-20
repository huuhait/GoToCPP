# ADR-066: The debugger support: gdb's script travels inside every program, and gdb is the full debugger

Status: accepted (2026-10-18). Step 13.2 of [Phase 13](../../plans/2026-10-13-gocpp/phase-13-tooling/README.md).

## Context

A gocpp program has debug information that points at `.gp` lines ([ADR-003](003-readable-cpp20-output.md): `#line`
directives), so `break hello.gp:12` works in any debugger. What the debugger shows is C++: a `gp::slice<long>` is a
struct with an owner, a pointer, a length and a capacity; a `gp::map` is a `std::unordered_map` inside a shared
pointer; an interface is a type descriptor pointer and a shared box; a closure frame is `operator()<>(void)`; a
function is `gpkg_main::fact`. Phase 13's exit criterion asks that scripted gdb and lldb sessions show these values
"nicely" and stop at `.gp` breakpoints, also in a `-trimpath` build.

## Decision

1. **gdb is the debugger with the full support**: pretty printers for every runtime value and for the structs that
   gocpp writes (Go syntax: `[1 2 3] (len 3, cap 3)`, `map["a":1]`, `&{X:7 Y:8} @0x...`, `(main.Point) {X:2 Y:3}` for an
   interface), a frame filter (Go names: `main.f`, `main.(*T).Inc`, `main.main.func1`, `main.Max[...]`), the commands
   `gp goroutines`, `gp bt`, `gp locals`, `gp print`, `gp whatis`, `gp break`, `gp skip-runtime`, and the convenience functions
   `$gp_index`, `$gp_len`, `$gp_deref` and `$gp_blocked` (gdb can't call `operator[]` or `operator*`). It is one Python file,
   `runtime/debug/gocpp-gdb.py`.
2. **The printers read memory and never call code in the program.** They work on a core file and on a program that is
   stopped anywhere, at the price of knowing the layout of the runtime's classes and of libstdc++'s hash table (maps).
   The tests fail when a layout changes. At `-O2` a value that the compiler dropped is `<optimized out>`.
3. **The script travels inside every program**, as the text of a `.debug_gdb_scripts` section (type 4, Python text) that
   `runtime/src/os/linux/debug_scripts.cpp` writes from `gocpp_gdb.inc` (now `runtime/src/os/gocpp_gdb.inc`, shared with Windows by [ADR-116](116-debugging-and-typed-interop-on-windows.md); generated from the `.py` by
   `go test ./runtime -run TestDebugScript -update-debug-script`; the test fails when it is stale). The section has no `A` flag,
   so it is not loaded into memory (about 90 KB in the file; `strip` removes it, and then there are no printers), and a program that moves to another
   machine keeps its printers. gdb runs it only for a safe path: `gocpp debug` adds the program with
   `add-auto-load-safe-path` (by hand, with `source`, when the path has a colon); the guide shows the line for a gdbinit and says what the trust means: auto-loading a program is like running it, so a safe path holds only programs that you built. This rules out a script next to the program (it gets
   lost) and a script installed with gocpp (a program from another machine has none).
4. **Go names come from the debug info and from the `.gp` file.** g++ names every func literal `operator()<>` and no
   compiler says that a method has a pointer receiver, so the filter reads the `.gp` file (a small tokenizer: declarations,
   receivers, literals in source order) through gdb's source lookup (`substitute-path` applies). Without the file the
   frames keep the names that the C++ names and the debug info give.
5. **`gocpp debug [build flags] [-debugger gdb|lldb] [-ex CMD]... [-batch] [-map FROM=TO]... [package | file.gp...] [args]`**
   builds like `gocpp run`, runs the debugger on the program in the work folder (stdin, stdout and stderr are the
   terminal's, SIGINT and SIGQUIT are the debugger's) and exits with its status. `-ex`/`-batch` make scripted sessions;
   `-map` is `set substitute-path` (gdb) or `target.source-map` (lldb) for a `-trimpath` build or a moved source tree.
6. **lldb is best effort**: `runtime/debug/gocpp_lldb.py` (embedded in the gocpp binary; `gocpp debug` writes it to
   the work folder) gives LLDB summaries for strings, slices, arrays, pointers, channels, funcs, interfaces, structs and
   named types, and the commands `gp_print` and `gp_locals`. Maps show their length only; there are no Go names in
   backtraces (LLDB has no frame filters) and no goroutine list. LLDB 21 shows `<incomplete type>` for a runtime class
   the second time `frame variable` shows it, with or without our script.
7. **Tests**: `TestGDBPrinters` (gdb sessions for five programs, golden output, every command a subtest; `-O0`, `-O2`,
   trimmed paths with and without the source, `-asan`), `TestLLDBPrinters`, `TestDebugGDB`/`TestDebugLLDB` (through `gocpp debug`),
   and `runtime/debug/test_gocpp_gdb.py` for the part that needs no gdb (stubbed `gdb` module).
8. **VS Code**: breakpoints in `.gp` files (the extension has contributed them since Phase 11) and a documented
   `launch.json` for `cppdbg` with gdb (`miDebuggerArgs` for the safe path, `set gocpp print-style tree` for the variables
   view) and CodeLLDB; no debug adapter of our own.

9. **The printers never raise.** `fmt` catches every exception, a string length below zero is the empty string, a type
   name is read byte by byte (invalid UTF-8 replaced; escaped in a C locale, where gdb cannot write it), and `gp locals`
   catches each variable's error and goes on, so garbage in a frame that is not yet initialized costs one line, not the
   command. A value is cut at four times gdb's `print elements` in all, not only per list.
10. **`step` is usable by default**: the skip rules of `gp skip-runtime` are set when the script loads (opt out:
    `gp skip-runtime off`, or `GOCPP_GDB_NOSKIP=1`); a rule has no backslash (gdb eats it) and setting them twice changes
    nothing. The frame filter also hides the helper frames of a deferred call and of a goroutine's start
    (`std::apply`, `defer_frame::gp_run`, `call_task::run`, `worker_main`, `start_thread`, `clone3`).
11. **Go spellings where cheap**: `gp print`/`gp whatis` read `x.f`, `*p`, `s[i]` and `len(x)` when gdb's reading fails;
    `gp break main.f`; `gp locals` shows a boxed variable as its Go type, decided by the declaration line in the `.gp`
    file (the C++ type of `x := 20` captured by a closure and of `p := &T{}` is the same `gp::ptr`).

## Consequences

- Every Linux program has the script; a user who reads gdb's `info auto-load` sees it. No change for other platforms
  (the section is written for ELF only; Mach-O's `__DATA,__lldbsummaries` and PE come with Phase 14).
- A change of the runtime's layouts or of libstdc++'s `unordered_map` needs a change of the printers, and the golden
  files show the effect of any change of the text. The unit tests and the goldens are the specification.
- `-trimpath` itself belongs to Phase 06 (6.11/6.12); the tests build with `-ffile-prefix-map` (what it does for the debug
  info) and `-map`/`substitute-path` is the documented way back to the sources.
