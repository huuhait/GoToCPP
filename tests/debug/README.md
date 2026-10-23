# tests/debug

A debug test is a `.gp` program with `// gdb:` comments. It checks that a gocpp program can be
debugged at the `.gp` level: the `#line` directives of the generated C++
([Phase 02](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md) §2.1, §2.8) make
the debug info point at `.gp` lines. `TestDebug` builds each program in this folder with
`gocpp build` (the default build: `-g -O0`, no columns; `GOCPP_FLAGS` doesn't apply), once per
C++ compiler, and checks the binary twice:

- **`lines`** reads the DWARF line table of the binary (with Go's `debug/dwarf`, so it needs no
  tool) and checks that:
  - every address of a function of the program maps to the `.gp` file, to a line of that
    function (line 0, "no line", is allowed: debuggers skip it). So does every address of a
    lambda that gocpp writes into a statement (`fmt.Println(f())` with several results);
  - no other address maps to the `.gp` file, so generated-only code (the `main` wrapper, the
    runtime) never lands on an unrelated `.gp` line;
  - the functions that only gocpp generates (`main`, later the init helpers) map to the
    generated C++ files of the program: `main` is in `pkg_init.cpp` (the driver sets the folder of the
    generated files, `gen/<hash>/gpkg/<import path>/`, as cppgen's root);
  - every line that must have code has an `is_stmt` row, so `break x.gp:N` stops at line N:
    the `func` line, the first line of every statement that does something, and the closing
    brace of a func that can run off its end. Blocks, labels, `const` and `type` declarations,
    `fallthrough`, `for {`, the `switch` line and the `case` lines may have code, but don't have
    to (a switch on a variable compares it at the case lines; a switch on an integer becomes a
    C++ `switch`, whose cases have no code).
- **`gdb`** runs one gdb session in batch mode, made from the `// gdb:` comments, and checks
  what gdb answers. It is skipped, with a message, when gdb is not installed, or when gdb may
  not trace programs (no `ptrace` in some containers and sandboxes): only when its `run` didn't
  stop and gdb said so. With `GOCPP_REQUIRE_GDB=1`, which `make ci` sets, both are failures.

[hello.gp](hello.gp) (line 11 is the `return`, line 16 the `fmt.Println`):

```go
// phase: 02

// The program of the Phase 02 README (§2.1, §2.8): gdb stops at .gp lines,
// prints the locals by their Go names, and bt shows the .gp lines of the
// program's frames.
package main

import "fmt"

func add(a, b int) (int, bool) {
	return a + b, a+b > 10 // gdb: print a = 7; print b = 5; bt gpkg_main::add gpkg_main::main
}

func main() {
	s, big := add(7, 5)
	fmt.Println("hello, gocpp", s, big) // gdb: print s = 12; print big = true; bt gpkg_main::main; next 17
}
```

A `// gdb:` comment ends a line with a statement and makes it a **stop**: gdb stops there the
first time the line runs, then runs the commands of the comment, separated by `;`:

| Command | gdb runs | It passes when |
|---|---|---|
| `print EXPR = VALUE` | `print EXPR` | gdb prints `$N = VALUE`, exactly |
| `print EXPR ~ REGEXP` | `print EXPR` | the value matches the regexp (for values with addresses). The first ` = ` or ` ~ ` ends `EXPR` |
| `bt F1 F2 ...` | `bt` | the program's frames (the ones in `gpkg_main::`, and the lambdas of its func literals) are `F1 F2 ...`, from frame #0 on, each at a `.gp` line of its func or func literal (frame #0 at the stop), and the outermost frame is the generated `main`, at a line of `pkg_init.cpp`. Between two of the program's frames only a deferred call's frames or a func value's may come: a run of the runtime's and the C++ library's (`gp::`, `std::`, `__`) that ends with `gp::defer_frame::gp_run` or a frame of `gp::func`, just inside the next program frame. Write the names as gdb shows them, `gpkg_main::add`, a method as a member of its type's struct: `gpkg_main::Point::Move`, and a func literal by its Go name: `gpkg_main::main.func1` (the first literal of `main`), `gpkg_main::main.func1.1` (the first one in it) |
| `next L1 L2 ...` | `next`, once per line | each `next` stops at the next line of the list, in the `.gp` file, in the func of that line: so `next` never goes into the `.cpp` file or a runtime header for a user statement |

The session is: for each stop, in the order of the lines, `tbreak x.gp:N` (gdb must put it at
line N, in the `.gp` file), `run` the first time and `continue` after that, a check that gdb
stopped at line N, then the commands. When the last `next` of a stop stops at the line of the
next stop, there is no breakpoint and no `continue` for that one. After the last stop,
`continue` must run the program to its end, with exit status 0. So:

- the stops must run in the order of their lines: put the funcs that run first higher up in the
  file, as in [results.gp](results.gp), and don't let a `next` go past the next stop;
- a line that runs more than once (a loop) stops the first time only;
- write the line numbers of `next` as absolute numbers; a test failure says which line gdb
  went to instead.

The lines of a `next` depend a little on the compiler: g++ gives the closing brace of a func
that ends with a `return` a line of its own, clang++ doesn't. So a `next` at the last line of
such a func goes to the brace with g++ and straight back to the caller with clang++. Step out of
a func without results instead (`report` in [loops.gp](loops.gp)).

The file has the phase line (and maybe a `//go:build` line, see [tests/README.md](../README.md))
and no `// Output:` block. A malformed `// gdb:` comment fails the test, even when the test is
skipped by its phase.

## Debugging a gocpp program by hand

Build it (`gocpp build` builds with `-g -O0` in this phase), then start gdb on the binary and
name `.gp` files and lines, and Go names:

```sh
gocpp build -o hello hello.gp
gdb ./hello
(gdb) break hello.gp:16
(gdb) run
(gdb) print s
$1 = 12
(gdb) bt
#0  gpkg_main::main () at /home/me/hello/hello.gp:16
#1  0x000055555555b8d0 in gp::run_main (...) at /home/me/.cache/gocpp/rt/.../src/rt.cpp:519
#2  0x000055555555749e in main (argc=1, argv=0x7fffffff9438) at /tmp/gocpp-build123/hello.gp.cpp:27
(gdb) next
```

lldb works the same way: `lldb ./hello`, `b hello.gp:16`, `run`, `p s`, `bt`, `next`.

What the debugger shows in this phase (the gdb and lldb pretty printers come with Phase 13):

- **Names.** Locals and parameters keep their Go names, and functions are
  `gpkg_<package>::name` (`gpkg_main::add`). Names that gocpp escapes keep their C++ form
  (`new` is `new_`).
- **Shadowed names.** A variable that shadows one of the same name keeps its Go name too, in a
  C++ block of its own that starts at its declaration and ends with its Go block:
  `v := v + i + 1` is `int64_t gp_t0 = v + i + 1; { int64_t v = gp_t0; ... }`. The temporary,
  outside the block, reads the outer `v` (in C++ a new name is in scope already in its own
  initializer). So `print v` shows the variable that Go means there: the outer `v` on the lines
  of the block above the declaration, the inner `v` after the declaration, and the outer one
  again after the block. On the declaration's own line it shows the outer `v` when the value
  reads the name, as here (the line starts with the temporary); a declaration whose value
  doesn't read it (`x := 7 + i`, `err := f()`) has no temporary, and on its own line `print x`
  shows the inner `x`, not set yet, as for any C++ declaration. [loops.gp](loops.gp) checks this
  in the loop's second iteration, and [shadow.gp](shadow.gp) the other places: a line above
  the declaration, an if with an init, a loop body that declares its range variable again,
  and `x, y := 2, x`.
- **Temporaries.** `info locals` also lists gocpp's temporaries (`gp_t0`, the hidden counter
  `gp_i0` of a `for range`), and C++ shows every variable of a scope, also the ones that are
  declared further down and hold garbage until then.
- **Strings.** A `string` is a `gp::string`, shown with its C++ fields:
  `{gp_owner_ = ..., gp_data_ = 0x55555557d510 "go!", gp_len_ = 3}`. gdb prints `gp_data_` up
  to a zero byte, which needn't be the end of the string; `print *s.gp_data_@s.gp_len_` shows
  exactly the bytes.
- **Small integers.** `uint8` and `int8` are C++ `uint8_t` and `int8_t`, which are character
  types, so gdb adds the character: `4 '\004'`. `print/d u8` shows the number alone.
- **Columns.** The debug info has no columns (`gocpp build` passes `-gno-column-info`): the
  compilers only know the columns of the C++ code, and lldb would show them on the `.gp` line
  (`hello.gp:16:26`, with its `^` at the wrong place). So debuggers show lines only.
- **Closures.** A func literal is a C++ lambda whose body keeps its `.gp` lines
  ([closures.gp](closures.gp)): `break` and `next` work in it, and `print` shows its parameters,
  its locals and what it captured by their Go names (a variable that the closure shares with its
  function is boxed: `print *count`; one that only a local closure uses, a literal that is only
  called where it is made ([ADR-114](../../docs/adr/114-escape-analysis-keeps-non-escaping-objects-on-the-stack.md)),
  is captured by reference, and gdb without gocpp's printers shows the reference:
  `(int64_t &) @0x7fffffff9300: 5`), and `next` over a statement that makes a closure goes on to
  the next statement (the C++ after the lambda's closing brace, `);`, is mapped to the
  statement's line, not to the brace's; a local closure `f := func() {` is `auto f = ([...]() {`
  for that, as g++ would give the copies of its captures the brace's line without the `)`). gdb
  names the lambda's frame `operator()<>(int64_t)` (g++)
  or `gpkg_main::main()::$_0::operator()<1>(long)` (clang++); the `<1>` is the literal's number in
  its function, as in Go's `main.main.func1`. Between the lambda and the frame that called the
  func value are two frames of `gp::func` (`gocpp/func.hpp`): `operator()` (or `gp_call`) and
  `gp_call_obj`, which calls the lambda. On both compilers they show up when you step:
  - `step` at a call of a func value (`inc()`) goes through the runtime's lines first:
    `func.hpp`'s `operator()` (three lines: the nil check and the call), then
    `std::__shared_ptr::get` (`shared_ptr_base.h`), then `gp_call_obj` (`func.hpp`), and only then
    to the closure's first line: five stops in the runtime before the closure (the sixth `step` reaches
    it). A `break` on the closure's first line is quicker;
  - `step` at a call that passes a func value (`apply(add, 1)`) goes into `gp::func`'s copy
    constructor (`func.hpp`), then into `std::shared_ptr`'s, before it reaches the callee;
  - `finish` from a closure stops in `gp_call_obj`, then in `operator()`: three `finish`es get
    back to the frame that called the func value;
  - with g++, `next` from the last line of a closure that a func value called stops in
    `gp_call_obj` (`func.hpp`) too; clang++ goes back to the caller.

  Phase 13's `gocpp debug` adds skip rules for the runtime's frames.
- **Generated code.** The `main` wrapper maps to the generated `pkg_init.cpp`, which stays in the cache
  folder (`gen/<hash>`; `gocpp build -work` puts it in the work folder), and the runtime to its files in the gocpp cache.
