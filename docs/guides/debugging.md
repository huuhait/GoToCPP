# Debugging gocpp programs

A gocpp program is a native program with debug information that points at your `.gp` lines
(the generated C++ has `#line` directives, so `break hello.gp:12` works). On top of that, gocpp
teaches gdb to show its values as Go values: strings, slices, maps, pointers, interfaces,
channels, closures and structs print the way Go's `%+v` prints them, backtraces show Go
function names, and a command lists the goroutines. gdb is the debugger with the full support;
LLDB gets the values (best effort).

```
$ gocpp debug hello.gp
(gdb) break hello.gp:11
(gdb) run
Breakpoint 1, gpkg_main::main () at /home/me/hello/hello.gp:11
11	        fmt.Println("hello, gocpp", s, big)
(gdb) gp locals
s (int) = 12
big (bool) = true
(gdb) bt
#0  main.main () at /home/me/hello/hello.gp:11
```

## gocpp debug

```
gocpp debug [build flags] [debug flags] [package | file.gp...] [arguments...]
```

It builds the program like `gocpp run` does (with debug information, without optimization),
starts gdb on it and removes the work folder when gdb ends. The arguments after the program go to
the program. It has the build flags of `gocpp build` (`-asan` for AddressSanitizer, `-x`, `-work`
to keep the work folder) and these:

| Flag | |
|---|---|
| `-debugger gdb\|lldb` | the debugger; the default is gdb, or lldb when gdb is not installed |
| `-ex COMMAND` | a debugger command to run after the program is loaded; repeatable: `-ex 'break hello.gp:12' -ex run` |
| `-batch` | run the `-ex` commands and exit, with no prompt (for scripts, tests and CI) |
| `-map FROM=TO` | show the sources that the debug information names under `FROM` from `TO` (see "Moved sources") |
| `-print` | print the debugger's command line and don't start it |
| `-vscode` | print the `.vscode/launch.json` for the program and the target (`-os`), and build nothing (see "VS Code") |
| `-x` | also print which debugger was found, and where (`# gdb: C:\msys64\ucrt64\bin\gdb.exe (MSYS2 ucrt64 (C:\msys64\ucrt64\bin))`; from PATH it is `(PATH)`) |

