# tests/gdb

The tests of gocpp's debugger support ([docs/guides/debugging.md](../../docs/guides/debugging.md)):
`TestGDBPrinters` (gdb and `runtime/debug/gocpp-gdb.py`) and `TestLLDBPrinters` (lldb and
`runtime/debug/gocpp_lldb.py`). Both need the debugger, and a system that lets it trace programs;
without them the tests are skipped, and with `GOCPP_REQUIRE_GDB=1` (`make ci` sets it) a missing gdb
or a gdb that may not trace is a failure. (lldb is optional.)

A program is `NAME.gp`, a Go program with its `// phase: NN` line and `// @tag` comments at the end of
the lines where a session stops. A session is a file with one debugger command in each line:

```
# a comment
break values.gp:@show        @show is the line of the "// @show" comment
run
gp locals
#filter ^(ints|m1) =         only the lines of the output of the last command that match are compared
#want (?m)^ints \(.*\)$      the output of the last command must match (a #want line is a Go regexp)
```

`TestGDBPrinters` builds the program with each compiler (`g++`, `clang++`) in every variant, runs one gdb
session on the binary (`gdb -batch`, no gdbinit files, auto-loading of the program's script allowed) and checks
the output of each command in its own subtest. The text is normalized first: no addresses (`0xADDR`),
folders (only the file name stays), thread and process numbers, and for a stop only the place:
`Breakpoint 1, at values.gp:129` (the C++ name and the arguments of the function are the compiler's).

| Variant | Build | Session | Checks |
|---|---|---|---|
| `O0` | the default, `-g -O0` | `NAME.gdb` | the golden file `NAME.golden` (every command's output; the same for both compilers) |
| `O2` | `GOCPP_CXXFLAGS=-O2` | `NAME.O2.gdb` | `#want` lines: what must still work when values are optimized out |
| `trimpath` | `-ffile-prefix-map=DIR=/trim`: the debug info names `/trim/values.gp`; gdb gets `set substitute-path` | `NAME.gdb` | the golden file (`values`, `frames`) |
| `nosource` | like `trimpath` without the substitute-path: the sources can't be read | `NAME.nosrc.gdb` | `#want` lines: closures and methods keep the names of the debug info, nothing fails |
| `asan` | `gocpp build -asan` (g++) | `NAME.gdb` | the golden file (`values`, `chans`) |

`go test ./tests -run TestGDBPrinters -update-gdb` rewrites the golden files from g++'s `-O0` run; read the
diff, it is the specification. `TestGDBPrinters/values/g++/O0/05_gp_locals` is the subtest of command 5
of values.gdb.

| Program | What it shows |
|---|---|
| `values.gp` | every kind of value: integers, floats, complex numbers, strings (empty, escapes, UTF-8, long), nil and empty slices, slices of slices and of structs, arrays, maps of several key and value types, pointers, linked lists and a cycle, `any`, `error` and interfaces with a struct and a pointer inside, func values and closures, named types, nested and anonymous structs with the names of C++ keywords (`new`, `class`), `set gocpp print-style tree` |
| `frames.gp` | Go names in backtraces: methods (value and pointer receiver), closures, nested closures, a closure of a package variable, a deferred call, recursion; `set gocpp names cpp` |
| `generics.gp` | the names of generic instances (`main.Max[...]`) and the values that they handle |
| `robust.gp` | bad memory and the daily commands: a string with a negative length and an interface with a damaged descriptor (`set var`) still print and `gp locals` goes on; boxed variables (`count (int)`, also in a closure); Go expressions in `gp print` (`acc.n`, `*acc`, `len(grid)`, `grid[1][2]`); a 300x300 `[][]int` stays small; float map keys sort by value; `$gp_blocked()` without goroutines is 0; the skip rules (on at load, once, `off`); `step` never stops in the C++ library; a deferred call's `bt` has no helper frames |
| `chans.gp` | channels: nil, buffered, unbuffered, closed, in an `any`, a slice and a map |
| `goroutines.gp` | `gp goroutines`: goroutines blocked in a receive, a send, a select and a `select {}`; the breakpoint (`$gp_blocked() == 6`) waits until all six are blocked, so the list never depends on timing; a failing condition ends the program after 40 s, so a broken `$gp_blocked` fails fast; the backtrace of a goroutine thread ends at its Go function |
| `borrowed.gp` | borrowed pointers (`gp::bptr`, ADR-086): a pointer receiver and a pointer parameter that the function only uses print like owning pointers (`borrowed.lldb` only: lldb session) |

The runtime's frames are filtered out of the golden files (`#filter`): their line numbers change with
the runtime.

The part of `gocpp-gdb.py` that needs no gdb (names, the Go text of strings and numbers, the reader of
`.gp` sources) has unit tests of its own: `runtime/debug/test_gocpp_gdb.py`, run by
`go test ./runtime -run TestDebugScriptUnits`.
