# gocpp docs

| Path | What is there |
|---|---|
| [spec/](spec/README.md) | The language: `language.md` (gocpp as a list of differences from the Go spec), `cpp-mapping.md` (how gocpp code maps to C++), `interop.md` (C/C++ headers, C++ namespaces, the `C` package) and `interop-cookbook.md` (recipes) |
| [tour/](tour/README.md) | A Tour of gocpp: thirteen short lessons from hello world to goroutines, generics, C++ containers and macros. Every program is checked and run by the tests |
| [index.md](index.md) | The home page of the documentation site |
| [adr/](adr/README.md) | Decision records (ADRs): one file per decision, `NNN-short-title.md`. An accepted ADR is never rewritten: a new or changed decision gets a new ADR |
| [guides/](guides/editors.md) | How-to guides for users: [editors.md](guides/editors.md) (VS Code, Neovim, Helix, Emacs, Zed and the language server), [race.md](guides/race.md) (the race detector, `gocpp run -race`), [vet.md](guides/vet.md) (`gocpp vet`), [doc.md](guides/doc.md) (`gocpp doc`), [modules.md](guides/modules.md) (`gocpp get`, `gocpp.mod`, `gocpp.sum`), [llvm-tools.md](guides/llvm-tools.md) (compiler plugins and LLVM IR), [embed.md](guides/embed.md) (`//go:embed`, `#embed` and the built-in macros), [expand.md](guides/expand.md) (`gocpp expand`: what macros, `#Derive` and `!` become), [generics-compile-time.md](guides/generics-compile-time.md) (what generics cost to compile, and the cheap ways to lower it), [packaging.md](guides/packaging.md) (`go install`, release archives, the Nix package). [sanitizers.md](guides/sanitizers.md), [go-developers.md](guides/go-developers.md), [cpp-developers.md](guides/cpp-developers.md), [interop-examples.md](guides/interop-examples.md) and [docs-site.md](guides/docs-site.md) (how this folder becomes a site) |
| [benchmarks/](benchmarks/README.md) | The benchmark suite (gocpp against Go and C++, `make bench`), the regression gate and the generated table [runtime.md](benchmarks/runtime.md); the language server against gopls ([lsp.md](benchmarks/lsp.md), `make lsp-bench`) and on a Windows PC ([lsp-windows.md](benchmarks/lsp-windows.md), `TestWindowsLSPBench`) |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How work is done here: the manager → coder → reviewer loop, tests, Go and C++ style, platforms, commits |

The docs are published as a static site: `make docs` writes it to `site/` (see [guides/docs-site.md](guides/docs-site.md)); it adds the standard library reference, generated from the sources.

The plan behind all of this is in [plans/2026-10-13-gocpp/](../plans/2026-10-13-gocpp/README.md).
