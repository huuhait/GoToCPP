# gocpp

gocpp is a small programming language that looks and feels like Go. You write Go syntax in
`.gp` files, and the `gocpp` tool translates them into normal, readable C++20, then builds a
native program with `clang++` or `g++`. Because the output is plain C++, a `.gp` file can import
any C or C++ header (`"<vector>"`, `"<SDL2/SDL.h>"`, `"./engine.hpp"`) and call it directly.

gocpp keeps Go's syntax and style (packages, `:=`, multiple results, methods, interfaces, slices,
maps, `defer`, goroutines, generics, gofmt formatting) and drops what doesn't fit C++, such as the
garbage collector: memory is reference counted. It adds a few things in Go style: `!` (must),
which returns errors for you, typed macros, and a language server as fast as gopls. It targets
Linux and Windows.

## How Go++ compares

Go++ is not a replacement for Go, C++ or Rust. It sits between them: Go's syntax and tooling feel
with C++ as the target, so you can call C and C++ libraries without glue code. The table is a
rough guide, not a benchmark. Entries marked *planned* are not built yet, and the Status section
below says what works today.

| | **Go++** | **Go** | **C++** | **Rust** |
|---|---|---|---|---|
| Syntax | Go's (`.gp` files, gofmt style) | Go's | C-style, many features | Own syntax, expression-oriented |
| Learning curve | Low if you know Go | Low | High | High (ownership and lifetimes) |
| Compiles to | Readable C++20, then native code via g++ or clang++ | Native code (own compiler) | Native code | Native code (LLVM) |
| Memory management | Reference counting; cycles leak (`weak.Pointer` breaks them) | Garbage collector | Manual or RAII (`unique_ptr`, `shared_ptr`) | Ownership and borrow checker, no GC |
| Memory safety | Bounds and nil checks, Go-style panics; raw C/C++ pointers are unsafe | Safe (except `unsafe` and cgo) | Not safe by default (undefined behavior) | Safe by default (except `unsafe`) |
| Concurrency | Goroutines and channels, one OS thread each (about 20,000 to 30,000 at most); `select` built, `sync`/`time` in progress | Goroutines on a scheduler, millions; the most mature model | Threads, async libraries, coroutines | Threads, async/await, channels; data races caught at compile time |
| Generics | Go-style type parameters; generic functions built, generic types in progress | Yes | Templates (very powerful, slow to compile) | Traits and generics |
| Error handling | `error` values, `panic`/`recover`, `!` (must) *planned* | `error` values, `panic`/`recover` | Exceptions or error codes | `Result` and `?` |
| Calling C | Import a header and call it | cgo (call overhead, extra build step) | Native | `bindgen` or hand-written FFI |
| Calling C++ | Import a header and call it (`std.vector[int]`); typed checking *planned* | Not directly (cgo only reaches C) | Native | `cxx` crate or C wrappers |
| Build speed | Slower than Go (a C++ compile); caching and incremental builds in progress | Very fast | Slow | Slow to medium |
| Runtime speed | Goal: within 2x of Go in the worst case; not measured yet | Fast | Fastest | As fast as C++ |
| Standard library | Small today (`fmt`, `errors`); porting Go's library is the largest remaining phase | Large and mature | Large, but no networking or JSON built in | Medium, plus a very large crate ecosystem |
| Tooling | `gocpp fmt`, `check`, `build`, `run`; language server (`gocpp lsp`) | Excellent (`go` command, gopls) | Fragmented (CMake, clangd) | Excellent (`cargo`, rust-analyzer) |
| Cross-compiling | Linux now; Windows build from Linux in progress | Very easy | Hard | Easy to medium |
| Maturity | Young: phases 00 to 05 built, many packages missing | Mature | Mature | Mature |

**When Go++ makes sense:** you like Go's syntax and workflow, but the code must call C or C++
libraries (a game engine, SQLite, a vendor SDK) or live inside a C++ code base, and you accept
reference counting instead of a garbage collector.

**When something else does:** use Go for network services and tools that need a mature standard
library and fast builds. Use C++ when you need full control and the largest ecosystem. Use Rust
when compile-time memory safety matters most. Go++ is not production-ready yet.

## Status

Phases 00 to 05 are built: the front-end, the walking skeleton, the core language, C/C++ interop
and interfaces, embedding, errors and `panic`/`recover` (`gocpp version` prints `phase 05`). The
language server's first version is done too, and programs of several packages work (Phase 06 is
under way: the build cache and the build modes are next). What works today:

