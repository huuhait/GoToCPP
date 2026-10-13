# ADR-001: The compiler is written in Go

Status: accepted (2026-10-13)

## Context

gocpp needs a compiler that reads `.gp` files (Go syntax) and writes C++. The language we write
it in decides how much code we can reuse, how fast the tools are, and how pleasant the work is.
The project's author likes Go and wants to avoid writing C++ by hand.

## Decision

The whole `gocpp` tool is one Go program (module `github.com/huuhait/gocpp`): the compiler, the
build driver, `gocpp fmt`, `vet`, `doc` and the language server.

Go fits because:
- Go ships its own front-end as libraries: `go/scanner`, `go/parser`, `go/ast`, `go/types`,
  `go/constant` and `go/format`. gocpp is Go syntax, so they do most of the front-end work
  ([ADR-002](002-front-end-go-parser-fork.md)).
- Go's tools work on the same data: `go/doc`, `golang.org/x/tools/go/analysis` (for vet), and
  the design of gopls ([ADR-015](015-language-server.md)).
- Go builds one static binary. `go:embed` puts the C++ runtime and the standard library inside
  it ([ADR-009](009-embedded-runtime.md)), and building `gocpp.exe` for Windows is one command.

## Alternatives

- **C++** (rejected): the language we want to avoid writing by hand.
- **Rust** (rejected): Go ships its own parser and type checker as libraries (`go/parser`,
  `go/types`; root README §5), and only a Go program can use them.

## Consequences

Good:
- Most of the front-end is reused code. Our own work goes into what is new: header imports,
  C++ code generation and the runtime.
- The language server runs the same engine as gopls, so gopls-like speed is realistic.
- The real Go toolchain is the reference: differential tests run the same program with
  `go run` and with `gocpp run`.

Bad:
- Two toolchains: Go builds `gocpp`, and a C++20 compiler (installed separately) builds the
  user's programs.
- gocpp follows Go releases: the parser fork is copied from the dev shell's Go (1.25.14 now,
  with a re-fork script), and `x/tools` is pinned together with the Go toolchain. Go 1.25 is
  end-of-life upstream and was removed from nixos-unstable, so the dev shell pins nixpkgs to
  `nixos-26.05`.

## Evidence

- Spike 2026-10-13: stock `go/parser` + `go/types` (with `FakeImportC`) type-checked an
  interop-heavy `.gp` file with 0 false errors ([frontend](../../spikes/2026-10-13/frontend/)).
- Spike 2026-10-13: `go/types` re-checks `net/http` (28.5k lines) in about 29 ms with cached
  dependencies ([lsp-speed](../../spikes/2026-10-13/lsp-speed/)).

See: [root README §1](../../plans/2026-10-13-gocpp/README.md#1-the-idea), [§5](../../plans/2026-10-13-gocpp/README.md#5-key-decisions-details-and-alternatives-in-phase-00), [Phase 00 §2.5](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#25-dev-environment), [Phase 01 §1](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#1-idea), [§2.2](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#22-the-fork-three-patches-to-goscanner--goparser), [Phase 11 §1](../../plans/2026-10-13-gocpp/phase-11-language-server/README.md#1-idea), [Phase 15 §2.4](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#24-packaging-and-install).