gdb only runs the scripts that a program carries (the `.debug_gdb_scripts` section) from a
"safe path". `gocpp debug` makes the program one (`add-auto-load-safe-path`; if the program's path
has a colon, which a safe path list can't hold, it loads the script by hand), so the printers
are there. Without gocpp debug, tell gdb once, in `~/.config/gdb/gdbinit`:

```
add-auto-load-safe-path /home/me/projects/myapp/bin
```

or load the script by hand, in a program without it or in a core file:

```
(gdb) source /path/to/gocpp/runtime/debug/gocpp-gdb.py
```

If the script itself fails (a Python error printed by gdb, `Python Exception` or a traceback), load the file by
hand as above before reporting it: the copy inside a program has no blank or comment lines, so the line numbers
of its tracebacks are not the file's; `source` gives the file's own line numbers.

The script is inside every program that gocpp links on Linux: a section that is not loaded into
memory (it makes the file about 90 KB bigger, and `strip` removes it with the debug
information, so a program built with `-strip` has no printers), so a program that you move to another
machine still has its printers. A `-release` build has no debug information at all: debug the default build, or build
with `-release -cflags=-g -ldflags=-g` to keep the debug information of an optimized program.
`gocpp debug -strip` is an error (nothing is left for the debugger to show), and `gocpp debug -release` prints one line
saying that the program has no debug information, unless the flags put it back at the compiles (`-cflags` or `-cxxflags`, or `GOCPP_CFLAGS`, `GOCPP_CXXFLAGS`) and at the link
(`-ldflags` or `GOCPP_LDFLAGS`: with `-flto` the link writes the debug information, so a `-g` that only the compiles have
leaves none), and goes on.

**Trust.** Auto-loading a program's script is like running the program: the script is Python, and gdb
runs it as you when it loads the program. That is why gdb asks for a safe path. Add only
folders whose programs you built yourself (the folder of your own `bin/`, not `/home/me` or `/tmp`),
and never `set auto-load safe-path /` outside a throwaway test machine: any program that a safe path
holds can run code in your gdb session just by being debugged. For a program that
someone else built, leave auto-loading off (gdb's default) and load the script from your own gocpp
checkout with `source`; running the program under any debugger still runs its code.

## Debugging on Windows

`gocpp debug` works on a Windows host, for a program built for Windows (a program for another system
cannot run under the debugger of this one: `gocpp debug -os windows` on Linux says so, and `-print` still
shows the command line). It starts the gdb of MinGW-w64 and needs one on this machine: gocpp looks in
PATH, then in the folders where an install puts it (MSYS2's `ucrt64`, `clang64` and `mingw64`, Chocolatey,
Scoop, WinGet's WinLibs package, `C:\mingw64`, `C:\w64devkit` and the like), then in the pinned llvm-mingw
toolchain if gocpp has downloaded it. The folders are the ones the compiler search uses, including some a user can create
(`C:\mingw64`, `C:\w64devkit`, `C:\winlibs`): gocpp runs what it finds there as you, as it does with the compiler. It looks before it builds anything, so a missing debugger
costs no build. `-x` says which one it started. When there is none, the message says what to install:

```
gocpp debug: gdb was not found in PATH, in the folders of MSYS2, WinGet, Scoop or Chocolatey (install MinGW-w64
with gdb: winget install BrechtSanders.WinLibs.POSIX.UCRT, or MSYS2: pacman -S mingw-w64-ucrt-x86_64-gdb;
-debugger lldb uses the lldb of llvm-mingw)
```

WinLibs and MSYS2 have gdb; llvm-mingw has lldb and no gdb. gdb needs Python for the printers (WinLibs' gdb has it);
without Python gdb still debugs the program, with C++ names and without printers.

- **The script is in the program.** A program for Windows carries the same `.debug_gdb_scripts` section as one for
  Linux (a PE file holds it as a section with a long name; gdb reads it from the symbol table), so
  `gp locals`, `gp goroutines` and the printers are there. gdb runs it for a "safe path", which on Windows is
  the program's path with forward slashes: `gocpp debug` passes `-iex 'add-auto-load-safe-path
  C:/Users/me/my app/hello.exe'` (a list of safe paths is separated by `;` there, so the colon of a
  drive letter is no problem, and a backslash would be eaten by gdb's command parser). Spaces in the
  folders of the sources, the program and `TEMP` are fine.
- **Paths.** Breakpoints take the `.gp` file name, as on Linux (`break hello.gp:12`). Backtraces show
  `.gp` lines (the `#line` directives of the generated code), with the drive letter and forward slashes:
  `main.main () at C:/Users/me/my app/hello.gp:12`. `gocpp debug` writes every path it hands to gdb with forward
  slashes; type them that way in your own commands, because gdb takes a backslash as an escape.
- **A stack overflow** stops at the `.gp` line of the recursion: gdb reports `SIGSEGV` (the guard page), and
  `bt 5` shows `main.down (n=...) at deep.gp:6`. Do not run `bt` or `gp bt` without a number on a
  stack that is a hundred thousand frames deep: gdb walks every one of them, slowly. `gp bt 20` and `bt -20`
  are the ends.
- **A Go panic** (an index out of range, a nil dereference, `panic(...)`) is a C++ exception inside the
  program. `catch throw` stops at the point where it is thrown, and `bt` shows the Go function that caused it
  with its `.gp` line (`main.at (...) at boom.gp:6`) and its callers:

  ```
  (gdb) catch throw
  (gdb) run
  (gdb) bt 40
  ```

  Without the catchpoint a panic that nothing recovers prints Go's report (`panic: runtime error: index out
  of range [7] with length 3`, with the `.gp` line) and the program exits with status 2, as it does on Linux.
- **Names.** On Windows `long` has 32 bits, so `int` and `int64` are both `long long` and `uint`, `uint64`
  and `uintptr` are `unsigned long long`; the printers know the data model and show `int`, `int64`, as on
  Linux. A C type from `<windows.h>` such as `DWORD` is an `unsigned long`.
- **Goroutines.** The goroutine of a thread is read from the runtime's TLS slot in the thread's TEB (the Windows
  runtime keeps it there instead of in a `thread_local`), so `gp goroutines`, `info goroutines` and the
  frames of a goroutine's thread work as on Linux.

## What gdb shows

```
(gdb) print ints                    [1 2 3] (len 3, cap 3)
(gdb) print strs                    ["a" "bb"] (len 2, cap 2)
(gdb) print m                       map["a":1 "b":2] (len 2)
(gdb) print pp                      &{X:7 Y:8} @0x5555556310b0
(gdb) print out                     {Inner:{A:-1 B:2} Name:"out" new:3 class:4.5 Ptr:0x5555556310b0 Any:5}
(gdb) print sh                      (main.Point) {X:2 Y:3}          an interface: its dynamic type
(gdb) print err                     (*errors.errorString) &{s:"boom"} @0x5555556320b0
(gdb) print ch                      chan(0x5555556301d0 len 2 cap 3)
(gdb) print fn                      func(0x55555556f9e6 main.add)
```

- Maps print sorted by key when they are small (Go's `fmt` does, too); a nil map is
  `map[] (nil)`, a nil slice `[] (nil)`. Long strings and slices stop after gdb's `print elements`
  (200); `set print elements 0` shows all.
- A struct prints its Go field names (`new`, not `new_`); a named type prints its value; a
  generic instance prints like any struct. A cycle in pointers stops at the second level: only the
  top of a value shows the address of a pointer.
- Strings are quoted the way Go quotes them. With a UTF-8 locale gdb shows `héllo`; in a C
  locale it escapes (`héllo`).
- Values are read from memory: the printers never run code in the program, so they work on a
  core file too (`gdb prog core`). gdb can't call `operator[]` or `operator*` of the runtime's
  classes, so there are convenience functions: `$gp_index(ints, 1)`, `$gp_len(m)`,
  `$gp_deref(p)`. `$gp_blocked()` is the number of goroutines that wait for a channel or a select
  (a breakpoint condition such as `$gp_blocked() == 3` waits for them).
- The commands: `gp locals` (the locals of the frame with their Go types and names), `gp print
  EXPR` (an expression with its Go type), `gp whatis EXPR`, `gp bt` (a backtrace with Go names,
  also if you set `gocpp names cpp`), `gp break main.main` (a breakpoint on a Go function by its
  Go name: `main.f`, `main.T.M`, `main.(*T).M`, `main.init` for the package variables' initializer and `main.init.0` for the first `init()`), `gp catch panic` (see "Where a panic starts"), `gp skip-runtime [on|off]`.
- Go expressions. gdb's own `print` and `break` speak C++: `print c->n` or `print $gp_deref(c).n`
  for a `*T`, `break gpkg_main::main` for `main.main`. `gp print` and `gp whatis` also read `c.n`
  (through a pointer, as Go does), `*c`, `s[i]` (slice, array, string, and a pointer to an array)
  and `len(x)` (`*p` and `len(x)` are always read as Go: gdb would call the runtime's `operator*`
  in your program), and a variable that is a heap box shows as its Go variable, as in `gp locals`: `gp print acc.n`, `gp print len(grid)`, `gp print grid[1][2]`. (Map indexing and
  calls are not supported: `gp print m` shows the map.)
- A variable whose address escapes, or that a closure captures and changes while the closure can
  outlive the call, lives in a heap box in the C++ (`gp::ptr<int64_t>`); `gp locals` shows the Go
  variable (`x (int) = 21`), judging by the `.gp` line that declares it (`x := 20`, `var err error`,
  not `p := &T{}`; when the line does not say, the C++ value is shown as it is; the same unwrapping
  applies to a variable named in `gp print`). `info locals` and `print` show the C++ variable
  (`x = &21`), and its temporaries (`gp_df`, `gp_t0`). A variable whose address is only used inside
  the function (the escape analysis, [ADR-114](../adr/114-escape-analysis-keeps-non-escaping-objects-on-the-stack.md))
  is not boxed: it is a plain C++ variable on the stack, and a pointer to it (`p := &x`) is a
  `gp::ptr` that owns nothing; so is the object of a `p := &T{}` that does not escape (a
  `gp::local_ptr<T>`, which holds the object itself). `gocpp build -gp:m` says which variables are
  which. In a closure, `gp locals` lists the
  captured variables too (under their Go names; the C++ calls them `__count` in g++; a variable that
  the closure captures by reference is read through the reference).
- A func literal that is only called in its function (the escape analysis keeps it local) is a plain C++
  lambda on the stack, not a `gp::func`. `print`, `info locals` and `gp locals` show it as a func value,
  `inc (func()) = func(0x7fffffffe2a8 closure in main.main)`: the address is the closure object's, and
  `main.main` is the function whose frame holds it (the selected frame: `up` first to see the closure of a
  caller). The Go type is read from the literal on the line that declares the variable (`inc := func() {`), so
  gdb needs the `.gp` file, as for the names in "Moved sources"; without it the type is `func(...)`. A local
  closure shows as a func value only when the selected frame is in a `.gp` file: from a runtime frame (after
  `gp catch panic`, or `catch throw`, stops in `__cxa_throw`) it is the compiler's raw struct, so `up` to your
  function first. A closure
  that is stored, returned or passed on is a `gp::func`, and prints as `func(0x55555556f9e6 closure in main.f)`
  with its address in the code.
- Bad memory does not stop a command. A frame stopped before its locals are declared holds
  whatever the stack held, a string may have a negative length, a type descriptor may be
  damaged: the value prints as `<unreadable: ...>` or as much as can be read, and `gp locals`
  still prints the other variables. A long nested value is cut: each list stops at gdb's
  `print elements` and a whole value at four times that (`[...]` for the lists that are left out).
- `step` goes over the runtime and the C++ library, into your functions and closures (the runtime's
  own folder is named in the rules, so your C++ files in any other `rt/x/src` folder are not skipped): the
  skip rules of `gp skip-runtime` are on when the script loads (`GOCPP_GDB_NOSKIP=1` in the
  environment of gdb turns that off, and `gp skip-runtime off` in a session removes them to
  step into `gp::string` or `std::shared_ptr`). Calls through a func value still show the two
  frames of `gp::func`: `step` into one and gdb stops in the closure. The stop line, `frame`,
  `up` and `finish` print the compiler's names (`gpkg_main::Acc::Add`, `operator()<>`);
  `bt` and `gp bt` print the Go names.
- `set gocpp print-style tree` shows a short summary and children: the style an IDE needs for its
  variable tree (VS Code uses it); `set gocpp names cpp` goes back to C++ names in backtraces.

## Where a panic starts

A Go panic (an index out of range, a nil dereference, `panic(...)`) is a C++ exception, and `catch throw`
stops at the point where it is thrown: `bt` shows the Go function that caused it with its `.gp` line, and its
callers. Two things make a plain `catch throw` miss or repeat:

- The `panic` statement of a function that has deferred calls is not thrown: the function handles it in its own
  frame (a `goto` to its exit, [ADR-085](../adr/085-panic-stacks-up-to-the-handler-and-no-throw-for-a-local-panic.md)),
  so `catch throw` does not stop there when a `recover()` takes it.
- A panic that nothing recovers is thrown again when the function's deferred calls have run, so `catch throw` stops a
  second time, at the closing brace of the function.

`gp catch panic` handles both: it sets the catchpoint with a condition that skips that second throw, and a breakpoint
on the runtime function that takes a panic statement (`gp::defer_frame::gp_panic`). The debugger stops once for each
panic, and the Go function that panicked is the selected frame (the stop itself is in the runtime: `bt` lists the
runtime's frames above it, `down` goes back to them). The frame line that gdb prints at the stop uses the C++ name
(`gpkg_main::direct`), as `frame`, `up` and `finish` do; `bt` and `gp bt` show the Go name (`main.direct`).
`runtime.Goexit` is a throw too, and stops once, also in a goroutine with deferred calls. Running the
command again replaces the first one's stops (no duplicates); `delete` removes them.

A program without deferred calls has no `gp_panic` (the linker drops it): the command says so, and the throws are
then all there is to stop at, with the Go function selected as before.

Your own breakpoints are not touched. A `catch throw` or a breakpoint on a runtime function (`break gp::panic_constant`)
that you set yourself stops in its own frame, also while `gp catch panic` is set, and also at the second throw of an
unrecovered panic (which `gp catch panic` skips): only a stop that `gp catch panic` made selects the Go function. (An `ignore` count on its breakpoints changes nothing there.)

```
(gdb) gp catch panic
Catchpoint 1 (throw)
Breakpoint 2 at 0x...
(gdb) run
Catchpoint 1 (exception thrown), 0x... in __cxa_throw () from .../libstdc++.so.6
#2  0x... in gpkg_main::direct () at panics.gp:17                a throw: panic("direct") in a function without defers
17	        panic("direct")
(gdb) continue
Breakpoint 2, gp::defer_frame::gp_panic (...)
#1  0x... in gpkg_main::withDefer () at panics.gp:22             the panic statement of a function with defers
22	        panic("with defer")
(gdb) continue                                                   (an unrecovered panic: one stop at gp_panic, not two)
```

## Go names and goroutines

A backtrace shows `main.f`, `main.T.Get`, `main.(*T).Inc`, `main.main.func1` (a closure),
`main.Max[...]` (an instance of a generic function), `main.init` and `main.init.func1` (a
package variable's initializer); the helper frames of the runtime that Go does not have (the call
of a `gp::func`) are left out. The names come from the debug information and from the `.gp` file
(which says what a method's receiver is and which literal a line belongs to; gdb needs the file,
see "Moved sources"). The frames between two Go frames (the runtime's call of a `gp::func`, a
deferred call, a goroutine's start, `std::apply`) and the frames under a goroutine's first Go frame
(`worker_main`, `start_thread`, `clone3`) are left out; gdb keeps its own frame numbers, so
the list can go `#0`, `#7`, `#8`. `gp bt` and `set gocpp names cpp` / `bt` show the same
frames, with the C++ names for the second.

Every goroutine is a thread, and `gp goroutines` (or `info goroutines`) lists them:

```
(gdb) gp goroutines
*    1  running                      thread 1   main.poll (goroutines.gp:59)
     2  waiting [chan receive]       thread 2   main.worker (goroutines.gp:11)
     5  waiting [chan send]          thread 5   main.sender (goroutines.gp:18)
     6  waiting [select]             thread 6   main.main.func2 (goroutines.gp:37)
     7  waiting [select (no cases)]  thread 7   main.main.func3 (goroutines.gp:44)
(gdb) thread 5
(gdb) bt
```

The state is the runtime's own (the wait reason of the goroutine record); `*` marks the selected
thread; `gp goroutines -a` also lists the threads that run no goroutine (the runtime's idle workers).

## Optimized builds

`GOCPP_CXXFLAGS=-O2` (or a build flag, when the build has one) optimizes; the program is still
debuggable. A local that the compiler dropped prints `<optimized out>`, a function that was inlined
has no frame of its own (gdb shows it inside its caller's frame; a breakpoint on its line has
several locations), and a closure that the compiler inlined has no Go name. Backtraces, the goroutine list and
the values that are still in memory keep working. Debug at `-O0` when you need every variable.

AddressSanitizer builds (`-asan`) work the same way. LeakSanitizer can't run under a debugger
(ptrace): `set environment ASAN_OPTIONS=detect_leaks=0`.

## Moved sources (-trimpath)

The debug information names your `.gp` files with the paths of the build machine. If the files
are somewhere else (a build with `-trimpath`, a program built in a container, a core file from
another machine), tell the debugger where they are:

```
$ gocpp debug -map /trimmed/src=/home/me/project hello.gp       (gdb: set substitute-path)
(gdb) set substitute-path /trimmed/src /home/me/project
(lldb) settings append target.source-map /trimmed/src /home/me/project
```

`break hello.gp:12` works without it (gdb and lldb match the file name), and values print without
it; only the source lines and the Go names of closures and methods need the file (gdb reads
the same text, through its substitute-path). Without the file, a closure frame keeps its C++
name (`operator()<>`).

A frame in the standard library names `$GOCPP/stdlib/errors/errors.gp`: `$GOCPP` is gocpp's own
folder, and the standard library is inside the `gocpp` program, so gdb can't find the file. In a
checkout of gocpp, `-map '$GOCPP=/path/to/gocpp'` (or `set substitute-path '$GOCPP' /path/to/gocpp`)
shows the sources. Paths with spaces are fine: `gocpp debug` quotes them for the debugger.

## LLDB

```
$ gocpp debug -debugger lldb hello.gp
(lldb) breakpoint set -f hello.gp -l 11
(lldb) run
(lldb) gp_locals
(lldb) gp_print ints
```

`gocpp debug` loads `runtime/debug/gocpp_lldb.py` (which it writes to the work folder; by hand:
`command script import runtime/debug/gocpp_lldb.py`). It gives LLDB the summaries of the same
values as the gdb printers: `gp::string`, `gp::slice`, `gp::array`, `gp::ptr`, `gp::chan`, `gp::func`,
interfaces (the dynamic type, and the value of a scalar, a string and a struct), structs and named types
(`frame variable` and `gp_locals` show them). The gaps: a map shows its length only, not its entries; there are no Go
names in backtraces and no goroutine list (LLDB has no frame filters; use gdb for them); and
LLDB 21 shows `<incomplete type>` for a `gp::slice` or another runtime class the second time
`frame variable` shows it (also without gocpp's script: it is how LLDB loads these C++ templates),
while `gp_locals` and `gp_print` always work.

## VS Code

Install the gocpp extension (`editors/vscode`): it lets you set breakpoints in `.gp` files. To start
a program, use the C/C++ extension (ms-vscode.cpptools) with gdb, or lldb-dap. `gocpp debug -vscode`
prints the `.vscode/launch.json` for the program (the name of the `.gp` file or the package, else the
folder) and the target (`-os`), so it never goes out of date with the flags:

```
gocpp debug -vscode hello.gp > .vscode/launch.json            # gdb, for this system
gocpp debug -vscode -os windows hello.gp > .vscode/launch.json
gocpp debug -vscode -debugger lldb hello.gp > .vscode/launch.json
```

`.vscode/tasks.json` builds the program with debug information (the default build has it); for Windows
the program is `app.exe`:

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "gocpp: build",
      "type": "shell",
      "command": "gocpp",
      "args": ["build", "-o", "${workspaceFolder}/bin/app", "."],
      "group": "build",
      "problemMatcher": []
    }
  ]
}
```

and `.vscode/launch.json` starts it under gdb, with gocpp's printers (the auto-load line) and the
tree style that VS Code's variables view needs. For Linux (`gocpp debug -vscode -os linux app.gp`):

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "gocpp: debug",
      "type": "cppdbg",
      "request": "launch",
      "program": "${workspaceFolder}/bin/app",
      "args": [],
      "cwd": "${workspaceFolder}",
      "preLaunchTask": "gocpp: build",
      "MIMode": "gdb",
      "miDebuggerPath": "gdb",
      "miDebuggerArgs": "-iex \"add-auto-load-safe-path ${workspaceFolder}/bin/app\"",
      "setupCommands": [
        {
          "text": "-enable-pretty-printing",
          "ignoreFailures": true
        },
        {
          "text": "set gocpp print-style tree"
        }
      ],
      "externalConsole": false
    }
  ]
}
```

For Windows (`gocpp debug -vscode -os windows app.gp`; `miDebuggerPath` is the name `gdb.exe` when no
gdb was found on this machine, and the full path of the gdb that gocpp found when there is one, such as
`C:/msys64/ucrt64/bin/gdb.exe`, because VS Code usually does not have MinGW's folder in its PATH):

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "gocpp: debug",
      "type": "cppdbg",
      "request": "launch",
      "program": "${workspaceFolder}/bin/app.exe",
      "args": [],
      "cwd": "${workspaceFolder}",
      "preLaunchTask": "gocpp: build",
      "MIMode": "gdb",
      "miDebuggerPath": "gdb.exe",
      "miDebuggerArgs": "-iex \"add-auto-load-safe-path ${workspaceFolder}/bin/app.exe\"",
      "setupCommands": [
        {
          "text": "-enable-pretty-printing",
          "ignoreFailures": true
        },
        {
          "text": "set gocpp print-style tree"
        }
      ],
      "externalConsole": false
    }
  ]
}
```

The safe path in `miDebuggerArgs` is the program itself, with forward slashes and in double quotes
because a workspace folder may have a space in its name. `externalConsole` is `false`: the program's output is
in VS Code's debug console. With `-map FROM=TO` the file gets a `sourceFileMap` (cppdbg) or a `sourceMap`
(lldb-dap) for sources that the debug information names under another folder; for a program built with
`-trimpath`, add `{ "text": "set substitute-path /trimmed/src ${workspaceFolder}" }` to `setupCommands`.

For LLDB, `-debugger lldb` prints a configuration for the lldb-dap extension (the `lldb-dap` of LLVM 18 or
later, or the "LLDB DAP" extension of the LLVM project) and writes `.vscode/gocpp_lldb.py`, the summaries
that its `initCommands` load:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "gocpp: debug (lldb)",
      "type": "lldb-dap",
      "request": "launch",
      "program": "${workspaceFolder}/bin/app",
      "args": [],
      "cwd": "${workspaceFolder}",
      "preLaunchTask": "gocpp: build",
      "initCommands": [
        "command script import \"${workspaceFolder}/.vscode/gocpp_lldb.py\""
      ]
    }
  ]
}
```

Not tried: nobody has started these files in VS Code yet (the tests check that they are valid JSON with the
fields above, and that the gdb command line of the Windows launch is the one that was run by hand on a real
PC, see "Limits"). If a field is wrong for your version of the C/C++ extension, `gocpp debug -print` shows the command line
that gocpp itself uses, which is known to work.

## Other editors and terminals

Any debugger front end that speaks gdb's MI protocol (Emacs gud, CLion, Neovim's nvim-dap with
gdb) works with the same two lines: `add-auto-load-safe-path` for the program, and `set gocpp
print-style tree` if the front end shows a variable tree. In a terminal, `gdb -tui` shows the `.gp`
source.

## Limits

- Maps print their entries only with gdb (they are read from libstdc++'s hash table: the
  printer needs libstdc++'s debug information, which g++ and clang++ with libstdc++ emit).
  A runtime that changes the layout of a class has to change the printers with it: the tests
  (`tests/gdb`, `runtime/debug/test_gocpp_gdb.py`) fail until it does.
- `int` and `int64` are the same C++ type, so a variable of type `int64` shows as `int` in
  a local list, and `uint64` as `uint` (an interface's dynamic type is exact: it comes from the type
  descriptor). A number that gdb itself makes (`gp whatis 1+2`) has C types (`int32`).
- A map prints in Go's order of keys only when it fits `print elements` (numbers by value,
  other keys by their text); a bigger map prints in the table's order.
- Windows: `gocpp debug` with gdb is tried on a real PC (WinLibs' gdb 17.2): the printers load from the
  program's own section, breakpoints, `gp locals`, `gp goroutines`, `catch throw` and a stack overflow work, with
  `.gp` lines (see "Debugging on Windows"). Not tried: lldb and lldb-dap on Windows (no PC with one), a
  VS Code launch on Windows, stepping with `next` and `step` through a long session, and debugging a program
  that runs as a service or in another session. macOS is not a target; the Mach-O
  section `__DATA,__lldbsummaries` is not written.