- `gocpp fmt` formats `.gp` files like gofmt (`gocpp fmt -h`).
- `gocpp check` parses and type-checks packages, C/C++ header imports included, and prints their
  errors like `go build` (`gocpp check ./...`; `gocpp check -h`). Features of later phases get a
  "not supported yet (Phase NN)" error; [docs/spec/feature-matrix.md](docs/spec/feature-matrix.md)
  lists them.
- `gocpp build`, `gocpp run` and `gocpp emit` compile programs to C++20 and build them with g++ or
  clang++: Go's language without generics, `select` and the `sync`/`time` packages, so basic and
  complex types, functions, loops, `switch`, labels, pointers, slices, arrays, maps, structs, named
  types, methods, closures, `defer`, interfaces (`any`, `error`, type assertions and type switches,
  embedding, method values and expressions), `panic` and `recover` (also for C++ exceptions),
  `weak.Pointer`, the `errors` package, `fmt` for every value and verb, and programs of several
  packages (a module with `gocpp.mod`). Panics print Go's messages and stack traces, and a program
  exits with Go's exit codes (`-nochecks` turns off the bounds and nil checks).
  [examples/hello](examples/hello/main.gp), [structs](examples/structs/main.gp),
  [closures](examples/closures/main.gp), [wordfreq-core](examples/wordfreq-core/main.gp),
  [shapes](examples/shapes/main.gp) (interfaces and embedding) and [multi-pkg](examples/multi-pkg/main.gp) run,
  and gdb can debug them at the `.gp` level. C/C++ headers and libraries work (`"<vector>"`,
  `"<sqlite3.h>"`, `std.vector[int]`; see [docs/spec/interop.md](docs/spec/interop.md)):
  [cpp-std](examples/cpp-std/main.gp), [mixed-sources](examples/mixed-sources/main.gp) and
  [sqlite](examples/sqlite/main.gp) run. Goroutines and channels are built but gated until the rest of
  Phase 08 (`select`, `sync`, `time`) lands.
- `gocpp lsp` is the language server (`gocpp lsp -h`): errors as you type, hover, go to
  definition, signature help, completion (with auto-import), formatting and organize imports. The
  VS Code extension in [editors/vscode](editors/vscode/README.md) starts it;
  [docs/guides/editors.md](docs/guides/editors.md) sets up Neovim, Helix and Emacs. It keeps
  summaries of the packages it checks in a cache on disk, `gocpp/lsp` in your user cache folder
  (set `GOCPP_LSP_CACHE` to an absolute folder to move it, or to `off`).

A command that is not built yet says which phase builds it.

- The plan: [plans/2026-10-13-gocpp/README.md](plans/2026-10-13-gocpp/README.md)
- Progress: [plans/2026-10-13-gocpp/STATE.md](plans/2026-10-13-gocpp/STATE.md)
- How work is done: [plans/2026-10-13-gocpp/WORKFLOW.md](plans/2026-10-13-gocpp/WORKFLOW.md)

## Build and test

You need Go 1.25 and make, and `make ci` also needs `g++` and `clang++`. The Nix dev shell has
all of them and the rest of the toolchain: run `nix develop` in the repo root, or use direnv
with the committed `.envrc` (run `direnv allow` once).

```sh
make ci           # every required check: lint, test, build
make ci-sanitize  # the test programs and emitter tests with ASan, LSan and UBSan
make build        # build bin/gocpp
make test         # go test ./...
make lint         # gofmt check (not spikes/ or testdata/), then go vet ./...
make staticcheck  # staticcheck ./... (not in ci: go run downloads it once)
make vscode-test  # VS Code extension tests (not in ci: npm downloads the packages once)
make clean        # remove bin/

bin/gocpp version
```

For the test programs (`make test-programs`) and their variables `GOCPP_CXX`,
`GOCPP_REQUIRE_CXX`, `GOCPP_FLAGS`, `GOCPP_TEST_PHASE` and `GOCPP_TEST_PENDING`, see
[tests/README.md](tests/README.md). `make ci` runs the test programs up to Phase 08, not only up to
the last built phase (`CI_TEST_PHASE` in the Makefile), so that the goroutines, channels and
multi-package tests stay green; a test that still waits for a step says `// pending:` and is skipped.

`spikes/` holds the throwaway experiments behind the plan. It is a separate Go module, so
`./...` in the root doesn't include it; see [spikes/2026-10-13/README.md](spikes/2026-10-13/README.md)
for how to re-run them.

## License

BSD-3-Clause, see [LICENSE](LICENSE).
