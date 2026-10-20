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

The script is inside every program that gocpp links on Linux: a section that is not loaded into
memory (it makes the file about 90 KB bigger, and `strip` removes it with the debug
information, so a stripped program has no printers), so a program that you move to another
machine still has its printers.

**Trust.** Auto-loading a program's script is like running the program: the script is Python, and gdb
runs it as you when it loads the program. That is why gdb asks for a safe path. Add only
folders whose programs you built yourself (the folder of your own `bin/`, not `/home/me` or `/tmp`),
and never `set auto-load safe-path /` outside a throwaway test machine: any program that a safe path
holds can run code in your gdb session just by being debugged. For a program that
someone else built, leave auto-loading off (gdb's default) and load the script from your own gocpp
checkout with `source`; running the program under any debugger still runs its code.

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
  Go name: `main.f`, `main.T.M`, `main.(*T).M`, `main.init` for the package variables' initializer and `main.init.0` for the first `init()`), `gp skip-runtime [on|off]`.
- Go expressions. gdb's own `print` and `break` speak C++: `print c->n` or `print $gp_deref(c).n`
  for a `*T`, `break gpkg_main::main` for `main.main`. `gp print` and `gp whatis` also read `c.n`
  (through a pointer, as Go does), `*c`, `s[i]` (slice, array, string, and a pointer to an array)
  and `len(x)` (`*p` and `len(x)` are always read as Go: gdb would call the runtime's `operator*`
  in your program), and a variable that is a heap box shows as its Go variable, as in `gp locals`: `gp print acc.n`, `gp print len(grid)`, `gp print grid[1][2]`. (Map indexing and
  calls are not supported: `gp print m` shows the map.)
- A variable that a closure captures, or whose address is taken, lives in a heap box in the C++
  (`gp::ptr<int64_t>`); `gp locals` shows the Go variable (`x (int) = 21`), judging by the `.gp`
  line that declares it (`x := 20`, `var err error`, not `p := &T{}`; when the line does not say,
  the C++ value is shown as it is; the same unwrapping applies to a variable named in `gp print`). `info locals` and `print` show the C++ variable
  (`x = &21`), and its temporaries (`gp_df`, `gp_t0`). In a closure, `gp locals` lists the
  captured variables too (under their Go names; the C++ calls them `__count` in g++).
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
a program, use the C/C++ extension (ms-vscode.cpptools) with gdb. `.vscode/tasks.json` builds the
program with debug information (the default build has it):

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
tree style that VS Code's variables view needs:

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
      "miDebuggerArgs": "-iex 'add-auto-load-safe-path ${workspaceFolder}/bin/app'",
      "setupCommands": [
        { "text": "-enable-pretty-printing", "ignoreFailures": true },
        { "text": "set gocpp print-style tree" }
      ]
    }
  ]
}
```

With a program built with `-trimpath`, add `{ "text": "set substitute-path /trimmed/src ${workspaceFolder}" }` to
`setupCommands`. For LLDB use the CodeLLDB extension (vadimcn.vscode-lldb): `"type": "lldb"`,
`"program": "${workspaceFolder}/bin/app"`, `"initCommands": ["command script import /path/to/gocpp/runtime/debug/gocpp_lldb.py"]`.

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
- Windows: the gdb printers load from a PE program too (checked on a real PC with WinLibs' gdb: breakpoints on `.gp` lines, Go names in `bt`, strings, slices and `gp goroutines`);
  stepping, lldb and a VS Code launch template are not tried yet ([gocpp on Windows](windows.md#editing-on-windows)). macOS is not a target; the Mach-O
  section `__DATA,__lldbsummaries` is not written.
