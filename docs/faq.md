# Frequently asked questions

Short answers, each one with the place that has the details. The facts here are checked by the
tests of the documentation site.

## Is gocpp Go?

It is Go 1.25 with a short list of differences (D1 to D28 in the [language spec](spec/language.md)),
and three additions: header imports, must functions and macros. Most Go programs that stay within
the supported standard packages compile after renaming `.go` to `.gp`; see
[gocpp for Go developers](guides/go-developers.md). It is not a goal that every Go program or module
builds unchanged.

## Do I have to install a C++ compiler?

Yes, `g++` 13 or newer, or `clang++` 17 or newer, unless you use the Nix package, which brings GCC 15 and
clang 21. `gocpp env` shows the compiler that was found. See [installing gocpp](guides/packaging.md).

## Which platforms does it run on?

Linux on x86-64 (the platform the tests run on every day), Linux on arm64 (released, not yet tested in CI), and
Windows 10 or later on x86-64, with a MinGW-w64 compiler. macOS is not a target. The details are in
[installing gocpp](guides/packaging.md#platforms-and-requirements).

<!-- platforms: linux/amd64 linux/arm64 windows/amd64 -->

## Is it ready for production?

Not yet. The compiler is built in phases, `gocpp version` prints the last finished one, and the
[feature matrix](spec/feature-matrix.md) says which features work and which are hidden behind
`GOCPP_GATE_PHASE`. The goal is a v1.0 with a Go-1-style compatibility promise for programs (not for the
exact generated C++).

## Why reference counting, and what about cycles?

C++ has no garbage collector, so memory is reference counted (`gp::ptr<T>`, a `std::shared_ptr`). A cycle of
pointers is never freed; break it with `weak.Pointer[T]`. A data race can also corrupt a count. See
[D1](spec/language.md#d1-reference-counting-instead-of-a-garbage-collector) and
[D10](spec/language.md#d10-data-races-can-break-reference-counts).

## Are goroutines cheap?

A goroutine is an OS thread, so tens of thousands are fine and millions are not
([D7](spec/language.md#d7-goroutines-are-os-threads)). The benchmark page has the numbers against Go
([benchmarks](benchmarks/README.md)).

## Can I call a C or C++ library?

Yes, that is the point: `import "<vector>"` or `import "<sqlite3.h>"` and use the names, with no binding code.
Start with [tour lesson 9](tour/09-cpp-containers.md), then the [interop guide](spec/interop.md), the
[cookbook](spec/interop-cookbook.md) and the [examples walkthrough](guides/interop-examples.md).

## Can C or C++ call my gocpp code?

Yes, a function marked `//export` is callable when the package is built as a C archive or shared library
(`gocpp build -buildmode=c-archive`): [calling gocpp from C and C++](guides/libraries.md).

## Which standard packages exist?

A subset of Go's: 37 packages, listed in the [standard library reference](stdlib/index.html). `log`,
`math/rand`, `path/filepath`, `net/http` and others are missing; the list is in
[what is not there](guides/go-developers.md#what-is-not-there).

## How do I see the C++ that gocpp generates?

`gocpp emit .` prints it; `gocpp emit -annotate .` adds the `.gp` line of each part and the reasons for the
choices. [gocpp for C++ developers](guides/cpp-developers.md) reads some of it with you.

## How do I debug a program?

`gocpp debug` builds the program and starts `gdb` or `lldb` on it; the generated C++ carries `#line`
directives, so the debugger shows your `.gp` lines. See the [debugging guide](guides/debugging.md). The race
detector is `gocpp run -race` ([guide](guides/race.md)).

## Why is the first build slow?

The first build also compiles the runtime (once per compiler and flags, then it is cached), and C++ compile
times are what they are. [What generics cost to compile](guides/generics-compile-time.md) shows the cheap
ways down.

## Is there editor support?

Yes: `gocpp lsp` is a language server, and the [editor guide](guides/editors.md) has VS Code, Neovim, Helix,
Emacs and Zed.

## Where is the standard library reference, and how is it made?

In the [reference](stdlib/index.html), generated from the `.gp` sources by the code of `gocpp doc -all`
([how the site is built](guides/docs-site.md)). `gocpp doc strings.Builder` shows the same text in a
terminal.
