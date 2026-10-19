# A Tour of gocpp

This tour is for people who know a little Go, or a little C++. It goes from a first program to
goroutines, generics, C and C++ libraries and macros in thirteen short lessons. Every program in
it is checked by the test suite: it compiles with `gocpp check`, it runs and prints exactly what
the page shows, and the ones that are valid Go print the same with the `go` command.

gocpp is a compiler for a language that is Go (go1.25) with a short list of differences. It writes
readable C++20 and builds it with `g++` or `clang++`. You do not need to know C++ to start, but you
need a C++ compiler on the machine (`g++` 13 or newer, or `clang++` 17 or newer): the Nix package of gocpp
(`nix run github:huuhait/gocpp`) brings one, and the [packaging guide](../guides/packaging.md) has the other ways to
install and the supported platforms.

## How to run a lesson

Make a folder, save the program as `main.gp`, and run it there:

```sh
mkdir hello && cd hello
# write main.gp
gocpp run .          # compile and run
gocpp check .        # only parse and type-check; no C++ compiler is needed
gocpp build .        # leave the executable in the folder
```

`.gp` is the extension of gocpp files. Go's tools do not read them, but a program that uses no
gocpp-only feature is also valid Go: rename it to `main.go` and `go run` prints the same. The
lessons that are valid Go say so.

## What works today, and the switch

gocpp is built in phases ([the plan](../../plans/2026-10-13-gocpp/README.md)). The compiler refuses
a feature whose phase is not finished, with an error that names the phase, so you never get broken
C++. Phases 00 to 05 are finished (`gocpp version` prints the last finished phase). The [feature matrix](../spec/feature-matrix.md)
lists every feature with its phase.

> **Experimental today.** A lesson with this note uses a feature of a phase that is still being
> built. It works, but the compiler hides it behind a switch: set the environment variable
> `GOCPP_GATE_PHASE` to the phase the lesson names, for example
> `GOCPP_GATE_PHASE=08 gocpp run .`. The switch is for gocpp's own development and goes away when the phase is
> finished. The lessons say which phase each one needs.

| Lesson | Needs |
|---|---|
| [1 Hello](01-hello.md) to [5 Interfaces and errors](05-interfaces-errors.md) | nothing: phases 02 to 05 are finished |
| [6 Generics](06-generics.md) | nothing: generic functions and types are open from phase 05 on |
| [7 Goroutines and channels](07-goroutines.md) | `GOCPP_GATE_PHASE=08` (experimental) |
| [8 The standard library](08-stdlib.md) | `GOCPP_GATE_PHASE=09` (experimental) |
| [9 C++ containers](09-cpp-containers.md) and [10 C libraries](10-c-libraries.md) | nothing: interop with C and C++ is finished (phase 04) |
| [11 Must functions](11-must-functions.md) | `GOCPP_GATE_PHASE=09` (experimental: the lesson uses `strconv`; the `!` itself needs nothing) |
| [12 Macros](12-macros.md) | nothing: `!`, macros and `#Derive` are always on (phase 10) |

## The lessons

1. [Hello, gocpp](01-hello.md): a program, `gocpp run`, and what `package main` is.
2. [Values and control flow](02-values-and-control-flow.md): constants, loops, `switch`, functions with several results.
3. [Slices, maps, structs and methods](03-data.md): the data types, pointers and methods.
4. [Closures and defer](04-closures-and-defer.md): function values, `defer` order and named results.
5. [Interfaces and errors](05-interfaces-errors.md): interfaces, type switches, `error` values and `errors.Is`.
6. [Generics](06-generics.md): type parameters, constraints, generic types and `iter.Seq`.
7. [Goroutines and channels](07-goroutines.md): `go`, channels, `select`, `sync.WaitGroup`; a goroutine is an OS thread.
8. [The standard library](08-stdlib.md): `strings`, `strconv`, `sort`, `bufio`, `os`.
9. [C++ containers](09-cpp-containers.md): `std::vector`, `std::map` and `std::string` as they are.
10. [C libraries](10-c-libraries.md): call `<math.h>` and `<stdio.h>`; the rules at the border.
11. [Must functions](11-must-functions.md): `func f!()` and `F!()`, errors without `if err != nil`.
12. [Macros](12-macros.md): `macro`, `#Assert`, `#Derive`, `#Trace`.
13. [Where to go next](13-next.md): modules, tests, the tools, and the guides.
