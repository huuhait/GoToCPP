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

**Phase 00 (foundations).** The repo skeleton is in place, and the decisions and the language
spec are being written. Nothing compiles `.gp` files yet: `gocpp version` and `gocpp help` work,
and every other command says which phase builds it.

- The plan: [plans/2026-10-13-gocpp/README.md](plans/2026-10-13-gocpp/README.md)
- Progress: [plans/2026-10-13-gocpp/STATE.md](plans/2026-10-13-gocpp/STATE.md)
- How work is done: [plans/2026-10-13-gocpp/WORKFLOW.md](plans/2026-10-13-gocpp/WORKFLOW.md)

## Build and test

You need Go 1.25 and make. On NixOS, `nix develop` will give you a shell with the whole toolchain
once `flake.nix` lands (Phase 00, step 0.7).

```sh
make ci           # every check: lint, test, build
make build        # build bin/gocpp
make test         # go test ./...
make lint         # gofmt check (not spikes/ or testdata/), then go vet ./...
make staticcheck  # staticcheck ./... (not in ci: go run downloads it once)
make clean        # remove bin/

bin/gocpp version
```

`spikes/` holds the throwaway experiments behind the plan. It is a separate Go module, so
`./...` in the root doesn't include it; see [spikes/2026-10-13/README.md](spikes/2026-10-13/README.md)
for how to re-run them.

## License

BSD-3-Clause, see [LICENSE](LICENSE).
