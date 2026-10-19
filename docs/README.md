# gocpp docs

| Path | What is there |
|---|---|
| [spec/](spec/README.md) | The language: `language.md` (gocpp as a list of differences from the Go spec), `cpp-mapping.md` (how gocpp code maps to C++), `interop.md` (C/C++ headers, C++ namespaces, the `C` package) and `interop-cookbook.md` (recipes) |
| [adr/](adr/README.md) | Decision records (ADRs): one file per decision, `NNN-short-title.md`. An accepted ADR is never rewritten: a new or changed decision gets a new ADR |
| [guides/](guides/editors.md) | How-to guides for users: [editors.md](guides/editors.md) (VS Code, Neovim, Helix, Emacs, Zed and the language server), [vet.md](guides/vet.md) (`gocpp vet`), [doc.md](guides/doc.md) (`gocpp doc`). More come with the docs site in Phase 15 |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How work is done here: the manager → coder → reviewer loop, tests, Go and C++ style, platforms, commits |

The plan behind all of this is in [plans/2026-10-13-gocpp/](../plans/2026-10-13-gocpp/README.md).
