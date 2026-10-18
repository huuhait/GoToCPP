# Architecture decision records (ADRs)

An ADR records one decision that the rest of gocpp depends on. Each file says why the decision
was needed (Context), what was decided (Decision), what was rejected and why (Alternatives), and
what follows from it, good and bad (Consequences). Each ADR also lists the experiments and
findings behind it (Evidence) and links to the plan sections that build it (See).

These ADRs come from the plan
([Phase 00 §2.1](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#21-decision-records-adrs))
and from the user's answers to Q1–Q13
([TODO.md](../../plans/2026-10-13-gocpp/TODO.md#decisions-answered-2026-10-13)).

| ADR | Title | Status | In one line |
|---|---|---|---|
| [001](001-compiler-in-go.md) | The compiler is written in Go | accepted | Go ships its own parser and type checker as libraries, and gocpp is Go syntax. |
| [002](002-front-end-go-parser-fork.md) | The front-end is a fork of go/scanner + go/parser, with stock go/types | accepted | 3 parser patches in Phase 01 and 3 in Phase 10; `go/types` stays stock so x/tools keeps working. |
| [003](003-readable-cpp20-output.md) | The output is readable C++20 with `#line` directives | accepted | GCC ≥ 13 and Clang ≥ 17; debuggers and compiler errors point at `.gp` lines. |
| [004](004-memory-arc.md) | Memory is automatic reference counting (ARC) | accepted | `*T` is `gp::ptr<T>` over `std::shared_ptr`; reference cycles need `weak.Pointer`. |
| [005](005-interop-header-packages.md) | C/C++ headers and C++ namespaces are used like packages | accepted | `raylib.InitWindow`, `std.vector[int]`; `C` keeps only C's built-in types and helpers; opaque first. |
| [006](006-numbers-and-checks.md) | `int` is 64-bit, and arithmetic and runtime checks work like Go | accepted | `-fwrapv`, small integers cast back, checked division and shifts, Go's panic messages. |
| [007](007-goroutines-os-threads.md) | Goroutines are OS threads, with no scheduler | accepted | `go` starts a real thread; there is no goroutine scheduler, now or later. |
| [008](008-build-driver.md) | gocpp has its own build driver with a content-hash cache | accepted | It calls `$CXX` itself and writes `compile_commands.json` and CMake files for other tools. |
| [009](009-embedded-runtime.md) | The runtime is a C++20 library embedded in the gocpp binary | accepted | A mostly-header library in `namespace gp`, shipped inside `gocpp` with `go:embed`. |
| [010](010-cpp-naming.md) | Generated C++ names: flat package namespaces, fully qualified C names | accepted (the `v := v` row superseded by 022) | `gpkg_<escaped import path>` namespaces, `::time` for C names, `_` escapes for keywords and macros. |
| [011](011-errors-panic-defer.md) | `error` is an interface, `panic` is a C++ exception, `defer` is a frame | accepted | Go's exact defer and recover order in a single-exit C++ function shape. |
| [012](012-go-subset-and-extensions.md) | gocpp is Go 1.25 syntax plus a small, explicit set of extensions | accepted | The language is a short, written list of differences from the Go spec. |
| [013](013-must-errors.md) | `!` (must) returns errors for you, only where it is written | accepted | `func f!()` and `F!(...)`; in `main` the error is printed and the program exits 1. |
| [014](014-macros.md) | Macros are `macro` declarations used as `#Name(...)` | accepted | Typed macros inside code or on top of a func (start or wrap style), and `#Derive` on types. |
| [015](015-language-server.md) | The language server copies gopls and has speed budgets in CI | accepted | Snapshots, content-hash caches, export-data summaries; p95 budgets checked in CI. |
| [016](016-interfaces-itabs.md) | Interfaces use Go-style type descriptors and itabs | accepted | Descriptors follow the Go type, so `int` and `int64` stay different; no C++ inheritance. |
| [017](017-generics-templates.md) | Generics are C++ templates with explicit type arguments | accepted | Type arguments always come from `go/types`, never from C++ deduction. |
| [018](018-stdlib-ported-from-go.md) | The standard library is ported from Go's own source | accepted | Copied by a tool with small patches, plus a `reflect` subset on the type descriptors. |
| [019](019-output-layout.md) | Output layout: one header per package, one `.cpp` per `.gp` file | accepted | `pkg.hpp`, `<file>.gp.cpp`, `pkg_init.cpp`, and Go 1.21's init order in `gp_init_all()`. |
| [020](020-modules-gocpp-mod.md) | Modules use a `gocpp.mod` file in `go.mod` syntax | accepted | `replace` and `vendor/` first; downloads with minimal version selection in Phase 15. |
| [021](021-platforms-linux-windows.md) | The platforms are Linux and Windows (MinGW-w64 first) | accepted | Linux first, Windows before v1.0; MSVC ABI later; no macOS. |
| [022](022-shadowed-names.md) | Shadowed variables keep their Go names in C++ | accepted | A shadowing declaration opens a C++ block; supersedes ADR-010's `v := v` row. |
| [023](023-module-downloads.md) | Modules are downloaded from git hosts, with hashes in `gocpp.sum` and trust on first use | accepted | Tag or commit archives from GitHub, GitLab and Codeberg (or `GOCPP_MODURL`), Go's `h1:` tree hash, no checksum database yet. |
| [024](024-cxx-recipes.md) | C and C++ libraries are built from a `cxx` recipe in `gocpp.mod` | accepted | Sources, include folders, defines and pkg-config, through the flag allow-list; objects in the shared object cache. (023 is job 6.1c's.) |

## Changing a decision

An accepted ADR keeps its decision: to change it, write a new ADR that says
"Supersedes ADR-004", and set the old ADR's status to `superseded by ADR-0xx`
([Phase 00 §6](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#6-risks)). A new
decision gets a new ADR too, for example a `go/types` fork
([Phase 12 §2.4](../../plans/2026-10-13-gocpp/phase-12-typed-interop/README.md#24-type-checking-an-interop-checker-no-gotypes-fork))
or a direction chosen after 1.0
([Phase 15 §2.8](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#28-after-10-each-becomes-an-adr-when-chosen)).
