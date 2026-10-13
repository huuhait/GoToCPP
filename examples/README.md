# Target programs

These are the 19 target programs of the plan
([Phase 00 §2.4](../plans/2026-10-13-gocpp/phase-00-foundations/README.md#24-target-programs-acceptance-examples)).
Each one is a small, real program, and each names the phase that must make it pass: together
they are the "definition of done" for the project. Until its phase is reached, the test harness
skips a program.

| Example | Phase | Shows | `output.txt` comes from |
|---|---|---|---|
| [hello](hello/main.gp) | 02 | printing one line (milestone M1) | `go run` |
| [fizzbuzz](fizzbuzz/main.gp) | 02 | loops (3-clause, condition only, `range` over an int), `if`, tagless and string `switch`, multiple results | `go run` |
| [primes](primes/main.gp) | 02 | trial division with loops only, `continue` with a label, `break`, `switch` | `go run` |
| [structs](structs/main.gp) | 03 | structs, value and pointer receivers, pointers to structs, struct copies and `==`, method values, a named `float64` type with methods | `go run` |
| [wordfreq-core](wordfreq-core/main.gp) | 03 | slices, maps, strings and UTF-8 `range`, `[]rune`, `copy`, `delete`, sorting by hand (no `strings` or `sort` yet) | `go run` |
| [closures](closures/main.gp) | 03 | counters, func values in slices and maps, variadic funcs, Go 1.22 loop variables, `defer` order, arguments evaluated at `defer`, a defer that changes a named result | `go run` |
| [cpp-std](cpp-std/main.gp) | 04 | `std.vector`, `std.map`, `std.string`, `<algorithm>`, `std::filesystem` through a renamed namespace (`fs`); C++ rules for C++ values | hand-written (interop) |
| [sqlite](sqlite/main.gp) | 04 | the sqlite3 C API: raw C pointers, macros, `C.CString`/`C.free`, `C.cstr`, `C.GoString`, callbacks from C, a `void*` argument | hand-written (interop) |
| [raylib](raylib/main.gp) | 04 | a window and a game loop with the raylib C library; smoke mode for CI | hand-written (interop) |
| [mixed-sources](mixed-sources/main.gp) | 04 | one package of two `.gp` files, a `.c` file with its `.h` (put in `extern "C"`), and a `.cpp` file with its `.hpp` (a header that is also a namespace) | hand-written (interop) |
| [shapes](shapes/main.gp) | 05 | interfaces, type switches and assertions, interface embedding, struct embedding with promoted fields and methods, `error` values (`errors.Is`/`As`, `%w`), the nil-interface rule, `recover` | `go run` |
| [multi-pkg](multi-pkg/main.gp) | 06 | an app and 3 packages in one module (`gocpp.mod`), one of them with C sources; Go 1.21's init order across packages; embedding and interfaces across packages | hand-written (interop) |
| [generics](generics/main.gp) | 07 | `Map`/`Filter`/`Reduce`, `Stack[T]`, constraints with type sets, `int8` wrapping in generic code, `iter.Seq`/`iter.Seq2`, `cmp`, `maps`, `slices`, a generic type alias, `%T` of instances | `go run` |
| [workers](workers/main.gp) | 08 | goroutines, buffered and unbuffered channels, a pipeline, `select` with a timeout and with `default`, nil channels, `sync.WaitGroup` (also `Go`), `sync.Mutex`, `sync/atomic`; the output is the same on every run | `go run` |
| [wc](wc/main.gp) | 09 | `os`, `bufio`, `strings`, `flag`, `io`, `unicode/utf8`; reads files and standard input; tests in [main_test.gp](wc/main_test.gp) | `go run` (and `go test` passes) |
| [grep-lite](grep-lite/main.gp) | 09 | `os`, `bufio`, `strings`, `flag`, `regexp`; exit status 0, 1 or 2 like grep; tests in [main_test.gp](grep-lite/main_test.gp) | `go run` (and `go test` passes) |
| [must-errors](must-errors/main.gp) | 10 | must functions: `func loadConfig!()` with a declared error, `errors.Wrapf` and `defer f.Close()`, and `func port!()`, which gets its error result; `F!(...)` calls (one and two results, and in a function without an error result); `func main!()` | twin |
| [macros](macros/main.gp) | 10 | `macro` declarations; `#Assert` (with a `lazy` message) and `#Dbg` inside code; `#Trace` and `#Locked` (start style) and `#Recover` and `#InThread` (wrap style) on top of funcs; `#Derive(String)` on an enum and `#Derive(JSON)` on a struct | twin |
| [win32](win32/main.gp) | 14 | a Windows program through the `<windows.h>` header package: `MessageBoxW`, a window class, a window procedure and a message loop; smoke mode for CI | hand-written (interop) |

## The files of an example

An example is a folder with these files:

| File | What it is |
|---|---|
| `main.gp` | the program. More `.gp` files, packages in subfolders, a `gocpp.mod`, C/C++ sources and headers, and `testdata/` sit next to it when the program needs them |
| `output.txt` | the expected standard output |
| `args.txt` | optional: the arguments, on one line, split on spaces (no quoting) |
| `stdin.txt` | optional: the standard input; without it, standard input is empty |
| `env.txt` | optional: `KEY=VALUE` lines added to the environment (the GUI examples set `GOCPP_SMOKE=1`) |
| `twin/` | for gocpp-only syntax: the program expanded by hand into valid Go, a module of its own (see below) |

`main.gp` starts with comment lines that the harness reads, before the package clause
([Phase 00 §2.3](../plans/2026-10-13-gocpp/phase-00-foundations/README.md#23-test-harness-design)):

| Line | Meaning |
|---|---|
| `// phase: NN` | always the first line: the phase that must make the program pass (two digits); until then the program is skipped |
| `// pkg-config: raylib` | the program needs these libraries: skipped with a message when `pkg-config --exists` fails |
| `// display: required` | the program opens a real window even in smoke mode: skipped without a display (neither `DISPLAY` nor `WAYLAND_DISPLAY` set); `xvfb-run -a` gives it one (with `XDG_SESSION_TYPE=x11`, see below) |
| `// ExitCode: N` | the expected exit status (0 when the line is missing) |
| `//go:build windows` | a normal Go build constraint, honored: [win32](win32/main.gp) runs only for Windows targets (`gocpp build -os windows`, under Wine from Phase 14) |

Directives for the compiler (`//gocpp:pkg-config raylib`) are separate: they stay at the end of
the package's doc comment, where gofmt puts them.

**How a program runs.** The harness runs `gocpp run . <args>` inside the example's folder, with
the arguments from `args.txt`, the environment from `env.txt` and the input from `stdin.txt`.
So relative paths work, in `args.txt` (`testdata/poem.txt`) and in the code (`app.conf` in
must-errors). Standard output must equal `output.txt` and the exit status must match
`// ExitCode:`; standard error is not compared. Every program prints the same output on every
run: no map printed unsorted, no times, no output that depends on which goroutine runs first.
The files here keep their LF line endings in every checkout ([.gitattributes](.gitattributes)),
because they are compared byte for byte.

**GUI programs** ([raylib](raylib/main.gp), [win32](win32/main.gp)) have a smoke mode: when the
environment variable `GOCPP_SMOKE` is set (their `env.txt` sets it), they open their window,
close it right away and print a few lines. It is an environment variable and not an argument
because a Phase 04 program can't read its arguments yet (`os.Args` comes with Phase 09).
- raylib needs a display even for its hidden window. Run it under `xvfb-run -a` with
  `XDG_SESSION_TYPE=x11`, so that GLFW uses the Xvfb display. Otherwise GLFW tries Wayland
  first, and on GNOME or KDE it finds the desktop's socket (`$XDG_RUNTIME_DIR/wayland-0`)
  even without `WAYLAND_DISPLAY`. Under Xvfb its check program prints exactly `output.txt`;
  with no display at all it prints "raylib: no window" instead.
- win32 needs no display under Wine when Wine's graphics driver is `null`:
  `wine reg add 'HKCU\Software\Wine\Drivers' /v Graphics /d null /f` once in the prefix. Give Wine
  its own `WINEPREFIX`, `WINEDEBUG=-all` and
  `WINEDLLOVERRIDES="winemenubuilder.exe=d;mscoree,mshtml=;winewayland.drv=d"`, and stop it
  afterwards with `wineserver -k`. `winewayland.drv=d` matters while that first command creates
  the prefix, before the driver is `null`: without it, Wine's desktop process finds the Wayland
  desktop's socket in the same way.
- Both write into the home folder. Even with `winemenubuilder.exe=d`, Wine writes fontconfig
  caches there, whatever `WINEPREFIX` says. Without `winemenubuilder.exe=d`, it also writes
  desktop entries and icons. The GL drivers write shader caches into `~/.cache`. So run them
  with `HOME`, `XDG_DATA_HOME`, `XDG_CONFIG_HOME` and `XDG_CACHE_HOME` pointing at a scratch
  folder, and without `DISPLAY` and `WAYLAND_DISPLAY`. With the settings above, no window shows
  up on the desktop, not even on a Wayland one. The check script below does all of this.

## Where the expected outputs come from

**Never written by hand when Go can produce them.**

- **`go run`**: the program is valid Go. [tools/gen-example-output](../tools/gen-example-output)
  copies the folder into a temporary module, renames `.gp` to `.go`, builds it with `go build`,
  runs it the way the harness runs the gocpp program (in the example's folder, with `args.txt`,
  `env.txt` and `stdin.txt`, checking `// ExitCode:`) and writes its standard output to
  `output.txt`. When there are `_test.gp` files, `go test` must pass too.
- **twin**: the program uses gocpp-only syntax (`!`, macros). `twin/` holds the same program
  expanded by hand into the Go code that gocpp generates for it (plan Phase 10 §2.1 and §2.2,
  like the spikes in `spikes/2026-10-13/must-functions` and `macros-v3-wrap`), and the tool runs
  the twin. The twin of [macros](macros/twin/main.go) writes each macro use site with its
  `main.gp` line, so `#file()` and `#line()` print the right place; the tool checks that each
  site still points at a line of `main.gp` that has the macro use and its argument text.
- **hand-written (interop)**: the program uses C/C++, which Go can't run. Its `output.txt` was
  written from what the program must print and then checked by running the C/C++ side for real:
  - cpp-std, sqlite, raylib and mixed-sources: a C++ program that does the same calls, built
    with g++ and clang++ in the dev shell, gave the same output with both; mixed-sources used its
    own `checksum.c` (built with the C compiler) and `textutil.cpp`; raylib ran in smoke mode
    under Xvfb;
  - multi-pkg: the same packages as a Go module, with `codes` using cgo on the same `luhn.c`;
    real Go printed this output, including the init order (`codes`, `geom`, `catalog`, `main`);
  - win32: a C++ program with the same calls, built with the MinGW-w64 cross compiler
    (`-static`, with `UNICODE`, `WIN32_LEAN_AND_MEAN` and `NOMINMAX` as gocpp defines them) and
    run in smoke mode under Wine. It writes `\n` like Go does, not the C runtime's `\r\n`.

  These check programs are in
  [tools/gen-example-output/testdata/interop](../tools/gen-example-output/testdata/interop), and
  `nix develop -c sh tools/gen-example-output/testdata/interop/check.sh` runs them all again
  (raylib runs under `xvfb-run`, which the dev shell has). When an interop example changes,
  change its `output.txt` by hand, change its check program the same way, and run the script.

## Regenerating the outputs

In the repository root:

```sh
go run ./tools/gen-example-output              # rewrite every output.txt made by Go
go run ./tools/gen-example-output -check       # only check: fails if one is missing or out of date
go run ./tools/gen-example-output -check -race # the same, with the race detector
go run ./tools/gen-example-output wc macros    # only these examples
```

Before running a program the tool also runs `go vet`, checks the header lines of `main.gp`, and
checks that every `.gp` file parses and is formatted like gofmt. In the twin and interop
examples, gocpp-only syntax (`name!(`, `#Name`, `macro`, `lazy`, `std.map`, `std.array[int, 5]`,
`std.chrono.seconds` as a type) is mapped to Go of the same shape first, like
`spikes/2026-10-13/tools/gpfmtcheck.py`; a file that still doesn't parse is an error.
`go test ./tools/gen-example-output` runs the same check over all examples, so `make test` fails
when an `output.txt` is out of date.
