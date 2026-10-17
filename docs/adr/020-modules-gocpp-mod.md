# ADR-020: Modules use a `gocpp.mod` file in `go.mod` syntax

Status: accepted (2026-10-13)

## Context

Real projects have many packages and depend on other modules. Go users already know `go.mod`.
gocpp also needs module-wide C/C++ settings: the C++ standard, compiler flags, `pkg-config`
libraries.

## Decision

A module has a `gocpp.mod` file at its root, in `go.mod` syntax, plus a few gocpp lines. The
module root is found by walking up to it.

```
module github.com/huuhait/game

gocpp 0.1                    // minimum gocpp version
cxx c++20                    // C++ standard for generated + user C++ files

require github.com/huuhait/mathx v0.3.0
replace github.com/huuhait/mathx => ../mathx

cflags -DGAME_DEBUG
pkg-config sdl2
```

- It is parsed with `golang.org/x/mod/modfile` in lax mode: `ParseLax` accepts the unknown
  gocpp lines and fills `module` and `require`. Lax mode skips `replace`, so gocpp reads
  `replace` and its own lines from the raw syntax tree (`f.Syntax`). If that gets awkward, a
  small parser of our own (about 200 lines) is fine.
- In v1, `require` resolves through `replace` (a local path) or a `vendor/` folder.
- Downloads come in Phase 15 with `gocpp get`, Go-modules style: versions from git tags
  (`vX.Y.Z`), a module cache in `$GOCPPMODCACHE`, hashes in `gocpp.sum`, and Go's minimal
  version selection.

## Implementation note (Phase 06 step 6.1)

The "small parser of our own" was chosen: `golang.org/x/mod` is not a requirement of the gocpp module,
and the version that `golang.org/x/tools` needs (v0.35.0) is not in the offline module cache of the
build machines (the newer ones would pull in a newer `x/tools` and `x/sync`). `internal/load/gocppmod`
(about 1300 lines with the vendor list and its checks) reads `go.mod` syntax and does what the ADR asked of lax mode,
with `replace` and the gocpp lines read as ordinary statements. It also reports every broken statement
instead of stopping at the first one, keeps unknown statements and brace blocks, and writes the file
back (`Format`). Switching to `x/mod/modfile` later is possible: the syntax tree is close to its shape.
See [docs/spec/modules.md](../spec/modules.md).

## Alternatives

- **A new module format** (rejected): one more format to learn, while `go.mod` syntax fits and
  already has a parser (x/mod).
- **Requiring CMake or Conan** (rejected): an extra tool to install ([ADR-008](008-build-driver.md)).

## Consequences

Good: familiar to Go users; x/mod gives `modfile`, `semver`, `module`, `zip` and `sumdb/dirhash`.

Bad:
- No downloads before Phase 15: only local paths and vendoring.
- x/mod has two catches (found in the audit): `module.CheckPath` rejects paths without a dot,
  so local module names like `myapp` must skip it; and `x/mod/zip` only knows `go.mod` as a
  module boundary, so gocpp filters out nested `gocpp.mod` folders itself.
- C/C++ libraries are not modules: they come from the system (`pkg-config`, flags, directives).

## Evidence

- Audit: `ParseLax` accepts the gocpp lines and fills `module`/`require`, skips `replace`, and
  `f.Syntax` still contains every line.

See: [Phase 06 §2.1](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#21-the-module-file-gocppmod), [Phase 01 §2.6](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#26-package-loading-and-the-importer), [Phase 15 §2.5](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#25-module-downloads-gocpp-get), divergence [D11](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#22-the-language-as-a-diff-against-go).
