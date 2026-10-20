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

Go++ sits between Go and C++: Go's syntax and tooling, with C++ as the target, so you can call C
and C++ libraries without glue code. The table is a rough guide, not a benchmark.
✅ yes, ❌ no, 🚧 being built (the Status section below says what works today).

| | Go++ | Go | C++ | Rust |
|---|:---:|:---:|:---:|:---:|
| Simple syntax, easy to learn | ✅ | ✅ | ❌ | ❌ |
| No garbage collector | ✅ | ❌ | ✅ | ✅ |
| Memory safe by default | ✅ | ✅ | ❌ | ✅ |
| Memory safety proven at compile time | ❌ | ❌ | ❌ | ✅ |
| Goroutines, channels and `select` | ✅ | ✅ | ❌ | ❌ |
| Millions of lightweight tasks | ❌ | ✅ | ❌ | ✅ |
| Generics | 🚧 | ✅ | ✅ | ✅ |
| Call C directly, no wrapper | ✅ | ❌ | ✅ | ❌ |
| Call C++ directly, no wrapper | ✅ | ❌ | ✅ | ❌ |
| Fast builds | ❌ | ✅ | ❌ | ❌ |
| Formatter and language server included | ✅ | ✅ | ❌ | ✅ |
| One tool for build, test and packages | 🚧 | ✅ | ❌ | ✅ |
| Easy cross-compiling | 🚧 | ✅ | ❌ | ✅ |
| Large standard library | 🚧 | ✅ | ✅ | ❌ |
| Ready for production | ❌ | ✅ | ✅ | ✅ |

Notes: Go++ counts as memory safe except for raw C and C++ pointers. Its memory is reference
counted, so cycles leak (`weak.Pointer` breaks them). Goroutines are OS threads, so about 20,000
to 30,000 is the limit. Rust's standard library is small, but its crate ecosystem is large.

**Use Go++ when** you like Go's syntax and workflow but must call C or C++ libraries (a game
engine, SQLite, a vendor SDK) or live inside a C++ code base, and reference counting is fine.

**Use something else when** you need:
- a mature standard library and fast builds: Go
- full control and the biggest ecosystem: C++
- memory safety proven at compile time: Rust

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
- `!` (must), must functions, macros (`macro`, `lazy`, `#Name(...)`, `#file()`, `#line()`, `#func()`,
  `#text()`, `#body()`, `#embed()`), `#Derive` and `//go:embed` are always on: every command
  accepts them, and no environment variable is needed ([docs/spec/language.md](docs/spec/language.md#d12-must-for-errors),
  [examples/macros](examples/macros/main.gp), [examples/must-errors](examples/must-errors/main.gp)).
- `gocpp expand` prints the plain Go that the macros, `#Derive` lines and `!` forms of a package
  stand for, so you can see what the compiler does ([docs/guides/expand.md](docs/guides/expand.md);
  `gocpp expand -h`).
- `gocpp build`, `gocpp run` and `gocpp emit` compile programs to C++20 and build them with g++ or
  clang++: Go's language without generics (the Phase 07 work is gated), so basic and
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
  [sqlite](examples/sqlite/main.gp) run. Goroutines, channels, `select`, `sync`, `sync/atomic` and `time` are built but gated until
  Phase 08 closes.
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

## Install

```sh
go install github.com/huuhait/gocpp/cmd/gocpp@latest    # needs Go 1.25 and a C++ compiler (g++ 13+ or clang++ 17+)
nix run github:huuhait/gocpp -- run hello.gp             # brings g++ and clang++
```

Release archives are configured for linux amd64/arm64 and windows amd64; that is not a claim
that a v0.1 release is available or validated on every target. The runtime and standard library
are inside the binary. [Installation](docs/guides/packaging.md) covers source builds and the
packaging mechanisms; [v0.1 preparation](docs/guides/release-candidate.md) records the draft
changelog, known limits and open acceptance gates. No release is approved by these notes.

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
[tests/README.md](tests/README.md). `make ci` runs the test programs up to Phase 10, not only up to
the last built phase (`CI_TEST_PHASE` in the Makefile), so that the goroutines, channels and
multi-package tests stay green; a test that still waits for a step says `// pending:` and is skipped.

`spikes/` holds the throwaway experiments behind the plan. It is a separate Go module, so
`./...` in the root doesn't include it; see [spikes/2026-10-13/README.md](spikes/2026-10-13/README.md)
for how to re-run them.

## License

BSD-3-Clause, see [LICENSE](LICENSE).
