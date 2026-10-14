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

## Status

Phase 00 (foundations) and Phase 01 (the front-end) are done; Phase 02 (the walking skeleton) is
under way. What works today:

- `gocpp fmt` formats `.gp` files like gofmt (`gocpp fmt -h`).
- `gocpp check` parses and type-checks packages, C/C++ header imports included, and prints their
  errors like `go build` (`gocpp check ./...`; `gocpp check -h`). Features of later phases get a
  "not supported yet (Phase NN)" error; [docs/spec/feature-matrix.md](docs/spec/feature-matrix.md)
  lists them.
- `gocpp build`, `gocpp run` and `gocpp emit` compile programs of the Phase 02 subset (basic
  types, functions, loops, `switch`, labels, `fmt.Println`) to C++20 and build them with g++ or
  clang++: [examples/hello](examples/hello/main.gp), [fizzbuzz](examples/fizzbuzz/main.gp) and
  [primes](examples/primes/main.gp) run, and gdb can debug them at the `.gp` level.
- `gocpp lsp` is the start of the language server: it speaks LSP over stdin and stdout and keeps
  the text of the open files, but has no features yet (`gocpp lsp -h`).

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
make build        # build bin/gocpp
make test         # go test ./...
make lint         # gofmt check (not spikes/ or testdata/), then go vet ./...
make staticcheck  # staticcheck ./... (not in ci: go run downloads it once)
make vscode-test  # VS Code grammar tests (not in ci: npm downloads the tools once)
make clean        # remove bin/

bin/gocpp version
```

For the test programs (`make test-programs`) and their variables `GOCPP_CXX`,
`GOCPP_REQUIRE_CXX`, `GOCPP_FLAGS` and `GOCPP_TEST_PHASE`, see [tests/README.md](tests/README.md).

`spikes/` holds the throwaway experiments behind the plan. It is a separate Go module, so
`./...` in the root doesn't include it; see [spikes/2026-10-13/README.md](spikes/2026-10-13/README.md)
for how to re-run them.

## License

BSD-3-Clause, see [LICENSE](LICENSE).
